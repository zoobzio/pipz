package main

import (
	"context"
	"fmt"
	"time"

	"github.com/zoobzio/pipz"
)

// Maintenance status constants.
const (
	maintenanceStatusServiceNow = "service_now"
	maintenanceStatusCritical   = "critical"
)

// Maintenance domain - owns MaintenanceCheck processing.

// MaintenanceCheck represents a maintenance evaluation.
type MaintenanceCheck struct { //nolint:govet // Demo struct
	Vehicle         Vehicle
	CurrentMileage  int
	EngineHours     float64
	LastServiceDate time.Time
	ServiceHistory  []ServiceRecord

	// Current diagnostics.
	DiagnosticCodes []string
	TirePressure    map[string]float64 // FL, FR, RL, RR
	OilLevel        float64
	BrakeWear       map[string]float64

	// Parts inventory.
	PartsAvailable map[string]int

	// Results (populated by pipeline).
	ProcessingResult MaintenanceResult
}

// Clone implements Cloner[MaintenanceCheck].
func (m MaintenanceCheck) Clone() MaintenanceCheck {
	clone := m

	// Deep copy slices.
	if m.ServiceHistory != nil {
		clone.ServiceHistory = make([]ServiceRecord, len(m.ServiceHistory))
		copy(clone.ServiceHistory, m.ServiceHistory)
	}
	if m.DiagnosticCodes != nil {
		clone.DiagnosticCodes = make([]string, len(m.DiagnosticCodes))
		copy(clone.DiagnosticCodes, m.DiagnosticCodes)
	}

	// Deep copy maps.
	if m.TirePressure != nil {
		clone.TirePressure = make(map[string]float64)
		for k, v := range m.TirePressure {
			clone.TirePressure[k] = v
		}
	}
	if m.BrakeWear != nil {
		clone.BrakeWear = make(map[string]float64)
		for k, v := range m.BrakeWear {
			clone.BrakeWear[k] = v
		}
	}
	if m.PartsAvailable != nil {
		clone.PartsAvailable = make(map[string]int)
		for k, v := range m.PartsAvailable {
			clone.PartsAvailable[k] = v
		}
	}

	return clone
}

// ServiceRecord tracks maintenance history.
type ServiceRecord struct { //nolint:govet // Demo struct
	Date          time.Time
	Mileage       int
	Type          string // "oil_change", "tire_rotation", etc.
	Cost          float64
	ServiceCenter string
	Technician    string
	Notes         string
}

// MaintenanceResult is returned after maintenance evaluation.
type MaintenanceResult struct { //nolint:govet // Demo struct
	Status           string // "ok", "service_soon", "service_now", "critical"
	NextServiceDate  time.Time
	NextServiceMiles int
	UrgentRepairs    []string
	EstimatedCost    float64
	PartsNeeded      map[string]int
}

// Internal pipeline - not exported!.
var maintenancePipeline *pipz.Sequence[MaintenanceCheck]

