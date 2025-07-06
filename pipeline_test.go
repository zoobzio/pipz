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
	
	const (
		testV1Key TestKey = "test-v1"
		v1Key     TestKey = "v1"
		v2Key     TestKey = "v2"
	)
	
	t.Run("SingletonBehavior", func(t *testing.T) {
		
		// Get contract twice
		contract1 := GetContract[TestData](testV1Key)
		contract2 := GetContract[TestData](testV1Key)
		
		// They should have the same registry key
		if contract1.String() != contract2.String() {
			t.Error("GetContract should return contracts with the same registry key")
		}
	})
	
	t.Run("DifferentKeysGetDifferentContracts", func(t *testing.T) {
		contract1 := GetContract[TestData](v1Key)
		contract2 := GetContract[TestData](v2Key)
		
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
	
	const processTestKey ProcessKey = "process-test"
	contract := GetContract[ProcessData](processTestKey)
	
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