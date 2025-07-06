package examples_test

import (
	"strings"
	"testing"

	"pipz"
	"pipz/examples"
)

func TestPaymentValidation(t *testing.T) {
	tests := []struct {
		name    string
		payment examples.Payment
		wantErr bool
		errMsg  string
	}{
		{
			name: "ValidPayment",
			payment: examples.Payment{
				ID:         "PAY-001",
				Amount:     100.00,
				CardNumber: "****4242",
			},
			wantErr: false,
		},
		{
			name: "ZeroAmount",
			payment: examples.Payment{
				ID:         "PAY-002",
				Amount:     0,
				CardNumber: "****4242",
			},
			wantErr: true,
			errMsg:  "invalid amount",
		},
		{
			name: "NegativeAmount",
			payment: examples.Payment{
				ID:         "PAY-003",
				Amount:     -50.00,
				CardNumber: "****4242",
			},
			wantErr: true,
			errMsg:  "invalid amount",
		},
		{
			name: "MissingCard",
			payment: examples.Payment{
				ID:     "PAY-004",
				Amount: 100.00,
			},
			wantErr: true,
			errMsg:  "missing card",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := examples.ValidatePayment(tt.payment)
			
			if tt.wantErr {
				if err == nil {
					t.Fatal("Expected error but got none")
				}
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("Error message %q doesn't contain %q", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}
				if result.ID != tt.payment.ID {
					t.Error("Payment ID changed during validation")
				}
			}
		})
	}
}

func TestFraudDetection(t *testing.T) {
	t.Run("LargeFirstTransaction", func(t *testing.T) {
		payment := examples.Payment{
			ID:         "PAY-FRAUD",
			Amount:     15000.00,
			CardNumber: "****9999",
			Attempts:   0, // First time
		}

		_, err := examples.CheckFraud(payment)
		if err == nil {
			t.Fatal("Expected fraud detection for large first transaction")
		}

		if !strings.Contains(err.Error(), "FRAUD") {
			t.Errorf("Wrong error type: %v", err)
		}
	})

	t.Run("LargeRepeatTransaction", func(t *testing.T) {
		payment := examples.Payment{
			ID:         "PAY-OK",
			Amount:     15000.00,
			CardNumber: "****9999",
			Attempts:   5, // Existing customer
		}

		_, err := examples.CheckFraud(payment)
		if err != nil {
			t.Fatalf("Repeat customer flagged for fraud: %v", err)
		}
	})

	t.Run("SmallTransaction", func(t *testing.T) {
		payment := examples.Payment{
			ID:         "PAY-SMALL",
			Amount:     50.00,
			CardNumber: "****1111",
			Attempts:   0,
		}

		_, err := examples.CheckFraud(payment)
		if err != nil {
			t.Fatalf("Small transaction flagged: %v", err)
		}
	})
}

func TestPaymentPipeline(t *testing.T) {
	// Create a simple payment processing pipeline
	const testKey examples.PaymentKey = "test"
	contract := pipz.GetContract[examples.PaymentKey, examples.Payment](testKey)
	
	err := contract.Register(
		pipz.Apply(examples.ValidatePayment),
		pipz.Apply(examples.CheckFraud),
		pipz.Apply(examples.UpdatePaymentStatus),
	)
	if err != nil {
		t.Fatalf("Failed to register payment pipeline: %v", err)
	}

	t.Run("SuccessfulPayment", func(t *testing.T) {
		payment := examples.Payment{
			ID:         "PAY-SUCCESS",
			Amount:     250.00,
			CardNumber: "****4242",
			Attempts:   1,
		}

		result, err := contract.Process(payment)
		if err != nil {
			t.Fatalf("Payment processing failed: %v", err)
		}

		// Verify status was updated
		if result.Attempts != 2 {
			t.Errorf("Attempts not incremented: %d", result.Attempts)
		}

		if result.Status != "pending" {
			t.Errorf("Wrong status: %s", result.Status)
		}
	})

	t.Run("FailedValidation", func(t *testing.T) {
		payment := examples.Payment{
			ID:     "PAY-FAIL",
			Amount: -100.00, // Invalid
		}

		_, err := contract.Process(payment)
		if err == nil {
			t.Fatal("Expected validation failure")
		}
	})
}