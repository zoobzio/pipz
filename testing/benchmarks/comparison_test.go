package benchmarks

import (
	"context"
	"errors"
	"math"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/zoobzio/pipz"
)

// ProfileUpdate represents a user profile modification - tests basic cloning with simple fields..
type ProfileUpdate struct {
	UserID    int64
	Field     string // The field being updated
	OldValue  string // Previous value
	NewValue  string // New value
	Timestamp int64  // Unix timestamp
	Priority  int    // Update priority (0-10)
}

func (p ProfileUpdate) Clone() ProfileUpdate {
	// Shallow copy is sufficient - all fields are value types
	return ProfileUpdate{
		UserID:    p.UserID,
		Field:     p.Field,
		OldValue:  p.OldValue,
		NewValue:  p.NewValue,
		Timestamp: p.Timestamp,
		Priority:  p.Priority,
	}
}

// Realistic data structures for benchmarking.
type User struct {
	ID       int64
	Name     string
	Email    string
	Age      int
	Metadata map[string]interface{}
}

func (u User) Clone() User {
	// Deep copy the metadata map
	clonedMetadata := make(map[string]interface{}, len(u.Metadata))
	for k, v := range u.Metadata {
		clonedMetadata[k] = v
	}
	return User{
		ID:       u.ID,
		Name:     u.Name,
		Email:    u.Email,
		Age:      u.Age,
		Metadata: clonedMetadata,
	}
}

type Order struct {
	ID         string
	CustomerID int64
	Items      []OrderItem
	Total      float64
	Status     string
	Timestamp  time.Time
	Shipping   ShippingInfo
}

func (o Order) Clone() Order {
	// Deep copy items slice
	clonedItems := make([]OrderItem, len(o.Items))
	copy(clonedItems, o.Items)

	return Order{
		ID:         o.ID,
		CustomerID: o.CustomerID,
		Items:      clonedItems,
		Total:      o.Total,
		Status:     o.Status,
		Timestamp:  o.Timestamp,
		Shipping:   o.Shipping,
	}
}

type OrderItem struct {
	ProductID string
	Quantity  int
	Price     float64
}

type ShippingInfo struct {
	Address string
	City    string
	State   string
	ZipCode string
	Country string
	Method  string
	Cost    float64
}

// BenchmarkErrorHandlingPatterns compares pipz error handling with traditional patterns.
func BenchmarkErrorHandlingPatterns(b *testing.B) {
	ctx := context.Background()

	b.Run("Traditional_Error_Chain", func(b *testing.B) {
		// Traditional Go error handling pattern
		data := 42

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			result, err := validateTraditional(data)
			if err != nil {
				continue // Expected for benchmark
			}

			result, err = transformTraditional(result)
			if err != nil {
				continue
			}

			result, err = enrichTraditional(result)
			if err != nil {
				continue
			}

			result = finalizeTraditional(result)
			_ = result
		}
	})

	b.Run("Pipz_Error_Chain", func(b *testing.B) {
		// pipz equivalent
		pipeline := pipz.NewSequence("error-chain",
			pipz.Apply("validate", func(_ context.Context, n int) (int, error) {
				return validateTraditional(n)
			}),
			pipz.Apply("transform", func(_ context.Context, n int) (int, error) {
				return transformTraditional(n)
			}),
			pipz.Apply("enrich", func(_ context.Context, n int) (int, error) {
				return enrichTraditional(n)
			}),
			pipz.Transform("finalize", func(_ context.Context, n int) int {
				return finalizeTraditional(n)
			}),
		)

		data := 42

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			result, err := pipeline.Process(ctx, data)
			if err != nil {
				continue // Expected for benchmark
			}
			_ = result
		}
	})

	b.Run("Traditional_With_Detailed_Errors", func(b *testing.B) {
		data := 42

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			result, err := validateTraditional(data)
			if err != nil {
				// Validation error - continue to next iteration
				continue
			}

			result, err = transformTraditional(result)
			if err != nil {
				// Transformation error - continue to next iteration
				continue
			}

			result, err = enrichTraditional(result)
			if err != nil {
				// Enrichment error - continue to next iteration
				continue
			}

			result = finalizeTraditional(result)
			_ = result
		}
	})

	b.Run("Pipz_With_Rich_Errors", func(b *testing.B) {
		// pipz automatically provides rich error context
		pipeline := pipz.NewSequence("rich-errors",
			pipz.Apply("validate", func(_ context.Context, n int) (int, error) {
				return validateTraditional(n)
			}),
			pipz.Apply("transform", func(_ context.Context, n int) (int, error) {
				return transformTraditional(n)
			}),
			pipz.Apply("enrich", func(_ context.Context, n int) (int, error) {
				return enrichTraditional(n)
			}),
			pipz.Transform("finalize", func(_ context.Context, n int) int {
				return finalizeTraditional(n)
			}),
		)

		data := 42

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			result, err := pipeline.Process(ctx, data)
			if err != nil {
				// pipz error includes path, timing, etc.
				_ = err.Error()
				continue
			}
			_ = result
		}
	})
}

