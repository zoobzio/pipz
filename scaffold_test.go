package pipz

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestScaffold(t *testing.T) {
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
}
