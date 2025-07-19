package moderation

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/zoobzio/pipz"
)

func TestAnalyzeText(t *testing.T) {
	tests := []struct {
		name          string
		text          string
		minScore      float64
		maxScore      float64
		expectedFlags []string
	}{
		{
			name:     "Clean text",
			text:     "This is a perfectly normal comment about the weather",
			minScore: 0.0,
			maxScore: 0.1,
		},
		{
			name:          "Profanity",
			text:          "This badword1 text contains profanity",
			minScore:      0.3,
			maxScore:      0.4,
			expectedFlags: []string{"profanity"},
		},
		{
			name:          "Spam patterns",
			text:          "CLICK HERE to WIN the LOTTERY! Limited time offer!!!",
			minScore:      0.4,
			maxScore:      0.7,
			expectedFlags: []string{"spam"},
		},
		{
			name:          "Toxic language",
			text:          "I hate you, you stupid person",
			minScore:      0.7,
			maxScore:      0.9,
			expectedFlags: []string{"toxic"},
		},
		{
			name:          "All caps shouting",
			text:          "THIS IS ALL IN CAPITAL LETTERS",
			minScore:      0.0,
			maxScore:      0.4,
			expectedFlags: []string{"spam"}, // Changed from formatting to spam
		},
		{
			name:          "Multiple issues",
			text:          "You STUPID badword1! CLICK HERE NOW!!!",
			minScore:      0.8,
			maxScore:      1.0,
			expectedFlags: []string{"profanity", "spam", "toxic"},
		},
	}

	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content := Content{
				ID:   "test",
				Text: tt.text,
			}

			result, err := AnalyzeText(ctx, content)
			if err != nil {
				t.Fatalf("AnalyzeText failed: %v", err)
			}

			if result.TextScore < tt.minScore || result.TextScore > tt.maxScore {
				t.Errorf("Expected score between %.2f and %.2f, got %.2f",
					tt.minScore, tt.maxScore, result.TextScore)
			}

			// Check flags
			flagTypes := make(map[string]bool)
			for _, flag := range result.Flags {
				flagTypes[flag.Type] = true
			}

			for _, expected := range tt.expectedFlags {
				if !flagTypes[expected] {
					t.Errorf("Expected flag %s not found", expected)
				}
			}
		})
	}
}

func TestMockImageAnalyzer(t *testing.T) {
	analyzer := NewMockImageAnalyzer()
	ctx := context.Background()

	tests := []struct {
		name      string
		imageData []byte
		imageURL  string
	}{
		{
			name:      "Image data",
			imageData: []byte("test image data"),
		},
		{
			name:     "Image URL",
			imageURL: "https://example.com/image.jpg",
		},
		{
			name: "No image",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content := Content{
				ID:        "test",
				ImageData: tt.imageData,
				ImageURL:  tt.imageURL,
			}

			result, err := analyzer.Analyze(ctx, content)
			if err != nil {
				t.Fatalf("Analyze failed: %v", err)
			}

			if tt.imageData == nil && tt.imageURL == "" {
				if result.ImageScore != 0 {
					t.Error("Expected zero score for no image")
				}
			} else {
				if result.ImageScore < 0 || result.ImageScore > 1 {
					t.Errorf("Image score out of range: %.2f", result.ImageScore)
				}
			}
		})
	}

	// Test caching
	t.Run("Caching", func(t *testing.T) {
		content := Content{
			ID:        "cache-test",
			ImageData: []byte("cached image"),
		}

		// First call
		start := time.Now()
		result1, err := analyzer.Analyze(ctx, content)
		if err != nil {
			t.Fatalf("First analyze failed: %v", err)
		}
		firstDuration := time.Since(start)

		// Second call (should be cached)
		start = time.Now()
		result2, err := analyzer.Analyze(ctx, content)
		if err != nil {
			t.Fatalf("Second analyze failed: %v", err)
		}
		cachedDuration := time.Since(start)

		if result1.ImageScore != result2.ImageScore {
			t.Error("Cached result differs from original")
		}

		if cachedDuration > firstDuration/2 {
			t.Error("Cached call not significantly faster")
		}
	})
}

