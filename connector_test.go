package pipz

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestSequential(t *testing.T) {
	t.Run("Empty Sequential", func(t *testing.T) {
		seq := Sequential[int]()
		result, err := seq.Process(context.Background(), 10)
		if err != nil {
			t.Errorf("empty sequential should not error: %v", err)
		}
		if result != 10 {
			t.Errorf("empty sequential should return input unchanged: got %d, want 10", result)
		}
	})

	t.Run("Single Chainable", func(t *testing.T) {
		double := ProcessorFunc[int](func(_ context.Context, n int) (int, error) {
			return n * 2, nil
		})

		seq := Sequential(double)
		result, err := seq.Process(context.Background(), 5)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result != 10 {
			t.Errorf("expected 10, got %d", result)
		}
	})

	t.Run("Multiple Chainables", func(t *testing.T) {
		double := ProcessorFunc[int](func(_ context.Context, n int) (int, error) {
			return n * 2, nil
		})
		addTen := ProcessorFunc[int](func(_ context.Context, n int) (int, error) {
			return n + 10, nil
		})

		seq := Sequential(double, addTen)
		result, err := seq.Process(context.Background(), 5)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result != 20 {
			t.Errorf("expected 20, got %d", result)
		}
	})

	t.Run("Error Propagation", func(t *testing.T) {
		double := ProcessorFunc[int](func(_ context.Context, n int) (int, error) {
			return n * 2, nil
		})
		fail := ProcessorFunc[int](func(_ context.Context, n int) (int, error) {
			return n, errors.New("intentional error")
		})
		neverRun := ProcessorFunc[int](func(_ context.Context, n int) (int, error) {
			t.Error("this should not run")
			return n, nil
		})

		seq := Sequential(double, fail, neverRun)
		result, err := seq.Process(context.Background(), 5)
		if err == nil || err.Error() != "intentional error" {
			t.Errorf("expected error 'intentional error', got: %v", err)
		}
		if result != 10 {
			t.Errorf("expected partial result 10, got %d", result)
		}
	})
}

func TestSwitch(t *testing.T) {
	t.Run("Route to Correct Branch", func(t *testing.T) {
		condition := func(_ context.Context, s string) string {
			if strings.HasPrefix(s, "test") {
				return "test"
			}
			return "other"
		}

		testRoute := ProcessorFunc[string](func(_ context.Context, s string) (string, error) {
			return s + "_test_route", nil
		})
		otherRoute := ProcessorFunc[string](func(_ context.Context, s string) (string, error) {
			return s + "_other_route", nil
		})

		sw := Switch(condition, map[string]Chainable[string]{
			"test":  testRoute,
			"other": otherRoute,
		})

		// Test "test" route
		result, err := sw.Process(context.Background(), "test123")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result != "test123_test_route" {
			t.Errorf("expected 'test123_test_route', got '%s'", result)
		}

		// Test "other" route
		result, err = sw.Process(context.Background(), "hello")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result != "hello_other_route" {
			t.Errorf("expected 'hello_other_route', got '%s'", result)
		}
	})

	t.Run("Default Route", func(t *testing.T) {
		condition := func(_ context.Context, n int) string {
			if n > 0 {
				return "positive"
			}
			return "negative"
		}

		positiveRoute := ProcessorFunc[int](func(_ context.Context, n int) (int, error) {
			return n * 2, nil
		})
		defaultRoute := ProcessorFunc[int](func(_ context.Context, n int) (int, error) {
			return n * -1, nil
		})

		sw := Switch(condition, map[string]Chainable[int]{
			"positive": positiveRoute,
			"default":  defaultRoute,
		})

		// Test positive route
		result, err := sw.Process(context.Background(), 5)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result != 10 {
			t.Errorf("expected 10, got %d", result)
		}

		// Test default route (negative)
		result, err = sw.Process(context.Background(), -5)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result != 5 {
			t.Errorf("expected 5, got %d", result)
		}
	})

	t.Run("No Matching Route", func(t *testing.T) {
		condition := func(_ context.Context, s string) string {
			return s
		}

		sw := Switch(condition, map[string]Chainable[string]{
			"a": ProcessorFunc[string](func(_ context.Context, s string) (string, error) { return s, nil }),
			"b": ProcessorFunc[string](func(_ context.Context, s string) (string, error) { return s, nil }),
		})

		result, err := sw.Process(context.Background(), "c")
		if err == nil {
			t.Error("expected error for no matching route")
		}
		if !strings.Contains(err.Error(), "no route for condition result: c") {
			t.Errorf("unexpected error message: %v", err)
		}
		if result != "c" {
			t.Errorf("expected input 'c' to be returned, got '%s'", result)
		}
	})
}

