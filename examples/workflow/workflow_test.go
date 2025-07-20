package workflow

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/zoobzio/pipz"
)

func TestValidateOrder(t *testing.T) {
	tests := []struct {
		name        string
		order       Order
		wantErr     bool
		errContains string
	}{
		{
			name: "Valid standard order",
			order: Order{
				CustomerID: "CUST-001",
				Type:       OrderTypeStandard,
				Items: []OrderItem{
					{ProductID: "P1", Name: "Widget", Quantity: 2, Price: 10.00},
				},
				ShippingAddr: Address{
					Street: "123 Main St",
					City:   "San Francisco",
				},
			},
			wantErr: false,
		},
		{
			name: "Valid digital order without address",
			order: Order{
				CustomerID: "CUST-002",
				Type:       OrderTypeDigital,
				Items: []OrderItem{
					{ProductID: "D1", Name: "License", Quantity: 1, Price: 99.99},
				},
			},
			wantErr: false,
		},
		{
			name: "Missing customer ID",
			order: Order{
				Type: OrderTypeStandard,
				Items: []OrderItem{
					{ProductID: "P1", Name: "Widget", Quantity: 1, Price: 10.00},
				},
			},
			wantErr:     true,
			errContains: "missing customer ID",
		},
		{
			name: "No items",
			order: Order{
				CustomerID: "CUST-003",
				Type:       OrderTypeStandard,
			},
			wantErr:     true,
			errContains: "no items",
		},
		{
			name: "Invalid quantity",
			order: Order{
				CustomerID: "CUST-004",
				Type:       OrderTypeStandard,
				Items: []OrderItem{
					{ProductID: "P1", Name: "Widget", Quantity: 0, Price: 10.00},
				},
				ShippingAddr: Address{
					Street: "123 Main St",
					City:   "San Francisco",
				},
			},
			wantErr:     true,
			errContains: "invalid quantity",
		},
		{
			name: "Missing shipping address for physical order",
			order: Order{
				CustomerID: "CUST-005",
				Type:       OrderTypeStandard,
				Items: []OrderItem{
					{ProductID: "P1", Name: "Widget", Quantity: 1, Price: 10.00},
				},
			},
			wantErr:     true,
			errContains: "invalid shipping address",
		},
	}

	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ValidateOrder(ctx, tt.order)

			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateOrder() error = %v, wantErr %v", err, tt.wantErr)
			}

			if err != nil && tt.errContains != "" {
				if !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("Expected error containing %q, got %q", tt.errContains, err.Error())
				}
			}

			if err == nil {
				// Check calculations
				if result.Status != StatusValidated {
					t.Error("Expected status to be validated")
				}

				if result.Total == 0 {
					t.Error("Expected total to be calculated")
				}

				// Check shipping cost
				switch result.Type {
				case OrderTypeExpress:
					if result.ShippingCost != 25.0 {
						t.Errorf("Expected express shipping to be $25, got $%.2f", result.ShippingCost)
					}
				case OrderTypeStandard:
					if result.ShippingCost != 5.0 {
						t.Errorf("Expected standard shipping to be $5, got $%.2f", result.ShippingCost)
					}
				case OrderTypeDigital:
					if result.ShippingCost != 0.0 {
						t.Errorf("Expected digital shipping to be $0, got $%.2f", result.ShippingCost)
					}
				}
			}
		})
	}
}

