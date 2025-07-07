package main

import (
	"strings"
	"testing"

	"pipz"
)

func TestParseCSV(t *testing.T) {
	tests := []struct {
		name    string
		ctx     TransformContext
		wantErr bool
		errMsg  string
		wantID  int
	}{
		{
			name: "valid CSV",
			ctx: TransformContext{
				CSV: &CSVRecord{
					Fields: []string{"123", "John Doe", "john@example.com", "555-1234"},
				},
			},
			wantErr: false,
			wantID:  123,
		},
		{
			name: "insufficient fields",
			ctx: TransformContext{
				CSV: &CSVRecord{
					Fields: []string{"123", "John"},
				},
			},
			wantErr: true,
			errMsg:  "insufficient CSV fields",
		},
		{
			name: "nil CSV",
			ctx: TransformContext{
				CSV: nil,
			},
			wantErr: true,
			errMsg:  "insufficient CSV fields",
		},
		{
			name: "invalid ID",
			ctx: TransformContext{
				CSV: &CSVRecord{
					Fields: []string{"abc", "John Doe", "john@example.com", "555-1234"},
				},
			},
			wantErr: false,
			wantID:  0, // Should default to 0 but add error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseCSV(tt.ctx)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseCSV() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && tt.errMsg != "" {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("ParseCSV() error = %v, want error containing %v", err, tt.errMsg)
				}
			}
			if err == nil && result.DB != nil && result.DB.ID != tt.wantID {
				t.Errorf("ParseCSV() ID = %v, want %v", result.DB.ID, tt.wantID)
			}
			if err != nil && result.CSV != nil {
				t.Error("ParseCSV() should return zero value on error")
			}
		})
	}
}

func TestValidateEmail(t *testing.T) {
	tests := []struct {
		name          string
		ctx           TransformContext
		expectedEmail string
		expectError   bool
	}{
		{
			name: "valid email",
			ctx: TransformContext{
				DB: &DatabaseRecord{
					Email: "John.Doe@Example.COM",
				},
			},
			expectedEmail: "john.doe@example.com",
			expectError:   false,
		},
		{
			name: "missing @",
			ctx: TransformContext{
				DB: &DatabaseRecord{
					Email: "johndoe.com",
				},
			},
			expectedEmail: "johndoe.com",
			expectError:   true,
		},
		{
			name: "email with spaces",
			ctx: TransformContext{
				DB: &DatabaseRecord{
					Email: "  john@example.com  ",
				},
			},
			expectedEmail: "john@example.com",
			expectError:   false,
		},
		{
			name: "nil DB",
			ctx: TransformContext{
				DB: nil,
			},
			expectedEmail: "",
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateEmail(tt.ctx)
			
			if tt.ctx.DB == nil {
				if !containsError(result.Errors, "no database record") {
					t.Error("ValidateEmail() should add error for nil DB")
				}
				return
			}
			
			if result.DB.Email != tt.expectedEmail {
				t.Errorf("ValidateEmail() email = %v, want %v", result.DB.Email, tt.expectedEmail)
			}
			
			if tt.expectError && !containsError(result.Errors, "invalid email format") {
				t.Error("ValidateEmail() should add error for invalid email")
			}
		})
	}
}

func TestNormalizePhone(t *testing.T) {
	tests := []struct {
		name          string
		ctx           TransformContext
		expectedPhone string
		expectError   bool
	}{
		{
			name: "10 digit phone",
			ctx: TransformContext{
				DB: &DatabaseRecord{
					Phone: "(555) 123-4567",
				},
			},
			expectedPhone: "+1-555-123-4567",
			expectError:   false,
		},
		{
			name: "11 digit phone with 1",
			ctx: TransformContext{
				DB: &DatabaseRecord{
					Phone: "1-555-987-6543",
				},
			},
			expectedPhone: "+1-555-987-6543",
			expectError:   false,
		},
		{
			name: "international phone",
			ctx: TransformContext{
				DB: &DatabaseRecord{
					Phone: "+44 20 7946 0958",
				},
			},
			expectedPhone: "+44 20 7946 0958", // Should remain unchanged
			expectError:   true,
		},
		{
			name: "too few digits",
			ctx: TransformContext{
				DB: &DatabaseRecord{
					Phone: "555-1234",
				},
			},
			expectedPhone: "555-1234",
			expectError:   true,
		},
		{
			name: "nil DB",
			ctx: TransformContext{
				DB: nil,
			},
			expectedPhone: "",
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizePhone(tt.ctx)
			
			if tt.ctx.DB == nil {
				if result.DB != nil {
					t.Error("NormalizePhone() should not create DB")
				}
				return
			}
			
			if result.DB.Phone != tt.expectedPhone {
				t.Errorf("NormalizePhone() phone = %v, want %v", result.DB.Phone, tt.expectedPhone)
			}
			
			if tt.expectError && !containsError(result.Errors, "invalid phone number format") {
				t.Error("NormalizePhone() should add error for invalid phone")
			}
		})
	}
}