func TestFallback(t *testing.T) {
	t.Run("Primary Success", func(t *testing.T) {
		primary := ProcessorFunc[int](func(_ context.Context, n int) (int, error) {
			return n * 2, nil
		})
		fallback := ProcessorFunc[int](func(_ context.Context, n int) (int, error) {
			t.Error("fallback should not be called")
			return n, nil
		})

		fb := Fallback(primary, fallback)
		result, err := fb.Process(context.Background(), 5)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result != 10 {
			t.Errorf("expected 10, got %d", result)
		}
	})

	t.Run("Primary Fails, Fallback Success", func(t *testing.T) {
		primary := ProcessorFunc[int](func(_ context.Context, n int) (int, error) {
			return n, errors.New("primary failed")
		})
		fallback := ProcessorFunc[int](func(_ context.Context, n int) (int, error) {
			return n * 3, nil
		})

		fb := Fallback(primary, fallback)
		result, err := fb.Process(context.Background(), 5)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result != 15 {
			t.Errorf("expected 15, got %d", result)
		}
	})

	t.Run("Both Fail", func(t *testing.T) {
		primary := ProcessorFunc[int](func(_ context.Context, n int) (int, error) {
			return n, errors.New("primary failed")
		})
		fallback := ProcessorFunc[int](func(_ context.Context, n int) (int, error) {
			return n, errors.New("fallback failed")
		})

		fb := Fallback(primary, fallback)
		result, err := fb.Process(context.Background(), 5)
		if err == nil || err.Error() != "fallback failed" {
			t.Errorf("expected 'fallback failed' error, got: %v", err)
		}
		if result != 5 {
			t.Errorf("expected 5, got %d", result)
		}
	})
}

func TestRetry(t *testing.T) {
	t.Run("Success on First Try", func(t *testing.T) {
		attempts := 0
		proc := ProcessorFunc[int](func(_ context.Context, n int) (int, error) {
			attempts++
			return n * 2, nil
		})

		retry := Retry(proc, 3)
		result, err := retry.Process(context.Background(), 5)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result != 10 {
			t.Errorf("expected 10, got %d", result)
		}
		if attempts != 1 {
			t.Errorf("expected 1 attempt, got %d", attempts)
		}
	})

	t.Run("Success After Retries", func(t *testing.T) {
		attempts := 0
		proc := ProcessorFunc[int](func(_ context.Context, n int) (int, error) {
			attempts++
			if attempts < 3 {
				return n, errors.New("temporary failure")
			}
			return n * 2, nil
		})

		retry := Retry(proc, 3)
		result, err := retry.Process(context.Background(), 5)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result != 10 {
			t.Errorf("expected 10, got %d", result)
		}
		if attempts != 3 {
			t.Errorf("expected 3 attempts, got %d", attempts)
		}
	})

	t.Run("All Attempts Fail", func(t *testing.T) {
		attempts := 0
		proc := ProcessorFunc[int](func(_ context.Context, n int) (int, error) {
			attempts++
			return n, errors.New("persistent failure")
		})

		retry := Retry(proc, 3)
		result, err := retry.Process(context.Background(), 5)
		if err == nil {
			t.Error("expected error after all retries failed")
		}
		if !strings.Contains(err.Error(), "failed after 3 attempts") {
			t.Errorf("unexpected error message: %v", err)
		}
		if !strings.Contains(err.Error(), "persistent failure") {
			t.Errorf("error should wrap original error: %v", err)
		}
		if result != 5 {
			t.Errorf("expected 5, got %d", result)
		}
		if attempts != 3 {
			t.Errorf("expected 3 attempts, got %d", attempts)
		}
	})

	t.Run("Context Cancellation", func(t *testing.T) {
		attempts := 0
		proc := ProcessorFunc[int](func(ctx context.Context, n int) (int, error) {
			attempts++
			// Always check context first
			select {
			case <-ctx.Done():
				return n, ctx.Err()
			default:
			}
			return n, errors.New("failure")
		})

		retry := Retry(proc, 5)
		ctx, cancel := context.WithCancel(context.Background())

		// Cancel context immediately
		cancel()

		_, err := retry.Process(ctx, 5)
		if err == nil || !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled error, got: %v", err)
		}
		if attempts > 2 {
			t.Errorf("should stop retrying on context cancellation, got %d attempts", attempts)
		}
	})

	t.Run("Zero Max Attempts", func(t *testing.T) {
		proc := ProcessorFunc[int](func(_ context.Context, n int) (int, error) {
			return n * 2, nil
		})

		retry := Retry(proc, 0) // Should default to 1
		result, err := retry.Process(context.Background(), 5)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result != 10 {
			t.Errorf("expected 10, got %d", result)
		}
	})
}

