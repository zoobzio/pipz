// Package payment demonstrates how to use pipz for payment error recovery
// with error categorization, customer notification, and retry strategies.
package payment

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/zoobzio/pipz"
)

// Payment represents a payment transaction
type Payment struct {
	ID            string
	CustomerID    string
	CustomerEmail string
	Amount        float64
	Currency      string
	PaymentMethod PaymentMethod
	Description   string
	Timestamp     time.Time
	Provider      string
	RetryCount    int
	MaxRetries    int
	Metadata      map[string]interface{}
}

// PaymentMethod represents different payment methods
type PaymentMethod struct {
	Type       string // "card", "bank", "wallet"
	Last4      string
	ExpiryDate string
	Brand      string // Visa, Mastercard, etc.
}

// PaymentError represents a payment error with recovery context
type PaymentError struct {
	Payment         Payment
	OriginalError   error
	ErrorType       string
	ErrorCode       string
	Recoverable     bool
	RecoveryAction  string
	RetryAfter      time.Duration
	CustomerMessage string
	InternalMessage string
	Timestamp       time.Time
}

// PaymentResult represents the outcome of a payment attempt
type PaymentResult struct {
	Success        bool
	TransactionID  string
	Provider       string
	ProcessingTime time.Duration
	Fees           float64
}

// NotificationResult represents the outcome of a customer notification
type NotificationResult struct {
	Sent      bool
	Channel   string // email, sms, push
	MessageID string
	Error     error
}

// PaymentProvider interface for different payment Providers
type PaymentProvider interface {
	Name() string
	ProcessPayment(ctx context.Context, payment Payment) (*PaymentResult, error)
	IsAvailable() bool
	GetFees(amount float64) float64
}

// Simple mock payment Providers
type StripeProvider struct {
	failureRate float64
	mu          sync.Mutex
}

func (s *StripeProvider) Name() string { return "stripe" }

func (s *StripeProvider) ProcessPayment(ctx context.Context, payment Payment) (*PaymentResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Simulate processing time
	time.Sleep(100 * time.Millisecond)

	// Simulate various error scenarios
	if rand.Float64() < s.failureRate {
		errorTypes := []string{
			"card_declined:insufficient_funds",
			"card_declined:expired_card",
			"network_error:timeout",
			"rate_limit_error",
			"invalid_card_number",
		}
		errorType := errorTypes[rand.Intn(len(errorTypes))]
		return nil, fmt.Errorf("%s", errorType)
	}

	return &PaymentResult{
		Success:        true,
		TransactionID:  fmt.Sprintf("stripe_%s_%d", payment.ID, time.Now().Unix()),
		Provider:       "stripe",
		ProcessingTime: 100 * time.Millisecond,
		Fees:           payment.Amount * 0.029, // 2.9% fee
	}, nil
}

func (s *StripeProvider) IsAvailable() bool              { return true }
func (s *StripeProvider) GetFees(amount float64) float64 { return amount * 0.029 }

type PayPalProvider struct {
	failureRate float64
	available   bool
	mu          sync.Mutex
}

func (p *PayPalProvider) Name() string { return "paypal" }

func (p *PayPalProvider) ProcessPayment(ctx context.Context, payment Payment) (*PaymentResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.available {
		return nil, errors.New("provider_unavailable")
	}

	// Simulate processing time
	time.Sleep(150 * time.Millisecond)

	// Simulate failures
	if rand.Float64() < p.failureRate {
		return nil, errors.New("network_error:connection_reset")
	}

	return &PaymentResult{
		Success:        true,
		TransactionID:  fmt.Sprintf("paypal_%s_%d", payment.ID, time.Now().Unix()),
		Provider:       "paypal",
		ProcessingTime: 150 * time.Millisecond,
		Fees:           payment.Amount * 0.034, // 3.4% fee
	}, nil
}

func (p *PayPalProvider) IsAvailable() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.available
}

func (p *PayPalProvider) GetFees(amount float64) float64 { return amount * 0.034 }

// Global state for demonstration
var (
	Providers = map[string]PaymentProvider{
		"stripe": &StripeProvider{failureRate: 0.1},
		"paypal": &PayPalProvider{failureRate: 0.05, available: true},
	}

	EmailQueue       = make(chan EmailMessage, 100)
	AuditLog         = make([]AuditEntry, 0)
	auditMutex       sync.Mutex
	ReviewQueue      = make(chan Payment, 50)
	ProviderStatsMap = make(map[string]*ProviderStats)
	statsMutex       sync.Mutex
)

