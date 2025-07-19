// Package etl demonstrates using pipz for Extract-Transform-Load pipelines
// with error recovery, schema validation, and progress tracking.
package etl

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/zoobzio/pipz"
)

// Record represents a generic data record that can be CSV or JSON
type Record struct {
	// Source information
	SourceFile   string
	SourceLine   int
	SourceFormat string // "csv", "json"

	// Parsed data
	Data map[string]interface{}

	// Processing metadata
	ProcessedAt time.Time
	Errors      []string
	Warnings    []string
}

// Schema defines the expected structure and validation rules
type Schema struct {
	Name     string
	Version  string
	Fields   []FieldDef
	Required []string
}

// FieldDef defines a field's type and validation rules
type FieldDef struct {
	Name         string
	Type         string // "string", "int", "float", "date", "bool"
	Format       string // e.g., "YYYY-MM-DD" for dates
	Min          *float64
	Max          *float64
	Pattern      string // regex pattern
	DefaultValue interface{}
}

// ETLStats tracks processing statistics
type ETLStats struct {
	TotalRecords   int64
	ValidRecords   int64
	InvalidRecords int64
	Transformed    int64
	Written        int64
	StartTime      time.Time
	mu             sync.RWMutex
}

// Progress reports ETL progress
type Progress struct {
	Processed   int64
	Total       int64
	Rate        float64 // records per second
	ETASeconds  int
	CurrentFile string
}

// Global stats for demo (in production, pass this through context)
var globalStats = &ETLStats{StartTime: time.Now()}

// CSV Processors

// ParseCSV reads CSV records from a file
func ParseCSV(_ context.Context, filename string) ([]Record, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open CSV file: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)

	// Read header
	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV header: %w", err)
	}

	var records []Record
	lineNum := 2 // Start after header

	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			// Record error but continue processing
			records = append(records, Record{
				SourceFile:   filename,
				SourceLine:   lineNum,
				SourceFormat: "csv",
				Errors:       []string{err.Error()},
			})
			lineNum++
			continue
		}

		// Convert to map
		data := make(map[string]interface{})
		for i, value := range row {
			if i < len(header) {
				data[header[i]] = value
			}
		}

		records = append(records, Record{
			SourceFile:   filename,
			SourceLine:   lineNum,
			SourceFormat: "csv",
			Data:         data,
		})

		lineNum++
		atomic.AddInt64(&globalStats.TotalRecords, 1)
	}

	return records, nil
}

// ParseJSON reads JSON records from a file (newline-delimited JSON)
func ParseJSON(_ context.Context, filename string) ([]Record, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open JSON file: %w", err)
	}
	defer file.Close()

	var records []Record
	decoder := json.NewDecoder(file)
	lineNum := 1

	for {
		var data map[string]interface{}
		err := decoder.Decode(&data)
		if err == io.EOF {
			break
		}
		if err != nil {
			// Record error but continue
			records = append(records, Record{
				SourceFile:   filename,
				SourceLine:   lineNum,
				SourceFormat: "json",
				Errors:       []string{err.Error()},
			})
			lineNum++
			continue
		}

		records = append(records, Record{
			SourceFile:   filename,
			SourceLine:   lineNum,
			SourceFormat: "json",
			Data:         data,
		})

		lineNum++
		atomic.AddInt64(&globalStats.TotalRecords, 1)
	}

	return records, nil
}

// Schema Validation

// ValidateSchema checks if a record conforms to the schema
func ValidateSchema(schema Schema) func(context.Context, Record) error {
	return func(_ context.Context, record Record) error {
		if record.Data == nil {
			return errors.New("record has no data")
		}

		// Check required fields
		for _, required := range schema.Required {
			if _, exists := record.Data[required]; !exists {
				return fmt.Errorf("missing required field: %s", required)
			}
		}

		// Validate each field
		for _, field := range schema.Fields {
			value, exists := record.Data[field.Name]
			if !exists {
				if field.DefaultValue != nil {
					record.Data[field.Name] = field.DefaultValue
				}
				continue
			}

			if err := validateField(field, value); err != nil {
				return fmt.Errorf("field %s: %w", field.Name, err)
			}
		}

		atomic.AddInt64(&globalStats.ValidRecords, 1)
		return nil
	}
}