func TestRetryWithBackoff(t *testing.T) {
	t.Run("Success After Backoff", func(t *testing.T) {
		attempts := 0
		attemptTimes := []time.Time{}
		proc := ProcessorFunc[int](func(_ context.Context, n int) (int, error) {
			attempts++
			attemptTimes = append(attemptTimes, time.Now())
			if attempts < 3 {
				return n, errors.New("temporary failure")
			}
			return n * 2, nil
		})

		retry := RetryWithBackoff(proc, 3, 10*time.Millisecond)
		start := time.Now()
		result, err := retry.Process(context.Background(), 5)
		elapsed := time.Since(start)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result != 10 {
			t.Errorf("expected 10, got %d", result)
		}
		if attempts != 3 {
			t.Errorf("expected 3 attempts, got %d", attempts)
		}

		// Check backoff timing
		// First retry after 10ms, second after 20ms (total 30ms minimum)
		if elapsed < 30*time.Millisecond {
			t.Errorf("expected at least 30ms elapsed, got %v", elapsed)
		}

		// Verify exponential backoff
		if len(attemptTimes) >= 2 {
			firstDelay := attemptTimes[1].Sub(attemptTimes[0])
			if firstDelay < 9*time.Millisecond {
				t.Errorf("first delay too short: %v", firstDelay)
			}
		}
		if len(attemptTimes) >= 3 {
			secondDelay := attemptTimes[2].Sub(attemptTimes[1])
			if secondDelay < 19*time.Millisecond {
				t.Errorf("second delay too short: %v", secondDelay)
			}
		}
	})

	t.Run("Context Cancellation During Backoff", func(t *testing.T) {
		attempts := 0
		proc := ProcessorFunc[int](func(_ context.Context, n int) (int, error) {
			attempts++
			return n, errors.New("failure")
		})

		retry := RetryWithBackoff(proc, 5, 100*time.Millisecond)
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		start := time.Now()
		_, err := retry.Process(ctx, 5)
		elapsed := time.Since(start)

		if err == nil || !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("expected context deadline exceeded, got: %v", err)
		}
		// Should have attempted once, then canceled during the 100ms backoff
		if attempts != 1 {
			t.Errorf("expected 1 attempt before cancellation, got %d", attempts)
		}
		if elapsed > 60*time.Millisecond {
			t.Errorf("should have canceled quickly, took %v", elapsed)
		}
	})
}

