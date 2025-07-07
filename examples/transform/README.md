# Data Transformation Example

This example demonstrates using pipz for ETL (Extract, Transform, Load) pipelines that convert CSV data into structured database records.

## Overview

The transformation pipeline shows how to:
- Parse raw CSV data into structured records
- Validate and normalize data formats
- Handle transformation errors gracefully
- Create both strict and lenient pipelines
- Enrich data with formatting and metadata

## Structure

```go
type TransformContext struct {
    CSV    *CSVRecord      // Source data
    DB     *DatabaseRecord // Transformed data
    Errors []string        // Transformation warnings/errors
}

type DatabaseRecord struct {
    ID        int
    Name      string
    Email     string
    Phone     string
    CreatedAt time.Time
}
```

## Transformation Steps

1. **ParseCSV**: Converts CSV fields to database record structure
2. **ValidateEmail**: Validates and normalizes email addresses
3. **NormalizePhone**: Formats phone numbers to +1-XXX-XXX-XXXX
4. **EnrichData**: Title cases names and adds metadata
5. **ValidateTransform**: Ensures transformation succeeded (strict mode)

## Usage

```go
// Create transformation pipeline
transformer := CreateTransformPipeline()

// Transform CSV data
ctx := TransformContext{
    CSV: &CSVRecord{
        Fields: []string{"123", "john doe", "john@example.com", "(555) 123-4567"},
    },
}

result, err := transformer.Process(ctx)
if err != nil {
    // Transformation failed - result is zero value
    log.Printf("Transformation failed: %v", err)
}
```

## Pipeline Modes

### Strict Pipeline
Fails on any validation errors:
```go
pipeline := CreateTransformPipeline()
```

### Lenient Pipeline
Continues processing, recording errors:
```go
pipeline := CreateLenientTransformPipeline()
```

## Running

```bash
# Run the example
go run transform.go

# Run tests
go test
```

## Key Features

- **Multi-Stage Processing**: Each stage handles specific transformation
- **Error Collection**: Tracks warnings without failing (lenient mode)
- **Data Normalization**: Consistent formatting for emails and phones
- **Data Enrichment**: Title casing and metadata addition
- **Batch Support**: Process multiple records efficiently

## Example Output

```
Test 1: Valid CSV transformation
✓ Record transformed successfully:
  ID: 123
  Name: John Doe
  Email: john.doe@example.com
  Phone: +1-555-123-4567
  Errors: []

Test 2: Invalid email (lenient pipeline)
✓ Record transformed with warnings:
  Name: Jane Smith
  Email: janesmith.com (normalized)
  Phone: +1-555-987-6543
  Warnings: [invalid email format: missing @]

Test 3: International phone number
✓ Record transformed:
  Name: Bob Johnson
  Email: bob@example.com
  Phone: +44 20 7946 0958 (original)
  Warnings: [invalid phone number format: +44 20 7946 0958]
```

## Use Cases

- CSV file imports
- Data migration pipelines
- API data transformation
- Batch processing systems
- Data quality monitoring