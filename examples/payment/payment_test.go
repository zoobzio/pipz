package main

import (
	"testing"

	"pipz"
)

func TestValidatePayment(t *testing.T) {
	tests := []struct {
		name    string
		payment Payment
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid payment",
			payment: Payment{
				ID:         "PAY-001",
				Amount:     99.99,
				Currency:   "USD",
				CardNumber: "1234",
			},
			wantErr: false,
		},
		{
			name: "zero amount",
			payment: Payment{
				ID:         "PAY-002",
				Amount:     0,
				Currency:   "USD",
				CardNumber: "1234",
			},
			wantErr: true,
			errMsg:  "invalid amount",
		},
		{
			name: "negative amount",
			payment: Payment{
				ID:         "PAY-003",
				Amount:     -50.00,
				Currency:   "USD",
				CardNumber: "1234",
			},
			wantErr: true,
			errMsg:  "invalid amount",
		},
		{
			name: "missing card",
			payment: Payment{
				ID:       "PAY-004",
				Amount:   100.00,
				Currency: "USD",
			},
			wantErr: true,
			errMsg:  "missing card information",
		},
		{
			name: "missing currency",
			payment: Payment{
				ID:         "PAY-005",
				Amount:     100.00,
				CardNumber: "1234",
			},
			wantErr: true,
			errMsg:  "missing currency",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ValidatePayment(tt.payment)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePayment() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && tt.errMsg != "" {
				if !contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidatePayment() error = %v, want error containing %v", err, tt.errMsg)
				}
			}
			if err != nil && result.ID != "" {
				t.Error("ValidatePayment() should return zero value on error")
			}
		})
	}
}

func TestCheckFraud(t *testing.T) {
	tests := []struct {
		name    string
		payment Payment
		wantErr bool
		errMsg  string
	}{
		{
			name: "normal transaction",
			payment: Payment{
				Amount:   500.00,
				Attempts: 1,
			},
			wantErr: false,
		},
		{
			name: "large first-time transaction",
			payment: Payment{
				Amount:   15000.00,
				Attempts: 0,
			},
			wantErr: true,
			errMsg:  "FRAUD: large first-time transaction",
		},
		{
			name: "exceeds card limit",
			payment: Payment{
				Amount:    5000.00,
				CardLimit: 3000.00,
				Attempts:  1,
			},
			wantErr: true,
			errMsg:  "FRAUD: amount exceeds card limit",
		},
		{
			name: "within card limit",
			payment: Payment{
				Amount:    2000.00,
				CardLimit: 3000.00,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := CheckFraud(tt.payment)
			if (err != nil) != tt.wantErr {
				t.Errorf("CheckFraud() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && tt.errMsg != "" {
				if !contains(err.Error(), tt.errMsg) {
					t.Errorf("CheckFraud() error = %v, want error containing %v", err, tt.errMsg)
				}
			}
			if err != nil && result.ID != "" {
				t.Error("CheckFraud() should return zero value on error")
			}
		})
	}
}

func TestUpdatePaymentStatus(t *testing.T) {
	tests := []struct {
		name           string
		payment        Payment
		expectedStatus string
		expectedAttempts int
	}{
		{
			name: "new payment",
			payment: Payment{
				ID:       "PAY-001",
				Attempts: 0,
			},
			expectedStatus:   "pending",
			expectedAttempts: 1,
		},
		{
			name: "existing payment",
			payment: Payment{
				ID:       "PAY-002",
				Status:   "processing",
				Attempts: 2,
			},
			expectedStatus:   "processing",
			expectedAttempts: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := UpdatePaymentStatus(tt.payment)
			if result.Status != tt.expectedStatus {
				t.Errorf("UpdatePaymentStatus() status = %v, want %v", result.Status, tt.expectedStatus)
			}
			if result.Attempts != tt.expectedAttempts {
				t.Errorf("UpdatePaymentStatus() attempts = %v, want %v", result.Attempts, tt.expectedAttempts)
			}
		})
	}
}

func TestProcessWithPrimary(t *testing.T) {
	tests := []struct {
		name    string
		payment Payment
		wantErr bool
		errMsg  string
	}{
		{
			name: "small amount",
			payment: Payment{
				ID:     "PAY-001",
				Amount: 100.00,
			},
			wantErr: false,
		},
		{
			name: "large amount",
			payment: Payment{
				ID:     "PAY-002",
				Amount: 6000.00,
			},
			wantErr: true,
			errMsg:  "primary provider: amount too large",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ProcessWithPrimary(tt.payment)
			if (err != nil) != tt.wantErr {
				t.Errorf("ProcessWithPrimary() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil {
				if result.Status != "completed" {
					t.Error("ProcessWithPrimary() should set status to completed")
				}
				if result.Provider != "primary" {
					t.Error("ProcessWithPrimary() should set provider to primary")
				}
				if result.ProcessedAt.IsZero() {
					t.Error("ProcessWithPrimary() should set ProcessedAt")
				}
			}
		})
	}
}