func TestCombineScores(t *testing.T) {
	tests := []struct {
		name          string
		contentType   ContentType
		textScore     float64
		imageScore    float64
		expectedScore float64
	}{
		{
			name:          "Text only",
			contentType:   TypeText,
			textScore:     0.5,
			imageScore:    0.0,
			expectedScore: 0.5,
		},
		{
			name:          "Image only",
			contentType:   TypeImage,
			textScore:     0.0,
			imageScore:    0.7,
			expectedScore: 0.7,
		},
		{
			name:          "Mixed - high text",
			contentType:   TypeMixed,
			textScore:     0.8,
			imageScore:    0.2,
			expectedScore: 0.71, // 0.7*0.8 + 0.3*((0.8+0.2)/2)
		},
		{
			name:          "Mixed - high image",
			contentType:   TypeMixed,
			textScore:     0.3,
			imageScore:    0.9,
			expectedScore: 0.81, // 0.7*0.9 + 0.3*((0.3+0.9)/2)
		},
	}

	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content := Content{
				Type:       tt.contentType,
				TextScore:  tt.textScore,
				ImageScore: tt.imageScore,
			}

			result, err := CombineScores(ctx, content)
			if err != nil {
				t.Fatalf("CombineScores failed: %v", err)
			}

			if result.OverallScore != tt.expectedScore {
				t.Errorf("Expected score %.2f, got %.2f", tt.expectedScore, result.OverallScore)
			}

			if result.AnalyzedAt.IsZero() {
				t.Error("AnalyzedAt not set")
			}
		})
	}
}

func TestMakeDecision(t *testing.T) {
	thresholds := Thresholds{
		AutoApprove: 0.2,
		Review:      0.4,
		Quarantine:  0.7,
		AutoReject:  0.9,
	}

	decider := MakeDecision(thresholds)

	tests := []struct {
		name             string
		score            float64
		expectedDecision Decision
	}{
		{
			name:             "Auto approve",
			score:            0.1,
			expectedDecision: DecisionApprove,
		},
		{
			name:             "Review",
			score:            0.5,
			expectedDecision: DecisionReview,
		},
		{
			name:             "Quarantine",
			score:            0.8,
			expectedDecision: DecisionQuarantine,
		},
		{
			name:             "Auto reject",
			score:            0.95,
			expectedDecision: DecisionReject,
		},
	}

	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content := Content{
				ID:           "test",
				OverallScore: tt.score,
			}

			result, err := decider(ctx, content)
			if err != nil {
				t.Fatalf("MakeDecision failed: %v", err)
			}

			if result.Decision != tt.expectedDecision {
				t.Errorf("Expected decision %s, got %s", tt.expectedDecision, result.Decision)
			}

			if result.Reason == "" {
				t.Error("Decision reason not set")
			}
		})
	}
}

func TestReviewQueue(t *testing.T) {
	queue := NewReviewQueue()

	// Add content
	content1 := Content{ID: "1", Decision: DecisionReview}
	content2 := Content{ID: "2", Decision: DecisionQuarantine}

	queue.Add(content1)
	queue.Add(content2)

	// Get content
	retrieved, ok := queue.Get("1")
	if !ok {
		t.Error("Failed to retrieve content from queue")
	}
	if retrieved.ID != "1" {
		t.Error("Retrieved wrong content")
	}

	// List all
	items := queue.List()
	if len(items) != 2 {
		t.Errorf("Expected 2 items in queue, got %d", len(items))
	}

	// Review content
	err := queue.Review("1", DecisionApprove, "human_reviewer")
	if err != nil {
		t.Errorf("Review failed: %v", err)
	}

	// Should be removed from queue
	_, ok = queue.Get("1")
	if ok {
		t.Error("Content still in queue after review")
	}

	// Review non-existent
	err = queue.Review("999", DecisionApprove, "reviewer")
	if err == nil {
		t.Error("Expected error reviewing non-existent content")
	}
}

func TestModerationPipeline(t *testing.T) {
	analyzer := NewMockImageAnalyzer()
	pipeline := CreateModerationPipeline(analyzer, DefaultThresholds)
	ctx := context.Background()

	tests := []struct {
		name             string
		content          Content
		expectedDecision Decision
		expectError      bool
	}{
		{
			name: "Clean text content",
			content: Content{
				ID:     "clean",
				Text:   "This is a nice comment about puppies",
				Author: "user1",
			},
			expectedDecision: DecisionApprove,
		},
		{
			name: "Spam content",
			content: Content{
				ID:     "spam",
				Text:   "CLICK HERE!! WIN LOTTERY!! BUY VIAGRA!!",
				Author: "spammer",
			},
			expectedDecision: DecisionReview,
		},
		{
			name: "Toxic content",
			content: Content{
				ID:     "toxic",
				Text:   "I hate you stupid badword1 badword2",
				Author: "troll",
			},
			expectedDecision: DecisionReject,
		},
		{
			name: "Mixed content",
			content: Content{
				ID:        "mixed",
				Text:      "Check out this image",
				ImageData: []byte("image data"),
				Author:    "user2",
			},
			expectedDecision: DecisionApprove, // Depends on mock image score
		},
		{
			name: "Empty content",
			content: Content{
				ID:     "empty",
				Author: "user3",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := pipeline.Process(ctx, tt.content)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Fatalf("Pipeline failed: %v", err)
			}

			if result.Decision != tt.expectedDecision {
				t.Errorf("Expected decision %s, got %s (score: %.2f)",
					tt.expectedDecision, result.Decision, result.OverallScore)
			}

			if result.AnalyzedAt.IsZero() {
				t.Error("AnalyzedAt not set")
			}
		})
	}
}

