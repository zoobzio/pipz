package payment_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/zoobzio/pipz/examples/payment"
)

func BenchmarkPayment_ErrorCategorization(b *testing.B) {
	ctx := context.Background()

	errorTypes := []struct {
		name  string
		error string
	}{
		{"CardDeclined_NSF", "card_declined:insufficient_funds"},
		{"CardDeclined_Expired", "card_declined:expired_card"},
		{"NetworkError_Timeout", "network_error:timeout"},
		{"NetworkError_ConnReset", "network_error:connection_reset"},
		{"RateLimit", "rate_limit_error"},
		{"ProviderUnavailable", "provider_unavailable"},
		{"FraudSuspected", "fraud_suspected"},
		{"UnknownError", "something_went_wrong"},
	}

	for _, et := range errorTypes {
		b.Run(et.name, func(b *testing.B) {
			pe := payment.PaymentError{
				Payment: payment.Payment{
					ID:         "PAY-BENCH",
					CustomerID: "CUST-123",
					Amount:     99.99,
					Provider:   "stripe",
				},
				OriginalError: errors.New(et.error),
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _ = payment.CategorizeError(ctx, pe)
			}
		})
	}
}

func BenchmarkPayment_NotifyCustomer(b *testing.B) {
	ctx := context.Background()

	// Clear email queue
	for len(payment.EmailQueue) > 0 {
		<-payment.EmailQueue
	}

	b.Run("WithNotification", func(b *testing.B) {
		pe := payment.PaymentError{
			Payment: payment.Payment{
				ID:            "PAY-001",
				CustomerEmail: "test@example.com",
				Amount:        99.99,
				Currency:      "USD",
				PaymentMethod: payment.PaymentMethod{Last4: "4242"},
			},
			ErrorType:       "card_declined",
			ErrorCode:       "insufficient_funds",
			CustomerMessage: "Payment declined due to insufficient funds",
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = payment.NotifyCustomer(ctx, pe)
			// Clear queue to prevent overflow
			select {
			case <-payment.EmailQueue:
			default:
			}
		}
	})

	b.Run("WithoutNotification", func(b *testing.B) {
		pe := payment.PaymentError{
			Payment: payment.Payment{
				ID:            "PAY-002",
				CustomerEmail: "test@example.com",
			},
			CustomerMessage: "", // No message
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = payment.NotifyCustomer(ctx, pe)
		}
	})
}

func BenchmarkPayment_AttemptRecovery(b *testing.B) {
	ctx := context.Background()

	// Set up mock providers
	payment.Providers["bench_success"] = &mockBenchProvider{
		name:      "bench_success",
		available: true,
		succeed:   true,
	}
	payment.Providers["bench_fail"] = &mockBenchProvider{
		name:      "bench_fail",
		available: true,
		succeed:   false,
	}

	b.Run("NonRecoverable", func(b *testing.B) {
		pe := payment.PaymentError{
			Payment:        payment.Payment{ID: "PAY-001"},
			Recoverable:    false,
			RecoveryAction: "",
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = payment.AttemptRecovery(ctx, pe)
		}
	})

	b.Run("RetrySuccess", func(b *testing.B) {
		pe := payment.PaymentError{
			Payment: payment.Payment{
				ID:         "PAY-002",
				Provider:   "bench_success",
				RetryCount: 0,
				MaxRetries: 3,
			},
			OriginalError:  errors.New("network_error:timeout"),
			Recoverable:    true,
			RecoveryAction: "retry_with_backoff",
			RetryAfter:     1 * time.Microsecond, // Minimal wait for benchmark
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			peClone := pe // Clone to reset retry count
			_, _ = payment.AttemptRecovery(ctx, peClone)
		}
	})

	b.Run("RetryFailure", func(b *testing.B) {
		pe := payment.PaymentError{
			Payment: payment.Payment{
				ID:         "PAY-003",
				Provider:   "bench_fail",
				RetryCount: 0,
				MaxRetries: 3,
			},
			OriginalError:  errors.New("network_error:timeout"),
			Recoverable:    true,
			RecoveryAction: "retry_with_backoff",
			RetryAfter:     1 * time.Microsecond,
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			peClone := pe
			_, _ = payment.AttemptRecovery(ctx, peClone)
		}
	})

	b.Run("AlternateProvider", func(b *testing.B) {
		pe := payment.PaymentError{
			Payment: payment.Payment{
				ID:       "PAY-004",
				Provider: "bench_fail",
			},
			OriginalError:  errors.New("provider_unavailable"),
			Recoverable:    true,
			RecoveryAction: "use_alternate_provider",
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = payment.AttemptRecovery(ctx, pe)
		}
	})

	b.Run("ManualReview", func(b *testing.B) {
		// Clear review queue
		for len(payment.ReviewQueue) > 0 {
			<-payment.ReviewQueue
		}

		pe := payment.PaymentError{
			Payment: payment.Payment{
				ID: "PAY-005",
			},
			Recoverable:    true,
			RecoveryAction: "manual_review",
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = payment.AttemptRecovery(ctx, pe)
			// Clear queue to prevent overflow
			select {
			case <-payment.ReviewQueue:
			default:
			}
		}
	})
}

