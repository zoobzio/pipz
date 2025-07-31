package main

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/zoobzio/pipz"
)

func TestProfileUpdate_HappyPath(t *testing.T) {
	EnableFullProduction() // Enable all features for tests
	ResetAllServices()

	ctx := context.Background()
	profile := ProfileUpdate{
		UserID:    "user123",
		Photo:     []byte("fake image data"),
		Timestamp: time.Now(),
		RequestID: "req123",
	}

	result, err := ProfileUpdatePipeline.Process(ctx, profile)

	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	if result.ModerationResult != ModerationApproved {
		t.Errorf("Expected moderation approved, got %v", result.ModerationResult)
	}

	if result.ProcessedImages.Thumbnail == nil {
		t.Error("Expected processed thumbnail")
	}

	// Check processing log
	expectedLogs := []string{
		"validation: passed",
		"image: processed",
		"moderation-api: approved",
		"database: saved",
	}
	for _, expected := range expectedLogs {
		found := false
		for _, log := range result.ProcessingLog {
			if strings.Contains(log, expected) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected log entry '%s' not found in %v", expected, result.ProcessingLog)
		}
	}
}

func TestProfileUpdate_ModerationFallback(t *testing.T) {
	EnableFullProduction()
	ResetAllServices()

	// Primary moderation fails
	ModerationAPI.Behavior = BehaviorError

	ctx := context.Background()
	profile := ProfileUpdate{
		UserID: "user123",
		Photo:  []byte("fake image data"),
	}

	result, err := ProfileUpdatePipeline.Process(ctx, profile)

	if err != nil {
		t.Fatalf("Expected success with fallback, got error: %v", err)
	}

	if result.ModerationResult != ModerationApproved {
		t.Errorf("Expected moderation approved via backup, got %v", result.ModerationResult)
	}

	// Should have used backup service
	found := false
	for _, log := range result.ProcessingLog {
		if strings.Contains(log, "moderation-backup: approved") {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected to use backup moderation service")
	}
}

func TestProfileUpdate_ModerationSkipped(t *testing.T) {
	EnableFullProduction()
	ResetAllServices()

	// Both moderation services fail
	ModerationAPI.Behavior = BehaviorError
	ModerationBackup.Behavior = BehaviorError

	ctx := context.Background()
	profile := ProfileUpdate{
		UserID: "user123",
		Photo:  []byte("fake image data"),
	}

	result, err := ProfileUpdatePipeline.Process(ctx, profile)

	if err != nil {
		t.Fatalf("Expected success with skipped moderation, got error: %v", err)
	}

	if result.ModerationResult != ModerationSkipped {
		t.Errorf("Expected moderation skipped, got %v", result.ModerationResult)
	}

	// Should have skipped moderation
	found := false
	for _, log := range result.ProcessingLog {
		if strings.Contains(log, "moderation: skipped") {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected moderation to be skipped")
	}
}

func TestProfileUpdate_DatabaseRetry(t *testing.T) {
	EnableFullProduction()
	ResetAllServices()

	// Database fails twice then succeeds
	Database.Behavior = BehaviorTransientError
	Database.FailureCount = 2

	ctx := context.Background()
	profile := ProfileUpdate{
		UserID: "user123",
		Photo:  []byte("fake image data"),
	}

	result, err := ProfileUpdatePipeline.Process(ctx, profile)

	if err != nil {
		t.Fatalf("Expected success after retries, got error: %v", err)
	}

	// Should show retry in log
	found := false
	for _, log := range result.ProcessingLog {
		if strings.Contains(log, "database: saved after 3 attempts") {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected database to succeed after retries")
	}
}

func TestProfileUpdate_DatabaseFailureWithCleanup(t *testing.T) {
	EnableFullProduction()
	ResetAllServices()

	// Update the safe wrapper to use current pipeline
	ProfileUpdateSafe = pipz.NewHandle(
		PipelineProfileUpdateSafe,
		ProfileUpdatePipeline,
		CleanupImages,
	)

	// Database always fails
	Database.Behavior = BehaviorError

	ctx := context.Background()
	profile := ProfileUpdate{
		UserID: "user123",
		Photo:  []byte("fake image data"),
	}

	_, err := ProfileUpdateSafe.Process(ctx, profile)

	if err == nil {
		t.Fatal("Expected error from database failure")
	}

	var pipzErr *pipz.Error[ProfileUpdate]
	if !errors.As(err, &pipzErr) {
		t.Fatalf("Expected pipz.Error, got %T", err)
	}

	// Should have cleanup in log
	found := false
	for _, log := range pipzErr.InputData.ProcessingLog {
		if strings.Contains(log, "cleanup: removed processed images") {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected cleanup to run on failure")
	}

	// Images should be cleaned up
	if pipzErr.InputData.ProcessedImages.Thumbnail != nil {
		t.Error("Expected processed images to be cleaned up")
	}
}

func TestProfileUpdate_PartialCacheFailure(t *testing.T) {
	EnableFullProduction()
	ResetAllServices()

	// CDN fails but others work
	CDN.Behavior = BehaviorError

	ctx := context.Background()
	profile := ProfileUpdate{
		UserID: "user123",
		Photo:  []byte("fake image data"),
	}

	// Should succeed despite CDN failure
	result, err := ProfileUpdatePipeline.Process(ctx, profile)

	if err != nil {
		t.Fatalf("Expected success despite cache failure, got error: %v", err)
	}

	// Should have saved to database
	found := false
	for _, log := range result.ProcessingLog {
		if strings.Contains(log, "database: saved") {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected database save to succeed")
	}
}

func TestProfileUpdate_ImageProcessingTimeout(t *testing.T) {
	ResetPipeline() // Start with basic pipeline for this test
	ResetAllServices()

	// Create a slow processor for testing
	slowProcessor := pipz.Transform(ProcessorProcessImage, func(ctx context.Context, p ProfileUpdate) ProfileUpdate {
		select {
		case <-time.After(200 * time.Millisecond):
			p.ProcessedImages = ProcessedImages{
				Thumbnail: []byte("thumb"),
				Medium:    []byte("medium"),
				Large:     []byte("large"),
			}
			p.ProcessingLog = append(p.ProcessingLog, "image: processed (should not happen)")
		case <-ctx.Done():
			p.ProcessingLog = append(p.ProcessingLog, "image: canceled by timeout")
		}
		return p
	})

	// Create a test pipeline with short timeout
	testPipeline := pipz.NewSequence[ProfileUpdate](
		"test-pipeline",
		ValidateImage,
		pipz.NewTimeout("test-timeout", slowProcessor, 100*time.Millisecond),
		// Skip the rest for this test
	)

	ctx := context.Background()
	profile := ProfileUpdate{
		UserID: "user123",
		Photo:  []byte("fake image data"),
	}

	start := time.Now()
	result, err := testPipeline.Process(ctx, profile)
	duration := time.Since(start)

	// pipz timeout now returns an error when timeout occurs
	if err == nil {
		t.Fatal("Expected timeout error")
	}

	// Verify it's a timeout error
	var pipzErr *pipz.Error[ProfileUpdate]
	if !errors.As(err, &pipzErr) {
		t.Fatalf("Expected pipz.Error, got %T", err)
	}
	if !pipzErr.IsTimeout() {
		t.Errorf("Expected timeout error, got: %v", err)
	}

	// Check that processing was interrupted
	if result.ProcessedImages.Thumbnail != nil {
		t.Error("Expected image processing to be interrupted by timeout")
	}

	// Since the timeout occurred, the slow processor may not have had a chance
	// to update the processing log (it was interrupted)

	// Should complete around timeout duration
	if duration > 150*time.Millisecond {
		t.Errorf("Expected completion around 100ms, took %v", duration)
	}
}

func TestProfileUpdate_ConcurrentPerformance(t *testing.T) {
	EnableFullProduction()
	ResetAllServices()

	ctx := context.Background()
	profile := ProfileUpdate{
		UserID: "user123",
		Photo:  []byte("fake image data"),
	}

	// Time the sequential version
	sequentialStart := time.Now()
	_, _ = InvalidateCDN.Process(ctx, profile)     //nolint:errcheck // demo code
	_, _ = InvalidateRedis.Process(ctx, profile)   //nolint:errcheck // demo code
	_, _ = InvalidateVarnish.Process(ctx, profile) //nolint:errcheck // demo code
	sequentialDuration := time.Since(sequentialStart)

	// Reset services before concurrent test
	profile = ProfileUpdate{
		UserID: "user123",
		Photo:  []byte("fake image data"),
	}

	// Time the concurrent version
	concurrentStart := time.Now()
	_, err := CacheInvalidation.Process(ctx, profile)
	concurrentDuration := time.Since(concurrentStart)

	if err != nil {
		t.Fatalf("Concurrent cache invalidation failed: %v", err)
	}

	// Concurrent should be significantly faster
	if concurrentDuration > 250*time.Millisecond {
		t.Errorf("Concurrent took too long: %v", concurrentDuration)
	}

	if sequentialDuration < 300*time.Millisecond {
		t.Errorf("Sequential was too fast, services might not be working: %v", sequentialDuration)
	}

	// Concurrent should be at least 100ms faster
	if concurrentDuration > sequentialDuration-100*time.Millisecond {
		t.Errorf("Concurrent not faster enough. Sequential: %v, Concurrent: %v",
			sequentialDuration, concurrentDuration)
	}
}

func TestProfileUpdate_ValidationFailure(t *testing.T) {
	ResetPipeline() // Just need basic pipeline for validation
	ResetAllServices()

	tests := []struct {
		name  string
		photo []byte
		error string
	}{
		{
			name:  "no photo",
			photo: []byte{},
			error: "no photo provided",
		},
		{
			name:  "photo too large",
			photo: make([]byte, 11*1024*1024), // 11MB
			error: "photo too large",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			profile := ProfileUpdate{
				UserID: "user123",
				Photo:  tt.photo,
			}

			_, err := ProfileUpdatePipeline.Process(ctx, profile)

			if err == nil {
				t.Fatal("Expected validation error")
			}

			if !strings.Contains(err.Error(), tt.error) {
				t.Errorf("Expected error '%s', got %v", tt.error, err)
			}
		})
	}
}

func TestProfileUpdate_RuntimeModification(t *testing.T) {
	// Test the feature flag approach
	ResetPipeline() // Start with MVP
	ResetAllServices()

	ctx := context.Background()
	profile := ProfileUpdate{
		UserID: "user123",
		Photo:  []byte("fake image data"),
	}

	// Test base pipeline
	names := ProfileUpdatePipeline.Names()
	if len(names) != 2 {
		t.Errorf("Expected 2 processors in base pipeline, got %d: %v", len(names), names)
	}

	// Enable image processing
	EnableImageProcessing()
	names = ProfileUpdatePipeline.Names()
	if len(names) != 3 {
		t.Errorf("Expected 3 processors after enabling image processing, got %d: %v", len(names), names)
	}

	// Enable moderation
	EnableContentModeration()
	names = ProfileUpdatePipeline.Names()
	if len(names) != 4 {
		t.Errorf("Expected 4 processors after enabling moderation, got %d: %v", len(names), names)
	}

	// Test that pipeline still works
	result, err := ProfileUpdatePipeline.Process(ctx, profile)
	if err != nil {
		t.Fatalf("Expected success, got error: %v", err)
	}

	if result.ProcessedImages.Thumbnail == nil {
		t.Error("Expected images to be processed")
	}

	if result.ModerationResult != ModerationApproved {
		t.Errorf("Expected moderation approved, got %v", result.ModerationResult)
	}
}