func TestInventoryService(t *testing.T) {
	inventory := NewInventoryService()

	t.Run("Check availability", func(t *testing.T) {
		items := []OrderItem{
			{ProductID: "PROD-001", Name: "Widget", Quantity: 10},
			{ProductID: "PROD-002", Name: "Gadget", Quantity: 5},
		}

		available, unavailable := inventory.CheckAvailability(items)
		if !available {
			t.Errorf("Expected items to be available, got unavailable: %v", unavailable)
		}
	})

	t.Run("Check unavailable", func(t *testing.T) {
		items := []OrderItem{
			{ProductID: "PROD-001", Name: "Widget", Quantity: 1000}, // More than stock
		}

		available, unavailable := inventory.CheckAvailability(items)
		if available {
			t.Error("Expected items to be unavailable")
		}
		if len(unavailable) != 1 {
			t.Errorf("Expected 1 unavailable item, got %d", len(unavailable))
		}
	})

	t.Run("Reserve and release", func(t *testing.T) {
		// Check initial stock
		stock1, _ := inventory.stock.Load("PROD-001")
		initialStock := stock1.(int)

		items := []OrderItem{
			{ProductID: "PROD-001", Name: "Widget", Quantity: 5},
		}

		// Reserve
		reservations, err := inventory.Reserve("ORDER-001", items)
		if err != nil {
			t.Fatalf("Failed to reserve: %v", err)
		}

		if len(reservations) != 1 {
			t.Errorf("Expected 1 reservation, got %d", len(reservations))
		}

		// Check stock decreased
		stock2, _ := inventory.stock.Load("PROD-001")
		if stock2.(int) != initialStock-5 {
			t.Errorf("Expected stock to be %d, got %d", initialStock-5, stock2.(int))
		}

		// Release
		err = inventory.ReleaseReservations(reservations)
		if err != nil {
			t.Fatalf("Failed to release: %v", err)
		}

		// Check stock restored
		stock3, _ := inventory.stock.Load("PROD-001")
		if stock3.(int) != initialStock {
			t.Errorf("Expected stock to be restored to %d, got %d", initialStock, stock3.(int))
		}
	})
}

func TestFraudService(t *testing.T) {
	fraud := NewFraudService()

	tests := []struct {
		name     string
		order    Order
		minScore float64
		maxScore float64
	}{
		{
			name: "Normal order",
			order: Order{
				CustomerID: "GOOD-USER",
				Total:      100.00,
				Items:      []OrderItem{{ProductID: "P1", Quantity: 1}},
			},
			minScore: 0.0,
			maxScore: 0.3,
		},
		{
			name: "Blacklisted user",
			order: Order{
				CustomerID: "FRAUD-USER-001",
				Total:      100.00,
				Items:      []OrderItem{{ProductID: "P1", Quantity: 1}},
			},
			minScore: 0.9,
			maxScore: 1.0,
		},
		{
			name: "High value order",
			order: Order{
				CustomerID: "USER-001",
				Total:      6000.00,
				Items:      []OrderItem{{ProductID: "P1", Quantity: 1}},
			},
			minScore: 0.3,
			maxScore: 0.6,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score, err := fraud.CheckFraud(tt.order)
			if err != nil {
				t.Fatalf("CheckFraud failed: %v", err)
			}

			if score < tt.minScore || score > tt.maxScore {
				t.Errorf("Expected score between %.2f and %.2f, got %.2f",
					tt.minScore, tt.maxScore, score)
			}
		})
	}
}

func TestParallelChecks(t *testing.T) {
	inventory := NewInventoryService()
	fraud := NewFraudService()
	checker := ParallelChecks(inventory, fraud)

	ctx := context.Background()

	t.Run("Successful checks", func(t *testing.T) {
		order := Order{
			ID:         "TEST-001",
			CustomerID: "GOOD-USER",
			Total:      100.00,
			Items: []OrderItem{
				{ProductID: "PROD-001", Name: "Widget", Quantity: 5},
			},
		}

		result, err := checker(ctx, order)
		if err != nil {
			t.Fatalf("ParallelChecks failed: %v", err)
		}

		if !result.InventoryReserved {
			t.Error("Expected inventory to be reserved")
		}

		if len(result.ReservationIDs) == 0 {
			t.Error("Expected reservation IDs")
		}

		if !result.FraudCheckPassed {
			t.Error("Expected fraud check to pass")
		}

		// Cleanup
		inventory.ReleaseReservations(result.ReservationIDs)
	})

	t.Run("Inventory failure", func(t *testing.T) {
		order := Order{
			ID:         "TEST-002",
			CustomerID: "GOOD-USER",
			Items: []OrderItem{
				{ProductID: "PROD-001", Name: "Widget", Quantity: 10000}, // Too much
			},
		}

		_, err := checker(ctx, order)
		if err == nil {
			t.Error("Expected error for insufficient inventory")
		}
	})

	t.Run("Fraud detection", func(t *testing.T) {
		order := Order{
			ID:         "TEST-003",
			CustomerID: "FRAUD-USER-001",
			Items: []OrderItem{
				{ProductID: "PROD-001", Name: "Widget", Quantity: 1},
			},
		}

		_, err := checker(ctx, order)
		if err == nil {
			t.Error("Expected error for fraudulent order")
		}
		if !strings.Contains(err.Error(), "fraudulent") {
			t.Errorf("Expected fraud error, got: %v", err)
		}
	})
}

