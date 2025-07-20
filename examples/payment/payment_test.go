package payment

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/zoobzio/pipz"
)

func TestCategorizeError(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name            string
		pe              PaymentError
		expectedType    string
		expectedCode    string
		expectedRecover bool
		expectedAction  string
	}{
		{
			name: "insufficient funds",
			pe: PaymentError{
				Payment:       Payment{ID: "PAY-001"},
				OriginalError: errors.New("card_declined:insufficient_funds"),
			},
			expectedType:    "card_declined",
			expectedCode:    "insufficient_funds",
			expectedRecover: false,
			expectedAction:  "",
		},
		{
			name: "network timeout",
			pe: PaymentError{
				Payment:       Payment{ID: "PAY-002", RetryCount: 0},
				OriginalError: errors.New("network_error:timeout"),
			},
			expectedType:    "network_error",
			expectedCode:    "timeout",
			expectedRecover: true,
			expectedAction:  "retry_with_backoff",
		},
		{
			name: "rate limit",
			pe: PaymentError{
				Payment:       Payment{ID: "PAY-003"},
				OriginalError: errors.New("rate_limit_error"),
			},
			expectedType:    "rate_limit_error",
			expectedCode:    "",
			expectedRecover: true,
			expectedAction:  "retry_later",
		},
		{
			name: "provider unavailable",
			pe: PaymentError{
				Payment:       Payment{ID: "PAY-004", Provider: "stripe"},
				OriginalError: errors.New("provider_unavailable"),
			},
			expectedType:    "provider_unavailable",
			expectedCode:    "",
			expectedRecover: true,
			expectedAction:  "use_alternate_provider",
		},
		{
			name: "fraud suspected",
			pe: PaymentError{
				Payment:       Payment{ID: "PAY-005"},
				OriginalError: errors.New("fraud_suspected"),
			},
			expectedType:    "fraud_suspected",
			expectedCode:    "",
			expectedRecover: false,
			expectedAction:  "manual_review",
		},
		{
			name: "unknown error",
			pe: PaymentError{
				Payment:       Payment{ID: "PAY-006"},
				OriginalError: errors.New("something_went_wrong"),
			},
			expectedType:    "something_went_wrong",
			expectedCode:    "",
			expectedRecover: true,
			expectedAction:  "manual_review",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := CategorizeError(ctx, tt.pe)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.ErrorType != tt.expectedType {
				t.Errorf("expected error type %q, got %q", tt.expectedType, result.ErrorType)
			}

			if result.ErrorCode != tt.expectedCode {
				t.Errorf("expected error code %q, got %q", tt.expectedCode, result.ErrorCode)
			}

			if result.Recoverable != tt.expectedRecover {
				t.Errorf("expected recoverable %v, got %v", tt.expectedRecover, result.Recoverable)
			}

			if result.RecoveryAction != tt.expectedAction {
				t.Errorf("expected recovery action %q, got %q", tt.expectedAction, result.RecoveryAction)
			}

			if result.CustomerMessage == "" {
				t.Error("expected customer message to be set")
			}

			if result.InternalMessage == "" {
				t.Error("expected internal message to be set")
			}
		})
	}
}

func TestNotifyCustomer(t *testing.T) {
	ctx := context.Background()

	// Clear email queue
	for len(EmailQueue) > 0 {
		<-EmailQueue
	}

	tests := []struct {
		name          string
		pe            PaymentError
		expectEmail   bool
		checkTemplate string
	}{
		{
			name: "insufficient funds notification",
			pe: PaymentError{
				Payment: Payment{
					ID:            "PAY-001",
					CustomerEmail: "test@example.com",
					Amount:        99.99,
					Currency:      "USD",
					PaymentMethod: PaymentMethod{Last4: "4242"},
				},
				ErrorType:       "card_declined",
				ErrorCode:       "insufficient_funds",
				CustomerMessage: "Payment declined due to insufficient funds",
			},
			expectEmail:   true,
			checkTemplate: "payment_declined_nsf",
		},
		{
			name: "network error notification",
			pe: PaymentError{
				Payment: Payment{
					ID:            "PAY-002",
					CustomerEmail: "test@example.com",
				},
				ErrorType:       "network_error",
				CustomerMessage: "Temporary issue, will retry",
				Recoverable:     true,
			},
			expectEmail:   true,
			checkTemplate: "payment_delayed",
		},
		{
			name: "no message no notification",
			pe: PaymentError{
				Payment: Payment{
					ID:            "PAY-003",
					CustomerEmail: "test@example.com",
				},
				CustomerMessage: "",
			},
			expectEmail: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear queue
			for len(EmailQueue) > 0 {
				<-EmailQueue
			}

			result, err := NotifyCustomer(ctx, tt.pe)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.expectEmail {
				select {
				case email := <-EmailQueue:
					if email.To != tt.pe.Payment.CustomerEmail {
						t.Errorf("expected email to %q, got %q", tt.pe.Payment.CustomerEmail, email.To)
					}
					if tt.checkTemplate != "" && email.Template != tt.checkTemplate {
						t.Errorf("expected template %q, got %q", tt.checkTemplate, email.Template)
					}
				case <-time.After(100 * time.Millisecond):
					t.Error("expected email to be queued")
				}
			} else {
				if len(EmailQueue) > 0 {
					t.Error("unexpected email queued")
				}
			}

			// Verify result is unchanged
			if result.Payment.ID != tt.pe.Payment.ID {
				t.Error("payment error was modified unexpectedly")
			}
		})
	}
}