func TestBatchModerationPipeline(t *testing.T) {
	analyzer := NewMockImageAnalyzer()
	pipeline := CreateBatchModerationPipeline(analyzer, DefaultThresholds)
	ctx := context.Background()

	batch := []Content{
		{ID: "1", Text: "Normal comment", Author: "user1"},
		{ID: "2", Text: "SPAM SPAM CLICK HERE", Author: "spammer"},
		{ID: "3", Text: "I love this product", Author: "user2"},
	}

	results, err := pipeline.Process(ctx, batch)
	if err != nil {
		t.Fatalf("Batch pipeline failed: %v", err)
	}

	if len(results) != len(batch) {
		t.Errorf("Expected %d results, got %d", len(batch), len(results))
	}

	// Verify each result
	for i, result := range results {
		if result.ID != batch[i].ID {
			t.Errorf("Result order mismatch at index %d", i)
		}

		if result.Decision == "" {
			t.Errorf("No decision for content %s", result.ID)
		}
	}
}

func TestReviewPipeline(t *testing.T) {
	queue := NewReviewQueue()
	storages := map[Decision]Storage{
		DecisionApprove:    NewMemoryStorage(),
		DecisionReject:     NewMemoryStorage(),
		DecisionQuarantine: NewMemoryStorage(),
	}

	pipeline := CreateReviewPipeline(queue, storages)
	ctx := context.Background()

	tests := []struct {
		name      string
		content   Content
		inQueue   bool
		inStorage bool
	}{
		{
			name: "Approved content",
			content: Content{
				ID:       "approved",
				Decision: DecisionApprove,
			},
			inQueue:   false,
			inStorage: true,
		},
		{
			name: "Review content",
			content: Content{
				ID:       "review",
				Decision: DecisionReview,
			},
			inQueue:   true,
			inStorage: false,
		},
		{
			name: "Quarantined content",
			content: Content{
				ID:       "quarantine",
				Decision: DecisionQuarantine,
			},
			inQueue:   true,
			inStorage: true,
		},
		{
			name: "Rejected content",
			content: Content{
				ID:       "rejected",
				Decision: DecisionReject,
			},
			inQueue:   false,
			inStorage: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := pipeline.Process(ctx, tt.content)
			if err != nil {
				t.Fatalf("Review pipeline failed: %v", err)
			}

			// Check queue
			_, inQueue := queue.Get(tt.content.ID)
			if inQueue != tt.inQueue {
				t.Errorf("Expected inQueue=%v, got %v", tt.inQueue, inQueue)
			}

			// Check storage
			if tt.inStorage {
				storage := storages[tt.content.Decision].(*MemoryStorage)
				_, stored := storage.Get(tt.content.ID)
				if !stored {
					t.Error("Content not stored")
				}
			}
		})
	}
}

func TestFullModerationWorkflow(t *testing.T) {
	// Setup
	analyzer := NewMockImageAnalyzer()
	queue := NewReviewQueue()
	storages := map[Decision]Storage{
		DecisionApprove:    NewMemoryStorage(),
		DecisionReject:     NewMemoryStorage(),
		DecisionQuarantine: NewMemoryStorage(),
	}

	// Create pipelines
	moderationPipeline := CreateModerationPipeline(analyzer, DefaultThresholds)
	reviewPipeline := CreateReviewPipeline(queue, storages)

	// Combine pipelines
	fullPipeline := pipz.Sequential(
		moderationPipeline,
		reviewPipeline,
	)

	ctx := context.Background()

	// Process content
	content := Content{
		ID:     "full-test",
		Text:   "This is a test with some spam: CLICK HERE",
		Author: "testuser",
	}

	result, err := fullPipeline.Process(ctx, content)
	if err != nil {
		t.Fatalf("Full pipeline failed: %v", err)
	}

	// Should be in review
	if result.Decision != DecisionReview {
		t.Errorf("Expected review decision, got %s", result.Decision)
	}

	// Should be in queue
	_, inQueue := queue.Get(result.ID)
	if !inQueue {
		t.Error("Content not in review queue")
	}

	// Simulate human review
	err = queue.Review(result.ID, DecisionApprove, "moderator1")
	if err != nil {
		t.Errorf("Human review failed: %v", err)
	}
}