func TestPaymentProcessing(t *testing.T) {
	inventory := NewInventoryService()
	payment := NewPaymentService()
	processor := ProcessPayment(payment, inventory)

	ctx := context.Background()

	// Pre-reserve inventory
	items := []OrderItem{
		{ProductID: "PROD-001", Name: "Widget", Quantity: 5},
	}
	reservations, _ := inventory.Reserve("PAY-TEST", items)

	order := Order{
		ID:                "PAY-TEST",
		Total:             100.00,
		InventoryReserved: true,
		ReservationIDs:    reservations,
	}

	// Test multiple times to account for random failures
	successCount := 0
	failureCount := 0

	for i := 0; i < 20; i++ {
		testOrder := order
		testOrder.ID = fmt.Sprintf("PAY-TEST-%d", i)

		// Reserve new inventory for this test
		res, _ := inventory.Reserve(testOrder.ID, items)
		testOrder.ReservationIDs = res

		result, err := processor(ctx, testOrder)

		if err != nil {
			failureCount++
			// Check inventory was released on failure
			if result.InventoryReserved {
				t.Error("Expected inventory to be released on payment failure")
			}
		} else {
			successCount++
			if result.PaymentID == "" {
				t.Error("Expected payment ID")
			}
			if result.PaymentStatus != PaymentSuccess {
				t.Error("Expected payment success status")
			}
			// Cleanup successful reservations
			inventory.ReleaseReservations(result.ReservationIDs)
		}
	}

	// With 10% failure rate, we expect some failures
	if failureCount == 0 {
		t.Log("Warning: No payment failures in test (expected ~10% failure rate)")
	}

	if successCount == 0 {
		t.Error("All payments failed (expected ~90% success rate)")
	}
}

func TestFulfillmentRouting(t *testing.T) {
	shipping := NewShippingService()

	tests := []struct {
		name            string
		orderType       OrderType
		expectedCarrier string
	}{
		{
			name:            "Express order",
			orderType:       OrderTypeExpress,
			expectedCarrier: "FedEx Priority",
		},
		{
			name:            "Standard order",
			orderType:       OrderTypeStandard,
			expectedCarrier: "USPS Ground",
		},
		{
			name:            "Digital order",
			orderType:       OrderTypeDigital,
			expectedCarrier: "Digital Delivery",
		},
	}

	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			order := Order{
				ID:   fmt.Sprintf("FULFILL-%s", tt.orderType),
				Type: tt.orderType,
				Items: []OrderItem{
					{ProductID: "P1", Name: "Product", Quantity: 1},
				},
			}

			var result Order
			var err error

			switch tt.orderType {
			case OrderTypeExpress:
				handler := FulfillExpressOrder(shipping)
				result, err = handler(ctx, order)
			case OrderTypeStandard:
				handler := FulfillStandardOrder(shipping)
				result, err = handler(ctx, order)
			case OrderTypeDigital:
				result, err = FulfillDigitalOrder(ctx, order)
			}

			if err != nil {
				t.Fatalf("Fulfillment failed: %v", err)
			}

			if result.ShippingCarrier != tt.expectedCarrier {
				t.Errorf("Expected carrier %s, got %s", tt.expectedCarrier, result.ShippingCarrier)
			}

			if tt.orderType != OrderTypeDigital && result.ShippingLabel == "" {
				t.Error("Expected shipping label for physical orders")
			}

			if result.TrackingNumber == "" {
				t.Error("Expected tracking number")
			}
		})
	}
}

func TestFullOrderPipeline(t *testing.T) {
	// Reset global stats to avoid test pollution
	ResetStats()
	
	// Create services
	inventory := NewInventoryService()
	fraud := NewFraudService()
	payment := NewPaymentService()
	shipping := NewShippingService()

	// Create pipeline
	pipeline := CreateOrderPipeline(inventory, fraud, payment, shipping)

	ctx := context.Background()

	tests := []struct {
		name          string
		orderType     OrderType
		customerID    string
		expectSuccess bool
	}{
		{
			name:          "Standard order success",
			orderType:     OrderTypeStandard,
			customerID:    "GOOD-CUSTOMER",
			expectSuccess: true,
		},
		{
			name:          "Express order success",
			orderType:     OrderTypeExpress,
			customerID:    "VIP-CUSTOMER",
			expectSuccess: true,
		},
		{
			name:          "Digital order success",
			orderType:     OrderTypeDigital,
			customerID:    "DIGITAL-CUSTOMER",
			expectSuccess: true,
		},
		{
			name:          "Fraudulent order",
			orderType:     OrderTypeStandard,
			customerID:    "FRAUD-USER-001",
			expectSuccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			order := CreateTestOrder(tt.orderType, tt.customerID)

			result, err := pipeline.Process(ctx, order)

			if tt.expectSuccess {
				if err != nil {
					t.Errorf("Expected success, got error: %v", err)
				}

				if result.Status != StatusCompleted {
					t.Errorf("Expected completed status, got %s", result.Status)
				}

				if result.PaymentStatus != PaymentSuccess {
					t.Error("Expected successful payment")
				}

				if result.TrackingNumber == "" {
					t.Error("Expected tracking number")
				}
			} else {
				if err == nil {
					t.Error("Expected failure for fraudulent order")
				}
			}
		})
	}
}