func TestAttemptRecovery(t *testing.T) {
	ctx := context.Background()

	// Set up test Providers
	Providers["test_provider"] = &mockProvider{
		name:        "test_provider",
		available:   true,
		shouldFail:  false,
		failMessage: "",
	}
	Providers["backup_provider"] = &mockProvider{
		name:        "backup_provider",
		available:   true,
		shouldFail:  false,
		failMessage: "",
	}
	Providers["failing_provider"] = &mockProvider{
		name:        "failing_provider",
		available:   true,
		shouldFail:  true,
		failMessage: "network_error:timeout",
	}

	tests := []struct {
		name          string
		pe            PaymentError
		expectSuccess bool
		expectAction  string
	}{
		{
			name: "non-recoverable error",
			pe: PaymentError{
				Payment:        Payment{ID: "PAY-001"},
				Recoverable:    false,
				RecoveryAction: "",
			},
			expectSuccess: false,
			expectAction:  "",
		},
		{
			name: "retry with backoff success",
			pe: PaymentError{
				Payment: Payment{
					ID:         "PAY-002",
					Provider:   "test_provider",
					RetryCount: 0,
					MaxRetries: 3,
				},
				OriginalError:  errors.New("network_error:timeout"),
				Recoverable:    true,
				RecoveryAction: "retry_with_backoff",
				RetryAfter:     1 * time.Millisecond,
			},
			expectSuccess: true,
			expectAction:  "retry_succeeded",
		},
		{
			name: "max retries exceeded",
			pe: PaymentError{
				Payment: Payment{
					ID:         "PAY-003",
					Provider:   "test_provider",
					RetryCount: 3,
					MaxRetries: 3,
				},
				Recoverable:    true,
				RecoveryAction: "retry_with_backoff",
			},
			expectSuccess: false,
			expectAction:  "max_retries_exceeded",
		},
		{
			name: "use alternate provider success",
			pe: PaymentError{
				Payment: Payment{
					ID:       "PAY-004",
					Provider: "failing_provider",
				},
				OriginalError:  errors.New("provider_unavailable"),
				Recoverable:    true,
				RecoveryAction: "use_alternate_provider",
			},
			expectSuccess: true,
			expectAction:  "recovered_via_backup_provider",
		},
		{
			name: "manual review",
			pe: PaymentError{
				Payment: Payment{
					ID: "PAY-005",
				},
				Recoverable:    true,
				RecoveryAction: "manual_review",
			},
			expectSuccess: false,
			expectAction:  "queued_for_review",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear review queue
			for len(ReviewQueue) > 0 {
				<-ReviewQueue
			}

			result, err := AttemptRecovery(ctx, tt.pe)
			if err != nil && !strings.Contains(tt.name, "fail") {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.expectSuccess {
				if result.OriginalError != nil {
					t.Errorf("expected recovery to succeed, but got error: %v", result.OriginalError)
				}
			} else {
				if result.OriginalError == nil && tt.pe.OriginalError != nil {
					t.Error("expected recovery to fail, but error was cleared")
				}
			}

			if tt.expectAction != "" && result.RecoveryAction != tt.expectAction {
				t.Errorf("expected recovery action %q, got %q", tt.expectAction, result.RecoveryAction)
			}
		})
	}
}

