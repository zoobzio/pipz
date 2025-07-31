# User Profile Update Pipeline

A real-world example showing how pipz handles complex multi-step operations with external services.

## The Problem

Updating a user profile seems simple - just save the new photo, right? In reality, a production profile update involves:

1. **Validation** - File size, format, dimensions
2. **Image Processing** - Generate thumbnails, optimize for web
3. **Content Moderation** - Check for inappropriate content
4. **Data Updates** - Save to database, update search index
5. **Cache Invalidation** - Clear CDN, Redis, application caches
6. **Notifications** - Email confirmation, notify connected services

## What Goes Wrong

Without proper pipeline management, developers face:

### 🔥 **Partial Failures**
- Image processed but database update fails
- Database updated but cache invalidation fails
- User sees old photo while system has new one

### ⏱️ **Timeout Cascades**
- Image processing takes too long
- Moderation API is slow
- User request times out but processing continues

### 💸 **Resource Waste**
- Retrying entire flow when only one step failed
- Processing same image multiple times
- Paying for API calls that could be avoided

### 🤯 **Error Handling Complexity**
```go
// Traditional approach - nested error handling nightmare
func UpdateProfile(photo []byte) error {
    validated, err := validateImage(photo)
    if err != nil {
        return fmt.Errorf("validation failed: %w", err)
    }
    
    processed, err := processImage(validated)
    if err != nil {
        // Should we rollback? Log? Retry?
        return fmt.Errorf("processing failed: %w", err)
    }
    
    moderated, err := moderateContent(processed)
    if err != nil {
        // Moderation down - fail or continue?
        // If we continue, how do we track this?
    }
    
    // ... more nesting
}
```

## The pipz Solution

With pipz, we start simple and evolve through feature flags:

### 📈 **Progressive Enhancement Through Feature Flags**

**Sprint 1 - MVP:**
```go
// Just validate and save - ship it!
var ProfileUpdatePipeline = pipz.NewSequence[ProfileUpdate](
    PipelineProfileUpdate,
    ValidateImage,
    UpdateDatabase,
)
```

**Sprint 3 - Add Image Processing:**
```go
// Mobile team needs thumbnails
EnableImageProcessing()
// Pipeline now: Validate → ProcessImage → Database
```

**Sprint 5 - Add Content Moderation:**
```go
// Legal requires content checking
EnableContentModeration()
// Pipeline now: Validate → Process → Moderate → Database
```

**Sprint 7+ - Production Hardening:**
```go
EnableDatabaseResilience()   // Retry transient failures
EnableCacheInvalidation()    // Keep data consistent
EnableNotifications()        // User engagement
```

Each feature flag modifies the pipeline at runtime without changing existing code!

### 🔄 **Provides Transaction-Like Behavior**
```go
// Wrap with compensation logic
var ProfileUpdateSafe = pipz.NewHandle(
    PipelineProfileUpdateSafe,
    ProfileUpdatePipeline,
    CleanupImages,  // Cleanup on failure
)
```

### 📊 **Enables Clear Observability**
Each component is named and trackable:
- See exactly where failures occur
- Measure performance of each step
- Add metrics without changing core logic

## Running the Example

### See It In Action
```bash
# Watch the pipeline evolve through feature flags
go run .

# Run tests to see different failure modes
go test -v
```

### Example Output
```
🚀 SPRINT 1: MVP Launch
--------------------
Requirements: Just save profile photos
Pipeline: [validate_image update_database]
✅ Success - basic upload completed in 30.5ms

📱 SPRINT 3: Mobile Optimization
------------------------------
Problem: Mobile users complaining about 10MB photos using all their data
Solution: Add image processing with thumbnails
Pipeline: [validate_image image-processing update_database]
✅ Success - process images completed in 130.8ms
   ✓ Images processed (thumbnail, medium, large)

=== Scenario: moderation-fallback ===
✅ Success (with fallback)
   Moderation: Approved
   Processing steps: [validation: passed, image: processed, moderation-backup: approved, database: saved]
   Duration: 1.58s (slower backup service)

=== Scenario: database-retry ===
✅ Success (after retries)
   Processing steps: [validation: passed, image: processed, moderation-api: approved, database: saved after 3 attempts]
   Duration: 1.44s

=== Scenario: partial-cache-failure ===
✅ Success (best effort)
   Note: CDN invalidation failed but request succeeded
   Duration: 1.33s
```

## Key Patterns Demonstrated

### 1. **Timeout Protection**
```go
var ImageProcessingTimeout = pipz.NewTimeout(
    ConnectorImageProcessing,
    ProcessImage,
    5*time.Second,  // Prevent hanging on slow processing
)
```

### 2. **Fallback Chains**
```go
var ModerationFallback = pipz.NewFallback[ProfileUpdate](
    ConnectorModeration,
    ModerateWithAPI,      // Try primary (fast)
    ModerateWithBackup,   // Try backup (slower)
    SkipModeration,       // Graceful degradation
)
```

### 3. **Retry Logic**
```go
var DatabaseWithRetry = pipz.NewRetry(
    ConnectorDatabase,
    UpdateDatabase,
    3,  // Handle transient database issues
)
```

### 4. **Concurrent I/O**
```go
var CacheInvalidation = pipz.NewConcurrent[ProfileUpdate](
    ConnectorCacheInvalidation,
    InvalidateCDN,      // ~200ms
    InvalidateRedis,    // ~50ms
    InvalidateVarnish,  // ~100ms
)
// Completes in ~200ms instead of 350ms
```

### 5. **Error Compensation**
```go
var ProfileUpdateSafe = pipz.NewHandle(
    PipelineProfileUpdateSafe,
    ProfileUpdatePipeline,
    CleanupImages,  // Remove processed images on failure
)
```

## Architecture Benefits

### Testability
Each component can be tested in isolation:
```go
// Test just the moderation fallback logic
func TestModerationFallback(t *testing.T) {
    ModerationAPI.Behavior = BehaviorError
    result, err := ModerationFallback.Process(ctx, profile)
    assert.Equal(t, ModerationSkipped, result.ModerationResult)
}
```

### Runtime Flexibility
Modify behavior without code changes:
```go
// Disable moderation for testing
ProfileUpdatePipeline.Replace(ConnectorModeration, SkipModeration)

// Add metrics to specific step
ProfileUpdatePipeline.After(ConnectorDatabase, RecordMetrics)
```

### Clear Error Context
Errors include the full path through the pipeline:
```
Error: profile-update -> image-processing -> timeout after 5s
```

## Learn More

- See `pipeline.go` for the complete implementation
- Run `go test -v` to see all test scenarios
- Check `services.go` to understand the stubbed services
- Read the [pipz documentation](https://github.com/zoobzio/pipz) for more patterns