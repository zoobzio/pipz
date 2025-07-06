package benchmarks

import (
	"fmt"
	"testing"
	"time"
	
	"pipz"
	"pipz/examples"
)

// BenchmarkPaymentPipeline tests the performance of a payment processing pipeline
func BenchmarkPaymentPipeline(b *testing.B) {
	// Setup
	const paymentKey examples.PaymentKey = "bench-payment"
	contract := pipz.GetContract[examples.Payment](paymentKey)
	contract.Register(
		pipz.Apply(examples.ValidatePayment),
		pipz.Apply(examples.CheckFraud),
		pipz.Apply(examples.UpdatePaymentStatus),
	)
	
	// Test data - typical payment
	payment := examples.Payment{
		ID:            "PAY-123456",
		Amount:        99.99,
		Currency:      "USD",
		CardNumber:    "1111",
		CardLimit:     5000.00,
		CustomerEmail: "customer@example.com",
		CustomerPhone: "555-0123",
		Status:        "initiated",
		Attempts:      0,
	}
	
	b.ResetTimer()
	
	// Benchmark
	for i := 0; i < b.N; i++ {
		_, err := contract.Process(payment)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkPaymentPipelineWithFraud tests performance with fraud detection triggered
func BenchmarkPaymentPipelineWithFraud(b *testing.B) {
	// Setup
	const paymentKey examples.PaymentKey = "bench-payment-fraud"
	contract := pipz.GetContract[examples.Payment](paymentKey)
	contract.Register(
		pipz.Apply(examples.ValidatePayment),
		pipz.Apply(examples.CheckFraud),
		pipz.Apply(examples.UpdatePaymentStatus),
	)
	
	// Test data - high amount that triggers fraud check
	payment := examples.Payment{
		ID:            "PAY-999999",
		Amount:        15000.00, // Triggers fraud detection
		Currency:      "USD",
		CardNumber:    "1111",
		CardLimit:     20000.00,
		CustomerEmail: "newcustomer@example.com",
		CustomerPhone: "555-9999",
		Status:        "initiated",
		Attempts:      0, // First transaction triggers fraud check
	}
	
	b.ResetTimer()
	
	// Benchmark
	for i := 0; i < b.N; i++ {
		_, err := contract.Process(payment)
		if err != nil {
			// Fraud detection returns error - this is expected
			continue
		}
	}
}

// BenchmarkPaymentErrorPipeline tests the error handling pipeline performance
func BenchmarkPaymentErrorPipeline(b *testing.B) {
	// Setup
	const errorKey examples.PaymentErrorKey = "bench-error"
	contract := pipz.GetContract[examples.PaymentError](errorKey)
	contract.Register(
		pipz.Apply(func(pe examples.PaymentError) (examples.PaymentError, error) {
			// Categorize error
			pe.ErrorType = "payment_failed"
			pe.ErrorCode = "PF001"
			return pe, nil
		}),
		pipz.Apply(func(pe examples.PaymentError) (examples.PaymentError, error) {
			// Determine recovery
			pe.RecoveryAttempted = true
			pe.AlertLevel = "ops"
			return pe, nil
		}),
	)
	
	// Test data
	paymentError := examples.PaymentError{
		Payment: examples.Payment{
			ID:     "PAY-ERROR-001",
			Amount: 50.00,
		},
		OriginalError: fmt.Errorf("insufficient funds"),
		Timestamp:     time.Now(),
	}
	
	b.ResetTimer()
	
	// Benchmark
	for i := 0; i < b.N; i++ {
		_, err := contract.Process(paymentError)
		if err != nil {
			b.Fatal(err)
		}
	}
}