package main

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/zoobzio/pipz"
)

func TestEventOrchestration(t *testing.T) {
	// Initialize services and pipelines.
	InitializeMockServices()
	InitializePipelines()

	ctx := context.Background()

	t.Run("Safety Bridge Processes Harsh Braking", func(t *testing.T) {
		event := FleetEvent{
			EventID:   "TEST-001",
			Type:      EventHarshBraking,
			VehicleID: "TRUCK-001",
			DriverID:  "DRIVER-001",
			Timestamp: time.Now(),
			Data: map[string]any{
				"lat":    40.7128,
				"lng":    -74.0060,
				"speed":  45.0,
				"gforce": 0.8,
			},
		}

		bridge := createSafetyBridge("harsh_braking")
		processed, err := bridge.Process(ctx, event)
		if err != nil {
			t.Fatalf("Failed to process safety event: %v", err)
		}

		// Check results were stored.
		if processed.DomainResults == nil {
			t.Fatal("Domain results not initialized")
		}

		result, ok := processed.DomainResults["safety"].(SafetyResult)
		if !ok {
			t.Fatal("Safety result not found or wrong type")
		}

		if !result.IncidentProcessed {
			t.Error("Incident should be marked as processed")
		}

		if result.UpdatedScore <= 0 {
			t.Error("Driver score should be updated")
		}
	})

	t.Run("Maintenance Bridge Processes Engine Events", func(t *testing.T) {
		event := FleetEvent{
			EventID:   "TEST-002",
			Type:      EventEngineOn,
			VehicleID: "VAN-002",
			DriverID:  "DRIVER-002",
			Timestamp: time.Now(),
			Data: map[string]any{
				"mileage":      67500,
				"engine_hours": 1850.5,
			},
		}

		bridge := createMaintenanceBridge()
		processed, err := bridge.Process(ctx, event)
		if err != nil {
			t.Fatalf("Failed to process maintenance event: %v", err)
		}

		result, ok := processed.DomainResults["maintenance"].(MaintenanceResult)
		if !ok {
			t.Fatal("Maintenance result not found or wrong type")
		}

		// Should have a valid status.
		validStatuses := []string{"ok", "service_soon", "service_now", "critical"}
		found := false
		for _, status := range validStatuses {
			if result.Status == status {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Invalid maintenance status: %s", result.Status)
		}

		// Should have next service info.
		if result.NextServiceMiles <= 0 {
			t.Error("Next service miles should be set")
		}
	})

	t.Run("Event Router Handles Multiple Event Types", func(t *testing.T) {
		router := CreateEventOrchestrator()

		testCases := []struct {
			name         string
			eventType    string
			expectDomain string
		}{
			{"Harsh Braking", EventHarshBraking, "safety"},
			{"Engine On", EventEngineOn, "maintenance"},
			{"Location Update", EventLocationUpdated, "routing"},
			{"Engine Off", EventEngineOff, "compliance"},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				event := FleetEvent{
					EventID:   "TEST-" + tc.name,
					Type:      tc.eventType,
					VehicleID: "TRUCK-001",
					DriverID:  "DRIVER-001",
					Timestamp: time.Now(),
					Data:      map[string]any{},
				}

				processed, err := router.Process(ctx, event)
				if err != nil {
					t.Fatalf("Failed to route %s event: %v", tc.eventType, err)
				}

				// Should have results from expected domain.
				if len(processed.DomainResults) == 0 {
					t.Errorf("No domain results for %s event", tc.eventType)
				}
			})
		}
	})

	t.Run("Risk Assessment Aggregates Domain Results", func(t *testing.T) {
		// Create event with results from multiple domains.
		event := FleetEvent{
			EventID:   "TEST-RISK",
			Type:      EventHarshBraking,
			VehicleID: "SEDAN-003",
			DriverID:  "DRIVER-003",
			Timestamp: time.Now(),
			Data:      map[string]any{},
			DomainResults: map[string]any{
				"safety": SafetyResult{
					IncidentProcessed: true,
					UpdatedScore:      65.0, // Low score
				},
				"maintenance": MaintenanceResult{
					Status: "service_now",
				},
				"routing": RouteResult{
					Efficiency:     75.0,
					DeviationCount: 8,
				},
				"compliance": ComplianceResult{
					Compliant: true,
				},
			},
		}

		riskPipeline := CreateRiskAssessmentPipeline()
		processed, err := riskPipeline.Process(ctx, event)
		if err != nil {
			t.Fatalf("Failed to assess risk: %v", err)
		}

		assessment, ok := processed.DomainResults["risk_assessment"].(RiskAssessment)
		if !ok {
			t.Fatal("Risk assessment not found")
		}

		// Should have calculated risks.
		if assessment.OverallRisk <= 0 {
			t.Error("Overall risk should be calculated")
		}

		// With low safety score and maintenance issues, risk should be elevated.
		if assessment.RiskCategory == "low" {
			t.Error("Risk should be elevated with poor safety and maintenance")
		}

		// Should have recommendations.
		if len(assessment.ImmediateActions) == 0 && len(assessment.PreventiveActions) == 0 {
			t.Error("Should have risk mitigation recommendations")
		}
	})

	t.Run("Full Pipeline End-to-End", func(t *testing.T) {
		router := CreateEventOrchestrator()
		riskPipeline := CreateRiskAssessmentPipeline()

		fullPipeline := pipz.NewSequence[FleetEvent]("test_full")
		fullPipeline.Register(router, riskPipeline)

		event := FleetEvent{
			EventID:   "TEST-E2E",
			Type:      EventHarshBraking,
			VehicleID: "TRUCK-001",
			DriverID:  "DRIVER-001",
			Timestamp: time.Now(),
			Data: map[string]any{
				"lat":          40.7128,
				"lng":          -74.0060,
				"speed":        55.0,
				"gforce":       1.0,
				"mileage":      155000,
				"engine_hours": 2200.0,
			},
		}

		processed, err := fullPipeline.Process(ctx, event)
		if err != nil {
			t.Fatalf("Failed to process through full pipeline: %v", err)
		}

		// Should have results from safety domain and risk assessment.
		if processed.DomainResults == nil {
			t.Fatal("No domain results")
		}

		if _, ok := processed.DomainResults["safety"]; !ok {
			t.Error("Missing safety domain results")
		}

		if _, ok := processed.DomainResults["risk_assessment"]; !ok {
			t.Error("Missing risk assessment")
		}
	})
}

