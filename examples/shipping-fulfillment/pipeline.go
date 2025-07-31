package main

import (
	"context"
	"fmt"
	"time"

	"github.com/zoobzio/pipz"
)

// ShippingPipeline orchestrates the shipping fulfillment process.
var ShippingPipeline *pipz.Sequence[Shipment]

// Initialize sets up the shipping pipeline with all processors and connectors.
func Initialize(providers map[string]ShippingProvider) {
	// Create the main pipeline
	ShippingPipeline = pipz.NewSequence[Shipment](PipelineShippingFulfillment)

	// Step 1: Validate shipment
	validateShipment := pipz.Apply(ProcessorValidateShipment, func(_ context.Context, shipment Shipment) (Shipment, error) {
		// Basic validation
		if shipment.Weight <= 0 {
			return shipment, fmt.Errorf("invalid weight: %f", shipment.Weight)
		}
		if shipment.From.ZIP == "" || shipment.To.ZIP == "" {
			return shipment, fmt.Errorf("missing ZIP codes")
		}
		if len(shipment.Contents) == 0 {
			return shipment, fmt.Errorf("shipment has no contents")
		}

		// Check for hazmat restrictions
		for _, item := range shipment.Contents {
			if item.HazMat {
				shipment.ProcessingLog = append(shipment.ProcessingLog,
					fmt.Sprintf("⚠️  Hazmat item detected: %s", item.Name))
			}
		}

		shipment.Status = StatusPending
		shipment.ProcessingLog = append(shipment.ProcessingLog, "✓ Shipment validated")
		return shipment, nil
	})

	// Step 2: Rate shopping with Contest
	rateShoppingContest := createRateShoppingContest(providers)

	// Step 3: Create shipping label
	createLabel := pipz.Apply(ProcessorCreateLabel, func(ctx context.Context, shipment Shipment) (Shipment, error) {
		if shipment.SelectedRate == nil {
			return shipment, fmt.Errorf("no rate selected")
		}

		// Find the provider that gave us this rate
		provider, exists := providers[shipment.SelectedRate.ProviderName]
		if !exists {
			return shipment, fmt.Errorf("provider %s not found", shipment.SelectedRate.ProviderName)
		}

		// Create label with timeout
		labelCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		label, err := provider.CreateLabel(labelCtx, shipment, *shipment.SelectedRate)
		if err != nil {
			return shipment, fmt.Errorf("label creation failed: %w", err)
		}

		shipment.Label = label
		shipment.TrackingNumber = label.TrackingNumber
		shipment.Status = StatusLabeled
		shipment.ProcessingLog = append(shipment.ProcessingLog,
			fmt.Sprintf("✓ Label created: %s", label.TrackingNumber))

		return shipment, nil
	})

	// Step 4: Dispatch shipment (simulate handoff to carrier)
	dispatchShipment := pipz.Apply(ProcessorDispatchShipment, func(_ context.Context, shipment Shipment) (Shipment, error) {
		if shipment.Label == nil {
			return shipment, fmt.Errorf("cannot dispatch without label")
		}

		// Simulate dispatch process
		time.Sleep(50 * time.Millisecond)

		shipment.Status = StatusShipped
		shipment.Timestamps["dispatched"] = time.Now()
		shipment.ProcessingLog = append(shipment.ProcessingLog,
			fmt.Sprintf("✓ Dispatched to %s", shipment.Label.ProviderName))

		return shipment, nil
	})

	// Assemble the pipeline with error handling and timeouts
	ShippingPipeline.Register(
		validateShipment,
		pipz.NewTimeout(ConnectorShipmentTimeout, rateShoppingContest, 3*time.Second),
		pipz.NewRetry(ConnectorProviderRetry, createLabel, 3),
		dispatchShipment,
	)
}

// createRateShoppingContest creates the Contest connector for finding the best shipping rate.
func createRateShoppingContest(providers map[string]ShippingProvider) pipz.Chainable[Shipment] {
	// Create processors for each provider
	rateProcessors := make([]pipz.Chainable[Shipment], 0, len(providers))

	for name, provider := range providers {
		// Capture provider in closure
		p := provider
		providerName := name

		processor := pipz.Apply(fmt.Sprintf("%s_rates", providerName),
			func(ctx context.Context, shipment Shipment) (Shipment, error) {
				// Track attempt
				shipment.AttemptedProviders = append(shipment.AttemptedProviders, providerName)

				// Ensure map is initialized
				if shipment.FailedProviders == nil {
					shipment.FailedProviders = make(map[string]string)
				}

				// Get rates from provider
				rates, err := p.GetRates(ctx, shipment)
				if err != nil {
					shipment.FailedProviders[providerName] = err.Error()
					return shipment, fmt.Errorf("%s rate query failed: %w", providerName, err)
				}

				// Find the best rate from this provider based on service level
				var bestRate *Rate
				for i := range rates {
					rate := &rates[i]

					// Filter by service level requirements
					if !meetsServiceLevel(shipment, rate) {
						continue
					}

					// Select cheapest rate that meets requirements
					if bestRate == nil || rate.Cost < bestRate.Cost {
						bestRate = rate
					}
				}

				if bestRate == nil {
					return shipment, fmt.Errorf("%s has no rates meeting requirements", providerName)
				}

				// Update shipment with this provider's best rate
				shipment.SelectedRate = bestRate
				shipment.ProcessingLog = append(shipment.ProcessingLog,
					fmt.Sprintf("📦 %s offered: %s - $%.2f (%d days)",
						providerName, bestRate.ServiceName, bestRate.Cost, bestRate.EstimatedDays))

				return shipment, nil
			})

		rateProcessors = append(rateProcessors, processor)
	}

	// Define the winning condition for Contest
	// We want the cheapest rate that meets our delivery requirements
	acceptableRateCondition := func(_ context.Context, shipment Shipment) bool {
		if shipment.SelectedRate == nil {
			return false
		}

		// Define acceptance criteria
		maxAcceptableCost := 50.0 // Maximum we're willing to pay

		// For expedited shipping, relax the cost constraint
		if shipment.ServiceLevel >= ServiceExpedited {
			maxAcceptableCost = 100.0
		}

		// Check if rate meets criteria
		return shipment.SelectedRate.Cost <= maxAcceptableCost
	}

	// Create Contest connector
	contest := pipz.NewContest(ConnectorProviderRace, acceptableRateCondition, rateProcessors...)

	// Wrap with processor to handle no-winner scenario
	return pipz.Apply(ProcessorCalculateRates, func(ctx context.Context, shipment Shipment) (Shipment, error) {
		result, err := contest.Process(ctx, shipment)

		if err != nil {
			// No acceptable rate found - find the cheapest regardless
			return selectCheapestAvailable(shipment)
		}

		// Contest found an acceptable rate
		result.Status = StatusRated
		if result.SelectedRate != nil {
			result.ProcessingLog = append(result.ProcessingLog,
				fmt.Sprintf("✅ Selected: %s %s - $%.2f",
					result.SelectedRate.ProviderName,
					result.SelectedRate.ServiceName,
					result.SelectedRate.Cost))
		}

		return result, nil
	})
}

