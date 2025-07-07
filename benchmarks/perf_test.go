package benchmarks

import (
	"testing"
	
	"pipz"
)

// Simple data structure for performance testing
type PerfData struct {
	ID    int
	Value string
	Score float64
}

// Simple processors
func doubleScore(d PerfData) PerfData {
	d.Score *= 2
	return d
}

func addPrefix(d PerfData) PerfData {
	d.Value = "PREFIX_" + d.Value
	return d
}

func incrementID(d PerfData) PerfData {
	d.ID++
	return d
}

// Benchmark pipz pipeline
func BenchmarkPipzPipeline(b *testing.B) {
	pipeline := pipz.NewContract[PerfData]()
	pipeline.Register(
		pipz.Transform(doubleScore),
		pipz.Transform(addPrefix),
		pipz.Transform(incrementID),
	)

	data := PerfData{
		ID:    100,
		Value: "test",
		Score: 42.5,
	}

	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		_, err := pipeline.Process(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Benchmark direct function calls (baseline)
func BenchmarkDirectCalls(b *testing.B) {
	data := PerfData{
		ID:    100,
		Value: "test",
		Score: 42.5,
	}

	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		result := doubleScore(data)
		result = addPrefix(result)
		result = incrementID(result)
		_ = result
	}
}

// Benchmark function slice approach
func BenchmarkFunctionSlice(b *testing.B) {
	processors := []func(PerfData) PerfData{
		doubleScore,
		addPrefix,
		incrementID,
	}

	data := PerfData{
		ID:    100,
		Value: "test",
		Score: 42.5,
	}

	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		result := data
		for _, proc := range processors {
			result = proc(result)
		}
		_ = result
	}
}

// Benchmark interface approach
type Processor interface {
	Process(PerfData) PerfData
}

type scoreDoubler struct{}
func (s scoreDoubler) Process(d PerfData) PerfData { return doubleScore(d) }

type prefixAdder struct{}
func (p prefixAdder) Process(d PerfData) PerfData { return addPrefix(d) }

type idIncrementer struct{}
func (i idIncrementer) Process(d PerfData) PerfData { return incrementID(d) }

func BenchmarkInterfaceApproach(b *testing.B) {
	processors := []Processor{
		scoreDoubler{},
		prefixAdder{},
		idIncrementer{},
	}

	data := PerfData{
		ID:    100,
		Value: "test",
		Score: 42.5,
	}

	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		result := data
		for _, proc := range processors {
			result = proc.Process(result)
		}
		_ = result
	}
}

// Benchmark pipz with many processors
func BenchmarkPipzManyProcessors(b *testing.B) {
	pipeline := pipz.NewContract[PerfData]()
	
	// Register 10 processors
	for i := 0; i < 10; i++ {
		pipeline.Register(
			pipz.Transform(doubleScore),
			pipz.Transform(addPrefix),
			pipz.Transform(incrementID),
		)
	}

	data := PerfData{
		ID:    100,
		Value: "test",
		Score: 42.5,
	}

	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		_, err := pipeline.Process(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Benchmark chain performance
func BenchmarkPipzChain(b *testing.B) {
	// Create 3 separate pipelines
	pipeline1 := pipz.NewContract[PerfData]()
	pipeline1.Register(pipz.Transform(doubleScore))

	pipeline2 := pipz.NewContract[PerfData]()
	pipeline2.Register(pipz.Transform(addPrefix))

	pipeline3 := pipz.NewContract[PerfData]()
	pipeline3.Register(pipz.Transform(incrementID))

	// Chain them
	chain := pipz.NewChain[PerfData]()
	chain.Add(pipeline1, pipeline2, pipeline3)

	data := PerfData{
		ID:    100,
		Value: "test",
		Score: 42.5,
	}

	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		_, err := chain.Process(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Benchmark memory allocations
func BenchmarkPipzAllocations(b *testing.B) {
	pipeline := pipz.NewContract[PerfData]()
	pipeline.Register(
		pipz.Transform(doubleScore),
		pipz.Transform(addPrefix),
		pipz.Transform(incrementID),
	)

	data := PerfData{
		ID:    100,
		Value: "test",
		Score: 42.5,
	}

	b.ReportAllocs()
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		_, err := pipeline.Process(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Benchmark parallel execution
func BenchmarkPipzParallel(b *testing.B) {
	pipeline := pipz.NewContract[PerfData]()
	pipeline.Register(
		pipz.Transform(doubleScore),
		pipz.Transform(addPrefix),
		pipz.Transform(incrementID),
	)

	data := PerfData{
		ID:    100,
		Value: "test",
		Score: 42.5,
	}

	b.ResetTimer()
	
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := pipeline.Process(data)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}