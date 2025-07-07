package main

import (
	"fmt"
	"log"
	"time"

	"pipz"
)

// Payment represents a financial transaction.
type Payment struct {
	ID             string
	Amount         float64
	Currency       string
	CardNumber     string // Last 4 digits only
	CardLimit      float64
	CustomerEmail  string
	CustomerPhone  string
	Provider       string // primary, backup, tertiary
	Attempts       int
	Status         string
	ProcessedAt    time.Time
}

// ValidatePayment ensures the payment amount is valid.
// This is a basic validation that would be expanded in production.
func ValidatePayment(p Payment) (Payment, error) {
	if p.Amount <= 0 {
		return Payment{}, fmt.Errorf("invalid amount: %.2f", p.Amount)
	}
	if p.CardNumber == "" {
		return Payment{}, fmt.Errorf("missing card information")
	}
	if p.Currency == "" {
		return Payment{}, fmt.Errorf("missing currency")
	}
	return p, nil
}

// CheckFraud performs basic fraud detection.
// In production, this would use ML models and historical data.
func CheckFraud(p Payment) (Payment, error) {
	// Simulate fraud check for large first-time transactions
	if p.Amount > 10000 && p.Attempts == 0 {
		return Payment{}, fmt.Errorf("FRAUD: large first-time transaction")
	}
	// Check if amount exceeds card limit
	if p.CardLimit > 0 && p.Amount > p.CardLimit {
		return Payment{}, fmt.Errorf("FRAUD: amount exceeds card limit")
	}
	return p, nil
}

// UpdatePaymentStatus increments the attempt counter and sets the status.
func UpdatePaymentStatus(p Payment) Payment {
	p.Attempts++
	if p.Status == "" {
		p.Status = "pending"
	}
	return p
}

// ProcessWithPrimary simulates processing with the primary payment provider.
func ProcessWithPrimary(p Payment) (Payment, error) {
	if p.Provider == "" {
		p.Provider = "primary"
	}
	
	// Simulate processing
	if p.Amount > 5000 {
		// Simulate primary provider failure for large amounts
		return Payment{}, fmt.Errorf("primary provider: amount too large")
	}
	
	p.Status = "completed"
	p.ProcessedAt = time.Now()
	return p, nil
}

// ProcessWithBackup simulates processing with the backup payment provider.
func ProcessWithBackup(p Payment) (Payment, error) {
	p.Provider = "backup"
	
	// Simulate processing - backup handles larger amounts
	if p.Amount > 15000 {
		return Payment{}, fmt.Errorf("backup provider: amount exceeds limit")
	}
	
	p.Status = "completed"
	p.ProcessedAt = time.Now()
	return p, nil
}

// CreatePaymentPipeline creates a standard payment processing pipeline
func CreatePaymentPipeline() *pipz.Contract[Payment] {
	pipeline := pipz.NewContract[Payment]()
	pipeline.Register(
		pipz.Apply(ValidatePayment),
		pipz.Apply(CheckFraud),
		pipz.Transform(UpdatePaymentStatus),
		pipz.Apply(ProcessWithPrimary),
	)
	return pipeline
}

// CreatePaymentWithFallbackPipeline creates a payment pipeline with fallback logic
func CreatePaymentWithFallbackPipeline() *pipz.Contract[Payment] {
	// Primary pipeline
	primary := pipz.NewContract[Payment]()
	primary.Register(
		pipz.Apply(ValidatePayment),
		pipz.Apply(CheckFraud),
		pipz.Transform(UpdatePaymentStatus),
		pipz.Apply(ProcessWithPrimary),
	)
	
	// Backup pipeline (only runs if primary fails)
	backup := pipz.NewContract[Payment]()
	backup.Register(
		pipz.Transform(func(p Payment) Payment {
			p.Attempts++
			return p
		}),
		pipz.Apply(ProcessWithBackup),
	)
	
	// Note: In practice, you'd implement fallback logic at the application level
	// since pipz returns zero values on error
	return primary
}

func main() {
	// Create the payment pipeline
	processor := CreatePaymentPipeline()
	
	// Valid payment
	payment1 := Payment{
		ID:            "PAY-001",
		Amount:        99.99,
		Currency:      "USD",
		CardNumber:    "1234",
		CardLimit:     5000,
		CustomerEmail: "customer@example.com",
		CustomerPhone: "+1234567890",
	}
	
	fmt.Println("Processing valid payment...")
	result, err := processor.Process(payment1)
	if err != nil {
		log.Printf("Payment failed: %v", err)
	} else {
		fmt.Printf("✓ Payment %s processed successfully by %s provider\n", result.ID, result.Provider)
	}
	
	// Large payment (will fail with primary)
	payment2 := Payment{
		ID:            "PAY-002",
		Amount:        7500.00,
		Currency:      "USD",
		CardNumber:    "5678",
		CardLimit:     10000,
		CustomerEmail: "bigspender@example.com",
	}
	
	fmt.Println("\nProcessing large payment...")
	result, err = processor.Process(payment2)
	if err != nil {
		fmt.Printf("✗ Primary processing failed: %v\n", err)
		
		// Try backup processor
		backup := pipz.NewContract[Payment]()
		backup.Register(
			pipz.Transform(func(p Payment) Payment {
				p.Attempts++
				return p
			}),
			pipz.Apply(ProcessWithBackup),
		)
		
		result, err = backup.Process(payment2)
		if err != nil {
			fmt.Printf("✗ Backup processing also failed: %v\n", err)
		} else {
			fmt.Printf("✓ Payment %s processed successfully by %s provider\n", result.ID, result.Provider)
		}
	}
	
	// Fraudulent payment
	payment3 := Payment{
		ID:            "PAY-003",
		Amount:        15000.00,
		Currency:      "USD",
		CardNumber:    "9999",
		CardLimit:     5000,
		CustomerEmail: "suspicious@example.com",
		Attempts:      0,
	}
	
	fmt.Println("\nProcessing suspicious payment...")
	_, err = processor.Process(payment3)
	if err != nil {
		fmt.Printf("✗ Payment blocked: %v\n", err)
	}
}