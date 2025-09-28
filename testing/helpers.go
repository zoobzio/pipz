// Package testing provides test utilities and helpers for pipz-based applications.
//
// This package includes mock processors, assertion helpers, and chaos testing
// tools to make testing pipz pipelines easier and more comprehensive.
//
// Example usage:
//
//	func TestMyPipeline(t *testing.T) {
//		mock := testing.NewMockProcessor[string](t, "mock-processor")
//		mock.WithReturn("processed", nil)
//
//		pipeline := pipz.NewSequence[string]("test-pipeline", mock)
//		result, err := pipeline.Process(context.Background(), "input")
//
//		require.NoError(t, err)
//		assert.Equal(t, "processed", result)
//		testing.AssertProcessed(t, mock, 1)
//	}
package testing

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	mathrand "math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/zoobzio/metricz"
	"github.com/zoobzio/pipz"
	"github.com/zoobzio/tracez"
)

// MockProcessor provides a configurable mock implementation of pipz.Chainable[T].
// It tracks calls, allows configuring return values and delays, and provides
// assertion methods for testing pipeline behavior.
type MockProcessor[T any] struct { //nolint:govet // fieldalignment: Test helper struct optimized for functionality over memory efficiency
	t           *testing.T
	name        string
	callCount   int64
	lastInput   T
	returnVal   T
	returnErr   error
	delay       time.Duration
	panicMsg    string
	mu          sync.RWMutex
	callHistory []MockCall[T]
	maxHistory  int
}

// MockCall represents a single call to the mock processor.
type MockCall[T any] struct {
	Input     T
	Timestamp time.Time
	Context   context.Context
}

// NewMockProcessor creates a new mock processor for testing.
// The processor tracks all calls and provides configurable behavior.
func NewMockProcessor[T any](t *testing.T, name string) *MockProcessor[T] {
	return &MockProcessor[T]{
		t:          t,
		name:       name,
		maxHistory: 100, // Keep last 100 calls by default
	}
}

// WithReturn configures the mock to return specific values.
// The mock will return these values for all subsequent calls.
func (m *MockProcessor[T]) WithReturn(val T, err error) *MockProcessor[T] {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.returnVal = val
	m.returnErr = err
	return m
}

// WithDelay configures the mock to delay execution.
// This is useful for testing timeout behavior and concurrent processing.
func (m *MockProcessor[T]) WithDelay(d time.Duration) *MockProcessor[T] {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.delay = d
	return m
}

// WithPanic configures the mock to panic with a specific message.
// This is useful for testing error recovery and panic handling.
func (m *MockProcessor[T]) WithPanic(msg string) *MockProcessor[T] {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.panicMsg = msg
	return m
}

// WithHistorySize configures how many calls to keep in history.
// Set to 0 to disable history tracking.
func (m *MockProcessor[T]) WithHistorySize(size int) *MockProcessor[T] {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.maxHistory = size
	if size == 0 {
		m.callHistory = nil
	} else if len(m.callHistory) > size {
		// Trim history to new size
		m.callHistory = m.callHistory[len(m.callHistory)-size:]
	}
	return m
}

// Name returns the name of the mock processor.
func (m *MockProcessor[T]) Name() pipz.Name {
	return pipz.Name(m.name)
}

// Metrics returns nil for mock processors.
func (*MockProcessor[T]) Metrics() *metricz.Registry {
	return nil
}

// Tracer returns nil for mock processors.
func (*MockProcessor[T]) Tracer() *tracez.Tracer {
	return nil
}

// Close returns nil for mock processors.
func (*MockProcessor[T]) Close() error {
	return nil
}

// Process implements pipz.Chainable[T]. It records the call and returns
// the configured values, potentially after a delay or panic.
func (m *MockProcessor[T]) Process(ctx context.Context, data T) (T, error) {
	// Record the call
	atomic.AddInt64(&m.callCount, 1)

	m.mu.Lock()
	m.lastInput = data
	if m.maxHistory > 0 {
		call := MockCall[T]{
			Input:     data,
			Timestamp: time.Now(),
			Context:   ctx,
		}
		m.callHistory = append(m.callHistory, call)
		if len(m.callHistory) > m.maxHistory {
			m.callHistory = m.callHistory[1:] // Remove oldest
		}
	}

	// Get configured behavior
	delay := m.delay
	returnVal := m.returnVal
	returnErr := m.returnErr
	panicMsg := m.panicMsg
	m.mu.Unlock()

	// Handle panic
	if panicMsg != "" {
		panic(panicMsg)
	}

	// Handle delay with context cancellation
	if delay > 0 {
		select {
		case <-time.After(delay):
			// Continue
		case <-ctx.Done():
			return data, ctx.Err()
		}
	}

	return returnVal, returnErr
}

// CallCount returns the number of times Process has been called.
func (m *MockProcessor[T]) CallCount() int {
	return int(atomic.LoadInt64(&m.callCount))
}