// EmailMessage represents an email to be sent
type EmailMessage struct {
	To       string
	Subject  string
	Body     string
	Template string
	Data     map[string]interface{}
}

// AuditEntry represents an audit log entry
type AuditEntry struct {
	Timestamp      time.Time
	PaymentID      string
	CustomerID     string
	Amount         float64
	Provider       string
	ErrorType      string
	RecoveryAction string
	Success        bool
}

// ProviderStats tracks provider performance
type ProviderStats struct {
	TotalAttempts   int
	SuccessCount    int
	FailureCount    int
	TotalFees       float64
	AverageTime     time.Duration
	LastFailureTime time.Time
}

// CategorizeError analyzes the error and determines recovery strategy
func CategorizeError(_ context.Context, pe PaymentError) (PaymentError, error) {
	errMsg := pe.OriginalError.Error()

	// Parse error type and code
	parts := strings.Split(errMsg, ":")
	if len(parts) >= 1 {
		pe.ErrorType = parts[0]
		if len(parts) >= 2 {
			pe.ErrorCode = parts[1]
		}
	}

	// Categorize and set recovery strategy
	switch pe.ErrorType {
	case "card_declined":
		pe.Recoverable = false
		switch pe.ErrorCode {
		case "insufficient_funds":
			pe.CustomerMessage = "Your payment was declined due to insufficient funds. Please use a different payment method or contact your bank."
			pe.InternalMessage = "Card declined: NSF"
		case "expired_card":
			pe.CustomerMessage = "Your card has expired. Please update your payment information."
			pe.InternalMessage = "Card declined: Expired"
		case "stolen_card":
			pe.CustomerMessage = "Your payment could not be processed. Please contact your card issuer."
			pe.InternalMessage = "Card declined: Stolen card reported"
			pe.RecoveryAction = "block_customer"
		default:
			pe.CustomerMessage = "Your payment was declined. Please try a different payment method."
			pe.InternalMessage = fmt.Sprintf("Card declined: %s", pe.ErrorCode)
		}

	case "network_error":
		pe.Recoverable = true
		pe.RecoveryAction = "retry_with_backoff"
		pe.RetryAfter = time.Second * time.Duration(pe.Payment.RetryCount+1)
		pe.CustomerMessage = "We're experiencing a temporary issue. Your payment will be retried shortly."
		pe.InternalMessage = fmt.Sprintf("Network error: %s", pe.ErrorCode)

	case "rate_limit_error":
		pe.Recoverable = true
		pe.RecoveryAction = "retry_later"
		pe.RetryAfter = time.Minute * 5
		pe.CustomerMessage = "Our payment system is currently busy. Please try again in a few minutes."
		pe.InternalMessage = "Rate limit exceeded"

	case "invalid_card_number":
		pe.Recoverable = false
		pe.CustomerMessage = "The card number provided is invalid. Please check and try again."
		pe.InternalMessage = "Invalid card number format"

	case "provider_unavailable":
		pe.Recoverable = true
		pe.RecoveryAction = "use_alternate_provider"
		pe.CustomerMessage = "Processing your payment through an alternative provider."
		pe.InternalMessage = fmt.Sprintf("Provider %s unavailable", pe.Payment.Provider)

	case "fraud_suspected":
		pe.Recoverable = false
		pe.RecoveryAction = "manual_review"
		pe.CustomerMessage = "Your payment requires additional verification. Our team will contact you shortly."
		pe.InternalMessage = "Fraud detection triggered"

	default:
		// Unknown error - be conservative
		pe.Recoverable = true
		pe.RecoveryAction = "manual_review"
		pe.CustomerMessage = "An unexpected error occurred. Our team has been notified."
		pe.InternalMessage = fmt.Sprintf("Unknown error: %s", errMsg)
	}

	pe.Timestamp = time.Now()
	return pe, nil
}

// NotifyCustomer sends appropriate notification based on error type
func NotifyCustomer(_ context.Context, pe PaymentError) (PaymentError, error) {
	if pe.CustomerMessage == "" {
		return pe, nil
	}

	// Determine notification template based on error type
	var template string
	switch pe.ErrorType {
	case "card_declined":
		if pe.ErrorCode == "insufficient_funds" {
			template = "payment_declined_nsf"
		} else {
			template = "payment_declined_generic"
		}
	case "network_error", "rate_limit_error":
		template = "payment_delayed"
	case "fraud_suspected":
		template = "payment_review"
	default:
		template = "payment_failed_generic"
	}

	// Queue email notification
	email := EmailMessage{
		To:       pe.Payment.CustomerEmail,
		Subject:  "Payment Update - Action Required",
		Template: template,
		Data: map[string]interface{}{
			"payment_id":      pe.Payment.ID,
			"amount":          pe.Payment.Amount,
			"currency":        pe.Payment.Currency,
			"error_message":   pe.CustomerMessage,
			"payment_method":  fmt.Sprintf("**** %s", pe.Payment.PaymentMethod.Last4),
			"retry_scheduled": pe.Recoverable,
		},
	}

	select {
	case EmailQueue <- email:
		// Email queued successfully
	case <-time.After(time.Second):
		return pe, errors.New("failed to queue customer notification")
	}

	return pe, nil
}

