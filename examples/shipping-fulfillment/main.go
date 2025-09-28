package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/zoobzio/pipz"
)

func main() {
	// Initialize random seed (removed - no longer needed in Go 1.20+)

	fmt.Println("🚢 Shipping Fulfillment Example")
	fmt.Println("===============================")
	fmt.Println()
	fmt.Println("This example demonstrates pipz's adapter pattern and Contest connector")
	fmt.Println("for e-commerce shipping fulfillment.")
	fmt.Println()

	// Initialize shipping providers
	providers := map[string]ShippingProvider{
		"FedEx":        NewFedExProvider(),
		"UPS":          NewUPSProvider(),
		"USPS":         NewUSPSProvider(),
		"LocalCourier": NewLocalCourierProvider(),
	}

	// Initialize the pipeline
	Initialize(providers)

	// Create test shipments
	shipments := createTestShipments()

	// Process each shipment
	metrics := &ShippingMetrics{
		ProviderBreakdown: make(map[string]int),
		ProviderCosts:     make(map[string]float64),
		ServiceBreakdown:  make(map[ServiceLevel]int),
		FastestShipment:   time.Hour, // Start with high value
	}

	for i := range shipments {
		fmt.Printf("\n📦 Processing Shipment %d/%d\n", i+1, len(shipments))
		fmt.Printf("   Weight: %.1f lbs, From: %s, To: %s\n",
			shipments[i].Weight, shipments[i].From.State, shipments[i].To.State)
		fmt.Printf("   Service Level: %s\n", shipments[i].ServiceLevel)

		// Process the shipment
		start := time.Now()
		result, err := ProcessShipment(context.Background(), shipments[i])
		duration := time.Since(start)

		// Update metrics
		updateMetrics(metrics, result, duration, err)

		// Display results
		if err != nil {
			var pipeErr *pipz.Error[Shipment]
			if errors.As(err, &pipeErr) {
				fmt.Printf("   ❌ Failed: %v\n", pipeErr.Err)
				pathStrs := make([]string, len(pipeErr.Path))
				for i, name := range pipeErr.Path {
					pathStrs[i] = string(name)
				}
				fmt.Printf("   Path: %s\n", strings.Join(pathStrs, " → "))
			} else {
				fmt.Printf("   ❌ Failed: %v\n", err)
			}
		} else {
			fmt.Printf("   ✅ Success: %s via %s\n",
				result.TrackingNumber, result.SelectedRate.ProviderName)
			fmt.Printf("   Cost: $%.2f, Delivery: %d days\n",
				result.SelectedRate.Cost, result.SelectedRate.EstimatedDays)
		}

		// Show detailed report for first shipment
		if i == 0 {
			fmt.Println(GenerateShipmentReport(result))
		}

		// Small delay between shipments for readability
		time.Sleep(100 * time.Millisecond)
	}

	// Display final metrics
	fmt.Println(metrics.GenerateReport())

	// Demonstrate Contest vs Race difference
	demonstrateContestBenefit()
}

// createTestShipments generates various test shipments.
func createTestShipments() []Shipment {
	return []Shipment{
		// Standard domestic shipment
		{
			ID:         "SHIP001",
			OrderID:    "ORD001",
			CustomerID: "CUST001",
			Weight:     5.5,
			Dimensions: Dimensions{Length: 12, Width: 8, Height: 6},
			Value:      99.99,
			Contents: []Item{
				{SKU: "WIDGET-001", Name: "Premium Widget", Quantity: 2, Value: 49.99},
			},
			From: Address{
				Name:    "Warehouse A",
				Street1: "123 Commerce St",
				City:    "Seattle",
				State:   "WA",
				ZIP:     "98101",
				Country: "US",
			},
			To: Address{
				Name:    "John Doe",
				Street1: "456 Main St",
				City:    "Portland",
				State:   "OR",
				ZIP:     "97201",
				Country: "US",
			},
			ServiceLevel: ServiceStandard,
		},
		// Expedited shipment
		{
			ID:         "SHIP002",
			OrderID:    "ORD002",
			CustomerID: "CUST002",
			Weight:     2.0,
			Dimensions: Dimensions{Length: 8, Width: 6, Height: 4},
			Value:      199.99,
			Contents: []Item{
				{SKU: "GADGET-002", Name: "Smart Gadget", Quantity: 1, Value: 199.99, Fragile: true},
			},
			From: Address{
				State: "CA",
				ZIP:   "90210",
			},
			To: Address{
				State: "NY",
				ZIP:   "10001",
			},
			ServiceLevel: ServiceExpedited,
		},
		// Local same-day delivery
		{
			ID:         "SHIP003",
			OrderID:    "ORD003",
			CustomerID: "CUST003",
			Weight:     1.0,
			Dimensions: Dimensions{Length: 6, Width: 4, Height: 2},
			Value:      49.99,
			Contents: []Item{
				{SKU: "URGENT-003", Name: "Urgent Item", Quantity: 1, Value: 49.99},
			},
			From: Address{
				State: "WA",
				ZIP:   "98101",
			},
			To: Address{
				State: "WA",
				ZIP:   "98105",
			},
			ServiceLevel: ServiceSameDay,
		},
		// Heavy shipment
		{
			ID:         "SHIP004",
			OrderID:    "ORD004",
			CustomerID: "CUST004",
			Weight:     25.0,
			Dimensions: Dimensions{Length: 24, Width: 18, Height: 12},
			Value:      299.99,
			Contents: []Item{
				{SKU: "HEAVY-004", Name: "Industrial Equipment", Quantity: 1, Value: 299.99},
			},
			From: Address{
				State: "TX",
				ZIP:   "75001",
			},
			To: Address{
				State: "FL",
				ZIP:   "33101",
			},
			ServiceLevel: ServiceStandard,
		},
		// Overnight delivery
		{
			ID:         "SHIP005",
			OrderID:    "ORD005",
			CustomerID: "CUST005",
			Weight:     0.5,
			Dimensions: Dimensions{Length: 4, Width: 3, Height: 1},
			Value:      999.99,
			Contents: []Item{
				{SKU: "PRIORITY-005", Name: "Priority Document", Quantity: 1, Value: 999.99},
			},
			From: Address{
				State: "IL",
				ZIP:   "60601",
			},
			To: Address{
				State: "MA",
				ZIP:   "02101",
			},
			ServiceLevel: ServiceOvernight,
		},
	}
}

