# ETL Pipeline Example

This example demonstrates building a flexible Extract, Transform, Load (ETL) pipeline with pipz.

## Overview

The ETL pipeline showcases:
- Multi-format data ingestion (CSV, JSON)
- Schema validation and type normalization
- Data enrichment and transformation
- Batch processing with progress tracking
- Error recovery and dead letter queues

## Architecture

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│   Extract   │────▶│  Transform  │────▶│    Load     │
│ (CSV/JSON)  │     │ & Validate  │     │ (DB/File)   │
└─────────────┘     └─────────────┘     └─────────────┘
        │                   │                    │
        └───────────────────┴────────────────────┘
                            │
                    ┌───────────────┐
                    │ Error Handler │
                    │   (DLQ/Log)   │
                    └───────────────┘
```

## Implementation

### Data Models

```go
// Schema defines the expected structure
type Schema struct {
    Name   string
    Fields []FieldDef
}

type FieldDef struct {
    Name     string
    Type     string // "string", "int", "float", "bool", "date"
    Required bool
    Pattern  string // Optional regex pattern
    Min      *float64
    Max      *float64
}

// Record represents a single data record
type Record struct {
    SourceFile string
    SourceLine int
    Data       map[string]interface{}
    Errors     []string
    ProcessedAt time.Time
}
```

### Extract Phase

Support multiple input formats:

```go
// CSV Parser
func ParseCSV(ctx context.Context, filename string) ([]Record, error) {
    file, err := os.Open(filename)
    if err != nil {
        return nil, fmt.Errorf("failed to open CSV: %w", err)
    }
    defer file.Close()

    reader := csv.NewReader(file)
    headers, err := reader.Read()
    if err != nil {
        return nil, fmt.Errorf("failed to read headers: %w", err)
    }

    var records []Record
    line := 1
    
    for {
        row, err := reader.Read()
        if err == io.EOF {
            break
        }
        if err != nil {
            log.Printf("Skipping line %d: %v", line, err)
            continue
        }

        data := make(map[string]interface{})
        for i, header := range headers {
            if i < len(row) {
                data[header] = row[i]
            }
        }

        records = append(records, Record{
            SourceFile: filename,
            SourceLine: line,
            Data:       data,
        })
        
        line++
    }

    return records, nil
}

// JSON Parser
func ParseJSON(ctx context.Context, filename string) ([]Record, error) {
    data, err := os.ReadFile(filename)
    if err != nil {
        return nil, fmt.Errorf("failed to read JSON: %w", err)
    }

    var rawRecords []map[string]interface{}
    if err := json.Unmarshal(data, &rawRecords); err != nil {
        return nil, fmt.Errorf("invalid JSON: %w", err)
    }

    records := make([]Record, len(rawRecords))
    for i, raw := range rawRecords {
        records[i] = Record{
            SourceFile: filename,
            SourceLine: i + 1,
            Data:       raw,
        }
    }

    return records, nil
}
```

### Transform Phase

Schema validation and type conversion:

```go
// Validate against schema
func ValidateSchema(schema Schema) func(context.Context, Record) error {
    requiredFields := make(map[string]FieldDef)
    for _, field := range schema.Fields {
        if field.Required {
            requiredFields[field.Name] = field
        }
    }

    return func(ctx context.Context, record Record) error {
        // Check required fields
        for name, field := range requiredFields {
            if _, exists := record.Data[name]; !exists {
                return fmt.Errorf("missing required field: %s", name)
            }
        }

        // Validate field constraints
        for _, field := range schema.Fields {
            value, exists := record.Data[field.Name]
            if !exists {
                continue
            }

            if err := validateField(field, value); err != nil {
                return fmt.Errorf("field %s: %w", field.Name, err)
            }
        }

        return nil
    }
}

