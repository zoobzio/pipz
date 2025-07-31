package main

import (
	"context"
	"fmt"
	"time"

	"github.com/zoobzio/pipz"
)

// InitializePipelines initializes all domain pipelines.
func InitializePipelines() {
	// Each domain initializes its own internal pipeline.
	InitializeSafety()
	InitializeMaintenance()
	InitializeRouting()
	InitializeCompliance()
}

// CreateEventOrchestrator creates the main event routing pipeline.
func CreateEventOrchestrator() *pipz.Switch[FleetEvent, string] {
	// Create the event router using Switch with a condition that returns event type.
	condition := func(_ context.Context, event FleetEvent) string {
		return event.Type
	}
	router := pipz.NewSwitch[FleetEvent, string](ProcessorEventRouter, condition)

	// Safety events.
	router.AddRoute(EventHarshBraking, createSafetyBridge("harsh_braking"))
	router.AddRoute(EventHarshAccel, createSafetyBridge("harsh_acceleration"))
	router.AddRoute(EventSpeeding, createSafetyBridge("speeding"))
	router.AddRoute(EventPanicButton, createSafetyBridge("panic"))

	// Maintenance events.
	router.AddRoute(EventEngineOn, createMaintenanceBridge())
	router.AddRoute(EventEngineOff, createMaintenanceBridge())
	router.AddRoute(EventMaintenanceDue, createMaintenanceBridge())

	// Routing events.
	router.AddRoute(EventLocationUpdated, createRoutingBridge())
	router.AddRoute(EventRouteDeviation, createRoutingBridge())

	// Compliance events (any event could trigger compliance check).
	router.AddRoute(EventEngineOff, createComplianceBridge()) // Check at end of shift

	return router
}

// Bridge functions translate between FleetEvent and domain types.

func createSafetyBridge(incidentType string) pipz.Chainable[FleetEvent] {
	return pipz.Apply(ProcessorSafetyBridge, func(ctx context.Context, event FleetEvent) (FleetEvent, error) {
		fmt.Printf("\n🔒 Safety Domain Processing: %s\n", incidentType)

		// Get vehicle and driver info.
		vehicle := vehicleRepo.Get(event.VehicleID)
		driver := driverRepo.GetByVehicle(event.VehicleID)

		// Build safety incident from event data.
		incident := SafetyIncident{
			IncidentID: event.EventID,
			Vehicle:    vehicle,
			Driver:     driver,
			Type:       incidentType,
			Timestamp:  event.Timestamp,
			VideoURL:   videoService.GetVideoURL(event.VehicleID, event.Timestamp),
		}

		// Extract event-specific data.
		if lat, ok := event.Data["lat"].(float64); ok {
			incident.Location.Latitude = lat
		}
		if lng, ok := event.Data["lng"].(float64); ok {
			incident.Location.Longitude = lng
		}
		if speed, ok := event.Data["speed"].(float64); ok {
			incident.Location.Speed = speed
		}
		if gforce, ok := event.Data["gforce"].(float64); ok {
			incident.GForce = gforce
		}

		// Get context data.
		weather := weatherService.GetConditions(incident.Location.Latitude, incident.Location.Longitude)
		incident.WeatherConditions = weather.Type

		traffic := trafficService.GetTraffic(incident.Location.Latitude, incident.Location.Longitude)
		incident.TrafficLevel = traffic.Level

		// Set speed limit based on location (mock).
		incident.SpeedLimit = 65 // Would use real map data

		// Process through safety pipeline.
		processed, pErr := ProcessSafetyIncident(ctx, incident)
		if pErr != nil {
			return event, fmt.Errorf("safety processing failed: %w", pErr)
		}

		// Store results back in event.
		if event.DomainResults == nil {
			event.DomainResults = make(map[string]any)
		}
		event.DomainResults["safety"] = processed.ProcessingResult

		return event, nil
	})
}

