package pipz

import (
	"reflect"
	"sync"
	"testing"
)

func TestTypeName(t *testing.T) {
	t.Run("BasicTypes", func(t *testing.T) {
		// Test basic types
		stringName := typeName[string]()
		if stringName != "string" {
			t.Errorf("expected 'string', got %s", stringName)
		}
		
		intName := typeName[int]()
		if intName != "int" {
			t.Errorf("expected 'int', got %s", intName)
		}
		
		boolName := typeName[bool]()
		if boolName != "bool" {
			t.Errorf("expected 'bool', got %s", boolName)
		}
	})
	
	t.Run("StructTypes", func(t *testing.T) {
		type TestStruct struct {
			ID   int
			Name string
		}
		
		structName := typeName[TestStruct]()
		expected := "pipz.TestStruct"
		if structName != expected {
			t.Errorf("expected '%s', got %s", expected, structName)
		}
	})
	
	t.Run("PointerTypes", func(t *testing.T) {
		type TestStruct struct {
			Value int
		}
		
		ptrName := typeName[*TestStruct]()
		expected := "*pipz.TestStruct"
		if ptrName != expected {
			t.Errorf("expected '%s', got %s", expected, ptrName)
		}
	})
	
	t.Run("SliceTypes", func(t *testing.T) {
		sliceName := typeName[[]string]()
		expected := "[]string"
		if sliceName != expected {
			t.Errorf("expected '%s', got %s", expected, sliceName)
		}
	})
	
	t.Run("MapTypes", func(t *testing.T) {
		mapName := typeName[map[string]int]()
		expected := "map[string]int"
		if mapName != expected {
			t.Errorf("expected '%s', got %s", expected, mapName)
		}
	})
	
	t.Run("CustomTypes", func(t *testing.T) {
		type UserKey string
		type User struct {
			ID   string
			Name string
		}
		
		keyName := typeName[UserKey]()
		expectedKey := "pipz.UserKey"
		if keyName != expectedKey {
			t.Errorf("expected '%s', got %s", expectedKey, keyName)
		}
		
		userName := typeName[User]()
		expectedUser := "pipz.User"
		if userName != expectedUser {
			t.Errorf("expected '%s', got %s", expectedUser, userName)
		}
	})
}

func TestTypeNameCaching(t *testing.T) {
	t.Run("CacheHit", func(t *testing.T) {
		// Clear cache first
		typeCache = make(map[reflect.Type]string)
		
		// First call should populate cache
		name1 := typeName[string]()
		
		// Second call should hit cache
		name2 := typeName[string]()
		
		if name1 != name2 {
			t.Errorf("cached result differs: %s != %s", name1, name2)
		}
		
		// Verify cache was used (should have entry)
		var stringType string
		typ := reflect.TypeOf(stringType)
		cacheMu.RLock()
		_, exists := typeCache[typ]
		cacheMu.RUnlock()
		
		if !exists {
			t.Error("expected type to be cached")
		}
	})
	
	t.Run("MultipleConcurrentAccess", func(t *testing.T) {
		// Clear cache
		typeCache = make(map[reflect.Type]string)
		
		const numGoroutines = 100
		var wg sync.WaitGroup
		results := make([]string, numGoroutines)
		
		// Launch multiple goroutines calling typeName concurrently
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				results[index] = typeName[int]()
			}(i)
		}
		
		wg.Wait()
		
		// All results should be identical
		expected := "int"
		for i, result := range results {
			if result != expected {
				t.Errorf("goroutine %d got different result: %s", i, result)
			}
		}
		
		// Cache should have exactly one entry for int
		var intType int
		typ := reflect.TypeOf(intType)
		cacheMu.RLock()
		cached, exists := typeCache[typ]
		cacheMu.RUnlock()
		
		if !exists {
			t.Error("expected int type to be cached")
		}
		if cached != expected {
			t.Errorf("cached value incorrect: %s", cached)
		}
	})
}

