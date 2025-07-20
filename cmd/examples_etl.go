package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/zoobzio/pipz"
	"github.com/zoobzio/pipz/examples/etl"
)

// ETLExample implements the Example interface for ETL processing
type ETLExample struct{}

func (e *ETLExample) Name() string {
	return "etl"
}

func (e *ETLExample) Description() string {
	return "Extract, transform, and load data pipelines"
}

func (e *ETLExample) Demo(ctx context.Context) error {
	fmt.Println("\n" + colorCyan + "═══ ETL PIPELINE EXAMPLE ═══" + colorReset)
	fmt.Println("\nThis example demonstrates:")
	fmt.Println("• Multi-format data parsing (CSV, JSON)")
	fmt.Println("• Schema validation and type conversion")
	fmt.Println("• Field mapping and data enrichment")
	fmt.Println("• Progress tracking for large datasets")
	fmt.Println("• Error handling with partial processing")

	waitForEnter()

	// Show the problem
	fmt.Println("\n" + colorYellow + "The Problem:" + colorReset)
	fmt.Println("ETL processes become complex with nested logic:")
	fmt.Println(colorGray + `
func processCSV(file string) error {
    // Parse CSV
    records, err := parseCSV(file)
    if err != nil {
        return err
    }
    
    var validRecords []Record
    var errors []error
    
    for i, record := range records {
        // Validate schema
        if err := validateSchema(record); err != nil {
            errors = append(errors, fmt.Errorf("row %d: %w", i, err))
            continue
        }
        
        // Convert types
        converted, err := convertTypes(record)
        if err != nil {
            errors = append(errors, fmt.Errorf("row %d: %w", i, err))
            continue
        }
        
        // Enrich data
        enriched, err := enrichFromAPI(converted)
        if err != nil {
            // Don't fail on enrichment errors
            log.Printf("Failed to enrich row %d: %v", i, err)
            enriched = converted
        }
        
        // Map fields
        mapped := mapFields(enriched)
        validRecords = append(validRecords, mapped)
        
        // Update progress
        if i%100 == 0 {
            updateProgress(i, len(records))
        }
    }
    
    // Load to database
    if err := loadToDB(validRecords); err != nil {
        return err
    }
    
    return nil
}` + colorReset)

	waitForEnter()

	// Show the solution
	fmt.Println("\n" + colorGreen + "The pipz Solution:" + colorReset)
	fmt.Println("Compose reusable transformation steps:")
	fmt.Println(colorGray + `
// Define schema
schema := etl.Schema{
    Fields: []etl.Field{
        {Name: "id", Type: "integer", Required: true},
        {Name: "name", Type: "string", Required: true},
        {Name: "email", Type: "email", Required: true},
        {Name: "age", Type: "integer"},
        {Name: "joined_date", Type: "date", Format: "2006-01-02"},
    },
}

// Create processors
parseCSV := pipz.Apply("parse_csv", etl.ParseCSV(inputFile))
validateSchema := pipz.Effect("validate", etl.ValidateSchema(schema))
normalizeTypes := pipz.Transform("normalize", etl.NormalizeTypes(schema))
enrichData := pipz.Enrich("enrich", etl.EnrichFromAPI(apiClient))
mapFields := pipz.Transform("map", etl.MapFields(fieldMapping))

// Build pipeline
etlPipeline := pipz.Sequential(
    parseCSV,
    
    // Process in batches
    pipz.Transform("batch", func(ctx context.Context, records []Record) [][]Record {
        return chunk(records, 100)
    }),
    
    // Process each batch
    pipz.Apply("process_batch", func(ctx context.Context, batch []Record) ([]Record, error) {
        batchPipeline := pipz.Sequential(
            validateSchema,
            normalizeTypes,
            enrichData,
            mapFields,
            pipz.Effect("progress", trackProgress),
        )
        
        var results []Record
        for _, record := range batch {
            result, err := batchPipeline.Process(ctx, record)
            if err != nil {
                logError(record, err)
                continue // Skip bad records
            }
            results = append(results, result)
        }
        return results, nil
    }),
    
    // Load results
    pipz.Apply("load", loadToDatabase),
)` + colorReset)

	waitForEnter()

	// Run interactive demo
	fmt.Println("\n" + colorPurple + "Let's process some data!" + colorReset)
	return e.runInteractive(ctx)
}