func createMaintenanceBridge() pipz.Chainable[FleetEvent] {
	return pipz.Apply(ProcessorMaintenanceBridge, func(ctx context.Context, event FleetEvent) (FleetEvent, error) {
		fmt.Printf("\n🔧 Maintenance Domain Processing\n")

		// Get vehicle info.
		vehicle := vehicleRepo.Get(event.VehicleID)

		// Build maintenance check.
		check := MaintenanceCheck{
			Vehicle:        vehicle,
			ServiceHistory: maintenanceRepo.GetHistory(event.VehicleID),
		}

		// Extract current data from event.
		if mileage, ok := event.Data["mileage"].(int); ok {
			check.CurrentMileage = mileage
		}
		if hours, ok := event.Data["engine_hours"].(float64); ok {
			check.EngineHours = hours
		}

		// Mock diagnostic data.
		check.DiagnosticCodes = []string{} // Would come from OBD
		check.TirePressure = map[string]float64{
			"FL": 35.2, "FR": 34.8, "RL": 35.0, "RR": 34.5,
		}
		check.OilLevel = 0.75
		check.BrakeWear = map[string]float64{
			"FL": 0.4, "FR": 0.45, "RL": 0.3, "RR": 0.35,
		}
		check.PartsAvailable = map[string]int{
			"oil_filter": 5, "air_filter": 3, "brake_pads": 2,
		}

		// Process through maintenance pipeline.
		processed, pErr := ProcessMaintenanceCheck(ctx, check)
		if pErr != nil {
			return event, fmt.Errorf("maintenance processing failed: %w", pErr)
		}

		// Store results.
		if event.DomainResults == nil {
			event.DomainResults = make(map[string]any)
		}
		event.DomainResults["maintenance"] = processed.ProcessingResult

		return event, nil
	})
}

func createRoutingBridge() pipz.Chainable[FleetEvent] {
	return pipz.Apply(ProcessorRoutingBridge, func(ctx context.Context, event FleetEvent) (FleetEvent, error) {
		fmt.Printf("\n📍 Routing Domain Processing\n")

		// Get vehicle and driver.
		vehicle := vehicleRepo.Get(event.VehicleID)
		driver := driverRepo.GetByVehicle(event.VehicleID)

		// Build route analysis.
		analysis := RouteAnalysis{
			Vehicle:      vehicle,
			Driver:       driver,
			PlannedRoute: routeRepo.GetActiveRoute(event.VehicleID),
			StartTime:    time.Now().Add(-4 * time.Hour), // Mock
		}

		// Add current location to actual path.
		if lat, ok := event.Data["lat"].(float64); ok {
			coord := GPSCoordinate{
				Latitude: lat,
				Longitude: func() float64 {
					if lng, ok := event.Data["lng"].(float64); ok {
						return lng
					}
					return 0.0
				}(),
				Timestamp: event.Timestamp,
			}
			if speed, ok := event.Data["speed"].(float64); ok {
				coord.Speed = speed
			}
			analysis.ActualPath = append(analysis.ActualPath, coord)
		}

		// Mock delivery data.
		analysis.PlannedStops = []DeliveryStop{
			{ID: "STOP-1", ScheduledTime: time.Now().Add(-2 * time.Hour)},
			{ID: "STOP-2", ScheduledTime: time.Now().Add(-1 * time.Hour)},
		}
		analysis.CompletedStops = []DeliveryStop{
			{ID: "STOP-1", Status: "completed", ActualTime: time.Now().Add(-90 * time.Minute)},
		}

		// Add conditions.
		if len(analysis.ActualPath) > 0 {
			weather := weatherService.GetConditions(analysis.ActualPath[0].Latitude, analysis.ActualPath[0].Longitude)
			analysis.WeatherData = weather

			traffic := trafficService.GetTraffic(analysis.ActualPath[0].Latitude, analysis.ActualPath[0].Longitude)
			analysis.TrafficData = traffic
		}

		// Mock metrics.
		analysis.PlannedDistance = 250.5
		analysis.ActualDistance = 265.2
		analysis.FuelUsed = 28.5

		// Process through routing pipeline.
		processed, pErr := ProcessRouteAnalysis(ctx, analysis)
		if pErr != nil {
			return event, fmt.Errorf("routing processing failed: %w", pErr)
		}

		// Store results.
		if event.DomainResults == nil {
			event.DomainResults = make(map[string]any)
		}
		event.DomainResults["routing"] = processed.ProcessingResult

		return event, nil
	})
}