// InitializeMaintenance sets up the maintenance domain pipeline.
func InitializeMaintenance() {
	maintenancePipeline = pipz.NewSequence[MaintenanceCheck](PipelineMaintenanceCheck)

	// Step 1: Analyze service history.
	analyzeHistory := pipz.Apply(ProcessorServiceHistory, func(_ context.Context, check MaintenanceCheck) (MaintenanceCheck, error) {
		// Determine when last major services were performed.
		lastOilChange := time.Time{}
		lastTireRotation := time.Time{}
		lastBrakeService := time.Time{}

		for _, record := range check.ServiceHistory {
			switch record.Type {
			case "oil_change":
				if record.Date.After(lastOilChange) {
					lastOilChange = record.Date
				}
			case "tire_rotation":
				if record.Date.After(lastTireRotation) {
					lastTireRotation = record.Date
				}
			case "brake_service":
				if record.Date.After(lastBrakeService) {
					lastBrakeService = record.Date
				}
			}
		}

		// Calculate days since each service.
		daysSinceOil := int(time.Since(lastOilChange).Hours() / 24)
		daysSinceTires := int(time.Since(lastTireRotation).Hours() / 24)
		daysSinceBrakes := int(time.Since(lastBrakeService).Hours() / 24)

		fmt.Printf("   📊 Service history: Oil (%dd ago), Tires (%dd ago), Brakes (%dd ago)\n",
			daysSinceOil, daysSinceTires, daysSinceBrakes)

		// Initialize result.
		check.ProcessingResult.Status = "ok"
		check.ProcessingResult.UrgentRepairs = []string{}
		check.ProcessingResult.PartsNeeded = make(map[string]int)

		return check, nil
	})

	// Step 2: Check diagnostic codes.
	checkDiagnostics := pipz.Apply(ProcessorDiagnostics, func(_ context.Context, check MaintenanceCheck) (MaintenanceCheck, error) {
		if len(check.DiagnosticCodes) > 0 {
			fmt.Printf("   ⚠️  Active diagnostic codes: %v\n", check.DiagnosticCodes)

			// Critical codes that need immediate attention.
			criticalCodes := map[string]bool{
				"P0301": true, // Cylinder misfire
				"P0171": true, // System too lean
				"P0420": true, // Catalyst efficiency
				"P0442": true, // Evap leak
			}

			for _, code := range check.DiagnosticCodes {
				if criticalCodes[code] {
					fmt.Printf("   🚨 CRITICAL CODE: %s requires immediate service!\n", code)
					check.ProcessingResult.Status = maintenanceStatusServiceNow
					check.ProcessingResult.UrgentRepairs = append(check.ProcessingResult.UrgentRepairs,
						fmt.Sprintf("Diagnostic code %s", code))
					check.ProcessingResult.EstimatedCost += 200.0
				}
			}
		}

		return check, nil
	})

	// Step 3: Analyze wear levels.
	analyzeWear := pipz.Apply(ProcessorWearAnalysis, func(_ context.Context, check MaintenanceCheck) (MaintenanceCheck, error) {
		// Check tire pressure.
		lowPressureCount := 0
		for position, pressure := range check.TirePressure {
			if pressure < 32.0 { // PSI threshold
				fmt.Printf("   ⚠️  Low tire pressure: %s = %.1f PSI\n", position, pressure)
				lowPressureCount++
			}
		}

		// Check brake wear.
		for position, wear := range check.BrakeWear {
			if wear > 0.7 { // 70% worn
				fmt.Printf("   ⚠️  Brake wear high: %s = %.0f%% worn\n", position, wear*100)
				if wear > 0.8 {
					check.ProcessingResult.Status = maintenanceStatusCritical
					check.ProcessingResult.UrgentRepairs = append(check.ProcessingResult.UrgentRepairs,
						fmt.Sprintf("Replace %s brake pads", position))
					check.ProcessingResult.EstimatedCost += 150.0
					check.ProcessingResult.PartsNeeded["brake_pads"]++
				}
			}
		}

		// Oil level.
		if check.OilLevel < 0.3 {
			fmt.Printf("   🛢️  Low oil level: %.0f%%\n", check.OilLevel*100)
			if check.ProcessingResult.Status == "ok" {
				check.ProcessingResult.Status = maintenanceStatusServiceNow
			}
			check.ProcessingResult.UrgentRepairs = append(check.ProcessingResult.UrgentRepairs, "Oil change")
			check.ProcessingResult.EstimatedCost += 75.0
			check.ProcessingResult.PartsNeeded["oil_filter"] = 1
		}

		return check, nil
	})

	// Step 4: Calculate next service.
	calculateService := pipz.Apply(ProcessorServiceScheduling, func(_ context.Context, check MaintenanceCheck) (MaintenanceCheck, error) {
		// Service intervals (simplified).
		oilChangeInterval := 5000     // miles
		tireRotationInterval := 7500  // miles
		majorServiceInterval := 30000 // miles

		// Find last service mileage (not currently used but would be for more complex intervals).
		// lastServiceMileage := 0.
		// if len(check.ServiceHistory) > 0 {
		// 	lastServiceMileage = check.ServiceHistory[len(check.ServiceHistory)-1].Mileage
		// }.

		// Determine what's needed.
		services := []string{}

		if check.CurrentMileage%oilChangeInterval < 500 {
			services = append(services, "Oil Change")
			check.ProcessingResult.PartsNeeded["oil_filter"] = 1
		}
		if check.CurrentMileage%tireRotationInterval < 500 {
			services = append(services, "Tire Rotation")
		}
		if check.CurrentMileage%majorServiceInterval < 1000 {
			services = append(services, "Major Service")
			check.ProcessingResult.PartsNeeded["air_filter"] = 1
		}

		if len(services) > 0 {
			fmt.Printf("   🔧 Services due soon: %v (at %d miles)\n", services, check.CurrentMileage)
			if check.ProcessingResult.Status == "ok" {
				check.ProcessingResult.Status = "service_soon"
			}
		}

		// Calculate next service.
		nextServiceMiles := ((check.CurrentMileage / 5000) + 1) * 5000
		daysUntilService := (nextServiceMiles - check.CurrentMileage) / 100 // Assume 100 miles/day
		nextServiceDate := time.Now().Add(time.Duration(daysUntilService) * 24 * time.Hour)

		check.ProcessingResult.NextServiceDate = nextServiceDate
		check.ProcessingResult.NextServiceMiles = nextServiceMiles

		return check, nil
	})

	// Step 5: Check parts inventory.
	checkInventory := pipz.Apply(ProcessorPartsCheck, func(_ context.Context, check MaintenanceCheck) (MaintenanceCheck, error) {
		// See what's missing.
		missingParts := []string{}
		for part, needed := range check.ProcessingResult.PartsNeeded {
			available := check.PartsAvailable[part]
			if available < needed {
				missingParts = append(missingParts, part)
			}
		}

		if len(missingParts) > 0 {
			fmt.Printf("   📦 Parts to order: %v\n", missingParts)
		}

		return check, nil
	})

	// Register all steps.
	maintenancePipeline.Register(
		analyzeHistory,
		checkDiagnostics,
		analyzeWear,
		calculateService,
		checkInventory,
	)
}

// ProcessMaintenanceCheck is the public API for the maintenance domain.
func ProcessMaintenanceCheck(ctx context.Context, check MaintenanceCheck) (MaintenanceCheck, *pipz.Error[MaintenanceCheck]) {
	// Process through internal pipeline.
	return maintenancePipeline.Process(ctx, check)
}

// Processor names.
const (
	PipelineMaintenanceCheck   = "maintenance_check"
	ProcessorServiceHistory    = "service_history"
	ProcessorDiagnostics       = "diagnostics"
	ProcessorWearAnalysis      = "wear_analysis"
	ProcessorServiceScheduling = "service_scheduling"
	ProcessorPartsCheck        = "parts_check"
)
