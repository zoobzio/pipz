// Order Processing Pipeline Example.
// =================================.
// This is a DEMONSTRATION of building an e-commerce order processing system.
// using the pipz library. It shows how a simple MVP can evolve into a.
// production-grade system through progressive enhancement.
//
// IMPORTANT: This example uses MOCK services for demonstration purposes.
// In a real implementation, you would integrate with actual payment providers,.
// inventory systems, and notification services.
//
// Run with: go run .
// Run specific sprint: go run . -sprint=4.

package main

import (
	"context"
	"flag"
	"fmt"
	"time"
)

func main() {
	var sprint int
	flag.IntVar(&sprint, "sprint", 0, "Run specific sprint (1-11), 0 for full demo")
	flag.Parse()

	// Enable test mode for faster demos.
	TestMode = true

	fmt.Println("=== Order Processing Pipeline Demo ===")
	fmt.Println("Building an e-commerce order system that evolves from MVP to enterprise-grade")
	fmt.Println()

	ctx := context.Background()

	if sprint > 0 {
		// Run specific sprint.
		runSprint(ctx, sprint)
	} else {
		// Run full evolution demo.
		runFullDemo(ctx)
	}
}

func runFullDemo(ctx context.Context) {
	// Sprint 1: MVP.
	fmt.Println("📦 SPRINT 1: MVP - Just Take Payments!")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	ResetPipeline()
	SetAllServicesNormal()
	MetricsCollector.Clear()

	order1 := createTestOrder("CUST-001", 299.99, "LAPTOP-001", "Gaming Laptop", 1)

	result1, err := ProcessOrder(ctx, order1)
	if err != nil {
		fmt.Printf("❌ Error: %v\n", err) //nolint:errcheck // demo code.
	} else {
		fmt.Printf("✅ Order processed in %v\n", result1.Timestamps["completed"].Sub(result1.CreatedAt)) //nolint:errcheck // demo code.
		fmt.Printf("   Status: %s, Payment: %s\n", result1.Status, result1.PaymentID)                   //nolint:errcheck // demo code.
	}

	// Simulate the problem.
	fmt.Println("\n💥 Later that day...")
	fmt.Println("Customer: 'I was charged but the order isn't in the system!'")
	fmt.Println("Dev: 'Looks like payment went through but database save failed...'")
	fmt.Println()
	time.Sleep(2 * time.Second)

	// Sprint 2: Inventory Management.
	fmt.Println("\n📦 SPRINT 2: Inventory Management - No More Overselling!")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("Last week: $50k in refunds for oversold items 😱")
	EnableInventoryManagement()
	ResetInventory()

	// Test inventory reservation.
	order3 := createTestOrder("CUST-002", 999.99, "PHONE-001", "Latest Phone", 2)
	result3, _ := ProcessOrder(ctx, order3)                                                            //nolint:errcheck // demo code.
	fmt.Printf("✅ Order with inventory: %v\n", result3.Timestamps["completed"].Sub(result3.CreatedAt)) //nolint:errcheck // demo code.
	fmt.Printf("   Reservation: %s\n", result3.ReservationID)                                          //nolint:errcheck // demo code.

	// Test out of stock.
	orderOOS := createTestOrder("CUST-003", 9999.99, "PHONE-001", "Latest Phone", 200)
	_, err = ProcessOrder(ctx, orderOOS)
	if err != nil {
		fmt.Printf("✅ Correctly rejected out-of-stock order: %v\n", err) //nolint:errcheck // demo code.
	}

	fmt.Printf("\n💰 Savings: No more refunding oversold items!\n") //nolint:errcheck // demo code.
	time.Sleep(2 * time.Second)

	// Sprint 3: Notifications.
	fmt.Println("\n\n📦 SPRINT 3: Customer Communications - Real-time Updates!")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("Support tickets down 60% with proactive notifications!")
	EnableNotifications()

	order5 := createTestOrder("CUST-004", 1299.99, "TABLET-001", "Pro Tablet", 1)

	fmt.Println("\nWithout concurrent notifications: ~2.5s")
	fmt.Println("With concurrent notifications: ~1s")

	result5, _ := ProcessOrder(ctx, order5)                                                                  //nolint:errcheck // demo code.
	fmt.Printf("\n✅ Order with notifications: %v\n", result5.Timestamps["completed"].Sub(result5.CreatedAt)) //nolint:errcheck // demo code.
	fmt.Printf("   Notifications sent: email, sms, analytics, warehouse\n")                                  //nolint:errcheck // demo code.
	fmt.Printf("⚡ Performance gain: 60%% faster checkout!\n")                                                //nolint:errcheck // demo code.
	time.Sleep(2 * time.Second)

	// Sprint 4: Fraud Detection.
	fmt.Println("\n\n📦 SPRINT 4: Fraud Detection - Stop the Scammers!")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("Last month: $50k lost to fraud + $49k in chargeback fees!")
	EnableFraudDetection()

	// Low risk order.
	orderLow := createTestOrder("CUST-GOLD", 199.99, "HEADPHONES-001", "Wireless Headphones", 1)
	resultLow, _ := ProcessOrder(ctx, orderLow)                                                  //nolint:errcheck // demo code.
	fmt.Printf("\n✅ Low risk order (score: %.2f): %s\n", resultLow.FraudScore, resultLow.Status) //nolint:errcheck // demo code.

	// High risk order.
	orderHigh := createTestOrder("CUST-NEW", 5999.99, "LAPTOP-001", "Gaming Laptop", 20)
	orderHigh.IsFirstOrder = true
	resultHigh, err := ProcessOrder(ctx, orderHigh)
	if err != nil {
		fmt.Printf("🚫 High risk order blocked (score: %.2f): %v\n", resultHigh.FraudScore, err) //nolint:errcheck // demo code.
	}

	fmt.Printf("\n💰 Fraud prevention: $99k saved this month!\n") //nolint:errcheck // demo code.
	time.Sleep(2 * time.Second)

	// Sprint 5: Payment Resilience.
	fmt.Println("\n\n📦 SPRINT 5: Payment Resilience - Never Lose an Order!")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("Black Friday: Payment provider went down, but we stayed up!")
	EnablePaymentResilience()

	// Simulate payment failure.
	PaymentGateway.Behavior = BehaviorIntermittent
	defer func() { PaymentGateway.Behavior = BehaviorNormal }()

	order9 := createTestOrder("CUST-005", 399.99, "WATCH-001", "Smart Watch", 1)
	result9, err := ProcessOrder(ctx, order9)
	if err == nil {
		fmt.Printf("✅ Order succeeded despite payment issues (retry worked!)\n") //nolint:errcheck // demo code.
	} else {
		fmt.Printf("⚠️  Order saved for retry: %s\n", result9.Status) //nolint:errcheck // demo code.
	}

	fmt.Println("\n📊 Stats: 65% of failed payments succeed on retry!")
	time.Sleep(2 * time.Second)

	// Sprint 6: Enterprise Features.
	fmt.Println("\n\n📦 SPRINT 6: Enterprise Ready - Full Production System!")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	EnableEnterpriseFeatures()
	PaymentGateway.Behavior = BehaviorNormal

	// Process several orders to show metrics.
	fmt.Println("\nProcessing multiple orders...")
	orders := []Order{
		createTestOrder("CUST-PLAT", 2999.99, "LAPTOP-001", "Gaming Laptop", 2),
		createTestOrder("CUST-GOLD", 599.99, "PHONE-001", "Latest Phone", 1),
		createTestOrder("CUST-NEW", 99.99, "HEADPHONES-001", "Wireless Headphones", 1),
	}

	for i := range orders {
		if i == 0 {
			// Enrich first order as platinum customer.
			orders[i].CustomerTier = TierPlatinum
		}
		result, err := ProcessOrder(ctx, orders[i])
		if err != nil {
			fmt.Printf("❌ Order %d failed: %v\n", i+1, err) //nolint:errcheck // demo code.
		} else {
			fmt.Printf("✅ Order %d: %s (%.2f risk score, %v processing time)\n", //nolint:errcheck // demo code.
				i+1, result.Status, result.FraudScore,
				result.Timestamps["completed"].Sub(result.CreatedAt))
		}
	}

	// Show final metrics.
	fmt.Println("\n" + MetricsCollector.GetSummary().GenerateReport())

	// Success story.
	fmt.Println("\n🎉 SUCCESS STORY")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("✅ Started with simple payment processing")
	fmt.Println("✅ Now handling 10k+ orders/hour with:")
	fmt.Println("   • Zero overselling with inventory management")
	fmt.Println("   • 60% fewer support tickets with notifications")
	fmt.Println("   • 90% fraud reduction saving $99k/month")
	fmt.Println("   • 99.9% order success rate with resilience")
	fmt.Println("   • Enterprise features for growth")
	fmt.Println("\n🚀 From MVP to enterprise in 6 sprints!")
}