func validateField(field FieldDef, value interface{}) error {
	// Type validation
	switch field.Type {
	case "string":
		str, ok := value.(string)
		if !ok {
			return fmt.Errorf("expected string, got %T", value)
		}
		if field.Pattern != "" {
			// In production, compile regex once
			// For demo, simplified check
			if field.Pattern == "email" && !strings.Contains(str, "@") {
				return errors.New("invalid email format")
			}
		}

	case "int":
		var num float64
		switch v := value.(type) {
		case float64:
			num = v
		case string:
			parsed, err := strconv.ParseFloat(v, 64)
			if err != nil {
				return fmt.Errorf("cannot parse as int: %v", err)
			}
			num = parsed
		default:
			return fmt.Errorf("expected int, got %T", value)
		}

		if field.Min != nil && num < *field.Min {
			return fmt.Errorf("value %v below minimum %v", num, *field.Min)
		}
		if field.Max != nil && num > *field.Max {
			return fmt.Errorf("value %v above maximum %v", num, *field.Max)
		}

	case "float":
		var num float64
		switch v := value.(type) {
		case float64:
			num = v
		case string:
			parsed, err := strconv.ParseFloat(v, 64)
			if err != nil {
				return fmt.Errorf("cannot parse as float: %v", err)
			}
			num = parsed
		default:
			return fmt.Errorf("expected float, got %T", value)
		}

		if field.Min != nil && num < *field.Min {
			return fmt.Errorf("value %v below minimum %v", num, *field.Min)
		}
		if field.Max != nil && num > *field.Max {
			return fmt.Errorf("value %v above maximum %v", num, *field.Max)
		}

	case "bool":
		switch v := value.(type) {
		case bool:
			// Valid
		case string:
			if v != "true" && v != "false" {
				return fmt.Errorf("cannot parse %q as bool", v)
			}
		default:
			return fmt.Errorf("expected bool, got %T", value)
		}

	case "date":
		str, ok := value.(string)
		if !ok {
			return fmt.Errorf("expected date string, got %T", value)
		}
		// Simplified date validation
		if len(str) < 10 {
			return errors.New("invalid date format")
		}
	}

	return nil
}

// Transformers

// NormalizeTypes converts string values to their proper types based on schema
func NormalizeTypes(schema Schema) func(context.Context, Record) Record {
	fieldMap := make(map[string]FieldDef)
	for _, field := range schema.Fields {
		fieldMap[field.Name] = field
	}

	return func(_ context.Context, record Record) Record {
		if record.Data == nil {
			return record
		}

		for name, value := range record.Data {
			field, ok := fieldMap[name]
			if !ok {
				continue
			}

			// Convert string values to proper types
			if str, ok := value.(string); ok {
				switch field.Type {
				case "int":
					if num, err := strconv.ParseInt(str, 10, 64); err == nil {
						record.Data[name] = num
					}
				case "float":
					if num, err := strconv.ParseFloat(str, 64); err == nil {
						record.Data[name] = num
					}
				case "bool":
					record.Data[name] = str == "true"
				}
			}
		}

		return record
	}
}

// EnrichRecord adds computed fields and metadata
func EnrichRecord(_ context.Context, record Record) Record {
	if record.Data == nil {
		return record
	}

	// Add processing timestamp
	record.ProcessedAt = time.Now()

	// Example enrichments
	if email, ok := record.Data["email"].(string); ok {
		// Extract domain
		parts := strings.Split(email, "@")
		if len(parts) == 2 {
			record.Data["email_domain"] = parts[1]
		}
	}

	// Add data quality score
	qualityScore := 1.0
	if len(record.Errors) > 0 {
		qualityScore = 0.0
	} else if len(record.Warnings) > 0 {
		qualityScore = 0.5
	}
	record.Data["quality_score"] = qualityScore

	atomic.AddInt64(&globalStats.Transformed, 1)
	return record
}

// MapFields renames and restructures fields
func MapFields(mapping map[string]string) func(context.Context, Record) Record {
	return func(_ context.Context, record Record) Record {
		if record.Data == nil {
			return record
		}

		newData := make(map[string]interface{})
		for oldName, value := range record.Data {
			if newName, exists := mapping[oldName]; exists {
				newData[newName] = value
			} else {
				// Keep unmapped fields
				newData[oldName] = value
			}
		}

		record.Data = newData
		return record
	}
}

