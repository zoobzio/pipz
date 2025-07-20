package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/zoobzio/pipz"
	"github.com/zoobzio/pipz/examples/workflow"
)

// WorkflowExample implements the Example interface for complex order workflows
type WorkflowExample struct{}

func (w *WorkflowExample) Name() string {
	return "workflow"
}

func (w *WorkflowExample) Description() string {
	return "Complex multi-stage business workflows with parallel execution and compensating transactions"
}

func (w *WorkflowExample) Demo(ctx context.Context) error {
	fmt.Println("\n" + colorCyan + "═══ WORKFLOW EXAMPLE ═══" + colorReset)
	fmt.Println("\nThis example demonstrates:")
	fmt.Println("• Complex multi-stage order processing workflows")
	fmt.Println("• Parallel execution of inventory and fraud checks")
	fmt.Println("• Conditional routing based on order type")
	fmt.Println("• Compensating transactions for rollback")
	fmt.Println("• Payment retry strategies with automatic recovery")

	waitForEnter()

	// Show the problem
	fmt.Println("\n" + colorYellow + "The Problem:" + colorReset)
	fmt.Println("Order processing requires complex orchestration:")
	fmt.Println(colorGray + `
func processOrder(order Order) error {
    // Validate order
    if err := validateOrder(order); err != nil {
        return err
    }
    
    // Check inventory and fraud in parallel
    var inventoryErr, fraudErr error
    var wg sync.WaitGroup
    wg.Add(2)
    
    go func() {
        defer wg.Done()
        inventoryErr = checkInventory(order)
    }()
    
    go func() {
        defer wg.Done()
        fraudErr = checkFraud(order)
    }()
    
    wg.Wait()
    
    if inventoryErr != nil {
        return inventoryErr
    }
    if fraudErr != nil {
        // Need to release inventory
        releaseInventory(order)
        return fraudErr
    }
    
    // Process payment with retries
    for attempt := 0; attempt < 3; attempt++ {
        err := processPayment(order)
        if err == nil {
            break
        }
        if attempt == 2 {
            // All attempts failed - rollback
            releaseInventory(order)
            return err
        }
        time.Sleep(time.Duration(attempt+1) * time.Second)
    }
    
    // Route to fulfillment based on order type
    switch order.Type {
    case "express":
        return fulfillExpressOrder(order)
    case "standard":
        return fulfillStandardOrder(order)
    case "digital":
        return fulfillDigitalOrder(order)
    }
    
    return nil
}` + colorReset)

	waitForEnter()

	// Show the solution
	fmt.Println("\n" + colorGreen + "The pipz Solution:" + colorReset)
	fmt.Println("Use the real workflow example with proper orchestration:")
	fmt.Println(colorGray + `
// Import the real workflow package
import "github.com/zoobzio/pipz/examples/workflow"

// Create services
inventory := workflow.NewInventoryService()
fraud := workflow.NewFraudService()
payment := workflow.NewPaymentService()
shipping := workflow.NewShippingService()

// Build the complete workflow
orderPipeline := workflow.CreateOrderPipeline(
    inventory, fraud, payment, shipping,
)

// Process orders with automatic rollback on failures
result, err := orderPipeline.Process(ctx, order)` + colorReset)

	waitForEnter()

	// Run interactive demo
	fmt.Println("\n" + colorPurple + "Let's process some orders!" + colorReset)
	return w.runInteractive(ctx)
}