func createComplianceBridge() pipz.Chainable[FleetEvent] {
	return pipz.Apply(ProcessorComplianceBridge, func(ctx context.Context, event FleetEvent) (FleetEvent, error) {
		fmt.Printf("\n⚖️ Compliance Domain Processing\n")

		// Get driver and vehicle.
		driver := driverRepo.Get(event.DriverID)
		vehicle := vehicleRepo.Get(event.VehicleID)

		// Build compliance check.
		check := ComplianceCheck{
			Driver:  driver,
			Vehicle: vehicle,
			Date:    time.Now(),
		}

		// Mock HOS data.
		check.DrivingHours = 9.5
		check.OnDutyHours = 12.0
		check.RestPeriods = []RestPeriod{
			{
				Start: time.Now().Add(-5 * time.Hour),
				End:   time.Now().Add(-270 * time.Minute),
				Type:  "short_break",
			},
		}

		// Mock 7-day history.
		check.Last7Days = []DailyLog{
			{Date: time.Now().Add(-1 * 24 * time.Hour), DrivingHours: 10.5, OnDutyHours: 13.0},
			{Date: time.Now().Add(-2 * 24 * time.Hour), DrivingHours: 9.0, OnDutyHours: 11.5},
		}

		// License info.
		check.DriverLicense = LicenseStatus{
			Number:       driver.LicenseNumber,
			Class:        "CDL-A",
			Expiry:       driver.LicenseExpiry,
			Endorsements: []string{"CDL-A", "HAZMAT"},
			Valid:        true,
		}

		// Mock permits.
		check.VehiclePermits = []Permit{
			{
				Type:   "Interstate",
				Number: "ICC-12345",
				Expiry: time.Now().Add(60 * 24 * time.Hour),
				States: []string{"CA", "NV", "AZ"},
			},
		}

		// Insurance.
		check.Insurance = InsurancePolicy{
			Provider:     "Fleet Insurance Co",
			PolicyNumber: "FL-2024-001",
			Coverage:     1000000.0,
			Expiry:       time.Now().Add(180 * 24 * time.Hour),
		}

		// Recent violations (mock).
		check.RecentViolations = []Violation{}

		// Process through compliance pipeline.
		processed, pErr := ProcessComplianceCheck(ctx, check)
		if pErr != nil {
			return event, fmt.Errorf("compliance processing failed: %w", pErr)
		}

		// Store results.
		if event.DomainResults == nil {
			event.DomainResults = make(map[string]any)
		}
		event.DomainResults["compliance"] = processed.ProcessingResult

		return event, nil
	})
}