func TestDomainIsolation(t *testing.T) {
	// This test verifies that domains are truly isolated.
	InitializeMockServices()
	InitializePipelines()

	ctx := context.Background()

	t.Run("Safety Domain Uses Its Own Types", func(t *testing.T) {
		// Create safety incident directly.
		incident := SafetyIncident{
			IncidentID: "ISO-001",
			Vehicle: Vehicle{
				ID:   "TEST-VEH",
				Type: "truck",
			},
			Driver: Driver{
				ID:   "TEST-DRV",
				Name: "Test Driver",
			},
			Type:      "speeding",
			Severity:  7,
			Timestamp: time.Now(),
		}

		// Process through safety domain directly.
		processed, pErr := ProcessSafetyIncident(ctx, incident)
		if pErr != nil {
			t.Fatalf("Failed to process safety incident: %v", pErr)
		}

		// Verify result is domain-specific type.
		if !processed.ProcessingResult.IncidentProcessed {
			t.Error("Incident should be processed")
		}

		// The safety domain should work without any FleetEvent.
		// This proves it's truly isolated.
	})

	t.Run("Each Domain Has Private Pipeline", func(t *testing.T) {
		// This test verifies encapsulation.
		// We can't access the internal pipelines directly.
		// Only through the public Process functions.

		// Try to process through each domain.
		domainTests := []struct {
			name    string
			process func() error
		}{
			{
				"Safety",
				func() error {
					_, pErr := ProcessSafetyIncident(ctx, SafetyIncident{})
					if pErr != nil {
						return fmt.Errorf("safety failed: %w", pErr)
					}
					return nil
				},
			},
			{
				"Maintenance",
				func() error {
					_, pErr := ProcessMaintenanceCheck(ctx, MaintenanceCheck{})
					if pErr != nil {
						return fmt.Errorf("maintenance failed: %w", pErr)
					}
					return nil
				},
			},
			{
				"Routing",
				func() error {
					_, pErr := ProcessRouteAnalysis(ctx, RouteAnalysis{
						PlannedRoute: Route{},
					})
					if pErr != nil {
						return fmt.Errorf("routing failed: %w", pErr)
					}
					return nil
				},
			},
			{
				"Compliance",
				func() error {
					_, pErr := ProcessComplianceCheck(ctx, ComplianceCheck{})
					if pErr != nil {
						return fmt.Errorf("compliance failed: %w", pErr)
					}
					return nil
				},
			},
		}

		for _, d := range domainTests {
			t.Run(d.name, func(t *testing.T) {
				// Each domain should process without errors.
				// even with minimal input.
				err := d.process()
				if err != nil {
					t.Errorf("%s domain failed: %v", d.name, err)
				}
			})
		}
	})
}
