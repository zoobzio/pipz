package etl

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func BenchmarkParseCSV(b *testing.B) {
	// Create test CSV with various sizes
	sizes := []int{100, 1000, 10000}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("Records_%d", size), func(b *testing.B) {
			// Generate CSV content
			csvContent := "id,email,name,age,created_at,is_active\n"
			for i := 0; i < size; i++ {
				csvContent += fmt.Sprintf("%d,user%d@example.com,User %d,%d,2024-01-01,true\n",
					i, i, i, i%100)
			}

			tmpFile := filepath.Join(b.TempDir(), "bench.csv")
			if err := os.WriteFile(tmpFile, []byte(csvContent), 0644); err != nil {
				b.Fatal(err)
			}

			ctx := context.Background()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				_, err := ParseCSV(ctx, tmpFile)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkValidateSchema(b *testing.B) {
	validator := ValidateSchema(CustomerSchema)
	ctx := context.Background()

	// Different record scenarios
	records := []struct {
		name   string
		record Record
	}{
		{
			name: "Valid",
			record: Record{
				Data: map[string]interface{}{
					"id":         "1",
					"email":      "test@example.com",
					"name":       "Test User",
					"age":        "30",
					"created_at": "2024-01-01",
					"is_active":  "true",
				},
			},
		},
		{
			name: "Invalid",
			record: Record{
				Data: map[string]interface{}{
					"id":    "1",
					"email": "invalid-email",
					"age":   "200", // Out of range
				},
			},
		},
		{
			name: "MissingFields",
			record: Record{
				Data: map[string]interface{}{
					"id": "1",
				},
			},
		},
	}

	for _, r := range records {
		b.Run(r.name, func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				validator(ctx, r.record)
			}
		})
	}
}

func BenchmarkNormalizeTypes(b *testing.B) {
	normalizer := NormalizeTypes(CustomerSchema)
	ctx := context.Background()

	record := Record{
		Data: map[string]interface{}{
			"id":         "1",
			"email":      "test@example.com",
			"name":       "Test User",
			"age":        "30",   // String to int
			"is_active":  "true", // String to bool
			"created_at": "2024-01-01",
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		normalizer(ctx, record)
	}
}

func BenchmarkEnrichRecord(b *testing.B) {
	ctx := context.Background()

	records := []Record{
		{
			Data: map[string]interface{}{
				"email": "user@example.com",
				"name":  "Test User",
			},
		},
		{
			Data: map[string]interface{}{
				"email": "user@subdomain.example.com",
				"name":  "Test User",
			},
			Errors: []string{"some error"},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		EnrichRecord(ctx, records[i%len(records)])
	}
}

func BenchmarkFullPipeline(b *testing.B) {
	sizes := []int{10, 100, 1000}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("Records_%d", size), func(b *testing.B) {
			// Generate test data
			csvContent := "id,email,name,age,created_at,is_active\n"
			for i := 0; i < size; i++ {
				csvContent += fmt.Sprintf("%d,user%d@example.com,User %d,%d,2024-01-01,true\n",
					i, i, i, i%100)
			}

			tmpDir := b.TempDir()
			csvFile := filepath.Join(tmpDir, "input.csv")
			outputFile := filepath.Join(tmpDir, "output.json")

			if err := os.WriteFile(csvFile, []byte(csvContent), 0644); err != nil {
				b.Fatal(err)
			}

			ctx := context.Background()
			pipeline := CreateCSVToJSONPipeline(CustomerSchema, outputFile)

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				ResetStats() // Reset stats for each run

				records, err := ParseCSV(ctx, csvFile)
				if err != nil {
					b.Fatal(err)
				}

				_, err = pipeline.Process(ctx, records)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkMapFields(b *testing.B) {
	mapper := MapFields(map[string]string{
		"customer_email":   "email",
		"customer_name":    "name",
		"customer_id":      "id",
		"order_total":      "total",
		"order_date":       "date",
		"shipping_address": "address",
		"billing_address":  "bill_to",
		"phone_number":     "phone",
	})

	ctx := context.Background()

	record := Record{
		Data: map[string]interface{}{
			"customer_email":   "test@example.com",
			"customer_name":    "Test User",
			"customer_id":      "12345",
			"order_total":      99.99,
			"order_date":       "2024-01-01",
			"shipping_address": "123 Main St",
			"billing_address":  "456 Oak Ave",
			"phone_number":     "555-1234",
			"extra_field_1":    "value1",
			"extra_field_2":    "value2",
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mapper(ctx, record)
	}
}

func BenchmarkWriteJSON(b *testing.B) {
	sizes := []int{10, 100, 1000}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("Records_%d", size), func(b *testing.B) {
			// Generate records
			records := make([]Record, size)
			for i := 0; i < size; i++ {
				records[i] = Record{
					Data: map[string]interface{}{
						"id":         fmt.Sprintf("%d", i),
						"email":      fmt.Sprintf("user%d@example.com", i),
						"name":       fmt.Sprintf("User %d", i),
						"age":        i % 100,
						"created_at": "2024-01-01",
						"is_active":  i%2 == 0,
					},
				}
			}

			tmpFile := filepath.Join(b.TempDir(), "output.json")
			writer := WriteJSON(tmpFile)
			ctx := context.Background()

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				ResetStats()
				if err := writer(ctx, records); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkWriteCSV(b *testing.B) {
	columns := []string{"id", "email", "name", "age", "created_at", "is_active"}
	sizes := []int{10, 100, 1000}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("Records_%d", size), func(b *testing.B) {
			// Generate records
			records := make([]Record, size)
			for i := 0; i < size; i++ {
				records[i] = Record{
					Data: map[string]interface{}{
						"id":         fmt.Sprintf("%d", i),
						"email":      fmt.Sprintf("user%d@example.com", i),
						"name":       fmt.Sprintf("User %d", i),
						"age":        i % 100,
						"created_at": "2024-01-01",
						"is_active":  i%2 == 0,
					},
				}
			}

			tmpFile := filepath.Join(b.TempDir(), "output.csv")
			writer := WriteCSV(tmpFile, columns)
			ctx := context.Background()

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				ResetStats()
				if err := writer(ctx, records); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkConcurrentValidation(b *testing.B) {
	validator := ValidateSchema(CustomerSchema)
	ctx := context.Background()

	record := Record{
		Data: map[string]interface{}{
			"id":         "1",
			"email":      "test@example.com",
			"name":       "Test User",
			"age":        "30",
			"created_at": "2024-01-01",
		},
	}

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			validator(ctx, record)
		}
	})
}

var globalRecord Record // Prevent compiler optimizations

func BenchmarkMemoryUsage(b *testing.B) {
	pipeline := CreateStreamingETLPipeline(CustomerSchema)
	ctx := context.Background()

	b.Run("SingleRecord", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			record := Record{
				Data: map[string]interface{}{
					"id":            fmt.Sprintf("%d", i),
					"email":         "test@example.com",
					"customer_name": "Test User",
					"age":           "30",
					"created_at":    "2024-01-01",
				},
			}

			result, err := pipeline.Process(ctx, record)
			if err != nil {
				b.Fatal(err)
			}
			globalRecord = result // Prevent optimization
		}
	})
}
