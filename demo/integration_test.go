package main_test

import (
	"fmt"
	"testing"

	"pipz"
	"pipz/examples"
)

// TestTypeUniverseIsolation verifies that different key types create isolated pipelines
func TestTypeUniverseIsolation(t *testing.T) {
	// Define different key types for the same data
	type StandardKey string
	type PremiumKey string
	type TestKey string

	// Define const keys
	const standardKey StandardKey = "v1"
	const premiumKey PremiumKey = "v1"
	const testKey TestKey = "v1"

	// Create three different pipelines with the same key value
	standardContract := pipz.GetContract[examples.Payment](standardKey)
	premiumContract := pipz.GetContract[examples.Payment](premiumKey)
	testContract := pipz.GetContract[examples.Payment](testKey)

	// Register different processors for each
	var standardProcessed, premiumProcessed, testProcessed bool

	standardContract.Register(func(p examples.Payment) ([]byte, error) {
		standardProcessed = true
		p.Provider = "standard"
		return pipz.Encode(p)
	})

	premiumContract.Register(func(p examples.Payment) ([]byte, error) {
		premiumProcessed = true
		p.Provider = "premium"
		return pipz.Encode(p)
	})

	testContract.Register(func(p examples.Payment) ([]byte, error) {
		testProcessed = true
		p.Provider = "test"
		return pipz.Encode(p)
	})

	// Process the same payment through each pipeline
	payment := examples.Payment{
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

	// Define const keys
	const paymentKeyV1 PaymentKey = "v1"
	const paymentKeyV2 PaymentKey = "v2"

	// Register v1 pipeline - simple validation
	v1 := pipz.GetContract[examples.Payment](paymentKeyV1)
	v1.Register(
		pipz.Apply(examples.ValidatePayment),
	)

	// Register v2 pipeline - validation + fraud check
	v2 := pipz.GetContract[examples.Payment](paymentKeyV2)
	v2.Register(
		pipz.Apply(examples.ValidatePayment),
		pipz.Apply(examples.CheckFraud),
	)

	// Test payment that passes v1 but fails v2
	payment := examples.Payment{
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
	// Define key types
	type EnrichmentKey string
	
	// Define const keys
	const validatorKey examples.ValidatorKey = "test"
	const enrichmentKey EnrichmentKey = "test"

	// Create individual contracts
	validationContract := pipz.GetContract[examples.Order](validatorKey)
	validationContract.Register(
		pipz.Apply(examples.ValidateOrderID),
		pipz.Apply(examples.ValidateItems),
	)

	// Create a second contract for enrichment
	enrichmentContract := pipz.GetContract[examples.Order](enrichmentKey)
	enrichmentContract.Register(func(o examples.Order) ([]byte, error) {
		// Add some metadata
		if o.CustomerID == "" {
			o.CustomerID = "GUEST"
		}
		return pipz.Encode(o)
	})

	// Create a chain
	chain := pipz.NewChain[examples.Order]()
	chain.Add(
		validationContract.Link(),
		enrichmentContract.Link(),
	)

	// Test the chain
	order := examples.Order{
		ID: "ORD-123",
		Items: []examples.OrderItem{
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
	const key examples.PaymentKey = "concurrent"
	contract := pipz.GetContract[examples.Payment](key)

	var counter int
	contract.Register(func(p examples.Payment) ([]byte, error) {
		counter++
		return pipz.Encode(p)
	})

	// Run concurrent processes
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			payment := examples.Payment{
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