func (w *WorkflowExample) runInteractive(ctx context.Context) error {
	// Reset statistics for clean demo
	workflow.ResetStats()

	// Initialize services
	inventory := workflow.NewInventoryService()
	fraud := workflow.NewFraudService()
	payment := workflow.NewPaymentService()
	shipping := workflow.NewShippingService()

	// Create the main order processing pipeline
	orderPipeline := workflow.CreateOrderPipeline(inventory, fraud, payment, shipping)
	compensatingPipeline := workflow.CreateCompensatingPipeline(inventory, payment)

	// Test scenarios
	scenarios := []struct {
		name     string
		order    workflow.Order
		expected string
		special  string
	}{
		{
			name:     "Standard Order - Happy Path",
			order:    workflow.CreateTestOrder(workflow.OrderTypeStandard, "CUSTOMER-001"),
			expected: "completed",
		},
		{
			name:     "Express Order - Fast Processing",
			order:    workflow.CreateTestOrder(workflow.OrderTypeExpress, "CUSTOMER-002"),
			expected: "completed",
		},
		{
			name:     "Digital Order - Immediate Delivery",
			order:    workflow.CreateTestOrder(workflow.OrderTypeDigital, "CUSTOMER-003"),
			expected: "completed",
		},
		{
			name:     "High Risk Order - Fraud Detection",
			order:    workflow.CreateTestOrder(workflow.OrderTypeExpress, "FRAUD-USER-001"),
			expected: "failed",
		},
		{
			name:    "Payment Retry Test",
			order:   workflow.CreateTestOrder(workflow.OrderTypeStandard, "CUSTOMER-004"),
			special: "payment_retry",
		},
		{
			name:    "Order Cancellation",
			order:   workflow.CreateTestOrder(workflow.OrderTypeStandard, "CUSTOMER-005"),
			special: "cancellation",
		},
	}

	for i, scenario := range scenarios {
		fmt.Printf("\n%s═══ Scenario %d: %s ═══%s\n",
			colorWhite, i+1, scenario.name, colorReset)

		// Show order details
		fmt.Printf("\nOrder Details:\n")
		fmt.Printf("  ID: %s\n", scenario.order.ID)
		fmt.Printf("  Customer: %s\n", scenario.order.CustomerID)
		fmt.Printf("  Type: %s\n", scenario.order.Type)
		fmt.Printf("  Items: %d\n", len(scenario.order.Items))
		fmt.Printf("  Total: $%.2f\n", scenario.order.Total)

		if scenario.special == "cancellation" {
			// Test cancellation workflow
			fmt.Printf("\n%sSimulating order cancellation after partial processing...%s\n",
				colorYellow, colorReset)

			// Start processing
			fmt.Printf("\n%sStarting order processing...%s\n", colorYellow, colorReset)
			start := time.Now()

			// Process up to payment (will succeed)
			partialPipeline := pipz.Sequential(
				pipz.Apply("validate_order", workflow.ValidateOrder),
				pipz.Apply("parallel_checks", workflow.ParallelChecks(inventory, fraud)),
			)

			partialResult, err := partialPipeline.Process(ctx, scenario.order)
			if err != nil {
				fmt.Printf("\n%s❌ Partial Processing Failed: %s%s\n", colorRed, err.Error(), colorReset)
				continue
			}

			fmt.Printf("\n%s⚠️  Customer requested cancellation - initiating rollback...%s\n",
				colorYellow, colorReset)

			// Run compensating pipeline
			compensatedResult, err := compensatingPipeline.Process(ctx, partialResult)
			duration := time.Since(start)

			if err != nil {
				fmt.Printf("\n%s❌ Cancellation Failed: %s%s\n", colorRed, err.Error(), colorReset)
			} else {
				fmt.Printf("\n%s✅ Order Successfully Cancelled%s\n", colorGreen, colorReset)
				fmt.Printf("  Status: %s\n", compensatedResult.Status)
				fmt.Printf("  Inventory Released: %v\n", !compensatedResult.InventoryReserved)
				fmt.Printf("  Processing Time: %v\n", duration)
			}
		} else if scenario.special == "payment_retry" {
			// Test payment retry logic
			fmt.Printf("\n%sSimulating payment failures and retries...%s\n",
				colorYellow, colorReset)

			start := time.Now()
			result, err := orderPipeline.Process(ctx, scenario.order)
			duration := time.Since(start)

			if err != nil {
				fmt.Printf("\n%s❌ Order Processing Failed%s\n", colorRed, colorReset)
				if strings.Contains(err.Error(), "payment") {
					fmt.Printf("  Payment failed after retries\n")
				}
				fmt.Printf("  Error: %s\n", err.Error())
			} else {
				fmt.Printf("\n%s✅ Order Processed Successfully (despite retry attempts)%s\n", colorGreen, colorReset)
				fmt.Printf("  Payment Status: %s\n", result.PaymentStatus)
				fmt.Printf("  Final Status: %s\n", result.Status)
			}
			fmt.Printf("  Processing Time: %v\n", duration)
		} else {
			// Normal order processing
			fmt.Printf("\n%sProcessing order...%s\n", colorYellow, colorReset)
			start := time.Now()

			result, err := orderPipeline.Process(ctx, scenario.order)
			duration := time.Since(start)

			if err != nil {
				fmt.Printf("\n%s❌ Order Processing Failed%s\n", colorRed, colorReset)

				// Check if it was expected failure
				if scenario.expected == "failed" {
					fmt.Printf("  %s(Expected failure - fraud detection working correctly)%s\n",
						colorGray, colorReset)
				}

				var pipelineErr *pipz.PipelineError[workflow.Order]
				if errors.As(err, &pipelineErr) {
					fmt.Printf("  Failed at: %s%s%s (stage %d)\n",
						colorYellow, pipelineErr.ProcessorName, colorReset,
						pipelineErr.StageIndex)
				}

				fmt.Printf("  Error: %s\n", err.Error())
			} else {
				fmt.Printf("\n%s✅ Order Processed Successfully%s\n", colorGreen, colorReset)
				fmt.Printf("  Final Status: %s\n", result.Status)
				fmt.Printf("  Payment ID: %s\n", result.PaymentID)
				fmt.Printf("  Shipping Carrier: %s\n", result.ShippingCarrier)

				if result.TrackingNumber != "" {
					fmt.Printf("  Tracking Number: %s\n", result.TrackingNumber)
				}

				fmt.Printf("  Fraud Score: %.2f\n", result.FraudScore)
				fmt.Printf("  Processing Time: %v\n", duration)

				// Show workflow progression
				fmt.Printf("\n  Workflow Timeline:\n")
				if !result.ValidatedAt.IsZero() {
					fmt.Printf("    ✓ Validated: %v\n", result.ValidatedAt.Sub(result.CreatedAt))
				}
				if !result.PaidAt.IsZero() {
					fmt.Printf("    ✓ Paid: %v\n", result.PaidAt.Sub(result.CreatedAt))
				}
				if !result.ShippedAt.IsZero() {
					fmt.Printf("    ✓ Shipped: %v\n", result.ShippedAt.Sub(result.CreatedAt))
				}
				if !result.CompletedAt.IsZero() {
					fmt.Printf("    ✓ Completed: %v\n", result.CompletedAt.Sub(result.CreatedAt))
				}
			}
		}

		if i < len(scenarios)-1 {
			waitForEnter()
		}
	}

	// Show workflow statistics
	fmt.Printf("\n%s═══ WORKFLOW STATISTICS ═══%s\n", colorCyan, colorReset)
	stats := workflow.GetStats()

	fmt.Printf("\nTotal Orders Processed: %d\n", stats.Total)

	if len(stats.ByType) > 0 {
		fmt.Printf("\nBy Order Type:\n")
		for orderType, count := range stats.ByType {
			fmt.Printf("  %s: %d\n", orderType, count)
		}
	}

	if len(stats.ByStatus) > 0 {
		fmt.Printf("\nBy Final Status:\n")
		for status, count := range stats.ByStatus {
			fmt.Printf("  %s: %d\n", status, count)
		}
	}

	if stats.TotalRevenue > 0 {
		fmt.Printf("\nRevenue Metrics:\n")
		fmt.Printf("  Total Revenue: $%.2f\n", stats.TotalRevenue)
		fmt.Printf("  Average Order Value: $%.2f\n", stats.AverageValue)
	}

	if stats.ProcessingTime > 0 {
		fmt.Printf("  Average Processing Time: %v\n", stats.ProcessingTime)
	}

	// Show best practices
	fmt.Printf("\n%s═══ WORKFLOW BEST PRACTICES ═══%s\n", colorCyan, colorReset)
	fmt.Printf("\n• Use parallel processing for independent operations\n")
	fmt.Printf("• Implement compensating transactions for rollback\n")
	fmt.Printf("• Add retry logic for transient failures\n")
	fmt.Printf("• Route based on business rules (order type, priority)\n")
	fmt.Printf("• Track detailed metrics for monitoring and optimization\n")
	fmt.Printf("• Design workflows to be resumable and idempotent\n")
	fmt.Printf("• Use timeouts to prevent hanging operations\n")

	return nil
}

func (w *WorkflowExample) Benchmark(b *testing.B) error {
	return nil
}
