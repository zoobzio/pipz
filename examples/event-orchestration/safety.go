package main

import (
	"context"
	"fmt"
	"time"

	"github.com/zoobzio/pipz"
)

// Safety domain - owns SafetyIncident processing.

// SafetyIncident represents a safety-related event with full context.
type SafetyIncident struct { //nolint:govet // Demo struct
	IncidentID string
	Vehicle    Vehicle
	Driver     Driver
	Type       string // "harsh_braking", "speeding", etc.
	Severity   int    // 1-10 scale
	Location   GPSCoordinate
	Timestamp  time.Time
	GForce     float64
	VideoURL   string // Dash cam footage

	// Context.
	SpeedLimit        int
	WeatherConditions string
	TrafficLevel      string
	NearbyVehicles    []string

	// Results (populated by pipeline).
	ProcessingResult SafetyResult
}

// Clone implements Cloner[SafetyIncident].
func (s SafetyIncident) Clone() SafetyIncident {
	clone := s
	// Deep copy slices.
	if s.NearbyVehicles != nil {
		clone.NearbyVehicles = make([]string, len(s.NearbyVehicles))
		copy(clone.NearbyVehicles, s.NearbyVehicles)
	}
	return clone
}

// SafetyResult holds the processing results.
type SafetyResult struct { //nolint:govet // Demo struct
	IncidentProcessed bool
	UpdatedScore      float64
	AlertsSent        []string
	TrainingRequired  bool
	VideoArchived     bool
}

// Internal pipeline - not exported!.
var safetyPipeline *pipz.Sequence[SafetyIncident]

// InitializeSafety sets up the safety domain pipeline.
func InitializeSafety() {
	safetyPipeline = pipz.NewSequence[SafetyIncident](PipelineSafetyIncident)

	// Step 1: Analyze severity.
	analyzeSeverity := pipz.Mutate(ProcessorSeverityAnalysis,
		func(_ context.Context, incident SafetyIncident) SafetyIncident {
			// Calculate severity based on multiple factors.
			severity := 0

			// Speed factor.
			if incident.Location.Speed > float64(incident.SpeedLimit+20) {
				severity += 5
			} else if incident.Location.Speed > float64(incident.SpeedLimit+10) {
				severity += 3
			}

			// G-force factor (harsh events).
			if incident.GForce > 0.5 {
				severity += 4
			} else if incident.GForce > 0.3 {
				severity += 2
			}

			// Environmental factors.
			if incident.WeatherConditions != "clear" {
				severity += 2
			}
			if incident.TrafficLevel == "heavy" {
				severity += 2
			}

			// Cap at 10.
			if severity > 10 {
				severity = 10
			}

			incident.Severity = severity
			fmt.Printf("   📊 Calculated severity: %d/10\n", severity)

			return incident
		},
		func(_ context.Context, incident SafetyIncident) bool {
			return incident.Severity == 0 // Only calculate if not set
		},
	)

	// Step 2: Update driver profile.
	updateDriverProfile := pipz.Apply(ProcessorDriverScoring, func(_ context.Context, incident SafetyIncident) (SafetyIncident, error) {
		// In real system, this would update a database.
		// For demo, we'll simulate the scoring.

		// Mock current score.
		currentScore := 85.0

		// Deduct points based on severity.
		pointDeduction := float64(incident.Severity) * 2.5
		newScore := currentScore - pointDeduction

		if newScore < 0 {
			newScore = 0
		}

		fmt.Printf("   📊 Driver %s score: %.1f → %.1f (-%0.1f for severity %d incident)\n",
			incident.Driver.Name, currentScore, newScore, pointDeduction, incident.Severity)

		// Store in result.
		incident.ProcessingResult.UpdatedScore = newScore

		return incident, nil
	})

	// Step 3: Archive video if severe.
	archiveVideo := pipz.Mutate(ProcessorVideoArchive,
		func(_ context.Context, incident SafetyIncident) SafetyIncident {
			fmt.Printf("   📹 Archiving dash cam footage: %s\n", incident.VideoURL)
			// In real system, would trigger video archival.
			incident.ProcessingResult.VideoArchived = true
			return incident
		},
		func(_ context.Context, incident SafetyIncident) bool {
			return incident.Severity >= 7 // Only archive severe incidents
		},
	)

	// Step 4: Send alerts based on severity.
	sendAlerts := pipz.Apply(ProcessorSafetyAlerts, func(_ context.Context, incident SafetyIncident) (SafetyIncident, error) {
		alerts := []string{}

		if incident.Severity >= 8 {
			// Critical - alert fleet manager immediately.
			alert := fmt.Sprintf("🚨 CRITICAL: %s - %s (Severity %d/10)",
				incident.Driver.Name, incident.Type, incident.Severity)
			alerts = append(alerts, alert)

			// Also alert safety director.
			alerts = append(alerts, fmt.Sprintf("Safety Director: Review incident %s immediately", incident.IncidentID))
		} else if incident.Severity >= 5 {
			// Moderate - daily report.
			alert := fmt.Sprintf("⚠️ Safety incident: %s - %s (Severity %d/10)",
				incident.Driver.Name, incident.Type, incident.Severity)
			alerts = append(alerts, alert)
		}

		// Send alerts (mock).
		for _, alert := range alerts {
			fmt.Printf("   %s\n", alert)
			_ = notificationService.SendAlert("Fleet Manager", alert) //nolint:errcheck // Demo notification
		}

		incident.ProcessingResult.AlertsSent = alerts

		return incident, nil
	})

	// Step 5: Check if training required.
	checkTraining := pipz.Apply(ProcessorTrainingCheck, func(_ context.Context, incident SafetyIncident) (SafetyIncident, error) {
		// Mock: 3 incidents in a month = training required.
		recentIncidents := 2 // Would query from database

		if recentIncidents+1 >= 3 {
			fmt.Printf("   🎓 Training required for driver %s (3 incidents in 30 days)\n", incident.Driver.Name)
			incident.ProcessingResult.TrainingRequired = true
		}

		// Mark as processed.
		incident.ProcessingResult.IncidentProcessed = true

		return incident, nil
	})

	// Register all steps.
	safetyPipeline.Register(
		analyzeSeverity,
		updateDriverProfile,
		archiveVideo,
		sendAlerts,
		checkTraining,
	)
}

// ProcessSafetyIncident is the public API for the safety domain.
func ProcessSafetyIncident(ctx context.Context, incident SafetyIncident) (SafetyIncident, error) {
	// Add processing timestamp.
	incident.Timestamp = time.Now()

	// Initialize result.
	incident.ProcessingResult = SafetyResult{
		UpdatedScore: 75.0, // Default score
	}

	// Process through internal pipeline.
	return safetyPipeline.Process(ctx, incident)
}

// Processor names.
const (
	PipelineSafetyIncident    = "safety_incident"
	ProcessorSeverityAnalysis = "severity_analysis"
	ProcessorDriverScoring    = "driver_scoring"
	ProcessorVideoArchive     = "video_archive"
	ProcessorSafetyAlerts     = "safety_alerts"
	ProcessorTrainingCheck    = "training_check"
)
