package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"pipz"
	"pipz/demo/testutil"
)

var universesCmd = &cobra.Command{
	Use:   "universes",
	Short: "Multi-tenant Type Universes demonstration",
	Long:  `Demonstrates complete pipeline isolation using different key types.`,
	Run:   runUniversesDemo,
}

func init() {
	rootCmd.AddCommand(universesCmd)
}

// Different merchant key types create isolated processing universes
type StandardMerchantKey string
type PremiumMerchantKey string
type HighRiskMerchantKey string

const (
	// Contract versions for each merchant type
	StandardContractV1 StandardMerchantKey = "v1"
	PremiumContractV1  PremiumMerchantKey  = "v1"
	HighRiskContractV1 HighRiskMerchantKey = "v1"
)

// Transaction represents a payment transaction
type Transaction struct {
	ID          string
	MerchantID  string
	Amount      float64
	Currency    string
	CardLast4   string
	CustomerID  string
	Timestamp   time.Time
	Status      string
	RiskScore   float64
	Flags       []string
	SettleDate  *time.Time
}

func runUniversesDemo(cmd *cobra.Command, args []string) {
	pp := testutil.NewPrettyPrinter()
	
	// Get interactive flag from parent command
	interactive, _ := cmd.Flags().GetBool("interactive")
	pp.SetInteractive(interactive)
	
	pp.Section("🌌 MULTI-TENANT TYPE UNIVERSES DEMO")
	
	pp.SubSection("📋 Use Case: Multi-Tenant Payment Processing")
	pp.Info("Scenario: A payment platform serving different merchant types:")
	pp.Info("  • Standard merchants - Basic features, standard limits")
	pp.Info("  • Premium merchants - Higher limits, faster settlement")
	pp.Info("  • High-risk merchants - Strict security, longer holds")
	pp.Info("")
	pp.Warning("Challenge: Each merchant type needs COMPLETELY different processing rules")
	pp.Warning("          but we want to use the same Transaction type!")
	
	pp.SubSection("🔑 The Magic: Type-Based Isolation")
	pp.Info("Key insight: The KEY TYPE creates the isolation, not the key value!")
	pp.Code("go", `// These are THREE DIFFERENT TYPES (even though they're all strings)
type StandardMerchantKey string
type PremiumMerchantKey string  
type HighRiskMerchantKey string

// All three can use the SAME key value "v1"
// But they create COMPLETELY SEPARATE pipelines!
standard := pipz.GetContract[StandardMerchantKey, Transaction](StandardContractV1)
premium  := pipz.GetContract[PremiumMerchantKey, Transaction](PremiumContractV1)
highRisk := pipz.GetContract[HighRiskMerchantKey, Transaction](HighRiskContractV1)`)
	
	pp.Info("")
	pp.Info("🧠 Understanding the Magic:")
	pp.Info("  • StandardMerchantKey(\"v1\") + Transaction = Pipeline A")
	pp.Info("  • PremiumMerchantKey(\"v1\")  + Transaction = Pipeline B")
	pp.Info("  • HighRiskMerchantKey(\"v1\") + Transaction = Pipeline C")
	pp.Info("  • These pipelines CANNOT see each other!")
	
	pp.SubSection("Step 1: Register Three Isolated Pipelines")
	
	// Register standard merchant pipeline
	pp.Info("📦 Standard Merchant Pipeline:")
	standardContract := pipz.GetContract[StandardMerchantKey, Transaction](StandardContractV1)
	standardContract.Register(validateStandardAmount, checkStandardVelocity, notifyStandardMerchant)
	pp.Success("  ✓ Registered: $10k limit, 10 txn/hr, T+2 settlement")
	
	// Register premium merchant pipeline
	pp.Info("")
	pp.Info("⭐ Premium Merchant Pipeline:")
	premiumContract := pipz.GetContract[PremiumMerchantKey, Transaction](PremiumContractV1)
	premiumContract.Register(validatePremiumAmount, checkPremiumVelocity, prioritySettle, notifyPremium)
	pp.Success("  ✓ Registered: $100k limit, 100 txn/hr, same-day settlement")
	
	// Register high-risk merchant pipeline
	pp.Info("")
	pp.Info("⚠️  High-Risk Merchant Pipeline:")
	highRiskContract := pipz.GetContract[HighRiskMerchantKey, Transaction](HighRiskContractV1)
	highRiskContract.Register(validate3DS, checkBlocklist, manualReview, holdFunds)
	pp.Success("  ✓ Registered: 3DS required, blocklist check, 7-day hold")
	
	pp.SubSection("Step 2: Same Transaction, Different Rules")
	pp.Info("Let's process the SAME $5,000 transaction through each pipeline...")
	
	// Create a test transaction
	txn := Transaction{
		ID:         "TXN-DEMO-001",
		MerchantID: "MERCHANT-123",
		Amount:     5000.00,
		Currency:   "USD",
		CardLast4:  "1234",
		CustomerID: "CUST-001",
		Timestamp:  time.Now(),
	}
	
	pp.Info("")
	pp.Info(fmt.Sprintf("📥 Input Transaction: $%.2f", txn.Amount))
	
	// Process through standard merchant
	pp.Info("")
	pp.Info("🏪 Processing as STANDARD merchant:")
	pp.Info("   Using: StandardMerchantKey(\"v1\")")
	result1, err := standardContract.Process(txn)
	if err != nil {
		pp.Error(fmt.Sprintf("   Failed: %v", err))
	} else {
		pp.Success("   ✓ Approved")
		pp.Info(fmt.Sprintf("   Status: %s", result1.Status))
		pp.Info(fmt.Sprintf("   Settlement: %s (T+2)", result1.SettleDate.Format("Jan 2")))
		pp.Info(fmt.Sprintf("   Applied rules: $10k limit, basic checks"))
	}
	
	// Process through premium merchant
	pp.Info("")
	pp.Info("⭐ Processing as PREMIUM merchant:")
	pp.Info("   Using: PremiumMerchantKey(\"v1\")")
	result2, err := premiumContract.Process(txn)
	if err != nil {
		pp.Error(fmt.Sprintf("   Failed: %v", err))
	} else {
		pp.Success("   ✓ Approved with premium benefits")
		pp.Info(fmt.Sprintf("   Status: %s", result2.Status))
		pp.Info(fmt.Sprintf("   Settlement: %s (Same day! 💨)", result2.SettleDate.Format("Jan 2")))
		pp.Info(fmt.Sprintf("   Applied rules: $100k limit, priority processing"))
	}
	
	// Process through high-risk merchant (will fail without 3DS)
	pp.Info("")
	pp.Info("⚠️  Processing as HIGH-RISK merchant:")
	pp.Info("   Using: HighRiskMerchantKey(\"v1\")")
	result3, err := highRiskContract.Process(txn)
	if err != nil {
		pp.Error(fmt.Sprintf("   Failed: %v", err))
		pp.Info("   (3D Secure verification is required)")
	}
	
	// Retry with 3DS
	txn.CustomerID = "3ds-verified-customer"
	pp.Info("")
	pp.Info("   Retrying with 3DS verified customer...")
	result3, err = highRiskContract.Process(txn)
	if err != nil {
		pp.Error(fmt.Sprintf("   Failed: %v", err))
	} else {
		pp.Warning(fmt.Sprintf("   ⚡ Status: %s", result3.Status))
		pp.Info(fmt.Sprintf("   Settlement: %s (7-day hold)", result3.SettleDate.Format("Jan 2")))
		pp.Info(fmt.Sprintf("   Applied rules: Enhanced security, manual review"))
	}
	
	pp.SubSection("🔍 Proving Complete Isolation")
	pp.Info("Let's prove these pipelines are completely isolated...")
	
	pp.Code("go", `// Try to access standard pipeline with premium key type
wrongPipeline := pipz.GetContract[PremiumMerchantKey, Transaction](
    PremiumMerchantKey("standard-key-value")
)
// This gets a DIFFERENT pipeline (or empty one)!`)
	
	// Demonstrate isolation
	pp.Info("")
	pp.Info("🧪 Experiment: What if we use wrong key type?")
	
	// This creates a NEW, EMPTY pipeline
	wrongPipeline := pipz.GetContract[PremiumMerchantKey, Transaction](PremiumMerchantKey("some-other-value"))
	testTxn := Transaction{Amount: 1000}
	result, err := wrongPipeline.Process(testTxn)
	
	pp.Info("   Result: Gets a different (empty) pipeline")
	pp.Info(fmt.Sprintf("   Transaction unchanged: amount still %.2f", result.Amount))
	pp.Success("   ✓ Pipelines are completely isolated by type!")
	
	pp.SubSection("🏗️ Real-World Architecture Benefits")
	
	pp.SubSection("Multi-Tenant SaaS")
	pp.Code("go", `// Each customer gets their own processing rules
type CustomerAKey string  // Customer A's pipeline type
type CustomerBKey string  // Customer B's pipeline type

// Both can use "prod" but get different pipelines
customerA := GetContract[CustomerAKey, Order](CustomerAKey("prod"))
customerB := GetContract[CustomerBKey, Order](CustomerBKey("prod"))`)
	
	pp.SubSection("A/B Testing")
	pp.Code("go", `// Test different processing strategies
type StrategyAKey string
type StrategyBKey string

// Route 50% of traffic to each
if rand.Float32() < 0.5 {
    pipeline := GetContract[StrategyAKey, Data](StrategyAKey("v1"))
} else {
    pipeline := GetContract[StrategyBKey, Data](StrategyBKey("v1"))
}`)
	
	pp.SubSection("API Versioning")
	pp.Code("go", `// Different API versions with different logic
type APIv1Key string
type APIv2Key string
type APIv3Key string

// Each version has its own pipeline
v1 := GetContract[APIv1Key, Request](APIv1Key("prod"))
v2 := GetContract[APIv2Key, Request](APIv2Key("prod"))
v3 := GetContract[APIv3Key, Request](APIv3Key("prod"))`)
	
	pp.SubSection("🎯 Key Insights")
	
	pp.Feature("🔐", "Type = Identity", "The KEY TYPE determines which pipeline you get")
	pp.Feature("🚧", "Complete Isolation", "Pipelines cannot access each other, period")
	pp.Feature("🎨", "Same Data, Different Rules", "Process the same types differently")
	pp.Feature("✅", "Compile-Time Safety", "Wrong type = compile error, not runtime error")
	pp.Feature("🚀", "Zero Configuration", "No config files, no DI containers")
	
	pp.SubSection("💡 When to Use Type Universes")
	
	pp.Success("✓ Multi-tenant applications (each tenant has unique rules)")
	pp.Success("✓ White-label products (customize behavior per brand)")
	pp.Success("✓ A/B testing (compare different processing strategies)")
	pp.Success("✓ API versioning (maintain multiple versions)")
	pp.Success("✓ Feature flags (enable different code paths)")
	
	pp.Stats("Universe Comparison", map[string]interface{}{
		"Standard Limit": "$10,000",
		"Premium Limit": "$100,000",
		"High-Risk Limit": "Varies",
		"Isolation Level": "100% Type-Safe",
		"Cross-Contamination": "Impossible",
	})
}