func TestTimeout(t *testing.T) {
	t.Run("Success Within Timeout", func(t *testing.T) {
		proc := ProcessorFunc[int](func(_ context.Context, n int) (int, error) {
			time.Sleep(10 * time.Millisecond)
			return n * 2, nil
		})

		timeout := Timeout(proc, 50*time.Millisecond)
		result, err := timeout.Process(context.Background(), 5)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result != 10 {
			t.Errorf("expected 10, got %d", result)
		}
	})

	t.Run("Timeout Exceeded", func(t *testing.T) {
		proc := ProcessorFunc[int](func(ctx context.Context, n int) (int, error) {
			select {
			case <-time.After(100 * time.Millisecond):
				return n * 2, nil
			case <-ctx.Done():
				return n, ctx.Err()
			}
		})

		timeout := Timeout(proc, 20*time.Millisecond)
		start := time.Now()
		result, err := timeout.Process(context.Background(), 5)
		elapsed := time.Since(start)

		if err == nil {
			t.Error("expected timeout error")
		}
		if !strings.Contains(err.Error(), "timeout after 20ms") {
			t.Errorf("unexpected error message: %v", err)
		}
		if result != 5 {
			t.Errorf("expected input value 5, got %d", result)
		}
		if elapsed > 30*time.Millisecond {
			t.Errorf("timeout took too long: %v", elapsed)
		}
	})

	t.Run("Propagate Processor Error", func(t *testing.T) {
		expectedErr := errors.New("processor error")
		proc := ProcessorFunc[int](func(_ context.Context, n int) (int, error) {
			return n, expectedErr
		})

		timeout := Timeout(proc, 50*time.Millisecond)
		result, err := timeout.Process(context.Background(), 5)
		if !errors.Is(err, expectedErr) {
			t.Errorf("expected processor error, got: %v", err)
		}
		if result != 5 {
			t.Errorf("expected 5, got %d", result)
		}
	})
}

func TestRace(t *testing.T) {
	t.Run("Empty Race", func(t *testing.T) {
		race := Race[int]()
		_, err := race.Process(context.Background(), 5)
		if err == nil || !strings.Contains(err.Error(), "no processors provided") {
			t.Errorf("expected 'no processors provided' error, got: %v", err)
		}
	})

	t.Run("First Success Wins", func(t *testing.T) {
		fast := ProcessorFunc[int](func(_ context.Context, n int) (int, error) {
			time.Sleep(10 * time.Millisecond)
			return n * 2, nil
		})
		slow := ProcessorFunc[int](func(_ context.Context, n int) (int, error) {
			time.Sleep(50 * time.Millisecond)
			return n * 3, nil
		})

		race := Race(fast, slow)
		start := time.Now()
		result, err := race.Process(context.Background(), 5)
		elapsed := time.Since(start)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result != 10 {
			t.Errorf("expected fast processor result (10), got %d", result)
		}
		// Should complete shortly after fast processor
		if elapsed > 20*time.Millisecond {
			t.Errorf("took too long: %v", elapsed)
		}
	})

	t.Run("All Fail", func(t *testing.T) {
		fail1 := ProcessorFunc[int](func(_ context.Context, n int) (int, error) {
			time.Sleep(10 * time.Millisecond)
			return n, errors.New("error 1")
		})
		fail2 := ProcessorFunc[int](func(_ context.Context, n int) (int, error) {
			time.Sleep(20 * time.Millisecond)
			return n, errors.New("error 2")
		})

		race := Race(fail1, fail2)
		result, err := race.Process(context.Background(), 5)

		if err == nil {
			t.Error("expected error when all processors fail")
		}
		if !strings.Contains(err.Error(), "all processors failed") {
			t.Errorf("unexpected error message: %v", err)
		}
		if result != 5 {
			t.Errorf("expected input value 5, got %d", result)
		}
	})

	t.Run("Context Cancellation", func(t *testing.T) {
		slow := ProcessorFunc[int](func(ctx context.Context, n int) (int, error) {
			select {
			case <-time.After(100 * time.Millisecond):
				return n * 2, nil
			case <-ctx.Done():
				return n, ctx.Err()
			}
		})

		race := Race(slow, slow)
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()

		_, err := race.Process(ctx, 5)
		if err == nil || !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("expected deadline exceeded error, got: %v", err)
		}
	})

	t.Run("Mutations Are Isolated", func(t *testing.T) {
		type Data struct {
			Value int
			Slice []int
		}

		proc1 := Apply("race1", func(_ context.Context, d Data) (Data, error) {
			time.Sleep(10 * time.Millisecond)
			d.Value = 100
			d.Slice[0] = 100
			return d, nil
		})

		proc2 := Apply("race2", func(_ context.Context, d Data) (Data, error) {
			time.Sleep(20 * time.Millisecond)
			d.Value = 200
			d.Slice[0] = 200
			return d, nil
		})

		race := Race(proc1, proc2)
		original := Data{Value: 42, Slice: []int{1, 2, 3}}
		result, err := race.Process(context.Background(), original)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		// Should get result from proc1 (faster)
		if result.Value != 100 {
			t.Errorf("expected value 100 from faster processor, got %d", result.Value)
		}
		if result.Slice[0] != 100 {
			t.Errorf("expected slice[0] = 100 from faster processor, got %d", result.Slice[0])
		}
	})
}

