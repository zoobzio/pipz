package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/zoobzio/pipz"
)

// ============================================================================.
// SPRINT 1: MVP - Just Save Profile Photos (Week 1).
// ============================================================================.
// "We need to launch tomorrow! Just validate the photo and save it."
// Requirements:.
// - Basic validation (size, presence).
// - Save to database.
// - Ship it!.

var (
	//nolint:godot // Variable comment with period issue
	// ValidateImage ensures the photo meets basic requirements.
	// Added: Sprint 1 - Core requirement
	ValidateImage = pipz.Apply(ProcessorValidateImage, func(_ context.Context, p ProfileUpdate) (ProfileUpdate, error) {
		if len(p.Photo) == 0 {
			return p, errors.New("no photo provided")
		}
		if len(p.Photo) > 10*1024*1024 { // 10MB
			return p, errors.New("photo too large")
		}
		p.ProcessingLog = append(p.ProcessingLog, "validation: passed")
		return p, nil
	})

	//nolint:godot // Variable comment with period issue
	// UpdateDatabase saves the profile update to the database.
	// Added: Sprint 1 - Core requirement
	UpdateDatabase = pipz.Apply(ProcessorUpdateDatabase, func(ctx context.Context, p ProfileUpdate) (ProfileUpdate, error) {
		return Database.Save(ctx, p)
	})
)

// ============================================================================.
// SPRINT 3: Mobile Optimization (Week 3).
// ============================================================================.
// "Mobile team is screaming! Users are downloading 10MB photos and burning.
// through their data plans. We need thumbnails NOW!"
//
// THE MATH: 10MB photos × 5,000 mobile uploads/day × $0.10/MB = $5,000/day
// After thumbnails: 100KB × 5,000 × $0.10/MB = $50/day (99% reduction!)
//
// New requirements:.
// - Generate multiple image sizes.
// - Add timeout protection (some images take forever).

var (
	// ProcessImage generates thumbnails and optimized versions.
	// Added: Sprint 3 - Mobile optimization.
	ProcessImage = pipz.Transform(ProcessorProcessImage, func(ctx context.Context, p ProfileUpdate) ProfileUpdate {
		// Simulate image processing.
		select {
		case <-time.After(100 * time.Millisecond):
			p.ProcessedImages = ProcessedImages{
				Thumbnail: []byte("thumb_" + p.UserID),
				Medium:    []byte("medium_" + p.UserID),
				Large:     []byte("large_" + p.UserID),
			}
			p.ProcessingLog = append(p.ProcessingLog, "image: processed")
		case <-ctx.Done():
			p.ProcessingLog = append(p.ProcessingLog, "image: canceled")
		}
		return p
	})

	// ImageProcessingTimeout prevents slow image processing from blocking.
	// Added: Sprint 3 - Learned the hard way when a corrupt image blocked for 30 minutes.
	ImageProcessingTimeout = pipz.NewTimeout(
		ConnectorImageProcessing,
		ProcessImage,
		5*time.Second,
	)
)

// ============================================================================.
// SPRINT 5: Legal Compliance Crisis (Week 5).
// ============================================================================.
// 3 AM SATURDAY: "EMERGENCY! Legal just called. Someone uploaded inappropriate
// content and it's trending on the homepage. The CEO is threatening to shut
// down the entire platform if we don't fix this NOW!".
//
// Dev at 3:15 AM: "Why didn't we add moderation from day one?! 😱".
// Dev at 3:30 AM: "The moderation API they want has 90% uptime... on a good day"
// Dev at 4:00 AM: "Fine, we'll add fallbacks to fallbacks to fallbacks..."
//
// Crisis requirements:.
// - Check all images for inappropriate content.
// - Handle API failures gracefully.
// - If all else fails, flag for manual review.