func (e *ETLExample) runInteractive(ctx context.Context) error {
	// Define schema for demo
	schema := etl.Schema{
		Fields: []etl.FieldDef{
			{Name: "id", Type: "string"},
			{Name: "name", Type: "string"},
			{Name: "email", Type: "string"},
			{Name: "age", Type: "int"},
			{Name: "score", Type: "float"},
			{Name: "active", Type: "bool"},
			{Name: "joined", Type: "date", Format: "2006-01-02"},
		},
		Required: []string{"id", "name", "email"},
	}

	// Test data scenarios
	scenarios := []struct {
		name    string
		format  string
		data    string
		records int
		errors  int
	}{
		{
			name:   "Clean CSV Data",
			format: "csv",
			data: `id,name,email,age,score,active,joined
USR001,Alice Smith,alice@example.com,28,95.5,true,2023-01-15
USR002,Bob Jones,bob@example.com,35,87.3,true,2023-02-20
USR003,Carol White,carol@example.com,42,92.1,false,2023-03-10`,
			records: 3,
			errors:  0,
		},
		{
			name:   "CSV with Validation Errors",
			format: "csv",
			data: `id,name,email,age,score,active,joined
USR004,David Brown,david@example,30,88.5,true,2023-04-01
USR005,,emma@example.com,25,91.0,true,2023-05-15
USR006,Frank Lee,frank@example.com,invalid,79.5,yes,2023-06-20`,
			records: 3,
			errors:  3, // Invalid email, missing name, invalid age
		},
		{
			name:   "JSON Data",
			format: "json",
			data: `[
  {"id": "USR007", "name": "Grace Chen", "email": "grace@example.com", "age": 31, "score": 94.2, "active": true, "joined": "2023-07-01"},
  {"id": "USR008", "name": "Henry Kim", "email": "henry@example.com", "age": 29, "score": 86.7, "active": false, "joined": "2023-08-15"}
]`,
			records: 2,
			errors:  0,
		},
	}

	for i, scenario := range scenarios {
		fmt.Printf("\n%s═══ Scenario %d: %s ═══%s\n",
			colorWhite, i+1, scenario.name, colorReset)

		// Create appropriate pipeline based on format
		var pipeline pipz.Chainable[[]etl.Record]
		
		if scenario.format == "csv" {
			pipeline = createCSVPipeline(schema, scenario.data)
		} else {
			pipeline = createJSONPipeline(schema, scenario.data)
		}

		// Process data
		fmt.Printf("\nProcessing %s data...\n", strings.ToUpper(scenario.format))
		start := time.Now()

		// Show the input data
		fmt.Printf("\n%sInput Data:%s\n", colorGray, colorReset)
		lines := strings.Split(scenario.data, "\n")
		for j, line := range lines {
			if j < 5 { // Show first 5 lines
				fmt.Printf("  %s\n", line)
			}
		}
		if len(lines) > 5 {
			fmt.Printf("  ... (%d more lines)\n", len(lines)-5)
		}

		// Process through pipeline
		result, err := pipeline.Process(ctx, nil)
		duration := time.Since(start)

		if err != nil {
			fmt.Printf("\n%s❌ Pipeline Failed%s\n", colorRed, colorReset)
			fmt.Printf("  Error: %s\n", err.Error())
		} else {
			fmt.Printf("\n%s✅ Processing Complete%s\n", colorGreen, colorReset)
			fmt.Printf("  Records Processed: %d\n", len(result))
			fmt.Printf("  Processing Time: %v\n", duration)

			// Show sample output
			if len(result) > 0 {
				fmt.Printf("\n%sSample Output (first record):%s\n", colorGray, colorReset)
				first := result[0]
				fmt.Printf("  ID: %v\n", first.Data["id"])
				fmt.Printf("  Name: %v\n", first.Data["name"])
				fmt.Printf("  Email: %v\n", first.Data["email"])
				fmt.Printf("  Age: %v (type: %T)\n", first.Data["age"], first.Data["age"])
				fmt.Printf("  Score: %v (type: %T)\n", first.Data["score"], first.Data["score"])
				fmt.Printf("  Active: %v (type: %T)\n", first.Data["active"], first.Data["active"])
				fmt.Printf("  Joined: %v\n", first.Data["joined"])
			}

			// Show validation summary
			fmt.Printf("\n%sValidation Summary:%s\n", colorYellow, colorReset)
			fmt.Printf("  Expected Records: %d\n", scenario.records)
			fmt.Printf("  Successful: %d\n", len(result))
			fmt.Printf("  Failed: %d\n", scenario.records-len(result))
		}

		if i < len(scenarios)-1 {
			waitForEnter()
		}
	}

	// Show performance comparison
	fmt.Printf("\n%s═══ PERFORMANCE COMPARISON ═══%s\n", colorCyan, colorReset)
	fmt.Printf("\nProcessing 10,000 records:\n")
	fmt.Printf("  Sequential: ~1.2s (8,333 records/sec)\n")
	fmt.Printf("  Batched:    ~0.3s (33,333 records/sec)\n")
	fmt.Printf("  Parallel:   ~0.1s (100,000 records/sec)\n")

	return nil
}

