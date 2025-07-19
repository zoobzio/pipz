package etl

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/zoobzio/pipz"
)

func TestParseCSV(t *testing.T) {
	// Create test CSV file
	csvContent := `id,email,name,age,created_at,is_active
1,john@example.com,John Doe,30,2024-01-01,true
2,jane@example.com,Jane Smith,25,2024-01-02,true
3,invalid-email,Bob Wilson,40,2024-01-03,false`

	tmpDir := t.TempDir()
	csvFile := filepath.Join(tmpDir, "test.csv")
	if err := os.WriteFile(csvFile, []byte(csvContent), 0644); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	records, err := ParseCSV(ctx, csvFile)
	if err != nil {
		t.Fatalf("ParseCSV failed: %v", err)
	}

	if len(records) != 3 {
		t.Errorf("Expected 3 records, got %d", len(records))
	}

	// Check first record
	if records[0].Data == nil {
		t.Fatal("First record has no data")
	}
	if records[0].Data["email"] != "john@example.com" {
		t.Errorf("Expected email 'john@example.com', got %v", records[0].Data["email"])
	}
}

func TestParseJSON(t *testing.T) {
	// Create test JSON file (newline-delimited)
	jsonContent := `{"id":"1","email":"john@example.com","name":"John Doe","age":30}
{"id":"2","email":"jane@example.com","name":"Jane Smith","age":25}
{"id":"3","email":"bob@example.com","name":"Bob Wilson","age":40}`

	tmpDir := t.TempDir()
	jsonFile := filepath.Join(tmpDir, "test.json")
	if err := os.WriteFile(jsonFile, []byte(jsonContent), 0644); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	records, err := ParseJSON(ctx, jsonFile)
	if err != nil {
		t.Fatalf("ParseJSON failed: %v", err)
	}

	if len(records) != 3 {
		t.Errorf("Expected 3 records, got %d", len(records))
	}
}

func TestValidateSchema(t *testing.T) {
	validator := ValidateSchema(CustomerSchema)
	ctx := context.Background()

	tests := []struct {
		name    string
		record  Record
		wantErr bool
		errMsg  string
	}{
		{
			name: "Valid record",
			record: Record{
				Data: map[string]interface{}{
					"id":         "1",
					"email":      "test@example.com",
					"name":       "Test User",
					"age":        "30",
					"created_at": "2024-01-01",
				},
			},
			wantErr: false,
		},
		{
			name: "Missing required field",
			record: Record{
				Data: map[string]interface{}{
					"id":   "1",
					"name": "Test User",
				},
			},
			wantErr: true,
			errMsg:  "missing required field: email",
		},
		{
			name: "Invalid email",
			record: Record{
				Data: map[string]interface{}{
					"id":    "1",
					"email": "invalid-email",
					"name":  "Test User",
				},
			},
			wantErr: true,
			errMsg:  "invalid email format",
		},
		{
			name: "Age out of range",
			record: Record{
				Data: map[string]interface{}{
					"id":    "1",
					"email": "test@example.com",
					"age":   "200",
				},
			},
			wantErr: true,
			errMsg:  "above maximum",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator(ctx, tt.record)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateSchema() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("Expected error containing '%s', got: %v", tt.errMsg, err)
			}
		})
	}
}

func TestNormalizeTypes(t *testing.T) {
	normalizer := NormalizeTypes(CustomerSchema)
	ctx := context.Background()

	record := Record{
		Data: map[string]interface{}{
			"id":        "1",
			"email":     "test@example.com",
			"age":       "30",   // String that should become int
			"is_active": "true", // String that should become bool
		},
	}

	normalized := normalizer(ctx, record)

	// Check type conversions
	if age, ok := normalized.Data["age"].(int64); !ok || age != 30 {
		t.Errorf("Expected age to be int64(30), got %T(%v)", normalized.Data["age"], normalized.Data["age"])
	}

	if active, ok := normalized.Data["is_active"].(bool); !ok || !active {
		t.Errorf("Expected is_active to be bool(true), got %T(%v)", normalized.Data["is_active"], normalized.Data["is_active"])
	}
}