// updateMetrics updates shipping metrics based on results.
func updateMetrics(metrics *ShippingMetrics, shipment Shipment, duration time.Duration, err error) {
	metrics.TotalShipments++

	if err == nil {
		metrics.SuccessfulShipments++

		if shipment.SelectedRate != nil {
			// Update provider metrics
			provider := shipment.SelectedRate.ProviderName
			metrics.ProviderBreakdown[provider]++
			metrics.ProviderCosts[provider] += shipment.SelectedRate.Cost
			metrics.TotalCost += shipment.SelectedRate.Cost

			// Update service level breakdown
			metrics.ServiceBreakdown[shipment.ServiceLevel]++
		}

		// Track timing
		metrics.AverageProcessTime = (metrics.AverageProcessTime*time.Duration(metrics.TotalShipments-1) + duration) / time.Duration(metrics.TotalShipments)
		if duration < metrics.FastestShipment {
			metrics.FastestShipment = duration
		}
		if duration > metrics.SlowestShipment {
			metrics.SlowestShipment = duration
		}

		// Success rates
		metrics.RateSuccessRate = float64(len(shipment.AttemptedProviders)-len(shipment.FailedProviders)) / float64(len(shipment.AttemptedProviders))
		if shipment.Label != nil {
			metrics.LabelSuccessRate = 1.0
		}
	} else {
		metrics.FailedShipments++
	}

	// Calculate average cost
	if metrics.SuccessfulShipments > 0 {
		metrics.AverageCost = metrics.TotalCost / float64(metrics.SuccessfulShipments)
	}
}

// demonstrateContestBenefit shows the advantage of Contest over Race.
func demonstrateContestBenefit() {
	fmt.Println("\n📊 Contest vs Race Comparison")
	fmt.Println("=============================")
	fmt.Println()
	fmt.Println("Contest Behavior:")
	fmt.Println("- Runs all providers in parallel")
	fmt.Println("- Returns FIRST result meeting criteria (e.g., < $50)")
	fmt.Println("- Cancels remaining providers once winner found")
	fmt.Println()
	fmt.Println("Example Timeline:")
	fmt.Println("  0ms:   All providers start")
	fmt.Println("  50ms:  LocalCourier returns $45 (✅ meets criteria, wins!)")
	fmt.Println("  100ms: UPS would return $38 (better price, but too late)")
	fmt.Println("  150ms: FedEx would return $52 (wouldn't qualify anyway)")
	fmt.Println("  250ms: USPS would return $28 (best price, but too slow)")
	fmt.Println()
	fmt.Println("With Race: Would get LocalCourier @ $45 (fastest)")
	fmt.Println("With Contest: Get LocalCourier @ $45 (fastest acceptable)")
	fmt.Println("With Sequential: Would need 550ms to check all providers")
	fmt.Println()
	fmt.Println("✨ Contest gives us the best of both worlds:")
	fmt.Println("   - Fast response time (50ms vs 550ms)")
	fmt.Println("   - Quality guarantee (price < $50)")
	fmt.Println("   - Resource efficient (cancels slower providers)")
}
