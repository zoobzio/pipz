package main

import (
	"fmt"
	"math"
	"strings"

	"github.com/spf13/cobra"
	"pipz"
	"pipz/demo/testutil"
)

var validationCmd = &cobra.Command{
	Use:   "validation",
	Short: "Data Validation Pipeline demonstration",
	Long:  `Demonstrates complex data validation with reusable validators.`,
	Run:   runValidationDemo,
}

func init() {
	rootCmd.AddCommand(validationCmd)
}

// Demo types for validation
type ValidatorKey string

const (
	// ValidatorContractV1 is the current version of the validator contract
	ValidatorContractV1 ValidatorKey = "v1"
)

type Order struct {
	ID         string
	CustomerID string
	Items      []OrderItem
	Total      float64
}

type OrderItem struct {
	ProductID string
	Quantity  int
	Price     float64
}

// Validators
func validateOrderID(o Order) ([]byte, error) {
	if o.ID == "" {
		return nil, fmt.Errorf("order ID required")
	}
	if !strings.HasPrefix(o.ID, "ORD-") {
		return nil, fmt.Errorf("order ID must start with ORD-")
	}
	return nil, nil // No changes, validation passed
}

func validateItems(o Order) ([]byte, error) {
	if len(o.Items) == 0 {
		return nil, fmt.Errorf("order must have at least one item")
	}
	for i, item := range o.Items {
		if item.Quantity <= 0 {
			return nil, fmt.Errorf("item %d: quantity must be positive", i)
		}
		if item.Price < 0 {
			return nil, fmt.Errorf("item %d: price cannot be negative", i)
		}
	}
	return nil, nil // No changes, validation passed
}

func validateTotal(o Order) ([]byte, error) {
	calculated := 0.0
	for _, item := range o.Items {
		calculated += float64(item.Quantity) * item.Price
	}
	if math.Abs(calculated-o.Total) > 0.01 {
		return nil, fmt.Errorf("total mismatch: expected %.2f, got %.2f", calculated, o.Total)
	}
	return nil, nil // No changes, validation passed
}

func runValidationDemo(cmd *cobra.Command, args []string) {
	pp := testutil.NewPrettyPrinter()
	
	// Get interactive flag from parent command
	interactive, _ := cmd.Flags().GetBool("interactive")
	pp.SetInteractive(interactive)
	
	pp.Section("✅ DATA VALIDATION PIPELINE DEMO")
	
	pp.SubSection("📋 Use Case: E-commerce Order Validation")
	pp.Info("Scenario: An e-commerce platform needs to validate orders before processing.")
	pp.Info("Requirements:")
	pp.Info("  • Validate order ID format (must start with 'ORD-')")
	pp.Info("  • Ensure all items have valid quantities and prices")
	pp.Info("  • Verify order total matches sum of items")
	pp.Info("  • Provide clear error messages for debugging")
	
	pp.SubSection("🔧 Pipeline Configuration")
	pp.Code("go", `type ValidatorKey string

const (
    ValidatorContractV1 ValidatorKey = "v1"
)

// Register validation pipeline
validationContract := pipz.GetContract[ValidatorKey, Order](ValidatorContractV1)
validationContract.Register(
    validateOrderID,  // Check ID format
    validateItems,    // Validate all items
    validateTotal,    // Verify total accuracy
)`)
	
	// Register the pipeline
	validationContract := pipz.GetContract[ValidatorKey, Order](ValidatorContractV1)
	err := validationContract.Register(validateOrderID, validateItems, validateTotal)
	if err != nil {
		pp.Error(fmt.Sprintf("Failed to register pipeline: %v", err))
		return
	}
	
	pp.Success("Pipeline registered successfully")
	
	pp.SubSection("🔍 Live Validation Examples")
	
	// Example 1: Valid order
	pp.Info("Example 1: Valid order")
	validOrder := Order{
		ID:         "ORD-12345",
		CustomerID: "CUST-789",
		Items: []OrderItem{
			{ProductID: "PROD-A", Quantity: 2, Price: 29.99},
			{ProductID: "PROD-B", Quantity: 1, Price: 49.99},
		},
		Total: 109.97,
	}
	
	result, err := validationContract.Process(validOrder)
	if err != nil {
		pp.Error(fmt.Sprintf("Validation failed: %v", err))
	} else {
		pp.Success("Order validated successfully!")
		pp.Info(fmt.Sprintf("  Order ID: %s", result.ID))
		pp.Info(fmt.Sprintf("  Items: %d", len(result.Items)))
		pp.Info(fmt.Sprintf("  Total: $%.2f", result.Total))
	}
	
	pp.Info("")
	pp.Info("Example 2: Invalid order ID")
	invalidIDOrder := Order{
		ID:         "12345", // Missing ORD- prefix
		CustomerID: "CUST-789",
		Items: []OrderItem{
			{ProductID: "PROD-A", Quantity: 1, Price: 29.99},
		},
		Total: 29.99,
	}
	
	_, err = validationContract.Process(invalidIDOrder)
	if err != nil {
		pp.Error(fmt.Sprintf("Validation failed: %v", err))
	}
	
	pp.Info("")
	pp.Info("Example 3: Mismatched total")
	mismatchedOrder := Order{
		ID:         "ORD-67890",
		CustomerID: "CUST-456",
		Items: []OrderItem{
			{ProductID: "PROD-C", Quantity: 3, Price: 19.99},
			{ProductID: "PROD-D", Quantity: 2, Price: 39.99},
		},
		Total: 100.00, // Should be 139.95
	}
	
	_, err = validationContract.Process(mismatchedOrder)
	if err != nil {
		pp.Error(fmt.Sprintf("Validation failed: %v", err))
	}
	
	pp.Info("")
	pp.Info("Example 4: Invalid item quantity")
	invalidQuantityOrder := Order{
		ID:         "ORD-11111",
		CustomerID: "CUST-999",
		Items: []OrderItem{
			{ProductID: "PROD-E", Quantity: -1, Price: 99.99}, // Negative quantity
		},
		Total: -99.99,
	}
	
	_, err = validationContract.Process(invalidQuantityOrder)
	if err != nil {
		pp.Error(fmt.Sprintf("Validation failed: %v", err))
	}
	
	pp.SubSection("🎯 Validation Pipeline Benefits")
	
	pp.Feature("🔍", "Composable Validators", "Mix and match validation rules")
	pp.Feature("⚡", "Fast Failure", "Stops at first validation error")
	pp.Feature("📝", "Clear Errors", "Detailed messages for debugging")
	pp.Feature("♻️", "Reusable", "Same validators across different pipelines")
	pp.Feature("🧪", "Testable", "Each validator can be unit tested")
	
	pp.SubSection("Validation Flow")
	pp.Info("1. Order ID Format Check")
	pp.Info("2. Item Validation (quantity, price)")
	pp.Info("3. Total Calculation Verification")
	pp.Info("4. Return validated order or error")
	
	pp.Stats("Validation Performance", map[string]interface{}{
		"Validators": 3,
		"Avg Time": "< 1ms",
		"Memory": "~100 bytes",
		"Type Safety": "100%",
	})
}