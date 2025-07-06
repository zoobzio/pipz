package pipz

import (
	"testing"
)

func TestProcessUnregisteredKey(t *testing.T) {
	t.Run("ProcessUnregisteredKey", func(t *testing.T) {
		_, err := Process("nonexistent", []byte("test"))
		if err == nil {
			t.Fatal("expected error for unregistered key")
		}
	})
}

func TestGetContract(t *testing.T) {
	type TestKey string
	type TestData struct {
		Value string
	}
	
	t.Run("SingletonBehavior", func(t *testing.T) {
		key := TestKey("test-v1")
		
		// Get contract twice
		contract1 := GetContract[TestKey, TestData](key)
		contract2 := GetContract[TestKey, TestData](key)
		
		// They should have the same registry key
		if contract1.String() != contract2.String() {
			t.Error("GetContract should return contracts with the same registry key")
		}
	})
	
	t.Run("DifferentKeysGetDifferentContracts", func(t *testing.T) {
		contract1 := GetContract[TestKey, TestData](TestKey("v1"))
		contract2 := GetContract[TestKey, TestData](TestKey("v2"))
		
		if contract1.String() == contract2.String() {
			t.Error("Different keys should get different registry keys")
		}
	})
}

func TestProcess(t *testing.T) {
	type ProcessKey string
	type ProcessData struct {
		Count int
	}
	
	key := ProcessKey("process-test")
	contract := GetContract[ProcessKey, ProcessData](key)
	
	// Register a simple incrementing processor
	contract.Register(func(data ProcessData) ([]byte, error) {
		data.Count++
		return Encode(data)
	})
	
	t.Run("SuccessfulProcessing", func(t *testing.T) {
		input := ProcessData{Count: 5}
		output, err := contract.Process(input)
		if err != nil {
			t.Fatal(err)
		}
		
		if output.Count != 6 {
			t.Errorf("expected count to be 6, got %d", output.Count)
		}
	})
	
	t.Run("UnregisteredKey", func(t *testing.T) {
		_, err := Process("unknown-key", []byte("test"))
		if err == nil {
			t.Fatal("expected error for unregistered key")
		}
	})
}