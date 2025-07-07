package main

import (
	"fmt"
	"time"

	"pipz"
)

// Payment types for demo
type Payment struct {
	ID            string
	Amount        float64
	Currency      string
	CardNumber    string // Last 4 digits only
	Provider      string
	Status        string
	ProcessedAt   time.Time
	RetryCount    int
}

// Payment validation
func validatePayment(p Payment) (Payment, error) {
	if p.Amount <= 0 {
		return Payment{}, fmt.Errorf("invalid amount: %.2f", p.Amount)
	}
	if p.Currency == "" {
		return Payment{}, fmt.Errorf("currency required")
	}
	if p.CardNumber == "" {
		return Payment{}, fmt.Errorf("card information required")
	}
	return p, nil
}

// Simple fraud check
func checkPaymentFraud(p Payment) (Payment, error) {
	if p.Amount > 10000 {
		return Payment{}, fmt.Errorf("amount exceeds fraud threshold")
	}
	return p, nil
}

// Process with primary provider
func processPrimary(p Payment) (Payment, error) {
	// Simulate primary provider limits
	if p.Amount > 5000 {
		return Payment{}, fmt.Errorf("primary provider: amount too large")
	}
	
	p.Provider = "primary"
	p.Status = "completed"
	p.ProcessedAt = time.Now()
	return p, nil
}

// Process with backup provider
func processBackup(p Payment) (Payment, error) {
	// Backup handles larger amounts
	if p.Amount > 15000 {
		return Payment{}, fmt.Errorf("backup provider: amount exceeds limit")
	}
	
	p.Provider = "backup"
	p.Status = "completed"
	p.ProcessedAt = time.Now()
	return p, nil
}

func runPaymentDemo() {
	section("PAYMENT PROCESSING PIPELINE")
	
	info("Use Case: Payment Processing with Fallback")
	info("• Validate payment details")
	info("• Check for fraud")
	info("• Process with primary provider")
	info("• Fallback to backup provider on failure")

	// Create primary pipeline
	primaryPipeline := pipz.NewContract[Payment]()
	primaryPipeline.Register(
		pipz.Apply(validatePayment),
		pipz.Apply(checkPaymentFraud),
		pipz.Apply(processPrimary),
	)

	// Create backup pipeline
	backupPipeline := pipz.NewContract[Payment]()
	backupPipeline.Register(
		pipz.Transform(func(p Payment) Payment {
			p.RetryCount++
			return p
		}),
		pipz.Apply(processBackup),
	)

	code("go", `// Primary pipeline
primary := pipz.NewContract[Payment]()
primary.Register(
    pipz.Apply(validatePayment),
    pipz.Apply(checkPaymentFraud),
    pipz.Apply(processPrimary),
)

// Backup pipeline
backup := pipz.NewContract[Payment]()
backup.Register(
    pipz.Transform(incrementRetry),
    pipz.Apply(processBackup),
)`)

	// Test Case 1: Small payment (primary succeeds)
	fmt.Println("\n💳 Test Case 1: Small Payment")
	payment1 := Payment{
		ID:         "PAY-001",
		Amount:     100.00,
		Currency:   "USD",
		CardNumber: "1234",
	}

	result, err := primaryPipeline.Process(payment1)
	if err != nil {
		showError(fmt.Sprintf("Payment failed: %v", err))
	} else {
		success(fmt.Sprintf("Payment processed by %s provider", result.Provider))
	}

	// Test Case 2: Large payment (primary fails, backup succeeds)
	fmt.Println("\n💳 Test Case 2: Large Payment with Fallback")
	payment2 := Payment{
		ID:         "PAY-002",
		Amount:     7500.00,
		Currency:   "USD",
		CardNumber: "5678",
	}

	result, err = primaryPipeline.Process(payment2)
	if err != nil {
		fmt.Printf("⚠️  Primary failed: %v\n", err)
		
		// Try backup
		result, err = backupPipeline.Process(payment2)
		if err != nil {
			showError(fmt.Sprintf("Backup also failed: %v", err))
		} else {
			success(fmt.Sprintf("Payment processed by %s provider (retry %d)", 
				result.Provider, result.RetryCount))
		}
	}

	// Test Case 3: Fraud detection
	fmt.Println("\n💳 Test Case 3: Fraud Detection")
	payment3 := Payment{
		ID:         "PAY-003",
		Amount:     12000.00,
		Currency:   "USD",
		CardNumber: "9999",
	}

	_, err = primaryPipeline.Process(payment3)
	if err != nil {
		showError(fmt.Sprintf("Payment blocked: %v", err))
	}

	fmt.Println("\n🔍 Fallback Pattern:")
	info("• Primary pipeline handles normal cases")
	info("• Backup pipeline for failover scenarios")
	info("• Application-level retry logic")
	info("• Each provider has different limits")
}