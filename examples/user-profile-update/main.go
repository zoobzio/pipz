package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/zoobzio/pipz"
)

func main() {
	fmt.Println("🖼️  pipz User Profile Update Example")
	fmt.Println("=====================================")
	fmt.Println()
	fmt.Println("Follow the evolution of our profile update pipeline through feature flags...")
	fmt.Println()

	// Sprint 1: MVP
	fmt.Println("🚀 SPRINT 1: MVP Launch")
	fmt.Println("--------------------")
	fmt.Println("Requirements: Just save profile photos")
	ResetPipeline() // Start with MVP
	ResetAllServices()
	fmt.Printf("Pipeline: %v\n", ProfileUpdatePipeline.Names())
	runDemoScenario("basic upload")
	fmt.Println()

	// Sprint 3: Image Processing
	fmt.Println("📱 SPRINT 3: Mobile Optimization")
	fmt.Println("------------------------------")
	fmt.Println("Problem: Mobile users complaining about 10MB photos using all their data")
	fmt.Println("Solution: Add image processing with thumbnails")
	EnableImageProcessing()
	fmt.Printf("Pipeline: %v\n", ProfileUpdatePipeline.Names())
	runDemoScenario("process images")
	fmt.Println()

	// Sprint 5: Content Moderation
	fmt.Println("⚖️  SPRINT 5: Legal Compliance")
	fmt.Println("---------------------------")
	fmt.Println("Crisis: Inappropriate content uploaded, legal team panicking!")
	fmt.Println("Solution: Add content moderation with fallback")
	EnableContentModeration()
	fmt.Printf("Pipeline: %v\n", ProfileUpdatePipeline.Names())

	// Show fallback in action
	fmt.Println("\nDemonstrating moderation fallback:")
	ModerationAPI.Behavior = BehaviorError
	runDemoScenario("moderation fallback")
	fmt.Println()

	// Sprint 7: Database Resilience
	fmt.Println("💪 SPRINT 7: Scale Issues")
	fmt.Println("-----------------------")
	fmt.Println("Problem: Database deadlocks during peak traffic")
	fmt.Println("Solution: Add retry logic")
	EnableDatabaseResilience()
	fmt.Printf("Pipeline: %v\n", ProfileUpdatePipeline.Names())

	// Show retry in action
	Database.Behavior = BehaviorTransientError
	Database.FailureCount = 2
	runDemoScenario("database retry")
	fmt.Println()

	// Sprint 9: Cache Invalidation
	fmt.Println("🔄 SPRINT 9: Consistency Issues")
	fmt.Println("-----------------------------")
	fmt.Println("Problem: Users see old photos due to caching layers")
	fmt.Println("Solution: Invalidate all caches (in parallel!)")
	EnableCacheInvalidation()
	fmt.Printf("Pipeline: %v\n", ProfileUpdatePipeline.Names())
	demonstrateConcurrentPerformance()
	fmt.Println()

	// Sprint 11: Notifications
	fmt.Println("📬 SPRINT 11: User Engagement")
	fmt.Println("---------------------------")
	fmt.Println("Request: Email confirmations, SMS alerts, webhooks")
	fmt.Println("Solution: Add async notifications with timeout")
	EnableNotifications()
	fmt.Printf("Pipeline: %v\n", ProfileUpdatePipeline.Names())
	runDemoScenario("full production")
	fmt.Println()

	// Final thoughts
	fmt.Println("📈 JOURNEY COMPLETE")
	fmt.Println("==================")
	fmt.Println("From 2-step MVP to production-grade pipeline:")
	fmt.Println("- Started with: [validate_image -> update_database]")
	fmt.Printf("- Ended with:   %v\n", ProfileUpdatePipeline.Names())
	fmt.Println()
	fmt.Println("Each feature was added without changing existing code - just feature flags!")
	fmt.Println("This is the power of pipz: complexity through composition, not modification.")
}

func runDemoScenario(scenarioName string) {
	ctx := context.Background()
	profile := ProfileUpdate{
		UserID:    "user123",
		Photo:     []byte("fake profile photo data"),
		Timestamp: time.Now(),
		RequestID: fmt.Sprintf("req-%d", time.Now().Unix()),
	}

	start := time.Now()
	result, err := ProfileUpdatePipeline.Process(ctx, profile)
	duration := time.Since(start)

	if err != nil {
		fmt.Printf("❌ Error in %s: %v\n", scenarioName, err)
		var pipzErr *pipz.Error[ProfileUpdate]
		if errors.As(err, &pipzErr) {
			fmt.Printf("   Failed at: %v\n", pipzErr.Path)
		}
	} else {
		fmt.Printf("✅ Success - %s completed in %v\n", scenarioName, duration)

		// Show relevant details based on features enabled
		if len(result.ProcessingLog) > 0 {
			fmt.Printf("   Steps: %v\n", result.ProcessingLog)
		}

		if result.ProcessedImages.Thumbnail != nil {
			fmt.Printf("   ✓ Images processed (thumbnail, medium, large)\n")
		}

		if result.ModerationResult != ModerationPending {
			fmt.Printf("   ✓ Content moderation: %s\n", result.ModerationResult)
		}
	}
}

func demonstrateConcurrentPerformance() {
	ResetAllServices()

	ctx := context.Background()
	profile := ProfileUpdate{
		UserID: "user123",
		Photo:  []byte("test"),
	}

	// Sequential cache invalidation
	fmt.Println("Sequential cache invalidation:")
	start := time.Now()
	_, _ = InvalidateCDN.Process(ctx, profile)     //nolint:errcheck // demo code
	_, _ = InvalidateRedis.Process(ctx, profile)   //nolint:errcheck // demo code
	_, _ = InvalidateVarnish.Process(ctx, profile) //nolint:errcheck // demo code
	sequentialTime := time.Since(start)
	fmt.Printf("  CDN (200ms) → Redis (50ms) → Varnish (100ms) = %v\n", sequentialTime)

	// Concurrent cache invalidation
	fmt.Println("\nConcurrent cache invalidation:")
	start = time.Now()
	_, _ = CacheInvalidation.Process(ctx, profile) //nolint:errcheck // demo code
	concurrentTime := time.Since(start)
	fmt.Printf("  CDN + Redis + Varnish (parallel) = %v\n", concurrentTime)

	fmt.Printf("\nTime saved: %v (%.0f%% faster)\n",
		sequentialTime-concurrentTime,
		float64(sequentialTime-concurrentTime)/float64(sequentialTime)*100)
}
