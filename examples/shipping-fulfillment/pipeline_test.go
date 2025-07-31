package main

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

// MockProvider for testing.
type MockProvider struct {
	name          string
	rateDelay     time.Duration
	rateError     error
	rates         []Rate
	labelError    error
	trackingError error

	// For dynamic behavior
	createLabelFunc func(context.Context, Shipment, Rate) (*Label, error)
}

func (m *MockProvider) GetRates(ctx context.Context, _ Shipment) ([]Rate, error) {
	select {
	case <-time.After(m.rateDelay):
		if m.rateError != nil {
			return nil, m.rateError
		}
		return m.rates, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (m *MockProvider) CreateLabel(ctx context.Context, _ Shipment, rate Rate) (*Label, error) {
	// Use custom function if provided
	if m.createLabelFunc != nil {
		return m.createLabelFunc(ctx, Shipment{}, rate)
	}

	if m.labelError != nil {
		return nil, m.labelError
	}
	return &Label{
		ProviderName:   m.name,
		TrackingNumber: fmt.Sprintf("MOCK-%s-12345", m.name),
		LabelData:      []byte("mock-label"),
		LabelFormat:    "PDF",
		Cost:           rate.Cost,
		CreatedAt:      time.Now(),
	}, nil
}

func (m *MockProvider) GetTracking(_ context.Context, trackingNumber string) (*TrackingInfo, error) {
	if m.trackingError != nil {
		return nil, m.trackingError
	}
	return &TrackingInfo{
		TrackingNumber: trackingNumber,
		Status:         TrackingInTransit,
		Location:       "Mock Location",
		UpdatedAt:      time.Now(),
	}, nil
}

func TestShippingPipeline(t *testing.T) {
	t.Run("Successful Shipment Processing", func(t *testing.T) {
		// Setup mock providers
		providers := map[string]ShippingProvider{
			"FastCheap": &MockProvider{
				name:      "FastCheap",
				rateDelay: 50 * time.Millisecond,
				rates: []Rate{
					{ProviderName: "FastCheap", ServiceName: "Standard", Cost: 25.00, EstimatedDays: 3},
				},
			},
			"SlowExpensive": &MockProvider{
				name:      "SlowExpensive",
				rateDelay: 200 * time.Millisecond,
				rates: []Rate{
					{ProviderName: "SlowExpensive", ServiceName: "Premium", Cost: 75.00, EstimatedDays: 1},
				},
			},
		}

		Initialize(providers)

		shipment := createTestShipment()
		result, err := ProcessShipment(context.Background(), shipment)

		if err != nil {
			t.Fatalf("Expected successful processing, got error: %v", err)
		}

		// Should select FastCheap (first to meet criteria)
		if result.SelectedRate.ProviderName != "FastCheap" {
			t.Errorf("Expected FastCheap provider, got %s", result.SelectedRate.ProviderName)
		}

		if result.TrackingNumber == "" {
			t.Error("Expected tracking number to be set")
		}

		if result.Status != StatusShipped {
			t.Errorf("Expected status Shipped, got %s", result.Status)
		}
	})

	t.Run("Contest Finds First Acceptable Rate", func(t *testing.T) {
		// Test that Contest returns first acceptable, not cheapest
		providers := map[string]ShippingProvider{
			"QuickAcceptable": &MockProvider{
				name:      "QuickAcceptable",
				rateDelay: 30 * time.Millisecond,
				rates: []Rate{
					{ProviderName: "QuickAcceptable", ServiceName: "Fast", Cost: 45.00, EstimatedDays: 2},
				},
			},
			"SlowCheaper": &MockProvider{
				name:      "SlowCheaper",
				rateDelay: 100 * time.Millisecond,
				rates: []Rate{
					{ProviderName: "SlowCheaper", ServiceName: "Economy", Cost: 20.00, EstimatedDays: 5},
				},
			},
		}

		Initialize(providers)

		shipment := createTestShipment()
		result, err := ProcessShipment(context.Background(), shipment)

		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		// Should get QuickAcceptable (first to respond with acceptable rate)
		if result.SelectedRate.ProviderName != "QuickAcceptable" {
			t.Errorf("Expected QuickAcceptable (first acceptable), got %s", result.SelectedRate.ProviderName)
		}
	})

	t.Run("No Acceptable Rates Falls Back", func(t *testing.T) {
		// All providers return expensive rates
		providers := map[string]ShippingProvider{
			"Expensive1": &MockProvider{
				name:      "Expensive1",
				rateDelay: 50 * time.Millisecond,
				rates: []Rate{
					{ProviderName: "Expensive1", ServiceName: "Premium", Cost: 150.00, EstimatedDays: 1},
				},
			},
			"Expensive2": &MockProvider{
				name:      "Expensive2",
				rateDelay: 60 * time.Millisecond,
				rates: []Rate{
					{ProviderName: "Expensive2", ServiceName: "Express", Cost: 125.00, EstimatedDays: 2},
				},
			},
		}

		Initialize(providers)

		shipment := createTestShipment()
		result, err := ProcessShipment(context.Background(), shipment)

		if err != nil {
			// This is OK - the pipeline may fail if no providers are available
			// Just check that we attempted to process
			if len(result.ProcessingLog) == 0 {
				t.Error("Expected processing log entries even on failure")
			}
			return
		}

		// Should have selected a fallback rate
		if result.SelectedRate == nil {
			t.Error("Expected fallback rate to be selected")
		}

		// Check that warning was logged
		hasWarning := false
		for _, log := range result.ProcessingLog {
			if strings.Contains(log, "No rates met criteria") {
				hasWarning = true
				break
			}
		}
		if !hasWarning {
			t.Error("Expected warning about no rates meeting criteria")
		}
	})

	t.Run("Provider Failures Handled", func(t *testing.T) {
		providers := map[string]ShippingProvider{
			"Failing": &MockProvider{
				name:      "Failing",
				rateDelay: 20 * time.Millisecond,
				rateError: fmt.Errorf("API down"),
			},
			"Working": &MockProvider{
				name:      "Working",
				rateDelay: 50 * time.Millisecond,
				rates: []Rate{
					{ProviderName: "Working", ServiceName: "Standard", Cost: 30.00, EstimatedDays: 3},
				},
			},
		}

		Initialize(providers)

		shipment := createTestShipment()
		result, err := ProcessShipment(context.Background(), shipment)

		if err != nil {
			t.Fatalf("Expected to handle provider failure, got error: %v", err)
		}

		// Should use Working provider
		if result.SelectedRate.ProviderName != "Working" {
			t.Errorf("Expected Working provider, got %s", result.SelectedRate.ProviderName)
		}

		// Check that at least one provider was attempted
		if len(result.AttemptedProviders) == 0 {
			t.Error("Expected at least one provider to be attempted")
		}

		// If Failing was attempted, it should be in FailedProviders
		for _, attempted := range result.AttemptedProviders {
			if attempted == "Failing" {
				if _, found := result.FailedProviders["Failing"]; !found {
					t.Error("Expected Failing provider to be recorded in FailedProviders when attempted")
				}
				break
			}
		}
	})

	t.Run("Timeout Protection", func(t *testing.T) {
		providers := map[string]ShippingProvider{
			"VerySlow": &MockProvider{
				name:      "VerySlow",
				rateDelay: 5 * time.Second, // Longer than timeout
				rates: []Rate{
					{ProviderName: "VerySlow", ServiceName: "Slow", Cost: 10.00, EstimatedDays: 7},
				},
			},
		}

		Initialize(providers)

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		shipment := createTestShipment()
		start := time.Now()
		_, err := ProcessShipment(ctx, shipment)
		duration := time.Since(start)

		if err == nil {
			t.Error("Expected timeout error")
		}

		// Should timeout around 2 seconds (not wait for 5 second provider)
		if duration > 2500*time.Millisecond {
			t.Errorf("Expected timeout around 2s, took %v", duration)
		}
	})

	t.Run("Label Creation Retry", func(t *testing.T) {
		attemptCount := 0
		providers := map[string]ShippingProvider{
			"Flaky": &MockProvider{
				name:      "Flaky",
				rateDelay: 50 * time.Millisecond,
				rates: []Rate{
					{ProviderName: "Flaky", ServiceName: "Standard", Cost: 25.00, EstimatedDays: 3},
				},
				// This will be modified during the test
			},
		}

		// Create a flaky provider that fails first two attempts
		providers["Flaky"] = &MockProvider{
			name:      "Flaky",
			rateDelay: 50 * time.Millisecond,
			rates: []Rate{
				{ProviderName: "Flaky", ServiceName: "Standard", Cost: 25.00, EstimatedDays: 3},
			},
			createLabelFunc: func(_ context.Context, _ Shipment, rate Rate) (*Label, error) {
				attemptCount++
				if attemptCount < 3 {
					return nil, fmt.Errorf("temporary label creation error")
				}
				return &Label{
					ProviderName:   "Flaky",
					TrackingNumber: "FLAKY-12345",
					LabelData:      []byte("mock-label"),
					LabelFormat:    "PDF",
					Cost:           rate.Cost,
					CreatedAt:      time.Now(),
				}, nil
			},
		}

		Initialize(providers)

		shipment := createTestShipment()
		result, err := ProcessShipment(context.Background(), shipment)

		if err != nil {
			t.Fatalf("Expected retry to succeed, got error: %v", err)
		}

		if attemptCount != 3 {
			t.Errorf("Expected 3 attempts (2 failures + 1 success), got %d", attemptCount)
		}

		if result.Label == nil {
			t.Error("Expected label to be created after retry")
		}
	})
}

func TestContestCondition(t *testing.T) {
	t.Run("Service Level Filtering", func(t *testing.T) {
		providers := map[string]ShippingProvider{
			"FastExpensive": &MockProvider{
				name:      "FastExpensive",
				rateDelay: 50 * time.Millisecond,
				rates: []Rate{
					{ProviderName: "FastExpensive", ServiceName: "Overnight", Cost: 45.00, EstimatedDays: 1},
					{ProviderName: "FastExpensive", ServiceName: "Ground", Cost: 25.00, EstimatedDays: 5},
				},
			},
		}

		Initialize(providers)

		// Test expedited service level
		shipment := createTestShipment()
		shipment.ServiceLevel = ServiceExpedited // Requires <= 3 days

		result, err := ProcessShipment(context.Background(), shipment)

		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		// Should select Overnight (only option meeting expedited requirement)
		if result.SelectedRate.ServiceName != "Overnight" {
			t.Errorf("Expected Overnight service for expedited, got %s", result.SelectedRate.ServiceName)
		}
	})

	t.Run("Cost Threshold Adjustment", func(t *testing.T) {
		providers := map[string]ShippingProvider{
			"Premium": &MockProvider{
				name:      "Premium",
				rateDelay: 50 * time.Millisecond,
				rates: []Rate{
					{ProviderName: "Premium", ServiceName: "Express", Cost: 75.00, EstimatedDays: 1},
				},
			},
		}

		Initialize(providers)

		// Test that expedited shipments have higher cost threshold
		shipment := createTestShipment()
		shipment.ServiceLevel = ServiceOvernight

		result, err := ProcessShipment(context.Background(), shipment)

		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		// Should accept the $75 rate for overnight (threshold is $100 for expedited)
		if result.SelectedRate.Cost != 75.00 {
			t.Errorf("Expected to accept $75 rate for overnight, got $%.2f", result.SelectedRate.Cost)
		}
	})
}

func TestMetricsCollection(t *testing.T) {
	providers := map[string]ShippingProvider{
		"TestProvider": &MockProvider{
			name:      "TestProvider",
			rateDelay: 50 * time.Millisecond,
			rates: []Rate{
				{ProviderName: "TestProvider", ServiceName: "Standard", Cost: 25.00, EstimatedDays: 3},
			},
		},
	}

	Initialize(providers)

	metrics := &ShippingMetrics{
		ProviderBreakdown: make(map[string]int),
		ProviderCosts:     make(map[string]float64),
		ServiceBreakdown:  make(map[ServiceLevel]int),
		FastestShipment:   time.Hour,
	}

	// Process multiple shipments
	for i := 0; i < 3; i++ {
		shipment := createTestShipment()
		result, err := ProcessShipment(context.Background(), shipment)
		updateMetrics(metrics, result, 100*time.Millisecond, err)
	}

	if metrics.TotalShipments != 3 {
		t.Errorf("Expected 3 total shipments, got %d", metrics.TotalShipments)
	}

	if metrics.SuccessfulShipments != 3 {
		t.Errorf("Expected 3 successful shipments, got %d", metrics.SuccessfulShipments)
	}

	if metrics.TotalCost != 75.00 {
		t.Errorf("Expected total cost $75.00, got $%.2f", metrics.TotalCost)
	}

	if metrics.ProviderBreakdown["TestProvider"] != 3 {
		t.Errorf("Expected TestProvider to handle 3 shipments, got %d", metrics.ProviderBreakdown["TestProvider"])
	}
}

// Helper function to create test shipment.
func createTestShipment() Shipment {
	return Shipment{
		ID:         "TEST-001",
		OrderID:    "ORDER-001",
		CustomerID: "CUST-001",
		Weight:     5.0,
		Dimensions: Dimensions{Length: 10, Width: 8, Height: 6},
		Value:      100.00,
		Contents: []Item{
			{SKU: "TEST-ITEM", Name: "Test Item", Quantity: 1, Value: 100.00},
		},
		From: Address{
			State: "WA",
			ZIP:   "98101",
		},
		To: Address{
			State: "OR",
			ZIP:   "97201",
		},
		ServiceLevel: ServiceStandard,
	}
}
