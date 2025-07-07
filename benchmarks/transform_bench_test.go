package benchmarks

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"
	
	"pipz"
)

// Transform types
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

// Transform processors
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

func BenchmarkTransformPipeline(b *testing.B) {
	// Setup pipeline
	pipeline := pipz.NewContract[ETLContext]()
	pipeline.Register(
		pipz.Apply(parseCSV),
		pipz.Transform(normalizeEmail),
		pipz.Transform(formatPhone),
		pipz.Transform(titleCaseName),
	)

	ctx := ETLContext{
		CSV: &CSVData{
			Fields: []string{"123", "john doe", "JOHN.DOE@EXAMPLE.COM", "555-123-4567"},
		},
	}

	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		// Fresh context each iteration
		freshCtx := ETLContext{
			CSV: &CSVData{
				Fields: ctx.CSV.Fields,
			},
		}
		
		_, err := pipeline.Process(freshCtx)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkTransformBatch(b *testing.B) {
	// Setup lenient pipeline (no validation)
	pipeline := pipz.NewContract[ETLContext]()
	pipeline.Register(
		pipz.Apply(parseCSV),
		pipz.Transform(normalizeEmail),
		pipz.Transform(formatPhone),
		pipz.Transform(titleCaseName),
	)

	// Create batch of records
	batch := []ETLContext{
		{CSV: &CSVData{Fields: []string{"100", "alice wonder", "alice@test.com", "5551234567"}}},
		{CSV: &CSVData{Fields: []string{"101", "bob builder", "BOB@TEST.COM", "(555) 987-6543"}}},
		{CSV: &CSVData{Fields: []string{"102", "charlie brown", "charlie@example.com", "555-555-5555"}}},
		{CSV: &CSVData{Fields: []string{"103", "diana prince", "diana@amazon.com", "9995551234"}}},
		{CSV: &CSVData{Fields: []string{"104", "edward norton", "ed@fight.club", "4155551234"}}},
	}

	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		for _, ctx := range batch {
			freshCtx := ETLContext{
				CSV: &CSVData{
					Fields: ctx.CSV.Fields,
				},
			}
			
			_, err := pipeline.Process(freshCtx)
			if err != nil {
				b.Fatal(err)
			}
		}
	}
}

func BenchmarkParseCSV(b *testing.B) {
	ctx := ETLContext{
		CSV: &CSVData{
			Fields: []string{"123", "john doe", "john@example.com", "555-123-4567"},
		},
	}

	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		freshCtx := ETLContext{
			CSV: &CSVData{
				Fields: ctx.CSV.Fields,
			},
		}
		
		_, err := parseCSV(freshCtx)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkNormalization(b *testing.B) {
	ctx := ETLContext{
		DB: &DBRecord{
			Name:  "JOHN DOE",
			Email: "JOHN.DOE@EXAMPLE.COM",
			Phone: "(555) 123-4567",
		},
	}

	// Pipeline with just normalization
	pipeline := pipz.NewContract[ETLContext]()
	pipeline.Register(
		pipz.Transform(normalizeEmail),
		pipz.Transform(formatPhone),
		pipz.Transform(titleCaseName),
	)

	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		freshCtx := ETLContext{
			DB: &DBRecord{
				Name:  ctx.DB.Name,
				Email: ctx.DB.Email,
				Phone: ctx.DB.Phone,
			},
		}
		
		_, err := pipeline.Process(freshCtx)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkTransformChain(b *testing.B) {
	// Parser pipeline
	parser := pipz.NewContract[ETLContext]()
	parser.Register(pipz.Apply(parseCSV))

	// Normalizer pipeline
	normalizer := pipz.NewContract[ETLContext]()
	normalizer.Register(
		pipz.Transform(normalizeEmail),
		pipz.Transform(formatPhone),
		pipz.Transform(titleCaseName),
	)

	// Chain them
	chain := pipz.NewChain[ETLContext]()
	chain.Add(parser, normalizer)

	ctx := ETLContext{
		CSV: &CSVData{
			Fields: []string{"999", "test user", "TEST@EXAMPLE.COM", "8005551234"},
		},
	}

	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		freshCtx := ETLContext{
			CSV: &CSVData{
				Fields: ctx.CSV.Fields,
			},
		}
		
		_, err := chain.Process(freshCtx)
		if err != nil {
			b.Fatal(err)
		}
	}
}