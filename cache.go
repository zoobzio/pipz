// Package pipz provides a type-safe pipeline processing system with
// generic contracts and a unified byte-based registry.
package pipz

import (
	"fmt"
	"reflect"
	"sync"
)

var (
	// typeCache stores the string representation of types to avoid repeated reflection.
	typeCache = make(map[reflect.Type]string)
	// cacheMu protects concurrent access to the type cache.
	cacheMu   sync.RWMutex
)

// typeName returns the cached string representation of a type T.
// The result is cached after the first call for each unique type,
// making subsequent calls efficient. This function is safe for concurrent use.
func typeName[T any]() string {
	var zero T
	typ := reflect.TypeOf(zero)
	
	cacheMu.RLock()
	if name, ok := typeCache[typ]; ok {
		cacheMu.RUnlock()
		return name
	}
	cacheMu.RUnlock()
	
	cacheMu.Lock()
	defer cacheMu.Unlock()
	
	// Double-check after acquiring write lock
	if name, ok := typeCache[typ]; ok {
		return name
	}
	
	name := typ.String()
	typeCache[typ] = name
	return name
}

// Signature returns a unique signature string for the contract identified by
// key type K, key value, and data type T. Each type is cached independently,
// allowing efficient reuse across different combinations. The signature format
// is "K:keyValue:T" where K and T are the string representations of the types.
func Signature[K comparable, T any](key K) string {
	keyType := typeName[K]()
	valueType := typeName[T]()
	return fmt.Sprintf("%s:%v:%s", keyType, key, valueType)
}