// LastInput returns the input from the most recent call.
func (m *MockProcessor[T]) LastInput() T {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lastInput
}

// CallHistory returns a copy of all recorded calls.
// Returns empty slice if history tracking is disabled.
func (m *MockProcessor[T]) CallHistory() []MockCall[T] {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.maxHistory == 0 {
		return nil
	}
	history := make([]MockCall[T], len(m.callHistory))
	copy(history, m.callHistory)
	return history
}

// Reset clears all call tracking and resets the mock to initial state.
func (m *MockProcessor[T]) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	atomic.StoreInt64(&m.callCount, 0)
	m.lastInput = *new(T)
	m.callHistory = nil
}

// Assertion Helpers

// AssertProcessed verifies that a mock processor was called exactly n times.
func AssertProcessed[T any](t *testing.T, mock *MockProcessor[T], expectedCalls int) {
	t.Helper()
	actualCalls := mock.CallCount()
	if actualCalls != expectedCalls {
		t.Errorf("expected mock processor %s to be called %d times, but was called %d times",
			mock.name, expectedCalls, actualCalls)
	}
}

// AssertNotProcessed verifies that a mock processor was never called.
func AssertNotProcessed[T any](t *testing.T, mock *MockProcessor[T]) {
	t.Helper()
	AssertProcessed(t, mock, 0)
}

// AssertProcessedWith verifies that a mock processor was called with specific input.
func AssertProcessedWith[T comparable](t *testing.T, mock *MockProcessor[T], expectedInput T) {
	t.Helper()
	if mock.CallCount() == 0 {
		t.Errorf("expected mock processor %s to be called with input %v, but it was never called",
			mock.name, expectedInput)
		return
	}

	actualInput := mock.LastInput()
	if actualInput != expectedInput {
		t.Errorf("expected mock processor %s to be called with input %v, but was called with %v",
			mock.name, expectedInput, actualInput)
	}
}

// AssertProcessedBetween verifies that a mock processor was called between min and max times.
func AssertProcessedBetween[T any](t *testing.T, mock *MockProcessor[T], minCalls, maxCalls int) {
	t.Helper()
	actualCalls := mock.CallCount()
	if actualCalls < minCalls || actualCalls > maxCalls {
		t.Errorf("expected mock processor %s to be called between %d and %d times, but was called %d times",
			mock.name, minCalls, maxCalls, actualCalls)
	}
}

// ChaosProcessor introduces controlled failures and delays for chaos testing.
// It wraps another processor and randomly introduces failures based on configured rates.
type ChaosProcessor[T any] struct { //nolint:govet // fieldalignment: Test helper struct optimized for functionality over memory efficiency
	name         string
	wrapped      pipz.Chainable[T]
	failureRate  float64
	latencyMin   time.Duration
	latencyMax   time.Duration
	timeoutRate  float64
	panicRate    float64
	rng          *mathrand.Rand
	mu           sync.Mutex
	totalCalls   int64
	failedCalls  int64
	timeoutCalls int64
	panicCalls   int64
}

// ChaosConfig holds configuration for chaos testing.
type ChaosConfig struct {
	FailureRate float64       // Probability of returning an error (0.0 to 1.0)
	LatencyMin  time.Duration // Minimum additional latency to inject
	LatencyMax  time.Duration // Maximum additional latency to inject
	TimeoutRate float64       // Probability of simulating timeout (0.0 to 1.0)
	PanicRate   float64       // Probability of panicking (0.0 to 1.0)
	Seed        int64         // Random seed for reproducible chaos (0 for random seed)
}

// NewChaosProcessor creates a chaos processor that wraps another processor.
func NewChaosProcessor[T any](name string, wrapped pipz.Chainable[T], config ChaosConfig) *ChaosProcessor[T] {
	seed := config.Seed
	if seed == 0 {
		// Use crypto/rand for better randomness
		var seedBytes [8]byte
		if _, err := rand.Read(seedBytes[:]); err != nil {
			// Fallback to time-based seed if crypto/rand fails
			seed = time.Now().UnixNano()
		} else {
			seed = int64(seedBytes[0])<<56 | int64(seedBytes[1])<<48 | int64(seedBytes[2])<<40 | int64(seedBytes[3])<<32 |
				int64(seedBytes[4])<<24 | int64(seedBytes[5])<<16 | int64(seedBytes[6])<<8 | int64(seedBytes[7])
		}
	}

	return &ChaosProcessor[T]{
		name:        name,
		wrapped:     wrapped,
		failureRate: config.FailureRate,
		latencyMin:  config.LatencyMin,
		latencyMax:  config.LatencyMax,
		timeoutRate: config.TimeoutRate,
		panicRate:   config.PanicRate,
		rng:         mathrand.New(mathrand.NewSource(seed)), //nolint:gosec // G404: Test utility uses weak RNG for deterministic chaos scenarios
	}
}

// Name returns the name of the chaos processor.
func (c *ChaosProcessor[T]) Name() pipz.Name {
	return pipz.Name(c.name)
}

