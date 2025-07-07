package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"pipz"
)

// Transform types for demo
type ETLContext struct {
	CSV    *CSVData
	DB     *DBRecord
	Errors []string
}

type CSVData struct {
	Fields []string
}

type DBRecord struct {
	ID        int
	Name      string
	Email     string
	Phone     string
	CreatedAt time.Time
}

// Transform functions
func parseCSV(ctx ETLContext) (ETLContext, error) {
	if ctx.CSV == nil || len(ctx.CSV.Fields) < 4 {
		return ETLContext{}, fmt.Errorf("insufficient CSV fields")
	}

	id, err := strconv.Atoi(ctx.CSV.Fields[0])
	if err != nil {
		ctx.Errors = append(ctx.Errors, "invalid ID format")
		id = 0
	}

	ctx.DB = &DBRecord{
		ID:        id,
		Name:      ctx.CSV.Fields[1],
		Email:     ctx.CSV.Fields[2],
		Phone:     ctx.CSV.Fields[3],
		CreatedAt: time.Now(),
	}

	return ctx, nil
}

func normalizeEmail(ctx ETLContext) ETLContext {
	if ctx.DB == nil {
		return ctx
	}

	email := strings.TrimSpace(ctx.DB.Email)
	if !strings.Contains(email, "@") {
		ctx.Errors = append(ctx.Errors, "invalid email format")
	}
	ctx.DB.Email = strings.ToLower(email)
	return ctx
}

func formatPhone(ctx ETLContext) ETLContext {
	if ctx.DB == nil {
		return ctx
	}

	// Extract digits only
	digits := ""
	for _, r := range ctx.DB.Phone {
		if r >= '0' && r <= '9' {
			digits += string(r)
		}
	}

	// Format US phone numbers
	if len(digits) == 10 {
		ctx.DB.Phone = fmt.Sprintf("(%s) %s-%s", 
			digits[:3], digits[3:6], digits[6:])
	} else {
		ctx.Errors = append(ctx.Errors, "invalid phone format")
	}

	return ctx
}

func titleCaseName(ctx ETLContext) ETLContext {
	if ctx.DB == nil {
		return ctx
	}

	words := strings.Fields(strings.ToLower(ctx.DB.Name))
	for i, word := range words {
		if len(word) > 0 {
			words[i] = strings.ToUpper(word[:1]) + word[1:]
		}
	}
	ctx.DB.Name = strings.Join(words, " ")
	return ctx
}

func runTransformDemo() {
	section("ETL TRANSFORMATION PIPELINE")
	
	info("Use Case: CSV to Database Import")
	info("• Parse CSV data into structured records")
	info("• Normalize and validate data")
	info("• Apply formatting rules")
	info("• Handle errors gracefully")

	// Create strict pipeline
	strictPipeline := pipz.NewContract[ETLContext]()
	strictPipeline.Register(
		pipz.Apply(parseCSV),
		pipz.Transform(normalizeEmail),
		pipz.Transform(formatPhone),
		pipz.Transform(titleCaseName),
		pipz.Validate(func(ctx ETLContext) error {
			if len(ctx.Errors) > 0 {
				return fmt.Errorf("transformation errors: %v", ctx.Errors)
			}
			return nil
		}),
	)

	// Create lenient pipeline
	lenientPipeline := pipz.NewContract[ETLContext]()
	lenientPipeline.Register(
		pipz.Apply(parseCSV),
		pipz.Transform(normalizeEmail),
		pipz.Transform(formatPhone),
		pipz.Transform(titleCaseName),
		// No validation - continues with errors
	)

	code("go", `// Strict pipeline fails on errors
strict := pipz.NewContract[ETLContext]()
strict.Register(
    pipz.Apply(parseCSV),
    pipz.Transform(normalizeEmail),
    pipz.Transform(formatPhone),
    pipz.Transform(titleCaseName),
    pipz.Validate(checkErrors),
)

// Lenient pipeline logs warnings
lenient := pipz.NewContract[ETLContext]()
lenient.Register(
    // Same transforms, no validation
)`)

	// Test Case 1: Valid CSV
	fmt.Println("\n📊 Test Case 1: Valid CSV Data")
	ctx1 := ETLContext{
		CSV: &CSVData{
			Fields: []string{"123", "john doe", "JOHN@EXAMPLE.COM", "555-123-4567"},
		},
	}

	result, err := strictPipeline.Process(ctx1)
	if err != nil {
		showError(fmt.Sprintf("Transform failed: %v", err))
	} else {
		success("Data transformed successfully")
		fmt.Printf("   Name: %s\n", result.DB.Name)
		fmt.Printf("   Email: %s\n", result.DB.Email)
		fmt.Printf("   Phone: %s\n", result.DB.Phone)
	}

	// Test Case 2: Invalid data (lenient)
	fmt.Println("\n📊 Test Case 2: Invalid Data (Lenient Mode)")
	ctx2 := ETLContext{
		CSV: &CSVData{
			Fields: []string{"abc", "JANE SMITH", "jane.smith", "12345"},
		},
	}

	result, err = lenientPipeline.Process(ctx2)
	if err != nil {
		showError(fmt.Sprintf("Transform failed: %v", err))
	} else {
		fmt.Println("⚠️  Transform completed with warnings:")
		fmt.Printf("   Name: %s\n", result.DB.Name)
		fmt.Printf("   Email: %s\n", result.DB.Email)
		fmt.Printf("   Phone: %s\n", result.DB.Phone)
		fmt.Printf("   Warnings: %v\n", result.Errors)
	}

	// Test Case 3: Batch simulation
	fmt.Println("\n📊 Test Case 3: Batch Processing")
	batch := []ETLContext{
		{CSV: &CSVData{Fields: []string{"200", "alice wonder", "alice@test.com", "5551234567"}}},
		{CSV: &CSVData{Fields: []string{"201", "bob builder", "BOB@TEST.COM", "(555) 987-6543"}}},
		{CSV: &CSVData{Fields: []string{"202", "charlie brown", "charlie@test", "555"}}},
	}

	successful := 0
	for i, ctx := range batch {
		result, err := lenientPipeline.Process(ctx)
		if err == nil && result.DB != nil {
			if len(result.Errors) == 0 {
				successful++
				fmt.Printf("   [%d] ✅ %s processed\n", i+1, result.DB.Name)
			} else {
				fmt.Printf("   [%d] ⚠️  %s processed with warnings\n", i+1, result.DB.Name)
			}
		}
	}
	fmt.Printf("   Batch complete: %d/%d clean records\n", successful, len(batch))

	fmt.Println("\n🔍 ETL Features:")
	info("• Multi-stage transformation")
	info("• Error collection without failing")
	info("• Data normalization and enrichment")
	info("• Flexible validation strategies")
}