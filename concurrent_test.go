package pipz

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/zoobzio/capitan"
)

// TestData implements Cloner for testing.
type TestData struct {
	Counter *int32
	Value   int
}

func (t TestData) Clone() TestData {
	return TestData{
		Value:   t.Value,
		Counter: t.Counter, // Intentionally share counter for testing
	}
}

func TestConcurrent(t *testing.T) {
	t.Run("Runs All Processors", func(t *testing.T) {
		var counter int32
		data := TestData{Value: 1, Counter: &counter}

		p1 := Effect(NewIdentity("inc1", ""), func(_ context.Context, d TestData) error {
			atomic.AddInt32(d.Counter, 1)
			return nil
		})
		p2 := Effect(NewIdentity("inc2", ""), func(_ context.Context, d TestData) error {
			atomic.AddInt32(d.Counter, 10)
			return nil
		})
		p3 := Effect(NewIdentity("inc3", ""), func(_ context.Context, d TestData) error {
			atomic.AddInt32(d.Counter, 100)
			return nil
		})

		concurrent := NewConcurrent(NewIdentity("test-concurrent", ""), nil, p1, p2, p3)
		result, err := concurrent.Process(context.Background(), data)

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

	t.Run("Processors Run Concurrently", func(t *testing.T) {
		var wg sync.WaitGroup
		wg.Add(3)

		var order []int
		var mu sync.Mutex

		makeProcessor := func(id int, delay time.Duration) Chainable[TestData] {
			return Effect(NewIdentity("proc", ""), func(_ context.Context, _ TestData) error {
				wg.Done()
				wg.Wait() // Wait for all to start
				time.Sleep(delay)
				mu.Lock()
				order = append(order, id)
				mu.Unlock()
				return nil
			})
		}

		p1 := makeProcessor(1, 30*time.Millisecond)
		p2 := makeProcessor(2, 10*time.Millisecond)
		p3 := makeProcessor(3, 20*time.Millisecond)

		concurrent := NewConcurrent(NewIdentity("test", ""), nil, p1, p2, p3)
		data := TestData{Value: 1}

		_, err := concurrent.Process(context.Background(), data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Should complete in order of delays: 2, 3, 1
		if len(order) != 3 || order[0] != 2 || order[1] != 3 || order[2] != 1 {
			t.Errorf("expected order [2,3,1], got %v", order)
		}
	})

	t.Run("Context Cancellation", func(t *testing.T) {
		blocker := Effect(NewIdentity("block", ""), func(ctx context.Context, _ TestData) error {
			<-ctx.Done()
			return ctx.Err()
		})

		concurrent := NewConcurrent(NewIdentity("test", ""), nil, blocker)
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		data := TestData{Value: 1}
		_, err := concurrent.Process(ctx, data)

		if err != nil {
			t.Errorf("expected no error with new behavior, got %v", err)
		}
	})

	t.Run("Context Cancellation With Reducer", func(t *testing.T) {
		blocker := Effect(NewIdentity("slow", ""), func(ctx context.Context, _ TestData) error {
			<-ctx.Done()
			return ctx.Err()
		})
		fast := Transform(NewIdentity("fast", ""), func(_ context.Context, d TestData) TestData {
			return TestData{Value: d.Value * 2, Counter: d.Counter}
		})

		var reducerCalled bool
		reducer := func(original TestData, results map[Identity]TestData, _ map[Identity]error) TestData {
			reducerCalled = true
			// Return sum of whatever results we have plus original
			sum := original.Value
			for _, res := range results {
				sum += res.Value
			}
			return TestData{Value: sum, Counter: original.Counter}
		}

		concurrent := NewConcurrent(NewIdentity("test-cancel-reducer", ""), reducer, blocker, fast)
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		data := TestData{Value: 5}
		result, err := concurrent.Process(ctx, data)

		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}

		if !reducerCalled {
			t.Error("expected reducer to be called on context cancellation")
		}

		// Fast processor should complete before timeout, so we get its result
		// 5 (original) + 10 (fast: 5*2) = 15
		if result.Value != 15 {
			t.Errorf("expected 15, got %d", result.Value)
		}
	})

	t.Run("Empty Processors", func(t *testing.T) {
		concurrent := NewConcurrent[TestData](NewIdentity("empty", ""), nil)
		data := TestData{Value: 42}

		result, err := concurrent.Process(context.Background(), data)
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

		concurrent := NewConcurrent(NewIdentity("test", ""), nil, p1, p2)

		if concurrent.Len() != 2 {
			t.Errorf("expected 2 processors, got %d", concurrent.Len())
		}

		concurrent.Add(p3)
		if concurrent.Len() != 3 {
			t.Errorf("expected 3 processors after add, got %d", concurrent.Len())
		}

		err := concurrent.Remove(1)
		if err != nil {
			t.Fatalf("unexpected error removing processor: %v", err)
		}
		if concurrent.Len() != 2 {
			t.Errorf("expected 2 processors after remove, got %d", concurrent.Len())
		}

		concurrent.Clear()
		if concurrent.Len() != 0 {
			t.Errorf("expected 0 processors after clear, got %d", concurrent.Len())
		}

		concurrent.SetProcessors(p1, p2, p3)
		if concurrent.Len() != 3 {
			t.Errorf("expected 3 processors after SetProcessors, got %d", concurrent.Len())
		}
	})

	t.Run("Name Method", func(t *testing.T) {
		concurrent := NewConcurrent[TestData](NewIdentity("my-concurrent", ""), nil)
		if concurrent.Identity().Name() != "my-concurrent" {
			t.Errorf("expected 'my-concurrent', got %q", concurrent.Identity().Name())
		}
	})

	t.Run("Remove Out of Bounds", func(t *testing.T) {
		p1 := Transform(NewIdentity("p1", ""), func(_ context.Context, d TestData) TestData { return d })
		concurrent := NewConcurrent(NewIdentity("test", ""), nil, p1)

		// Test negative index
		err := concurrent.Remove(-1)
		if err == nil {
			t.Error("expected error for negative index")
		}
		if !errors.Is(err, ErrIndexOutOfBounds) {
			t.Errorf("expected ErrIndexOutOfBounds, got %v", err)
		}

		// Test index >= length
		err = concurrent.Remove(1)
		if err == nil {
			t.Error("expected error for index >= length")
		}
		if !errors.Is(err, ErrIndexOutOfBounds) {
			t.Errorf("expected ErrIndexOutOfBounds, got %v", err)
		}
	})

	t.Run("Concurrent panic recovery", func(t *testing.T) {
		panicProcessor := Transform(NewIdentity("panic_processor", ""), func(_ context.Context, _ TestData) TestData {
			panic("concurrent processor panic")
		})
		normalProcessor := Transform(NewIdentity("normal_processor", ""), func(_ context.Context, d TestData) TestData {
			return TestData{Value: d.Value + 1, Counter: d.Counter}
		})

		concurrent := NewConcurrent(NewIdentity("panic_concurrent", ""), nil, panicProcessor, normalProcessor)
		data := TestData{Value: 42}

		result, err := concurrent.Process(context.Background(), data)

		// Concurrent should return original data and no error even if individual processors panic
		// This is by design - concurrent processors run independently and don't propagate errors
		if err != nil {
			t.Errorf("expected no error from concurrent processor, got %v", err)
		}

		if result.Value != 42 {
			t.Errorf("expected original value 42, got %d", result.Value)
		}
	})

	t.Run("No deadlock when processors panic", func(t *testing.T) {
		// Test that verifies deadlock prevention when processor panics occur
		// Multiple panicking processors ensure we test the WaitGroup properly
		panicProcessor1 := Transform(NewIdentity("panic1", ""), func(_ context.Context, _ TestData) TestData {
			panic("first processor panic")
		})
		panicProcessor2 := Transform(NewIdentity("panic2", ""), func(_ context.Context, _ TestData) TestData {
			panic("second processor panic")
		})
		panicProcessor3 := Effect(NewIdentity("panic3", ""), func(_ context.Context, _ TestData) error {
			panic("third processor panic")
		})

		concurrent := NewConcurrent(NewIdentity("panic_deadlock_test", ""), nil, panicProcessor1, panicProcessor2, panicProcessor3)
		data := TestData{Value: 42}

		// This should complete without deadlocking despite all processors panicking
		done := make(chan struct{})
		var result TestData
		var err error

		go func() {
			defer close(done)
			result, err = concurrent.Process(context.Background(), data)
		}()

		// Set a timeout to detect deadlock - if the fix works, this should complete quickly
		select {
		case <-done:
			// Success - no deadlock occurred
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if result.Value != 42 {
				t.Errorf("expected original value 42, got %d", result.Value)
			}
		case <-time.After(1 * time.Second):
			t.Fatal("deadlock detected: concurrent.Process() did not complete within timeout")
		}
	})

	t.Run("Reducer aggregates results", func(t *testing.T) {
		p1 := Transform(NewIdentity("add10", ""), func(_ context.Context, d TestData) TestData {
			return TestData{Value: d.Value + 10, Counter: d.Counter}
		})
		p2 := Transform(NewIdentity("add20", ""), func(_ context.Context, d TestData) TestData {
			return TestData{Value: d.Value + 20, Counter: d.Counter}
		})
		p3 := Transform(NewIdentity("add30", ""), func(_ context.Context, d TestData) TestData {
			return TestData{Value: d.Value + 30, Counter: d.Counter}
		})

		reducer := func(original TestData, results map[Identity]TestData, _ map[Identity]error) TestData {
			sum := original.Value
			for _, res := range results {
				sum += res.Value
			}
			return TestData{Value: sum, Counter: original.Counter}
		}

		concurrent := NewConcurrent(NewIdentity("test-reducer", ""), reducer, p1, p2, p3)
		data := TestData{Value: 5}

		result, err := concurrent.Process(context.Background(), data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Original: 5
		// p1 result: 5+10 = 15
		// p2 result: 5+20 = 25
		// p3 result: 5+30 = 35
		// sum: 5 + 15 + 25 + 35 = 80
		if result.Value != 80 {
			t.Errorf("expected 80, got %d", result.Value)
		}
	})

	t.Run("Reducer handles errors", func(t *testing.T) {
		p1 := Transform(NewIdentity("success", ""), func(_ context.Context, d TestData) TestData {
			return TestData{Value: d.Value + 10, Counter: d.Counter}
		})
		p2 := Effect(NewIdentity("failure", ""), func(_ context.Context, _ TestData) error {
			return errors.New("processor failed")
		})

		reducer := func(original TestData, results map[Identity]TestData, errs map[Identity]error) TestData {
			successCount := len(results)
			errorCount := len(errs)
			return TestData{Value: successCount*100 + errorCount*10, Counter: original.Counter}
		}

		concurrent := NewConcurrent(NewIdentity("test-errors", ""), reducer, p1, p2)
		data := TestData{Value: 5}

		result, err := concurrent.Process(context.Background(), data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// 1 success, 1 error = 1*100 + 1*10 = 110
		if result.Value != 110 {
			t.Errorf("expected 110, got %d", result.Value)
		}
	})

	t.Run("Reducer receives panics as errors", func(t *testing.T) {
		panicID := NewIdentity("panicking", "")
		successID := NewIdentity("success", "")

		p1 := Transform(panicID, func(_ context.Context, _ TestData) TestData {
			panic("processor panic")
		})
		p2 := Transform(successID, func(_ context.Context, d TestData) TestData {
			return TestData{Value: d.Value * 2, Counter: d.Counter}
		})

		var capturedErrs map[Identity]error
		reducer := func(original TestData, results map[Identity]TestData, errs map[Identity]error) TestData {
			capturedErrs = errs
			sum := original.Value
			for _, res := range results {
				sum += res.Value
			}
			return TestData{Value: sum, Counter: original.Counter}
		}

		concurrent := NewConcurrent(NewIdentity("test-panic-reducer", ""), reducer, p1, p2)
		data := TestData{Value: 5}

		result, err := concurrent.Process(context.Background(), data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Success processor: 5 * 2 = 10, plus original 5 = 15
		if result.Value != 15 {
			t.Errorf("expected 15, got %d", result.Value)
		}

		// Verify panic was captured as error
		if len(capturedErrs) != 1 {
			t.Fatalf("expected 1 error from panic, got %d", len(capturedErrs))
		}

		panicErr, ok := capturedErrs[panicID]
		if !ok {
			t.Fatal("expected error keyed by panicking processor identity")
		}

		if panicErr == nil {
			t.Fatal("expected non-nil panic error")
		}

		// Verify error message contains panic info
		errMsg := panicErr.Error()
		if !strings.Contains(errMsg, "panic") {
			t.Errorf("expected error to mention panic, got: %s", errMsg)
		}
		if !strings.Contains(errMsg, "panicking") {
			t.Errorf("expected error to contain processor name, got: %s", errMsg)
		}
	})

	t.Run("Reducer uses processor identities", func(t *testing.T) {
		idA := NewIdentity("processor-a", "")
		idB := NewIdentity("processor-b", "")
		p1 := Transform(idA, func(_ context.Context, d TestData) TestData {
			return TestData{Value: 100, Counter: d.Counter}
		})
		p2 := Transform(idB, func(_ context.Context, d TestData) TestData {
			return TestData{Value: 200, Counter: d.Counter}
		})

		var capturedIdentities []Identity
		reducer := func(original TestData, results map[Identity]TestData, _ map[Identity]error) TestData {
			for id := range results {
				capturedIdentities = append(capturedIdentities, id)
			}
			return original
		}

		concurrent := NewConcurrent(NewIdentity("test-identities", ""), reducer, p1, p2)
		data := TestData{Value: 5}

		_, err := concurrent.Process(context.Background(), data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(capturedIdentities) != 2 {
			t.Fatalf("expected 2 processor identities, got %d", len(capturedIdentities))
		}

		// Check both identities are present (order may vary)
		hasA := false
		hasB := false
		for _, id := range capturedIdentities {
			if id == idA {
				hasA = true
			}
			if id == idB {
				hasB = true
			}
		}

		if !hasA || !hasB {
			t.Errorf("expected both processor-a and processor-b identities, got %v", capturedIdentities)
		}
	})

}

func TestConcurrentClose(t *testing.T) {
	t.Run("Closes All Children", func(t *testing.T) {
		p1 := newTrackingProcessor[TestData](NewIdentity("p1", ""))
		p2 := newTrackingProcessor[TestData](NewIdentity("p2", ""))

		c := NewConcurrent(NewIdentity("test", ""), nil, p1, p2)
		err := c.Close()

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

		c := NewConcurrent(NewIdentity("test", ""), nil, p1, p2)
		err := c.Close()

		if err == nil {
			t.Error("expected error")
		}
		if p1.CloseCalls() != 1 || p2.CloseCalls() != 1 {
			t.Error("expected all processors to be closed")
		}
	})

	t.Run("Idempotency", func(t *testing.T) {
		p := newTrackingProcessor[TestData](NewIdentity("p", ""))
		c := NewConcurrent(NewIdentity("test", ""), nil, p)

		_ = c.Close()
		_ = c.Close()

		if p.CloseCalls() != 1 {
			t.Errorf("expected 1 close call, got %d", p.CloseCalls())
		}
	})
}

func TestConcurrentSignals(t *testing.T) {
	t.Run("Emits Completed Signal", func(t *testing.T) {
		var signalReceived bool
		var signalName string
		var signalProcessorCount int
		var signalErrorCount int
		var signalDuration float64

		listener := capitan.Hook(SignalConcurrentCompleted, func(_ context.Context, e *capitan.Event) {
			signalReceived = true
			signalName, _ = FieldName.From(e)
			signalProcessorCount, _ = FieldProcessorCount.From(e)
			signalErrorCount, _ = FieldErrorCount.From(e)
			signalDuration, _ = FieldDuration.From(e)
		})
		defer listener.Close()

		concurrent := NewConcurrent[TestData](NewIdentity("signal-test-concurrent", ""), nil,
			Transform(NewIdentity("p1", ""), func(_ context.Context, d TestData) TestData { return d }),
			Transform(NewIdentity("p2", ""), func(_ context.Context, d TestData) TestData { return d }),
		)

		data := TestData{Value: 5}
		_, err := concurrent.Process(context.Background(), data)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if err := listener.Drain(context.Background()); err != nil {
			t.Fatalf("drain failed: %v", err)
		}

		if !signalReceived {
			t.Error("expected signal to be received")
		}
		if signalName != "signal-test-concurrent" {
			t.Errorf("expected name 'signal-test-concurrent', got %q", signalName)
		}
		if signalProcessorCount != 2 {
			t.Errorf("expected processor_count 2, got %d", signalProcessorCount)
		}
		if signalErrorCount != 0 {
			t.Errorf("expected error_count 0, got %d", signalErrorCount)
		}
		if signalDuration <= 0 {
			t.Error("expected positive duration")
		}
	})

	t.Run("Tracks Error Count In Signal", func(t *testing.T) {
		var signalErrorCount int

		listener := capitan.Hook(SignalConcurrentCompleted, func(_ context.Context, e *capitan.Event) {
			signalErrorCount, _ = FieldErrorCount.From(e)
		})
		defer listener.Close()

		concurrent := NewConcurrent[TestData](NewIdentity("error-count-test", ""), nil,
			Transform(NewIdentity("success", ""), func(_ context.Context, d TestData) TestData { return d }),
			Apply(NewIdentity("fail", ""), func(_ context.Context, _ TestData) (TestData, error) {
				return TestData{}, errors.New("intentional failure")
			}),
		)

		data := TestData{Value: 5}
		_, _ = concurrent.Process(context.Background(), data)

		if err := listener.Drain(context.Background()); err != nil {
			t.Fatalf("drain failed: %v", err)
		}

		if signalErrorCount != 1 {
			t.Errorf("expected error_count 1, got %d", signalErrorCount)
		}
	})

	t.Run("Schema", func(t *testing.T) {
		proc1 := Transform(NewIdentity("proc1", ""), func(_ context.Context, d TestData) TestData { return d })
		proc2 := Transform(NewIdentity("proc2", ""), func(_ context.Context, d TestData) TestData { return d })

		concurrent := NewConcurrent[TestData](NewIdentity("test-concurrent", "Concurrent connector"), nil, proc1, proc2)

		schema := concurrent.Schema()

		if schema.Identity.Name() != "test-concurrent" {
			t.Errorf("Schema Identity.Name() = %v, want %v", schema.Identity.Name(), "test-concurrent")
		}
		if schema.Type != "concurrent" {
			t.Errorf("Schema Type = %v, want %v", schema.Type, "concurrent")
		}

		flow, ok := ConcurrentKey.From(schema)
		if !ok {
			t.Fatal("Expected ConcurrentFlow")
		}
		if len(flow.Tasks) != 2 {
			t.Errorf("len(Flow.Tasks) = %d, want 2", len(flow.Tasks))
		}
	})
}
