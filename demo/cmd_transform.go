package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"pipz"
	"pipz/demo/testutil"
)

var transformCmd = &cobra.Command{
	Use:   "transform",
	Short: "Data Transformation Pipeline demonstration",
	Long:  `Demonstrates CSV to database conversion with validation and normalization.`,
	Run:   runTransformDemo,
}

func init() {
	rootCmd.AddCommand(transformCmd)
}

// Demo types for data transformation
type TransformKey string

const (
	// TransformCSVToDB is the key for CSV to database transformation
	TransformCSVToDB TransformKey = "csv-to-db"
)

type CSVRecord struct {
	Fields []string
}

type DatabaseRecord struct {
	ID        int
	Name      string
	Email     string
	Phone     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type TransformContext struct {
	CSV    *CSVRecord
	DB     *DatabaseRecord
	Errors []string
}

func runTransformDemo(cmd *cobra.Command, args []string) {
	pp := testutil.NewPrettyPrinter()
	
	// Get interactive flag from parent command
	interactive, _ := cmd.Flags().GetBool("interactive")
	pp.SetInteractive(interactive)
	
	pp.Section("🔄 DATA TRANSFORMATION PIPELINE DEMO")
	
	pp.SubSection("Pipeline Configuration")
	pp.Code("go", `// Define transformation context to hold both source and destination
type TransformKey string

type TransformContext struct {
    CSV    *CSVRecord      // Source data
    DB     *DatabaseRecord // Transformed data
    Errors []string        // Accumulated errors
}

// Register transformation pipeline
transformContract := pipz.GetContract[TransformContext](TransformCSVToDB)
transformContract.Register(
    parseCSV,        // Parse CSV fields to struct
    validateEmail,   // Validate email format
    normalizePhone,  // Format phone numbers
    enrichData,      // Title case names, lowercase emails
)`)
	
	// Create and register the pipeline
	transformContract := pipz.GetContract[TransformContext](TransformCSVToDB)
	err := transformContract.Register(parseCSV, validateTransformEmail, normalizePhone, enrichData)
	if err != nil {
		pp.Error(fmt.Sprintf("Failed to register pipeline: %v", err))
		return
	}
	
	pp.SubSection("Live Demonstrations")
	
	// Demo 1: Valid transformation
	pp.Info("Demo 1: Valid CSV transformation")
	ctx1 := TransformContext{
		CSV: &CSVRecord{
			Fields: []string{"123", "JOHN DOE", "john.doe@example.com", "(555) 123-4567"},
		},
		Errors: []string{},
	}
	
	pp.Info("  Input: " + strings.Join(ctx1.CSV.Fields, ", "))
	
	result1, err := transformContract.Process(ctx1)
	if err != nil {
		pp.Error(fmt.Sprintf("Error: %v", err))
	} else {
		pp.Success("Transformation successful")
		pp.Info(fmt.Sprintf("  ID: %d", result1.DB.ID))
		pp.Info(fmt.Sprintf("  Name: %s (formatted)", result1.DB.Name))
		pp.Info(fmt.Sprintf("  Email: %s (normalized)", result1.DB.Email))
		pp.Info(fmt.Sprintf("  Phone: %s (standardized)", result1.DB.Phone))
	}
	
	// Demo 2: Invalid email
	pp.Info("")
	pp.Info("Demo 2: Invalid email handling")
	ctx2 := TransformContext{
		CSV: &CSVRecord{
			Fields: []string{"456", "Jane Smith", "invalid-email", "555-987-6543"},
		},
		Errors: []string{},
	}
	
	pp.Info("  Input: " + strings.Join(ctx2.CSV.Fields, ", "))
	
	result2, err := transformContract.Process(ctx2)
	if err != nil {
		pp.Error(fmt.Sprintf("Error: %v", err))
	} else {
		pp.Warning("Transformation completed with errors")
		for _, e := range result2.Errors {
			pp.Error("  " + e)
		}
		pp.Info(fmt.Sprintf("  Data still processed: %s (%s)", result2.DB.Name, result2.DB.Phone))
	}
	
	// Demo 3: Multiple phone formats
	pp.Info("")
	pp.Info("Demo 3: Phone number normalization")
	phoneTests := []string{
		"(555) 123-4567",
		"5551234567",
		"1-555-123-4567",
		"555.123.4567",
	}
	
	for _, phone := range phoneTests {
		ctx := TransformContext{
			CSV: &CSVRecord{
				Fields: []string{"999", "Test User", "test@test.com", phone},
			},
			Errors: []string{},
		}
		
		result, _ := transformContract.Process(ctx)
		pp.Info(fmt.Sprintf("  %s → %s", phone, result.DB.Phone))
	}
	
	pp.SubSection("Transformation Process")
	pp.Step(1, "Parse CSV fields into structured database record")
	pp.Step(2, "Validate email addresses with detailed error messages")
	pp.Step(3, "Normalize phone numbers to +1-XXX-XXX-XXXX format")
	pp.Step(4, "Enrich data with formatting and timestamps")
	
	pp.SubSection("Key Features")
	pp.Feature("📊", "Format Conversion", "CSV → Structured Database Records")
	pp.Feature("✅", "Multi-Field Validation", "Email, phone, and data integrity checks")
	pp.Feature("🌐", "Data Normalization", "Consistent formatting across records")
	pp.Feature("⚠️", "Error Accumulation", "Collects all validation errors, not just first")
	pp.Feature("🔄", "Graceful Processing", "Continues despite errors when possible")
	
	pp.SubSection("Name Formatting Examples")
	pp.Info("john doe      → John Doe")
	pp.Info("JANE SMITH    → Jane Smith")
	pp.Info("mArY jOhNsOn  → Mary Johnson")
}

// Pipeline processors
func parseCSV(ctx TransformContext) ([]byte, error) {
	if ctx.CSV == nil {
		return nil, fmt.Errorf("no CSV data provided")
	}

	if len(ctx.CSV.Fields) < 4 {
		return nil, fmt.Errorf("insufficient fields: expected at least 4, got %d", len(ctx.CSV.Fields))
	}

	// Parse ID
	id, err := strconv.Atoi(strings.TrimSpace(ctx.CSV.Fields[0]))
	if err != nil {
		ctx.Errors = append(ctx.Errors, fmt.Sprintf("invalid ID format: %v", err))
		id = 0
	}

	ctx.DB = &DatabaseRecord{
		ID:        id,
		Name:      strings.TrimSpace(ctx.CSV.Fields[1]),
		Email:     strings.TrimSpace(ctx.CSV.Fields[2]),
		Phone:     strings.TrimSpace(ctx.CSV.Fields[3]),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	return pipz.Encode(ctx)
}

func validateTransformEmail(ctx TransformContext) ([]byte, error) {
	if ctx.DB == nil {
		return nil, fmt.Errorf("no database record to validate")
	}

	email := ctx.DB.Email
	if email == "" {
		ctx.Errors = append(ctx.Errors, "email is required")
	} else if !strings.Contains(email, "@") {
		ctx.Errors = append(ctx.Errors, "invalid email format: missing @")
	} else {
		parts := strings.Split(email, "@")
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			ctx.Errors = append(ctx.Errors, "invalid email format")
		} else if !strings.Contains(parts[1], ".") {
			ctx.Errors = append(ctx.Errors, "invalid email domain")
		}
	}

	// Continue processing even with validation errors
	return pipz.Encode(ctx)
}

func normalizePhone(ctx TransformContext) ([]byte, error) {
	if ctx.DB == nil {
		return nil, fmt.Errorf("no database record to normalize")
	}

	// Remove all non-digit characters
	digits := ""
	for _, ch := range ctx.DB.Phone {
		if ch >= '0' && ch <= '9' {
			digits += string(ch)
		}
	}

	// Format based on length
	switch len(digits) {
	case 10: // US phone without country code
		ctx.DB.Phone = fmt.Sprintf("+1-%s-%s-%s", digits[0:3], digits[3:6], digits[6:10])
	case 11: // US phone with country code
		if digits[0] == '1' {
			ctx.DB.Phone = fmt.Sprintf("+1-%s-%s-%s", digits[1:4], digits[4:7], digits[7:11])
		} else {
			ctx.Errors = append(ctx.Errors, "invalid phone number format")
		}
	default:
		ctx.Errors = append(ctx.Errors, fmt.Sprintf("invalid phone length: %d digits", len(digits)))
	}

	return pipz.Encode(ctx)
}

func enrichData(ctx TransformContext) ([]byte, error) {
	if ctx.DB == nil {
		return nil, fmt.Errorf("no database record to enrich")
	}

	// Title case the name
	words := strings.Fields(ctx.DB.Name)
	for i, word := range words {
		if len(word) > 0 {
			words[i] = strings.ToUpper(word[0:1]) + strings.ToLower(word[1:])
		}
	}
	ctx.DB.Name = strings.Join(words, " ")

	// Lowercase email
	ctx.DB.Email = strings.ToLower(ctx.DB.Email)

	// Add processing timestamp
	ctx.DB.UpdatedAt = time.Now()

	return pipz.Encode(ctx)
}