func TestPaymentPipeline(t *testing.T) {
	pipeline := CreatePaymentPipeline()

	tests := []struct {
		name    string
		payment Payment
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid small payment",
			payment: Payment{
				ID:            "PAY-001",
				Amount:        99.99,
				Currency:      "USD",
				CardNumber:    "1234",
				CardLimit:     5000,
				CustomerEmail: "customer@example.com",
			},
			wantErr: false,
		},
		{
			name: "invalid amount",
			payment: Payment{
				ID:         "PAY-002",
				Amount:     -50.00,
				Currency:   "USD",
				CardNumber: "1234",
			},
			wantErr: true,
			errMsg:  "invalid amount",
		},
		{
			name: "fraud detection",
			payment: Payment{
				ID:         "PAY-003",
				Amount:     15000.00,
				Currency:   "USD",
				CardNumber: "9999",
				Attempts:   0,
			},
			wantErr: true,
			errMsg:  "FRAUD",
		},
		{
			name: "primary provider limit",
			payment: Payment{
				ID:         "PAY-004",
				Amount:     6000.00,
				Currency:   "USD",
				CardNumber: "5678",
				CardLimit:  10000,
			},
			wantErr: true,
			errMsg:  "primary provider",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := pipeline.Process(tt.payment)
			if (err != nil) != tt.wantErr {
				t.Errorf("Process() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && tt.errMsg != "" {
				if !contains(err.Error(), tt.errMsg) {
					t.Errorf("Process() error = %v, want error containing %v", err, tt.errMsg)
				}
			}
			if err == nil {
				if result.Status != "completed" {
					t.Error("Process() should set status to completed for successful payment")
				}
				if result.Attempts != 1 {
					t.Error("Process() should increment attempts")
				}
			}
		})
	}
}

func TestPaymentChaining(t *testing.T) {
	// Create validation pipeline
	validator := pipz.NewContract[Payment]()
	validator.Register(
		pipz.Apply(ValidatePayment),
		pipz.Apply(CheckFraud),
	)

	// Create processing pipeline
	processor := pipz.NewContract[Payment]()
	processor.Register(
		pipz.Transform(UpdatePaymentStatus),
		pipz.Apply(ProcessWithPrimary),
	)

	// Chain them together
	chain := pipz.NewChain[Payment]()
	chain.Add(validator, processor)

	// Test valid payment
	validPayment := Payment{
		ID:         "PAY-001",
		Amount:     500.00,
		Currency:   "USD",
		CardNumber: "1234",
		CardLimit:  5000,
	}

	result, err := chain.Process(validPayment)
	if err != nil {
		t.Errorf("Chain.Process() unexpected error: %v", err)
	}
	if result.Status != "completed" {
		t.Error("Chain.Process() should complete valid payment")
	}

	// Test invalid payment (should fail in validation)
	invalidPayment := Payment{
		ID:     "PAY-002",
		Amount: -100.00,
	}

	_, err = chain.Process(invalidPayment)
	if err == nil {
		t.Error("Chain.Process() expected error for invalid payment")
	}
}

func TestPaymentMutate(t *testing.T) {
	// Create a pipeline that applies discounts
	pipeline := pipz.NewContract[Payment]()
	pipeline.Register(
		pipz.Apply(ValidatePayment),
		// Apply 10% discount for amounts over $1000
		pipz.Mutate(
			func(p Payment) Payment {
				p.Amount = p.Amount * 0.9
				return p
			},
			func(p Payment) bool {
				return p.Amount > 1000
			},
		),
		pipz.Apply(ProcessWithPrimary),
	)

	// Test payment with discount
	payment := Payment{
		ID:         "PAY-001",
		Amount:     2000.00,
		Currency:   "USD",
		CardNumber: "1234",
	}

	result, err := pipeline.Process(payment)
	if err != nil {
		t.Errorf("Process() unexpected error: %v", err)
	}
	// Original amount was 2000, with 10% discount should be 1800
	expectedAmount := 1800.00
	if result.Amount != expectedAmount {
		t.Errorf("Process() amount = %v, want %v", result.Amount, expectedAmount)
	}
}

// TestPaymentErrorPropagation verifies zero values are returned on error
func TestPaymentErrorPropagation(t *testing.T) {
	pipeline := CreatePaymentPipeline()
	
	// Payment that will fail validation
	payment := Payment{
		ID:     "PAY-001",
		Amount: -100.00, // Invalid amount
	}
	
	result, err := pipeline.Process(payment)
	if err == nil {
		t.Error("Expected error for invalid payment")
	}
	
	// Verify zero value is returned
	if result != (Payment{}) {
		t.Error("Expected zero value Payment on error")
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && len(substr) > 0 && findSubstring(s, substr) != -1)
}

func findSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}