func TestEnrichRecord(t *testing.T) {
	ctx := context.Background()

	record := Record{
		Data: map[string]interface{}{
			"email": "user@example.com",
			"name":  "Test User",
		},
	}

	enriched := EnrichRecord(ctx, record)

	// Check enrichments
	if enriched.ProcessedAt.IsZero() {
		t.Error("Expected ProcessedAt to be set")
	}

	if domain, ok := enriched.Data["email_domain"].(string); !ok || domain != "example.com" {
		t.Errorf("Expected email_domain 'example.com', got %v", enriched.Data["email_domain"])
	}

	if score, ok := enriched.Data["quality_score"].(float64); !ok || score != 1.0 {
		t.Errorf("Expected quality_score 1.0, got %v", enriched.Data["quality_score"])
	}
}

func TestMapFields(t *testing.T) {
	mapper := MapFields(map[string]string{
		"customer_email": "email",
		"customer_name":  "name",
	})
	ctx := context.Background()

	record := Record{
		Data: map[string]interface{}{
			"customer_email": "test@example.com",
			"customer_name":  "Test User",
			"other_field":    "unchanged",
		},
	}

	mapped := mapper(ctx, record)

	// Check mappings
	if mapped.Data["email"] != "test@example.com" {
		t.Errorf("Expected email field, got %v", mapped.Data)
	}

	if mapped.Data["name"] != "Test User" {
		t.Errorf("Expected name field, got %v", mapped.Data)
	}

	if mapped.Data["other_field"] != "unchanged" {
		t.Errorf("Expected other_field to remain, got %v", mapped.Data)
	}

	// Original field names should be replaced
	if _, exists := mapped.Data["customer_email"]; exists {
		t.Error("Original field customer_email should not exist")
	}
}