func TestComposition(t *testing.T) {
	t.Run("Sequential with Retry", func(t *testing.T) {
		attempts := 0
		flaky := ProcessorFunc[int](func(_ context.Context, n int) (int, error) {
			attempts++
			if attempts < 2 {
				return n, errors.New("temporary failure")
			}
			return n * 2, nil
		})
		addTen := ProcessorFunc[int](func(_ context.Context, n int) (int, error) {
			return n + 10, nil
		})

		workflow := Sequential(
			Retry(flaky, 3),
			addTen,
		)

		result, err := workflow.Process(context.Background(), 5)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result != 20 {
			t.Errorf("expected 20, got %d", result)
		}
		if attempts != 2 {
			t.Errorf("expected 2 attempts, got %d", attempts)
		}
	})

	t.Run("Fallback with Timeout", func(t *testing.T) {
		slow := ProcessorFunc[int](func(ctx context.Context, n int) (int, error) {
			select {
			case <-time.After(100 * time.Millisecond):
				return n * 2, nil
			case <-ctx.Done():
				return n, ctx.Err()
			}
		})
		fast := ProcessorFunc[int](func(_ context.Context, n int) (int, error) {
			return n * 3, nil
		})

		workflow := Fallback(
			Timeout(slow, 20*time.Millisecond),
			fast,
		)

		start := time.Now()
		result, err := workflow.Process(context.Background(), 5)
		elapsed := time.Since(start)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result != 15 {
			t.Errorf("expected fallback result 15, got %d", result)
		}
		// Should timeout quickly and use fallback
		if elapsed > 50*time.Millisecond {
			t.Errorf("took too long: %v", elapsed)
		}
	})

	t.Run("Nested Sequential", func(t *testing.T) {
		double := ProcessorFunc[int](func(_ context.Context, n int) (int, error) {
			return n * 2, nil
		})
		triple := ProcessorFunc[int](func(_ context.Context, n int) (int, error) {
			return n * 3, nil
		})
		addFive := ProcessorFunc[int](func(_ context.Context, n int) (int, error) {
			return n + 5, nil
		})

		step1 := Sequential(double, triple)
		workflow := Sequential(step1, addFive)

		result, err := workflow.Process(context.Background(), 2)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result != 17 {
			t.Errorf("expected 17, got %d", result)
		}
	})

	t.Run("Timeout Retry Combination", func(t *testing.T) {
		attemptCount := atomic.Int32{}
		proc := ProcessorFunc[int](func(_ context.Context, n int) (int, error) {
			attempt := attemptCount.Add(1)
			if attempt == 1 {
				// First attempt takes too long
				time.Sleep(50 * time.Millisecond)
			}
			return n * 2, nil
		})

		// Each attempt gets 30ms timeout, retry up to 3 times
		workflow := Retry(Timeout(proc, 30*time.Millisecond), 3)

		start := time.Now()
		result, err := workflow.Process(context.Background(), 5)
		elapsed := time.Since(start)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result != 10 {
			t.Errorf("expected 10, got %d", result)
		}
		// First attempt should timeout after 30ms, second should succeed
		if elapsed > 100*time.Millisecond {
			t.Errorf("took too long: %v", elapsed)
		}
	})
}

