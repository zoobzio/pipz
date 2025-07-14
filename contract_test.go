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

	// Tests for new pipeline modification features
	t.Run("Utility_Methods", func(t *testing.T) {
		contract := NewContract[TestData]()

		// Test empty contract
		if !contract.IsEmpty() {
			t.Error("new contract should be empty")
		}
		if contract.Len() != 0 {
			t.Errorf("expected length 0, got %d", contract.Len())
		}

		// Add some processors
		contract.Register(
			func(d TestData) (TestData, error) { return d, nil },
			func(d TestData) (TestData, error) { return d, nil },
		)

		if contract.IsEmpty() {
			t.Error("contract with processors should not be empty")
		}
		if contract.Len() != 2 {
			t.Errorf("expected length 2, got %d", contract.Len())
		}

		// Test clear
		contract.Clear()
		if !contract.IsEmpty() {
			t.Error("cleared contract should be empty")
		}
		if contract.Len() != 0 {
			t.Errorf("expected length 0 after clear, got %d", contract.Len())
		}
	})

	t.Run("Queue_Stack_Operations", func(t *testing.T) {
		contract := NewContract[TestData]()

		proc1 := func(d TestData) (TestData, error) {
			d.Value = 1
			return d, nil
		}
		proc2 := func(d TestData) (TestData, error) {
			d.Value = 2
			return d, nil
		}
		proc3 := func(d TestData) (TestData, error) {
			d.Value = 3
			return d, nil
		}

		// Test PushTail
		contract.PushTail(proc1, proc2)
		if contract.Len() != 2 {
			t.Errorf("expected length 2, got %d", contract.Len())
		}

		// Test PushHead
		contract.PushHead(proc3)
		if contract.Len() != 3 {
			t.Errorf("expected length 3, got %d", contract.Len())
		}

		// Verify order: proc3 should run first (sets Value to 3), but proc2 runs last (sets Value to 2)
		result, err := contract.Process(TestData{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Value != 2 {
			t.Errorf("expected final Value 2, got %d", result.Value)
		}

		// Test PopHead
		popped, err := contract.PopHead()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if contract.Len() != 2 {
			t.Errorf("expected length 2 after PopHead, got %d", contract.Len())
		}

		// Verify popped processor
		testResult, err := popped(TestData{})
		if err != nil {
			t.Fatalf("unexpected error from popped processor: %v", err)
		}
		if testResult.Value != 3 {
			t.Errorf("expected popped processor to set Value to 3, got %d", testResult.Value)
		}

		// Test PopTail
		_, err = contract.PopTail()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if contract.Len() != 1 {
			t.Errorf("expected length 1 after PopTail, got %d", contract.Len())
		}

		// Test PopHead on empty
		contract.Clear()
		_, err = contract.PopHead()
		if !errors.Is(err, ErrEmptyPipeline) {
			t.Errorf("expected ErrEmptyPipeline, got %v", err)
		}

		// Test PopTail on empty
		_, err = contract.PopTail()
		if !errors.Is(err, ErrEmptyPipeline) {
			t.Errorf("expected ErrEmptyPipeline, got %v", err)
		}
	})

	t.Run("Movement_Operations", func(t *testing.T) {
		contract := NewContract[TestData]()

		proc1 := func(d TestData) (TestData, error) {
			d.Text += "1"
			return d, nil
		}
		proc2 := func(d TestData) (TestData, error) {
			d.Text += "2"
			return d, nil
		}
		proc3 := func(d TestData) (TestData, error) {
			d.Text += "3"
			return d, nil
		}

		contract.PushTail(proc1, proc2, proc3)

		// Test MoveToHead
		err := contract.MoveToHead(2) // Move proc3 to front
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		result, err := contract.Process(TestData{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Text != "312" {
			t.Errorf("expected Text '312', got '%s'", result.Text)
		}

		// Test MoveToTail
		err = contract.MoveToTail(0) // Move proc3 to back
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		result, err = contract.Process(TestData{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Text != "123" {
			t.Errorf("expected Text '123', got '%s'", result.Text)
		}

		// Test MoveTo
		err = contract.MoveTo(2, 0) // Move proc3 to front again
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		result, err = contract.Process(TestData{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Text != "312" {
			t.Errorf("expected Text '312', got '%s'", result.Text)
		}

		// Test Swap
		err = contract.Swap(0, 1) // Swap proc3 and proc1
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		result, err = contract.Process(TestData{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Text != "132" {
			t.Errorf("expected Text '132', got '%s'", result.Text)
		}

		// Test Reverse
		contract.Reverse()
		result, err = contract.Process(TestData{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Text != "231" {
			t.Errorf("expected Text '231', got '%s'", result.Text)
		}

		// Test bounds checking
		err = contract.MoveToHead(5)
		if !errors.Is(err, ErrIndexOutOfBounds) {
			t.Errorf("expected ErrIndexOutOfBounds, got %v", err)
		}

		err = contract.Swap(-1, 0)
		if !errors.Is(err, ErrIndexOutOfBounds) {
			t.Errorf("expected ErrIndexOutOfBounds, got %v", err)
		}
	})

	t.Run("Precise_Operations", func(t *testing.T) {
		contract := NewContract[TestData]()

		proc1 := func(d TestData) (TestData, error) {
			d.Text += "1"
			return d, nil
		}
		proc2 := func(d TestData) (TestData, error) {
			d.Text += "2"
			return d, nil
		}
		proc3 := func(d TestData) (TestData, error) {
			d.Text += "3"
			return d, nil
		}

		contract.PushTail(proc1, proc3)

		// Test InsertAt
		err := contract.InsertAt(1, proc2)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		result, err := contract.Process(TestData{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Text != "123" {
			t.Errorf("expected Text '123', got '%s'", result.Text)
		}

		// Test ReplaceAt
		procX := func(d TestData) (TestData, error) {
			d.Text += "X"
			return d, nil
		}
		err = contract.ReplaceAt(1, procX)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		result, err = contract.Process(TestData{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Text != "1X3" {
			t.Errorf("expected Text '1X3', got '%s'", result.Text)
		}

		// Test RemoveAt
		err = contract.RemoveAt(1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		result, err = contract.Process(TestData{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Text != "13" {
			t.Errorf("expected Text '13', got '%s'", result.Text)
		}

		// Test bounds checking
		err = contract.InsertAt(5, proc1)
		if !errors.Is(err, ErrIndexOutOfBounds) {
			t.Errorf("expected ErrIndexOutOfBounds, got %v", err)
		}

		err = contract.RemoveAt(-1)
		if !errors.Is(err, ErrIndexOutOfBounds) {
			t.Errorf("expected ErrIndexOutOfBounds, got %v", err)
		}

		err = contract.ReplaceAt(10, proc1)
		if !errors.Is(err, ErrIndexOutOfBounds) {
			t.Errorf("expected ErrIndexOutOfBounds, got %v", err)
		}
	})
}