var (
	// ModerateWithAPI checks content with the primary moderation service.
	// Added: Sprint 5 - Legal requirement.
	ModerateWithAPI = pipz.Apply(ProcessorModerateContent, func(ctx context.Context, p ProfileUpdate) (ProfileUpdate, error) {
		return ModerationAPI.Moderate(ctx, p)
	})

	// ModerateWithBackup uses the backup moderation service.
	// Added: Sprint 5 - Because the primary API went down during the CEO's profile update.
	ModerateWithBackup = pipz.Apply(ProcessorModerateBackup, func(ctx context.Context, p ProfileUpdate) (ProfileUpdate, error) {
		return ModerationBackup.Moderate(ctx, p)
	})

	// SkipModeration flags content for manual review when both APIs fail.
	// Added: Sprint 5 - Better to be slow than sued.
	SkipModeration = pipz.Transform(ProcessorSkipModeration, func(_ context.Context, p ProfileUpdate) ProfileUpdate {
		p.ModerationResult = ModerationSkipped
		p.ProcessingLog = append(p.ProcessingLog, "moderation: skipped")
		return p
	})

	// ModerationWithBackup tries primary then backup service.
	// Added: Sprint 5 - First level of fallback.
	ModerationWithBackup = pipz.NewFallback[ProfileUpdate](
		"moderation-with-backup",
		ModerateWithAPI,
		ModerateWithBackup,
	)

	// ModerationFallback provides complete moderation resilience.
	// Added: Sprint 5 - Complete fallback chain.
	ModerationFallback = pipz.NewFallback[ProfileUpdate](
		ConnectorModeration,
		ModerationWithBackup,
		SkipModeration,
	)
)

// ============================================================================.
// SPRINT 7: Scale Issues (Week 7).
// ============================================================================.
// "We're getting 1000 uploads/minute! Database is showing deadlocks during.
// peak hours. Users are losing their uploads!"
// Scaling requirements:.
// - Retry transient failures.
// - Don't lose user data.

var (
	// DatabaseWithRetry handles transient database failures.
	// Added: Sprint 7 - After losing 50 uploads in one hour.
	DatabaseWithRetry = pipz.NewRetry(
		ConnectorDatabase,
		UpdateDatabase,
		3,
	)
)

// ============================================================================.
// SPRINT 9: Cache Consistency (Week 9).
// ============================================================================.
// "Users are complaining they see old photos even after updating. We have
// CDN cache, Redis cache, and Varnish cache. They all need to be cleared!"
// Cache requirements:.
// - Invalidate all cache layers.
// - Don't let cache failures block updates.
// - Run invalidations in parallel (they're slow!).

var (
	// InvalidateCDN clears the CDN cache for the user.
	// Added: Sprint 9 - First cache layer.
	InvalidateCDN = pipz.Effect(ProcessorInvalidateCDN, func(ctx context.Context, p ProfileUpdate) error {
		return CDN.Invalidate(ctx, p.UserID)
	})

	// InvalidateRedis clears the Redis cache.
	// Added: Sprint 9 - Session cache.
	InvalidateRedis = pipz.Effect(ProcessorInvalidateRedis, func(ctx context.Context, p ProfileUpdate) error {
		return Redis.Invalidate(ctx, p.UserID)
	})

	// InvalidateVarnish clears the Varnish cache.
	// Added: Sprint 9 - API cache.
	InvalidateVarnish = pipz.Effect(ProcessorInvalidateVarnish, func(ctx context.Context, p ProfileUpdate) error {
		return Varnish.Invalidate(ctx, p.UserID)
	})

	// CacheInvalidation runs all cache clears in parallel.
	// Added: Sprint 9 - Sequential was taking 350ms!.
	CacheInvalidation = pipz.NewConcurrent[ProfileUpdate](
		ConnectorCacheInvalidation,
		nil, // No reducer needed - fire and forget
		InvalidateCDN,     // ~200ms
		InvalidateRedis,   // ~50ms
		InvalidateVarnish, // ~100ms
	)

	// LogCacheErrors logs cache failures without blocking.
	// Added: Sprint 9 - CDN API has 5% error rate.
	LogCacheErrors = pipz.Effect(ProcessorLogCacheErrors, func(_ context.Context, pipeErr *pipz.Error[ProfileUpdate]) error {
		// In real app, this would log to your logging system.
		fmt.Printf("Cache invalidation errors for user %s: %v\n", pipeErr.InputData.UserID, pipeErr.Err)
		return nil
	})

	// CacheInvalidationSafe ensures cache errors don't fail the update.
	// Added: Sprint 9 - Better stale cache than failed upload.
	CacheInvalidationSafe = pipz.NewHandle(
		ConnectorCacheInvalidation,
		CacheInvalidation,
		LogCacheErrors,
	)
)

