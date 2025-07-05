package pipz

import (
	"fmt"
	"sync"
)

// ByteProcessor defines a function that transforms byte slices.
// It returns the processed bytes or an error if processing fails.
type ByteProcessor func([]byte) ([]byte, error)

// pipeline is a singleton that manages chains of byte processors.
// It provides thread-safe registration and execution of processor chains.
type pipeline struct {
	chains map[string][]ByteProcessor
	mu     sync.RWMutex
}

// service is the global singleton instance of the pipeline.
var service *pipeline

// init initializes the pipeline singleton at package import time.
func init() {
	service = &pipeline{
		chains: make(map[string][]ByteProcessor),
	}
}

// Register associates a chain of ByteProcessor functions with a string key.
// The processors will be executed in the order they are provided.
// If a key already exists, it will be overwritten with the new processor chain.
func Register(key string, processors ...ByteProcessor) error {
	service.mu.Lock()
	defer service.mu.Unlock()

	service.chains[key] = processors
	return nil
}

// Process executes the processor chain associated with the given key.
// The processors are executed sequentially, with each processor's output
// becoming the next processor's input. If any processor returns an error,
// execution stops and the error is returned. If the key does not exist,
// an error is returned.
func Process(key string, data []byte) ([]byte, error) {
	service.mu.RLock()
	chain, exists := service.chains[key]
	service.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("chain %q not found", key)
	}

	result := data
	for _, processor := range chain {
		var err error
		result, err = processor(result)
		if err != nil {
			return nil, err
		}
	}

	return result, nil
}
