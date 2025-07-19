package workflow

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/zoobzio/pipz"
)

func BenchmarkValidateOrder(b *testing.B) {
	orders := []Order{
		CreateTestOrder(OrderTypeStandard, "BENCH-USER"),
		CreateTestOrder(OrderTypeExpress, "BENCH-USER"),
		CreateTestOrder(OrderTypeDigital, "BENCH-USER"),
	}

	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		ValidateOrder(ctx, orders[i%len(orders)])
	}
}

func BenchmarkInventoryOperations(b *testing.B) {
	inventory := NewInventoryService()
	items := []OrderItem{
		{ProductID: "PROD-001", Name: "Widget", Quantity: 5},
		{ProductID: "PROD-002", Name: "Gadget", Quantity: 3},
	}

	b.Run("CheckAvailability", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			inventory.CheckAvailability(items)
		}
	})

	b.Run("Reserve", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			orderID := fmt.Sprintf("BENCH-%d", i)
			reservations, err := inventory.Reserve(orderID, items)
			if err == nil {
				// Immediately release to maintain stock
				inventory.ReleaseReservations(reservations)
			}
		}
	})

	b.Run("Release", func(b *testing.B) {
		// Pre-create reservations
		reservationSets := make([][]string, 100)
		for i := 0; i < 100; i++ {
			orderID := fmt.Sprintf("BENCH-PRE-%d", i)
			reservations, _ := inventory.Reserve(orderID, items[:1]) // Single item
			reservationSets[i] = reservations
		}

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			inventory.ReleaseReservations(reservationSets[i%100])
		}
	})
}

func BenchmarkFraudCheck(b *testing.B) {
	fraud := NewFraudService()

	scenarios := []struct {
		name  string
		order Order
	}{
		{
			name: "Normal_Order",
			order: Order{
				CustomerID: "GOOD-USER",
				Total:      100.00,
				Items:      []OrderItem{{ProductID: "P1", Quantity: 1}},
			},
		},
		{
			name: "High_Value",
			order: Order{
				CustomerID: "VIP-USER",
				Total:      5500.00,
				Items:      []OrderItem{{ProductID: "P1", Quantity: 10}},
			},
		},
		{
			name: "Many_Items",
			order: Order{
				CustomerID: "BULK-USER",
				Total:      1000.00,
				Items:      make([]OrderItem, 10),
			},
		},
	}

	for _, s := range scenarios {
		b.Run(s.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				fraud.CheckFraud(s.order)
			}
		})
	}
}

func BenchmarkParallelChecks(b *testing.B) {
	inventory := NewInventoryService()
	fraud := NewFraudService()
	checker := ParallelChecks(inventory, fraud)

	order := Order{
		ID:         "BENCH-001",
		CustomerID: "BENCH-USER",
		Total:      200.00,
		Items: []OrderItem{
			{ProductID: "PROD-001", Name: "Widget", Quantity: 2},
			{ProductID: "PROD-003", Name: "Tool", Quantity: 1},
		},
	}

	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		order.ID = fmt.Sprintf("BENCH-%d", i)
		result, err := checker(ctx, order)
		if err == nil && result.InventoryReserved {
			// Clean up
			inventory.ReleaseReservations(result.ReservationIDs)
		}
	}
}

func BenchmarkPaymentProcessing(b *testing.B) {
	payment := NewPaymentService()
	inventory := NewInventoryService()
	processor := ProcessPayment(payment, inventory)

	ctx := context.Background()

	// Pre-reserve some inventory
	items := []OrderItem{
		{ProductID: "PROD-001", Name: "Widget", Quantity: 1},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		orderID := fmt.Sprintf("PAY-BENCH-%d", i)
		reservations, _ := inventory.Reserve(orderID, items)

		order := Order{
			ID:                orderID,
			Total:             99.99,
			InventoryReserved: true,
			ReservationIDs:    reservations,
		}

		result, err := processor(ctx, order)
		if err == nil {
			// Clean up successful payment
			inventory.ReleaseReservations(result.ReservationIDs)
		}
	}
}

func BenchmarkFulfillment(b *testing.B) {
	shipping := NewShippingService()

	scenarios := []struct {
		name      string
		orderType OrderType
		handler   func(context.Context, Order) (Order, error)
	}{
		{
			name:      "Express",
			orderType: OrderTypeExpress,
			handler:   FulfillExpressOrder(shipping),
		},
		{
			name:      "Standard",
			orderType: OrderTypeStandard,
			handler:   FulfillStandardOrder(shipping),
		},
		{
			name:      "Digital",
			orderType: OrderTypeDigital,
			handler:   FulfillDigitalOrder,
		},
	}

	ctx := context.Background()

	for _, s := range scenarios {
		b.Run(s.name, func(b *testing.B) {
			order := Order{
				ID:   "FULFILL-BENCH",
				Type: s.orderType,
				Items: []OrderItem{
					{ProductID: "P1", Name: "Product", Quantity: 1},
				},
			}

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				s.handler(ctx, order)
			}
		})
	}
}

func BenchmarkFullPipeline(b *testing.B) {
	// Create services
	inventory := NewInventoryService()
	fraud := NewFraudService()
	payment := NewPaymentService()
	shipping := NewShippingService()

	pipeline := CreateOrderPipeline(inventory, fraud, payment, shipping)

	orderTypes := []OrderType{
		OrderTypeStandard,
		OrderTypeExpress,
		OrderTypeDigital,
	}

	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		order := CreateTestOrder(orderTypes[i%len(orderTypes)], "BENCH-USER")
		order.ID = fmt.Sprintf("BENCH-%d", i)

		// Suppress output
		order.ID = "bench"

		pipeline.Process(ctx, order)
	}
}

