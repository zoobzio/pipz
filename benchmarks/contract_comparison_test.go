package benchmarks

import (
	"testing"
	
	"pipz"
)

// Test data structure
type BenchData struct {
	ID    int
	Name  string
	Value float64
	Count int
}

// Simple processors for benchmarking
func incrementCount(d BenchData) ([]byte, error) {
	d.Count++
	return pipz.Encode(d)
}

func validateData(d BenchData) ([]byte, error) {
	if d.ID <= 0 {
		return nil, nil // Read-only validation
	}
	return nil, nil
}

func doubleValue(d BenchData) ([]byte, error) {
	d.Value *= 2
	return pipz.Encode(d)
}

// BenchmarkGlobalContract tests the standard Contract with global registry
func BenchmarkGlobalContract(b *testing.B) {
	// Setup
	type BenchKey string
	const benchKey BenchKey = "bench-global"
	contract := pipz.GetContract[BenchData](benchKey)
	contract.Register(
		validateData,
		incrementCount,
		doubleValue,
	)
	
	data := BenchData{
		ID:    123,
		Name:  "test",
		Value: 100.0,
		Count: 0,
	}
	
	b.ResetTimer()
	
	// Benchmark
	for i := 0; i < b.N; i++ {
		_, err := contract.Process(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkSimpleContract tests the SimpleContract without global registry
func BenchmarkSimpleContract(b *testing.B) {
	// Setup
	contract := pipz.NewSimpleContract[BenchData]()
	contract.Register(
		validateData,
		incrementCount,
		doubleValue,
	)
	
	data := BenchData{
		ID:    123,
		Name:  "test",
		Value: 100.0,
		Count: 0,
	}
	
	b.ResetTimer()
	
	// Benchmark
	for i := 0; i < b.N; i++ {
		_, err := contract.Process(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkGlobalContractConcurrent tests concurrent access to global registry
func BenchmarkGlobalContractConcurrent(b *testing.B) {
	// Setup
	type BenchKey string
	const benchKey BenchKey = "bench-global-concurrent"
	contract := pipz.GetContract[BenchData](benchKey)
	contract.Register(
		validateData,
		incrementCount,
		doubleValue,
	)
	
	data := BenchData{
		ID:    123,
		Name:  "test",
		Value: 100.0,
		Count: 0,
	}
	
	b.ResetTimer()
	
	// Benchmark with parallelism
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := contract.Process(data)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkSimpleContractConcurrent tests concurrent access without global locks
func BenchmarkSimpleContractConcurrent(b *testing.B) {
	// Setup
	contract := pipz.NewSimpleContract[BenchData]()
	contract.Register(
		validateData,
		incrementCount,
		doubleValue,
	)
	
	data := BenchData{
		ID:    123,
		Name:  "test",
		Value: 100.0,
		Count: 0,
	}
	
	b.ResetTimer()
	
	// Benchmark with parallelism
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := contract.Process(data)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkGlobalContractLookup tests the cost of repeated contract lookups
func BenchmarkGlobalContractLookup(b *testing.B) {
	// Setup
	type BenchKey string
	const benchKey BenchKey = "bench-lookup"
	
	// Pre-register a contract
	setupContract := pipz.GetContract[BenchData](benchKey)
	setupContract.Register(incrementCount)
	
	data := BenchData{ID: 1, Name: "test", Value: 100.0}
	
	b.ResetTimer()
	
	// Benchmark - includes lookup cost each time
	for i := 0; i < b.N; i++ {
		contract := pipz.GetContract[BenchData](benchKey)
		_, err := contract.Process(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkGlobalContractCached tests performance with cached contract reference
func BenchmarkGlobalContractCached(b *testing.B) {
	// Setup
	type BenchKey string
	const benchKey BenchKey = "bench-cached"
	contract := pipz.GetContract[BenchData](benchKey)
	contract.Register(incrementCount)
	
	data := BenchData{ID: 1, Name: "test", Value: 100.0}
	
	b.ResetTimer()
	
	// Benchmark - no lookup cost
	for i := 0; i < b.N; i++ {
		_, err := contract.Process(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}