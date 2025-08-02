package pipz

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
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

		concurrent := NewConcurrent("test-concurrent", p1, p2, p3)
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

		concurrent := NewConcurrent("test", p1, p2, p3)
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

		concurrent := NewConcurrent("test", blocker)
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		data := TestData{Value: 1}
		_, err := concurrent.Process(ctx, data)

		if err != nil {
			t.Errorf("expected no error with new behavior, got %v", err)
		}
	})

	t.Run("Empty Processors", func(t *testing.T) {
		concurrent := NewConcurrent[TestData]("empty")
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

		concurrent := NewConcurrent("test", p1, p2)

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
		concurrent := NewConcurrent[TestData]("my-concurrent")
		if concurrent.Name() != "my-concurrent" {
			t.Errorf("expected 'my-concurrent', got %q", concurrent.Name())
		}
	})

	t.Run("Remove Out of Bounds", func(t *testing.T) {
		p1 := Transform("p1", func(_ context.Context, d TestData) TestData { return d })
		concurrent := NewConcurrent("test", p1)

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
}
