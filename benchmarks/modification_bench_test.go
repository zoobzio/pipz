package benchmarks

import (
	"testing"

	"pipz"
)

// Benchmark pipeline modification operations
func BenchmarkPushTail(b *testing.B) {
	processor := pipz.Transform(doubleScore)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		contract := pipz.NewContract[PerfData]()
		contract.PushTail(processor)
	}
}

func BenchmarkPushHead(b *testing.B) {
	processor := pipz.Transform(doubleScore)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		contract := pipz.NewContract[PerfData]()
		contract.PushHead(processor)
	}
}

func BenchmarkPushTailMultiple(b *testing.B) {
	processors := []pipz.Processor[PerfData]{
		pipz.Transform(doubleScore),
		pipz.Transform(addPrefix),
		pipz.Transform(incrementID),
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		contract := pipz.NewContract[PerfData]()
		contract.PushTail(processors...)
	}
}

func BenchmarkPushHeadMultiple(b *testing.B) {
	processors := []pipz.Processor[PerfData]{
		pipz.Transform(doubleScore),
		pipz.Transform(addPrefix),
		pipz.Transform(incrementID),
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		contract := pipz.NewContract[PerfData]()
		contract.PushHead(processors...)
	}
}

func BenchmarkPopHead(b *testing.B) {
	contract := pipz.NewContract[PerfData]()
	
	// Pre-populate with processors
	for i := 0; i < 1000; i++ {
		contract.PushTail(pipz.Transform(doubleScore))
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if contract.IsEmpty() {
			// Repopulate if empty
			for j := 0; j < 1000; j++ {
				contract.PushTail(pipz.Transform(doubleScore))
			}
		}
		contract.PopHead()
	}
}

func BenchmarkPopTail(b *testing.B) {
	contract := pipz.NewContract[PerfData]()
	
	// Pre-populate with processors
	for i := 0; i < 1000; i++ {
		contract.PushTail(pipz.Transform(doubleScore))
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if contract.IsEmpty() {
			// Repopulate if empty
			for j := 0; j < 1000; j++ {
				contract.PushTail(pipz.Transform(doubleScore))
			}
		}
		contract.PopTail()
	}
}

func BenchmarkInsertAt(b *testing.B) {
	processor := pipz.Transform(doubleScore)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		contract := pipz.NewContract[PerfData]()
		// Pre-populate with some processors
		contract.PushTail(pipz.Transform(addPrefix), pipz.Transform(incrementID))
		// Insert in the middle
		contract.InsertAt(1, processor)
	}
}

func BenchmarkRemoveAt(b *testing.B) {
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		contract := pipz.NewContract[PerfData]()
		// Pre-populate with processors
		contract.PushTail(
			pipz.Transform(doubleScore),
			pipz.Transform(addPrefix),
			pipz.Transform(incrementID),
		)
		// Remove from middle
		contract.RemoveAt(1)
	}
}

func BenchmarkReplaceAt(b *testing.B) {
	replacement := pipz.Transform(doubleScore)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		contract := pipz.NewContract[PerfData]()
		// Pre-populate with processors
		contract.PushTail(
			pipz.Transform(addPrefix),
			pipz.Transform(incrementID),
		)
		// Replace first processor
		contract.ReplaceAt(0, replacement)
	}
}

func BenchmarkMoveToHead(b *testing.B) {
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		contract := pipz.NewContract[PerfData]()
		// Pre-populate with processors
		contract.PushTail(
			pipz.Transform(doubleScore),
			pipz.Transform(addPrefix),
			pipz.Transform(incrementID),
		)
		// Move last to head
		contract.MoveToHead(2)
	}
}

func BenchmarkMoveToTail(b *testing.B) {
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		contract := pipz.NewContract[PerfData]()
		// Pre-populate with processors
		contract.PushTail(
			pipz.Transform(doubleScore),
			pipz.Transform(addPrefix),
			pipz.Transform(incrementID),
		)
		// Move first to tail
		contract.MoveToTail(0)
	}
}

func BenchmarkSwap(b *testing.B) {
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		contract := pipz.NewContract[PerfData]()
		// Pre-populate with processors
		contract.PushTail(
			pipz.Transform(doubleScore),
			pipz.Transform(addPrefix),
			pipz.Transform(incrementID),
		)
		// Swap first and last
		contract.Swap(0, 2)
	}
}