func TestEnrichData(t *testing.T) {
	tests := []struct {
		name         string
		ctx          TransformContext
		expectedName string
	}{
		{
			name: "lowercase name",
			ctx: TransformContext{
				DB: &DatabaseRecord{
					Name: "john doe",
				},
			},
			expectedName: "John Doe",
		},
		{
			name: "uppercase name",
			ctx: TransformContext{
				DB: &DatabaseRecord{
					Name: "JANE SMITH",
				},
			},
			expectedName: "Jane Smith",
		},
		{
			name: "mixed case",
			ctx: TransformContext{
				DB: &DatabaseRecord{
					Name: "mArY jOhNsOn",
				},
			},
			expectedName: "Mary Johnson",
		},
		{
			name: "single name",
			ctx: TransformContext{
				DB: &DatabaseRecord{
					Name: "cher",
				},
			},
			expectedName: "Cher",
		},
		{
			name: "nil DB",
			ctx: TransformContext{
				DB: nil,
			},
			expectedName: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := EnrichData(tt.ctx)
			
			if tt.ctx.DB == nil {
				if result.DB != nil {
					t.Error("EnrichData() should not create DB")
				}
				return
			}
			
			if result.DB.Name != tt.expectedName {
				t.Errorf("EnrichData() name = %v, want %v", result.DB.Name, tt.expectedName)
			}
		})
	}
}

func TestValidateTransform(t *testing.T) {
	tests := []struct {
		name    string
		ctx     TransformContext
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid transform",
			ctx: TransformContext{
				DB:     &DatabaseRecord{ID: 1},
				Errors: []string{},
			},
			wantErr: false,
		},
		{
			name: "nil DB",
			ctx: TransformContext{
				DB:     nil,
				Errors: []string{},
			},
			wantErr: true,
			errMsg:  "no database record created",
		},
		{
			name: "with errors",
			ctx: TransformContext{
				DB:     &DatabaseRecord{ID: 1},
				Errors: []string{"error 1", "error 2"},
			},
			wantErr: true,
			errMsg:  "transformation had 2 errors",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTransform(tt.ctx)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateTransform() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && tt.errMsg != "" {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateTransform() error = %v, want error containing %v", err, tt.errMsg)
				}
			}
		})
	}
}

func TestTransformPipeline(t *testing.T) {
	pipeline := CreateTransformPipeline()

	tests := []struct {
		name    string
		ctx     TransformContext
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid CSV",
			ctx: TransformContext{
				CSV: &CSVRecord{
					Fields: []string{"100", "john doe", "john@example.com", "(555) 123-4567"},
				},
			},
			wantErr: false,
		},
		{
			name: "insufficient fields",
			ctx: TransformContext{
				CSV: &CSVRecord{
					Fields: []string{"101", "jane"},
				},
			},
			wantErr: true,
			errMsg:  "insufficient CSV fields",
		},
		{
			name: "invalid email",
			ctx: TransformContext{
				CSV: &CSVRecord{
					Fields: []string{"102", "bob smith", "bobsmith.com", "555-1234567"},
				},
			},
			wantErr: true,
			errMsg:  "transformation had 1 errors",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := pipeline.Process(tt.ctx)
			if (err != nil) != tt.wantErr {
				t.Errorf("Process() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && tt.errMsg != "" {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("Process() error = %v, want error containing %v", err, tt.errMsg)
				}
			}
			if err == nil {
				// Verify transformations were applied
				if result.DB == nil {
					t.Error("Process() should create DB record")
				} else {
					// Check name is title cased
					if result.DB.Name != "John Doe" && tt.name == "valid CSV" {
						t.Errorf("Process() name = %v, want title case", result.DB.Name)
					}
					// Check email is lowercase
					if result.DB.Email != strings.ToLower(result.DB.Email) {
						t.Error("Process() email should be lowercase")
					}
					// Check phone is formatted
					if tt.name == "valid CSV" && result.DB.Phone != "+1-555-123-4567" {
						t.Errorf("Process() phone = %v, want formatted", result.DB.Phone)
					}
				}
			}
		})
	}
}