// Output Writers

// WriteJSON writes records to a JSON file
func WriteJSON(filename string) func(context.Context, []Record) error {
	return func(_ context.Context, records []Record) error {
		file, err := os.Create(filename)
		if err != nil {
			return fmt.Errorf("failed to create output file: %w", err)
		}
		defer file.Close()

		encoder := json.NewEncoder(file)
		encoder.SetIndent("", "  ")

		for _, record := range records {
			if err := encoder.Encode(record.Data); err != nil {
				return fmt.Errorf("failed to write record: %w", err)
			}
			atomic.AddInt64(&globalStats.Written, 1)
		}

		return nil
	}
}

// WriteCSV writes records to a CSV file
func WriteCSV(filename string, columns []string) func(context.Context, []Record) error {
	return func(_ context.Context, records []Record) error {
		file, err := os.Create(filename)
		if err != nil {
			return fmt.Errorf("failed to create output file: %w", err)
		}
		defer file.Close()

		writer := csv.NewWriter(file)
		defer writer.Flush()

		// Write header
		if err := writer.Write(columns); err != nil {
			return fmt.Errorf("failed to write header: %w", err)
		}

		// Write records
		for _, record := range records {
			if record.Data == nil {
				continue
			}

			row := make([]string, len(columns))
			for i, col := range columns {
				if value, exists := record.Data[col]; exists {
					row[i] = fmt.Sprintf("%v", value)
				}
			}

			if err := writer.Write(row); err != nil {
				return fmt.Errorf("failed to write row: %w", err)
			}
			atomic.AddInt64(&globalStats.Written, 1)
		}

		return nil
	}
}

// Error Handling

// ErrorHandler records failed records for later processing
func ErrorHandler(errorFile string) func(context.Context, Record, error) {
	errorLog := &sync.Map{}

	return func(_ context.Context, record Record, err error) {
		record.Errors = append(record.Errors, err.Error())
		errorLog.Store(fmt.Sprintf("%s:%d", record.SourceFile, record.SourceLine), record)
		atomic.AddInt64(&globalStats.InvalidRecords, 1)

		// Periodically write errors to file
		// In production, use a proper error sink
	}
}

// Progress Tracking

// TrackProgress reports ETL progress
func TrackProgress(_ context.Context, record Record) error {
	stats := GetStats()
	progress := Progress{
		Processed:   stats.TotalRecords,
		Rate:        float64(stats.TotalRecords) / time.Since(stats.StartTime).Seconds(),
		CurrentFile: record.SourceFile,
	}

	// In production, send to monitoring service
	if stats.TotalRecords%1000 == 0 {
		fmt.Printf("[PROGRESS] Processed: %d, Valid: %d, Rate: %.1f/sec\n",
			progress.Processed, stats.ValidRecords, progress.Rate)
	}

	return nil
}

// Pipeline Builders

// CreateCSVToJSONPipeline creates a pipeline for CSV to JSON conversion
func CreateCSVToJSONPipeline(schema Schema, outputFile string) pipz.Chainable[[]Record] {
	return pipz.Sequential(
		// Validate each record
		pipz.Apply("validate_records", func(ctx context.Context, records []Record) ([]Record, error) {
			validator := ValidateSchema(schema)
			normalizer := NormalizeTypes(schema)
			errorHandler := ErrorHandler("errors.json")

			validRecords := make([]Record, 0, len(records))
			for _, record := range records {
				// Skip records with parse errors
				if len(record.Errors) > 0 {
					errorHandler(ctx, record, errors.New(strings.Join(record.Errors, "; ")))
					continue
				}

				// Validate
				if err := validator(ctx, record); err != nil {
					errorHandler(ctx, record, err)
					continue
				}

				// Normalize and enrich
				record = normalizer(ctx, record)
				record = EnrichRecord(ctx, record)

				validRecords = append(validRecords, record)
			}

			return validRecords, nil
		}),

		// Write to JSON
		pipz.Validate("write_json", WriteJSON(outputFile)),

		// Track progress
		pipz.Effect("progress", func(ctx context.Context, records []Record) error {
			if len(records) > 0 {
				TrackProgress(ctx, records[0])
			}
			return nil
		}),
	)
}

