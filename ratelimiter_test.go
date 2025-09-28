package pipz

import (
	"context"
	"errors"
	"math"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"

	"github.com/zoobzio/clockz"
	"github.com/zoobzio/tracez"
)

func TestRateLimiter_TokenBucket(t *testing.T) {
	t.Run("Initial State", func(t *testing.T) {
		clock := clockz.NewFakeClock()
		limiter := NewRateLimiter[int]("test", 10, 5)
		limiter.WithClock(clock)

		// Should start with full bucket
		if tokens := limiter.GetAvailableTokens(); tokens != 5.0 {
			t.Errorf("expected 5 tokens, got %f", tokens)
		}
	})

	t.Run("Token Consumption", func(t *testing.T) {
		clock := clockz.NewFakeClock()
		limiter := NewRateLimiter[int]("test", 10, 5)
		limiter.WithClock(clock).SetMode("drop")

		// Use 3 tokens
		for i := 0; i < 3; i++ {
			_, err := limiter.Process(context.Background(), i)
			if err != nil {
				t.Fatalf("unexpected error on request %d: %v", i, err)
			}
		}

		// Should have 2 tokens left
		if tokens := limiter.GetAvailableTokens(); tokens != 2.0 {
			t.Errorf("expected 2 tokens, got %f", tokens)
		}
	})

	t.Run("Token Refill Formula", func(t *testing.T) {
		clock := clockz.NewFakeClock()
		limiter := NewRateLimiter[int]("test", 10, 5) // 10 tokens/sec, burst 5
		limiter.WithClock(clock).SetMode("drop")

		// Use all tokens
		for i := 0; i < 5; i++ {
			limiter.Process(context.Background(), i)
		}

		// Advance time by 0.3 seconds (should add 3 tokens)
		clock.Advance(300 * time.Millisecond)

		// Should have 3 tokens (0 + 0.3 * 10)
		if tokens := limiter.GetAvailableTokens(); tokens != 3.0 {
			t.Errorf("expected 3 tokens, got %f", tokens)
		}

		// Advance time by 1 second (should cap at burst)
		clock.Advance(1 * time.Second)

		// Should be capped at 5 tokens (burst limit)
		if tokens := limiter.GetAvailableTokens(); tokens != 5.0 {
			t.Errorf("expected 5 tokens (burst cap), got %f", tokens)
		}
	})

	t.Run("Fractional Tokens", func(t *testing.T) {
		clock := clockz.NewFakeClock()
		limiter := NewRateLimiter[int]("test", 1.5, 3) // 1.5 tokens/sec
		limiter.WithClock(clock).SetMode("drop")

		// Use all tokens
		for i := 0; i < 3; i++ {
			limiter.Process(context.Background(), i)
		}

		// Advance by 2/3 second (should add 1 token: 2/3 * 1.5 = 1.0)
		clock.Advance(time.Second * 2 / 3)

		// Should have exactly 1.0 token
		if tokens := limiter.GetAvailableTokens(); math.Abs(tokens-1.0) > 0.001 {
			t.Errorf("expected 1.0 token, got %f", tokens)
		}

		// Advance by 1/3 second more (should add 0.5 token: 1/3 * 1.5 = 0.5)
		clock.Advance(time.Second * 1 / 3)

		// Should have 1.5 tokens
		if tokens := limiter.GetAvailableTokens(); math.Abs(tokens-1.5) > 0.001 {
			t.Errorf("expected 1.5 tokens, got %f", tokens)
		}
	})
}