func TestCompensatingPipeline(t *testing.T) {
	inventory := NewInventoryService()
	payment := NewPaymentService()

	compensator := CreateCompensatingPipeline(inventory, payment)

	ctx := context.Background()

	t.Run("Full rollback", func(t *testing.T) {
		// Create an order with reservations and payment
		items := []OrderItem{
			{ProductID: "PROD-001", Name: "Widget", Quantity: 5},
		}
		reservations, _ := inventory.Reserve("ROLLBACK-001", items)

		// Process a payment
		order := Order{
			ID:                "ROLLBACK-001",
			CustomerID:        "TEST-USER",
			Total:             100.00,
			InventoryReserved: true,
			ReservationIDs:    reservations,
		}

		// Manually process payment for test
		paymentID, _ := payment.ProcessPayment(order)
		order.PaymentID = paymentID
		order.PaymentStatus = PaymentSuccess

		// Run compensating pipeline
		result, err := compensator.Process(ctx, order)
		if err != nil {
			t.Fatalf("Compensation failed: %v", err)
		}

		if result.Status != StatusCancelled {
			t.Errorf("Expected cancelled status, got %s", result.Status)
		}

		if result.InventoryReserved {
			t.Error("Expected inventory to be released")
		}

		if result.PaymentStatus != PaymentRefunded {
			t.Error("Expected payment to be refunded")
		}
	})

	t.Run("Partial rollback - no payment", func(t *testing.T) {
		// Order with only inventory reserved
		items := []OrderItem{
			{ProductID: "PROD-002", Name: "Gadget", Quantity: 2},
		}
		reservations, _ := inventory.Reserve("ROLLBACK-002", items)

		order := Order{
			ID:                "ROLLBACK-002",
			CustomerID:        "TEST-USER",
			InventoryReserved: true,
			ReservationIDs:    reservations,
			PaymentStatus:     PaymentFailed,
		}

		result, err := compensator.Process(ctx, order)
		if err != nil {
			t.Fatalf("Compensation failed: %v", err)
		}

		if result.InventoryReserved {
			t.Error("Expected inventory to be released")
		}

		if result.Status != StatusCancelled {
			t.Error("Expected cancelled status")
		}
	})
}

func TestOrderStats(t *testing.T) {
	// Reset stats
	ResetStats()

	// Update with various orders
	orders := []Order{
		{Type: OrderTypeStandard, Status: StatusCompleted, Total: 100.00,
			CreatedAt: time.Now().Add(-5 * time.Minute), CompletedAt: time.Now()},
		{Type: OrderTypeExpress, Status: StatusCompleted, Total: 200.00,
			CreatedAt: time.Now().Add(-3 * time.Minute), CompletedAt: time.Now()},
		{Type: OrderTypeDigital, Status: StatusCompleted, Total: 50.00,
			CreatedAt: time.Now().Add(-1 * time.Minute), CompletedAt: time.Now()},
		{Type: OrderTypeStandard, Status: StatusCancelled, Total: 150.00},
	}

	for _, order := range orders {
		UpdateStats(order)
	}

	stats := GetStats()

	if stats.Total != 4 {
		t.Errorf("Expected 4 total orders, got %d", stats.Total)
	}

	if stats.ByType[OrderTypeStandard] != 2 {
		t.Errorf("Expected 2 standard orders, got %d", stats.ByType[OrderTypeStandard])
	}

	if stats.ByStatus[StatusCompleted] != 3 {
		t.Errorf("Expected 3 completed orders, got %d", stats.ByStatus[StatusCompleted])
	}

	expectedRevenue := 350.00 // Only completed orders count (100 + 200 + 50)
	if stats.TotalRevenue != expectedRevenue {
		t.Errorf("Expected revenue %.2f, got %.2f", expectedRevenue, stats.TotalRevenue)
	}

	if stats.ProcessingTime == 0 {
		t.Error("Expected processing time to be calculated")
	}
}

