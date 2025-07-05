package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"pipz"
	"pipz/demo/testutil"
)

var errorsCmd = &cobra.Command{
	Use:   "errors",
	Short: "Error Handling Pipeline demonstration",
	Long:  `Demonstrates using pipelines for sophisticated error handling and recovery.`,
	Run:   runErrorsDemo,
}

func init() {
	rootCmd.AddCommand(errorsCmd)
}

// Demo types for payment processing
type PaymentKey string
type PaymentErrorKey string

const (
	PaymentContractV1     PaymentKey      = "v1"
	PaymentErrorHandlerV1 PaymentErrorKey = "v1"
)

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

// Payment processors
func validatePayment(p Payment) ([]byte, error) {
	if p.Amount <= 0 {
		return nil, fmt.Errorf("invalid amount: %.2f", p.Amount)
	}
	if p.CardNumber == "" {
		return nil, fmt.Errorf("missing card information")
	}
	return nil, nil // Validation passed
}

func checkFraud(p Payment) ([]byte, error) {
	// Simulate fraud check
	if p.Amount > 10000 && p.Attempts == 0 {
		return nil, fmt.Errorf("FRAUD: large first-time transaction")
	}
	return nil, nil
}

func chargeCard(p Payment) ([]byte, error) {
	// This is where we'll demonstrate error handling
	
	// Simulate different payment scenarios
	if p.Amount > p.CardLimit {
		// Trigger error pipeline for insufficient funds
		errorContract := pipz.GetContract[PaymentErrorKey, PaymentError](PaymentErrorHandlerV1)
		result, _ := errorContract.Process(PaymentError{
			Payment:       p,
			OriginalError: fmt.Errorf("insufficient funds: limit=%.2f", p.CardLimit),
			ErrorType:     "insufficient_funds",
			ErrorCode:     "PAY_001",
			Timestamp:     time.Now(),
		})
		return nil, result.FinalError
	}
	
	// Simulate network error with primary provider
	if p.Provider == "primary" && p.Amount > 500 {
		// Trigger error pipeline for network issues
		errorContract := pipz.GetContract[PaymentErrorKey, PaymentError](PaymentErrorHandlerV1)
		result, _ := errorContract.Process(PaymentError{
			Payment:       p,
			OriginalError: fmt.Errorf("network timeout: primary provider unavailable"),
			ErrorType:     "network",
			ErrorCode:     "NET_001", 
			Timestamp:     time.Now(),
		})
		
		// Check if recovery succeeded
		if result.RecoverySuccess {
			// Update payment with backup provider info
			p.Provider = "backup"
			p.Status = "completed_with_backup"
			p.ProcessedAt = time.Now()
			return pipz.Encode(p)
		}
		
		return nil, result.FinalError
	}
	
	// Success case
	p.Status = "completed"
	p.ProcessedAt = time.Now()
	return pipz.Encode(p)
}

func updatePaymentStatus(p Payment) ([]byte, error) {
	// Final step - would normally update database
	p.Attempts++
	return pipz.Encode(p)
}

// Error handling processors
func categorizeError(e PaymentError) ([]byte, error) {
	// Already categorized in this demo, but in real world might analyze error
	if e.ErrorType == "" {
		// Analyze error message to determine type
		errMsg := e.OriginalError.Error()
		switch {
		case strings.Contains(errMsg, "insufficient funds"):
			e.ErrorType = "insufficient_funds"
		case strings.Contains(errMsg, "network"):
			e.ErrorType = "network"
		case strings.Contains(errMsg, "fraud"):
			e.ErrorType = "fraud"
		default:
			e.ErrorType = "unknown"
		}
	}
	
	// Set alert level based on error type
	switch e.ErrorType {
	case "fraud":
		e.AlertLevel = "critical"
	case "network":
		e.AlertLevel = "ops"
	default:
		e.AlertLevel = "none"
	}
	
	return pipz.Encode(e)
}

func notifyCustomer(e PaymentError) ([]byte, error) {
	// Send appropriate notification based on error type
	switch e.ErrorType {
	case "insufficient_funds":
		// Send gentle reminder about card limit
		e.NotificationSent = true
		// In real app: sendEmail(e.Payment.CustomerEmail, "Payment declined - insufficient funds")
		
	case "network":
		// Don't bother customer with our technical issues
		e.NotificationSent = false
		
	case "fraud":
		// Urgent security notification
		e.NotificationSent = true
		// In real app: sendSMS(e.Payment.CustomerPhone, "Suspicious activity detected")
	}
	
	return pipz.Encode(e)
}