func BenchmarkCompensatingPipeline(b *testing.B) {
	inventory := NewInventoryService()
	payment := NewPaymentService()
	compensator := CreateCompensatingPipeline(inventory, payment)

	ctx := context.Background()

	b.Run("WithPayment", func(b *testing.B) {
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			// Create order with payment
			order := Order{
				ID:                fmt.Sprintf("COMP-%d", i),
				InventoryReserved: true,
				ReservationIDs:    []string{"RES-1", "RES-2"},
				PaymentID:         fmt.Sprintf("PAY-%d", i),
				PaymentStatus:     PaymentSuccess,
			}

			// Add to payment service
			payment.transactions.Store(order.PaymentID, map[string]interface{}{
				"orderID": order.ID,
				"amount":  100.00,
				"status":  PaymentSuccess,
			})

			compensator.Process(ctx, order)
		}
	})

	b.Run("NoPayment", func(b *testing.B) {
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			order := Order{
				ID:                fmt.Sprintf("COMP-NP-%d", i),
				InventoryReserved: true,
				ReservationIDs:    []string{"RES-1"},
			}

			compensator.Process(ctx, order)
		}
	})
}

func BenchmarkRouting(b *testing.B) {
	shipping := NewShippingService()

	routeFunc := func(ctx context.Context, order Order) string {
		return string(order.Type)
	}

	handlers := map[string]pipz.Chainable[Order]{
		string(OrderTypeExpress):  pipz.Apply("express", FulfillExpressOrder(shipping)),
		string(OrderTypeStandard): pipz.Apply("standard", FulfillStandardOrder(shipping)),
		string(OrderTypeDigital):  pipz.Apply("digital", FulfillDigitalOrder),
	}

	router := pipz.Switch(routeFunc, handlers)

	orders := []Order{
		{ID: "bench", Type: OrderTypeExpress, Items: []OrderItem{{ProductID: "P1", Quantity: 1}}},
		{ID: "bench", Type: OrderTypeStandard, Items: []OrderItem{{ProductID: "P1", Quantity: 1}}},
		{ID: "bench", Type: OrderTypeDigital, Items: []OrderItem{{ProductID: "P1", Quantity: 1}}},
	}

	ctx := context.Background()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		router.Process(ctx, orders[i%len(orders)])
	}
}

func BenchmarkConcurrentOrders(b *testing.B) {
	inventory := NewInventoryService()
	fraud := NewFraudService()
	payment := NewPaymentService()
	shipping := NewShippingService()

	pipeline := CreateOrderPipeline(inventory, fraud, payment, shipping)
	ctx := context.Background()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			order := CreateTestOrder(OrderTypeStandard, fmt.Sprintf("CONCURRENT-%d", i))
			order.ID = "bench" // Suppress output
			pipeline.Process(ctx, order)
			i++
		}
	})
}

func BenchmarkStats(b *testing.B) {
	// Pre-populate some stats
	for i := 0; i < 100; i++ {
		order := Order{
			Type:        OrderType([]OrderType{OrderTypeStandard, OrderTypeExpress, OrderTypeDigital}[i%3]),
			Status:      OrderStatus([]OrderStatus{StatusCompleted, StatusCancelled, StatusShipped}[i%3]),
			Total:       float64(i * 10),
			CreatedAt:   time.Now().Add(-time.Duration(i) * time.Minute),
			CompletedAt: time.Now(),
		}
		UpdateStats(order)
	}

	b.Run("Update", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			order := Order{
				Type:        OrderTypeStandard,
				Status:      StatusCompleted,
				Total:       99.99,
				CreatedAt:   time.Now().Add(-5 * time.Minute),
				CompletedAt: time.Now(),
			}
			UpdateStats(order)
		}
	})

	b.Run("Get", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			GetStats()
		}
	})
}

var globalOrder Order // Prevent optimizations

func BenchmarkMemoryUsage(b *testing.B) {
	b.Run("Order_Allocation", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			order := Order{
				ID:         fmt.Sprintf("ORD-%d", i),
				CustomerID: "CUST-001",
				Type:       OrderTypeStandard,
				Items: []OrderItem{
					{ProductID: "P1", SKU: "SKU1", Name: "Widget", Quantity: 2, Price: 10.00},
					{ProductID: "P2", SKU: "SKU2", Name: "Gadget", Quantity: 1, Price: 20.00},
				},
				ShippingAddr: Address{
					Street: "123 Main St", City: "SF", State: "CA", Zip: "94105", Country: "USA",
				},
				ReservationIDs: []string{"RES-1", "RES-2"},
				Errors:         make([]OrderError, 0, 2),
			}
			globalOrder = order
		}
	})

	b.Run("Pipeline_Processing", func(b *testing.B) {
		inventory := NewInventoryService()
		fraud := NewFraudService()
		payment := NewPaymentService()
		shipping := NewShippingService()

		pipeline := CreateOrderPipeline(inventory, fraud, payment, shipping)

		order := CreateTestOrder(OrderTypeStandard, "BENCH-USER")
		order.ID = "bench"

		ctx := context.Background()
		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			result, _ := pipeline.Process(ctx, order)
			globalOrder = result
		}
	})
}