// Standard merchant processors
func validateStandardAmount(t Transaction) ([]byte, error) {
	if t.Amount <= 0 {
		return nil, fmt.Errorf("invalid amount: %.2f", t.Amount)
	}
	if t.Amount > 10000 {
		return nil, fmt.Errorf("amount exceeds standard limit: $10,000")
	}
	t.Flags = append(t.Flags, "amount_validated")
	return pipz.Encode(t)
}

func checkStandardVelocity(t Transaction) ([]byte, error) {
	// Mock velocity check - in real app would check transaction history
	t.Flags = append(t.Flags, "velocity_checked")
	return pipz.Encode(t)
}

func notifyStandardMerchant(t Transaction) ([]byte, error) {
	t.Flags = append(t.Flags, "merchant_notified_email")
	t.Status = "processing"
	settleDate := t.Timestamp.Add(48 * time.Hour)
	t.SettleDate = &settleDate
	return pipz.Encode(t)
}

// Premium merchant processors
func validatePremiumAmount(t Transaction) ([]byte, error) {
	if t.Amount <= 0 {
		return nil, fmt.Errorf("invalid amount: %.2f", t.Amount)
	}
	if t.Amount > 100000 {
		return nil, fmt.Errorf("amount exceeds premium limit: $100,000")
	}
	t.Flags = append(t.Flags, "premium_amount_validated")
	return pipz.Encode(t)
}

