package main

import (
	"fmt"
	"time"
)

// Shipment represents a package to be shipped.
//
//nolint:govet // Field alignment is optimized for readability
type Shipment struct {
	// Basic shipment info.
	ID         string
	OrderID    string
	CustomerID string

	// Package details.
	Weight     float64    // pounds
	Dimensions Dimensions // inches
	Value      float64    // USD
	Contents   []Item

	// Addresses.
	From Address
	To   Address

	// Shipping preferences.
	ServiceLevel ServiceLevel
	RequiredBy   *time.Time // delivery deadline

	// State that evolves through pipeline.
	Status         ShipmentStatus
	SelectedRate   *Rate
	Label          *Label
	TrackingNumber string

	// Processing metadata.
	CreatedAt     time.Time
	ProcessingLog []string
	Timestamps    map[string]time.Time

	// Provider selection metadata.
	AttemptedProviders []string
	FailedProviders    map[string]string // provider -> error message
}

// Clone implements Cloner for concurrent processing.
func (s Shipment) Clone() Shipment {
	// Deep copy contents.
	contents := make([]Item, len(s.Contents))
	copy(contents, s.Contents)

	// Deep copy processing log.
	log := make([]string, len(s.ProcessingLog))
	copy(log, s.ProcessingLog)

	// Deep copy timestamps.
	timestamps := make(map[string]time.Time, len(s.Timestamps))
	for k, v := range s.Timestamps {
		timestamps[k] = v
	}

	// Deep copy attempted providers.
	attempted := make([]string, len(s.AttemptedProviders))
	copy(attempted, s.AttemptedProviders)

	// Deep copy failed providers.
	failed := make(map[string]string, len(s.FailedProviders))
	for k, v := range s.FailedProviders {
		failed[k] = v
	}

	// Deep copy selected rate and label if they exist.
	var selectedRate *Rate
	if s.SelectedRate != nil {
		rateCopy := *s.SelectedRate
		selectedRate = &rateCopy
	}

	var label *Label
	if s.Label != nil {
		labelCopy := *s.Label
		label = &labelCopy
	}

	return Shipment{
		ID:                 s.ID,
		OrderID:            s.OrderID,
		CustomerID:         s.CustomerID,
		Weight:             s.Weight,
		Dimensions:         s.Dimensions,
		Value:              s.Value,
		Contents:           contents,
		From:               s.From,
		To:                 s.To,
		ServiceLevel:       s.ServiceLevel,
		RequiredBy:         s.RequiredBy,
		Status:             s.Status,
		SelectedRate:       selectedRate,
		Label:              label,
		TrackingNumber:     s.TrackingNumber,
		CreatedAt:          s.CreatedAt,
		ProcessingLog:      log,
		Timestamps:         timestamps,
		AttemptedProviders: attempted,
		FailedProviders:    failed,
	}
}

// Dimensions represents package dimensions.
type Dimensions struct {
	Length float64 // inches
	Width  float64 // inches
	Height float64 // inches
}

// Item represents an item in the shipment.
type Item struct {
	SKU      string
	Name     string
	Quantity int
	Value    float64 // USD per item
	Weight   float64 // pounds per item
	HazMat   bool    // hazardous materials
	Fragile  bool
}

const (
	// Common string constants.
	unknownStatus = "unknown"
)

// Address represents shipping addresses.
type Address struct {
	Name    string
	Company string
	Street1 string
	Street2 string
	City    string
	State   string
	ZIP     string
	Country string
	Phone   string
	Email   string
}

// ServiceLevel represents shipping speed/service.
type ServiceLevel int

const (
	ServiceStandard ServiceLevel = iota
	ServiceExpedited
	ServiceOvernight
	ServiceSameDay
	ServiceInternational
)

func (sl ServiceLevel) String() string {
	switch sl {
	case ServiceStandard:
		return "standard"
	case ServiceExpedited:
		return "expedited"
	case ServiceOvernight:
		return "overnight"
	case ServiceSameDay:
		return "same_day"
	case ServiceInternational:
		return "international"
	default:
		return unknownStatus
	}
}

// ShipmentStatus represents the current state of a shipment.
type ShipmentStatus int

const (
	StatusPending ShipmentStatus = iota
	StatusRated
	StatusLabeled
	StatusShipped
	StatusInTransit
	StatusDelivered
	StatusException
	StatusCancelled
)

func (s ShipmentStatus) String() string {
	switch s {
	case StatusPending:
		return "pending"
	case StatusRated:
		return "rated"
	case StatusLabeled:
		return "labeled"
	case StatusShipped:
		return "shipped"
	case StatusInTransit:
		return "in_transit"
	case StatusDelivered:
		return "delivered"
	case StatusException:
		return "exception"
	case StatusCancelled:
		return "canceled"
	default:
		return unknownStatus
	}
}

// Rate represents a shipping rate quote.
//
//nolint:govet // Field alignment is optimized for readability
type Rate struct {
	ProviderName  string
	ServiceName   string
	Cost          float64 // USD
	EstimatedDays int
	DeliveryDate  *time.Time
	Currency      string

	// Additional details.
	Fuel        float64 // fuel surcharge
	Residential float64 // residential delivery fee
	Insurance   float64 // insurance cost
	Signature   float64 // signature required fee
}

// Label represents a shipping label.
//
//nolint:govet // Field alignment is optimized for readability
type Label struct {
	ProviderName   string
	TrackingNumber string
	LabelData      []byte  // PDF or image data
	LabelFormat    string  // "PDF", "PNG", etc.
	Cost           float64 // final cost charged

	// Metadata.
	CreatedAt time.Time
	ExpiresAt *time.Time
}