func runSprint(ctx context.Context, sprintNum int) {
	fmt.Printf("Running Sprint %d Demo\n", sprintNum) //nolint:errcheck // demo code.
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	// Reset everything.
	ResetPipeline()
	SetAllServicesNormal()
	ResetInventory()
	MetricsCollector.Clear()

	switch sprintNum {
	case 1:
		fmt.Println("Sprint 1: MVP - Basic order processing")
		// Pipeline already in MVP state.

	case 2:
		fmt.Println("Sprint 2: Inventory Management")
		EnableInventoryManagement()

	case 3:
		fmt.Println("Sprint 3: Customer Notifications")
		EnableInventoryManagement()
		EnableNotifications()

	case 4:
		fmt.Println("Sprint 4: Fraud Detection")
		EnableFraudDetection()

	case 5:
		fmt.Println("Sprint 5: Payment Resilience")
		EnablePaymentResilience()

	case 6:
		fmt.Println("Sprint 6: Enterprise Features")
		EnableEnterpriseFeatures()

	default:
		fmt.Printf("Invalid sprint number: %d (valid: 1-6)\n", sprintNum) //nolint:errcheck // demo code.
		return
	}

	// Run test orders.
	fmt.Println("\nTesting pipeline...")
	testOrders := []struct { //nolint:govet // demo struct, alignment not critical
		name     string
		order    Order
		scenario string
	}{
		{
			name:     "Standard Order",
			order:    createTestOrder("TEST-001", 299.99, "LAPTOP-001", "Gaming Laptop", 1),
			scenario: "normal",
		},
		{
			name:     "High Value Order",
			order:    createTestOrder("TEST-002", 2999.99, "LAPTOP-001", "Gaming Laptop", 3),
			scenario: "high_value",
		},
		{
			name:     "Out of Stock",
			order:    createTestOrder("TEST-003", 999.99, "PHONE-001", "Latest Phone", 500),
			scenario: "out_of_stock",
		},
	}

	for idx := range testOrders {
		fmt.Printf("\n%s: %.50s\n", testOrders[idx].name, testOrders[idx].scenario) //nolint:errcheck // demo code.

		// Special handling for certain scenarios.
		if testOrders[idx].scenario == "high_value" && sprintNum >= 4 {
			testOrders[idx].order.IsFirstOrder = true // Makes it higher risk.
		}

		start := time.Now()
		result, err := ProcessOrder(ctx, testOrders[idx].order)
		duration := time.Since(start)

		if err != nil {
			fmt.Printf("❌ Failed: %v\n", err.Err)        //nolint:errcheck // demo code.
			fmt.Printf("   Status: %s\n", result.Status) //nolint:errcheck // demo code.
			if sprintNum >= 4 && result.FraudScore > 0 {
				fmt.Printf("   Fraud Score: %.2f\n", result.FraudScore) //nolint:errcheck // demo code.
			}
		} else {
			fmt.Printf("✅ Success!\n")                          //nolint:errcheck // demo code.
			fmt.Printf("   Status: %s\n", result.Status)        //nolint:errcheck // demo code.
			fmt.Printf("   Time: %v\n", duration)               //nolint:errcheck // demo code.
			fmt.Printf("   Total: $%.2f\n", result.TotalAmount) //nolint:errcheck // demo code.

			// Show sprint-specific info.
			if sprintNum >= 2 && result.ReservationID != "" {
				fmt.Printf("   Inventory: %s\n", result.ReservationID) //nolint:errcheck // demo code.
			}
			if sprintNum >= 4 {
				fmt.Printf("   Fraud Score: %.2f\n", result.FraudScore)   //nolint:errcheck // demo code.
				fmt.Printf("   Customer: %s tier\n", result.CustomerTier) //nolint:errcheck // demo code.
			}
			if sprintNum >= 6 && result.TrackingNumber != "" {
				fmt.Printf("   Tracking: %s\n", result.TrackingNumber) //nolint:errcheck // demo code.
			}
		}
	}

	// Show metrics if we have meaningful data.
	if sprintNum >= 3 {
		summary := MetricsCollector.GetSummary()
		if summary.TotalOrders > 0 {
			fmt.Println("\n--- Quick Metrics ---")
			fmt.Printf("Success Rate: %.1f%%\n", //nolint:errcheck // demo code.
				float64(summary.SuccessfulOrders)/float64(summary.TotalOrders)*100)
			fmt.Printf("Average Time: %v\n", summary.AverageProcessTime) //nolint:errcheck // demo code.
			if summary.TotalRevenue > 0 {
				fmt.Printf("Revenue: $%.2f\n", summary.TotalRevenue) //nolint:errcheck // demo code.
			}
		}
	}
}

// Helper function to create test orders.
func createTestOrder(customerID string, amount float64, productID, productName string, quantity int) Order {
	return Order{
		CustomerID:  customerID,
		TotalAmount: amount,
		Items: []OrderItem{
			{
				ProductID:   productID,
				ProductName: productName,
				Quantity:    quantity,
				Price:       amount / float64(quantity),
			},
		},
		CreatedAt: time.Now(),
		Status:    StatusPending,
	}
}