// BenchmarkRetryPatterns compares different retry implementations.
func BenchmarkRetryPatterns(b *testing.B) {
	ctx := context.Background()

	b.Run("Traditional_Manual_Retry", func(b *testing.B) {
		data := 42

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			result, err := retryManual(data, 3)
			if err != nil {
				continue // Some retries may still fail
			}
			_ = result
		}
	})

	b.Run("Pipz_Retry", func(b *testing.B) {
		flakyFunc := pipz.Apply("flaky", func(_ context.Context, n int) (int, error) {
			return flakyOperation(n)
		})

		retry := pipz.NewRetry("retry", flakyFunc, 3)
		data := 42

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			result, err := retry.Process(ctx, data)
			if err != nil {
				continue
			}
			_ = result
		}
	})

	b.Run("Traditional_Exponential_Backoff", func(b *testing.B) {
		data := 42

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			result, err := retryWithBackoff(data, 3, 1*time.Millisecond)
			if err != nil {
				continue
			}
			_ = result
		}
	})

	b.Run("Pipz_Backoff", func(b *testing.B) {
		flakyFunc := pipz.Apply("flaky", func(_ context.Context, n int) (int, error) {
			return flakyOperation(n)
		})

		backoff := pipz.NewBackoff("backoff", flakyFunc, 3, 1*time.Millisecond)
		data := 42

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			result, err := backoff.Process(ctx, data)
			if err != nil {
				continue
			}
			_ = result
		}
	})
}

// BenchmarkCircuitBreakerPatterns compares circuit breaker implementations.
func BenchmarkCircuitBreakerPatterns(b *testing.B) {
	ctx := context.Background()

	b.Run("Traditional_Manual_Circuit_Breaker", func(b *testing.B) {
		cb := &ManualCircuitBreaker{
			threshold: 5,
			timeout:   time.Minute,
		}
		data := 42

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			result, err := cb.Call(func() (int, error) {
				return stableOperation(data)
			})
			if err != nil {
				continue
			}
			_ = result
		}
	})

	b.Run("Pipz_Circuit_Breaker", func(b *testing.B) {
		processor := pipz.Apply("stable", func(_ context.Context, n int) (int, error) {
			return stableOperation(n)
		})

		cb := pipz.NewCircuitBreaker("cb", processor, 5, time.Minute)
		data := 42

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			result, err := cb.Process(ctx, data)
			if err != nil {
				continue
			}
			_ = result
		}
	})
}

