package pipz

import (
	"github.com/vmihailenco/msgpack/v5"
)

// Encode serializes a value of type T to bytes using msgpack encoding.
// This is the standard encoding method used by pipz contracts.
func Encode[T any](value T) ([]byte, error) {
	return msgpack.Marshal(value)
}

// Decode deserializes bytes into a value of type T using msgpack decoding.
// This is the standard decoding method used by pipz contracts.
func Decode[T any](data []byte) (T, error) {
	var value T
	err := msgpack.Unmarshal(data, &value)
	return value, err
}