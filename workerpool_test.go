package pipz

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestWorkerPool(t *testing.T) {
	t.Run("Runs All Processors", func(t *testing.T) {
		var counter int32
		data := TestData{Value: 1, Counter: &counter}

		p1 := Effect("inc1", func(_ context.Context, d TestData) error {
			atomic.AddInt32(d.Counter, 1)
			return nil
		})
		p2 := Effect("inc2", func(_ context.Context, d TestData) error {
			atomic.AddInt32(d.Counter, 10)
			return nil
		})
		p3 := Effect("inc3", func(_ context.Context, d TestData) error {
			atomic.AddInt32(d.Counter, 100)
			return nil
		})

		pool := NewWorkerPool("test-pool", 2, p1, p2, p3)
		result, err := pool.Process(context.Background(), data)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Original data should be returned unchanged
		if result.Value != 1 {
			t.Errorf("expected original value 1, got %d", result.Value)
		}
		// All processors should have run
		if atomic.LoadInt32(&counter) != 111 {
			t.Errorf("expected counter 111, got %d", atomic.LoadInt32(&counter))
		}
	})

	t.Run("Worker Pool Limits Parallelism", func(t *testing.T) {
		const workers = 2
		const totalProcessors = 5

		var activeWorkers int32
		var maxConcurrent int32
		var mu sync.Mutex

		makeProcessor := func(_ int) Chainable[TestData] {
			return Effect("proc", func(_ context.Context, _ TestData) error {
				// Track concurrent workers
				current := atomic.AddInt32(&activeWorkers, 1)

				// Update max seen concurrency
				mu.Lock()
				if current > maxConcurrent {
					maxConcurrent = current
				}
				mu.Unlock()

				// Simulate work
				time.Sleep(50 * time.Millisecond)

				// Decrement active workers
				atomic.AddInt32(&activeWorkers, -1)
				return nil
			})
		}

		var processors []Chainable[TestData]
		for i := 0; i < totalProcessors; i++ {
			processors = append(processors, makeProcessor(i))
		}

		pool := NewWorkerPool("test-pool", workers, processors...)
		data := TestData{Value: 1}

		_, err := pool.Process(context.Background(), data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		mu.Lock()
		observedMax := maxConcurrent
		mu.Unlock()

		// Should never exceed worker limit
		if observedMax > int32(workers) {
			t.Errorf("expected max concurrent workers <= %d, got %d", workers, observedMax)
		}

		// Should actually use multiple workers (not serialize everything)
		if observedMax < 2 {
			t.Errorf("expected to use at least 2 workers, only used %d", observedMax)
		}
	})

	t.Run("Semaphore Pattern Verification", func(t *testing.T) {
		const workers = 3

		// Use a channel to coordinate precise timing
		startChan := make(chan struct{})
		var startOnce sync.Once

		var activeCount int32
		var measurements []int32
		var measurementsMu sync.Mutex

		makeProcessor := func(_ int) Chainable[TestData] {
			return Effect("proc", func(_ context.Context, _ TestData) error {
				// Signal first processor to start measurements
				startOnce.Do(func() { close(startChan) })

				current := atomic.AddInt32(&activeCount, 1)

				// Take measurement
				measurementsMu.Lock()
				measurements = append(measurements, current)
				measurementsMu.Unlock()

				// Hold the semaphore slot for measurement
				time.Sleep(100 * time.Millisecond)

				atomic.AddInt32(&activeCount, -1)
				return nil
			})
		}

		processors := make([]Chainable[TestData], 6) // More than workers
		for i := range processors {
			processors[i] = makeProcessor(i)
		}

		pool := NewWorkerPool("semaphore-test", workers, processors...)

		go func() {
			<-startChan
			// Sample active count during execution
			ticker := time.NewTicker(10 * time.Millisecond)
			defer ticker.Stop()

			for i := 0; i < 20; i++ { // Sample for 200ms
				<-ticker.C
				current := atomic.LoadInt32(&activeCount)
				measurementsMu.Lock()
				measurements = append(measurements, current)
				measurementsMu.Unlock()
			}
		}()

		_, err := pool.Process(context.Background(), TestData{Value: 1})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		measurementsMu.Lock()
		allMeasurements := make([]int32, len(measurements))
		copy(allMeasurements, measurements)
		measurementsMu.Unlock()

		// Verify semaphore never allows more than worker limit
		for _, count := range allMeasurements {
			if count > int32(workers) {
				t.Errorf("semaphore failed: observed %d concurrent workers, limit is %d", count, workers)
			}
		}

		// Verify we actually used the full capacity at some point
		maxObserved := int32(0)
		for _, count := range allMeasurements {
			if count > maxObserved {
				maxObserved = count
			}
		}
		if maxObserved < int32(workers) {
			t.Errorf("expected to reach worker limit %d, max observed was %d", workers, maxObserved)
		}
	})

	t.Run("Error Handling", func(t *testing.T) {
		testErr := errors.New("test error")

		successProcessor := Effect("success", func(_ context.Context, d TestData) error {
			atomic.AddInt32(d.Counter, 1)
			return nil
		})

		errorProcessor := Effect("error", func(_ context.Context, _ TestData) error {
			return testErr
		})

		var counter int32
		data := TestData{Value: 1, Counter: &counter}

		pool := NewWorkerPool("error-test", 2, successProcessor, errorProcessor, successProcessor)
		result, err := pool.Process(context.Background(), data)

		if err == nil {
			t.Fatal("expected error, got nil")
		}

		// Should return original input on error
		if result.Value != 1 {
			t.Errorf("expected original value 1 on error, got %d", result.Value)
		}

		// Error should be wrapped properly
		var pipzErr *Error[TestData]
		if !errors.As(err, &pipzErr) {
			t.Errorf("expected pipz Error, got %T: %v", err, err)
		} else {
			if !errors.Is(pipzErr.Err, testErr) {
				t.Errorf("expected wrapped test error, got %v", pipzErr.Err)
			}
			if len(pipzErr.Path) == 0 || pipzErr.Path[0] != "error-test" {
				t.Errorf("expected path with 'error-test', got %v", pipzErr.Path)
			}
		}

		// All processors should still have a chance to run
		// Wait briefly for remaining processors
		time.Sleep(50 * time.Millisecond)

		// Counter should show that successful processors ran
		finalCount := atomic.LoadInt32(&counter)
		if finalCount == 0 {
			t.Error("expected at least some successful processors to run")
		}
	})

	t.Run("Context Cancellation", func(t *testing.T) {
		var started, canceled int32

		blocker := Effect("block", func(ctx context.Context, _ TestData) error {
			atomic.AddInt32(&started, 1)
			select {
			case <-time.After(200 * time.Millisecond):
				return nil
			case <-ctx.Done():
				atomic.AddInt32(&canceled, 1)
				return ctx.Err()
			}
		})

		pool := NewWorkerPool("cancel-test", 2, blocker, blocker, blocker)
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		data := TestData{Value: 1}
		_, err := pool.Process(ctx, data)

		// Should get an error due to context cancellation
		if err == nil {
			t.Fatal("expected context cancellation error, got nil")
		}

		// Wait for cleanup
		time.Sleep(100 * time.Millisecond)

		startedCount := atomic.LoadInt32(&started)
		canceledCount := atomic.LoadInt32(&canceled)

		// Some processors should have started
		if startedCount == 0 {
			t.Error("expected at least one processor to start")
		}

		// Some should have been canceled
		if canceledCount == 0 {
			t.Error("expected at least one processor to be canceled")
		}
	})

	t.Run("Empty Processors", func(t *testing.T) {
		pool := NewWorkerPool[TestData]("empty", 5)
		data := TestData{Value: 42}

		result, err := pool.Process(context.Background(), data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Value != 42 {
			t.Errorf("expected 42, got %d", result.Value)
		}
	})

	t.Run("Worker Count Configuration", func(t *testing.T) {
		pool := NewWorkerPool[TestData]("config-test", 3)

		if pool.GetWorkerCount() != 3 {
			t.Errorf("expected worker count 3, got %d", pool.GetWorkerCount())
		}

		pool.SetWorkerCount(5)
		if pool.GetWorkerCount() != 5 {
			t.Errorf("expected worker count 5 after update, got %d", pool.GetWorkerCount())
		}

		// Invalid worker count should be ignored
		pool.SetWorkerCount(0)
		if pool.GetWorkerCount() != 5 {
			t.Errorf("expected worker count to remain 5 after invalid update, got %d", pool.GetWorkerCount())
		}

		pool.SetWorkerCount(-1)
		if pool.GetWorkerCount() != 5 {
			t.Errorf("expected worker count to remain 5 after negative update, got %d", pool.GetWorkerCount())
		}
	})

	t.Run("Active Workers Tracking", func(t *testing.T) {
		const workers = 3

		// Use channels to precisely control timing
		proceedChan := make(chan struct{})
		var activeChecks []int
		var mu sync.Mutex

		blocker := Effect("block", func(_ context.Context, _ TestData) error {
			<-proceedChan // Wait for signal
			return nil
		})

		pool := NewWorkerPool("active-test", workers, blocker, blocker, blocker, blocker, blocker)

		// Start processing in background
		done := make(chan error, 1)
		go func() {
			_, err := pool.Process(context.Background(), TestData{Value: 1})
			done <- err
		}()

		// Give time for workers to acquire semaphore slots
		time.Sleep(50 * time.Millisecond)

		// Check active workers while they're blocked
		for i := 0; i < 5; i++ {
			active := pool.GetActiveWorkers()
			mu.Lock()
			activeChecks = append(activeChecks, active)
			mu.Unlock()
			time.Sleep(10 * time.Millisecond)
		}

		// Release processors
		close(proceedChan)

		// Wait for completion
		if err := <-done; err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Should have seen workers <= limit
		mu.Lock()
		checks := make([]int, len(activeChecks))
		copy(checks, activeChecks)
		mu.Unlock()

		for i, active := range checks {
			if active > workers {
				t.Errorf("check %d: active workers %d exceeded limit %d", i, active, workers)
			}
		}

		// Should have seen active workers during processing
		maxActive := 0
		for _, active := range checks {
			if active > maxActive {
				maxActive = active
			}
		}
		if maxActive == 0 {
			t.Error("expected to see active workers during processing")
		}

		// After completion, active workers should be 0
		time.Sleep(10 * time.Millisecond)
		finalActive := pool.GetActiveWorkers()
		if finalActive != 0 {
			t.Errorf("expected 0 active workers after completion, got %d", finalActive)
		}
	})

	t.Run("Configuration Methods", func(t *testing.T) {
		p1 := Transform("p1", func(_ context.Context, d TestData) TestData { return d })
		p2 := Transform("p2", func(_ context.Context, d TestData) TestData { return d })
		p3 := Transform("p3", func(_ context.Context, d TestData) TestData { return d })

		pool := NewWorkerPool("test", 2, p1, p2)

		if pool.Len() != 2 {
			t.Errorf("expected 2 processors, got %d", pool.Len())
		}

		pool.Add(p3)
		if pool.Len() != 3 {
			t.Errorf("expected 3 processors after add, got %d", pool.Len())
		}

		err := pool.Remove(1)
		if err != nil {
			t.Fatalf("unexpected error removing processor: %v", err)
		}
		if pool.Len() != 2 {
			t.Errorf("expected 2 processors after remove, got %d", pool.Len())
		}

		pool.Clear()
		if pool.Len() != 0 {
			t.Errorf("expected 0 processors after clear, got %d", pool.Len())
		}

		pool.SetProcessors(p1, p2, p3)
		if pool.Len() != 3 {
			t.Errorf("expected 3 processors after SetProcessors, got %d", pool.Len())
		}
	})

	t.Run("Name Method", func(t *testing.T) {
		pool := NewWorkerPool[TestData]("my-worker-pool", 3)
		if pool.Name() != "my-worker-pool" {
			t.Errorf("expected 'my-worker-pool', got %q", pool.Name())
		}
	})

	t.Run("Remove Out of Bounds", func(t *testing.T) {
		p1 := Transform("p1", func(_ context.Context, d TestData) TestData { return d })
		pool := NewWorkerPool("test", 2, p1)

		// Test negative index
		err := pool.Remove(-1)
		if err == nil {
			t.Error("expected error for negative index")
		}
		if !errors.Is(err, ErrIndexOutOfBounds) {
			t.Errorf("expected ErrIndexOutOfBounds, got %v", err)
		}

		// Test index >= length
		err = pool.Remove(1)
		if err == nil {
			t.Error("expected error for index >= length")
		}
		if !errors.Is(err, ErrIndexOutOfBounds) {
			t.Errorf("expected ErrIndexOutOfBounds, got %v", err)
		}
	})

	t.Run("Timeout Configuration", func(t *testing.T) {
		var executed int32

		// Processor that respects context cancellation for timeout
		slowProcessor := Effect("slow", func(ctx context.Context, _ TestData) error {
			atomic.AddInt32(&executed, 1)
			select {
			case <-time.After(100 * time.Millisecond):
				return nil // Would complete after 100ms
			case <-ctx.Done():
				return ctx.Err() // Canceled by timeout
			}
		})

		pool := NewWorkerPool("timeout-test", 2, slowProcessor, slowProcessor)
		pool.WithTimeout(50 * time.Millisecond)

		start := time.Now()
		_, err := pool.Process(context.Background(), TestData{Value: 1})
		elapsed := time.Since(start)

		// Should timeout and return error
		if err == nil {
			t.Fatal("expected timeout error, got nil")
		}

		// Should not take the full processor time
		if elapsed > 80*time.Millisecond {
			t.Errorf("expected timeout around 50ms, took %v", elapsed)
		}

		// Processors should have started but been canceled
		if count := atomic.LoadInt32(&executed); count == 0 {
			t.Error("expected processors to start before timeout")
		}
	})

	t.Run("Zero Workers Default", func(t *testing.T) {
		pool := NewWorkerPool[TestData]("zero-test", 0)
		if pool.GetWorkerCount() != 1 {
			t.Errorf("expected default worker count 1 for zero input, got %d", pool.GetWorkerCount())
		}

		pool2 := NewWorkerPool[TestData]("negative-test", -5)
		if pool2.GetWorkerCount() != 1 {
			t.Errorf("expected default worker count 1 for negative input, got %d", pool2.GetWorkerCount())
		}
	})

	t.Run("Thread Safety", func(_ *testing.T) {
		pool := NewWorkerPool[TestData]("thread-test", 5)

		var wg sync.WaitGroup

		// Concurrent modifications
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(_ int) {
				defer wg.Done()
				p := Transform("proc", func(_ context.Context, d TestData) TestData { return d })
				pool.Add(p)

				if pool.Len() > 0 {
					_ = pool.Remove(0) //nolint:errcheck // May fail in concurrent access, that's expected
				}

				_ = pool.Name()
				_ = pool.GetWorkerCount()
				_ = pool.GetActiveWorkers()
			}(i)
		}

		// Concurrent processing
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, _ = pool.Process(context.Background(), TestData{Value: 1}) //nolint:errcheck // Testing, errors not needed
			}()
		}

		wg.Wait()
		// If we get here without race conditions, the test passes
	})
}