// BenchmarkConcurrentPatterns compares concurrent processing patterns.
func BenchmarkConcurrentPatterns(b *testing.B) {
	ctx := context.Background()

	b.Run("Traditional_Fan_Out_Fan_In", func(b *testing.B) {
		data := 42

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			results, err := traditionalFanOutFanIn(ctx, data)
			if err != nil {
				b.Fatal(err)
			}
			_ = results
		}
	})

	b.Run("Pipz_Concurrent", func(b *testing.B) {
		concurrent := pipz.NewConcurrent("concurrent",
			nil,
			pipz.Transform("validate", func(_ context.Context, p ProfileUpdate) ProfileUpdate {
				if p.Priority < 0 {
					p.Priority = 0
				}
				if p.Priority > 10 {
					p.Priority = 10
				}
				return p
			}),
			pipz.Transform("normalize", func(_ context.Context, p ProfileUpdate) ProfileUpdate {
				p.Field = strings.ToLower(p.Field)
				return p
			}),
			pipz.Transform("timestamp", func(_ context.Context, p ProfileUpdate) ProfileUpdate {
				if p.Timestamp == 0 {
					p.Timestamp = time.Now().Unix()
				}
				return p
			}),
			pipz.Transform("prioritize", func(_ context.Context, p ProfileUpdate) ProfileUpdate {
				if p.Field == "email" || p.Field == "password" {
					p.Priority += 2
				}
				return p
			}),
		)

		data := ProfileUpdate{
			UserID:    12345,
			Field:     "email",
			OldValue:  "old@example.com",
			NewValue:  "new@example.com",
			Timestamp: 0, // Will be set by processor
			Priority:  5,
		}

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			result, err := concurrent.Process(ctx, data)
			if err != nil {
				b.Fatal(err)
			}
			_ = result
		}
	})

	b.Run("Traditional_First_Success", func(b *testing.B) {
		data := 42

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			result, err := traditionalFirstSuccess(ctx, data)
			if err != nil {
				b.Fatal(err)
			}
			_ = result
		}
	})

	b.Run("Pipz_Race", func(b *testing.B) {
		race := pipz.NewRace("race",
			pipz.Transform("fast-validate", func(_ context.Context, p ProfileUpdate) ProfileUpdate {
				if p.Priority < 5 {
					p.Priority = 5
				} // Quick priority boost
				return p
			}),
			pipz.Transform("slow-enrich", func(_ context.Context, p ProfileUpdate) ProfileUpdate {
				time.Sleep(time.Microsecond)
				p.Field = "enriched_" + p.Field // Simulate database lookup
				return p
			}),
			pipz.Transform("slower-audit", func(_ context.Context, p ProfileUpdate) ProfileUpdate {
				time.Sleep(2 * time.Microsecond)
				p.Timestamp = time.Now().Unix() // Audit timestamp
				return p
			}),
		)

		data := ProfileUpdate{
			UserID:    12345,
			Field:     "email",
			OldValue:  "old@example.com",
			NewValue:  "new@example.com",
			Timestamp: 0, // Will be set by processor
			Priority:  5,
		}

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			result, err := race.Process(ctx, data)
			if err != nil {
				b.Fatal(err)
			}
			_ = result
		}
	})
}

// BenchmarkRateLimitingPatterns compares rate limiting implementations.
func BenchmarkRateLimitingPatterns(b *testing.B) {
	ctx := context.Background()

	b.Run("Traditional_Token_Bucket", func(b *testing.B) {
		limiter := NewTraditionalRateLimiter(1000, 100) // 1000 RPS, burst 100
		data := 42

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			if limiter.Allow() {
				result, _ := stableOperation(data) //nolint:errcheck // Error ignored for benchmarking
				_ = result
			}
		}
	})

	b.Run("Pipz_Rate_Limiter", func(b *testing.B) {
		processor := pipz.Transform("stable", func(_ context.Context, n int) int {
			result, _ := stableOperation(n) //nolint:errcheck // Error ignored for benchmarking
			return result
		})

		// Create rate-limited pipeline
		limitedPipeline := pipz.NewSequence("rate-limited",
			pipz.NewRateLimiter[int]("limiter", 1000, 100),
			processor,
		)
		data := 42

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			result, err := limitedPipeline.Process(ctx, data)
			if err != nil {
				continue // Rate limited
			}
			_ = result
		}
	})
}