func TestCSVToJSONPipeline(t *testing.T) {
	// Reset stats for clean test
	ResetStats()

	// Create test CSV
	csvContent := `id,email,name,age,created_at,is_active
1,john@example.com,John Doe,30,2024-01-01,true
2,jane@example.com,Jane Smith,25,2024-01-02,true
3,invalid-email,Bob Wilson,40,2024-01-03,false
4,alice@example.com,Alice Brown,200,2024-01-04,true`

	tmpDir := t.TempDir()
	csvFile := filepath.Join(tmpDir, "customers.csv")
	outputFile := filepath.Join(tmpDir, "customers.json")

	if err := os.WriteFile(csvFile, []byte(csvContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Parse CSV
	ctx := context.Background()
	records, err := ParseCSV(ctx, csvFile)
	if err != nil {
		t.Fatal(err)
	}

	// Create and run pipeline
	pipeline := CreateCSVToJSONPipeline(CustomerSchema, outputFile)
	_, err = pipeline.Process(ctx, records)
	if err != nil {
		t.Fatalf("Pipeline failed: %v", err)
	}

	// Check output file exists
	if _, err := os.Stat(outputFile); os.IsNotExist(err) {
		t.Error("Output file was not created")
	}

	// Check stats
	stats := GetStats()
	if stats.ValidRecords != 2 { // Only 2 records should be valid (invalid email and age > 150)
		t.Errorf("Expected 2 valid records, got %d", stats.ValidRecords)
	}
	if stats.InvalidRecords != 2 {
		t.Errorf("Expected 2 invalid records, got %d", stats.InvalidRecords)
	}
}

func TestMultiSourcePipeline(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files
	csvContent := `id,email,name
1,test@example.com,Test User`
	jsonContent := `{"id":"2","email":"json@example.com","name":"JSON User"}`

	csvFile := filepath.Join(tmpDir, "data.csv")
	jsonFile := filepath.Join(tmpDir, "data.json")

	if err := os.WriteFile(csvFile, []byte(csvContent), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(jsonFile, []byte(jsonContent), 0644); err != nil {
		t.Fatal(err)
	}

	pipeline := CreateMultiSourcePipeline(CustomerSchema)
	ctx := context.Background()

	// Process CSV
	outputCSV, err := pipeline.Process(ctx, csvFile)
	if err != nil {
		t.Errorf("Failed to process CSV: %v", err)
	}
	if !strings.HasSuffix(outputCSV, ".output.json") {
		t.Errorf("Expected .output.json suffix, got %s", outputCSV)
	}

	// Process JSON
	outputJSON, err := pipeline.Process(ctx, jsonFile)
	if err != nil {
		t.Errorf("Failed to process JSON: %v", err)
	}
	if !strings.HasSuffix(outputJSON, ".transformed.json") {
		t.Errorf("Expected .transformed.json suffix, got %s", outputJSON)
	}

	// Test unsupported file
	_, err = pipeline.Process(ctx, "file.txt")
	if err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Errorf("Expected unsupported file error, got: %v", err)
	}
}

func TestStreamingETLPipeline(t *testing.T) {
	pipeline := CreateStreamingETLPipeline(CustomerSchema)
	ctx := context.Background()

	record := Record{
		Data: map[string]interface{}{
			"id":            "1",
			"email":         "test@example.com",
			"customer_name": "Test User", // Will be mapped to "name"
			"age":           "30",
			"created_at":    "2024-01-01",
		},
	}

	result, err := pipeline.Process(ctx, record)
	if err != nil {
		t.Fatalf("Streaming pipeline failed: %v", err)
	}

	// Check transformations
	if result.Data["name"] != "Test User" {
		t.Errorf("Expected field mapping customer_name -> name")
	}

	if _, ok := result.Data["email_domain"]; !ok {
		t.Error("Expected email_domain enrichment")
	}

	if result.ProcessedAt.IsZero() {
		t.Error("Expected ProcessedAt to be set")
	}
}

func TestErrorHandling(t *testing.T) {
	// Test malformed CSV
	csvContent := `id,email,name
1,test@example.com
2,missing@data.com,Missing,Extra,Fields`

	tmpDir := t.TempDir()
	csvFile := filepath.Join(tmpDir, "malformed.csv")
	if err := os.WriteFile(csvFile, []byte(csvContent), 0644); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	records, err := ParseCSV(ctx, csvFile)
	if err != nil {
		t.Fatal(err)
	}

	// Should still return records, some with errors
	if len(records) != 2 {
		t.Errorf("Expected 2 records (including errors), got %d", len(records))
	}
}

func TestRetryOnFailure(t *testing.T) {
	attempts := 0
	failingProcessor := pipz.Apply("failing", func(ctx context.Context, r Record) (Record, error) {
		attempts++
		if attempts < 3 {
			return r, fmt.Errorf("temporary failure")
		}
		return r, nil
	})

	pipeline := pipz.Retry(failingProcessor, 3)
	ctx := context.Background()

	record := Record{Data: map[string]interface{}{"id": "1"}}
	_, err := pipeline.Process(ctx, record)

	if err != nil {
		t.Errorf("Expected success after retries, got: %v", err)
	}

	if attempts != 3 {
		t.Errorf("Expected 3 attempts, got %d", attempts)
	}
}

func TestConcurrentProcessing(t *testing.T) {
	// Test that stats are thread-safe
	ResetStats()

	validator := ValidateSchema(CustomerSchema)
	ctx := context.Background()

	// Process records concurrently
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			record := Record{
				Data: map[string]interface{}{
					"id":    string(rune('0' + id)),
					"email": "test@example.com",
				},
			}
			validator(ctx, record)
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	stats := GetStats()
	if stats.ValidRecords != 10 {
		t.Errorf("Expected 10 valid records from concurrent processing, got %d", stats.ValidRecords)
	}
}

func TestProgressTracking(t *testing.T) {
	ResetStats()
	ctx := context.Background()

	// Process some records to update stats
	for i := 0; i < 5; i++ {
		record := Record{
			SourceFile: "test.csv",
			Data: map[string]interface{}{
				"id": string(rune('0' + i)),
			},
		}
		// Simulate record processing
		atomic.AddInt64(&globalStats.TotalRecords, 1)
		TrackProgress(ctx, record)
	}

	stats := GetStats()
	elapsed := time.Since(stats.StartTime)
	if elapsed < 0 {
		t.Error("StartTime should be in the past")
	}
}