func BenchmarkReverse(b *testing.B) {
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		contract := pipz.NewContract[PerfData]()
		// Pre-populate with processors
		contract.PushTail(
			pipz.Transform(doubleScore),
			pipz.Transform(addPrefix),
			pipz.Transform(incrementID),
		)
		// Reverse pipeline
		contract.Reverse()
	}
}

// Benchmark pipeline modification with large pipelines
func BenchmarkLargePipelineModification(b *testing.B) {
	const pipelineSize = 100

	b.Run("PushTail", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			contract := pipz.NewContract[PerfData]()
			// Build large pipeline
			for j := 0; j < pipelineSize; j++ {
				contract.PushTail(pipz.Transform(doubleScore))
			}
		}
	})

	b.Run("PushHead", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			contract := pipz.NewContract[PerfData]()
			// Build large pipeline from head
			for j := 0; j < pipelineSize; j++ {
				contract.PushHead(pipz.Transform(doubleScore))
			}
		}
	})

	b.Run("InsertMiddle", func(b *testing.B) {
		processor := pipz.Transform(doubleScore)

		for i := 0; i < b.N; i++ {
			contract := pipz.NewContract[PerfData]()
			// Pre-populate large pipeline
			for j := 0; j < pipelineSize; j++ {
				contract.PushTail(pipz.Transform(addPrefix))
			}
			// Insert in middle
			contract.InsertAt(pipelineSize/2, processor)
		}
	})

	b.Run("RemoveMiddle", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			contract := pipz.NewContract[PerfData]()
			// Pre-populate large pipeline
			for j := 0; j < pipelineSize; j++ {
				contract.PushTail(pipz.Transform(addPrefix))
			}
			// Remove from middle
			contract.RemoveAt(pipelineSize / 2)
		}
	})
}

// Benchmark concurrent modification performance
func BenchmarkConcurrentModification(b *testing.B) {
	contract := pipz.NewContract[PerfData]()
	
	// Pre-populate
	for i := 0; i < 10; i++ {
		contract.PushTail(pipz.Transform(doubleScore))
	}

	data := PerfData{ID: 1, Value: "test", Score: 42.0}

	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		counter := 0
		for pb.Next() {
			// Mix of read and write operations
			switch counter % 4 {
			case 0:
				// Read operation
				contract.Process(data)
			case 1:
				// Add operation
				contract.PushTail(pipz.Transform(incrementID))
			case 2:
				// Remove operation (if not empty)
				if !contract.IsEmpty() {
					contract.PopTail()
				}
			case 3:
				// Move operation (if has elements)
				if contract.Len() > 1 {
					contract.Swap(0, contract.Len()-1)
				}
			}
			counter++
		}
	})
}

// Benchmark memory allocations for modifications
func BenchmarkModificationAllocations(b *testing.B) {
	processor := pipz.Transform(doubleScore)

	b.Run("PushTail", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			contract := pipz.NewContract[PerfData]()
			contract.PushTail(processor)
		}
	})

	b.Run("PushHead", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			contract := pipz.NewContract[PerfData]()
			contract.PushHead(processor)
		}
	})

	b.Run("InsertAt", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			contract := pipz.NewContract[PerfData]()
			contract.PushTail(pipz.Transform(addPrefix))
			contract.InsertAt(0, processor)
		}
	})

	b.Run("Reverse", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			contract := pipz.NewContract[PerfData]()
			contract.PushTail(processor, processor, processor)
			contract.Reverse()
		}
	})
}

// Benchmark comparison: Register vs PushTail
func BenchmarkRegisterVsPushTail(b *testing.B) {
	processor := pipz.Transform(doubleScore)

	b.Run("Register", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			contract := pipz.NewContract[PerfData]()
			contract.Register(processor)
		}
	})

	b.Run("PushTail", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			contract := pipz.NewContract[PerfData]()
			contract.PushTail(processor)
		}
	})
}

// Benchmark Chain modification operations
func BenchmarkChainModification(b *testing.B) {
	chainable := pipz.NewContract[PerfData]()
	chainable.Register(pipz.Transform(doubleScore))

	b.Run("ChainPushTail", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			chain := pipz.NewChain[PerfData]()
			chain.PushTail(chainable)
		}
	})

	b.Run("ChainPushHead", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			chain := pipz.NewChain[PerfData]()
			chain.PushHead(chainable)
		}
	})

	b.Run("ChainInsertAt", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			chain := pipz.NewChain[PerfData]()
			chain.PushTail(chainable)
			chain.InsertAt(0, chainable)
		}
	})
}