// Benchmark tests for performance verification.
func BenchmarkWorkerPool(b *testing.B) {
	processor := Transform("bench", func(_ context.Context, d TestData) TestData {
		// Simulate minimal work
		d.Value++
		return d
	})

	b.Run("SingleWorker", func(b *testing.B) {
		pool := NewWorkerPool("bench-1", 1, processor)
		data := TestData{Value: 1}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = pool.Process(context.Background(), data) //nolint:errcheck // Testing, errors not needed
		}
	})

	b.Run("MultipleWorkers", func(b *testing.B) {
		processors := make([]Chainable[TestData], 10)
		for i := range processors {
			processors[i] = processor
		}

		pool := NewWorkerPool("bench-multi", 4, processors...)
		data := TestData{Value: 1}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = pool.Process(context.Background(), data) //nolint:errcheck // Testing, errors not needed
		}
	})

	b.Run("WorkerPoolCreation", func(b *testing.B) {
		processors := make([]Chainable[TestData], 5)
		for i := range processors {
			processors[i] = processor
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = NewWorkerPool("bench-create", 3, processors...)
		}
	})

	b.Run("SemaphoreOverhead", func(b *testing.B) {
		fastProcessor := Effect("fast", func(_ context.Context, _ TestData) error { return nil })
		pool := NewWorkerPool("overhead", 100, fastProcessor) // High worker limit
		data := TestData{Value: 1}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = pool.Process(context.Background(), data) //nolint:errcheck // Testing, errors not needed
		}
	})
}
