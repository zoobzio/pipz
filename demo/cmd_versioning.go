package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"pipz"
	"pipz/demo/testutil"
)

var versioningCmd = &cobra.Command{
	Use:   "versioning",
	Short: "Pipeline versioning and A/B/C testing demonstration",
	Long:  `Demonstrates how type universes naturally enable versioning and A/B/C testing.`,
	Run:   runVersioningDemo,
}

func init() {
	rootCmd.AddCommand(versioningCmd)
}

// Demo types - using different names to avoid conflicts
type VersionedPaymentKey string

type VersionedPayment struct {
	ID             string
	Amount         float64
	Currency       string
	CustomerID     string
	CustomerTier   string // basic, standard, premium
	Timestamp      time.Time
	
	// Fields populated by different versions
	Validated      bool
	ValidationMsg  string
	FraudScore     float64
	Provider       string
	NotificationSent bool
	AnalyticsTracked bool
	ProcessingTime time.Duration
	Version        string
}

// Version A processors - MVP (just charge)
func chargeCardSimple(p VersionedPayment) ([]byte, error) {
	// Simulate processing delay
	time.Sleep(10 * time.Millisecond)
	
	// Simple charge - no validation
	if p.Amount > 10000 {
		return nil, fmt.Errorf("amount too large")
	}
	
	p.Provider = "default-gateway"
	return pipz.Encode(p)
}

// Version B processors - Add validation
func validateAmount(p VersionedPayment) ([]byte, error) {
	if p.Amount <= 0 {
		return nil, fmt.Errorf("invalid amount: %.2f", p.Amount)
	}
	if p.Amount > 5000 {
		p.ValidationMsg = "large transaction flagged"
	}
	p.Validated = true
	return pipz.Encode(p)
}

func chargeCardWithValidation(p VersionedPayment) ([]byte, error) {
	time.Sleep(15 * time.Millisecond)
	
	// Use validation results
	if p.ValidationMsg != "" {
		// Extra checks for large transactions
		time.Sleep(5 * time.Millisecond)
	}
	
	p.Provider = "standard-gateway"
	return pipz.Encode(p)
}

func logPayment(p VersionedPayment) ([]byte, error) {
	// Basic logging
	return pipz.Encode(p)
}

// Version C processors - Enterprise features
func checkVelocity(p VersionedPayment) ([]byte, error) {
	// Check transaction velocity (mock)
	// In real system: check last N transactions
	time.Sleep(5 * time.Millisecond)
	return pipz.Encode(p)
}

func fraudScore(p VersionedPayment) ([]byte, error) {
	// ML-based fraud scoring (mock)
	time.Sleep(20 * time.Millisecond)
	
	// Simple mock scoring
	score := 0.1
	if p.Amount > 1000 {
		score += 0.2
	}
	if strings.Contains(p.CustomerID, "new") {
		score += 0.3
	}
	p.FraudScore = score
	
	if score > 0.7 {
		return nil, fmt.Errorf("high fraud risk: %.2f", score)
	}
	
	return pipz.Encode(p)
}

func routeProvider(p VersionedPayment) ([]byte, error) {
	// Smart routing based on amount, region, etc
	if p.Amount < 100 {
		p.Provider = "low-value-processor"
	} else if p.FraudScore > 0.5 {
		p.Provider = "high-risk-processor"
	} else {
		p.Provider = "premium-processor"
	}
	return pipz.Encode(p)
}

func chargeCardEnterprise(p VersionedPayment) ([]byte, error) {
	// Use routed provider
	time.Sleep(25 * time.Millisecond)
	return pipz.Encode(p)
}

func notify(p VersionedPayment) ([]byte, error) {
	// Multi-channel notifications
	p.NotificationSent = true
	return pipz.Encode(p)
}

func analytics(p VersionedPayment) ([]byte, error) {
	// Real-time analytics
	p.AnalyticsTracked = true
	p.ProcessingTime = time.Since(p.Timestamp)
	return pipz.Encode(p)
}

// Test group assignment (in real system: from database/feature flags)
func getTestGroup(customerID string) string {
	// Simulate customer segmentation
	if strings.Contains(customerID, "vip") || strings.Contains(customerID, "premium") {
		return "C" // Premium gets all features
	} else if strings.Contains(customerID, "standard") {
		return "B" // Standard gets validation
	}
	return "A" // Everyone else gets MVP
}

