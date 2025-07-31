package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/zoobzio/pipz"
)

var (
	sprint = flag.Int("sprint", 0, "Run specific sprint (0 = all sprints)")
)

func main() {
	flag.Parse()

	// Initialize mock services.
	InitializeMockServices()

	// Initialize all domain pipelines.
	InitializePipelines()

	// Show the evolution story.
	if *sprint == 0 {
		// Run all sprints to show evolution.
		showEvolution()
	} else {
		// Run specific sprint.
		runSprint(*sprint)
	}
}

func showEvolution() {
	fmt.Println("🚚 Fleet Management Event Orchestration")
	fmt.Println("=====================================")
	fmt.Println()

	// Sprint 1: The beginning.
	fmt.Println("📅 Sprint 1: MVP - Just Track Vehicles")
	fmt.Println("---------------------------------------")
	fmt.Println("\"We need to know where our trucks are!\"")
	fmt.Println()
	runSprint1()
	fmt.Println("\nPress Enter to continue to Sprint 3...")
	_, _ = fmt.Scanln() //nolint:errcheck // Demo pause

	// Sprint 3: The crisis.
	fmt.Println("\n📅 Sprint 3: The 3 AM Wake-up Call")
	fmt.Println("-----------------------------------")
	fmt.Println("Crisis: \"Why didn't we know the driver was speeding for 2 hours?\"")
	fmt.Println("Support: \"The system knew! But the alert was buried in the maintenance check code...\"")
	fmt.Println()
	runSprint3()
	fmt.Println("\nPress Enter to continue to Sprint 5...")
	_, _ = fmt.Scanln() //nolint:errcheck // Demo pause

	// Sprint 5: Domain separation.
	fmt.Println("\n📅 Sprint 5: Domain Separation")
	fmt.Println("------------------------------")
	fmt.Println("Solution: Break into domain pipelines with their own types!")
	fmt.Println()
	runSprint5()
	fmt.Println("\nPress Enter to continue to Sprint 7...")
	_, _ = fmt.Scanln() //nolint:errcheck // Demo pause

	// Sprint 7: The cascade.
	fmt.Println("\n📅 Sprint 7: The Cascade Effect")
	fmt.Println("-------------------------------")
	fmt.Println("New requirement: \"A harsh braking event needs to check EVERYTHING\"")
	fmt.Println()
	runSprint7()
	fmt.Println("\nPress Enter to continue to Sprint 9...")
	_, _ = fmt.Scanln() //nolint:errcheck // Demo pause

	// Sprint 9: Cross-domain intelligence.
	fmt.Println("\n📅 Sprint 9: Cross-Domain Intelligence")
	fmt.Println("--------------------------------------")
	fmt.Println("Fleet Manager: \"Vehicle 42 has 3 safety incidents AND overdue maintenance!\"")
	fmt.Println()
	runSprint9()
	fmt.Println("\nPress Enter to continue to Sprint 11...")
	_, _ = fmt.Scanln() //nolint:errcheck // Demo pause

	// Sprint 11: Predictive.
	fmt.Println("\n📅 Sprint 11: Predictive Operations")
	fmt.Println("-----------------------------------")
	fmt.Println("CEO: \"Can we prevent problems instead of reacting?\"")
	fmt.Println()
	runSprint11()
}

func runSprint(n int) {
	switch n {
	case 1:
		runSprint1()
	case 3:
		runSprint3()
	case 5:
		runSprint5()
	case 7:
		runSprint7()
	case 9:
		runSprint9()
	case 11:
		runSprint11()
	default:
		fmt.Printf("Unknown sprint: %d\n", n)
	}
}

// Sprint 1: Simple, monolithic handler.
func runSprint1() {
	fmt.Println("// Original monolithic code:")
	fmt.Println("func handleLocationUpdate(event FleetEvent) {")
	fmt.Println("    // 500 lines of tangled logic...")
	fmt.Println("    updateDatabase(event.VehicleID, event.Data[\"lat\"], event.Data[\"lng\"])")
	fmt.Println("    checkSpeed(event.Data[\"speed\"])")
	fmt.Println("    calculateFuel(event.Data[\"mileage\"])")
	fmt.Println("    checkMaintenance(event.VehicleID)")
	fmt.Println("    // etc...")
	fmt.Println("}")
	fmt.Println()

	// Simulate the problem.
	event := FleetEvent{
		EventID:   "EVT-001", //nolint:govet // Demo event structure
		Type:      EventLocationUpdated,
		VehicleID: "TRUCK-001",
		DriverID:  "DRIVER-001", //nolint:govet // Demo event structure
		Timestamp: time.Now(),   //nolint:govet // Demo event structure
		Data: map[string]any{
			"lat":     40.7128,
			"lng":     -74.0060,
			"speed":   75.0, // Speeding!
			"mileage": 155000,
		},
	}

	fmt.Println("🚨 Event received:", event.Type)
	fmt.Println("   Vehicle:", event.VehicleID)
	fmt.Println("   Speed:", event.Data["speed"], "mph")
	fmt.Println("   ⚠️  ALERT BURIED IN CODE - NO ONE SEES IT!")
}

