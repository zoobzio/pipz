package benchmarks

import (
	"fmt"
	"testing"
	"time"
	
	"pipz"
)

// Payment types
type Payment struct {
	ID            string
	Amount        float64
	Currency      string
	CardNumber    string
	Provider      string
	Status        string
	ProcessedAt   time.Time
	RetryCount    int
}

// Payment processors
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

func checkPaymentFraud(p Payment) (Payment, error) {
	if p.Amount > 10000 {
		return Payment{}, fmt.Errorf("amount exceeds fraud threshold")
	}
	return p, nil
}

func processPrimary(p Payment) (Payment, error) {
	if p.Amount > 5000 {
		return Payment{}, fmt.Errorf("primary provider: amount too large")
	}
	
	p.Provider = "primary"
	p.Status = "completed"
	p.ProcessedAt = time.Now()
	return p, nil
}

func processBackup(p Payment) (Payment, error) {
	if p.Amount > 15000 {
		return Payment{}, fmt.Errorf("backup provider: amount exceeds limit")
	}
	
	p.Provider = "backup"
	p.Status = "completed"
	p.ProcessedAt = time.Now()
	return p, nil
}

func BenchmarkPaymentPipeline(b *testing.B) {
	// Setup pipeline
	pipeline := pipz.NewContract[Payment]()
	pipeline.Register(
		pipz.Apply(validatePayment),
		pipz.Apply(checkPaymentFraud),
		pipz.Apply(processPrimary),
	)

	payment := Payment{
		ID:         "PAY-BENCH",
		Amount:     100.00,
		Currency:   "USD",
		CardNumber: "1234",
	}

	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		_, err := pipeline.Process(payment)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPaymentWithFallback(b *testing.B) {
	// Primary pipeline
	primary := pipz.NewContract[Payment]()
	primary.Register(
		pipz.Apply(validatePayment),
		pipz.Apply(checkPaymentFraud),
		pipz.Apply(processPrimary),
	)
	
	// Backup pipeline
	backup := pipz.NewContract[Payment]()
	backup.Register(
		pipz.Transform(func(p Payment) Payment {
			p.RetryCount++
			return p
		}),
		pipz.Apply(processBackup),
	)

	// Payment that fails primary
	payment := Payment{
		ID:         "PAY-BENCH",
		Amount:     7500.00,
		Currency:   "USD",
		CardNumber: "5678",
	}

	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		_, err := primary.Process(payment)
		if err != nil {
			// Fallback to backup
			_, err = backup.Process(payment)
			if err != nil {
				b.Fatal(err)
			}
		}
	}
}

func BenchmarkPaymentValidation(b *testing.B) {
	payment := Payment{
		ID:         "PAY-BENCH",
		Amount:     100.00,
		Currency:   "USD",
		CardNumber: "1234",
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := validatePayment(payment)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkFraudCheck(b *testing.B) {
	payment := Payment{
		Amount: 5000.00,
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := checkPaymentFraud(payment)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPaymentChain(b *testing.B) {
	// Create validation pipeline
	validator := pipz.NewContract[Payment]()
	validator.Register(
		pipz.Apply(validatePayment),
		pipz.Apply(checkPaymentFraud),
	)

	// Create processing pipeline
	processor := pipz.NewContract[Payment]()
	processor.Register(
		pipz.Apply(processPrimary),
	)

	// Chain them
	chain := pipz.NewChain[Payment]()
	chain.Add(validator, processor)

	payment := Payment{
		ID:         "PAY-BENCH",
		Amount:     1000.00,
		Currency:   "USD",
		CardNumber: "9999",
	}

	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		_, err := chain.Process(payment)
		if err != nil {
			b.Fatal(err)
		}
	}
}