func TestConcurrent(t *testing.T) {
	t.Run("Empty Concurrent", func(t *testing.T) {
		concurrent := Concurrent[int]()
		result, err := concurrent.Process(context.Background(), 42)
		if err != nil {
			t.Errorf("empty concurrent should not error: %v", err)
		}
		if result != 42 {
			t.Errorf("concurrent should return input unchanged: got %d, want 42", result)
		}
	})

	t.Run("Multiple Processors Execute", func(t *testing.T) {
		var count1, count2, count3 atomic.Int32

		proc1 := Effect("counter1", func(_ context.Context, _ int) error {
			count1.Add(1)
			time.Sleep(10 * time.Millisecond)
			return nil
		})

		proc2 := Effect("counter2", func(_ context.Context, _ int) error {
			count2.Add(1)
			time.Sleep(20 * time.Millisecond)
			return nil
		})

		proc3 := Effect("counter3", func(_ context.Context, _ int) error {
			count3.Add(1)
			time.Sleep(15 * time.Millisecond)
			return nil
		})

		concurrent := Concurrent(proc1, proc2, proc3)

		start := time.Now()
		result, err := concurrent.Process(context.Background(), 100)
		elapsed := time.Since(start)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result != 100 {
			t.Errorf("concurrent should return original input: got %d, want 100", result)
		}

		// All processors should have executed
		if count1.Load() != 1 {
			t.Errorf("proc1 should have executed once: got %d", count1.Load())
		}
		if count2.Load() != 1 {
			t.Errorf("proc2 should have executed once: got %d", count2.Load())
		}
		if count3.Load() != 1 {
			t.Errorf("proc3 should have executed once: got %d", count3.Load())
		}

		// Should complete in roughly the time of the slowest processor (20ms), not the sum (45ms)
		if elapsed > 30*time.Millisecond {
			t.Errorf("concurrent execution took too long: %v", elapsed)
		}
	})

	t.Run("Mutations Are Isolated", func(t *testing.T) {
		type Data struct {
			Value int
			Slice []int
		}

		proc1 := Apply("mutate1", func(_ context.Context, d Data) (Data, error) {
			d.Value = 999
			d.Slice[0] = 999
			return d, nil
		})

		proc2 := Apply("mutate2", func(_ context.Context, d Data) (Data, error) {
			d.Value = 777
			d.Slice[0] = 777
			return d, nil
		})

		concurrent := Concurrent(proc1, proc2)

		original := Data{Value: 42, Slice: []int{1, 2, 3}}
		result, err := concurrent.Process(context.Background(), original)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		// Original should be unchanged
		if result.Value != 42 {
			t.Errorf("original value should be unchanged: got %d, want 42", result.Value)
		}
		if result.Slice[0] != 1 {
			t.Errorf("original slice should be unchanged: got %d, want 1", result.Slice[0])
		}
	})

	t.Run("Errors Are Ignored", func(t *testing.T) {
		var executed atomic.Bool

		failing := Effect("fail", func(_ context.Context, _ int) error {
			return errors.New("this processor fails")
		})

		succeeding := Effect("succeed", func(_ context.Context, _ int) error {
			executed.Store(true)
			return nil
		})

		concurrent := Concurrent(failing, succeeding)

		result, err := concurrent.Process(context.Background(), 42)

		// Should not return error
		if err != nil {
			t.Errorf("concurrent should not return errors from processors: %v", err)
		}
		if result != 42 {
			t.Errorf("concurrent should return original input: got %d, want 42", result)
		}
		if !executed.Load() {
			t.Errorf("succeeding processor should have executed despite failing processor")
		}
	})

	t.Run("Context Cancellation", func(t *testing.T) {
		var started, completed atomic.Int32

		slowProc := Effect("slow", func(ctx context.Context, _ int) error {
			started.Add(1)
			select {
			case <-time.After(100 * time.Millisecond):
				completed.Add(1)
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})

		concurrent := Concurrent(slowProc, slowProc, slowProc)

		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()

		_, err := concurrent.Process(ctx, 42)

		if err == nil || !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("expected deadline exceeded error, got: %v", err)
		}

		// Give goroutines time to notice cancellation
		time.Sleep(10 * time.Millisecond)

		// All should have started
		if started.Load() != 3 {
			t.Errorf("all processors should have started: got %d", started.Load())
		}
		// None should have completed
		if completed.Load() != 0 {
			t.Errorf("no processors should have completed: got %d", completed.Load())
		}
	})
}