func runVersioningDemo(cmd *cobra.Command, args []string) {
	pp := testutil.NewPrettyPrinter()
	
	// Get interactive flag from parent command
	interactive, _ := cmd.Flags().GetBool("interactive")
	pp.SetInteractive(interactive)
	
	pp.Section("🔄 PIPELINE VERSIONING & A/B/C TESTING DEMO")
	
	pp.SubSection("📋 Use Case: Payment Processing Evolution")
	pp.Info("Scenario: An e-commerce platform evolving their payment system.")
	pp.Info("Challenge: Add features without breaking existing flows.")
	pp.Info("Solution: Run multiple versions simultaneously!")
	
	pp.SubSection("🎯 The Evolution Story")
	pp.Info("Version A (MVP): Just charge the card - ship fast!")
	pp.Info("Version B (v2): Add validation - learning from mistakes")
	pp.Info("Version C (Enterprise): Full feature set - scaling up")
	
	pp.SubSection("🔧 Natural Versioning Pattern")
	pp.Code("go", `// Version A: MVP - Ship it!
const versionA VersionedPaymentKey = "A"
vA := pipz.GetContract[VersionedPayment](versionA)
vA.Register(chargeCard)  // One function. That's it.

// Version B: Add safety
const versionB VersionedPaymentKey = "B"
vB := pipz.GetContract[VersionedPayment](versionB)
vB.Register(validateAmount, chargeCard, logPayment)

// Version C: Enterprise ready
const versionC VersionedPaymentKey = "C"
vC := pipz.GetContract[VersionedPayment](versionC)
vC.Register(
    validateAmount,
    checkVelocity,
    fraudScore,
    routeProvider,
    chargeCard,
    notify,
    analytics,
)

// The magic: All three coexist!
switch getTestGroup(customerID) {
    case "A": return vA.Process(payment)
    case "B": return vB.Process(payment)
    case "C": return vC.Process(payment)
}`)
	
	pp.Info("")
	pp.Info("💡 Key insight: Same VersionedPaymentKey type, different strings = different universes!")
	pp.Info("   No version managers. No migration tools. Just strings.")
	
	// Register all three versions
	pp.SubSection("Step 1: Register All Versions")
	
	// Define const keys
	const versionA VersionedPaymentKey = "A"
	const versionB VersionedPaymentKey = "B"
	const versionC VersionedPaymentKey = "C"
	
	// Version A - MVP
	vA := pipz.GetContract[VersionedPayment](versionA)
	vA.Register(chargeCardSimple)
	pp.Success("✓ Version A registered (1 processor)")
	
	// Version B - With validation
	vB := pipz.GetContract[VersionedPayment](versionB)
	vB.Register(validateAmount, chargeCardWithValidation, logPayment)
	pp.Success("✓ Version B registered (3 processors)")
	
	// Version C - Enterprise
	vC := pipz.GetContract[VersionedPayment](versionC)
	vC.Register(
		validateAmount,
		checkVelocity,
		fraudScore,
		routeProvider,
		chargeCardEnterprise,
		notify,
		analytics,
	)
	pp.Success("✓ Version C registered (7 processors)")
	
	pp.Info("")
	pp.Info("All three versions now exist simultaneously in memory.")
	pp.Info("They share NOTHING. Complete isolation.")
	
	pp.SubSection("🔍 Live A/B/C Testing")
	
	// Test payments
	testPayments := []VersionedPayment{
		{
			ID:           "PAY-001",
			Amount:       50.00,
			CustomerID:   "basic-user-123",
			CustomerTier: "basic",
			Timestamp:    time.Now(),
		},
		{
			ID:           "PAY-002", 
			Amount:       250.00,
			CustomerID:   "standard-user-456",
			CustomerTier: "standard",
			Timestamp:    time.Now(),
		},
		{
			ID:           "PAY-003",
			Amount:       2500.00,
			CustomerID:   "premium-vip-789",
			CustomerTier: "premium",
			Timestamp:    time.Now(),
		},
	}
	
	pp.Info("Processing same payments through different versions:")
	pp.Info("")
	
	for _, payment := range testPayments {
		group := getTestGroup(payment.CustomerID)
		payment.Version = group
		
		pp.Info(fmt.Sprintf("Customer: %s (Tier: %s) → Version %s", 
			payment.CustomerID, payment.CustomerTier, group))
		
		start := time.Now()
		var result VersionedPayment
		var err error
		
		switch group {
		case "A":
			result, err = vA.Process(payment)
		case "B":
			result, err = vB.Process(payment)
		case "C":
			result, err = vC.Process(payment)
		}
		
		processingTime := time.Since(start)
		
		if err != nil {
			pp.Error(fmt.Sprintf("  ✗ Failed: %v", err))
		} else {
			pp.Success(fmt.Sprintf("  ✓ Processed in %v", processingTime))
			pp.Info(fmt.Sprintf("    Provider: %s", result.Provider))
			if result.Validated {
				pp.Info("    Validated: ✓")
			}
			if result.FraudScore > 0 {
				pp.Info(fmt.Sprintf("    Fraud Score: %.2f", result.FraudScore))
			}
			if result.NotificationSent {
				pp.Info("    Notification: ✓")
			}
			if result.AnalyticsTracked {
				pp.Info("    Analytics: ✓")
			}
		}
		pp.Info("")
	}
	
	pp.WaitForEnter("")
	
	pp.SubSection("🔬 Side-by-Side Comparison")
	pp.Info("Let's process the SAME payment through ALL versions:")
	pp.Info("")
	
	comparePayment := VersionedPayment{
		ID:         "PAY-COMPARE",
		Amount:     1000.00,
		CustomerID: "test-user",
		Timestamp:  time.Now(),
	}
	
	// Process through all versions
	versions := []struct {
		name     string
		contract *pipz.Contract[VersionedPayment, VersionedPaymentKey]
	}{
		{"A (MVP)", vA},
		{"B (Standard)", vB},
		{"C (Enterprise)", vC},
	}
	
	for _, v := range versions {
		payment := comparePayment // Copy
		payment.Version = v.name
		
		start := time.Now()
		result, err := v.contract.Process(payment)
		duration := time.Since(start)
		
		pp.Info(fmt.Sprintf("Version %s:", v.name))
		if err != nil {
			pp.Error(fmt.Sprintf("  Failed: %v", err))
		} else {
			pp.Success(fmt.Sprintf("  Success in %v", duration))
			pp.Stats("  Features", map[string]interface{}{
				"Provider":     result.Provider,
				"Validated":    result.Validated,
				"Fraud Scored": result.FraudScore > 0,
				"Notified":     result.NotificationSent,
				"Analytics":    result.AnalyticsTracked,
			})
		}
	}
	
	pp.SubSection("🚀 Real-World Patterns")
	
	pp.Feature("📊", "Canary Deployment", "if rand.Float64() < 0.1 { useV2() }")
	pp.Feature("🎯", "Customer Segmentation", "Premium users get version C")
	pp.Feature("🔄", "Gradual Rollout", "Increase percentage over time")
	pp.Feature("⚡", "Instant Rollback", "Just change the string")
	pp.Feature("🧪", "Feature Testing", "Try risky features in isolation")
	
	pp.SubSection("The Power of Simplicity")
	pp.Info("• No version management framework")
	pp.Info("• No database migrations")
	pp.Info("• No configuration files")
	pp.Info("• No deployment complexity")
	pp.Info("")
	pp.Info("Just different strings creating different universes.")
	pp.Info("Each version is completely isolated.")
	pp.Info("Teams can work independently.")
	pp.Info("Evolution without risk.")
	
	pp.SubSection("💡 Extrapolate the Pattern")
	
	pp.Info("This same pattern works for:")
	pp.Info("  • API versions (v1, v2, v3...)")
	pp.Info("  • Regional variations (US, EU, APAC...)")
	pp.Info("  • Customer tiers (free, pro, enterprise...)")
	pp.Info("  • Experimental features (stable, beta, alpha...)")
	pp.Info("  • Time-based (black-friday, normal, holiday...)")
	pp.Info("")
	pp.Info("The possibilities are infinite because isolation is perfect.")
	pp.Info("Each universe knows nothing about the others.")
	
	pp.SubSection("🔧 Advanced: Feature Composition")
	pp.Code("go", `// You can even compose features from different versions!
const hybridKey VersionedPaymentKey = "hybrid"
hybridContract := pipz.GetContract[VersionedPayment](hybridKey)
hybridContract.Register(
    validateAmount,      // From version B
    fraudScore,         // From version C
    chargeCardSimple,   // From version A
)

// Or create specialized versions for specific scenarios
const blackFridayKey VersionedPaymentKey = "black-friday"
blackFridayContract := pipz.GetContract[VersionedPayment](blackFridayKey)
blackFridayContract.Register(
    skipValidation,     // Special rules for high volume
    batchProcess,       // Optimize for throughput
    deferredNotify,     // Send notifications later
)`)
	
	pp.Stats("Version Isolation Metrics", map[string]interface{}{
		"Versions Running": 3,
		"Shared State": "0%",
		"Deployment Risk": "Zero",
		"Rollback Time": "Instant",
		"Team Coupling": "None",
	})
}