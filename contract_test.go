package pipz

import (
	"errors"
	"fmt"
	"testing"
)

func TestContract(t *testing.T) {
	type TestKey string
	type TestData struct {
		Value int
		Text  string
	}
	
	const (
		testContractKey    TestKey = "test-contract"
		registerTestKey    TestKey = "register-test"
		emptyTestKey       TestKey = "empty-test"
		transformTestKey   TestKey = "transform-test"
		nilTestKey         TestKey = "nil-test"
		errorTestKey       TestKey = "error-test"
		chainableTestKey   TestKey = "chainable-test"
	)
	
	t.Run("GetContract", func(t *testing.T) {
		contract := GetContract[TestData](testContractKey)
		if contract == nil {
			t.Fatal("GetContract returned nil")
		}
		if contract.key != testContractKey {
			t.Errorf("expected key 'test-contract', got %v", contract.key)
		}
	})
	
	t.Run("RegisterProcessors", func(t *testing.T) {
		contract := GetContract[TestData](registerTestKey)
		
		// Register should succeed with valid processors
		err := contract.Register(
			func(data TestData) ([]byte, error) {
				data.Value++
				return Encode(data)
			},
			func(data TestData) ([]byte, error) {
				data.Text += "-processed"
				return Encode(data)
			},
		)
		
		if err != nil {
			t.Fatalf("Register failed: %v", err)
		}
		
		// Test processing works after registration
		input := TestData{Value: 10, Text: "test"}
		output, err := contract.Process(input)
		if err != nil {
			t.Fatal(err)
		}
		
		if output.Value != 11 {
			t.Errorf("expected value 11, got %d", output.Value)
		}
		if output.Text != "test-processed" {
			t.Errorf("expected text 'test-processed', got %s", output.Text)
		}
	})
	
	t.Run("RegisterEmptyProcessors", func(t *testing.T) {
		contract := GetContract[TestData](emptyTestKey)
		
		// Register with no processors should succeed
		err := contract.Register()
		if err != nil {
			t.Fatal("Register with no processors should succeed")
		}
	})
	
	t.Run("ProcessWithTransformation", func(t *testing.T) {
		contract := GetContract[TestData](transformTestKey)
		
		contract.Register(func(data TestData) ([]byte, error) {
			data.Value *= 2
			data.Text = fmt.Sprintf("%s-%d", data.Text, data.Value)
			return Encode(data)
		})
		
		input := TestData{Value: 5, Text: "num"}
		output, err := contract.Process(input)
		if err != nil {
			t.Fatal(err)
		}
		
		if output.Value != 10 {
			t.Errorf("expected value 10, got %d", output.Value)
		}
		if output.Text != "num-10" {
			t.Errorf("expected text 'num-10', got %s", output.Text)
		}
	})
	
	t.Run("ProcessorReturnsNil", func(t *testing.T) {
		contract := GetContract[TestData](nilTestKey)
		
		// Processor that returns nil (no modification)
		contract.Register(func(data TestData) ([]byte, error) {
			// Read-only processor - returns nil to indicate no change
			if data.Value > 0 {
				return nil, nil
			}
			data.Value = 100
			return Encode(data)
		})
		
		input := TestData{Value: 10, Text: "test"}
		output, err := contract.Process(input)
		if err != nil {
			t.Fatal(err)
		}
		
		// Should be unchanged since processor returned nil
		if output.Value != 10 {
			t.Errorf("expected value 10 (unchanged), got %d", output.Value)
		}
	})
	
	t.Run("ProcessErrorHandling", func(t *testing.T) {
		contract := GetContract[TestData](errorTestKey)
		
		expectedErr := errors.New("processing failed")
		contract.Register(
			func(data TestData) ([]byte, error) {
				data.Value++
				return Encode(data)
			},
			func(data TestData) ([]byte, error) {
				return nil, expectedErr
			},
		)
		
		input := TestData{Value: 10, Text: "test"}
		_, err := contract.Process(input)
		
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		
		// The error should be wrapped in processing context
		if !errors.Is(err, expectedErr) {
			t.Errorf("expected error to wrap %v, got %v", expectedErr, err)
		}
	})
	
	t.Run("ContractAsChainable", func(t *testing.T) {
		contract := GetContract[TestData](chainableTestKey)
		contract.Register(func(data TestData) ([]byte, error) {
			data.Value += 5
			return Encode(data)
		})
		
		// Use contract as a Chainable
		chainable := contract.Link()
		input := TestData{Value: 10, Text: "test"}
		output, err := chainable.Process(input)
		if err != nil {
			t.Fatal(err)
		}
		
		if output.Value != 15 {
			t.Errorf("expected value 15, got %d", output.Value)
		}
	})
}

func TestContractGlobalRegistry(t *testing.T) {
	type GlobalKey string
	type GlobalData struct {
		ID int
	}
	
	const (
		globalTestKey GlobalKey = "global-test"
		sharedKey     GlobalKey = "shared"
	)
	
	t.Run("ProcessThroughGlobalRegistry", func(t *testing.T) {
		contract := GetContract[GlobalData](globalTestKey)
		contract.Register(func(data GlobalData) ([]byte, error) {
			data.ID = 42
			return Encode(data)
		})
		
		// Process through the contract (which uses global registry internally)
		input := GlobalData{ID: 1}
		output, err := contract.Process(input)
		if err != nil {
			t.Fatal(err)
		}
		
		if output.ID != 42 {
			t.Errorf("expected ID 42, got %d", output.ID)
		}
	})
	
	t.Run("MultipleContractsSameKey", func(t *testing.T) {
		// Two contracts with same key should share the same pipeline
		
		contract1 := GetContract[GlobalData](sharedKey)
		contract1.Register(func(data GlobalData) ([]byte, error) {
			data.ID = 100
			return Encode(data)
		})
		
		// Get the same contract again
		contract2 := GetContract[GlobalData](sharedKey)
		
		// Process with second contract should use first contract's processors
		input := GlobalData{ID: 1}
		output, err := contract2.Process(input)
		if err != nil {
			t.Fatal(err)
		}
		
		if output.ID != 100 {
			t.Errorf("expected ID 100, got %d", output.ID)
		}
	})
}