func TestWithErrorHandler(t *testing.T) {
	t.Run("Success Case - No Error Handler Called", func(t *testing.T) {
		var errorHandlerCalled atomic.Bool

		proc := Apply("succeed", func(_ context.Context, n int) (int, error) {
			return n * 2, nil
		})

		errorHandler := Effect("error_handler", func(_ context.Context, _ error) error {
			errorHandlerCalled.Store(true)
			return nil
		})

		wrapped := WithErrorHandler(proc, errorHandler)
		result, err := wrapped.Process(context.Background(), 5)

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result != 10 {
			t.Errorf("expected 10, got %d", result)
		}
		if errorHandlerCalled.Load() {
			t.Errorf("error handler should not be called on success")
		}
	})

	t.Run("Error Case - Handler Called", func(t *testing.T) {
		var capturedError error
		expectedErr := errors.New("processing failed")

		proc := Apply("fail", func(_ context.Context, n int) (int, error) {
			return n, expectedErr
		})

		errorHandler := Effect("error_handler", func(_ context.Context, err error) error {
			capturedError = err
			return nil
		})

		wrapped := WithErrorHandler(proc, errorHandler)
		result, err := wrapped.Process(context.Background(), 5)

		// Original error should be returned
		if !errors.Is(err, expectedErr) {
			t.Errorf("expected original error %v, got %v", expectedErr, err)
		}
		if result != 5 {
			t.Errorf("expected result to pass through: got %d, want 5", result)
		}
		// Error handler should receive the error
		if !errors.Is(capturedError, expectedErr) {
			t.Errorf("error handler received wrong error: got %v, want %v", capturedError, expectedErr)
		}
	})

	t.Run("Error Handler Failure Is Ignored", func(t *testing.T) {
		originalErr := errors.New("original error")

		proc := Apply("fail", func(_ context.Context, n int) (int, error) {
			return n, originalErr
		})

		errorHandler := Effect("error_handler", func(_ context.Context, _ error) error {
			return errors.New("error handler also failed")
		})

		wrapped := WithErrorHandler(proc, errorHandler)
		result, err := wrapped.Process(context.Background(), 5)

		// Should still return the original error, not the handler's error
		if !errors.Is(err, originalErr) {
			t.Errorf("expected original error %v, got %v", originalErr, err)
		}
		if result != 5 {
			t.Errorf("expected result to pass through: got %d, want 5", result)
		}
	})

	t.Run("With Concurrent", func(t *testing.T) {
		var collectedErrors []string
		var mu sync.Mutex

		proc1 := Effect("fail1", func(_ context.Context, _ int) error {
			return errors.New("error from proc1")
		})

		proc2 := Effect("fail2", func(_ context.Context, _ int) error {
			return errors.New("error from proc2")
		})

		errorCollector := Effect("collect", func(_ context.Context, err error) error {
			mu.Lock()
			collectedErrors = append(collectedErrors, err.Error())
			mu.Unlock()
			return nil
		})

		concurrent := Concurrent(
			WithErrorHandler(proc1, errorCollector),
			WithErrorHandler(proc2, errorCollector),
		)

		_, err := concurrent.Process(context.Background(), 42)

		if err != nil {
			t.Errorf("concurrent should not return error: %v", err)
		}

		// Wait a bit for error handlers to complete
		time.Sleep(10 * time.Millisecond)

		mu.Lock()
		defer mu.Unlock()
		if len(collectedErrors) != 2 {
			t.Errorf("expected 2 errors collected, got %d: %v", len(collectedErrors), collectedErrors)
		}
	})
}