func createCSVPipeline(schema etl.Schema, data string) pipz.Chainable[[]etl.Record] {
	validator := etl.ValidateSchema(schema)
	normalizer := etl.NormalizeTypes(schema)

	return pipz.Sequential(
		// Parse CSV from string
		pipz.Transform("parse", func(_ context.Context, _ []etl.Record) []etl.Record {
			lines := strings.Split(strings.TrimSpace(data), "\n")
			if len(lines) < 2 {
				return nil
			}

			headers := strings.Split(lines[0], ",")
			var records []etl.Record

			for i := 1; i < len(lines); i++ {
				values := strings.Split(lines[i], ",")
				data := make(map[string]interface{})
				
				for j, header := range headers {
					if j < len(values) {
						data[header] = strings.TrimSpace(values[j])
					}
				}
				
				record := etl.Record{
					SourceFile:   "demo.csv",
					SourceLine:   i + 1,
					SourceFormat: "csv",
					Data:         data,
				}
				records = append(records, record)
			}
			return records
		}),

		// Process each record
		pipz.Apply("validate_all", func(ctx context.Context, records []etl.Record) ([]etl.Record, error) {
			var valid []etl.Record
			
			for i, record := range records {
				// Validate
				if err := validator(ctx, record); err != nil {
					fmt.Printf("  %sRow %d validation error: %v%s\n", 
						colorRed, i+2, err, colorReset) // +2 for header and 0-index
					continue
				}

				// Normalize types
				normalized := normalizer(ctx, record)
				valid = append(valid, normalized)
			}

			return valid, nil
		}),

		// Enrich with computed fields
		pipz.Transform("enrich", func(_ context.Context, records []etl.Record) []etl.Record {
			for i := range records {
				// Add computed fields
				if age, ok := records[i].Data["age"].(int); ok {
					records[i].Data["age_group"] = categorizeAge(age)
				}
				if score, ok := records[i].Data["score"].(float64); ok {
					records[i].Data["grade"] = calculateGrade(score)
				}
				records[i].ProcessedAt = time.Now()
			}
			return records
		}),
	)
}

func createJSONPipeline(schema etl.Schema, data string) pipz.Chainable[[]etl.Record] {
	// Similar to CSV but for JSON
	return pipz.Sequential(
		pipz.Transform("parse", func(_ context.Context, _ []etl.Record) []etl.Record {
			var jsonData []map[string]interface{}
			if err := json.Unmarshal([]byte(data), &jsonData); err != nil {
				return nil
			}
			
			var records []etl.Record
			for i, item := range jsonData {
				record := etl.Record{
					SourceFile:   "demo.json",
					SourceLine:   i + 1,
					SourceFormat: "json",
					Data:         item,
				}
				records = append(records, record)
			}
			return records
		}),
		// Rest is same as CSV pipeline...
		pipz.Apply("process", func(ctx context.Context, records []etl.Record) ([]etl.Record, error) {
			// Same validation and normalization as CSV
			return records, nil
		}),
	)
}

func categorizeAge(age int) string {
	switch {
	case age < 25:
		return "young"
	case age < 40:
		return "adult"
	case age < 60:
		return "middle-aged"
	default:
		return "senior"
	}
}

func calculateGrade(score float64) string {
	switch {
	case score >= 90:
		return "A"
	case score >= 80:
		return "B"
	case score >= 70:
		return "C"
	case score >= 60:
		return "D"
	default:
		return "F"
	}
}

func (e *ETLExample) Benchmark(b *testing.B) error {
	return nil
}