// CreateRiskAssessmentPipeline creates a pipeline that reads results from all domains.
func CreateRiskAssessmentPipeline() *pipz.Sequence[FleetEvent] {
	pipeline := pipz.NewSequence[FleetEvent](PipelineEventOrchestration)

	// Assess overall risk based on all domain results.
	assessRisk := pipz.Apply(ProcessorRiskAssessment, func(_ context.Context, event FleetEvent) (FleetEvent, error) {
		if event.DomainResults == nil {
			return event, nil
		}

		fmt.Printf("\n🎯 Risk Assessment\n")

		// Initialize risk assessment.
		assessment := RiskAssessment{
			Vehicle:   vehicleRepo.Get(event.VehicleID),
			Driver:    driverRepo.GetByVehicle(event.VehicleID),
			Timestamp: time.Now(),
		}

		// Safety domain contribution.
		if safetyResult, ok := event.DomainResults["safety"].(SafetyResult); ok {
			assessment.SafetyScore = safetyResult.UpdatedScore
			if safetyResult.UpdatedScore < 70 {
				assessment.ImmediateActions = append(assessment.ImmediateActions,
					"Schedule safety training")
			}
		}

		// Maintenance domain contribution.
		if maintResult, ok := event.DomainResults["maintenance"].(MaintenanceResult); ok {
			// Convert status to risk score.
			switch maintResult.Status {
			case "ok":
				assessment.MaintenanceRisk = 10
			case "service_soon":
				assessment.MaintenanceRisk = 30
			case "service_now":
				assessment.MaintenanceRisk = 60
			case maintenanceStatusCritical:
				assessment.MaintenanceRisk = 90
			}

			if len(maintResult.UrgentRepairs) > 0 {
				assessment.ImmediateActions = append(assessment.ImmediateActions,
					fmt.Sprintf("Schedule urgent repairs: %v", maintResult.UrgentRepairs))
			}
		}

		// Routing domain contribution.
		if routeResult, ok := event.DomainResults["routing"].(RouteResult); ok {
			// Poor efficiency increases risk.
			assessment.RouteRisk = 100 - routeResult.Efficiency
			if routeResult.DeviationCount > 5 {
				assessment.PreventiveActions = append(assessment.PreventiveActions,
					"Review route planning process")
			}
		}

		// Compliance domain contribution.
		if compResult, ok := event.DomainResults["compliance"].(ComplianceResult); ok {
			switch {
			case !compResult.Compliant:
				assessment.ComplianceRisk = 80
				assessment.ImmediateActions = append(assessment.ImmediateActions,
					"Address compliance violations immediately")
			case len(compResult.ExpiringItems) > 0:
				assessment.ComplianceRisk = 30
				assessment.PreventiveActions = append(assessment.PreventiveActions,
					"Renew expiring documents")
			default:
				assessment.ComplianceRisk = 10
			}
		}

		// Calculate overall risk.
		assessment.OverallRisk = (100-assessment.SafetyScore)*0.3 +
			assessment.MaintenanceRisk*0.3 +
			assessment.RouteRisk*0.2 +
			assessment.ComplianceRisk*0.2

			// Categorize risk.
		switch {
		case assessment.OverallRisk >= 75:
			assessment.RiskCategory = "critical"
		case assessment.OverallRisk >= 50:
			assessment.RiskCategory = "high"
		case assessment.OverallRisk >= 25:
			assessment.RiskCategory = "medium"
		default:
			assessment.RiskCategory = "low"
		}

		// Predictive analytics (simplified).
		assessment.AccidentProbability = assessment.OverallRisk / 100 * 0.05     // 5% max
		assessment.BreakdownProbability = assessment.MaintenanceRisk / 100 * 0.1 // 10% max

		fmt.Printf("   📊 Overall Risk: %.1f%% (%s)\n", assessment.OverallRisk, assessment.RiskCategory)
		fmt.Printf("   🎲 Accident probability: %.2f%%\n", assessment.AccidentProbability*100)
		fmt.Printf("   🔧 Breakdown probability: %.2f%%\n", assessment.BreakdownProbability*100)

		if len(assessment.ImmediateActions) > 0 {
			fmt.Printf("   ⚡ Immediate actions required:\n")
			for _, action := range assessment.ImmediateActions {
				fmt.Printf("      - %s\n", action)
			}
		}

		// Store assessment.
		event.DomainResults["risk_assessment"] = assessment

		// Send alerts if critical.
		if assessment.RiskCategory == "critical" {
			_ = notificationService.SendAlert("Fleet Manager", //nolint:errcheck // Demo notification
				fmt.Sprintf("CRITICAL RISK: Vehicle %s - %.0f%% risk score",
					event.VehicleID, assessment.OverallRisk))
		}

		return event, nil
	})

	pipeline.Register(assessRisk)
	return pipeline
}

// Pipeline names.
const (
	PipelineEventOrchestration = "event_orchestration"
	ProcessorEventRouter       = "event_router"
	ProcessorSafetyBridge      = "safety_bridge"
	ProcessorMaintenanceBridge = "maintenance_bridge"
	ProcessorRoutingBridge     = "routing_bridge"
	ProcessorComplianceBridge  = "compliance_bridge"
	ProcessorRiskAssessment    = "risk_assessment"
)