// CreateMultiSourcePipeline handles multiple input sources
func CreateMultiSourcePipeline(schema Schema) pipz.Chainable[string] {
	return pipz.Sequential(
		// Route based on file type
		pipz.Switch(
			func(_ context.Context, filename string) string {
				if strings.HasSuffix(filename, ".csv") {
					return "csv"
				} else if strings.HasSuffix(filename, ".json") {
					return "json"
				}
				return "unknown"
			},
			map[string]pipz.Chainable[string]{
				"csv": pipz.Apply("process_csv", func(ctx context.Context, filename string) (string, error) {
					records, err := ParseCSV(ctx, filename)
					if err != nil {
						return "", err
					}

					pipeline := CreateCSVToJSONPipeline(schema, filename+".output.json")
					_, err = pipeline.Process(ctx, records)
					return filename + ".output.json", err
				}),
				"json": pipz.Apply("process_json", func(ctx context.Context, filename string) (string, error) {
					records, err := ParseJSON(ctx, filename)
					if err != nil {
						return "", err
					}

					// Apply transformations
					transformed := make([]Record, len(records))
					for i, record := range records {
						transformed[i] = EnrichRecord(ctx, record)
					}

					return filename + ".transformed.json", WriteJSON(filename+".transformed.json")(ctx, transformed)
				}),
				"default": pipz.Apply("unsupported", func(_ context.Context, filename string) (string, error) {
					return "", fmt.Errorf("unsupported file type: %s", filename)
				}),
			},
		),
	)
}

// CreateStreamingETLPipeline processes records one at a time for large files
func CreateStreamingETLPipeline(schema Schema) pipz.Chainable[Record] {
	validator := ValidateSchema(schema)
	normalizer := NormalizeTypes(schema)

	return pipz.Sequential(
		// Validate
		pipz.Validate("validate", validator),

		// Transform
		pipz.Transform("normalize", normalizer),
		pipz.Transform("enrich", EnrichRecord),

		// Map fields if needed
		pipz.Transform("map_fields", MapFields(map[string]string{
			"customer_email": "email",
			"customer_name":  "name",
			"order_total":    "total_amount",
		})),

		// Track progress
		pipz.Effect("track", TrackProgress),
	)
}

// Utility Functions

// GetStats returns current processing statistics
func GetStats() ETLStats {
	globalStats.mu.RLock()
	defer globalStats.mu.RUnlock()

	return ETLStats{
		TotalRecords:   atomic.LoadInt64(&globalStats.TotalRecords),
		ValidRecords:   atomic.LoadInt64(&globalStats.ValidRecords),
		InvalidRecords: atomic.LoadInt64(&globalStats.InvalidRecords),
		Transformed:    atomic.LoadInt64(&globalStats.Transformed),
		Written:        atomic.LoadInt64(&globalStats.Written),
		StartTime:      globalStats.StartTime,
	}
}

// ResetStats resets global statistics
func ResetStats() {
	globalStats = &ETLStats{StartTime: time.Now()}
}

// Sample Schemas

// CustomerSchema defines a customer record schema
var CustomerSchema = Schema{
	Name:    "Customer",
	Version: "1.0",
	Fields: []FieldDef{
		{Name: "id", Type: "string"},
		{Name: "email", Type: "string", Pattern: "email"},
		{Name: "name", Type: "string"},
		{Name: "age", Type: "int", Min: float64Ptr(0), Max: float64Ptr(150)},
		{Name: "created_at", Type: "date", Format: "YYYY-MM-DD"},
		{Name: "is_active", Type: "bool", DefaultValue: true},
	},
	Required: []string{"id", "email"},
}

// OrderSchema defines an order record schema
var OrderSchema = Schema{
	Name:    "Order",
	Version: "1.0",
	Fields: []FieldDef{
		{Name: "order_id", Type: "string"},
		{Name: "customer_id", Type: "string"},
		{Name: "total", Type: "float", Min: float64Ptr(0)},
		{Name: "status", Type: "string"},
		{Name: "created_at", Type: "date"},
	},
	Required: []string{"order_id", "customer_id", "total"},
}

func float64Ptr(f float64) *float64 {
	return &f
}
