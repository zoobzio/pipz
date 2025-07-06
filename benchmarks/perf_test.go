package benchmarks

import (
	"testing"
	"pipz"
)

// Simple test struct
type PerfData struct {
	ID    int
	Name  string
	Count int
}

// Benchmark single processor pipeline
func BenchmarkSingleProcessorPipeline(b *testing.B) {
	type K string
	const perfKey K = "perf"
	
	processor := func(d PerfData) ([]byte, error) {
		d.Count++
		return pipz.Encode(d)
	}
	
	c := pipz.GetContract[PerfData](perfKey)
	c.Register(processor)
	
	data := PerfData{ID: 1, Name: "test", Count: 0}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Process(data)
	}
}

// Benchmark three processor pipeline
func BenchmarkThreeProcessorPipeline(b *testing.B) {
	type K string
	const threeKey K = "three"
	
	proc1 := func(d PerfData) ([]byte, error) {
		d.Count++
		return pipz.Encode(d)
	}
	
	proc2 := func(d PerfData) ([]byte, error) {
		if d.Count > 100 {
			d.Name = "high"
		}
		return nil, nil // No change
	}
	
	proc3 := func(d PerfData) ([]byte, error) {
		d.ID = d.ID * 2
		return pipz.Encode(d)
	}
	
	c := pipz.GetContract[PerfData](threeKey)
	c.Register(proc1, proc2, proc3)
	
	data := PerfData{ID: 1, Name: "test", Count: 0}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Process(data)
	}
}

// Benchmark read-only pipeline (validation)
func BenchmarkReadOnlyPipeline(b *testing.B) {
	type K string
	const readonlyKey K = "readonly"
	
	validate1 := func(d PerfData) ([]byte, error) {
		if d.ID <= 0 {
			return nil, nil
		}
		return nil, nil
	}
	
	validate2 := func(d PerfData) ([]byte, error) {
		if len(d.Name) == 0 {
			return nil, nil
		}
		return nil, nil
	}
	
	validate3 := func(d PerfData) ([]byte, error) {
		if d.Count < 0 {
			return nil, nil
		}
		return nil, nil
	}
	
	c := pipz.GetContract[PerfData](readonlyKey)
	c.Register(validate1, validate2, validate3)
	
	data := PerfData{ID: 1, Name: "test", Count: 10}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Process(data)
	}
}