// TrackingInfo represents package tracking status.
//
//nolint:govet // Field alignment is optimized for readability
type TrackingInfo struct {
	TrackingNumber    string
	Status            TrackingStatus
	Location          string
	UpdatedAt         time.Time
	EstimatedDelivery *time.Time
	Events            []TrackingEvent
}

// TrackingStatus represents current tracking state.
type TrackingStatus int

const (
	TrackingPending TrackingStatus = iota
	TrackingPickedUp
	TrackingInTransit
	TrackingOutForDelivery
	TrackingDelivered
	TrackingException
	TrackingReturned
)

func (ts TrackingStatus) String() string {
	switch ts {
	case TrackingPending:
		return "pending"
	case TrackingPickedUp:
		return "picked_up"
	case TrackingInTransit:
		return "in_transit"
	case TrackingOutForDelivery:
		return "out_for_delivery"
	case TrackingDelivered:
		return "delivered"
	case TrackingException:
		return "exception"
	case TrackingReturned:
		return "returned"
	default:
		return unknownStatus
	}
}

// TrackingEvent represents a tracking event.
//
//nolint:govet // Field alignment is optimized for readability
type TrackingEvent struct {
	Timestamp   time.Time
	Status      TrackingStatus
	Location    string
	Description string
}

// ShippingMetrics tracks shipping performance.
//
//nolint:govet // Field alignment is optimized for readability
type ShippingMetrics struct {
	TotalShipments      int
	SuccessfulShipments int
	FailedShipments     int
	TotalCost           float64
	AverageCost         float64
	AverageProcessTime  time.Duration

	// Provider breakdown.
	ProviderBreakdown map[string]int
	ProviderCosts     map[string]float64

	// Performance metrics.
	FastestShipment time.Duration
	SlowestShipment time.Duration

	// Service level breakdown.
	ServiceBreakdown map[ServiceLevel]int

	// Success rates.
	RateSuccessRate  float64
	LabelSuccessRate float64
}

// GenerateReport creates a formatted metrics report.
func (m ShippingMetrics) GenerateReport() string {
	report := "\n=== Shipping Fulfillment Metrics ===\n"
	report += fmt.Sprintf("Total Shipments: %d\n", m.TotalShipments)
	report += fmt.Sprintf("Success Rate: %.1f%% (%d succeeded)\n",
		float64(m.SuccessfulShipments)/float64(m.TotalShipments)*100, m.SuccessfulShipments)
	report += fmt.Sprintf("Total Shipping Cost: $%.2f\n", m.TotalCost)
	report += fmt.Sprintf("Average Cost per Shipment: $%.2f\n", m.AverageCost)
	report += fmt.Sprintf("Average Process Time: %v\n", m.AverageProcessTime)

	report += "\nProvider Usage:\n"
	for provider, count := range m.ProviderBreakdown {
		percentage := float64(count) / float64(m.TotalShipments) * 100
		avgCost := m.ProviderCosts[provider] / float64(count)
		report += fmt.Sprintf("  %s: %d shipments (%.1f%%), avg cost $%.2f\n",
			provider, count, percentage, avgCost)
	}

	report += "\nPerformance:\n"
	report += fmt.Sprintf("  Fastest: %v\n", m.FastestShipment)
	report += fmt.Sprintf("  Slowest: %v\n", m.SlowestShipment)

	report += "\nService Health:\n"
	report += fmt.Sprintf("  Rate Quote Success: %.1f%%\n", m.RateSuccessRate*100)
	report += fmt.Sprintf("  Label Generation Success: %.1f%%\n", m.LabelSuccessRate*100)

	return report
}

// Processor and Pipeline constants.
const (
	// Core shipping processors.
	ProcessorValidateShipment  = "validate_shipment"
	ProcessorCalculateRates    = "calculate_rates"
	ProcessorSelectOptimalRate = "select_optimal_rate"
	ProcessorCreateLabel       = "create_label"
	ProcessorDispatchShipment  = "dispatch_shipment"

	// Provider-specific processors.
	ProcessorFedExRates        = "fedex_rates"
	ProcessorUPSRates          = "ups_rates"
	ProcessorDHLRates          = "dhl_rates"
	ProcessorLocalCourierRates = "local_courier_rates"

	ProcessorFedExLabel        = "fedex_label"
	ProcessorUPSLabel          = "ups_label"
	ProcessorDHLLabel          = "dhl_label"
	ProcessorLocalCourierLabel = "local_courier_label"

	// Route determination.
	ProcessorDetermineRoute      = "determine_route"
	ProcessorDomesticRouter      = "domestic_router"
	ProcessorInternationalRouter = "international_router"
	ProcessorSameDayRouter       = "same_day_router"

	// Error handling.
	ProcessorProviderFallback = "provider_fallback"
	ProcessorRetryHandler     = "retry_handler"
	ProcessorShipmentFailure  = "shipment_failure"

	// Pipeline names.
	PipelineShippingFulfillment   = "shipping_fulfillment"
	PipelineRateShopping          = "rate_shopping"
	PipelineDomesticShipping      = "domestic_shipping"
	PipelineInternationalShipping = "international_shipping"
	PipelineSameDayShipping       = "same_day_shipping"
	PipelineExpressShipping       = "express_shipping"

	// Connector names.
	ConnectorProviderRace     = "provider_race"
	ConnectorShippingRouter   = "shipping_router"
	ConnectorProviderFallback = "provider_fallback"
	ConnectorShipmentTimeout  = "shipment_timeout"
	ConnectorProviderRetry    = "provider_retry"
)