// BenchmarkComplexComposition compares complex pipeline compositions.
func BenchmarkComplexComposition(b *testing.B) {
	ctx := context.Background()

	b.Run("Traditional_Complex_Processing", func(b *testing.B) {
		data := 42

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			result, err := traditionalComplexProcessing(ctx, data)
			if err != nil {
				continue
			}
			_ = result
		}
	})

	b.Run("Pipz_Complex_Processing", func(b *testing.B) {
		// Equivalent pipz pipeline
		pipeline := pipz.NewSequence("complex",
			// Validation with fallback
			pipz.NewFallback("validation",
				pipz.Apply("strict-validate", func(_ context.Context, n int) (int, error) {
					if n <= 0 {
						return n, errors.New("must be positive")
					}
					return n, nil
				}),
				pipz.Apply("lenient-validate", func(_ context.Context, n int) (int, error) {
					if n < -100 {
						return n, errors.New("too negative")
					}
					return n, nil
				}),
			),

			// Circuit breaker with timeout and retry
			pipz.NewCircuitBreaker("circuit-breaker",
				// Retry with backoff
				pipz.NewBackoff("retry",
					// Timeout protection
					pipz.NewTimeout("timeout",
						// Sequential processing (since concurrent needs ClonableInt)
						pipz.NewSequence("sequential-processing",
							pipz.Transform("proc1", func(_ context.Context, n int) int { return n * 2 }),
							pipz.Transform("proc2", func(_ context.Context, n int) int { return n + 100 }),
							pipz.Transform("proc3", func(_ context.Context, n int) int { return n - 10 }),
						),
						100*time.Millisecond,
					),
					3,
					time.Millisecond,
				),
				5,
				time.Minute,
			),

			// Final processing
			pipz.Transform("finalize", func(_ context.Context, n int) int {
				return n + 1000
			}),
		)

		data := 42

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			result, err := pipeline.Process(ctx, data)
			if err != nil {
				continue
			}
			_ = result
		}
	})
}

// Helper functions for traditional implementations

var operationCounter int64

func validateTraditional(n int) (int, error) {
	if n < 0 {
		return 0, errors.New("negative number")
	}
	return n, nil
}

func transformTraditional(n int) (int, error) {
	if n > 1000 {
		return 0, errors.New("number too large")
	}
	return n * 2, nil
}

func enrichTraditional(n int) (int, error) {
	if n == 0 {
		return 0, errors.New("cannot enrich zero")
	}
	return n + 100, nil
}

func finalizeTraditional(n int) int {
	return n + 1
}

func flakyOperation(n int) (int, error) {
	count := atomic.AddInt64(&operationCounter, 1)
	if count%3 != 0 { // Fail 2/3 of the time
		return 0, errors.New("flaky failure")
	}
	return n * 2, nil
}

func stableOperation(n int) (int, error) {
	return n + 10, nil
}

func retryManual(data int, maxAttempts int) (int, error) {
	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		result, err := flakyOperation(data)
		if err == nil {
			return result, nil
		}
		lastErr = err
	}
	return 0, lastErr
}

func retryWithBackoff(data int, maxAttempts int, initialDelay time.Duration) (int, error) {
	var lastErr error
	delay := initialDelay

	for attempt := 0; attempt < maxAttempts; attempt++ {
		result, err := flakyOperation(data)
		if err == nil {
			return result, nil
		}
		lastErr = err

		if attempt < maxAttempts-1 {
			time.Sleep(delay)
			delay *= 2
		}
	}
	return 0, lastErr
}

// ManualCircuitBreaker implements a simple circuit breaker for comparison.
type ManualCircuitBreaker struct {
	mu        sync.RWMutex
	failures  int64
	threshold int64
	timeout   time.Duration
	lastFail  time.Time
	state     int32 // 0: closed, 1: open, 2: half-open
}