func TestErrorRecoveryPipeline(t *testing.T) {
	ctx := context.Background()
	pipeline := CreateErrorRecoveryPipeline()

	// Set up test provider
	Providers["pipeline_test"] = &mockProvider{
		name:        "pipeline_test",
		available:   true,
		shouldFail:  false,
		failMessage: "",
	}

	tests := []struct {
		name          string
		pe            PaymentError
		expectRecover bool
		checkFn       func(t *testing.T, result PaymentError)
	}{
		{
			name: "successful recovery through pipeline",
			pe: PaymentError{
				Payment: Payment{
					ID:            "PAY-001",
					CustomerEmail: "test@example.com",
					Provider:      "pipeline_test",
					MaxRetries:    3,
				},
				OriginalError: errors.New("network_error:timeout"),
			},
			expectRecover: true,
		},
		{
			name: "non-recoverable error",
			pe: PaymentError{
				Payment: Payment{
					ID:            "PAY-002",
					CustomerEmail: "test@example.com",
				},
				OriginalError: errors.New("card_declined:insufficient_funds"),
			},
			expectRecover: false,
		},
		{
			name: "fraud detection",
			pe: PaymentError{
				Payment: Payment{
					ID:            "PAY-003",
					CustomerEmail: "fraud@example.com",
				},
				OriginalError: errors.New("fraud_suspected"),
			},
			expectRecover: false,
			checkFn: func(t *testing.T, result PaymentError) {
				if result.RecoveryAction != "manual_review" {
					t.Error("expected fraud to trigger manual review")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear queues
			for len(EmailQueue) > 0 {
				<-EmailQueue
			}
			for len(ReviewQueue) > 0 {
				<-ReviewQueue
			}

			result, err := pipeline.Process(ctx, tt.pe)
			if err != nil {
				t.Fatalf("pipeline error: %v", err)
			}

			if tt.expectRecover {
				if result.OriginalError != nil {
					t.Errorf("expected recovery, but still have error: %v", result.OriginalError)
				}
			} else {
				if result.OriginalError == nil {
					t.Error("expected error to persist, but it was cleared")
				}
			}

			if tt.checkFn != nil {
				tt.checkFn(t, result)
			}
		})
	}
}

func TestHighValuePipeline(t *testing.T) {
	ctx := context.Background()
	pipeline := CreateHighValuePipeline()

	pe := PaymentError{
		Payment: Payment{
			ID:            "PAY-HIGH-001",
			CustomerEmail: "vip@example.com",
			Amount:        15000.00,
		},
		OriginalError: errors.New("network_error:timeout"),
	}

	result, err := pipeline.Process(ctx, pe)
	if err != nil {
		t.Fatalf("pipeline error: %v", err)
	}

	// High-value payment should trigger manual review
	if result.RecoveryAction != "queued_for_review" {
		t.Errorf("expected queued_for_review for high-value payment, got %q", result.RecoveryAction)
	}

	if !strings.Contains(result.CustomerMessage, "high value") {
		t.Error("expected high-value specific message")
	}
}

func TestProcessPayment(t *testing.T) {
	ctx := context.Background()

	// Set up test Providers
	Providers["process_test"] = &mockProvider{
		name:        "process_test",
		available:   true,
		shouldFail:  false,
		failMessage: "",
	}
	Providers["process_failing"] = &mockProvider{
		name:        "process_failing",
		available:   true,
		shouldFail:  true,
		failMessage: "card_declined:insufficient_funds",
	}

	tests := []struct {
		name        string
		payment     Payment
		expectError bool
		errorMsg    string
	}{
		{
			name: "successful payment",
			payment: Payment{
				ID:            "PAY-001",
				CustomerEmail: "success@example.com",
				Amount:        99.99,
				Provider:      "process_test",
			},
			expectError: false,
		},
		{
			name: "payment failure non-recoverable",
			payment: Payment{
				ID:            "PAY-002",
				CustomerEmail: "fail@example.com",
				Amount:        49.99,
				Provider:      "process_failing",
			},
			expectError: true,
			errorMsg:    "card_declined",
		},
		{
			name: "invalid provider",
			payment: Payment{
				ID:       "PAY-003",
				Provider: "non_existent",
			},
			expectError: true,
			errorMsg:    "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ProcessPayment(ctx, tt.payment)

			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				} else if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestProviderStats(t *testing.T) {
	// Clear stats
	ProviderStatsMap = make(map[string]*ProviderStats)

	// Update some stats
	UpdateProviderStats("test_provider", true, 100*time.Millisecond)
	UpdateProviderStats("test_provider", true, 200*time.Millisecond)
	UpdateProviderStats("test_provider", false, 0)

	stats := GetProviderStats()

	if len(stats) != 1 {
		t.Errorf("expected 1 provider, got %d", len(stats))
	}

	testStats, exists := stats["test_provider"]
	if !exists {
		t.Fatal("test_provider stats not found")
	}

	if testStats.TotalAttempts != 3 {
		t.Errorf("expected 3 attempts, got %d", testStats.TotalAttempts)
	}

	if testStats.SuccessCount != 2 {
		t.Errorf("expected 2 successes, got %d", testStats.SuccessCount)
	}

	if testStats.FailureCount != 1 {
		t.Errorf("expected 1 failure, got %d", testStats.FailureCount)
	}
}

func TestAuditLog(t *testing.T) {
	// Clear audit log
	AuditLog = make([]AuditEntry, 0)

	ctx := context.Background()

	// Add some entries via AuditPaymentError
	pe1 := PaymentError{
		Payment: Payment{
			ID:         "PAY-001",
			CustomerID: "CUST-123",
			Amount:     99.99,
			Provider:   "test",
		},
		ErrorType:      "card_declined",
		RecoveryAction: "none",
		Timestamp:      time.Now(),
	}

	if err := AuditPaymentError(ctx, pe1); err != nil {
		t.Fatalf("audit error: %v", err)
	}

	pe2 := PaymentError{
		Payment: Payment{
			ID:         "PAY-002",
			CustomerID: "CUST-456",
			Amount:     199.99,
			Provider:   "test",
		},
		ErrorType:      "network_error",
		RecoveryAction: "retry_succeeded",
		Timestamp:      time.Now(),
		OriginalError:  nil, // Recovered
	}

	if err := AuditPaymentError(ctx, pe2); err != nil {
		t.Fatalf("audit error: %v", err)
	}

	// Get audit log
	entries := GetAuditLog(10)

	if len(entries) != 2 {
		t.Errorf("expected 2 audit entries, got %d", len(entries))
	}

	// Verify entries
	if entries[0].PaymentID != "PAY-001" {
		t.Errorf("expected first entry to be PAY-001, got %s", entries[0].PaymentID)
	}

	if entries[1].Success != true {
		t.Error("expected second entry to show success")
	}
}

func TestPipelineErrorContext(t *testing.T) {
	ctx := context.Background()
	pipeline := CreateErrorRecoveryPipeline()

	// Create a payment error with a valid error
	pe := PaymentError{
		Payment:       Payment{ID: "PAY-ERR"},
		OriginalError: errors.New("test_error"), // Provide a valid error
	}

	// This should process without issues
	result, err := pipeline.Process(ctx, pe)
	if err != nil {
		// Check if it's a pipeline error
		var pipelineErr *pipz.PipelineError[PaymentError]
		if !errors.As(err, &pipelineErr) {
			t.Fatalf("expected PipelineError, got %T", err)
		}
	}

	// Result should still be returned
	if result.Payment.ID != "PAY-ERR" {
		t.Error("expected payment ID to be preserved")
	}
}

func TestExample(t *testing.T) {
	// Just test that the example runs without panicking
	Example()
}

// Mock provider for testing
type mockProvider struct {
	name        string
	available   bool
	shouldFail  bool
	failMessage string
}

func (m *mockProvider) Name() string                   { return m.name }
func (m *mockProvider) IsAvailable() bool              { return m.available }
func (m *mockProvider) GetFees(amount float64) float64 { return amount * 0.03 }

func (m *mockProvider) ProcessPayment(ctx context.Context, payment Payment) (*PaymentResult, error) {
	if m.shouldFail {
		if m.failMessage != "" {
			return nil, fmt.Errorf(m.failMessage)
		}
		return nil, errors.New("mock failure")
	}

	return &PaymentResult{
		Success:        true,
		TransactionID:  fmt.Sprintf("mock_%s_%d", payment.ID, time.Now().Unix()),
		Provider:       m.name,
		ProcessingTime: 10 * time.Millisecond,
		Fees:           payment.Amount * 0.03,
	}, nil
}
