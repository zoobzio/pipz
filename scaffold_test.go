package pipz

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestScaffold(t *testing.T) {
	t.Run("Hooks fire on launch events", func(t *testing.T) {
		var launchedEvents []ScaffoldEvent
		var allLaunchedEvents []ScaffoldEvent
		var mu sync.Mutex

		effect1 := Effect("processor1", func(_ context.Context, _ TestData) error {
			time.Sleep(50 * time.Millisecond)
			return nil
		})
		effect2 := Effect("processor2", func(_ context.Context, _ TestData) error {
			time.Sleep(50 * time.Millisecond)
			return nil
		})
		effect3 := Effect("processor3", func(_ context.Context, _ TestData) error {
			time.Sleep(50 * time.Millisecond)
			return nil
		})

		scaffold := NewScaffold("test-scaffold", effect1, effect2, effect3)
		defer scaffold.Close()

		// Register hooks
		scaffold.OnLaunched(func(_ context.Context, event ScaffoldEvent) error {
			mu.Lock()
			launchedEvents = append(launchedEvents, event)
			mu.Unlock()
			return nil
		})

		scaffold.OnAllLaunched(func(_ context.Context, event ScaffoldEvent) error {
			mu.Lock()
			allLaunchedEvents = append(allLaunchedEvents, event)
			mu.Unlock()
			return nil
		})

		// Process to trigger launches
		data := TestData{Value: 42}
		result, err := scaffold.Process(context.Background(), data)

		// Should return immediately
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Value != 42 {
			t.Errorf("expected 42, got %d", result.Value)
		}

		// Wait a bit for hooks (they should be synchronous but may have mutex contention)
		time.Sleep(10 * time.Millisecond)

		mu.Lock()
		defer mu.Unlock()

		// Check launched events
		if len(launchedEvents) != 3 {
			t.Errorf("expected 3 launched events, got %d", len(launchedEvents))
		}

		expectedProcessors := map[string]bool{
			"processor1": false,
			"processor2": false,
			"processor3": false,
		}

		for i, event := range launchedEvents {
			if event.Name != "test-scaffold" {
				t.Errorf("event %d: expected name 'test-scaffold', got %s", i, event.Name)
			}
			if event.ProcessorCount != 3 {
				t.Errorf("event %d: expected processor count 3, got %d", i, event.ProcessorCount)
			}
			if event.ProcessorIndex < 0 || event.ProcessorIndex > 2 {
				t.Errorf("event %d: expected processor index 0-2, got %d", i, event.ProcessorIndex)
			}

			processorName := event.ProcessorName
			if _, ok := expectedProcessors[string(processorName)]; !ok {
				t.Errorf("event %d: unexpected processor name %s", i, processorName)
			} else {
				expectedProcessors[string(processorName)] = true
			}
		}

		// Verify all processors were launched
		for name, launched := range expectedProcessors {
			if !launched {
				t.Errorf("processor %s was not launched", name)
			}
		}

		// Check all launched event
		if len(allLaunchedEvents) != 1 {
			t.Errorf("expected 1 all launched event, got %d", len(allLaunchedEvents))
		}
		if len(allLaunchedEvents) > 0 {
			event := allLaunchedEvents[0]
			if event.Name != "test-scaffold" {
				t.Errorf("expected name 'test-scaffold', got %s", event.Name)
			}
			if event.ProcessorCount != 3 {
				t.Errorf("expected processor count 3, got %d", event.ProcessorCount)
			}
		}
	})

	t.Run("Hooks with empty processors", func(t *testing.T) {
		var allLaunchedEvents []ScaffoldEvent
		var mu sync.Mutex

		scaffold := NewScaffold[TestData]("empty-scaffold")
		defer scaffold.Close()

		scaffold.OnAllLaunched(func(_ context.Context, event ScaffoldEvent) error {
			mu.Lock()
			allLaunchedEvents = append(allLaunchedEvents, event)
			mu.Unlock()
			return nil
		})

		data := TestData{Value: 42}
		result, err := scaffold.Process(context.Background(), data)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Value != 42 {
			t.Errorf("expected 42, got %d", result.Value)
		}

		// No hooks should fire for empty scaffold
		time.Sleep(10 * time.Millisecond)

		mu.Lock()
		defer mu.Unlock()

		if len(allLaunchedEvents) != 0 {
			t.Errorf("expected no all launched events for empty scaffold, got %d", len(allLaunchedEvents))
		}
	})
	t.Run("Fire and Forget Behavior", func(t *testing.T) {
		var counter int32
		effect := Effect("increment", func(_ context.Context, _ TestData) error {
			// Simulate some work
			time.Sleep(50 * time.Millisecond)
			atomic.AddInt32(&counter, 1)
			return nil
		})

		scaffold := NewScaffold("test-scaffold", effect, effect, effect)
		data := TestData{Value: 1}

		start := time.Now()
		result, err := scaffold.Process(context.Background(), data)
		elapsed := time.Since(start)

		// Should return immediately
		if elapsed > 10*time.Millisecond {
			t.Errorf("Scaffold should return immediately, took %v", elapsed)
		}

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Should return original input unchanged
		if result.Value != 1 {
			t.Errorf("expected original value 1, got %d", result.Value)
		}

		// Counter should still be 0 immediately after return
		if c := atomic.LoadInt32(&counter); c != 0 {
			t.Errorf("expected counter to be 0 immediately, got %d", c)
		}

		// Wait for background processors to complete
		time.Sleep(100 * time.Millisecond)

		// All three should have executed
		if c := atomic.LoadInt32(&counter); c != 3 {
			t.Errorf("expected counter to be 3 after waiting, got %d", c)
		}
	})

	t.Run("Context Isolation", func(t *testing.T) {
		var executed int32
		blocker := Effect("block", func(ctx context.Context, _ TestData) error {
			select {
			case <-time.After(100 * time.Millisecond):
				atomic.AddInt32(&executed, 1)
				return nil
			case <-ctx.Done():
				// Should not happen - context should be isolated
				return ctx.Err()
			}
		})

		scaffold := NewScaffold("test", blocker)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()

		data := TestData{Value: 1}
		_, err := scaffold.Process(ctx, data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Wait for parent context to be canceled
		<-ctx.Done()

		// Wait for processor to complete despite parent cancellation
		time.Sleep(150 * time.Millisecond)

		if e := atomic.LoadInt32(&executed); e != 1 {
			t.Errorf("processor should have completed despite parent cancellation, executed=%d", e)
		}
	})

	t.Run("Empty Processors", func(t *testing.T) {
		scaffold := NewScaffold[TestData]("empty")
		data := TestData{Value: 42}

		result, err := scaffold.Process(context.Background(), data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Value != 42 {
			t.Errorf("expected 42, got %d", result.Value)
		}
	})

	t.Run("Configuration Methods", func(t *testing.T) {
		p1 := Transform("p1", func(_ context.Context, d TestData) TestData { return d })
		p2 := Transform("p2", func(_ context.Context, d TestData) TestData { return d })
		p3 := Transform("p3", func(_ context.Context, d TestData) TestData { return d })

		scaffold := NewScaffold("test", p1, p2)

		if scaffold.Len() != 2 {
			t.Errorf("expected 2 processors, got %d", scaffold.Len())
		}

		scaffold.Add(p3)
		if scaffold.Len() != 3 {
			t.Errorf("expected 3 processors after add, got %d", scaffold.Len())
		}

		err := scaffold.Remove(1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if scaffold.Len() != 2 {
			t.Errorf("expected 2 processors after remove, got %d", scaffold.Len())
		}

		scaffold.SetProcessors(p1, p2, p3)
		if scaffold.Len() != 3 {
			t.Errorf("expected 3 processors after set, got %d", scaffold.Len())
		}

		scaffold.Clear()
		if scaffold.Len() != 0 {
			t.Errorf("expected 0 processors after clear, got %d", scaffold.Len())
		}
	})

	t.Run("Name Method", func(t *testing.T) {
		scaffold := NewScaffold[TestData]("test-name")
		if scaffold.Name() != "test-name" {
			t.Errorf("expected name 'test-name', got '%s'", scaffold.Name())
		}
	})

	t.Run("Remove Out of Bounds", func(t *testing.T) {
		scaffold := NewScaffold[TestData]("test")

		err := scaffold.Remove(-1)
		if !errors.Is(err, ErrIndexOutOfBounds) {
			t.Errorf("expected ErrIndexOutOfBounds for negative index, got %v", err)
		}

		err = scaffold.Remove(0)
		if !errors.Is(err, ErrIndexOutOfBounds) {
			t.Errorf("expected ErrIndexOutOfBounds for empty scaffold, got %v", err)
		}

		scaffold.Add(Transform("p1", func(_ context.Context, d TestData) TestData { return d }))
		err = scaffold.Remove(1)
		if !errors.Is(err, ErrIndexOutOfBounds) {
			t.Errorf("expected ErrIndexOutOfBounds for index >= length, got %v", err)
		}
	})

	t.Run("Trace Context Preservation", func(t *testing.T) {
		type contextKey string
		const traceKey contextKey = "trace-id"

		capturedChan := make(chan string, 1)
		tracer := Effect("trace", func(ctx context.Context, _ TestData) error {
			if id, ok := ctx.Value(traceKey).(string); ok {
				capturedChan <- id
			}
			return nil
		})

		scaffold := NewScaffold("test", tracer)
		ctx := context.WithValue(context.Background(), traceKey, "test-trace-123")

		_, err := scaffold.Process(ctx, TestData{Value: 1})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Wait for background processor
		select {
		case capturedTraceID := <-capturedChan:
			if capturedTraceID != "test-trace-123" {
				t.Errorf("expected trace ID 'test-trace-123', got '%s'", capturedTraceID)
			}
		case <-time.After(100 * time.Millisecond):
			t.Error("timeout waiting for trace ID")
		}
	})

	t.Run("Scaffold panic recovery", func(t *testing.T) {
		// Scaffold is fire-and-forget, so panics happen in background goroutines
		// We test that the main process doesn't panic and returns original data
		panicEffect := Effect("panic_effect", func(_ context.Context, _ TestData) error {
			panic("scaffold panic")
		})

		scaffold := NewScaffold("panic_scaffold", panicEffect)

		original := TestData{Value: 42}
		result, err := scaffold.Process(context.Background(), original)

		// Scaffold should return original data unchanged (fire-and-forget)
		if err != nil {
			t.Errorf("expected no error from scaffold process, got %v", err)
		}

		if result != original {
			t.Errorf("expected original data %+v, got %+v", original, result)
		}

		// The panic happens in a background goroutine, so we can't directly test it
		// But we can verify that the main process completes successfully
		// This test ensures panic recovery in the main process works correctly
	})
}