// AttemptRecovery tries to recover from the error
func AttemptRecovery(ctx context.Context, pe PaymentError) (PaymentError, error) {
	if !pe.Recoverable {
		return pe, nil
	}

	switch pe.RecoveryAction {
	case "retry_with_backoff":
		if pe.Payment.RetryCount >= pe.Payment.MaxRetries {
			pe.Recoverable = false
			pe.RecoveryAction = "max_retries_exceeded"
			return pe, nil
		}

		// Wait before retry
		select {
		case <-time.After(pe.RetryAfter):
		case <-ctx.Done():
			return pe, ctx.Err()
		}

		// Retry with same provider
		provider := Providers[pe.Payment.Provider]
		if provider == nil {
			return pe, fmt.Errorf("provider %s not found", pe.Payment.Provider)
		}

		pe.Payment.RetryCount++
		result, err := provider.ProcessPayment(ctx, pe.Payment)
		if err == nil && result.Success {
			// Recovery successful!
			pe.OriginalError = nil
			pe.RecoveryAction = "retry_succeeded"
			UpdateProviderStats(pe.Payment.Provider, true, result.ProcessingTime)
			return pe, nil
		}

		// Still failing, will try again if retries remain
		pe.OriginalError = err
		UpdateProviderStats(pe.Payment.Provider, false, 0)

	case "use_alternate_provider":
		// Find an alternative provider
		var alternativeProvider PaymentProvider
		// Sort provider names for deterministic selection
		var providerNames []string
		for name := range Providers {
			providerNames = append(providerNames, name)
		}
		sort.Strings(providerNames)
		
		for _, name := range providerNames {
			provider := Providers[name]
			if name != pe.Payment.Provider && provider.IsAvailable() {
				alternativeProvider = provider
				pe.Payment.Provider = name
				break
			}
		}

		if alternativeProvider == nil {
			pe.RecoveryAction = "no_alternate_provider"
			return pe, errors.New("no alternative payment provider available")
		}

		// Try alternative provider
		result, err := alternativeProvider.ProcessPayment(ctx, pe.Payment)
		if err == nil && result.Success {
			// Recovery successful with alternative provider!
			pe.OriginalError = nil
			pe.RecoveryAction = fmt.Sprintf("recovered_via_%s", pe.Payment.Provider)
			UpdateProviderStats(pe.Payment.Provider, true, result.ProcessingTime)
			return pe, nil
		}

		// Alternative also failed
		pe.OriginalError = err
		UpdateProviderStats(pe.Payment.Provider, false, 0)

	case "retry_later":
		// Schedule for later retry (in production, this would use a job queue)
		// For demo, we'll just mark it for manual processing
		pe.RecoveryAction = "scheduled_retry"

	case "manual_review":
		// Queue for manual review
		select {
		case ReviewQueue <- pe.Payment:
			pe.RecoveryAction = "queued_for_review"
		case <-time.After(time.Second):
			return pe, errors.New("failed to queue for manual review")
		}
	}

	return pe, nil
}

// AuditPaymentError logs all payment errors for compliance and analysis
func AuditPaymentError(_ context.Context, pe PaymentError) error {
	auditMutex.Lock()
	defer auditMutex.Unlock()

	entry := AuditEntry{
		Timestamp:      pe.Timestamp,
		PaymentID:      pe.Payment.ID,
		CustomerID:     pe.Payment.CustomerID,
		Amount:         pe.Payment.Amount,
		Provider:       pe.Payment.Provider,
		ErrorType:      pe.ErrorType,
		RecoveryAction: pe.RecoveryAction,
		Success:        pe.OriginalError == nil,
	}

	AuditLog = append(AuditLog, entry)

	// Log critical errors
	if pe.ErrorType == "fraud_suspected" || pe.ErrorCode == "stolen_card" {
		fmt.Printf("[CRITICAL] Potential fraud detected: Payment %s, Customer %s\n",
			pe.Payment.ID, pe.Payment.CustomerID)
	}

	// Log recovery success
	if pe.OriginalError == nil && pe.RecoveryAction != "" {
		fmt.Printf("[RECOVERY] Payment %s recovered via %s\n",
			pe.Payment.ID, pe.RecoveryAction)
	}

	return nil
}