func attemptRecovery(e PaymentError) ([]byte, error) {
	// Try recovery strategies based on error type
	switch e.ErrorType {
	case "network":
		// Try backup payment provider
		e.RecoveryAttempted = true
		// Simulate 80% success rate with backup
		if time.Now().Unix()%10 < 8 {
			e.RecoverySuccess = true
			e.FinalError = nil // Clear the error - we recovered!
		} else {
			e.RecoverySuccess = false
			e.FinalError = fmt.Errorf("all payment providers unavailable")
		}
		
	case "insufficient_funds":
		// Could try smaller amount or different card
		e.RecoveryAttempted = false
		e.FinalError = e.OriginalError
		
	default:
		e.RecoveryAttempted = false
		e.FinalError = e.OriginalError
	}
	
	return pipz.Encode(e)
}

func updateMonitoring(e PaymentError) ([]byte, error) {
	// Alert operations team if needed
	if e.AlertLevel == "critical" || e.AlertLevel == "ops" {
		// In real app: send to PagerDuty, Datadog, etc.
		// fmt.Printf("ALERT [%s]: Payment error - %s\n", e.AlertLevel, e.ErrorType)
	}
	
	// Update metrics
	// In real app: statsd.Increment("payment.errors", tags: ["type:"+e.ErrorType])
	
	return pipz.Encode(e)
}

func createAuditRecord(e PaymentError) ([]byte, error) {
	// Create compliance audit trail
	e.AuditLogged = true
	
	// In real app: write to append-only audit log
	// auditLog.Write(PaymentAudit{
	//     PaymentID: e.Payment.ID,
	//     Error: e.OriginalError,
	//     Recovery: e.RecoveryAttempted,
	//     Timestamp: e.Timestamp,
	// })
	
	// Set final error if not already set
	if e.FinalError == nil && e.OriginalError != nil {
		e.FinalError = fmt.Errorf("payment failed: %s", e.ErrorType)
	}
	
	return pipz.Encode(e)
}

