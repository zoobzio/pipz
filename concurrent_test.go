package pipz

import (
	"context"
	"errors"
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

		concurrent := NewConcurrent("test-concurrent", nil, p1, p2, p3)
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
			return Effect("proc", func(_ context.Context, _ TestData) error {
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

		concurrent := NewConcurrent("test", nil, p1, p2, p3)
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
		blocker := Effect("block", func(ctx context.Context, _ TestData) error {
			<-ctx.Done()
			return ctx.Err()
		})

		concurrent := NewConcurrent("test", nil, blocker)
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		data := TestData{Value: 1}
		_, err := concurrent.Process(ctx, data)

		if err != nil {
			t.Errorf("expected no error with new behavior, got %v", err)
		}
	})

	t.Run("Empty Processors", func(t *testing.T) {
		concurrent := NewConcurrent[TestData]("empty", nil)
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
		p1 := Transform("p1", func(_ context.Context, d TestData) TestData { return d })
		p2 := Transform("p2", func(_ context.Context, d TestData) TestData { return d })
		p3 := Transform("p3", func(_ context.Context, d TestData) TestData { return d })

		concurrent := NewConcurrent("test", nil, p1, p2)

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
		concurrent := NewConcurrent[TestData]("my-concurrent", nil)
		if concurrent.Name() != "my-concurrent" {
			t.Errorf("expected 'my-concurrent', got %q", concurrent.Name())
		}
	})

	t.Run("Remove Out of Bounds", func(t *testing.T) {
		p1 := Transform("p1", func(_ context.Context, d TestData) TestData { return d })
		concurrent := NewConcurrent("test", nil, p1)

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
		panicProcessor := Transform("panic_processor", func(_ context.Context, _ TestData) TestData {
			panic("concurrent processor panic")
		})
		normalProcessor := Transform("normal_processor", func(_ context.Context, d TestData) TestData {
			return TestData{Value: d.Value + 1, Counter: d.Counter}
		})

		concurrent := NewConcurrent("panic_concurrent", nil, panicProcessor, normalProcessor)
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
		panicProcessor1 := Transform("panic1", func(_ context.Context, _ TestData) TestData {
			panic("first processor panic")
		})
		panicProcessor2 := Transform("panic2", func(_ context.Context, _ TestData) TestData {
			panic("second processor panic")
		})
		panicProcessor3 := Effect("panic3", func(_ context.Context, _ TestData) error {
			panic("third processor panic")
		})

		concurrent := NewConcurrent("panic_deadlock_test", nil, panicProcessor1, panicProcessor2, panicProcessor3)
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
		p1 := Transform("add10", func(_ context.Context, d TestData) TestData {
			return TestData{Value: d.Value + 10, Counter: d.Counter}
		})
		p2 := Transform("add20", func(_ context.Context, d TestData) TestData {
			return TestData{Value: d.Value + 20, Counter: d.Counter}
		})
		p3 := Transform("add30", func(_ context.Context, d TestData) TestData {
			return TestData{Value: d.Value + 30, Counter: d.Counter}
		})

		reducer := func(original TestData, results map[Name]TestData, _ map[Name]error) TestData {
			sum := original.Value
			for _, res := range results {
				sum += res.Value
			}
			return TestData{Value: sum, Counter: original.Counter}
		}

		concurrent := NewConcurrent("test-reducer", reducer, p1, p2, p3)
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
		p1 := Transform("success", func(_ context.Context, d TestData) TestData {
			return TestData{Value: d.Value + 10, Counter: d.Counter}
		})
		p2 := Effect("failure", func(_ context.Context, _ TestData) error {
			return errors.New("processor failed")
		})

		reducer := func(original TestData, results map[Name]TestData, errs map[Name]error) TestData {
			successCount := len(results)
			errorCount := len(errs)
			return TestData{Value: successCount*100 + errorCount*10, Counter: original.Counter}
		}

		concurrent := NewConcurrent("test-errors", reducer, p1, p2)
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

	t.Run("Reducer uses processor names", func(t *testing.T) {
		p1 := Transform("processor-a", func(_ context.Context, d TestData) TestData {
			return TestData{Value: 100, Counter: d.Counter}
		})
		p2 := Transform("processor-b", func(_ context.Context, d TestData) TestData {
			return TestData{Value: 200, Counter: d.Counter}
		})

		var capturedNames []Name
		reducer := func(original TestData, results map[Name]TestData, _ map[Name]error) TestData {
			for name := range results {
				capturedNames = append(capturedNames, name)
			}
			return original
		}

		concurrent := NewConcurrent("test-names", reducer, p1, p2)
		data := TestData{Value: 5}

		_, err := concurrent.Process(context.Background(), data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(capturedNames) != 2 {
			t.Fatalf("expected 2 processor names, got %d", len(capturedNames))
		}

		// Check both names are present (order may vary)
		hasA := false
		hasB := false
		for _, name := range capturedNames {
			if name == "processor-a" {
				hasA = true
			}
			if name == "processor-b" {
				hasB = true
			}
		}

		if !hasA || !hasB {
			t.Errorf("expected both processor-a and processor-b, got %v", capturedNames)
		}
	})

}

func TestConcurrentClose(t *testing.T) {
	t.Run("Closes All Children", func(t *testing.T) {
		p1 := newTrackingProcessor[TestData]("p1")
		p2 := newTrackingProcessor[TestData]("p2")

		c := NewConcurrent("test", nil, p1, p2)
		err := c.Close()

		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if p1.CloseCalls() != 1 || p2.CloseCalls() != 1 {
			t.Error("expected all processors to be closed")
		}
	})

	t.Run("Aggregates Errors", func(t *testing.T) {
		p1 := newTrackingProcessor[TestData]("p1").WithCloseError(errors.New("p1 error"))
		p2 := newTrackingProcessor[TestData]("p2").WithCloseError(errors.New("p2 error"))

		c := NewConcurrent("test", nil, p1, p2)
		err := c.Close()

		if err == nil {
			t.Error("expected error")
		}
		if p1.CloseCalls() != 1 || p2.CloseCalls() != 1 {
			t.Error("expected all processors to be closed")
		}
	})

	t.Run("Idempotency", func(t *testing.T) {
		p := newTrackingProcessor[TestData]("p")
		c := NewConcurrent("test", nil, p)

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

		concurrent := NewConcurrent[TestData]("signal-test-concurrent", nil,
			Transform("p1", func(_ context.Context, d TestData) TestData { return d }),
			Transform("p2", func(_ context.Context, d TestData) TestData { return d }),
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

		concurrent := NewConcurrent[TestData]("error-count-test", nil,
			Transform("success", func(_ context.Context, d TestData) TestData { return d }),
			Apply("fail", func(_ context.Context, _ TestData) (TestData, error) {
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
}
