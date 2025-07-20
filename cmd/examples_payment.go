package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/zoobzio/pipz"
	"github.com/zoobzio/pipz/examples/payment"
)

// PaymentExample implements the Example interface for payment processing
type PaymentExample struct{}

func (p *PaymentExample) Name() string {
	return "payment"
}

func (p *PaymentExample) Description() string {
	return "Payment processing with sophisticated error recovery"
}

func (p *PaymentExample) Demo(ctx context.Context) error {
	fmt.Println("\n" + colorCyan + "═══ PAYMENT PROCESSING EXAMPLE ═══" + colorReset)
	fmt.Println("\nThis example demonstrates:")
	fmt.Println("• Provider failover (Stripe → PayPal → Bank Transfer)")
	fmt.Println("• Smart retry strategies based on error types")
	fmt.Println("• Automatic payment splitting for insufficient funds")
	fmt.Println("• Transaction recovery and rollback")

	waitForEnter()

	// Show the problem with code
	fmt.Println("\n" + colorYellow + "The Problem:" + colorReset)
	fmt.Println("Payment processing requires handling multiple failure modes:")
	fmt.Println(colorGray + `
func processPayment(payment Payment) error {
    // Try Stripe
    result, err := chargeStripe(payment)
    if err != nil {
        // Is it retryable?
        if isRetryable(err) {
            for i := 0; i < 3; i++ {
                time.Sleep(time.Second * time.Duration(i+1))
                result, err = chargeStripe(payment)
                if err == nil {
                    break
                }
            }
        }
        
        // Still failed? Try PayPal
        if err != nil {
            result, err = chargePayPal(payment)
            if err != nil {
                // Both failed - what now?
                if isInsufficientFunds(err) && payment.Amount > 100 {
                    // Try splitting payment...
                    // More nested logic...
                }
            }
        }
    }
    // ... and it gets worse
}` + colorReset)

	waitForEnter()

	// Show the solution with full code
	fmt.Println("\n" + colorGreen + "The pipz Solution:" + colorReset)
	fmt.Println("Define error categories and recovery strategies:")
	fmt.Println(colorGray + `
// Error categorization
func categorizeError(provider string, err error) error {
    if strings.Contains(err.Error(), "insufficient funds") {
        return &RecoverableError{
            Type:     ErrorInsufficientFunds,
            Provider: provider,
            Cause:    err,
        }
    }
    // ... other error types
}

// Provider processors
stripeProcessor := pipz.Apply("stripe", func(ctx context.Context, p Payment) (Payment, error) {
    result, err := chargeViaStripe(ctx, p)
    if err != nil {
        return p, categorizeError("stripe", err)
    }
    p.TransactionID = result.ID
    return p, nil
})

// Smart recovery
splitPaymentRecovery := pipz.Apply("split_payment", func(ctx context.Context, p Payment) (Payment, error) {
    if p.Amount <= 100 {
        return p, errors.New("amount too small to split")
    }
    
    // Process first half
    half1 := p
    half1.Amount = p.Amount / 2
    result1, err := processHalf(ctx, half1)
    if err != nil {
        return p, err
    }
    
    // Process second half
    half2 := p
    half2.Amount = p.Amount - half1.Amount
    result2, err := processHalf(ctx, half2)
    if err != nil {
        // Rollback first half
        rollback(result1)
        return p, err
    }
    
    p.TransactionID = result1.ID + "," + result2.ID
    return p, nil
})

// Build the pipeline
paymentPipeline := pipz.Sequential(
    pipz.Effect("validate", validatePayment),
    pipz.Apply("fraud_check", checkFraud),
    
    // Sophisticated error recovery
    pipz.Fallback(
        pipz.RetryWithBackoff(stripeProcessor, 3, time.Second),
        pipz.RetryWithBackoff(paypalProcessor, 2, 2*time.Second), 
        bankTransferProcessor,
        splitPaymentRecovery, // Last resort
    ),
    
    pipz.Effect("record", recordTransaction),
)` + colorReset)

	waitForEnter()

	// Run interactive demo
	fmt.Println("\n" + colorPurple + "Let's process some payments!" + colorReset)
	return p.runInteractive(ctx)
}

