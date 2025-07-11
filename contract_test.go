package pipz

import (
	"errors"
	"testing"
)

func TestContract(t *testing.T) {
	type TestData struct {
		Value int
		Text  string
	}

	t.Run("NewContract", func(t *testing.T) {
		contract := NewContract[TestData]()
		if contract == nil {
			t.Fatal("NewContract returned nil")
		}
		if contract.processors == nil {
			t.Fatal("processors slice not initialized")
		}
		if len(contract.processors) != 0 {
			t.Errorf("expected empty processors, got %d", len(contract.processors))
		}
	})

	t.Run("Register", func(t *testing.T) {
		contract := NewContract[TestData]()

		processor1 := func(d TestData) (TestData, error) {
			d.Value++
			return d, nil
		}
		processor2 := func(d TestData) (TestData, error) {
			d.Text += "!"
			return d, nil
		}

		contract.Register(processor1, processor2)

		if len(contract.processors) != 2 {
			t.Errorf("expected 2 processors, got %d", len(contract.processors))
		}
	})

	t.Run("Process_Success", func(t *testing.T) {
		contract := NewContract[TestData]()
		contract.Register(
			func(d TestData) (TestData, error) {
				d.Value *= 2
				return d, nil
			},
			func(d TestData) (TestData, error) {
				d.Text += "!"
				return d, nil
			},
		)

		input := TestData{Value: 5, Text: "hello"}
		result, err := contract.Process(input)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Value != 10 {
			t.Errorf("expected Value 10, got %d", result.Value)
		}
		if result.Text != "hello!" {
			t.Errorf("expected Text 'hello!', got '%s'", result.Text)
		}
	})

	t.Run("Process_Error", func(t *testing.T) {
		contract := NewContract[TestData]()
		contract.Register(
			func(d TestData) (TestData, error) {
				d.Value++
				return d, nil
			},
			func(d TestData) (TestData, error) {
				return d, errors.New("processing failed")
			},
			func(d TestData) (TestData, error) {
				// This should not be executed
				d.Value = 999
				return d, nil
			},
		)

		input := TestData{Value: 10, Text: "test"}
		result, err := contract.Process(input)

		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if err.Error() != "processing failed" {
			t.Errorf("unexpected error: %v", err)
		}
		// Result should be zero value on error
		if result.Value != 0 || result.Text != "" {
			t.Error("expected zero value on error")
		}
	})

	t.Run("Process_EmptyPipeline", func(t *testing.T) {
		contract := NewContract[TestData]()

		input := TestData{Value: 42, Text: "unchanged"}
		result, err := contract.Process(input)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != input {
			t.Error("empty pipeline should return input unchanged")
		}
	})

	t.Run("Link", func(t *testing.T) {
		contract := NewContract[TestData]()
		contract.Register(func(d TestData) (TestData, error) {
			d.Value = 100
			return d, nil
		})

		chainable := contract.Link()
		if chainable == nil {
			t.Fatal("Link returned nil")
		}

		// Verify it works as Chainable
		input := TestData{Value: 1, Text: "test"}
		result, err := chainable.Process(input)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Value != 100 {
			t.Errorf("expected Value 100, got %d", result.Value)
		}
	})
}