func BenchmarkPayment_Pipeline(b *testing.B) {
	ctx := context.Background()

	// Set up providers
	payment.Providers["pipeline_bench"] = &mockBenchProvider{
		name:      "pipeline_bench",
		available: true,
		succeed:   true,
	}

	b.Run("StandardPipeline_Success", func(b *testing.B) {
		pipeline := payment.CreateErrorRecoveryPipeline()
		pe := payment.PaymentError{
			Payment: payment.Payment{
				ID:            "PAY-001",
				CustomerEmail: "test@example.com",
				Provider:      "pipeline_bench",
				MaxRetries:    3,
			},
			OriginalError: errors.New("network_error:timeout"),
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			peClone := pe
			_, _ = pipeline.Process(ctx, peClone)
			// Clear queues
			select {
			case <-payment.EmailQueue:
			default:
			}
		}
	})

	b.Run("StandardPipeline_NonRecoverable", func(b *testing.B) {
		pipeline := payment.CreateErrorRecoveryPipeline()
		pe := payment.PaymentError{
			Payment: payment.Payment{
				ID:            "PAY-002",
				CustomerEmail: "test@example.com",
			},
			OriginalError: errors.New("card_declined:insufficient_funds"),
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = pipeline.Process(ctx, pe)
			// Clear queues
			select {
			case <-payment.EmailQueue:
			default:
			}
		}
	})

	b.Run("HighValuePipeline", func(b *testing.B) {
		pipeline := payment.CreateHighValuePipeline()
		pe := payment.PaymentError{
			Payment: payment.Payment{
				ID:            "PAY-HIGH",
				CustomerEmail: "vip@example.com",
				Amount:        15000.00,
			},
			OriginalError: errors.New("network_error:timeout"),
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = pipeline.Process(ctx, pe)
			// Clear queues
			select {
			case <-payment.EmailQueue:
			case <-payment.ReviewQueue:
			default:
			}
		}
	})
}

func BenchmarkPayment_ProcessPayment(b *testing.B) {
	ctx := context.Background()

	// Set up providers with different characteristics
	payment.Providers["fast_provider"] = &mockBenchProvider{
		name:      "fast_provider",
		available: true,
		succeed:   true,
		delay:     1 * time.Millisecond,
	}
	payment.Providers["slow_provider"] = &mockBenchProvider{
		name:      "slow_provider",
		available: true,
		succeed:   true,
		delay:     10 * time.Millisecond,
	}
	payment.Providers["unreliable_provider"] = &mockBenchProvider{
		name:         "unreliable_provider",
		available:    true,
		succeed:      false,
		failureError: "network_error:timeout",
		delay:        5 * time.Millisecond,
	}

	b.Run("SuccessfulPayment", func(b *testing.B) {
		p := payment.Payment{
			ID:            "PAY-SUCCESS",
			CustomerEmail: "success@example.com",
			Amount:        99.99,
			Provider:      "fast_provider",
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = payment.ProcessPayment(ctx, p)
		}
	})

	b.Run("FailedPayment_NonRecoverable", func(b *testing.B) {
		// Use a provider that fails with non-recoverable error
		payment.Providers["declining_provider"] = &mockBenchProvider{
			name:         "declining_provider",
			available:    true,
			succeed:      false,
			failureError: "card_declined:insufficient_funds",
			delay:        2 * time.Millisecond,
		}

		p := payment.Payment{
			ID:            "PAY-FAIL",
			CustomerEmail: "fail@example.com",
			Amount:        49.99,
			Provider:      "declining_provider",
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = payment.ProcessPayment(ctx, p)
			// Clear queues
			select {
			case <-payment.EmailQueue:
			default:
			}
		}
	})

	b.Run("HighValuePayment", func(b *testing.B) {
		p := payment.Payment{
			ID:            "PAY-HIGH-VALUE",
			CustomerEmail: "vip@example.com",
			Amount:        25000.00,
			Provider:      "fast_provider",
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = payment.ProcessPayment(ctx, p)
		}
	})
}

func BenchmarkPayment_ErrorTypes(b *testing.B) {
	ctx := context.Background()
	pipeline := payment.CreateErrorRecoveryPipeline()

	errorScenarios := []struct {
		name     string
		errorMsg string
		amount   float64
	}{
		{"InsufficientFunds", "card_declined:insufficient_funds", 99.99},
		{"ExpiredCard", "card_declined:expired_card", 149.99},
		{"NetworkTimeout", "network_error:timeout", 199.99},
		{"RateLimit", "rate_limit_error", 299.99},
		{"ProviderUnavailable", "provider_unavailable", 399.99},
		{"FraudSuspected", "fraud_suspected", 999.99},
		{"UnknownError", "unknown_error:xyz", 599.99},
	}

	for _, scenario := range errorScenarios {
		b.Run(scenario.name, func(b *testing.B) {
			pe := payment.PaymentError{
				Payment: payment.Payment{
					ID:            fmt.Sprintf("PAY-%s", scenario.name),
					CustomerEmail: "bench@example.com",
					Amount:        scenario.amount,
					Provider:      "test",
				},
				OriginalError: errors.New(scenario.errorMsg),
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, _ = pipeline.Process(ctx, pe)
				// Clear queues
				select {
				case <-payment.EmailQueue:
				case <-payment.ReviewQueue:
				default:
				}
			}
		})
	}
}