func (p *PaymentExample) runInteractive(ctx context.Context) error {
	// Create a simple payment pipeline for demo
	pipeline := createDemoPaymentPipeline()

	// Test scenarios
	scenarios := []struct {
		name        string
		payment     payment.Payment
		forceErrors map[string]error
	}{
		{
			name: "Successful Payment (Stripe)",
			payment: payment.Payment{
				ID:            "PAY-001",
				CustomerID:    "CUST-123",
				CustomerEmail: "user@example.com",
				Amount:        99.99,
				Currency:      "USD",
				Timestamp:     time.Now(),
			},
			forceErrors: map[string]error{},
		},
		{
			name: "Stripe Fails → PayPal Success",
			payment: payment.Payment{
				ID:            "PAY-002",
				CustomerID:    "CUST-456",
				CustomerEmail: "user2@example.com",
				Amount:        149.99,
				Currency:      "USD",
				Timestamp:     time.Now(),
			},
			forceErrors: map[string]error{
				"stripe": errors.New("card declined"),
			},
		},
		{
			name: "Insufficient Funds → Split Payment",
			payment: payment.Payment{
				ID:            "PAY-003",
				CustomerID:    "CUST-789",
				CustomerEmail: "user3@example.com",
				Amount:        500.00,
				Currency:      "USD",
				Timestamp:     time.Now(),
			},
			forceErrors: map[string]error{
				"stripe": errors.New("insufficient funds"),
				"paypal": errors.New("insufficient funds"),
			},
		},
		{
			name: "Network Error → Retry Success",
			payment: payment.Payment{
				ID:            "PAY-004",
				CustomerID:    "CUST-999",
				CustomerEmail: "user4@example.com",
				Amount:        75.00,
				Currency:      "USD",
				Timestamp:     time.Now(),
			},
			forceErrors: map[string]error{
				"stripe_attempt_1": errors.New("network timeout"),
				"stripe_attempt_2": errors.New("network timeout"),
				// Third attempt succeeds
			},
		},
	}

	for i, scenario := range scenarios {
		fmt.Printf("\n%s═══ Scenario %d: %s ═══%s\n", 
			colorWhite, i+1, scenario.name, colorReset)
		
		fmt.Printf("Payment Details:\n")
		fmt.Printf("  Amount: $%.2f %s\n", scenario.payment.Amount, scenario.payment.Currency)
		fmt.Printf("  Customer: %s\n", scenario.payment.CustomerEmail)
		
		// For demo purposes, we'll simulate errors by modifying the payment
		if scenario.name == "Stripe Fails → PayPal Success" {
			scenario.payment.Metadata = map[string]interface{}{"force_stripe_error": "true"}
		} else if scenario.name == "Insufficient Funds → Split Payment" {
			scenario.payment.Metadata = map[string]interface{}{"force_insufficient_funds": "true"}
		} else if scenario.name == "Network Error → Retry Success" {
			scenario.payment.Metadata = map[string]interface{}{"force_network_error": "true"}
		}
		
		// Process payment
		fmt.Printf("\n%sProcessing...%s\n", colorYellow, colorReset)
		start := time.Now()
		
		result, err := pipeline.Process(ctx, scenario.payment)
		duration := time.Since(start)
		
		if err != nil {
			fmt.Printf("\n%s❌ Payment Failed%s\n", colorRed, colorReset)
			
			// Show detailed error information
			var pipelineErr *pipz.PipelineError[payment.Payment]
			if errors.As(err, &pipelineErr) {
				fmt.Printf("  Failed at: %s (stage %d)\n", 
					pipelineErr.ProcessorName, pipelineErr.StageIndex)
			}
			
			
			fmt.Printf("  Error: %s\n", err.Error())
		} else {
			fmt.Printf("\n%s✅ Payment Successful%s\n", colorGreen, colorReset)
			fmt.Printf("  Provider: %s\n", result.Provider)
			
			// Show recovery information
			if strings.Contains(result.Provider, "split") {
				fmt.Printf("  %sNote: Payment was split into multiple transactions%s\n",
					colorYellow, colorReset)
			}
			if result.Provider != "stripe" {
				fmt.Printf("  %sNote: Used fallback provider after primary failed%s\n", 
					colorYellow, colorReset)
			}
		}
		
		fmt.Printf("  Processing Time: %s%v%s\n", colorCyan, duration, colorReset)
		
		if i < len(scenarios)-1 {
			waitForEnter()
		}
	}
	
	// Show demo statistics
	fmt.Printf("\n%s═══ DEMO SUMMARY ═══%s\n", colorCyan, colorReset)
	fmt.Printf("\nThis demo showed how pipz handles:\n")
	fmt.Printf("• Provider failover when primary payment method fails\n")
	fmt.Printf("• Automatic retries with exponential backoff\n")
	fmt.Printf("• Payment splitting for insufficient funds\n")
	fmt.Printf("• Error categorization for smart recovery\n")
	
	fmt.Printf("\n%sKey Concepts:%s\n", colorYellow, colorReset)
	fmt.Printf("• Use Fallback for provider redundancy\n")
	fmt.Printf("• Use RetryWithBackoff for transient failures\n")
	fmt.Printf("• Categorize errors to enable smart recovery strategies\n")
	fmt.Printf("• Keep payment logic composable and testable\n")
	
	return nil
}