func (cb *ManualCircuitBreaker) Call(fn func() (int, error)) (int, error) {
	cb.mu.RLock()
	state := atomic.LoadInt32(&cb.state)
	_ = atomic.LoadInt64(&cb.failures)
	cb.mu.RUnlock()

	if state == 1 { // Open
		cb.mu.RLock()
		if time.Since(cb.lastFail) > cb.timeout {
			cb.mu.RUnlock()
			atomic.StoreInt32(&cb.state, 2) // Half-open
		} else {
			cb.mu.RUnlock()
			return 0, errors.New("circuit breaker open")
		}
	}

	result, err := fn()

	if err != nil {
		newFailures := atomic.AddInt64(&cb.failures, 1)
		if newFailures >= cb.threshold {
			cb.mu.Lock()
			cb.lastFail = time.Now()
			cb.mu.Unlock()
			atomic.StoreInt32(&cb.state, 1) // Open
		}
		return result, err
	}

	if state == 2 { // Half-open, success
		atomic.StoreInt64(&cb.failures, 0)
		atomic.StoreInt32(&cb.state, 0) // Closed
	}

	return result, nil
}

// TraditionalRateLimiter implements a simple token bucket for comparison.
type TraditionalRateLimiter struct {
	tokens   float64
	capacity float64
	rate     float64
	lastTime time.Time
	mu       sync.Mutex
}

func NewTraditionalRateLimiter(rate float64, capacity int) *TraditionalRateLimiter {
	return &TraditionalRateLimiter{
		tokens:   float64(capacity),
		capacity: float64(capacity),
		rate:     rate,
		lastTime: time.Now(),
	}
}

func (rl *TraditionalRateLimiter) Allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(rl.lastTime).Seconds()
	rl.lastTime = now

	rl.tokens = math.Min(rl.capacity, rl.tokens+elapsed*rl.rate)

	if rl.tokens >= 1.0 {
		rl.tokens--
		return true
	}
	return false
}

func traditionalFanOutFanIn(ctx context.Context, data int) ([]int, error) {
	results := make(chan int, 4)
	errors := make(chan error, 4)

	go func() {
		results <- data * 2
		errors <- nil
	}()
	go func() {
		results <- data + 10
		errors <- nil
	}()
	go func() {
		results <- data - 5
		errors <- nil
	}()
	go func() {
		results <- data * 3
		errors <- nil
	}()

	var collected []int
	for i := 0; i < 4; i++ {
		select {
		case result := <-results:
			collected = append(collected, result)
		case err := <-errors:
			if err != nil {
				return nil, err
			}
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	return collected, nil
}

func traditionalFirstSuccess(ctx context.Context, data int) (int, error) {
	result := make(chan int, 1)
	done := make(chan struct{})
	defer close(done)

	go func() {
		select {
		case result <- data * 2:
		case <-done:
		}
	}()
	go func() {
		time.Sleep(time.Microsecond)
		select {
		case result <- data + 10:
		case <-done:
		}
	}()
	go func() {
		time.Sleep(2 * time.Microsecond)
		select {
		case result <- data - 5:
		case <-done:
		}
	}()

	select {
	case r := <-result:
		return r, nil
	case <-ctx.Done():
		return 0, ctx.Err()
	}
}

func traditionalComplexProcessing(_ context.Context, data int) (int, error) {
	// FIXED: Genuinely equivalent implementation without artificial inefficiencies

	// Validation with fallback - equivalent to pipz Fallback
	result := data
	if result <= 0 {
		if result < -100 {
			return 0, errors.New("too negative")
		}
		// Fallback validation passed
	}

	// Sequential processing equivalent to pipz sequence
	// Step 1: multiply by 2
	result *= 2

	// Step 2: add 100
	result += 100

	// Step 3: subtract 10
	result -= 10

	// Final processing
	result += 1000

	return result, nil
}