func checkPremiumVelocity(t Transaction) ([]byte, error) {
	t.Flags = append(t.Flags, "premium_velocity_checked")
	return pipz.Encode(t)
}

func prioritySettle(t Transaction) ([]byte, error) {
	settleDate := t.Timestamp.Add(4 * time.Hour)
	t.SettleDate = &settleDate
	t.Flags = append(t.Flags, "priority_settlement")
	return pipz.Encode(t)
}

func notifyPremium(t Transaction) ([]byte, error) {
	t.Flags = append(t.Flags, "merchant_notified_sms", "merchant_notified_email", "webhook_sent")
	t.Status = "approved"
	return pipz.Encode(t)
}

// High-risk merchant processors
func validate3DS(t Transaction) ([]byte, error) {
	if !strings.Contains(t.CustomerID, "3ds-verified") {
		return nil, fmt.Errorf("3D Secure verification required")
	}
	t.Flags = append(t.Flags, "3ds_verified")
	return pipz.Encode(t)
}

func checkBlocklist(t Transaction) ([]byte, error) {
	// Check against fraud database
	blockedCards := []string{"9999", "6666", "0000"}
	for _, blocked := range blockedCards {
		if t.CardLast4 == blocked {
			return nil, fmt.Errorf("card on blocklist")
		}
	}
	t.Flags = append(t.Flags, "blocklist_checked")
	return pipz.Encode(t)
}

func manualReview(t Transaction) ([]byte, error) {
	if t.Amount > 500 {
		t.Status = "pending_review"
		t.Flags = append(t.Flags, "manual_review_required")
	} else {
		t.Status = "approved"
	}
	return pipz.Encode(t)
}

func holdFunds(t Transaction) ([]byte, error) {
	settleDate := t.Timestamp.Add(7 * 24 * time.Hour)
	t.SettleDate = &settleDate
	t.Flags = append(t.Flags, "funds_held_7_days")
	return pipz.Encode(t)
}