// ============================================================================.
// SPRINT 11: User Engagement (Week 11).
// ============================================================================.
// "Product wants email confirmations, SMS for premium users, and webhooks.
// for our enterprise integration partners. But don't slow down the upload!"
// Engagement requirements:.
// - Multiple notification channels.
// - Run in parallel.
// - Don't block on slow services.

var (
	// SendEmail sends email confirmation.
	// Added: Sprint 11 - User engagement.
	SendEmail = pipz.Effect(ProcessorSendEmail, func(ctx context.Context, p ProfileUpdate) error {
		return EmailService.Send(ctx, p.UserID)
	})

	// SendSMS sends SMS to premium users.
	// Added: Sprint 11 - Premium feature.
	SendSMS = pipz.Effect(ProcessorSendSMS, func(ctx context.Context, p ProfileUpdate) error {
		return SMSService.Send(ctx, p.UserID)
	})

	// SendSMSToPremium filters SMS sending to only premium users.
	// Added: Sprint 11 - Using Filter for conditional notifications.
	SendSMSToPremium = pipz.NewFilter(ProcessorSendSMSPremium,
		func(_ context.Context, p ProfileUpdate) bool {
			return p.IsPremium
		},
		SendSMS,
	)

	// PostWebhook notifies enterprise integrations.
	// Added: Sprint 11 - Enterprise feature.
	PostWebhook = pipz.Effect(ProcessorPostWebhook, func(ctx context.Context, p ProfileUpdate) error {
		return WebhookService.Send(ctx, p.UserID)
	})

	// NotificationsConcurrent sends all notifications in parallel.
	// Added: Sprint 11 - Don't wait 2.3s sequentially!
	NotificationsConcurrent = pipz.NewConcurrent[ProfileUpdate](
		ConnectorNotifications,
		nil, // No reducer needed - fire and forget
		SendEmail,   // ~1s
		SendSMS,     // ~800ms
		PostWebhook, // ~500ms
	)

	// NotificationsTimeout prevents slow notifications from blocking.
	// Added: Sprint 11 - Email service sometimes takes 10s+.
	NotificationsTimeout = pipz.NewTimeout(
		ConnectorNotifications,
		NotificationsConcurrent,
		2*time.Second,
	)
)

// ============================================================================.
// SPRINT 15: Error Recovery (Week 15).
// ============================================================================.
// "We had an outage where images were processed but database saves failed.
// Users lost their uploads and processed images were orphaned in S3!".
// Recovery requirements:.
// - Clean up on failure.
// - Provide transaction-like behavior.

var (
	// CleanupImages removes processed images on pipeline failure.
	// Added: Sprint 15 - Prevent orphaned resources.
	CleanupImages = pipz.Apply(ProcessorCleanupImages, func(_ context.Context, pipeErr *pipz.Error[ProfileUpdate]) (*pipz.Error[ProfileUpdate], error) {
		// Cleanup on failure.
		if pipeErr.InputData.ProcessedImages.Thumbnail != nil {
			pipeErr.InputData.ProcessingLog = append(pipeErr.InputData.ProcessingLog, "cleanup: removed processed images")
			// In real app, would delete from storage.
			pipeErr.InputData.ProcessedImages = ProcessedImages{}
		}
		return pipeErr, nil
	})
)

// ============================================================================.
// PIPELINE EVOLUTION.
// ============================================================================.
// The pipeline started as a simple 2-step process and evolved through.
// feature flags to handle production complexity without rewrites.

// ProfileUpdatePipeline is our main pipeline that evolves through feature flags.
// Sprint 1: Started with just [ValidateImage → UpdateDatabase].
// Sprint 3: Added image processing.
// Sprint 5: Added content moderation.
// Sprint 7: Added database resilience.
// Sprint 9: Added cache invalidation.
// Sprint 11: Added notifications.
// Sprint 15: Added error recovery.
var ProfileUpdatePipeline = pipz.NewSequence[ProfileUpdate](
	PipelineProfileUpdate,
	ValidateImage,  // Sprint 1: Original requirement
	UpdateDatabase, // Sprint 1: Original requirement
)

