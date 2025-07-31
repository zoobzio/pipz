package main

import (
	"time"
)

// Core Event Types - The central nervous system.

// FleetEvent is the central event type that flows through the main orchestration pipeline.
type FleetEvent struct { //nolint:govet // Demo struct
	EventID   string
	Type      string // "location.updated", "harsh.braking", etc.
	VehicleID string
	DriverID  string
	Timestamp time.Time
	Data      map[string]any // Event-specific data

	// Results from domain processing (populated by bridges).
	DomainResults map[string]any
}

// Common event types.
const (
	EventLocationUpdated = "location.updated"
	EventHarshBraking    = "harsh.braking"
	EventHarshAccel      = "harsh.acceleration"
	EventSpeeding        = "speed.violation"
	EventEngineOn        = "engine.on"
	EventEngineOff       = "engine.off"
	EventMaintenanceDue  = "maintenance.due"
	EventFuelLow         = "fuel.low"
	EventRouteDeviation  = "route.deviation"
	EventPanicButton     = "panic.button"
)

// Shared Core Types - Used across domains.

type Vehicle struct { //nolint:govet // Demo struct
	ID           string
	Type         string // "truck", "van", "sedan"
	Make         string
	Model        string
	Year         int
	LicensePlate string
	VIN          string
}

type Driver struct {
	ID            string
	Name          string
	LicenseNumber string
	LicenseExpiry time.Time
	EmployeeID    string
}

type GPSCoordinate struct { //nolint:govet // Demo struct
	Latitude  float64
	Longitude float64
	Timestamp time.Time
	Speed     float64
	Heading   float64
}

// SafetyProfile tracks driver safety over time (not used in pipelines).
type SafetyProfile struct { //nolint:govet // Demo struct
	Driver      Driver
	SafetyScore float64 // 0-100
	Incidents   []SafetyIncident
	LastUpdated time.Time

	// Metrics.
	TotalMiles     int
	HarshBraking   int
	HarshAccel     int
	SpeedingEvents int

	// Training status.
	RequiredTraining  []string
	CompletedTraining map[string]time.Time
}

// Risk Assessment Types (Cross-Domain).

// RiskAssessment combines results from all domains.
type RiskAssessment struct { //nolint:govet // Demo struct
	Vehicle   Vehicle
	Driver    Driver
	Timestamp time.Time

	// Domain scores.
	SafetyScore     float64
	MaintenanceRisk float64
	RouteRisk       float64
	ComplianceRisk  float64

	// Combined risk.
	OverallRisk  float64 // 0-100
	RiskCategory string  // "low", "medium", "high", "critical"

	// Recommendations.
	ImmediateActions  []string
	PreventiveActions []string

	// Predictive analytics.
	AccidentProbability  float64
	BreakdownProbability float64
}