// Sprint 3: The crisis reveals the problem.
func runSprint3() {
	// Show multiple events happening.
	events := []FleetEvent{
		{
			EventID:   "EVT-100",
			Type:      EventSpeeding,
			VehicleID: "TRUCK-001",
			DriverID:  "DRIVER-001",
			Timestamp: time.Now().Add(-2 * time.Hour),
			Data: map[string]any{
				"lat":   40.7128,
				"lng":   -74.0060,
				"speed": 85.0,
				"limit": 65,
			},
		},
		{
			EventID:   "EVT-101",
			Type:      EventMaintenanceDue,
			VehicleID: "TRUCK-001",
			Timestamp: time.Now().Add(-24 * time.Hour),
			Data: map[string]any{
				"service": "oil_change",
				"overdue": true,
			},
		},
		{
			EventID:   "EVT-102",
			Type:      EventEngineOff,
			VehicleID: "TRUCK-001",
			DriverID:  "DRIVER-001",
			Timestamp: time.Now(),
			Data: map[string]any{
				"reason": "ENGINE SEIZED",
			},
		},
	}

	for _, event := range events {
		fmt.Printf("⚡ Event: %s at %s\n", event.Type, event.Timestamp.Format("15:04"))
		if event.Type == EventEngineOff {
			fmt.Printf("   💥 CRITICAL: %s!\n", event.Data["reason"])
		}
	}

	fmt.Println("\n😱 Post-mortem:")
	fmt.Println("   - Driver was speeding for 2 hours")
	fmt.Println("   - Oil change was 5000 miles overdue")
	fmt.Println("   - Both alerts were logged but not visible")
	fmt.Println("   - Result: $50,000 engine replacement")
}

// Sprint 5: Domain separation in action.
func runSprint5() {
	ctx := context.Background()

	// Create a harsh braking event.
	event := FleetEvent{
		EventID:   "EVT-200",
		Type:      EventHarshBraking,
		VehicleID: "TRUCK-001",
		DriverID:  "DRIVER-001",
		Timestamp: time.Now(),
		Data: map[string]any{
			"lat":    40.7128,
			"lng":    -74.0060,
			"speed":  45.0,
			"gforce": 0.8, // Hard stop!
		},
		DomainResults: make(map[string]any),
	}

	fmt.Println("🚨 Harsh braking event detected!")
	fmt.Println()

	// Show how it's processed by the safety domain.
	safetyBridge := createSafetyBridge("harsh_braking")
	processedEvent, err := safetyBridge.Process(ctx, event)
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}

	if result, ok := processedEvent.DomainResults["safety"].(SafetyResult); ok {
		fmt.Printf("\n✅ Safety domain processed independently:\n")
		fmt.Printf("   - Incident recorded\n")
		fmt.Printf("   - Driver score updated: %.1f\n", result.UpdatedScore)
		fmt.Printf("   - Video archived: %v\n", result.VideoArchived)
	}
}

// Sprint 7: Multiple domains process concurrently.
func runSprint7() {
	ctx := context.Background()

	// Create event router.
	router := CreateEventOrchestrator()

	// Process a harsh braking event through multiple domains.
	event := FleetEvent{
		EventID:   "EVT-300",
		Type:      EventHarshBraking,
		VehicleID: "VAN-002",
		DriverID:  "DRIVER-002",
		Timestamp: time.Now(),
		Data: map[string]any{
			"lat":     40.7580,
			"lng":     -73.9855,
			"speed":   35.0,
			"gforce":  0.9,
			"mileage": 67500,
		},
	}

	fmt.Println("🚨 Harsh braking event - checking all systems!")

	// Process through router (which calls multiple bridges).
	start := time.Now()
	processedEvent, err := router.Process(ctx, event)
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}

	duration := time.Since(start)

	fmt.Printf("\n⚡ Processed in %v with concurrent domain checks\n", duration)
	fmt.Printf("✅ Domains processed: %d\n", len(processedEvent.DomainResults))
}

