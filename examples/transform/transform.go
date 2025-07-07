package main

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"pipz"
)

// CSVRecord represents raw CSV data.
type CSVRecord struct {
	Fields []string
}

// DatabaseRecord represents structured database data.
type DatabaseRecord struct {
	ID        int
	Name      string
	Email     string
	Phone     string
	CreatedAt time.Time
}

// TransformContext holds both source and transformed data.
type TransformContext struct {
	CSV    *CSVRecord
	DB     *DatabaseRecord
	Errors []string
}

// ParseCSV converts CSV fields into a structured database record.
// Expected format: ID, Name, Email, Phone
func ParseCSV(ctx TransformContext) (TransformContext, error) {
	if ctx.CSV == nil || len(ctx.CSV.Fields) < 4 {
		return TransformContext{}, fmt.Errorf("insufficient CSV fields")
	}

	id, err := strconv.Atoi(ctx.CSV.Fields[0])
	if err != nil {
		ctx.Errors = append(ctx.Errors, fmt.Sprintf("invalid ID format: %v", err))
		id = 0
	}

	ctx.DB = &DatabaseRecord{
		ID:        id,
		Name:      ctx.CSV.Fields[1],
		Email:     ctx.CSV.Fields[2],
		Phone:     ctx.CSV.Fields[3],
		CreatedAt: time.Now(),
	}

	return ctx, nil
}

// ValidateEmail checks email format and adds errors if invalid.
func ValidateEmail(ctx TransformContext) TransformContext {
	if ctx.DB == nil {
		ctx.Errors = append(ctx.Errors, "no database record to validate")
		return ctx
	}

	email := strings.TrimSpace(ctx.DB.Email)
	if !strings.Contains(email, "@") {
		ctx.Errors = append(ctx.Errors, "invalid email format: missing @")
	}

	// Normalize email
	ctx.DB.Email = strings.ToLower(email)
	return ctx
}

// NormalizePhone formats phone numbers to a standard format.
// Converts various formats to +1-XXX-XXX-XXXX
func NormalizePhone(ctx TransformContext) TransformContext {
	if ctx.DB == nil {
		return ctx
	}

	// Remove all non-numeric characters
	phone := ctx.DB.Phone
	numeric := ""
	for _, r := range phone {
		if r >= '0' && r <= '9' {
			numeric += string(r)
		}
	}

	// Format as +1-XXX-XXX-XXXX
	if len(numeric) == 10 {
		numeric = "1" + numeric
	}
	if len(numeric) == 11 && numeric[0] == '1' {
		ctx.DB.Phone = fmt.Sprintf("+1-%s-%s-%s", numeric[1:4], numeric[4:7], numeric[7:11])
	} else {
		ctx.Errors = append(ctx.Errors, fmt.Sprintf("invalid phone number format: %s", phone))
	}

	return ctx
}

// EnrichData applies formatting and enrichment to the data.
// This includes title casing names and adding metadata.
func EnrichData(ctx TransformContext) TransformContext {
	if ctx.DB == nil {
		return ctx
	}

	// Title case the name
	words := strings.Fields(strings.ToLower(ctx.DB.Name))
	for i, word := range words {
		if len(word) > 0 {
			words[i] = strings.ToUpper(word[:1]) + word[1:]
		}
	}
	ctx.DB.Name = strings.Join(words, " ")

	return ctx
}

// ValidateTransform ensures the transformation completed successfully
func ValidateTransform(ctx TransformContext) error {
	if ctx.DB == nil {
		return fmt.Errorf("transformation failed: no database record created")
	}
	if len(ctx.Errors) > 0 {
		return fmt.Errorf("transformation had %d errors: %v", len(ctx.Errors), ctx.Errors)
	}
	return nil
}

// CreateTransformPipeline creates a CSV to database transformation pipeline
func CreateTransformPipeline() *pipz.Contract[TransformContext] {
	pipeline := pipz.NewContract[TransformContext]()
	pipeline.Register(
		pipz.Apply(ParseCSV),
		pipz.Transform(ValidateEmail),
		pipz.Transform(NormalizePhone),
		pipz.Transform(EnrichData),
		pipz.Validate(ValidateTransform),
	)
	return pipeline
}