// meetsServiceLevel checks if a rate meets the shipment's service level requirements.
func meetsServiceLevel(shipment Shipment, rate *Rate) bool {
	switch shipment.ServiceLevel {
	case ServiceSameDay:
		return rate.EstimatedDays == 0
	case ServiceOvernight:
		return rate.EstimatedDays <= 1
	case ServiceExpedited:
		return rate.EstimatedDays <= 3
	case ServiceStandard:
		return rate.EstimatedDays <= 5
	case ServiceInternational:
		return true // Any international service
	default:
		return true
	}
}

// selectCheapestAvailable falls back to selecting the cheapest rate when Contest finds no acceptable rates.
func selectCheapestAvailable(shipment Shipment) (Shipment, error) {
	// Parse from processing log (in a real system, we'd store these properly)
	// For this example, we'll just select a fallback rate
	if len(shipment.AttemptedProviders) == 0 {
		return shipment, fmt.Errorf("no providers available")
	}

	// Simulate selecting the cheapest available
	shipment.SelectedRate = &Rate{
		ProviderName:  "USPS", // Typically cheapest
		ServiceName:   "USPS Ground Advantage",
		Cost:          75.00, // Higher than our ideal threshold
		EstimatedDays: 5,
		Currency:      "USD",
	}

	shipment.Status = StatusRated
	shipment.ProcessingLog = append(shipment.ProcessingLog,
		"⚠️  No rates met criteria - selected cheapest available")

	return shipment, nil
}

// ProcessShipment runs a shipment through the pipeline.
func ProcessShipment(ctx context.Context, shipment Shipment) (Shipment, error) {
	// Initialize tracking
	shipment.CreatedAt = time.Now()
	shipment.ProcessingLog = []string{"🚀 Starting shipping fulfillment"}
	shipment.Timestamps = map[string]time.Time{
		"started": time.Now(),
	}
	shipment.AttemptedProviders = []string{}
	shipment.FailedProviders = make(map[string]string)

	// Process through pipeline
	result, err := ShippingPipeline.Process(ctx, shipment)

	// Record completion
	if result.Timestamps == nil {
		result.Timestamps = make(map[string]time.Time)
	}
	result.Timestamps["completed"] = time.Now()
	duration := time.Since(shipment.CreatedAt)
	result.ProcessingLog = append(result.ProcessingLog,
		fmt.Sprintf("⏱️  Total processing time: %v", duration))

	return result, err
}

// GenerateShipmentReport creates a detailed report of the shipment processing.
func GenerateShipmentReport(shipment Shipment) string {
	report := "\n=== Shipment Report ===\n"
	report += fmt.Sprintf("ID: %s\n", shipment.ID)
	report += fmt.Sprintf("Status: %s\n", shipment.Status)

	if shipment.TrackingNumber != "" {
		report += fmt.Sprintf("Tracking: %s\n", shipment.TrackingNumber)
	}

	if shipment.SelectedRate != nil {
		report += "\nSelected Service:\n"
		report += fmt.Sprintf("  Provider: %s\n", shipment.SelectedRate.ProviderName)
		report += fmt.Sprintf("  Service: %s\n", shipment.SelectedRate.ServiceName)
		report += fmt.Sprintf("  Cost: $%.2f\n", shipment.SelectedRate.Cost)
		report += fmt.Sprintf("  Delivery: %d days\n", shipment.SelectedRate.EstimatedDays)
	}

	report += fmt.Sprintf("\nProviders Tried: %d\n", len(shipment.AttemptedProviders))
	for _, p := range shipment.AttemptedProviders {
		if err, failed := shipment.FailedProviders[p]; failed {
			report += fmt.Sprintf("  ❌ %s: %s\n", p, err)
		} else {
			report += fmt.Sprintf("  ✓ %s\n", p)
		}
	}

	report += "\nProcessing Log:\n"
	for _, log := range shipment.ProcessingLog {
		report += fmt.Sprintf("  %s\n", log)
	}

	return report
}
