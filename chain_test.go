package pipz

import (
	"errors"
	"testing"
)

// mockChainable is a test implementation of Chainable.
type mockChainable[T any] struct {
	processFunc func(T) (T, error)
}

func (m *mockChainable[T]) Process(value T) (T, error) {
	return m.processFunc(value)
}

func TestChain(t *testing.T) {
	type TestData struct {
		Value int
		Name  string
	}

	t.Run("NewChain", func(t *testing.T) {
		chain := NewChain[TestData]()
		if chain == nil {
			t.Fatal("NewChain returned nil")
		}
		if chain.processors == nil {
			t.Fatal("processors slice not initialized")
		}
		if len(chain.processors) != 0 {
			t.Errorf("expected empty processors, got %d", len(chain.processors))
		}
	})

	t.Run("Add", func(t *testing.T) {
		chain := NewChain[TestData]()

		proc1 := &mockChainable[TestData]{
			processFunc: func(d TestData) (TestData, error) {
				d.Value++
				return d, nil
			},
		}
		proc2 := &mockChainable[TestData]{
			processFunc: func(d TestData) (TestData, error) {
				d.Name += "!"
				return d, nil
			},
		}

		result := chain.Add(proc1, proc2)

		// Verify fluent interface
		if result != chain {
			t.Error("Add should return the chain for fluent interface")
		}

		if len(chain.processors) != 2 {
			t.Errorf("expected 2 processors, got %d", len(chain.processors))
		}
	})

	t.Run("Process_Success", func(t *testing.T) {
		chain := NewChain[TestData]()

		chain.Add(
			&mockChainable[TestData]{
				processFunc: func(d TestData) (TestData, error) {
					d.Value *= 2
					return d, nil
				},
			},
			&mockChainable[TestData]{
				processFunc: func(d TestData) (TestData, error) {
					d.Name += " processed"
					return d, nil
				},
			},
		)

		input := TestData{Value: 5, Name: "test"}
		result, err := chain.Process(input)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Value != 10 {
			t.Errorf("expected Value 10, got %d", result.Value)
		}
		if result.Name != "test processed" {
			t.Errorf("expected Name 'test processed', got '%s'", result.Name)
		}
	})

	t.Run("Process_Error", func(t *testing.T) {
		chain := NewChain[TestData]()

		chain.Add(
			&mockChainable[TestData]{
				processFunc: func(d TestData) (TestData, error) {
					d.Value++
					return d, nil
				},
			},
			&mockChainable[TestData]{
				processFunc: func(d TestData) (TestData, error) {
					return d, errors.New("chain failed")
				},
			},
			&mockChainable[TestData]{
				processFunc: func(d TestData) (TestData, error) {
					// This should not execute
					d.Value = 999
					return d, nil
				},
			},
		)

		input := TestData{Value: 10, Name: "test"}
		result, err := chain.Process(input)

		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if err.Error() != "chain failed" {
			t.Errorf("unexpected error: %v", err)
		}
		// Result should be zero value on error
		if result.Value != 0 || result.Name != "" {
			t.Error("expected zero value on error")
		}
	})

	t.Run("Process_EmptyChain", func(t *testing.T) {
		chain := NewChain[TestData]()

		input := TestData{Value: 42, Name: "unchanged"}
		result, err := chain.Process(input)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != input {
			t.Error("empty chain should return input unchanged")
		}
	})

	t.Run("Chain_With_Contracts", func(t *testing.T) {
		// Create two contracts
		contract1 := NewContract[TestData]()
		contract1.Register(func(d TestData) (TestData, error) {
			d.Value += 10
			return d, nil
		})

		contract2 := NewContract[TestData]()
		contract2.Register(func(d TestData) (TestData, error) {
			d.Value *= 2
			return d, nil
		})

		// Chain them together
		chain := NewChain[TestData]()
		chain.Add(contract1.Link(), contract2.Link())

		input := TestData{Value: 5, Name: "test"}
		result, err := chain.Process(input)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Value != 30 {
			t.Errorf("expected Value 30, got %d", result.Value)
		}
	})

	// Tests for new pipeline modification features
	t.Run("Utility_Methods", func(t *testing.T) {
		chain := NewChain[TestData]()

		// Test empty chain
		if !chain.IsEmpty() {
			t.Error("new chain should be empty")
		}
		if chain.Len() != 0 {
			t.Errorf("expected length 0, got %d", chain.Len())
		}

		// Add some processors
		chain.Add(
			&mockChainable[TestData]{func(d TestData) (TestData, error) { return d, nil }},
			&mockChainable[TestData]{func(d TestData) (TestData, error) { return d, nil }},
		)

		if chain.IsEmpty() {
			t.Error("chain with processors should not be empty")
		}
		if chain.Len() != 2 {
			t.Errorf("expected length 2, got %d", chain.Len())
		}

		// Test clear
		chain.Clear()
		if !chain.IsEmpty() {
			t.Error("cleared chain should be empty")
		}
		if chain.Len() != 0 {
			t.Errorf("expected length 0 after clear, got %d", chain.Len())
		}
	})

	t.Run("Queue_Stack_Operations", func(t *testing.T) {
		chain := NewChain[TestData]()

		proc1 := &mockChainable[TestData]{
			processFunc: func(d TestData) (TestData, error) {
				d.Value = 1
				return d, nil
			},
		}
		proc2 := &mockChainable[TestData]{
			processFunc: func(d TestData) (TestData, error) {
				d.Value = 2
				return d, nil
			},
		}
		proc3 := &mockChainable[TestData]{
			processFunc: func(d TestData) (TestData, error) {
				d.Value = 3
				return d, nil
			},
		}

		// Test PushTail
		chain.PushTail(proc1, proc2)
		if chain.Len() != 2 {
			t.Errorf("expected length 2, got %d", chain.Len())
		}

		// Test PushHead
		chain.PushHead(proc3)
		if chain.Len() != 3 {
			t.Errorf("expected length 3, got %d", chain.Len())
		}

		// Verify order: proc3 should run first (sets Value to 3), but proc2 runs last (sets Value to 2)
		result, err := chain.Process(TestData{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Value != 2 {
			t.Errorf("expected final Value 2, got %d", result.Value)
		}

		// Test PopHead
		popped, err := chain.PopHead()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if chain.Len() != 2 {
			t.Errorf("expected length 2 after PopHead, got %d", chain.Len())
		}

		// Verify popped processor
		testResult, err := popped.Process(TestData{})
		if err != nil {
			t.Fatalf("unexpected error from popped processor: %v", err)
		}
		if testResult.Value != 3 {
			t.Errorf("expected popped processor to set Value to 3, got %d", testResult.Value)
		}

		// Test PopTail
		_, err = chain.PopTail()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if chain.Len() != 1 {
			t.Errorf("expected length 1 after PopTail, got %d", chain.Len())
		}

		// Test PopHead on empty
		chain.Clear()
		_, err = chain.PopHead()
		if !errors.Is(err, ErrEmptyPipeline) {
			t.Errorf("expected ErrEmptyPipeline, got %v", err)
		}

		// Test PopTail on empty
		_, err = chain.PopTail()
		if !errors.Is(err, ErrEmptyPipeline) {
			t.Errorf("expected ErrEmptyPipeline, got %v", err)
		}
	})

	t.Run("Movement_Operations", func(t *testing.T) {
		chain := NewChain[TestData]()

		proc1 := &mockChainable[TestData]{
			processFunc: func(d TestData) (TestData, error) {
				d.Name += "1"
				return d, nil
			},
		}
		proc2 := &mockChainable[TestData]{
			processFunc: func(d TestData) (TestData, error) {
				d.Name += "2"
				return d, nil
			},
		}
		proc3 := &mockChainable[TestData]{
			processFunc: func(d TestData) (TestData, error) {
				d.Name += "3"
				return d, nil
			},
		}

		chain.PushTail(proc1, proc2, proc3)

		// Test MoveToHead
		err := chain.MoveToHead(2) // Move proc3 to front
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		result, err := chain.Process(TestData{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Name != "312" {
			t.Errorf("expected Name '312', got '%s'", result.Name)
		}

		// Test MoveToTail
		err = chain.MoveToTail(0) // Move proc3 to back
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		result, err = chain.Process(TestData{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Name != "123" {
			t.Errorf("expected Name '123', got '%s'", result.Name)
		}

		// Test MoveTo
		err = chain.MoveTo(2, 0) // Move proc3 to front again
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		result, err = chain.Process(TestData{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Name != "312" {
			t.Errorf("expected Name '312', got '%s'", result.Name)
		}

		// Test Swap
		err = chain.Swap(0, 1) // Swap proc3 and proc1
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		result, err = chain.Process(TestData{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Name != "132" {
			t.Errorf("expected Name '132', got '%s'", result.Name)
		}

		// Test Reverse
		chain.Reverse()
		result, err = chain.Process(TestData{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Name != "231" {
			t.Errorf("expected Name '231', got '%s'", result.Name)
		}

		// Test bounds checking
		err = chain.MoveToHead(5)
		if !errors.Is(err, ErrIndexOutOfBounds) {
			t.Errorf("expected ErrIndexOutOfBounds, got %v", err)
		}

		err = chain.Swap(-1, 0)
		if !errors.Is(err, ErrIndexOutOfBounds) {
			t.Errorf("expected ErrIndexOutOfBounds, got %v", err)
		}
	})

	t.Run("Precise_Operations", func(t *testing.T) {
		chain := NewChain[TestData]()

		proc1 := &mockChainable[TestData]{
			processFunc: func(d TestData) (TestData, error) {
				d.Name += "1"
				return d, nil
			},
		}
		proc2 := &mockChainable[TestData]{
			processFunc: func(d TestData) (TestData, error) {
				d.Name += "2"
				return d, nil
			},
		}
		proc3 := &mockChainable[TestData]{
			processFunc: func(d TestData) (TestData, error) {
				d.Name += "3"
				return d, nil
			},
		}

		chain.PushTail(proc1, proc3)

		// Test InsertAt
		err := chain.InsertAt(1, proc2)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		result, err := chain.Process(TestData{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Name != "123" {
			t.Errorf("expected Name '123', got '%s'", result.Name)
		}

		// Test ReplaceAt
		procX := &mockChainable[TestData]{
			processFunc: func(d TestData) (TestData, error) {
				d.Name += "X"
				return d, nil
			},
		}
		err = chain.ReplaceAt(1, procX)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		result, err = chain.Process(TestData{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Name != "1X3" {
			t.Errorf("expected Name '1X3', got '%s'", result.Name)
		}

		// Test RemoveAt
		err = chain.RemoveAt(1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		result, err = chain.Process(TestData{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Name != "13" {
			t.Errorf("expected Name '13', got '%s'", result.Name)
		}

		// Test bounds checking
		err = chain.InsertAt(5, proc1)
		if !errors.Is(err, ErrIndexOutOfBounds) {
			t.Errorf("expected ErrIndexOutOfBounds, got %v", err)
		}

		err = chain.RemoveAt(-1)
		if !errors.Is(err, ErrIndexOutOfBounds) {
			t.Errorf("expected ErrIndexOutOfBounds, got %v", err)
		}

		err = chain.ReplaceAt(10, proc1)
		if !errors.Is(err, ErrIndexOutOfBounds) {
			t.Errorf("expected ErrIndexOutOfBounds, got %v", err)
		}
	})
}