func BenchmarkPayment_ProviderStats(b *testing.B) {
	// Clear stats
	payment.ProviderStatsMap = make(map[string]*payment.ProviderStats)

	b.Run("UpdateStats", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			payment.UpdateProviderStats("bench_provider", i%2 == 0, time.Duration(i)*time.Millisecond)
		}
	})

	b.Run("GetStats", func(b *testing.B) {
		// Pre-populate some stats
		for i := 0; i < 10; i++ {
			provider := fmt.Sprintf("provider_%d", i)
			for j := 0; j < 100; j++ {
				payment.UpdateProviderStats(provider, j%3 != 0, time.Duration(j)*time.Millisecond)
			}
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = payment.GetProviderStats()
		}
	})
}

func BenchmarkPayment_AuditLog(b *testing.B) {
	ctx := context.Background()

	// Clear audit log
	payment.AuditLog = make([]payment.AuditEntry, 0)

	b.Run("AuditPaymentError", func(b *testing.B) {
		pe := payment.PaymentError{
			Payment: payment.Payment{
				ID:         "PAY-AUDIT",
				CustomerID: "CUST-123",
				Amount:     99.99,
				Provider:   "test",
			},
			ErrorType:      "card_declined",
			RecoveryAction: "none",
			Timestamp:      time.Now(),
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = payment.AuditPaymentError(ctx, pe)
		}
	})

	b.Run("GetAuditLog", func(b *testing.B) {
		// Pre-populate audit log
		for i := 0; i < 1000; i++ {
			pe := payment.PaymentError{
				Payment: payment.Payment{
					ID:         fmt.Sprintf("PAY-%d", i),
					CustomerID: fmt.Sprintf("CUST-%d", i%100),
					Amount:     float64(i),
					Provider:   "test",
				},
				ErrorType:      "test_error",
				RecoveryAction: "test_action",
				Timestamp:      time.Now(),
			}
			_ = payment.AuditPaymentError(ctx, pe)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = payment.GetAuditLog(100)
		}
	})
}

func BenchmarkPayment_ConcurrentProcessing(b *testing.B) {
	ctx := context.Background()

	// Set up a reliable provider
	payment.Providers["concurrent_provider"] = &mockBenchProvider{
		name:      "concurrent_provider",
		available: true,
		succeed:   true,
		delay:     1 * time.Millisecond,
	}

	b.Run("ConcurrentPayments", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				p := payment.Payment{
					ID:            fmt.Sprintf("PAY-CONC-%d-%d", time.Now().UnixNano(), i),
					CustomerEmail: "concurrent@example.com",
					Amount:        99.99,
					Provider:      "concurrent_provider",
				}
				_ = payment.ProcessPayment(ctx, p)
				i++
			}
		})
	})

	b.Run("ConcurrentErrorRecovery", func(b *testing.B) {
		pipeline := payment.CreateErrorRecoveryPipeline()

		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				pe := payment.PaymentError{
					Payment: payment.Payment{
						ID:            fmt.Sprintf("PAY-ERR-%d-%d", time.Now().UnixNano(), i),
						CustomerEmail: "error@example.com",
						Provider:      "concurrent_provider",
					},
					OriginalError: errors.New("network_error:timeout"),
				}
				_, _ = pipeline.Process(ctx, pe)
				i++

				// Clear queues periodically
				select {
				case <-payment.EmailQueue:
				default:
				}
			}
		})
	})
}

// Mock provider for benchmarks
type mockBenchProvider struct {
	name         string
	available    bool
	succeed      bool
	failureError string
	delay        time.Duration
}

func (m *mockBenchProvider) Name() string                   { return m.name }
func (m *mockBenchProvider) IsAvailable() bool              { return m.available }
func (m *mockBenchProvider) GetFees(amount float64) float64 { return amount * 0.03 }

func (m *mockBenchProvider) ProcessPayment(ctx context.Context, p payment.Payment) (*payment.PaymentResult, error) {
	if m.delay > 0 {
		time.Sleep(m.delay)
	}

	if !m.succeed {
		if m.failureError != "" {
			return nil, errors.New(m.failureError)
		}
		return nil, errors.New("payment failed")
	}

	return &payment.PaymentResult{
		Success:        true,
		TransactionID:  fmt.Sprintf("%s_%s_%d", m.name, p.ID, time.Now().Unix()),
		Provider:       m.name,
		ProcessingTime: m.delay,
		Fees:           p.Amount * 0.03,
	}, nil
}