func TestLenientTransformPipeline(t *testing.T) {
	pipeline := CreateLenientTransformPipeline()

	// Should allow records with errors
	ctx := TransformContext{
		CSV: &CSVRecord{
			Fields: []string{"103", "alice wonderland", "alice.wonderland", "bad-phone"},
		},
	}

	result, err := pipeline.Process(ctx)
	if err != nil {
		t.Errorf("LenientPipeline should not fail, got error: %v", err)
	}

	if result.DB == nil {
		t.Error("LenientPipeline should create DB record")
	}

	if len(result.Errors) == 0 {
		t.Error("LenientPipeline should record errors but continue")
	}

	// Should have errors for email and phone
	hasEmailError := containsError(result.Errors, "invalid email format")
	hasPhoneError := containsError(result.Errors, "invalid phone number format")
	
	if !hasEmailError || !hasPhoneError {
		t.Errorf("Expected errors for email and phone, got: %v", result.Errors)
	}
}

func TestTransformChaining(t *testing.T) {
	// Create parsing pipeline
	parser := pipz.NewContract[TransformContext]()
	parser.Register(pipz.Apply(ParseCSV))

	// Create validation/enrichment pipeline
	enricher := pipz.NewContract[TransformContext]()
	enricher.Register(
		pipz.Transform(ValidateEmail),
		pipz.Transform(NormalizePhone),
		pipz.Transform(EnrichData),
	)

	// Chain them together
	chain := pipz.NewChain[TransformContext]()
	chain.Add(parser, enricher)

	// Test valid data
	ctx := TransformContext{
		CSV: &CSVRecord{
			Fields: []string{"200", "mary jones", "MARY@EXAMPLE.COM", "555-999-8888"},
		},
	}

	result, err := chain.Process(ctx)
	if err != nil {
		t.Errorf("Chain.Process() unexpected error: %v", err)
	}

	if result.DB.Name != "Mary Jones" {
		t.Errorf("Chain should title case name, got: %v", result.DB.Name)
	}
	if result.DB.Email != "mary@example.com" {
		t.Errorf("Chain should lowercase email, got: %v", result.DB.Email)
	}
	if result.DB.Phone != "+1-555-999-8888" {
		t.Errorf("Chain should format phone, got: %v", result.DB.Phone)
	}
}

func TestTransformMutate(t *testing.T) {
	// Pipeline that applies discounts for VIP customers
	pipeline := pipz.NewContract[TransformContext]()
	pipeline.Register(
		pipz.Apply(ParseCSV),
		// Mark VIP customers (ID > 1000)
		pipz.Mutate(
			func(ctx TransformContext) TransformContext {
				if ctx.DB != nil {
					ctx.DB.Name = "VIP " + ctx.DB.Name
				}
				return ctx
			},
			func(ctx TransformContext) bool {
				return ctx.DB != nil && ctx.DB.ID > 1000
			},
		),
		pipz.Transform(EnrichData),
	)

	// Test VIP customer
	vipCtx := TransformContext{
		CSV: &CSVRecord{
			Fields: []string{"2000", "john doe", "john@vip.com", "555-1111"},
		},
	}

	result, err := pipeline.Process(vipCtx)
	if err != nil {
		t.Errorf("Process() unexpected error: %v", err)
	}
	if !strings.HasPrefix(result.DB.Name, "Vip ") { // EnrichData title cases it
		t.Errorf("VIP customer should have VIP prefix, got: %v", result.DB.Name)
	}

	// Test regular customer
	regularCtx := TransformContext{
		CSV: &CSVRecord{
			Fields: []string{"100", "jane doe", "jane@example.com", "555-2222"},
		},
	}

	result, err = pipeline.Process(regularCtx)
	if err != nil {
		t.Errorf("Process() unexpected error: %v", err)
	}
	if strings.HasPrefix(result.DB.Name, "VIP") {
		t.Errorf("Regular customer should not have VIP prefix, got: %v", result.DB.Name)
	}
}

// TestTransformErrorPropagation verifies zero values are returned on error
func TestTransformErrorPropagation(t *testing.T) {
	pipeline := CreateTransformPipeline()
	
	// Context that will fail parsing
	ctx := TransformContext{
		CSV: &CSVRecord{
			Fields: []string{"123"}, // Too few fields
		},
	}
	
	result, err := pipeline.Process(ctx)
	if err == nil {
		t.Error("Expected error for insufficient fields")
	}
	
	// Verify zero value is returned
	if result.CSV != nil || result.DB != nil || len(result.Errors) != 0 {
		t.Error("Expected zero value TransformContext on error")
	}
}

// Helper function
func containsError(errors []string, substr string) bool {
	for _, err := range errors {
		if strings.Contains(err, substr) {
			return true
		}
	}
	return false
}