// Type normalization
func NormalizeTypes(schema Schema) func(context.Context, Record) Record {
    return func(ctx context.Context, record Record) Record {
        for _, field := range schema.Fields {
            value, exists := record.Data[field.Name]
            if !exists {
                continue
            }

            // Convert string values to proper types
            if str, ok := value.(string); ok {
                switch field.Type {
                case "int":
                    if num, err := strconv.ParseInt(str, 10, 64); err == nil {
                        record.Data[field.Name] = num
                    }
                case "float":
                    if num, err := strconv.ParseFloat(str, 64); err == nil {
                        record.Data[field.Name] = num
                    }
                case "bool":
                    record.Data[field.Name] = strings.ToLower(str) == "true"
                case "date":
                    if t, err := time.Parse("2006-01-02", str); err == nil {
                        record.Data[field.Name] = t
                    }
                }
            }
        }

        return record
    }
}

// Data enrichment
func EnrichRecord(ctx context.Context, record Record) Record {
    record.ProcessedAt = time.Now()

    // Example enrichments
    if email, ok := record.Data["email"].(string); ok {
        // Extract domain
        parts := strings.Split(email, "@")
        if len(parts) == 2 {
            record.Data["email_domain"] = parts[1]
        }
    }

    // Add derived fields
    if age, ok := record.Data["age"].(int64); ok {
        record.Data["age_group"] = categorizeAge(age)
    }

    return record
}
```

### Load Phase

Flexible output destinations:

```go
// Database loader
func LoadToDatabase(db Database) func(context.Context, []Record) error {
    return func(ctx context.Context, records []Record) error {
        tx, err := db.BeginTx(ctx)
        if err != nil {
            return fmt.Errorf("failed to start transaction: %w", err)
        }
        defer tx.Rollback()

        stmt, err := tx.Prepare(`
            INSERT INTO processed_records (data, source_file, processed_at)
            VALUES ($1, $2, $3)
        `)
        if err != nil {
            return fmt.Errorf("failed to prepare statement: %w", err)
        }
        defer stmt.Close()

        for _, record := range records {
            jsonData, err := json.Marshal(record.Data)
            if err != nil {
                log.Printf("Failed to marshal record: %v", err)
                continue
            }

            _, err = stmt.ExecContext(ctx, jsonData, record.SourceFile, record.ProcessedAt)
            if err != nil {
                log.Printf("Failed to insert record: %v", err)
                continue
            }
        }

        return tx.Commit()
    }
}

// File loader (for data warehouse staging)
func LoadToParquet(filename string) func(context.Context, []Record) error {
    return func(ctx context.Context, records []Record) error {
        // Convert records to columnar format
        // Write to Parquet file
        // Implementation depends on parquet library
        return nil
    }
}
```

### Building the Pipeline

```go
func CreateETLPipeline(schema Schema, destination string) pipz.Chainable[string] {
    return pipz.Sequential(
        // Route based on file type
        pipz.Switch(
            func(ctx context.Context, filename string) string {
                switch {
                case strings.HasSuffix(filename, ".csv"):
                    return "csv"
                case strings.HasSuffix(filename, ".json"):
                    return "json"
                default:
                    return "unknown"
                }
            },
            map[string]pipz.Chainable[string]{
                "csv": createCSVPipeline(schema, destination),
                "json": createJSONPipeline(schema, destination),
                "unknown": pipz.Apply("reject", func(ctx context.Context, f string) (string, error) {
                    return "", fmt.Errorf("unsupported file type: %s", f)
                }),
            },
        ),
    )
}

