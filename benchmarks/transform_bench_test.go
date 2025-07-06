package benchmarks

import (
	"testing"
	
	"pipz"
	"pipz/examples"
)

// BenchmarkTransformPipeline tests the performance of a data transformation pipeline
func BenchmarkTransformPipeline(b *testing.B) {
	// Setup
	const transformKey examples.TransformKey = "bench-transform"
	contract := pipz.GetContract[examples.TransformContext](transformKey)
	contract.Register(
		pipz.Apply(examples.ParseCSV),
		pipz.Apply(examples.ValidateEmail),
		pipz.Apply(examples.NormalizePhone),
		pipz.Apply(examples.EnrichData),
	)
	
	// Test data - typical CSV record
	ctx := examples.TransformContext{
		CSV: &examples.CSVRecord{
			Fields: []string{
				"123",
				"john doe",
				"JOHN.DOE@EXAMPLE.COM",
				"555-123-4567",
			},
		},
	}
	
	b.ResetTimer()
	
	// Benchmark
	for i := 0; i < b.N; i++ {
		_, err := contract.Process(ctx)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkTransformPipelineInvalidData tests performance with validation errors
func BenchmarkTransformPipelineInvalidData(b *testing.B) {
	// Setup
	const transformKey examples.TransformKey = "bench-transform-invalid"
	contract := pipz.GetContract[examples.TransformContext](transformKey)
	contract.Register(
		pipz.Apply(examples.ParseCSV),
		pipz.Apply(examples.ValidateEmail),
		pipz.Apply(examples.NormalizePhone),
		pipz.Apply(examples.EnrichData),
	)
	
	// Test data - invalid email and phone
	ctx := examples.TransformContext{
		CSV: &examples.CSVRecord{
			Fields: []string{
				"456",
				"invalid user",
				"not-an-email",
				"123",
			},
		},
	}
	
	b.ResetTimer()
	
	// Benchmark
	for i := 0; i < b.N; i++ {
		result, _ := contract.Process(ctx)
		// Expect errors to be captured in result.Errors
		if len(result.Errors) == 0 {
			b.Fatal("Expected validation errors")
		}
	}
}

// BenchmarkTransformPipelineLargeData tests performance with larger records
func BenchmarkTransformPipelineLargeData(b *testing.B) {
	// Setup
	const transformKey examples.TransformKey = "bench-transform-large"
	contract := pipz.GetContract[examples.TransformContext](transformKey)
	contract.Register(
		pipz.Apply(examples.ParseCSV),
		pipz.Apply(examples.ValidateEmail),
		pipz.Apply(examples.NormalizePhone),
		pipz.Apply(examples.EnrichData),
	)
	
	// Test data - with very long name
	ctx := examples.TransformContext{
		CSV: &examples.CSVRecord{
			Fields: []string{
				"789",
				"maria garcia rodriguez de la cruz santos martinez lopez fernandez",
				"maria.garcia.rodriguez.de.la.cruz.santos.martinez.lopez.fernandez@very-long-domain-name-example.com",
				"1-800-555-0123",
			},
		},
	}
	
	b.ResetTimer()
	
	// Benchmark
	for i := 0; i < b.N; i++ {
		_, err := contract.Process(ctx)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkTransformPipelineMinimal tests performance with just parsing
func BenchmarkTransformPipelineMinimal(b *testing.B) {
	// Setup - only CSV parsing
	const transformKey examples.TransformKey = "bench-transform-minimal"
	contract := pipz.GetContract[examples.TransformContext](transformKey)
	contract.Register(
		pipz.Apply(examples.ParseCSV),
	)
	
	// Test data
	ctx := examples.TransformContext{
		CSV: &examples.CSVRecord{
			Fields: []string{
				"999",
				"test user",
				"test@example.com",
				"555-0000",
			},
		},
	}
	
	b.ResetTimer()
	
	// Benchmark
	for i := 0; i < b.N; i++ {
		_, err := contract.Process(ctx)
		if err != nil {
			b.Fatal(err)
		}
	}
}