// Metrics returns nil for chaos processors.
func (*ChaosProcessor[T]) Metrics() *metricz.Registry {
	return nil
}

// Tracer returns nil for chaos processors.
func (*ChaosProcessor[T]) Tracer() *tracez.Tracer {
	return nil
}

// Close returns nil for chaos processors.
func (*ChaosProcessor[T]) Close() error {
	return nil
}

// Process implements pipz.Chainable[T] with chaos injection.
func (c *ChaosProcessor[T]) Process(ctx context.Context, data T) (T, error) {
	atomic.AddInt64(&c.totalCalls, 1)

	c.mu.Lock()
	// Check for panic injection
	if c.rng.Float64() < c.panicRate {
		c.mu.Unlock()
		atomic.AddInt64(&c.panicCalls, 1)
		panic("chaos processor induced panic")
	}

	// Add latency if configured
	var latency time.Duration
	if c.latencyMax > c.latencyMin {
		latencyRange := c.latencyMax - c.latencyMin
		latency = c.latencyMin + time.Duration(c.rng.Int63n(int64(latencyRange)))
	} else if c.latencyMin > 0 {
		latency = c.latencyMin
	}

	// Check for timeout simulation
	simulateTimeout := c.rng.Float64() < c.timeoutRate

	// Check for failure injection
	injectFailure := c.rng.Float64() < c.failureRate

	c.mu.Unlock()

	// Apply latency with context cancellation
	if latency > 0 {
		select {
		case <-time.After(latency):
			// Continue
		case <-ctx.Done():
			return data, ctx.Err()
		}
	}

	// Simulate timeout
	if simulateTimeout {
		atomic.AddInt64(&c.timeoutCalls, 1)
		return data, context.DeadlineExceeded
	}

	// Call wrapped processor
	result, err := c.wrapped.Process(ctx, data)

	// Inject failure
	if injectFailure && err == nil {
		atomic.AddInt64(&c.failedCalls, 1)
		return data, errors.New("chaos processor induced failure")
	}

	return result, err
}

// Stats returns statistics about chaos injection.
func (c *ChaosProcessor[T]) Stats() ChaosStats {
	return ChaosStats{
		TotalCalls:   atomic.LoadInt64(&c.totalCalls),
		FailedCalls:  atomic.LoadInt64(&c.failedCalls),
		TimeoutCalls: atomic.LoadInt64(&c.timeoutCalls),
		PanicCalls:   atomic.LoadInt64(&c.panicCalls),
	}
}

// ChaosStats holds statistics about chaos injection.
type ChaosStats struct {
	TotalCalls   int64
	FailedCalls  int64
	TimeoutCalls int64
	PanicCalls   int64
}

// FailureRate returns the actual failure rate observed.
func (s ChaosStats) FailureRate() float64 {
	if s.TotalCalls == 0 {
		return 0
	}
	return float64(s.FailedCalls) / float64(s.TotalCalls)
}

// TimeoutRate returns the actual timeout rate observed.
func (s ChaosStats) TimeoutRate() float64 {
	if s.TotalCalls == 0 {
		return 0
	}
	return float64(s.TimeoutCalls) / float64(s.TotalCalls)
}

// PanicRate returns the actual panic rate observed.
func (s ChaosStats) PanicRate() float64 {
	if s.TotalCalls == 0 {
		return 0
	}
	return float64(s.PanicCalls) / float64(s.TotalCalls)
}

// String returns a human-readable representation of the stats.
func (s ChaosStats) String() string {
	return fmt.Sprintf("ChaosStats{Total: %d, Failed: %d (%.1f%%), Timeouts: %d (%.1f%%), Panics: %d (%.1f%%)}",
		s.TotalCalls, s.FailedCalls, s.FailureRate()*100,
		s.TimeoutCalls, s.TimeoutRate()*100,
		s.PanicCalls, s.PanicRate()*100)
}

// Helper Functions

// WaitForCalls waits for a mock processor to be called at least n times,
// with a timeout. Returns true if the expected calls were reached.
func WaitForCalls[T any](mock *MockProcessor[T], expectedCalls int, timeout time.Duration) bool {
	start := time.Now()
	for time.Since(start) < timeout {
		if mock.CallCount() >= expectedCalls {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

// ParallelTest runs a test function in parallel with multiple goroutines.
// Useful for testing concurrent behavior of processors.
func ParallelTest(t *testing.T, goroutines int, testFunc func(int)) {
	t.Helper()

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			testFunc(id)
		}(i)
	}

	wg.Wait()
}

// MeasureLatency measures the latency of a function call.
func MeasureLatency(fn func()) time.Duration {
	start := time.Now()
	fn()
	return time.Since(start)
}

// MeasureLatencyWithResult measures the latency of a function call and returns both the result and duration.
func MeasureLatencyWithResult[T any](fn func() T) (T, time.Duration) {
	start := time.Now()
	result := fn()
	return result, time.Since(start)
}