// UpdateMetrics updates payment processing metrics
func UpdateMetrics(_ context.Context, pe PaymentError) error {
	// In production, this would update Prometheus/DataDog metrics
	if pe.OriginalError == nil {
		fmt.Printf("[METRICS] Payment %s processed successfully via %s\n",
			pe.Payment.ID, pe.Payment.Provider)
	} else {
		fmt.Printf("[METRICS] Payment %s failed: %s (%s)\n",
			pe.Payment.ID, pe.ErrorType, pe.ErrorCode)
	}

	return nil
}

// CreateErrorRecoveryPipeline creates the main error handling pipeline
func CreateErrorRecoveryPipeline() *pipz.Pipeline[PaymentError] {
	pipeline := pipz.NewPipeline[PaymentError]()
	pipeline.Register(
		pipz.Apply("categorize_error", CategorizeError),
		pipz.Apply("notify_customer", NotifyCustomer),
		pipz.Apply("attempt_recovery", AttemptRecovery),
		pipz.Effect("audit_error", AuditPaymentError),
		pipz.Effect("update_metrics", UpdateMetrics),
	)
	return pipeline
}

// CreateHighValuePipeline creates a pipeline for high-value transactions
func CreateHighValuePipeline() *pipz.Pipeline[PaymentError] {
	pipeline := CreateErrorRecoveryPipeline()

	// Add extra validation for high-value payments
	pipeline.InsertAt(2, // Before recovery attempt
		pipz.Apply("verify_high_value", func(_ context.Context, pe PaymentError) (PaymentError, error) {
			if pe.Payment.Amount > 10000 {
				// High-value payments always require manual review on failure
				pe.RecoveryAction = "manual_review"
				pe.CustomerMessage = "Due to the high value of this transaction, our team will personally handle your payment."
			}
			return pe, nil
		}),
		pipz.Effect("notify_ops", func(_ context.Context, pe PaymentError) error {
			if pe.Payment.Amount > 10000 {
				fmt.Printf("[OPS_ALERT] High-value payment failure: %s ($%.2f)\n",
					pe.Payment.ID, pe.Payment.Amount)
			}
			return nil
		}),
	)

	return pipeline
}

// ProcessPayment attempts to process a payment with full error recovery
func ProcessPayment(ctx context.Context, payment Payment) error {
	// Set defaults
	if payment.MaxRetries == 0 {
		payment.MaxRetries = 3
	}
	if payment.Provider == "" {
		payment.Provider = "stripe" // Default provider
	}
	if payment.Metadata == nil {
		payment.Metadata = make(map[string]interface{})
	}

	// Get the provider
	provider, exists := Providers[payment.Provider]
	if !exists {
		return fmt.Errorf("payment provider %s not found", payment.Provider)
	}

	// Attempt payment
	startTime := time.Now()
	result, err := provider.ProcessPayment(ctx, payment)

	if err == nil && result.Success {
		// Payment successful!
		UpdateProviderStats(payment.Provider, true, time.Since(startTime))
		fmt.Printf("[SUCCESS] Payment %s processed via %s (Transaction: %s)\n",
			payment.ID, result.Provider, result.TransactionID)
		return nil
	}

	// Payment failed - run through error recovery pipeline
	UpdateProviderStats(payment.Provider, false, 0)

	// Determine which pipeline to use based on amount
	var pipeline *pipz.Pipeline[PaymentError]
	if payment.Amount > 5000 {
		pipeline = CreateHighValuePipeline()
	} else {
		pipeline = CreateErrorRecoveryPipeline()
	}

	paymentError := PaymentError{
		Payment:       payment,
		OriginalError: err,
		Timestamp:     time.Now(),
	}

	recoveredError, pipelineErr := pipeline.Process(ctx, paymentError)
	if pipelineErr != nil {
		return fmt.Errorf("error recovery pipeline failed: %w", pipelineErr)
	}

	// Check if recovery was successful
	if recoveredError.OriginalError == nil {
		fmt.Printf("[RECOVERED] Payment %s successfully recovered\n", payment.ID)
		return nil
	}

	// Payment failed after all recovery attempts
	return fmt.Errorf("payment failed: %s - %s",
		recoveredError.ErrorType, recoveredError.CustomerMessage)
}