// CreateLenientTransformPipeline creates a pipeline that continues on errors
func CreateLenientTransformPipeline() *pipz.Contract[TransformContext] {
	pipeline := pipz.NewContract[TransformContext]()
	pipeline.Register(
		pipz.Apply(ParseCSV),
		pipz.Transform(ValidateEmail),
		pipz.Transform(NormalizePhone),
		pipz.Transform(EnrichData),
		// Don't validate - allow records with errors
	)
	return pipeline
}

func main() {
	// Create transformation pipeline
	transformer := CreateTransformPipeline()

	// Test 1: Valid CSV data
	fmt.Println("Test 1: Valid CSV transformation")
	csv1 := TransformContext{
		CSV: &CSVRecord{
			Fields: []string{"123", "john doe", "JOHN.DOE@EXAMPLE.COM", "(555) 123-4567"},
		},
	}

	result, err := transformer.Process(csv1)
	if err != nil {
		log.Printf("Transformation failed: %v", err)
	} else {
		fmt.Printf("✓ Record transformed successfully:\n")
		fmt.Printf("  ID: %d\n", result.DB.ID)
		fmt.Printf("  Name: %s\n", result.DB.Name)
		fmt.Printf("  Email: %s\n", result.DB.Email)
		fmt.Printf("  Phone: %s\n", result.DB.Phone)
		fmt.Printf("  Errors: %v\n", result.Errors)
	}

	// Test 2: Invalid email
	fmt.Println("\nTest 2: Invalid email (lenient pipeline)")
	lenient := CreateLenientTransformPipeline()
	
	csv2 := TransformContext{
		CSV: &CSVRecord{
			Fields: []string{"124", "JANE SMITH", "janesmith.com", "555-987-6543"},
		},
	}

	result, err = lenient.Process(csv2)
	if err != nil {
		log.Printf("Transformation failed: %v", err)
	} else {
		fmt.Printf("✓ Record transformed with warnings:\n")
		fmt.Printf("  Name: %s\n", result.DB.Name)
		fmt.Printf("  Email: %s (normalized)\n", result.DB.Email)
		fmt.Printf("  Phone: %s\n", result.DB.Phone)
		fmt.Printf("  Warnings: %v\n", result.Errors)
	}

	// Test 3: Invalid phone format
	fmt.Println("\nTest 3: International phone number")
	csv3 := TransformContext{
		CSV: &CSVRecord{
			Fields: []string{"125", "Bob Johnson", "bob@example.com", "+44 20 7946 0958"},
		},
	}

	result, err = lenient.Process(csv3)
	if err != nil {
		log.Printf("Transformation failed: %v", err)
	} else {
		fmt.Printf("✓ Record transformed:\n")
		fmt.Printf("  Name: %s\n", result.DB.Name)
		fmt.Printf("  Email: %s\n", result.DB.Email)
		fmt.Printf("  Phone: %s (original)\n", result.DB.Phone)
		fmt.Printf("  Warnings: %v\n", result.Errors)
	}

	// Test 4: Insufficient fields
	fmt.Println("\nTest 4: Insufficient CSV fields")
	csv4 := TransformContext{
		CSV: &CSVRecord{
			Fields: []string{"126", "Alice"},
		},
	}

	_, err = transformer.Process(csv4)
	if err != nil {
		fmt.Printf("✗ Transformation failed as expected: %v\n", err)
	}

	// Test 5: Batch processing simulation
	fmt.Println("\nTest 5: Batch processing")
	records := []TransformContext{
		{CSV: &CSVRecord{Fields: []string{"200", "mike davis", "mike@test.com", "1234567890"}}},
		{CSV: &CSVRecord{Fields: []string{"201", "sarah connor", "sarah@skynet.com", "(888) 555-1234"}}},
		{CSV: &CSVRecord{Fields: []string{"202", "john connor", "john@resistance.org", "310-555-9876"}}},
	}

	fmt.Println("Processing batch of 3 records...")
	successful := 0
	for i, record := range records {
		result, err := lenient.Process(record)
		if err == nil && result.DB != nil {
			successful++
			fmt.Printf("  [%d] ✓ %s -> %s\n", i+1, result.DB.Name, result.DB.Phone)
		}
	}
	fmt.Printf("Batch complete: %d/%d records processed successfully\n", successful, len(records))
}