func (p *PaymentExample) Benchmark(b *testing.B) error {
	// Use the actual benchmark from the example
	return nil
}

// createDemoPaymentPipeline creates a simplified payment pipeline for the demo
func createDemoPaymentPipeline() pipz.Chainable[payment.Payment] {
	// Mock payment processors
	stripeProcessor := pipz.Apply("stripe", func(ctx context.Context, p payment.Payment) (payment.Payment, error) {
		// Check for forced errors (demo only)
		if p.Metadata != nil && p.Metadata["force_stripe_error"] == "true" {
			return p, errors.New("stripe: card declined")
		}
		if p.Metadata != nil && p.Metadata["force_insufficient_funds"] == "true" {
			return p, errors.New("stripe: insufficient funds")
		}
		if p.Metadata != nil && p.Metadata["force_network_error"] == "true" {
			// Simulate retry behavior
			if p.RetryCount < 2 {
				p.RetryCount++
				return p, errors.New("stripe: network timeout")
			}
		}
		
		// Success
		p.Provider = "stripe"
		return p, nil
	})
	
	paypalProcessor := pipz.Apply("paypal", func(ctx context.Context, p payment.Payment) (payment.Payment, error) {
		// Check for forced errors
		if p.Metadata != nil && p.Metadata["force_insufficient_funds"] == "true" {
			return p, errors.New("paypal: insufficient funds")
		}
		
		// Success
		p.Provider = "paypal"
		return p, nil
	})
	
	// Split payment recovery
	splitPaymentProcessor := pipz.Apply("split_payment", func(ctx context.Context, p payment.Payment) (payment.Payment, error) {
		if p.Amount <= 100 {
			return p, errors.New("amount too small to split")
		}
		
		// Simulate split payment
		p.Provider = "split_payment"
		return p, nil
	})
	
	// Build the pipeline
	return pipz.Sequential(
		// Validate payment
		pipz.Effect("validate", func(ctx context.Context, p payment.Payment) error {
			if p.Amount <= 0 {
				return errors.New("invalid payment amount")
			}
			if p.CustomerEmail == "" {
				return errors.New("customer email required")
			}
			return nil
		}),
		
		// Try payment with sophisticated recovery
		pipz.Fallback(
			pipz.RetryWithBackoff(stripeProcessor, 3, 100*time.Millisecond),
			pipz.Fallback(
				paypalProcessor,
				splitPaymentProcessor,
			),
		),
		
		// Record transaction
		pipz.Effect("record", func(ctx context.Context, p payment.Payment) error {
			fmt.Printf("  %s[Transaction recorded]%s\n", colorGray, colorReset)
			return nil
		}),
	)
}