func TestThresholdBoundaries(t *testing.T) {
	thresholds := Thresholds{
		AutoApprove: 0.2,
		Review:      0.4,
		Quarantine:  0.7,
		AutoReject:  0.9,
	}

	decider := MakeDecision(thresholds)
	ctx := context.Background()

	// Test exact boundaries
	boundaries := []struct {
		score    float64
		decision Decision
	}{
		{0.19, DecisionApprove},   // Just below auto-approve boundary
		{0.2, DecisionApprove},    // At auto-approve boundary (still approve)
		{0.4, DecisionReview},     // At review boundary
		{0.7, DecisionQuarantine}, // At quarantine boundary
		{0.9, DecisionReject},     // At auto-reject boundary
	}

	for _, b := range boundaries {
		t.Run(fmt.Sprintf("Score_%.2f", b.score), func(t *testing.T) {
			content := Content{OverallScore: b.score}
			result, _ := decider(ctx, content)
			if result.Decision != b.decision {
				t.Errorf("Score %.2f: expected %s, got %s",
					b.score, b.decision, result.Decision)
			}
		})
	}
}

func TestStats(t *testing.T) {
	// Reset stats
	globalStats = &ModerationStats{
		ByDecision: make(map[Decision]int64),
		ByType:     make(map[ContentType]int64),
	}

	// Update with various content
	contents := []Content{
		{Type: TypeText, Decision: DecisionApprove, OverallScore: 0.1},
		{Type: TypeText, Decision: DecisionReview, OverallScore: 0.5},
		{Type: TypeImage, Decision: DecisionReject, OverallScore: 0.9},
		{Type: TypeMixed, Decision: DecisionApprove, OverallScore: 0.2},
	}

	for _, c := range contents {
		UpdateStats(c)
	}

	stats := GetStats()

	if stats.Total != 4 {
		t.Errorf("Expected 4 total, got %d", stats.Total)
	}

	if stats.ByDecision[DecisionApprove] != 2 {
		t.Errorf("Expected 2 approved, got %d", stats.ByDecision[DecisionApprove])
	}

	if stats.ByType[TypeText] != 2 {
		t.Errorf("Expected 2 text, got %d", stats.ByType[TypeText])
	}

	expectedAvg := (0.1 + 0.5 + 0.9 + 0.2) / 4
	if stats.AverageScore != expectedAvg {
		t.Errorf("Expected average %.2f, got %.2f", expectedAvg, stats.AverageScore)
	}
}

func TestProfanityDetection(t *testing.T) {
	// Test that profanity patterns work correctly
	tests := []struct {
		text       string
		shouldFlag bool
	}{
		{"This is clean text", false},
		{"Contains badword1", true},
		{"BADWORD2 in caps", true},
		{"bad word1 with space", false}, // Should not match with space
		{"embeddedbadword3text", true},  // Should match embedded
	}

	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			content := Content{Text: tt.text}
			result, _ := AnalyzeText(ctx, content)

			hasProfanity := false
			for _, flag := range result.Flags {
				if flag.Type == "profanity" {
					hasProfanity = true
					break
				}
			}

			if hasProfanity != tt.shouldFlag {
				t.Errorf("Text %q: expected profanity=%v, got %v",
					tt.text, tt.shouldFlag, hasProfanity)
			}
		})
	}
}

func TestConcurrentProcessing(t *testing.T) {
	analyzer := NewMockImageAnalyzer()
	pipeline := CreateModerationPipeline(analyzer, DefaultThresholds)
	ctx := context.Background()

	// Process multiple items concurrently
	contents := make([]Content, 10)
	for i := range contents {
		contents[i] = Content{
			ID:   fmt.Sprintf("concurrent-%d", i),
			Text: strings.Repeat("test ", i+1), // Varying lengths
		}
	}

	results := make([]Content, len(contents))
	errors := make([]error, len(contents))

	// Process all concurrently
	done := make(chan bool, len(contents))
	for i := range contents {
		go func(idx int) {
			results[idx], errors[idx] = pipeline.Process(ctx, contents[idx])
			done <- true
		}(i)
	}

	// Wait for all
	for range contents {
		<-done
	}

	// Check results
	for i, err := range errors {
		if err != nil {
			t.Errorf("Content %d failed: %v", i, err)
		}
		if results[i].ID != contents[i].ID {
			t.Errorf("Result mismatch at index %d", i)
		}
	}
}