func TestConcurrentOrders(t *testing.T) {
	// Reset global stats to avoid test pollution
	ResetStats()
	
	// Create services
	inventory := NewInventoryService()
	fraud := NewFraudService()
	payment := NewPaymentService()
	shipping := NewShippingService()

	pipeline := CreateOrderPipeline(inventory, fraud, payment, shipping)

	ctx := context.Background()

	// Process multiple orders concurrently
	orderCount := 10
	results := make([]Order, orderCount)
	errors := make([]error, orderCount)

	done := make(chan bool, orderCount)

	for i := 0; i < orderCount; i++ {
		go func(idx int) {
			order := CreateTestOrder(OrderTypeStandard, fmt.Sprintf("CONCURRENT-%d", idx))
			order.ID = fmt.Sprintf("CONCURRENT-%d", idx)

			results[idx], errors[idx] = pipeline.Process(ctx, order)
			done <- true
		}(i)
	}

	// Wait for all
	for i := 0; i < orderCount; i++ {
		<-done
	}

	// Check results
	successCount := 0
	for i, err := range errors {
		if err == nil {
			successCount++
			if results[i].Status != StatusCompleted {
				t.Errorf("Order %d: expected completed status, got %s", i, results[i].Status)
			}
		}
	}

	// Expect most orders to succeed
	if successCount < orderCount*7/10 { // 70% success rate
		t.Errorf("Too many failures: %d/%d succeeded", successCount, orderCount)
	}
}

func TestEdgeCases(t *testing.T) {
	ctx := context.Background()

	t.Run("Zero price item", func(t *testing.T) {
		order := Order{
			CustomerID: "TEST",
			Type:       OrderTypeStandard,
			Items: []OrderItem{
				{ProductID: "P1", Name: "Free Sample", Quantity: 1, Price: 0},
			},
			ShippingAddr: Address{Street: "123 Main", City: "SF"},
		}

		_, err := ValidateOrder(ctx, order)
		if err == nil {
			t.Error("Expected error for zero price item")
		}
	})

	t.Run("Negative quantity", func(t *testing.T) {
		order := Order{
			CustomerID: "TEST",
			Type:       OrderTypeStandard,
			Items: []OrderItem{
				{ProductID: "P1", Name: "Widget", Quantity: -1, Price: 10.00},
			},
			ShippingAddr: Address{Street: "123 Main", City: "SF"},
		}

		_, err := ValidateOrder(ctx, order)
		if err == nil {
			t.Error("Expected error for negative quantity")
		}
	})
}

func TestPipelineErrorHandling(t *testing.T) {
	inventory := NewInventoryService()
	fraud := NewFraudService()
	payment := NewPaymentService()

	// Create a pipeline with error injection
	errorPipeline := pipz.Sequential(
		pipz.Apply("validate", ValidateOrder),
		pipz.Apply("inject_error", func(ctx context.Context, order Order) (Order, error) {
			if strings.Contains(order.ID, "ERROR") {
				return order, fmt.Errorf("injected error for testing")
			}
			return order, nil
		}),
		pipz.Apply("parallel_checks", ParallelChecks(inventory, fraud)),
		pipz.Apply("process_payment", ProcessPayment(payment, inventory)),
	)

	ctx := context.Background()

	t.Run("Error propagation", func(t *testing.T) {
		order := CreateTestOrder(OrderTypeStandard, "TEST-USER")
		order.ID = "ERROR-TEST"

		_, err := errorPipeline.Process(ctx, order)
		if err == nil {
			t.Error("Expected error to propagate")
		}

		if !strings.Contains(err.Error(), "injected error") {
			t.Errorf("Expected injected error, got: %v", err)
		}
	})

	t.Run("Normal flow continues", func(t *testing.T) {
		order := CreateTestOrder(OrderTypeStandard, "TEST-USER")
		order.ID = "NORMAL-TEST"

		_, err := errorPipeline.Process(ctx, order)
		// May succeed or fail due to payment randomness
		if err != nil && !strings.Contains(err.Error(), "payment") {
			t.Errorf("Unexpected error: %v", err)
		}
	})
}