// Sprint 9: Cross-domain risk assessment.
func runSprint9() {
	ctx := context.Background()

	// Create both pipelines.
	router := CreateEventOrchestrator()
	riskPipeline := CreateRiskAssessmentPipeline()

	// Chain them together.
	fullPipeline := pipz.NewSequence[FleetEvent]("full_orchestration")
	fullPipeline.Register(router, riskPipeline)

	// Create an event that will trigger multiple concerns.
	event := FleetEvent{
		EventID:   "EVT-400",
		Type:      EventHarshBraking,
		VehicleID: "SEDAN-003",
		DriverID:  "DRIVER-003",
		Timestamp: time.Now(),
		Data: map[string]any{
			"lat":          40.7489,
			"lng":          -73.9680,
			"speed":        55.0,
			"gforce":       1.1, // Very hard stop!
			"mileage":      89000,
			"engine_hours": 2100.5,
		},
	}

	fmt.Println("🚨 Processing event with cross-domain analysis...")

	processedEvent, err := fullPipeline.Process(ctx, event)
	if err != nil {
		log.Printf("Error: %v", err)
		return
	}

	// Show the risk assessment.
	if assessment, ok := processedEvent.DomainResults["risk_assessment"].(RiskAssessment); ok {
		fmt.Printf("\n📊 Cross-Domain Risk Assessment:\n")
		fmt.Printf("   Vehicle: %s (%s)\n", assessment.Vehicle.ID, assessment.Vehicle.Type)
		fmt.Printf("   Driver: %s\n", assessment.Driver.Name)
		fmt.Printf("   Risk Category: %s (%.1f%%)\n", assessment.RiskCategory, assessment.OverallRisk)
	}
}

// Sprint 11: Predictive analytics across domains.
func runSprint11() {
	ctx := context.Background()

	// Create full pipeline.
	router := CreateEventOrchestrator()
	riskPipeline := CreateRiskAssessmentPipeline()

	// Add predictive analytics.
	predictivePipeline := pipz.Apply("predictive_analytics",
		func(_ context.Context, event FleetEvent) (FleetEvent, error) {
			if event.DomainResults == nil {
				return event, nil
			}

			fmt.Println("\n🔮 Predictive Analytics Engine")

			// Analyze patterns across domains.
			patterns := []string{}

			// Safety patterns.
			if safety, ok := event.DomainResults["safety"].(SafetyResult); ok {
				if safety.UpdatedScore < 70 {
					patterns = append(patterns,
						"Driver showing risky behavior patterns")
				}
			}

			// Maintenance patterns.
			if maint, ok := event.DomainResults["maintenance"].(MaintenanceResult); ok {
				if maint.Status != "ok" {
					patterns = append(patterns,
						fmt.Sprintf("Vehicle maintenance degrading (%s)", maint.Status))
				}
			}

			// Route patterns.
			if route, ok := event.DomainResults["routing"].(RouteResult); ok {
				if route.Efficiency < 80 {
					patterns = append(patterns,
						"Route efficiency declining")
				}
			}

			// Make predictions.
			if len(patterns) >= 2 {
				fmt.Println("   ⚠️  PREDICTION: High probability of incident in next 30 days")
				fmt.Println("   Contributing factors:")
				for _, p := range patterns {
					fmt.Printf("      - %s\n", p)
				}

				// Preventive recommendations.
				fmt.Println("\n   💡 Preventive Actions:")
				fmt.Println("      1. Schedule immediate vehicle inspection")
				fmt.Println("      2. Assign driver to safety refresher course")
				fmt.Println("      3. Review and optimize current routes")
				fmt.Println("      4. Increase monitoring frequency")
			} else {
				fmt.Println("   ✅ All systems operating within normal parameters")
			}

			return event, nil
		})

	// Build complete pipeline.
	fullPipeline := pipz.NewSequence[FleetEvent]("predictive_orchestration")
	fullPipeline.Register(router, riskPipeline, predictivePipeline)

	// Simulate multiple events over time.
	vehicleIDs := []string{"TRUCK-001", "VAN-002", "SEDAN-003"}

	for i := 0; i < 3; i++ {
		vehicleID := vehicleIDs[i]

		// Generate event with concerning patterns.
		event := FleetEvent{
			EventID:   fmt.Sprintf("EVT-50%d", i),
			Type:      EventLocationUpdated,
			VehicleID: vehicleID,
			DriverID:  fmt.Sprintf("DRIVER-00%d", i+1),
			Timestamp: time.Now(),
			Data: map[string]any{
				"lat":          40.7128 + rand.Float64()*0.1,  //nolint:gosec // Mock data generation
				"lng":          -74.0060 + rand.Float64()*0.1, //nolint:gosec // Mock data generation
				"speed":        65.0 + rand.Float64()*20,      //nolint:gosec // Mock data generation
				"mileage":      150000 + rand.Intn(50000),     //nolint:gosec // Mock data generation
				"engine_hours": 2000.0 + rand.Float64()*500,   //nolint:gosec // Mock data generation
			},
		}

		fmt.Printf("\n📡 Processing vehicle %s...\n", vehicleID)

		_, err := fullPipeline.Process(ctx, event)
		if err != nil {
			log.Printf("Error: %v", err)
		}

		time.Sleep(500 * time.Millisecond) // Brief pause for readability
	}

	fmt.Println("\n🎯 Summary: Predictive operations prevent costly incidents!")
}