// Helper functions

func UpdateProviderStats(providerName string, success bool, duration time.Duration) {
	statsMutex.Lock()
	defer statsMutex.Unlock()

	stats, exists := ProviderStatsMap[providerName]
	if !exists {
		stats = &ProviderStats{}
		ProviderStatsMap[providerName] = stats
	}

	stats.TotalAttempts++
	if success {
		stats.SuccessCount++
		if stats.AverageTime == 0 {
			stats.AverageTime = duration
		} else {
			// Simple moving average
			stats.AverageTime = (stats.AverageTime + duration) / 2
		}
	} else {
		stats.FailureCount++
		stats.LastFailureTime = time.Now()
	}
}

// GetProviderStats returns current provider statistics
func GetProviderStats() map[string]ProviderStats {
	statsMutex.Lock()
	defer statsMutex.Unlock()

	result := make(map[string]ProviderStats)
	for name, stats := range ProviderStatsMap {
		result[name] = *stats
	}
	return result
}

// GetAuditLog returns recent audit entries
func GetAuditLog(limit int) []AuditEntry {
	auditMutex.Lock()
	defer auditMutex.Unlock()

	if limit <= 0 || limit > len(AuditLog) {
		limit = len(AuditLog)
	}

	// Return most recent entries
	start := len(AuditLog) - limit
	if start < 0 {
		start = 0
	}

	result := make([]AuditEntry, limit)
	copy(result, AuditLog[start:])
	return result
}

// Example demonstrates the payment error recovery pipeline
func Example() {
	ctx := context.Background()

	// Example payments with different scenarios
	payments := []Payment{
		{
			ID:            "PAY-001",
			CustomerID:    "CUST-123",
			CustomerEmail: "customer@example.com",
			Amount:        99.99,
			Currency:      "USD",
			PaymentMethod: PaymentMethod{
				Type:  "card",
				Last4: "4242",
				Brand: "Visa",
			},
			Description: "Monthly subscription",
		},
		{
			ID:            "PAY-002",
			CustomerID:    "CUST-456",
			CustomerEmail: "premium@example.com",
			Amount:        15000.00, // High-value payment
			Currency:      "USD",
			PaymentMethod: PaymentMethod{
				Type:  "card",
				Last4: "5555",
				Brand: "Mastercard",
			},
			Description: "Annual enterprise license",
		},
		{
			ID:            "PAY-003",
			CustomerID:    "CUST-789",
			CustomerEmail: "user@example.com",
			Amount:        49.99,
			Currency:      "USD",
			PaymentMethod: PaymentMethod{
				Type:  "card",
				Last4: "1234",
				Brand: "Visa",
			},
			Description: "One-time purchase",
			Provider:    "paypal", // Use alternative provider
		},
	}

	fmt.Println("=== Processing Payments ===")
	for _, payment := range payments {
		fmt.Printf("\nProcessing payment %s ($%.2f)...\n", payment.ID, payment.Amount)
		if err := ProcessPayment(ctx, payment); err != nil {
			fmt.Printf("Payment failed: %v\n", err)
		}
	}

	// Show provider statistics
	fmt.Println("\n=== Provider Statistics ===")
	stats := GetProviderStats()
	for provider, stat := range stats {
		successRate := float64(stat.SuccessCount) / float64(stat.TotalAttempts) * 100
		fmt.Printf("%s: %.1f%% success rate (%d/%d attempts), avg time: %v\n",
			provider, successRate, stat.SuccessCount, stat.TotalAttempts, stat.AverageTime)
	}

	// Show recent audit log
	fmt.Println("\n=== Recent Audit Log ===")
	recentAudits := GetAuditLog(5)
	for _, audit := range recentAudits {
		status := "FAILED"
		if audit.Success {
			status = "SUCCESS"
		}
		fmt.Printf("[%s] Payment %s: %s (Provider: %s, Amount: $%.2f)\n",
			status, audit.PaymentID, audit.RecoveryAction, audit.Provider, audit.Amount)
	}

	// Process emails (in production, this would be a separate worker)
	fmt.Println("\n=== Queued Emails ===")
	processedEmails := 0
	for len(EmailQueue) > 0 {
		select {
		case email := <-EmailQueue:
			fmt.Printf("Email to %s: %s (Template: %s)\n",
				email.To, email.Subject, email.Template)
			processedEmails++
		default:
			break
		}
	}
	fmt.Printf("Processed %d email notifications\n", processedEmails)
}
