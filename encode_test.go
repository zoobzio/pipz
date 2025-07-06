package pipz

import (
	"bytes"
	"testing"
)

func TestEncoding(t *testing.T) {
	t.Run("EncodeDecode", func(t *testing.T) {
		type TestStruct struct {
			Name    string
			Age     int
			Tags    []string
			Enabled bool
		}
		
		original := TestStruct{
			Name:    "Alice",
			Age:     30,
			Tags:    []string{"developer", "golang"},
			Enabled: true,
		}
		
		// Encode
		encoded, err := Encode(original)
		if err != nil {
			t.Fatalf("Encode failed: %v", err)
		}
		
		if len(encoded) == 0 {
			t.Fatal("Encoded data is empty")
		}
		
		// Decode
		decoded, err := Decode[TestStruct](encoded)
		if err != nil {
			t.Fatalf("Decode failed: %v", err)
		}
		
		// Verify
		if decoded.Name != original.Name {
			t.Errorf("Name mismatch: got %s, want %s", decoded.Name, original.Name)
		}
		if decoded.Age != original.Age {
			t.Errorf("Age mismatch: got %d, want %d", decoded.Age, original.Age)
		}
		if len(decoded.Tags) != len(original.Tags) {
			t.Errorf("Tags length mismatch: got %d, want %d", len(decoded.Tags), len(original.Tags))
		}
		for i, tag := range decoded.Tags {
			if tag != original.Tags[i] {
				t.Errorf("Tag[%d] mismatch: got %s, want %s", i, tag, original.Tags[i])
			}
		}
		if decoded.Enabled != original.Enabled {
			t.Errorf("Enabled mismatch: got %v, want %v", decoded.Enabled, original.Enabled)
		}
	})
	
	t.Run("EncodeNil", func(t *testing.T) {
		var nilValue *string
		encoded, err := Encode(nilValue)
		if err != nil {
			t.Fatalf("Encode nil failed: %v", err)
		}
		
		// msgpack encodes nil as a single byte
		if len(encoded) != 1 {
			t.Errorf("Expected encoded nil to be 1 byte, got %d", len(encoded))
		}
	})
	
	t.Run("DecodeInvalidData", func(t *testing.T) {
		_, err := Decode[string]([]byte{0xFF, 0xFF, 0xFF})
		if err == nil {
			t.Fatal("Expected error decoding invalid data")
		}
	})
	
	t.Run("DecodeTypeMismatch", func(t *testing.T) {
		// Encode a string
		encoded, err := Encode("hello")
		if err != nil {
			t.Fatal(err)
		}
		
		// Try to decode into an int
		_, err = Decode[int](encoded)
		if err == nil {
			t.Fatal("Expected error decoding string into int")
		}
	})
	
	t.Run("ComplexTypes", func(t *testing.T) {
		type Address struct {
			Street string
			City   string
		}
		
		type Metadata struct {
			Level    int
			Verified bool
			Tags     []string
		}
		
		type Person struct {
			Name      string
			Addresses []Address
			Metadata  Metadata
		}
		
		original := Person{
			Name: "Bob",
			Addresses: []Address{
				{Street: "123 Main St", City: "NYC"},
				{Street: "456 Oak Ave", City: "LA"},
			},
			Metadata: Metadata{
				Level:    42,
				Verified: true,
				Tags:     []string{"vip", "premium"},
			},
		}
		
		encoded, err := Encode(original)
		if err != nil {
			t.Fatal(err)
		}
		
		decoded, err := Decode[Person](encoded)
		if err != nil {
			t.Fatal(err)
		}
		
		if decoded.Name != original.Name {
			t.Errorf("Name mismatch")
		}
		if len(decoded.Addresses) != 2 {
			t.Errorf("Expected 2 addresses, got %d", len(decoded.Addresses))
		}
		if decoded.Addresses[0].Street != "123 Main St" {
			t.Errorf("Address mismatch")
		}
		if decoded.Metadata.Level != 42 {
			t.Errorf("Metadata level mismatch")
		}
		if !decoded.Metadata.Verified {
			t.Errorf("Metadata verified mismatch")
		}
		if len(decoded.Metadata.Tags) != 2 {
			t.Errorf("Expected 2 tags, got %d", len(decoded.Metadata.Tags))
		}
	})
	
	t.Run("LargeData", func(t *testing.T) {
		// Test with larger data
		type LargeStruct struct {
			Data []byte
		}
		
		original := LargeStruct{
			Data: bytes.Repeat([]byte("x"), 1024*10), // 10KB
		}
		
		encoded, err := Encode(original)
		if err != nil {
			t.Fatal(err)
		}
		
		decoded, err := Decode[LargeStruct](encoded)
		if err != nil {
			t.Fatal(err)
		}
		
		if !bytes.Equal(decoded.Data, original.Data) {
			t.Error("Large data mismatch")
		}
	})
}