func createCSVPipeline(schema Schema, destination string) pipz.Chainable[string] {
    return pipz.Apply("process_csv", func(ctx context.Context, filename string) (string, error) {
        // Extract
        records, err := ParseCSV(ctx, filename)
        if err != nil {
            return "", err
        }

        // Transform
        pipeline := pipz.Sequential(
            pipz.Effect("validate", ValidateSchema(schema)),
            pipz.Transform("normalize", NormalizeTypes(schema)),
            pipz.Transform("enrich", EnrichRecord),
        )

        var processed []Record
        var failed []Record

        for _, record := range records {
            result, err := pipeline.Process(ctx, record)
            if err != nil {
                record.Errors = append(record.Errors, err.Error())
                failed = append(failed, record)
                continue
            }
            processed = append(processed, result)
        }

        // Load successful records
        if len(processed) > 0 {
            loader := getLoader(destination)
            if err := loader(ctx, processed); err != nil {
                return "", fmt.Errorf("load failed: %w", err)
            }
        }

        // Handle failures
        if len(failed) > 0 {
            if err := writeFailedRecords(failed); err != nil {
                log.Printf("Failed to write error records: %v", err)
            }
        }

        return fmt.Sprintf("Processed %d/%d records", len(processed), len(records)), nil
    })
}
```

### Batch Processing

Handle large files efficiently:

```go
func CreateStreamingETLPipeline(schema Schema) pipz.Chainable[Record] {
    return pipz.Sequential(
        // Validate each record
        pipz.WithErrorHandler(
            pipz.Effect("validate", ValidateSchema(schema)),
            pipz.Effect("mark_invalid", func(ctx context.Context, err error) error {
                // Mark record as invalid but continue processing
                return nil
            }),
        ),
        
        // Transform valid records
        pipz.Transform("normalize", NormalizeTypes(schema)),
        pipz.Transform("enrich", EnrichRecord),
        
        // Batch for efficient loading
        pipz.ProcessorFunc[Record](func(ctx context.Context, record Record) (Record, error) {
            // Add to batch
            batch.Add(record)
            
            if batch.Size() >= 1000 {
                if err := flushBatch(ctx); err != nil {
                    return record, err
                }
            }
            
            return record, nil
        }),
    )
}
```

## Error Handling

Comprehensive error recovery:

```go
func CreateResilientETLPipeline(schema Schema) pipz.Chainable[string] {
    return pipz.Sequential(
        // Extract with retry for network sources
        pipz.RetryWithBackoff(
            extractData,
            3,
            time.Second,
        ),
        
        // Transform with error tracking
        pipz.ProcessorFunc[[]Record](func(ctx context.Context, records []Record) ([]Record, error) {
            var processed []Record
            errorCount := 0
            
            for _, record := range records {
                result, err := transformPipeline.Process(ctx, record)
                if err != nil {
                    errorCount++
                    logFailedRecord(record, err)
                    
                    // Continue processing other records
                    continue
                }
                processed = append(processed, result)
            }
            
            // Fail if error rate too high
            errorRate := float64(errorCount) / float64(len(records))
            if errorRate > 0.1 { // 10% threshold
                return nil, fmt.Errorf("error rate too high: %.2f%%", errorRate*100)
            }
            
            return processed, nil
        }),
        
        // Load with transaction support
        loadWithTransaction,
    )
}
```

## Monitoring

Track pipeline performance:

```go
type ETLMetrics struct {
    RecordsProcessed int64
    RecordsFailed    int64
    BytesProcessed   int64
    ProcessingTime   time.Duration
    
    mu sync.Mutex
}

func (m *ETLMetrics) WrapPipeline(pipeline pipz.Chainable[string]) pipz.Chainable[string] {
    return pipz.ProcessorFunc[string](func(ctx context.Context, filename string) (string, error) {
        start := time.Now()
        
        result, err := pipeline.Process(ctx, filename)
        
        m.mu.Lock()
        m.ProcessingTime += time.Since(start)
        if err != nil {
            m.RecordsFailed++
        }
        m.mu.Unlock()
        
        return result, err
    })
}
```

## Key Patterns

1. **Format Routing**: Use Switch to handle different file formats
2. **Schema Validation**: Validate early to catch bad data
3. **Type Safety**: Normalize types after parsing
4. **Batch Processing**: Process in chunks for efficiency
5. **Error Isolation**: Don't let bad records stop the pipeline
6. **Progress Tracking**: Monitor processing for large files
7. **Transaction Support**: Ensure data consistency

## Running the Example

```bash
cd examples/etl
go test -v
go run . input.csv output.db
```

## Next Steps

- [Event Processing](./event-processing.md) - Stream processing patterns
- [Error Recovery](../guides/error-recovery.md) - Advanced error handling
- [Performance](../guides/performance.md) - Optimizing ETL pipelines