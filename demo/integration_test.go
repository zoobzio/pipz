package main_test

import (
	"fmt"
	"testing"

	"pipz"
	"pipz/demo/processors"
)

// TestTypeUniverseIsolation verifies that different key types create isolated pipelines
func TestTypeUniverseIsolation(t *testing.T) {
	// Define different key types for the same data
	type StandardKey string
	type PremiumKey string
	type TestKey string

	// Create three different pipelines with the same key value
	standardContract := pipz.GetContract[StandardKey, processors.Payment](StandardKey("v1"))
	premiumContract := pipz.GetContract[PremiumKey, processors.Payment](PremiumKey("v1"))
	testContract := pipz.GetContract[TestKey, processors.Payment](TestKey("v1"))

	// Register different processors for each
	var standardProcessed, premiumProcessed, testProcessed bool

	standardContract.Register(func(p processors.Payment) ([]byte, error) {
		standardProcessed = true
		p.Provider = "standard"
		return pipz.Encode(p)
	})

	premiumContract.Register(func(p processors.Payment) ([]byte, error) {
		premiumProcessed = true
		p.Provider = "premium"
		return pipz.Encode(p)
	})

	testContract.Register(func(p processors.Payment) ([]byte, error) {
		testProcessed = true
		p.Provider = "test"
		return pipz.Encode(p)
	})

	// Process the same payment through each pipeline
	payment := processors.Payment{
		ID:     "PAY-001",
		Amount: 100.00,
	}

	// Standard pipeline
	result1, err := standardContract.Process(payment)
	if err != nil {
		t.Fatalf("Standard pipeline failed: %v", err)
	}
	if result1.Provider != "standard" {
		t.Errorf("Wrong provider: %s", result1.Provider)
	}
	if !standardProcessed || premiumProcessed || testProcessed {
		t.Error("Pipeline isolation violated")
	}

	// Reset flags
	standardProcessed, premiumProcessed, testProcessed = false, false, false

	// Premium pipeline
	result2, err := premiumContract.Process(payment)
	if err != nil {
		t.Fatalf("Premium pipeline failed: %v", err)
	}
	if result2.Provider != "premium" {
		t.Errorf("Wrong provider: %s", result2.Provider)
	}
	if standardProcessed || !premiumProcessed || testProcessed {
		t.Error("Pipeline isolation violated")
	}

	// Reset flags
	standardProcessed, premiumProcessed, testProcessed = false, false, false

	// Test pipeline
	result3, err := testContract.Process(payment)
	if err != nil {
		t.Fatalf("Test pipeline failed: %v", err)
	}
	if result3.Provider != "test" {
		t.Errorf("Wrong provider: %s", result3.Provider)
	}
	if standardProcessed || premiumProcessed || !testProcessed {
		t.Error("Pipeline isolation violated")
	}
}

// TestPipelineVersioning demonstrates A/B testing pattern
func TestPipelineVersioning(t *testing.T) {
	type PaymentKey string

	// Register v1 pipeline - simple validation
	v1 := pipz.GetContract[PaymentKey, processors.Payment](PaymentKey("v1"))
	v1.Register(
		processors.Adapt(processors.ValidatePayment),
	)

	// Register v2 pipeline - validation + fraud check
	v2 := pipz.GetContract[PaymentKey, processors.Payment](PaymentKey("v2"))
	v2.Register(
		processors.Adapt(processors.ValidatePayment),
		processors.Adapt(processors.CheckFraud),
	)

	// Test payment that passes v1 but fails v2
	payment := processors.Payment{
		ID:         "PAY-TEST",
		Amount:     15000.00, // Large amount
		CardNumber: "****4242",
		Attempts:   0, // First time - will trigger fraud in v2
	}

	// Should pass v1 (no fraud check)
	_, err := v1.Process(payment)
	if err != nil {
		t.Fatalf("V1 pipeline failed: %v", err)
	}

	// Should fail v2 (fraud check)
	_, err = v2.Process(payment)
	if err == nil {
		t.Fatal("V2 pipeline should have detected fraud")
	}
}

// TestChainComposition tests combining multiple contracts
func TestChainComposition(t *testing.T) {
	// Create individual contracts
	validationContract := pipz.GetContract[processors.ValidatorKey, processors.Order](processors.ValidatorKey("test"))
	validationContract.Register(
		processors.Adapt(processors.ValidateOrderID),
		processors.Adapt(processors.ValidateItems),
	)

	// Create a second contract for enrichment
	type EnrichmentKey string
	enrichmentContract := pipz.GetContract[EnrichmentKey, processors.Order](EnrichmentKey("test"))
	enrichmentContract.Register(func(o processors.Order) ([]byte, error) {
		// Add some metadata
		if o.CustomerID == "" {
			o.CustomerID = "GUEST"
		}
		return pipz.Encode(o)
	})

	// Create a chain
	chain := pipz.NewChain[processors.Order]()
	chain.Add(
		validationContract.Link(),
		enrichmentContract.Link(),
	)

	// Test the chain
	order := processors.Order{
		ID: "ORD-123",
		Items: []processors.OrderItem{
			{ProductID: "P1", Quantity: 1, Price: 10.00},
		},
		Total: 10.00,
	}

	result, err := chain.Process(order)
	if err != nil {
		t.Fatalf("Chain processing failed: %v", err)
	}

	// Verify both contracts were applied
	if result.CustomerID != "GUEST" {
		t.Errorf("Enrichment not applied: %s", result.CustomerID)
	}
}

// TestConcurrentPipelines verifies thread safety
func TestConcurrentPipelines(t *testing.T) {
	const key processors.PaymentKey = "concurrent"
	contract := pipz.GetContract[processors.PaymentKey, processors.Payment](key)

	var counter int
	contract.Register(func(p processors.Payment) ([]byte, error) {
		counter++
		return pipz.Encode(p)
	})

	// Run concurrent processes
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			payment := processors.Payment{
				ID:     fmt.Sprintf("PAY-%d", id),
				Amount: float64(id * 100),
			}
			_, err := contract.Process(payment)
			if err != nil {
				t.Errorf("Concurrent process %d failed: %v", id, err)
			}
			done <- true
		}(i)
	}

	// Wait for all to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// All should have been processed
	if counter != 10 {
		t.Errorf("Expected 10 processes, got %d", counter)
	}
}