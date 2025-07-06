package pipz

import (
	"errors"
	"testing"
)

type TestData struct {
	Value int
	Name  string
}

// TestProcessor implements Chainable[TestData] for testing
type TestProcessor struct {
	processFunc func(TestData) (TestData, error)
}

func (p *TestProcessor) Process(data TestData) (TestData, error) {
	return p.processFunc(data)
}

func TestChain(t *testing.T) {
	
	t.Run("EmptyChain", func(t *testing.T) {
		chain := NewChain[TestData]()
		
		input := TestData{Value: 10, Name: "test"}
		output, err := chain.Process(input)
		if err != nil {
			t.Fatal(err)
		}
		
		if output != input {
			t.Error("empty chain should return input unchanged")
		}
	})
	
	t.Run("SingleProcessor", func(t *testing.T) {
		chain := NewChain[TestData]()
		chain.Add(&TestProcessor{
			processFunc: func(data TestData) (TestData, error) {
				data.Value *= 2
				return data, nil
			},
		})
		
		input := TestData{Value: 10, Name: "test"}
		output, err := chain.Process(input)
		if err != nil {
			t.Fatal(err)
		}
		
		if output.Value != 20 {
			t.Errorf("expected value 20, got %d", output.Value)
		}
	})
	
	t.Run("MultipleProcessors", func(t *testing.T) {
		chain := NewChain[TestData]()
		chain.Add(
			&TestProcessor{
				processFunc: func(data TestData) (TestData, error) {
					data.Value += 5
					return data, nil
				},
			},
			&TestProcessor{
				processFunc: func(data TestData) (TestData, error) {
					data.Value *= 2
					return data, nil
				},
			},
			&TestProcessor{
				processFunc: func(data TestData) (TestData, error) {
					data.Name = data.Name + "-processed"
					return data, nil
				},
			},
		)
		
		input := TestData{Value: 10, Name: "test"}
		output, err := chain.Process(input)
		if err != nil {
			t.Fatal(err)
		}
		
		if output.Value != 30 { // (10 + 5) * 2
			t.Errorf("expected value 30, got %d", output.Value)
		}
		if output.Name != "test-processed" {
			t.Errorf("expected name 'test-processed', got %s", output.Name)
		}
	})
	
	t.Run("ProcessorError", func(t *testing.T) {
		expectedErr := errors.New("processor failed")
		chain := NewChain[TestData]()
		chain.Add(
			&TestProcessor{
				processFunc: func(data TestData) (TestData, error) {
					data.Value += 1
					return data, nil
				},
			},
			&TestProcessor{
				processFunc: func(data TestData) (TestData, error) {
					return data, expectedErr
				},
			},
			&TestProcessor{
				processFunc: func(data TestData) (TestData, error) {
					// This should not be called
					data.Value += 100
					return data, nil
				},
			},
		)
		
		input := TestData{Value: 10, Name: "test"}
		output, err := chain.Process(input)
		
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		
		if !errors.Is(err, expectedErr) {
			t.Errorf("expected specific error, got %v", err)
		}
		
		// On error, should return the original input
		if output.Value != 10 {
			t.Errorf("expected value 10 (original input), got %d", output.Value)
		}
	})
	
	t.Run("MethodChaining", func(t *testing.T) {
		chain := NewChain[TestData]().
			Add(&TestProcessor{
				processFunc: func(data TestData) (TestData, error) {
					data.Value++
					return data, nil
				},
			}).
			Add(&TestProcessor{
				processFunc: func(data TestData) (TestData, error) {
					data.Value *= 3
					return data, nil
				},
			})
		
		input := TestData{Value: 5, Name: "test"}
		output, err := chain.Process(input)
		if err != nil {
			t.Fatal(err)
		}
		
		if output.Value != 18 { // (5 + 1) * 3
			t.Errorf("expected value 18, got %d", output.Value)
		}
	})
}