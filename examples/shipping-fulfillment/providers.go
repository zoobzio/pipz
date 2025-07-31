package main

import (
	"context"
	"fmt"
	"math/rand"
	"time"
)

// ShippingProvider is the simple interface that external teams implement.
type ShippingProvider interface {
	// GetRates returns available shipping rates for a shipment
	GetRates(ctx context.Context, shipment Shipment) ([]Rate, error)

	// CreateLabel creates a shipping label for the selected rate
	CreateLabel(ctx context.Context, shipment Shipment, rate Rate) (*Label, error)

	// GetTracking returns current tracking information
	GetTracking(ctx context.Context, trackingNumber string) (*TrackingInfo, error)
}

// Mock provider implementations for demonstration.

// FedExProvider simulates FedEx integration.
type FedExProvider struct {
	name      string
	apiDelay  time.Duration
	errorRate float64
}

func NewFedExProvider() *FedExProvider {
	return &FedExProvider{
		name:      "FedEx",
		apiDelay:  150 * time.Millisecond, // Moderate speed
		errorRate: 0.05,                   // 5% error rate
	}
}

//nolint:dupl // Similar structure is intentional for provider implementations
func (p *FedExProvider) GetRates(ctx context.Context, shipment Shipment) ([]Rate, error) {
	// Simulate API call
	select {
	case <-time.After(p.apiDelay):
		// Check for simulated errors
		if rand.Float64() < p.errorRate { //nolint:gosec // Using weak RNG is fine for simulation
			return nil, fmt.Errorf("FedEx API temporarily unavailable")
		}

		// Calculate rates based on shipment
		baseRate := shipment.Weight * 3.5

		rates := []Rate{
			{
				ProviderName:  p.name,
				ServiceName:   "FedEx Ground",
				Cost:          baseRate * 1.0,
				EstimatedDays: 5,
				Currency:      "USD",
			},
			{
				ProviderName:  p.name,
				ServiceName:   "FedEx Express Saver",
				Cost:          baseRate * 1.5,
				EstimatedDays: 3,
				Currency:      "USD",
			},
			{
				ProviderName:  p.name,
				ServiceName:   "FedEx 2Day",
				Cost:          baseRate * 2.0,
				EstimatedDays: 2,
				Currency:      "USD",
			},
			{
				ProviderName:  p.name,
				ServiceName:   "FedEx Priority Overnight",
				Cost:          baseRate * 3.0,
				EstimatedDays: 1,
				Currency:      "USD",
			},
		}

		return rates, nil

	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (p *FedExProvider) CreateLabel(ctx context.Context, _ Shipment, rate Rate) (*Label, error) {
	// Simulate label creation
	select {
	case <-time.After(200 * time.Millisecond):
		return &Label{
			ProviderName:   p.name,
			TrackingNumber: fmt.Sprintf("FDX%d", rand.Intn(1000000)), //nolint:gosec // Mock tracking number
			LabelData:      []byte("mock-fedex-label-pdf"),
			LabelFormat:    "PDF",
			Cost:           rate.Cost,
			CreatedAt:      time.Now(),
		}, nil

	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (*FedExProvider) GetTracking(ctx context.Context, trackingNumber string) (*TrackingInfo, error) {
	// Simulate tracking lookup
	select {
	case <-time.After(100 * time.Millisecond):
		return &TrackingInfo{
			TrackingNumber: trackingNumber,
			Status:         TrackingInTransit,
			Location:       "Memphis, TN",
			UpdatedAt:      time.Now(),
		}, nil

	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// UPSProvider simulates UPS integration.
type UPSProvider struct {
	name      string
	apiDelay  time.Duration
	errorRate float64
}

func NewUPSProvider() *UPSProvider {
	return &UPSProvider{
		name:      "UPS",
		apiDelay:  100 * time.Millisecond, // Faster API
		errorRate: 0.03,                   // 3% error rate
	}
}

//nolint:dupl // Similar structure is intentional for provider implementations
func (p *UPSProvider) GetRates(ctx context.Context, shipment Shipment) ([]Rate, error) {
	// Simulate API call
	select {
	case <-time.After(p.apiDelay):
		// Check for simulated errors
		if rand.Float64() < p.errorRate { //nolint:gosec // Using weak RNG is fine for simulation
			return nil, fmt.Errorf("UPS API error: rate limit exceeded")
		}

		// UPS tends to be slightly cheaper for ground
		baseRate := shipment.Weight * 3.2

		rates := []Rate{
			{
				ProviderName:  p.name,
				ServiceName:   "UPS Ground",
				Cost:          baseRate * 0.9,
				EstimatedDays: 5,
				Currency:      "USD",
			},
			{
				ProviderName:  p.name,
				ServiceName:   "UPS 3 Day Select",
				Cost:          baseRate * 1.4,
				EstimatedDays: 3,
				Currency:      "USD",
			},
			{
				ProviderName:  p.name,
				ServiceName:   "UPS 2nd Day Air",
				Cost:          baseRate * 1.9,
				EstimatedDays: 2,
				Currency:      "USD",
			},
			{
				ProviderName:  p.name,
				ServiceName:   "UPS Next Day Air",
				Cost:          baseRate * 2.8,
				EstimatedDays: 1,
				Currency:      "USD",
			},
		}

		return rates, nil

	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (p *UPSProvider) CreateLabel(ctx context.Context, _ Shipment, rate Rate) (*Label, error) {
	// Simulate label creation
	select {
	case <-time.After(180 * time.Millisecond):
		return &Label{
			ProviderName:   p.name,
			TrackingNumber: fmt.Sprintf("1Z%d", rand.Intn(1000000)), //nolint:gosec // Mock tracking number
			LabelData:      []byte("mock-ups-label-pdf"),
			LabelFormat:    "PDF",
			Cost:           rate.Cost,
			CreatedAt:      time.Now(),
		}, nil

	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (*UPSProvider) GetTracking(ctx context.Context, trackingNumber string) (*TrackingInfo, error) {
	// Simulate tracking lookup
	select {
	case <-time.After(80 * time.Millisecond):
		return &TrackingInfo{
			TrackingNumber: trackingNumber,
			Status:         TrackingPickedUp,
			Location:       "Louisville, KY",
			UpdatedAt:      time.Now(),
		}, nil

	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// USPSProvider simulates USPS integration.
type USPSProvider struct {
	name      string
	apiDelay  time.Duration
	errorRate float64
}

func NewUSPSProvider() *USPSProvider {
	return &USPSProvider{
		name:      "USPS",
		apiDelay:  250 * time.Millisecond, // Slower API
		errorRate: 0.02,                   // 2% error rate
	}
}

func (p *USPSProvider) GetRates(ctx context.Context, shipment Shipment) ([]Rate, error) {
	// Simulate API call
	select {
	case <-time.After(p.apiDelay):
		// Check for simulated errors
		if rand.Float64() < p.errorRate { //nolint:gosec // Using weak RNG is fine for simulation
			return nil, fmt.Errorf("USPS API timeout")
		}

		// USPS is often cheapest for light packages
		baseRate := shipment.Weight * 2.8
		if shipment.Weight < 5 {
			baseRate = shipment.Weight * 2.2
		}

		rates := []Rate{
			{
				ProviderName:  p.name,
				ServiceName:   "USPS Ground Advantage",
				Cost:          baseRate * 0.8,
				EstimatedDays: 5,
				Currency:      "USD",
			},
			{
				ProviderName:  p.name,
				ServiceName:   "USPS Priority Mail",
				Cost:          baseRate * 1.2,
				EstimatedDays: 3,
				Currency:      "USD",
			},
			{
				ProviderName:  p.name,
				ServiceName:   "USPS Priority Mail Express",
				Cost:          baseRate * 2.5,
				EstimatedDays: 1,
				Currency:      "USD",
			},
		}

		return rates, nil

	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (p *USPSProvider) CreateLabel(ctx context.Context, _ Shipment, rate Rate) (*Label, error) {
	// Simulate label creation
	select {
	case <-time.After(300 * time.Millisecond):
		return &Label{
			ProviderName:   p.name,
			TrackingNumber: fmt.Sprintf("94%d", rand.Intn(10000000)), //nolint:gosec // Mock tracking number
			LabelData:      []byte("mock-usps-label-pdf"),
			LabelFormat:    "PDF",
			Cost:           rate.Cost,
			CreatedAt:      time.Now(),
		}, nil

	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (*USPSProvider) GetTracking(ctx context.Context, trackingNumber string) (*TrackingInfo, error) {
	// Simulate tracking lookup
	select {
	case <-time.After(150 * time.Millisecond):
		return &TrackingInfo{
			TrackingNumber: trackingNumber,
			Status:         TrackingInTransit,
			Location:       "Regional Facility",
			UpdatedAt:      time.Now(),
		}, nil

	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// LocalCourierProvider simulates a local/regional courier.
type LocalCourierProvider struct {
	name      string
	apiDelay  time.Duration
	errorRate float64
}

func NewLocalCourierProvider() *LocalCourierProvider {
	return &LocalCourierProvider{
		name:      "QuickShip Local",
		apiDelay:  50 * time.Millisecond, // Very fast API
		errorRate: 0.01,                  // 1% error rate
	}
}

func (p *LocalCourierProvider) GetRates(ctx context.Context, shipment Shipment) ([]Rate, error) {
	// Only serves local areas
	if shipment.To.State != shipment.From.State {
		return nil, fmt.Errorf("service area limited to %s", shipment.From.State)
	}

	// Simulate API call
	select {
	case <-time.After(p.apiDelay):
		// Check for simulated errors
		if rand.Float64() < p.errorRate { //nolint:gosec // Using weak RNG is fine for simulation
			return nil, fmt.Errorf("local courier system maintenance")
		}

		// Very competitive for same-day local
		baseRate := shipment.Weight*1.5 + 10

		rates := []Rate{
			{
				ProviderName:  p.name,
				ServiceName:   "Same Day Delivery",
				Cost:          baseRate,
				EstimatedDays: 0,
				Currency:      "USD",
			},
			{
				ProviderName:  p.name,
				ServiceName:   "Next Day Local",
				Cost:          baseRate * 0.7,
				EstimatedDays: 1,
				Currency:      "USD",
			},
		}

		return rates, nil

	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (p *LocalCourierProvider) CreateLabel(ctx context.Context, _ Shipment, rate Rate) (*Label, error) {
	// Simulate label creation
	select {
	case <-time.After(75 * time.Millisecond):
		return &Label{
			ProviderName:   p.name,
			TrackingNumber: fmt.Sprintf("QS%d", rand.Intn(100000)), //nolint:gosec // Mock tracking number
			LabelData:      []byte("mock-local-label-pdf"),
			LabelFormat:    "PDF",
			Cost:           rate.Cost,
			CreatedAt:      time.Now(),
		}, nil

	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (*LocalCourierProvider) GetTracking(ctx context.Context, trackingNumber string) (*TrackingInfo, error) {
	// Simulate tracking lookup
	select {
	case <-time.After(40 * time.Millisecond):
		return &TrackingInfo{
			TrackingNumber: trackingNumber,
			Status:         TrackingOutForDelivery,
			Location:       "Local Hub",
			UpdatedAt:      time.Now(),
		}, nil

	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
