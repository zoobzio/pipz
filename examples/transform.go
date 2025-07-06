package examples

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// TransformKey is the contract key type for data transformation pipelines.
type TransformKey string

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
		return ctx, fmt.Errorf("insufficient CSV fields")
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
func ValidateEmail(ctx TransformContext) (TransformContext, error) {
	if ctx.DB == nil {
		return ctx, fmt.Errorf("no database record to validate")
	}

	email := strings.TrimSpace(ctx.DB.Email)
	if !strings.Contains(email, "@") {
		ctx.Errors = append(ctx.Errors, "invalid email format: missing @")
	}

	// Normalize email
	ctx.DB.Email = strings.ToLower(email)
	return ctx, nil
}

// NormalizePhone formats phone numbers to a standard format.
// Converts various formats to +1-XXX-XXX-XXXX
func NormalizePhone(ctx TransformContext) (TransformContext, error) {
	if ctx.DB == nil {
		return ctx, nil
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

	return ctx, nil
}

// EnrichData applies formatting and enrichment to the data.
// This includes title casing names and adding metadata.
func EnrichData(ctx TransformContext) (TransformContext, error) {
	if ctx.DB == nil {
		return ctx, nil
	}

	// Title case the name
	words := strings.Fields(strings.ToLower(ctx.DB.Name))
	for i, word := range words {
		if len(word) > 0 {
			words[i] = strings.ToUpper(word[:1]) + word[1:]
		}
	}
	ctx.DB.Name = strings.Join(words, " ")

	return ctx, nil
}