func TestRateLimiter_EdgeCases(t *testing.T) {
	t.Run("Infinite Rate", func(t *testing.T) {
		clock := clockz.NewFakeClock()
		limiter := NewRateLimiter[int]("test", math.Inf(1), 5)
		limiter.WithClock(clock)

		// Should allow unlimited processing
		for i := 0; i < 100; i++ {
			start := clock.Now()
			_, err := limiter.Process(context.Background(), i)
			end := clock.Now()

			if err != nil {
				t.Fatalf("unexpected error with infinite rate: %v", err)
			}
			if !start.Equal(end) {
				t.Errorf("infinite rate should not wait, but time advanced")
			}
		}
	})

	t.Run("Zero Rate Blocks Forever", func(t *testing.T) {
		clock := clockz.NewFakeClock()
		limiter := NewRateLimiter[int]("test", 0, 1)
		limiter.WithClock(clock)

		// Use the initial token
		_, err := limiter.Process(context.Background(), 1)
		if err != nil {
			t.Fatalf("unexpected error using initial token: %v", err)
		}

		// Second request should block until context cancellation
		ctx, cancel := context.WithCancel(context.Background())

		done := make(chan error, 1)
		go func() {
			_, err := limiter.Process(ctx, 2)
			done <- err
		}()

		// Give goroutine time to start waiting
		clock.BlockUntilReady()

		// Cancel context
		cancel()

		// Should return with cancellation error
		select {
		case err := <-done:
			if err == nil {
				t.Fatal("expected cancellation error with zero rate")
			}
			var pipzErr *Error[int]
			if !errors.As(err, &pipzErr) || !pipzErr.IsCanceled() {
				t.Errorf("expected canceled error, got %v", err)
			}
		case <-time.After(100 * time.Millisecond):
			t.Fatal("zero rate cancellation took too long")
		}
	})

	t.Run("Context Cancellation During Wait", func(t *testing.T) {
		clock := clockz.NewFakeClock()
		limiter := NewRateLimiter[int]("test", 1, 1) // 1 token/sec, burst 1
		limiter.WithClock(clock)

		// Use the initial token
		_, err := limiter.Process(context.Background(), 1)
		if err != nil {
			t.Fatalf("unexpected error using initial token: %v", err)
		}

		// Create cancelable context
		ctx, cancel := context.WithCancel(context.Background())

		// Start processing in goroutine
		done := make(chan error, 1)
		go func() {
			_, err := limiter.Process(ctx, 2)
			done <- err
		}()

		// Wait for goroutine to start waiting
		clock.BlockUntilReady()

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
}

func TestRateLimiter_WaitMode(t *testing.T) {
	t.Run("Wait Formula Calculation", func(t *testing.T) {
		clock := clockz.NewFakeClock()
		limiter := NewRateLimiter[int]("test", 2, 1) // 2 tokens/sec, burst 1
		limiter.WithClock(clock)

		// Use the initial token
		_, err := limiter.Process(context.Background(), 1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Process second request in goroutine
		done := make(chan error, 1)
		go func() {
			_, err := limiter.Process(context.Background(), 2)
			done <- err
		}()

		// Should wait for 0.5 seconds (1 token / 2 tokens/sec)
		clock.BlockUntilReady()
		clock.Advance(500 * time.Millisecond)

		// Should complete successfully
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("unexpected error after wait: %v", err)
			}
		case <-time.After(100 * time.Millisecond):
			t.Fatal("wait did not complete")
		}
	})

	t.Run("Burst Processing", func(t *testing.T) {
		clock := clockz.NewFakeClock()
		limiter := NewRateLimiter[int]("test", 10, 3) // 10 tokens/sec, burst 3
		limiter.WithClock(clock)

		start := clock.Now()

		// Should allow burst of 3 immediately
		for i := 0; i < 3; i++ {
			_, err := limiter.Process(context.Background(), i)
			if err != nil {
				t.Fatalf("unexpected error on burst request %d: %v", i, err)
			}
		}

		end := clock.Now()
		if !start.Equal(end) {
			t.Errorf("burst should be immediate, but time advanced from %v to %v", start, end)
		}
	})

	t.Run("Rate Enforcement", func(t *testing.T) {
		clock := clockz.NewFakeClock()
		limiter := NewRateLimiter[int]("test", 10, 1) // 10 tokens/sec, burst 1
		limiter.WithClock(clock)

		// Process requests and track timing
		var results []time.Time
		for i := 0; i < 3; i++ {
			_, err := limiter.Process(context.Background(), i)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			results = append(results, clock.Now())

			// Advance time to allow next token if needed
			if i < 2 { // Don't advance after last request
				clock.Advance(100 * time.Millisecond) // 0.1 second = 1 token at 10/sec
			}
		}

		// First should be immediate
		expected := results[0]
		if !results[0].Equal(expected) {
			t.Errorf("first request timing wrong")
		}

		// Second should be 100ms later
		expected = expected.Add(100 * time.Millisecond)
		if !results[1].Equal(expected) {
			t.Errorf("second request timing wrong: expected %v, got %v", expected, results[1])
		}

		// Third should be 200ms from start
		expected = results[0].Add(200 * time.Millisecond)
		if !results[2].Equal(expected) {
			t.Errorf("third request timing wrong: expected %v, got %v", expected, results[2])
		}
	})
}

func TestRateLimiter_DropMode(t *testing.T) {
	t.Run("Immediate Error When No Tokens", func(t *testing.T) {
		clock := clockz.NewFakeClock()
		limiter := NewRateLimiter[int]("test", 1, 1) // 1 token/sec, burst 1
		limiter.WithClock(clock).SetMode("drop")

		// First request should succeed
		_, err := limiter.Process(context.Background(), 1)
		if err != nil {
			t.Fatalf("unexpected error on first request: %v", err)
		}

		// Second request should fail immediately
		start := clock.Now()
		_, err = limiter.Process(context.Background(), 2)
		end := clock.Now()

		if err == nil {
			t.Fatal("expected rate limit error")
		}
		if !strings.Contains(err.Error(), "rate limit exceeded") {
			t.Errorf("unexpected error: %v", err)
		}
		if !start.Equal(end) {
			t.Errorf("drop mode should be immediate")
		}
	})

	t.Run("Success After Token Refill", func(t *testing.T) {
		clock := clockz.NewFakeClock()
		limiter := NewRateLimiter[int]("test", 2, 1) // 2 tokens/sec, burst 1
		limiter.WithClock(clock).SetMode("drop")

		// Use the token
		_, err := limiter.Process(context.Background(), 1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Should fail immediately
		_, err = limiter.Process(context.Background(), 2)
		if err == nil {
			t.Fatal("expected rate limit error")
		}

		// Advance time by 0.5 seconds (should add 1 token)
		clock.Advance(500 * time.Millisecond)

		// Should succeed now
		_, err = limiter.Process(context.Background(), 3)
		if err != nil {
			t.Errorf("unexpected error after refill: %v", err)
		}
	})
}

func TestRateLimiter_Configuration(t *testing.T) {
	t.Run("SetRate During Operation", func(t *testing.T) {
		clock := clockz.NewFakeClock()
		limiter := NewRateLimiter[int]("test", 1, 3) // 1 token/sec, burst 3
		limiter.WithClock(clock).SetMode("drop")

		// Use all tokens
		for i := 0; i < 3; i++ {
			limiter.Process(context.Background(), i)
		}

		// Advance 0.5 seconds (should add 0.5 tokens at 1/sec)
		clock.Advance(500 * time.Millisecond)

		// Change rate to 2/sec
		limiter.SetRate(2)

		// Advance another 0.5 seconds (should add 1 token at 2/sec)
		clock.Advance(500 * time.Millisecond)

		// Should have 1.5 tokens total (0.5 + 1.0)
		if tokens := limiter.GetAvailableTokens(); math.Abs(tokens-1.5) > 0.001 {
			t.Errorf("expected 1.5 tokens after rate change, got %f", tokens)
		}
	})

	t.Run("SetBurst Caps Tokens", func(t *testing.T) {
		clock := clockz.NewFakeClock()
		limiter := NewRateLimiter[int]("test", 10, 5) // burst 5
		limiter.WithClock(clock)

		// Start with 5 tokens
		if tokens := limiter.GetAvailableTokens(); tokens != 5.0 {
			t.Errorf("expected 5 tokens initially, got %f", tokens)
		}

		// Reduce burst to 3
		limiter.SetBurst(3)

		// Should cap tokens to 3
		if tokens := limiter.GetAvailableTokens(); tokens != 3.0 {
			t.Errorf("expected 3 tokens after burst reduction, got %f", tokens)
		}
	})

	t.Run("Invalid Mode Handling", func(t *testing.T) {
		limiter := NewRateLimiter[int]("test", 10, 2)

		// Set invalid mode - should be ignored
		limiter.SetMode("invalid")

		// Should still be in wait mode
		if mode := limiter.GetMode(); mode != "wait" {
			t.Errorf("expected mode to remain wait, got %s", mode)
		}
	})

	t.Run("Configuration Getters", func(t *testing.T) {
		limiter := NewRateLimiter[int]("test-limiter", 15.5, 7)

		if rate := limiter.GetRate(); rate != 15.5 {
			t.Errorf("expected rate 15.5, got %f", rate)
		}
		if burst := limiter.GetBurst(); burst != 7 {
			t.Errorf("expected burst 7, got %d", burst)
		}
		if mode := limiter.GetMode(); mode != "wait" {
			t.Errorf("expected mode wait, got %s", mode)
		}
		if name := limiter.Name(); name != "test-limiter" {
			t.Errorf("expected name test-limiter, got %s", name)
		}
	})

	t.Run("Method Chaining", func(t *testing.T) {
		limiter := NewRateLimiter[int]("test", 10, 2)

		// Update settings using method chaining
		result := limiter.SetRate(20).SetBurst(5).SetMode("drop")

		// Should return the same instance
		if result != limiter {
			t.Error("method chaining should return same instance")
		}

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
}

func TestRateLimiter_ConcurrentAccess(t *testing.T) {
	t.Run("Concurrent Processing", func(t *testing.T) {
		limiter := NewRateLimiter[int]("test", 1000, 100) // High rate to avoid blocking

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
				limiter.SetRate(float64(500 + i*10))
				limiter.SetBurst(50 + i)
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

	t.Run("Thread Safety", func(_ *testing.T) {
		clock := clockz.NewFakeClock()
		limiter := NewRateLimiter[int]("test", 10000, 1000) // High rate/burst to avoid blocking
		limiter.WithClock(clock).SetMode("drop")            // Use drop mode to avoid waiting

		var wg sync.WaitGroup
		const goroutines = 10 // Reduced from 50
		const operations = 20 // Reduced from 100

		// Test concurrent access to all methods
		for i := 0; i < goroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := 0; j < operations; j++ {
					switch j % 6 {
					case 0:
						// Process ignoring errors (some may be rate limited in drop mode)
						limiter.Process(context.Background(), id*operations+j)
					case 1:
						limiter.SetRate(float64(100 + j%900)) // Higher base rate
					case 2:
						limiter.SetBurst(10 + j%90) // Higher base burst
					case 3:
						limiter.GetRate()
					case 4:
						limiter.GetBurst()
					case 5:
						limiter.Name()
					}
				}
			}(i)
		}

		wg.Wait()
		// If we reach here without deadlock or panic, the test passes
	})
}

func TestRateLimiter_InvalidModeProcessing(t *testing.T) {
	// Test hitting the default case in Process method
	limiter := NewRateLimiter[int]("test-limiter", 10, 0) // Zero burst to force switch statement

	// Use all tokens
	limiter.SetBurst(1)
	_, err := limiter.Process(context.Background(), 1)
	if err != nil {
		t.Fatalf("unexpected error consuming token: %v", err)
	}

	// Use reflection to set an invalid mode directly
	limiterValue := reflect.ValueOf(limiter).Elem()
	modeField := limiterValue.FieldByName("mode")
	modeField = reflect.NewAt(modeField.Type(), unsafe.Pointer(modeField.UnsafeAddr())).Elem()
	modeField.SetString("invalid-mode-for-testing")

	// Process should hit the default case and return an error
	_, err = limiter.Process(context.Background(), 42)
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
}

func TestRateLimiter_PanicRecovery(t *testing.T) {
	// Test panic recovery in rate limiter chain
	panicProcessor := Apply("panic_processor", func(_ context.Context, _ int) (int, error) {
		panic("rate limiter downstream panic")
	})

	limiter := NewRateLimiter[int]("panic_limiter", 100, 10)
	sequence := NewSequence("test_sequence", limiter, panicProcessor)

	result, err := sequence.Process(context.Background(), 42)

	if result != 0 {
		t.Errorf("expected zero value 0, got %d", result)
	}

	var pipzErr *Error[int]
	if !errors.As(err, &pipzErr) {
		t.Fatal("expected pipz.Error")
	}

	if pipzErr.InputData != 42 {
		t.Errorf("expected input data 42, got %d", pipzErr.InputData)
	}
}

func TestRateLimiter_Integration(t *testing.T) {
	t.Run("Pipeline Integration", func(t *testing.T) {
		clock := clockz.NewFakeClock()
		calls := atomic.Int32{}

		processor := Apply("counter", func(_ context.Context, n int) (int, error) {
			calls.Add(1)
			return n * 2, nil
		})

		// Create rate-limited pipeline (10/sec)
		limiter := NewRateLimiter[int]("rate-limit", 10, 1)
		limiter.WithClock(clock)
		pipeline := NewSequence[int]("test-pipeline")
		pipeline.Register(limiter, processor)

		// Process 3 items
		results := make([]int, 3)
		for i := 0; i < 3; i++ {
			// Process in goroutine to handle waiting
			done := make(chan struct{})
			var err error
			go func(idx int) {
				results[idx], err = pipeline.Process(context.Background(), idx)
				done <- struct{}{}
			}(i)

			// Advance time if needed
			if i > 0 {
				clock.BlockUntilReady()
				clock.Advance(100 * time.Millisecond) // 0.1 second = 1 token at 10/sec
			}

			<-done
			if err != nil {
				t.Errorf("unexpected error on request %d: %v", i, err)
			}
			if results[i] != i*2 {
				t.Errorf("expected %d, got %d", i*2, results[i])
			}
		}

		if int(calls.Load()) != 3 {
			t.Errorf("expected 3 calls, got %d", calls.Load())
		}
	})
}

func TestRateLimiter_Observability(t *testing.T) {
	t.Run("Metrics and Spans - Wait Mode", func(t *testing.T) {
		clock := clockz.NewFakeClock()
		limiter := NewRateLimiter[int]("test-limiter", 2, 1) // 2 tokens/sec, burst 1
		limiter.WithClock(clock)
		defer limiter.Close()

		// Verify observability components are initialized
		if limiter.Metrics() == nil {
			t.Error("expected metrics registry to be initialized")
		}
		if limiter.Tracer() == nil {
			t.Error("expected tracer to be initialized")
		}

		// Capture spans using the callback API
		var spans []tracez.Span
		var spanMu sync.Mutex
		limiter.Tracer().OnSpanComplete(func(span tracez.Span) {
			spanMu.Lock()
			spans = append(spans, span)
			spanMu.Unlock()
		})

		// First request should succeed immediately
		_, err := limiter.Process(context.Background(), 42)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		// Second request should wait (start it before advancing clock)
		done := make(chan error, 1)
		go func() {
			_, err := limiter.Process(context.Background(), 43)
			done <- err
		}()

		// Wait for goroutine to start waiting
		clock.BlockUntilReady()
		// Now advance clock to refill tokens
		clock.Advance(500 * time.Millisecond) // 0.5 seconds = 1 token at 2/sec

		err = <-done
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		// Allow time for spans to be collected
		time.Sleep(10 * time.Millisecond)

		// Verify metrics
		processedTotal := limiter.Metrics().Counter(RateLimiterProcessedTotal).Value()
		if processedTotal != 2 {
			t.Errorf("expected 2 processed items, got %f", processedTotal)
		}

		allowedTotal := limiter.Metrics().Counter(RateLimiterAllowedTotal).Value()
		if allowedTotal != 2 {
			t.Errorf("expected 2 allowed items, got %f", allowedTotal)
		}

		droppedTotal := limiter.Metrics().Counter(RateLimiterDroppedTotal).Value()
		if droppedTotal != 0 {
			t.Errorf("expected 0 dropped items, got %f", droppedTotal)
		}

		// Check gauge values
		burstLimit := limiter.Metrics().Gauge(RateLimiterBurstLimit).Value()
		if burstLimit != 1 {
			t.Errorf("expected burst limit 1, got %f", burstLimit)
		}

		rateLimit := limiter.Metrics().Gauge(RateLimiterRateLimit).Value()
		if rateLimit != 2 {
			t.Errorf("expected rate limit 2, got %f", rateLimit)
		}

		// Note: With FakeClock, no real time passes, so wait time gauge will be 0
		// This is expected behavior - FakeClock makes everything instant

		// Verify spans were captured
		spanMu.Lock()
		spanCount := len(spans)
		spanMu.Unlock()

		if spanCount != 2 {
			t.Errorf("expected 2 spans, got %d", spanCount)
		}

		// Check span details
		spanMu.Lock()
		for i, span := range spans {
			if span.Name != string(RateLimiterProcessSpan) {
				t.Errorf("span %d: expected name %s, got %s", i, RateLimiterProcessSpan, span.Name)
			}

			if mode, ok := span.Tags[RateLimiterTagMode]; !ok || mode != "wait" {
				t.Errorf("span %d: expected mode=wait tag", i)
			}

			if allowed, ok := span.Tags[RateLimiterTagAllowed]; !ok || allowed != "true" {
				t.Errorf("span %d: expected allowed=true tag", i)
			}

			// Note: With FakeClock, wait time will be 0 even if the request "waited"
			// because no real time passes
		}
		spanMu.Unlock()
	})

	t.Run("Metrics and Spans - Drop Mode", func(t *testing.T) {
		clock := clockz.NewFakeClock()
		limiter := NewRateLimiter[int]("test-limiter-drop", 1, 1) // 1 token/sec, burst 1
		limiter.WithClock(clock).SetMode("drop")
		defer limiter.Close()

		// Capture spans
		var spans []tracez.Span
		var spanMu sync.Mutex
		limiter.Tracer().OnSpanComplete(func(span tracez.Span) {
			spanMu.Lock()
			spans = append(spans, span)
			spanMu.Unlock()
		})

		// First request should succeed
		_, err := limiter.Process(context.Background(), 1)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		// Second request should be dropped immediately
		_, err = limiter.Process(context.Background(), 2)
		if err == nil {
			t.Fatal("expected rate limit error")
		}

		// Third request after refill should succeed
		clock.Advance(1 * time.Second) // Refill 1 token
		_, err = limiter.Process(context.Background(), 3)
		if err != nil {
			t.Errorf("unexpected error after refill: %v", err)
		}

		// Verify metrics
		processedTotal := limiter.Metrics().Counter(RateLimiterProcessedTotal).Value()
		if processedTotal != 3 {
			t.Errorf("expected 3 processed items, got %f", processedTotal)
		}

		allowedTotal := limiter.Metrics().Counter(RateLimiterAllowedTotal).Value()
		if allowedTotal != 2 {
			t.Errorf("expected 2 allowed items, got %f", allowedTotal)
		}

		droppedTotal := limiter.Metrics().Counter(RateLimiterDroppedTotal).Value()
		if droppedTotal != 1 {
			t.Errorf("expected 1 dropped item, got %f", droppedTotal)
		}

		// Check token tracking
		tokensUsed := limiter.Metrics().Gauge(RateLimiterTokensUsed).Value()
		if tokensUsed != 2 { // 2 successful requests
			t.Errorf("expected 2 tokens used, got %f", tokensUsed)
		}

		// Verify spans
		spanMu.Lock()
		if len(spans) != 3 {
			t.Errorf("expected 3 spans, got %d", len(spans))
		}

		// Check span tags
		for i, span := range spans {
			if mode, ok := span.Tags[RateLimiterTagMode]; !ok || mode != "drop" {
				t.Errorf("span %d: expected mode=drop tag", i)
			}

			if i == 1 { // Second request was dropped
				if allowed, ok := span.Tags[RateLimiterTagAllowed]; !ok || allowed != "false" {
					t.Errorf("span %d: expected allowed=false tag for dropped request", i)
				}
				if errTag, ok := span.Tags[RateLimiterTagError]; !ok || !strings.Contains(errTag, "rate limit exceeded") {
					t.Errorf("span %d: expected error tag for dropped request", i)
				}
			} else { // First and third requests succeeded
				if allowed, ok := span.Tags[RateLimiterTagAllowed]; !ok || allowed != "true" {
					t.Errorf("span %d: expected allowed=true tag for successful request", i)
				}
			}
		}
		spanMu.Unlock()
	})

	t.Run("Dynamic Configuration Metrics", func(t *testing.T) {
		limiter := NewRateLimiter[int]("test-dynamic", 10, 5)
		defer limiter.Close()

		// Initial values
		rateLimit := limiter.Metrics().Gauge(RateLimiterRateLimit).Value()
		if rateLimit != 10 {
			t.Errorf("expected initial rate 10, got %f", rateLimit)
		}

		burstLimit := limiter.Metrics().Gauge(RateLimiterBurstLimit).Value()
		if burstLimit != 5 {
			t.Errorf("expected initial burst 5, got %f", burstLimit)
		}

		tokensAvailable := limiter.Metrics().Gauge(RateLimiterTokensAvailable).Value()
		if tokensAvailable != 5 {
			t.Errorf("expected initial tokens 5, got %f", tokensAvailable)
		}

		// Update configuration
		limiter.SetRate(20).SetBurst(10)

		// Check updated metrics
		rateLimit = limiter.Metrics().Gauge(RateLimiterRateLimit).Value()
		if rateLimit != 20 {
			t.Errorf("expected updated rate 20, got %f", rateLimit)
		}

		burstLimit = limiter.Metrics().Gauge(RateLimiterBurstLimit).Value()
		if burstLimit != 10 {
			t.Errorf("expected updated burst 10, got %f", burstLimit)
		}

		// Token availability should be updated when burst increases
		tokensAvailable = limiter.Metrics().Gauge(RateLimiterTokensAvailable).Value()
		if tokensAvailable != 5 { // Tokens don't increase just because burst increased
			t.Errorf("expected tokens to remain at 5, got %f", tokensAvailable)
		}

		// Reduce burst below current tokens
		limiter.SetBurst(3)

		burstLimit = limiter.Metrics().Gauge(RateLimiterBurstLimit).Value()
		if burstLimit != 3 {
			t.Errorf("expected reduced burst 3, got %f", burstLimit)
		}

		tokensAvailable = limiter.Metrics().Gauge(RateLimiterTokensAvailable).Value()
		if tokensAvailable != 3 { // Tokens should be capped to new burst
			t.Errorf("expected tokens capped to 3, got %f", tokensAvailable)
		}
	})

	t.Run("Context Cancellation Metrics", func(t *testing.T) {
		clock := clockz.NewFakeClock()
		limiter := NewRateLimiter[int]("test-cancel", 1, 1) // 1 token/sec, burst 1
		limiter.WithClock(clock)
		defer limiter.Close()

		// Use the initial token
		_, err := limiter.Process(context.Background(), 1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Create cancelable context
		ctx, cancel := context.WithCancel(context.Background())

		// Start processing in goroutine
		done := make(chan error, 1)
		go func() {
			_, err := limiter.Process(ctx, 2)
			done <- err
		}()

		// Wait for goroutine to start waiting
		clock.BlockUntilReady()

		// Cancel context
		cancel()

		// Should return with cancellation error
		err = <-done
		if err == nil {
			t.Fatal("expected cancellation error")
		}

		// Check metrics
		processedTotal := limiter.Metrics().Counter(RateLimiterProcessedTotal).Value()
		if processedTotal != 2 {
			t.Errorf("expected 2 processed items (including canceled), got %f", processedTotal)
		}

		allowedTotal := limiter.Metrics().Counter(RateLimiterAllowedTotal).Value()
		if allowedTotal != 1 { // Only first request was allowed
			t.Errorf("expected 1 allowed item, got %f", allowedTotal)
		}
	})

	t.Run("Hooks fire on rate limiting events", func(t *testing.T) {
		clock := clockz.NewFakeClock()
		limiter := NewRateLimiter[int]("test-hooks", 1, 1) // 1 per second, burst of 1
		limiter.WithClock(clock).SetMode("drop")
		defer limiter.Close()

		var limitedCount atomic.Int32
		var exhaustedCount atomic.Int32

		// Register hooks
		limiter.OnLimited(func(_ context.Context, _ RateLimiterEvent) error {
			limitedCount.Add(1)
			return nil
		})

		limiter.OnExhausted(func(_ context.Context, _ RateLimiterEvent) error {
			exhaustedCount.Add(1)
			return nil
		})

		// First request should succeed (uses the 1 available token)
		_, err := limiter.Process(context.Background(), 1)
		if err != nil {
			t.Errorf("expected first request to succeed, got error: %v", err)
		}

		// Second request should be dropped (no tokens)
		_, err = limiter.Process(context.Background(), 2)
		if err == nil {
			t.Error("expected second request to be dropped")
		}

		// Wait for async hooks
		time.Sleep(10 * time.Millisecond)

		if limitedCount.Load() != 1 {
			t.Errorf("expected 1 limited event, got %d", limitedCount.Load())
		}

		// Switch to wait mode to test exhausted event
		limiter.SetMode("wait")

		// Try another request in a goroutine (will wait for token)
		go func() {
			limiter.Process(context.Background(), 3)
		}()

		// Give it a moment to enter wait state
		time.Sleep(10 * time.Millisecond)

		if exhaustedCount.Load() != 1 {
			t.Errorf("expected 1 exhausted event, got %d", exhaustedCount.Load())
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

	b.Run("Token Bucket Operations", func(b *testing.B) {
		limiter := NewRateLimiter[int]("bench-limiter", 100, 10)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// Test the core token bucket operations
			limiter.GetAvailableTokens()
		}
	})
}