// ResetPipeline resets the pipeline to its original MVP state.
// Used for demonstrations to show the progression from simple to complex.
func ResetPipeline() {
	ProfileUpdatePipeline.Clear()
	ProfileUpdatePipeline.Register(
		ValidateImage,
		UpdateDatabase,
	)
}

// EnableImageProcessing adds thumbnail generation to the pipeline.
// "Sprint 3: Mobile team is complaining about 10MB photos killing their data plans.
// We need thumbnails ASAP. Good thing we used pipz - just slot in the processor
// between validation and database. Ship it behind a flag to test with 10% first."
// Feature flag: ENABLE_IMAGE_PROCESSING.
func EnableImageProcessing() {
	ProfileUpdatePipeline.After( //nolint:errcheck // demo code
		ProcessorValidateImage,
		ImageProcessingTimeout,
	)
}

// EnableContentModeration adds content moderation with fallback.
// "Sprint 5: Legal just called at 3am. Someone uploaded inappropriate content
// and it's on the trending page. We need moderation NOW. Wait, the API they
// want us to use has 90% uptime? Better add a fallback to a backup service,.
// and if both fail, flag for manual review."
// Feature flag: ENABLE_CONTENT_MODERATION.
func EnableContentModeration() {
	// Insert moderation before database save.
	names := ProfileUpdatePipeline.Names()
	for _, name := range names {
		if name == ProcessorUpdateDatabase || name == ConnectorDatabase {
			ProfileUpdatePipeline.Before(name, ModerationFallback) //nolint:errcheck // demo code
			return
		}
	}
}

// EnableDatabaseResilience adds retry logic to database operations.
// "Sprint 7: We're getting 1000 uploads/minute now. Database is showing
// deadlocks during peak hours. Let's add retries with exponential backoff.
// Can't have users losing their uploads because of a transient DB issue."
// Feature flag: ENABLE_DATABASE_RESILIENCE.
func EnableDatabaseResilience() {
	ProfileUpdatePipeline.Replace( //nolint:errcheck // demo code
		ProcessorUpdateDatabase,
		DatabaseWithRetry,
	)
}

// EnableCacheInvalidation adds distributed cache clearing.
// "Sprint 9: Users are complaining they see old photos even after updating.
// Turns out we have CDN cache, Redis cache, and app-level cache. Need to
// invalidate all of them. These can run in parallel though - pipz Concurrent
// should speed this up."
// Feature flag: ENABLE_CACHE_INVALIDATION.
func EnableCacheInvalidation() {
	// Add after database save.
	ProfileUpdatePipeline.After( //nolint:errcheck // demo code
		ConnectorDatabase, // Works with both UpdateDatabase and DatabaseWithRetry
		CacheInvalidationSafe,
	)
}

// EnableNotifications adds user notifications.
// "Sprint 11: Product wants email confirmations, SMS alerts, and webhook.
// notifications for enterprise customers. These shouldn't block the main
// flow though - fire and forget with a timeout."
// Feature flag: ENABLE_NOTIFICATIONS.
func EnableNotifications() {
	// Add at the end.
	ProfileUpdatePipeline.Register(NotificationsTimeout)
}

// EnableFullProduction enables all production features.
// "Sprint 13: Going viral! Enable all the resilience patterns.
// Every feature we added saved us during the traffic spike:.
// - Image processing: Prevented mobile app crashes.
// - Moderation: Kept us out of the news.
// - Retries: Handled the database strain.
// - Cache invalidation: Kept data consistent.
// - Notifications: Kept users informed.
// The original 2-step pipeline is now production-grade!".
func EnableFullProduction() {
	// Reset to base.
	ProfileUpdatePipeline = pipz.NewSequence[ProfileUpdate](
		PipelineProfileUpdate,
		ValidateImage,
		UpdateDatabase,
	)

	// Apply all features in order.
	EnableImageProcessing()
	EnableContentModeration()
	EnableDatabaseResilience()
	EnableCacheInvalidation()
	EnableNotifications()
}

// ProfileUpdateSafe wraps the pipeline with error compensation.
// Added: Sprint 15 - Transaction-like behavior.
var ProfileUpdateSafe = pipz.NewHandle(
	PipelineProfileUpdateSafe,
	ProfileUpdatePipeline,
	CleanupImages,
)
