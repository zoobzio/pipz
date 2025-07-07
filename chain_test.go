package pipz

import (
	"errors"
	"testing"
)

// mockChainable is a test implementation of Chainable
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
					d.Name = d.Name + " processed"
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
		// (5 + 10) * 2 = 30
		if result.Value != 30 {
			t.Errorf("expected Value 30, got %d", result.Value)
		}
	})
}