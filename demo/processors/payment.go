package processors

import (
	"fmt"
	"time"
)

// PaymentKey is the contract key type for payment processing pipelines.
type PaymentKey string

// PaymentErrorKey is the contract key type for payment error handling pipelines.
type PaymentErrorKey string

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

// PaymentError represents a payment processing error with recovery options.
type PaymentError struct {
	Payment       Payment
	OriginalError error
	ErrorType     string // insufficient_funds, fraud, network, invalid_card
	ErrorCode     string
	Timestamp     time.Time
	
	// Error handling results
	RecoveryAttempted bool
	RecoverySuccess   bool
	NotificationSent  bool
	AlertLevel        string // none, ops, critical
	AuditLogged       bool
	FinalError        error
}

// ValidatePayment ensures the payment amount is valid.
// This is a basic validation that would be expanded in production.
func ValidatePayment(p Payment) (Payment, error) {
	if p.Amount <= 0 {
		return p, fmt.Errorf("invalid amount: %.2f", p.Amount)
	}
	if p.CardNumber == "" {
		return p, fmt.Errorf("missing card information")
	}
	return p, nil
}

// CheckFraud performs basic fraud detection.
// In production, this would use ML models and historical data.
func CheckFraud(p Payment) (Payment, error) {
	// Simulate fraud check for large first-time transactions
	if p.Amount > 10000 && p.Attempts == 0 {
		return p, fmt.Errorf("FRAUD: large first-time transaction")
	}
	return p, nil
}

// UpdatePaymentStatus increments the attempt counter and sets the status.
func UpdatePaymentStatus(p Payment) (Payment, error) {
	p.Attempts++
	if p.Status == "" {
		p.Status = "pending"
	}
	return p, nil
}