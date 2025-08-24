package pipz

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"
)

func TestRateLimiter(t *testing.T) {
	t.Run("Allows Processing Within Rate", func(t *testing.T) {
		// 10 requests per second with burst of 2
		limiter := NewRateLimiter[int]("test-limiter", 10, 2)

		// Should allow burst of 2 immediately
		for i := 0; i < 2; i++ {
			start := time.Now()
			result, err := limiter.Process(context.Background(), i)
			elapsed := time.Since(start)

			if err != nil {
				t.Fatalf("unexpected error on request %d: %v", i, err)
			}
			if result != i {
				t.Errorf("expected %d, got %d", i, result)
			}
			// Should be immediate (< 10ms)
			if elapsed > 10*time.Millisecond {
				t.Errorf("request %d took too long: %v", i, elapsed)
			}
		}
	})

	t.Run("Enforces Rate Limit in Wait Mode", func(t *testing.T) {
		// 5 requests per second (200ms between requests)
		limiter := NewRateLimiter[string]("test-limiter", 5, 1)

		start := time.Now()
		// First request should be immediate
		_, err := limiter.Process(context.Background(), "first")
		if err != nil {
			t.Fatalf("unexpected error on first request: %v", err)
		}

		// Second request should wait ~200ms
		_, err = limiter.Process(context.Background(), "second")
		if err != nil {
			t.Fatalf("unexpected error on second request: %v", err)
		}

		elapsed := time.Since(start)
		// Should take approximately 200ms (with some tolerance)
		if elapsed < 150*time.Millisecond || elapsed > 250*time.Millisecond {
			t.Errorf("expected ~200ms delay, got %v", elapsed)
		}
	})

	t.Run("Drop Mode Returns Error Immediately", func(t *testing.T) {
		// 1 request per second with burst of 1
		limiter := NewRateLimiter[int]("test-limiter", 1, 1)
		limiter.SetMode("drop")

		// First request should succeed
		_, err := limiter.Process(context.Background(), 1)
		if err != nil {
			t.Fatalf("unexpected error on first request: %v", err)
		}

		// Second request should fail immediately
		start := time.Now()
		_, err = limiter.Process(context.Background(), 2)
		elapsed := time.Since(start)

		if err == nil {
			t.Fatal("expected rate limit error")
		}
		if !strings.Contains(err.Error(), "rate limit exceeded") {
			t.Errorf("unexpected error: %v", err)
		}
		// Should fail immediately (< 10ms)
		if elapsed > 10*time.Millisecond {
			t.Errorf("drop mode took too long: %v", elapsed)
		}
	})

	t.Run("Context Cancellation During Wait", func(t *testing.T) {
		// Very low rate to ensure waiting
		limiter := NewRateLimiter[int]("test-limiter", 0.1, 1) // 1 request per 10 seconds

		// Use up the burst
		_, err := limiter.Process(context.Background(), 1)
		if err != nil {
			t.Fatalf("unexpected error using up burst: %v", err)
		}

		// Create cancelable context
		ctx, cancel := context.WithCancel(context.Background())

		// Start processing in goroutine
		done := make(chan error, 1)
		go func() {
			_, err := limiter.Process(ctx, 2)
			done <- err
		}()

		// Give it time to start waiting
		time.Sleep(50 * time.Millisecond)

		// Cancel the context
		cancel()

		// Should return quickly with cancellation error
		select {
		case err := <-done:
			if err == nil {
				t.Fatal("expected cancellation error")
			}
			var pipzErr *Error[int]
			if !errors.As(err, &pipzErr) || !pipzErr.IsCanceled() {
				t.Errorf("expected canceled error, got %v", err)
			}
		case <-time.After(100 * time.Millisecond):
			t.Fatal("cancellation took too long")
		}
	})

	t.Run("Concurrent Access Safety", func(t *testing.T) {
		limiter := NewRateLimiter[int]("test-limiter", 100, 10)

		var wg sync.WaitGroup
		errors := make(chan error, 100)

		// Run multiple goroutines concurrently
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := 0; j < 10; j++ {
					result, err := limiter.Process(context.Background(), id*10+j)
					if err != nil {
						errors <- err
						return
					}
					if result != id*10+j {
						errors <- err
						return
					}
				}
			}(i)
		}

		// Also modify configuration concurrently
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 20; i++ {
				limiter.SetRate(float64(50 + i*10))
				limiter.SetBurst(5 + i)
				limiter.SetMode([]string{"wait", "drop"}[i%2])
				time.Sleep(time.Millisecond)
			}
		}()

		wg.Wait()
		close(errors)

		// Check for any errors (ignoring rate limit errors from drop mode)
		for err := range errors {
			if err != nil && !strings.Contains(err.Error(), "rate limit exceeded") {
				t.Errorf("concurrent access error: %v", err)
			}
		}
	})

	t.Run("Runtime Configuration Changes", func(t *testing.T) {
		limiter := NewRateLimiter[int]("test-limiter", 10, 2)

		// Verify initial settings
		if rate := limiter.GetRate(); rate != 10 {
			t.Errorf("expected rate 10, got %f", rate)
		}
		if burst := limiter.GetBurst(); burst != 2 {
			t.Errorf("expected burst 2, got %d", burst)
		}
		if mode := limiter.GetMode(); mode != "wait" {
			t.Errorf("expected mode wait, got %s", mode)
		}

		// Update settings
		limiter.SetRate(20).SetBurst(5).SetMode("drop")

		// Verify updates
		if rate := limiter.GetRate(); rate != 20 {
			t.Errorf("expected rate 20, got %f", rate)
		}
		if burst := limiter.GetBurst(); burst != 5 {
			t.Errorf("expected burst 5, got %d", burst)
		}
		if mode := limiter.GetMode(); mode != "drop" {
			t.Errorf("expected mode drop, got %s", mode)
		}
	})

	t.Run("Invalid Mode Handling", func(t *testing.T) {
		limiter := NewRateLimiter[int]("test-limiter", 10, 2)

		// Set invalid mode - should be ignored
		limiter.SetMode("invalid")

		// Should still be in wait mode
		if mode := limiter.GetMode(); mode != "wait" {
			t.Errorf("expected mode to remain wait, got %s", mode)
		}
	})

	t.Run("Burst Capacity", func(t *testing.T) {
		// 2 requests per second with burst of 5
		limiter := NewRateLimiter[int]("test-limiter", 2, 5)
		limiter.SetMode("drop")

		// Should allow burst of 5 immediately
		for i := 0; i < 5; i++ {
			_, err := limiter.Process(context.Background(), i)
			if err != nil {
				t.Errorf("unexpected error on burst request %d: %v", i, err)
			}
		}

		// 6th request should fail
		_, err := limiter.Process(context.Background(), 6)
		if err == nil {
			t.Fatal("expected rate limit error after burst")
		}

		// Wait for tokens to replenish (at 2/sec, need 500ms for 1 token)
		time.Sleep(600 * time.Millisecond)

		// Should allow one more
		_, err = limiter.Process(context.Background(), 7)
		if err != nil {
			t.Errorf("unexpected error after replenish: %v", err)
		}
	})

	t.Run("Integration with Pipeline", func(t *testing.T) {
		calls := atomic.Int32{}
		processor := Apply("counter", func(_ context.Context, n int) (int, error) {
			calls.Add(1)
			return n * 2, nil
		})

		// Create rate-limited pipeline (10/sec)
		limiter := NewRateLimiter[int]("rate-limit", 10, 1)
		pipeline := NewSequence[int]("test-pipeline")
		pipeline.Register(limiter, processor)

		start := time.Now()
		// Process 3 items (should take ~200ms for the last 2)
		for i := 0; i < 3; i++ {
			result, err := pipeline.Process(context.Background(), i)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if result != i*2 {
				t.Errorf("expected %d, got %d", i*2, result)
			}
		}
		elapsed := time.Since(start)

		if int(calls.Load()) != 3 {
			t.Errorf("expected 3 calls, got %d", calls.Load())
		}
		// Should take at least 200ms for rate limiting
		if elapsed < 190*time.Millisecond {
			t.Errorf("processing too fast, elapsed: %v", elapsed)
		}
	})

	t.Run("Name Method", func(t *testing.T) {
		limiter := NewRateLimiter[int]("test-rate-limiter", 10, 2)
		// Test Name() method
		name := limiter.Name()
		if name != "test-rate-limiter" {
			t.Errorf("expected name 'test-rate-limiter', got '%s'", name)
		}
		// Test name is preserved during concurrent access
		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < 100; j++ {
					n := limiter.Name()
					if n != "test-rate-limiter" {
						t.Errorf("expected name 'test-rate-limiter', got '%s'", n)
					}
				}
			}()
		}
		wg.Wait()
	})

	t.Run("Invalid Mode in Process", func(t *testing.T) {
		// This test forces the default case in Process method by
		// directly manipulating the mode field to cover lines 102-109
		limiter := NewRateLimiter[int]("test-limiter", 10, 2)

		// Use reflection to set an invalid mode directly, bypassing SetMode validation
		limiterValue := reflect.ValueOf(limiter).Elem()
		modeField := limiterValue.FieldByName("mode")

		// Make field settable using unsafe
		modeField = reflect.NewAt(modeField.Type(), unsafe.Pointer(modeField.UnsafeAddr())).Elem()
		modeField.SetString("invalid-mode-for-testing")

		// Process should hit the default case and return an error
		_, err := limiter.Process(context.Background(), 42)
		if err == nil {
			t.Fatal("expected error for invalid mode")
		}

		var pipeErr *Error[int]
		if !errors.As(err, &pipeErr) {
			t.Fatal("expected Error type")
		}
		if !strings.Contains(pipeErr.Err.Error(), "invalid rate limiter mode") {
			t.Errorf("expected 'invalid rate limiter mode' error, got: %v", pipeErr.Err)
		}
		if pipeErr.InputData != 42 {
			t.Errorf("expected input data 42, got %d", pipeErr.InputData)
		}
		if len(pipeErr.Path) == 0 || pipeErr.Path[0] != "test-limiter" {
			t.Error("expected limiter name in error path")
		}
	})
}

func BenchmarkRateLimiter(b *testing.B) {
	b.Run("No Limit", func(b *testing.B) {
		// Very high rate that won't limit
		limiter := NewRateLimiter[int]("bench-limiter", 1000000, 1000)
		ctx := context.Background()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := limiter.Process(ctx, i)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("With Limiting", func(b *testing.B) {
		// Rate that will cause some waiting
		limiter := NewRateLimiter[int]("bench-limiter", 10000, 100)
		ctx := context.Background()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := limiter.Process(ctx, i)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("Drop Mode", func(b *testing.B) {
		limiter := NewRateLimiter[int]("bench-limiter", 10000, 100)
		limiter.SetMode("drop")
		ctx := context.Background()

		dropped := 0
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := limiter.Process(ctx, i)
			if err != nil {
				dropped++
			}
		}
		b.Logf("Dropped %d/%d requests", dropped, b.N)
	})
}
