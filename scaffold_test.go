package pipz

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/zoobzio/capitan"
)

func TestScaffold(t *testing.T) {
	t.Run("Fire and Forget Behavior", func(t *testing.T) {
		var counter int32
		effect := Effect(NewIdentity("increment", ""), func(_ context.Context, _ TestData) error {
			// Simulate some work
			time.Sleep(50 * time.Millisecond)
			atomic.AddInt32(&counter, 1)
			return nil
		})

		scaffold := NewScaffold(NewIdentity("test-scaffold", ""), effect, effect, effect)
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
		blocker := Effect(NewIdentity("block", ""), func(ctx context.Context, _ TestData) error {
			select {
			case <-time.After(100 * time.Millisecond):
				atomic.AddInt32(&executed, 1)
				return nil
			case <-ctx.Done():
				// Should not happen - context should be isolated
				return ctx.Err()
			}
		})

		scaffold := NewScaffold(NewIdentity("test", ""), blocker)
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
		scaffold := NewScaffold[TestData](NewIdentity("empty", ""))
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
		p1 := Transform(NewIdentity("p1", ""), func(_ context.Context, d TestData) TestData { return d })
		p2 := Transform(NewIdentity("p2", ""), func(_ context.Context, d TestData) TestData { return d })
		p3 := Transform(NewIdentity("p3", ""), func(_ context.Context, d TestData) TestData { return d })

		scaffold := NewScaffold(NewIdentity("test", ""), p1, p2)

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
		scaffold := NewScaffold[TestData](NewIdentity("test-name", ""))
		if scaffold.Identity().Name() != "test-name" {
			t.Errorf("expected name 'test-name', got '%s'", scaffold.Identity().Name())
		}
	})

	t.Run("Remove Out of Bounds", func(t *testing.T) {
		scaffold := NewScaffold[TestData](NewIdentity("test", ""))

		err := scaffold.Remove(-1)
		if !errors.Is(err, ErrIndexOutOfBounds) {
			t.Errorf("expected ErrIndexOutOfBounds for negative index, got %v", err)
		}

		err = scaffold.Remove(0)
		if !errors.Is(err, ErrIndexOutOfBounds) {
			t.Errorf("expected ErrIndexOutOfBounds for empty scaffold, got %v", err)
		}

		scaffold.Add(Transform(NewIdentity("p1", ""), func(_ context.Context, d TestData) TestData { return d }))
		err = scaffold.Remove(1)
		if !errors.Is(err, ErrIndexOutOfBounds) {
			t.Errorf("expected ErrIndexOutOfBounds for index >= length, got %v", err)
		}
	})

	t.Run("Trace Context Preservation", func(t *testing.T) {
		type contextKey string
		const traceKey contextKey = "trace-id"

		capturedChan := make(chan string, 1)
		tracer := Effect(NewIdentity("trace", ""), func(ctx context.Context, _ TestData) error {
			if id, ok := ctx.Value(traceKey).(string); ok {
				capturedChan <- id
			}
			return nil
		})

		scaffold := NewScaffold(NewIdentity("test", ""), tracer)
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
		panicEffect := Effect(NewIdentity("panic_effect", ""), func(_ context.Context, _ TestData) error {
			panic("scaffold panic")
		})

		scaffold := NewScaffold(NewIdentity("panic_scaffold", ""), panicEffect)

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

func TestScaffoldClose(t *testing.T) {
	t.Run("Closes All Children", func(t *testing.T) {
		p1 := newTrackingProcessor[TestData](NewIdentity("p1", ""))
		p2 := newTrackingProcessor[TestData](NewIdentity("p2", ""))

		s := NewScaffold(NewIdentity("test", ""), p1, p2)
		err := s.Close()

		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if p1.CloseCalls() != 1 || p2.CloseCalls() != 1 {
			t.Error("expected all processors to be closed")
		}
	})

	t.Run("Aggregates Errors", func(t *testing.T) {
		p1 := newTrackingProcessor[TestData](NewIdentity("p1", "")).WithCloseError(errors.New("p1 error"))
		p2 := newTrackingProcessor[TestData](NewIdentity("p2", "")).WithCloseError(errors.New("p2 error"))

		s := NewScaffold(NewIdentity("test", ""), p1, p2)
		err := s.Close()

		if err == nil {
			t.Error("expected error")
		}
		if p1.CloseCalls() != 1 || p2.CloseCalls() != 1 {
			t.Error("expected all processors to be closed")
		}
	})

	t.Run("Idempotency", func(t *testing.T) {
		p := newTrackingProcessor[TestData](NewIdentity("p", ""))
		s := NewScaffold(NewIdentity("test", ""), p)

		_ = s.Close()
		_ = s.Close()

		if p.CloseCalls() != 1 {
			t.Errorf("expected 1 close call, got %d", p.CloseCalls())
		}
	})
}

func TestScaffoldSignals(t *testing.T) {
	t.Run("Emits Dispatched Signal", func(t *testing.T) {
		var signalReceived bool
		var signalName string
		var signalProcessorCount int

		listener := capitan.Hook(SignalScaffoldDispatched, func(_ context.Context, e *capitan.Event) {
			signalReceived = true
			signalName, _ = FieldName.From(e)
			signalProcessorCount, _ = FieldProcessorCount.From(e)
		})
		defer listener.Close()

		scaffold := NewScaffold[TestData](NewIdentity("signal-test-scaffold", ""),
			Effect(NewIdentity("task1", ""), func(_ context.Context, _ TestData) error { return nil }),
			Effect(NewIdentity("task2", ""), func(_ context.Context, _ TestData) error { return nil }),
			Effect(NewIdentity("task3", ""), func(_ context.Context, _ TestData) error { return nil }),
		)

		data := TestData{Value: 5}
		_, err := scaffold.Process(context.Background(), data)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if err := listener.Drain(context.Background()); err != nil {
			t.Fatalf("drain failed: %v", err)
		}

		if !signalReceived {
			t.Error("expected signal to be received")
		}
		if signalName != "signal-test-scaffold" {
			t.Errorf("expected name 'signal-test-scaffold', got %q", signalName)
		}
		if signalProcessorCount != 3 {
			t.Errorf("expected processor_count 3, got %d", signalProcessorCount)
		}
	})

	t.Run("Schema", func(t *testing.T) {
		proc1 := Transform(NewIdentity("proc1", ""), func(_ context.Context, d TestData) TestData { return d })
		proc2 := Transform(NewIdentity("proc2", ""), func(_ context.Context, d TestData) TestData { return d })

		scaffold := NewScaffold(NewIdentity("test-scaffold", "Scaffold connector"), proc1, proc2)

		schema := scaffold.Schema()

		if schema.Identity.Name() != "test-scaffold" {
			t.Errorf("Schema Identity.Name() = %v, want %v", schema.Identity.Name(), "test-scaffold")
		}
		if schema.Type != "scaffold" {
			t.Errorf("Schema Type = %v, want %v", schema.Type, "scaffold")
		}

		flow, ok := ScaffoldKey.From(schema)
		if !ok {
			t.Fatal("Expected ScaffoldFlow")
		}
		if len(flow.Processors) != 2 {
			t.Errorf("len(Flow.Processors) = %d, want 2", len(flow.Processors))
		}
	})
}