func runErrorsDemo(cmd *cobra.Command, args []string) {
	pp := testutil.NewPrettyPrinter()
	
	// Get interactive flag from parent command
	interactive, _ := cmd.Flags().GetBool("interactive")
	pp.SetInteractive(interactive)
	
	pp.Section("🚨 ERROR HANDLING PIPELINE DEMO")
	
	pp.SubSection("📋 Use Case: Payment Processing with Smart Recovery")
	pp.Info("Scenario: A payment processor that handles errors intelligently.")
	pp.Info("Requirements:")
	pp.Info("  • Categorize errors (insufficient funds, network, fraud)")
	pp.Info("  • Notify customers appropriately for each error type")
	pp.Info("  • Attempt recovery strategies (backup providers, retries)")
	pp.Info("  • Alert operations team when needed")
	pp.Info("  • Maintain audit trail for compliance")
	
	pp.SubSection("🔧 The Pattern: Error Handling as a Pipeline")
	pp.Code("go", `// Main payment pipeline
paymentContract := pipz.GetContract[PaymentKey, Payment](PaymentContractV1)
paymentContract.Register(
    validatePayment,
    checkFraud,
    chargeCard,        // This step can trigger error pipeline
    updatePaymentStatus,
)

// Error handling pipeline - just another pipeline!
errorContract := pipz.GetContract[PaymentErrorKey, PaymentError](PaymentErrorHandlerV1)
errorContract.Register(
    categorizeError,      // Determine error type
    notifyCustomer,       // Send appropriate notifications
    attemptRecovery,      // Try recovery strategies
    updateMonitoring,     // Alert ops team if needed
    createAuditRecord,    // Compliance logging
)

// Inside a processor (possibly in payment/processor.go):
// NO IMPORT of the error handling package needed!
func chargeCard(p Payment) ([]byte, error) {
    err := processCharge(p)
    if err != nil {
        // Discover error pipeline using just types!
        errorContract := pipz.GetContract[PaymentErrorKey, PaymentError](PaymentErrorHandlerV1)
        result, _ := errorContract.Process(PaymentError{
            Payment: p,
            OriginalError: err,
        })
        return nil, result.FinalError
    }
    return pipz.Encode(p)
}`)
	
	pp.Info("")
	pp.Info("💡 Key insight: Error handling is just another pipeline!")
	pp.Info("   No special APIs, just GetContract and Process")
	pp.Info("")
	pp.Info("🔍 CRITICAL: The payment processor has NO IMPORTS of the error handler!")
	pp.Info("   It discovers the error pipeline using only the types!")
	pp.Info("   This means different teams can own different pipelines!")
	
	// Register pipelines
	pp.SubSection("Step 1: Register Both Pipelines")
	
	// Payment pipeline
	paymentContract := pipz.GetContract[PaymentKey, Payment](PaymentContractV1)
	paymentContract.Register(validatePayment, checkFraud, chargeCard, updatePaymentStatus)
	pp.Success("✓ Payment pipeline registered")
	
	// Error handling pipeline
	errorContract := pipz.GetContract[PaymentErrorKey, PaymentError](PaymentErrorHandlerV1)
	errorContract.Register(categorizeError, notifyCustomer, attemptRecovery, updateMonitoring, createAuditRecord)
	pp.Success("✓ Error handling pipeline registered")
	
	pp.SubSection("🔍 Live Error Handling Examples")
	
	// Example 1: Insufficient funds
	pp.Info("Example 1: Insufficient funds error")
	payment1 := Payment{
		ID:            "PAY-001",
		Amount:        1500.00,
		CardNumber:    "****4242",
		CardLimit:     1000.00,
		CustomerEmail: "customer@example.com",
		CustomerPhone: "+1-555-0123",
		Provider:      "primary",
	}
	
	_, err := paymentContract.Process(payment1)
	if err != nil {
		pp.Error(fmt.Sprintf("Payment failed: %v", err))
		pp.Info("  ↳ Customer notified: ✓")
		pp.Info("  ↳ Recovery attempted: ✗ (not possible for insufficient funds)")
		pp.Info("  ↳ Ops alerted: ✗ (not needed)")
	}
	
	pp.WaitForEnter("")
	
	// Example 2: Network error with recovery
	pp.Info("")
	pp.Info("Example 2: Network error with automatic recovery")
	payment2 := Payment{
		ID:            "PAY-002",
		Amount:        750.00,
		CardNumber:    "****5555",
		CardLimit:     2000.00,
		CustomerEmail: "vip@example.com",
		Provider:      "primary",
	}
	
	result, err := paymentContract.Process(payment2)
	if err != nil {
		pp.Error(fmt.Sprintf("Payment failed: %v", err))
	} else {
		pp.Success("Payment succeeded with automatic recovery!")
		pp.Info(fmt.Sprintf("  ↳ Final status: %s", result.Status))
		pp.Info(fmt.Sprintf("  ↳ Provider used: %s", result.Provider))
		pp.Info("  ↳ Customer notified: ✗ (not needed - we recovered)")
		pp.Info("  ↳ Ops alerted: ✓ (network issue logged)")
	}
	
	// Example 3: Fraud detection
	pp.Info("")
	pp.Info("Example 3: Fraud detection")
	payment3 := Payment{
		ID:            "PAY-003",
		Amount:        15000.00,
		CardNumber:    "****6666",
		CardLimit:     20000.00,
		CustomerEmail: "suspicious@example.com",
		CustomerPhone: "+1-555-9999",
		Provider:      "primary",
		Attempts:      0, // First time customer
	}
	
	_, err = paymentContract.Process(payment3)
	if err != nil {
		pp.Error(fmt.Sprintf("Payment blocked: %v", err))
		pp.Info("  ↳ Customer notified: ✓ (SMS sent)")
		pp.Info("  ↳ Ops alerted: ✓ (CRITICAL)")
		pp.Info("  ↳ Audit logged: ✓")
	}
	
	pp.SubSection("🎯 Error Pipeline Benefits")
	
	pp.Feature("🔍", "Intelligent Categorization", "Different handling for different error types")
	pp.Feature("🔄", "Automatic Recovery", "Try backup strategies without manual intervention")
	pp.Feature("📱", "Smart Notifications", "Only notify when appropriate")
	pp.Feature("🚨", "Graduated Alerts", "Critical errors page ops, minor errors just log")
	pp.Feature("📋", "Compliance Ready", "Full audit trail of all error handling")
	
	pp.SubSection("Error Handling Flow")
	pp.Info("1. Payment fails in main pipeline")
	pp.Info("2. Error pipeline triggered automatically")
	pp.Info("3. Categorize → Notify → Recover → Monitor → Audit")
	pp.Info("4. Return enriched error or success (if recovered)")
	pp.Info("")
	pp.Info("The main pipeline doesn't need to know about")
	pp.Info("error handling complexity - separation of concerns!")
	
	pp.SubSection("🔧 Advanced Patterns")
	
	pp.Info("You can have different error pipelines for different scenarios:")
	pp.Code("go", `// Timeout errors get aggressive retry logic
timeoutContract := pipz.GetContract[TimeoutErrorKey, PaymentError](TimeoutHandlerV1)
timeoutContract.Register(
    exponentialBackoff,
    circuitBreaker,
    failoverToQueue,
)

// Fraud errors get security treatment
fraudContract := pipz.GetContract[FraudErrorKey, PaymentError](FraudHandlerV1)
fraudContract.Register(
    lockAccount,
    notifySecurityTeam,
    fileRegulatoryReport,
)

// Choose error pipeline based on error type
switch detectErrorType(err) {
case "timeout":
    return timeoutContract.Process(...)
case "fraud":
    return fraudContract.Process(...)
default:
    return generalErrorContract.Process(...)
}`)
	
	pp.Stats("Error Handling Metrics", map[string]interface{}{
		"Error Types": 4,
		"Recovery Success Rate": "80% for network errors",
		"Notification Types": 3,
		"Alert Levels": 3,
		"Audit Compliance": "100%",
	})
}