func TestSignature(t *testing.T) {
	t.Run("BasicSignature", func(t *testing.T) {
		type TestKey string
		type TestData struct {
			Value int
		}
		
		signature := Signature[TestData, TestKey](TestKey("test"))
		expected := "pipz.TestData:pipz.TestKey:test"
		
		if signature != expected {
			t.Errorf("expected '%s', got '%s'", expected, signature)
		}
	})
	
	t.Run("DifferentKeyValues", func(t *testing.T) {
		type UserKey string
		type User struct {
			ID string
		}
		
		sig1 := Signature[User, UserKey](UserKey("v1"))
		sig2 := Signature[User, UserKey](UserKey("v2"))
		
		if sig1 == sig2 {
			t.Error("different key values should produce different signatures")
		}
		
		expected1 := "pipz.User:pipz.UserKey:v1"
		expected2 := "pipz.User:pipz.UserKey:v2"
		
		if sig1 != expected1 {
			t.Errorf("sig1: expected '%s', got '%s'", expected1, sig1)
		}
		if sig2 != expected2 {
			t.Errorf("sig2: expected '%s', got '%s'", expected2, sig2)
		}
	})
	
	t.Run("DifferentKeyTypes", func(t *testing.T) {
		type KeyA string
		type KeyB string
		type Data struct {
			Value int
		}
		
		sigA := Signature[Data, KeyA](KeyA("test"))
		sigB := Signature[Data, KeyB](KeyB("test"))
		
		if sigA == sigB {
			t.Error("different key types should produce different signatures")
		}
		
		expectedA := "pipz.Data:pipz.KeyA:test"
		expectedB := "pipz.Data:pipz.KeyB:test"
		
		if sigA != expectedA {
			t.Errorf("sigA: expected '%s', got '%s'", expectedA, sigA)
		}
		if sigB != expectedB {
			t.Errorf("sigB: expected '%s', got '%s'", expectedB, sigB)
		}
	})
	
	t.Run("DifferentDataTypes", func(t *testing.T) {
		type TestKey string
		type DataA struct {
			A string
		}
		type DataB struct {
			B int
		}
		
		sigA := Signature[DataA, TestKey](TestKey("test"))
		sigB := Signature[DataB, TestKey](TestKey("test"))
		
		if sigA == sigB {
			t.Error("different data types should produce different signatures")
		}
		
		expectedA := "pipz.DataA:pipz.TestKey:test"
		expectedB := "pipz.DataB:pipz.TestKey:test"
		
		if sigA != expectedA {
			t.Errorf("sigA: expected '%s', got '%s'", expectedA, sigA)
		}
		if sigB != expectedB {
			t.Errorf("sigB: expected '%s', got '%s'", expectedB, sigB)
		}
	})
	
	t.Run("ComplexTypes", func(t *testing.T) {
		type ComplexKey struct {
			ID   string
			Type string
		}
		type ComplexData map[string][]int
		
		key := ComplexKey{ID: "test", Type: "example"}
		signature := Signature[ComplexData, ComplexKey](key)
		
		// Should contain the type names and key value
		expectedPrefix := "pipz.ComplexData:pipz.ComplexKey:"
		if !startsWith(signature, expectedPrefix) {
			t.Errorf("signature should start with '%s', got '%s'", expectedPrefix, signature)
		}
	})
	
	t.Run("NumericKeys", func(t *testing.T) {
		type IntKey int
		type Data string
		
		sig1 := Signature[Data, IntKey](IntKey(1))
		sig2 := Signature[Data, IntKey](IntKey(2))
		
		if sig1 == sig2 {
			t.Error("different numeric keys should produce different signatures")
		}
		
		expected1 := "pipz.Data:pipz.IntKey:1"
		expected2 := "pipz.Data:pipz.IntKey:2"
		
		if sig1 != expected1 {
			t.Errorf("sig1: expected '%s', got '%s'", expected1, sig1)
		}
		if sig2 != expected2 {
			t.Errorf("sig2: expected '%s', got '%s'", expected2, sig2)
		}
	})
}

func TestSignatureUniqueness(t *testing.T) {
	t.Run("UniquenessAcrossAllCombinations", func(t *testing.T) {
		type KeyA string
		type KeyB string
		type DataX struct{ X int }
		type DataY struct{ Y string }
		
		signatures := map[string]bool{
			Signature[DataX, KeyA](KeyA("v1")): true,
			Signature[DataX, KeyA](KeyA("v2")): true,
			Signature[DataX, KeyB](KeyB("v1")): true,
			Signature[DataX, KeyB](KeyB("v2")): true,
			Signature[DataY, KeyA](KeyA("v1")): true,
			Signature[DataY, KeyA](KeyA("v2")): true,
			Signature[DataY, KeyB](KeyB("v1")): true,
			Signature[DataY, KeyB](KeyB("v2")): true,
		}
		
		if len(signatures) != 8 {
			t.Errorf("expected 8 unique signatures, got %d", len(signatures))
		}
	})
}

// Helper function since strings.HasPrefix might not be available
func startsWith(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}