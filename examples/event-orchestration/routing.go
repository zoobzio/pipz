package main

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/zoobzio/pipz"
)

// Routing domain - owns RouteAnalysis processing.

// RouteAnalysis evaluates route performance.
type RouteAnalysis struct { //nolint:govet // Demo struct
	Vehicle      Vehicle
	Driver       Driver
	PlannedRoute Route
	ActualPath   []GPSCoordinate
	StartTime    time.Time
	EndTime      time.Time

	// Deliveries.
	PlannedStops   []DeliveryStop
	CompletedStops []DeliveryStop

	// Conditions.
	TrafficData TrafficConditions
	WeatherData WeatherConditions

	// Efficiency metrics.
	PlannedDistance float64
	ActualDistance  float64
	FuelUsed        float64

	// Results (populated by pipeline).
	ProcessingResult RouteResult
}

// Clone implements Cloner[RouteAnalysis].
func (r RouteAnalysis) Clone() RouteAnalysis {
	clone := r

	// Deep copy slices.
	if r.ActualPath != nil {
		clone.ActualPath = make([]GPSCoordinate, len(r.ActualPath))
		copy(clone.ActualPath, r.ActualPath)
	}
	if r.PlannedStops != nil {
		clone.PlannedStops = make([]DeliveryStop, len(r.PlannedStops))
		copy(clone.PlannedStops, r.PlannedStops)
	}
	if r.CompletedStops != nil {
		clone.CompletedStops = make([]DeliveryStop, len(r.CompletedStops))
		copy(clone.CompletedStops, r.CompletedStops)
	}

	// Deep copy TrafficConditions.
	if r.TrafficData.Incidents != nil {
		clone.TrafficData.Incidents = make([]string, len(r.TrafficData.Incidents))
		copy(clone.TrafficData.Incidents, r.TrafficData.Incidents)
	}

	return clone
}

// Route represents a planned route.
type Route struct {
	ID        string
	Name      string
	Waypoints []GPSCoordinate
	Distance  float64
	Duration  time.Duration
}

// DeliveryStop represents a delivery location.
type DeliveryStop struct { //nolint:govet // Demo struct
	ID            string
	Location      GPSCoordinate
	ScheduledTime time.Time
	ActualTime    time.Time
	Customer      string
	Status        string // "pending", "completed", "failed"
}

// TrafficConditions at a point in time.
type TrafficConditions struct {
	Level     string // "light", "moderate", "heavy"
	Incidents []string
	AvgSpeed  float64
}

// WeatherConditions affecting driving.
type WeatherConditions struct {
	Type        string // "clear", "rain", "snow", "fog"
	Visibility  float64
	Temperature float64
	WindSpeed   float64
}

// RouteResult is returned after route analysis.
type RouteResult struct { //nolint:govet // Demo struct
	Efficiency       float64 // 0-100%
	DeviationCount   int
	DeliveryRate     float64 // Completed/Total
	FuelEfficiency   float64 // MPG
	RecommendedRoute *Route
	CostSavings      float64
}

// Internal pipeline - not exported!.
var routingPipeline *pipz.Sequence[RouteAnalysis]

// InitializeRouting sets up the routing domain pipeline.
func InitializeRouting() {
	routingPipeline = pipz.NewSequence[RouteAnalysis](PipelineRouteAnalysis)

	// Step 1: Calculate route deviation.
	calculateDeviation := pipz.Apply(ProcessorDeviationAnalysis, func(_ context.Context, analysis RouteAnalysis) (RouteAnalysis, error) {
		// Initialize result.
		analysis.ProcessingResult = RouteResult{}

		// Compare planned route to actual path.
		if len(analysis.ActualPath) == 0 {
			return analysis, nil
		}

		// Calculate total deviation distance.
		totalDeviation := 0.0
		significantDeviations := 0

		for _, actualPoint := range analysis.ActualPath {
			// Find closest planned waypoint.
			minDistance := math.MaxFloat64
			for _, plannedPoint := range analysis.PlannedRoute.Waypoints {
				dist := calculateDistance(actualPoint, plannedPoint)
				if dist < minDistance {
					minDistance = dist
				}
			}

			// If more than 0.5 miles off route
			if minDistance > 0.5 {
				significantDeviations++
				totalDeviation += minDistance
			}
		}

		analysis.ProcessingResult.DeviationCount = significantDeviations

		if significantDeviations > 0 {
			fmt.Printf("   📍 Route deviations: %d times, total %.1f miles off-route\n",
				significantDeviations, totalDeviation)
		}

		return analysis, nil
	})

	// Step 2: Analyze delivery performance.
	analyzeDeliveries := pipz.Apply(ProcessorDeliveryAnalysis, func(_ context.Context, analysis RouteAnalysis) (RouteAnalysis, error) {
		if len(analysis.PlannedStops) == 0 {
			analysis.ProcessingResult.DeliveryRate = 1.0
			return analysis, nil
		}

		onTimeCount := 0
		lateCount := 0
		failedCount := 0
		totalDelay := 0.0

		for i := range analysis.CompletedStops {
			if i >= len(analysis.PlannedStops) {
				break
			}

			stop := &analysis.CompletedStops[i]
			planned := &analysis.PlannedStops[i]

			switch stop.Status {
			case "completed":
				delay := stop.ActualTime.Sub(planned.ScheduledTime).Minutes()
				if delay <= 15 { // 15 min grace period
					onTimeCount++
				} else {
					lateCount++
					totalDelay += delay
				}
			case "failed":
				failedCount++
			}
		}

		deliveryRate := float64(len(analysis.CompletedStops)) / float64(len(analysis.PlannedStops))
		analysis.ProcessingResult.DeliveryRate = deliveryRate

		fmt.Printf("   📦 Deliveries: %.0f%% complete (%d on-time, %d late, %d failed)\n",
			deliveryRate*100, onTimeCount, lateCount, failedCount)

		if lateCount > 0 {
			avgDelay := totalDelay / float64(lateCount)
			fmt.Printf("   ⏱️  Average delay: %.0f minutes\n", avgDelay)
		}

		return analysis, nil
	})

	// Step 3: Calculate fuel efficiency.
	calculateEfficiency := pipz.Apply(ProcessorEfficiencyCalc, func(_ context.Context, analysis RouteAnalysis) (RouteAnalysis, error) {
		// Calculate actual vs planned efficiency.
		if analysis.FuelUsed > 0 && analysis.ActualDistance > 0 {
			actualMPG := analysis.ActualDistance / analysis.FuelUsed
			analysis.ProcessingResult.FuelEfficiency = actualMPG

			// Expected MPG based on vehicle type.
			expectedMPG := 8.0 // Default for truck
			if analysis.Vehicle.Type == "van" {
				expectedMPG = 15.0
			} else if analysis.Vehicle.Type == "sedan" {
				expectedMPG = 25.0
			}

			efficiency := (actualMPG / expectedMPG) * 100

			fmt.Printf("   ⛽ Fuel efficiency: %.1f MPG (%.0f%% of expected)\n",
				actualMPG, efficiency)

			// Check if traffic or weather impacted efficiency.
			if analysis.TrafficData.Level == "heavy" {
				fmt.Printf("   🚦 Heavy traffic reduced efficiency\n")
			}
			if analysis.WeatherData.Type != "clear" {
				fmt.Printf("   🌧️  %s conditions impacted fuel consumption\n", analysis.WeatherData.Type)
			}
		}

		// Calculate route efficiency.
		if analysis.PlannedRoute.Distance > 0 && analysis.ActualDistance > 0 {
			analysis.ProcessingResult.Efficiency = (analysis.PlannedRoute.Distance / analysis.ActualDistance) * 100
		} else {
			analysis.ProcessingResult.Efficiency = 100.0
		}

		return analysis, nil
	})

	// Step 4: Optimize future routes.
	optimizeRoute := pipz.Apply(ProcessorRouteOptimization, func(_ context.Context, analysis RouteAnalysis) (RouteAnalysis, error) {
		// Suggest optimizations based on analysis.
		suggestions := []string{}

		// If significant deviations, suggest route update.
		if analysis.EndTime.IsZero() {
			analysis.EndTime = time.Now() // For demo
		}
		duration := analysis.EndTime.Sub(analysis.StartTime)
		plannedDuration := analysis.PlannedRoute.Duration

		if duration > time.Duration(float64(plannedDuration)*1.2) {
			suggestions = append(suggestions, "Consider updating route to avoid traffic patterns")
		}

		// If deliveries were late, suggest earlier start.
		lateDeliveries := 0
		for i := range analysis.CompletedStops {
			stop := &analysis.CompletedStops[i]
			if stop.Status == "completed" && !stop.ActualTime.IsZero() {
				for j := range analysis.PlannedStops {
					planned := &analysis.PlannedStops[j]
					if stop.ID == planned.ID && stop.ActualTime.After(planned.ScheduledTime.Add(15*time.Minute)) {
						lateDeliveries++
						break
					}
				}
			}
		}

		if lateDeliveries > len(analysis.PlannedStops)/4 {
			suggestions = append(suggestions, "Start route 30 minutes earlier to improve on-time rate")
		}

		// Weather-based suggestions.
		if analysis.WeatherData.Type == "snow" || analysis.WeatherData.Type == "rain" {
			suggestions = append(suggestions, "Add 20% buffer to delivery times during adverse weather")
		}

		if len(suggestions) > 0 {
			fmt.Printf("   💡 Optimization suggestions:\n")
			for _, s := range suggestions {
				fmt.Printf("      - %s\n", s)
			}
		}

		return analysis, nil
	})

	// Step 5: Calculate cost impact.
	calculateCost := pipz.Apply(ProcessorCostAnalysis, func(_ context.Context, analysis RouteAnalysis) (RouteAnalysis, error) {
		// Calculate various costs.
		fuelCost := analysis.FuelUsed * 3.50 // $3.50/gallon

		// Time cost (driver wages).
		if analysis.EndTime.IsZero() {
			analysis.EndTime = time.Now()
		}
		duration := analysis.EndTime.Sub(analysis.StartTime).Hours()
		laborCost := duration * 25.0 // $25/hour

		// Efficiency loss cost.
		efficiencyLoss := 0.0
		if analysis.ActualDistance > analysis.PlannedRoute.Distance {
			extraMiles := analysis.ActualDistance - analysis.PlannedRoute.Distance
			efficiencyLoss = extraMiles * 0.65 // $0.65/mile operating cost
		}

		totalCost := fuelCost + laborCost + efficiencyLoss

		fmt.Printf("   💰 Route costs: Fuel $%.2f + Labor $%.2f + Inefficiency $%.2f = $%.2f\n",
			fuelCost, laborCost, efficiencyLoss, totalCost)

		// Calculate potential savings.
		if efficiencyLoss > 50 {
			potentialSavings := efficiencyLoss * 0.7 // Could save 70% with optimization
			analysis.ProcessingResult.CostSavings = potentialSavings
			fmt.Printf("   💵 Potential savings with optimization: $%.2f/day\n", potentialSavings)
		}

		return analysis, nil
	})

	// Register all steps.
	routingPipeline.Register(
		calculateDeviation,
		analyzeDeliveries,
		calculateEfficiency,
		optimizeRoute,
		calculateCost,
	)
}

// ProcessRouteAnalysis is the public API for the routing domain.
func ProcessRouteAnalysis(ctx context.Context, analysis RouteAnalysis) (RouteAnalysis, error) {
	// Process through internal pipeline.
	return routingPipeline.Process(ctx, analysis)
}

// Helper function to calculate distance between GPS coordinates.
func calculateDistance(p1, p2 GPSCoordinate) float64 {
	// Simplified distance calculation (not accurate for real GPS).
	latDiff := p1.Latitude - p2.Latitude
	lngDiff := p1.Longitude - p2.Longitude
	return math.Sqrt(latDiff*latDiff+lngDiff*lngDiff) * 69.0 // Rough miles conversion
}

// Processor names.
const (
	PipelineRouteAnalysis      = "route_analysis"
	ProcessorDeviationAnalysis = "deviation_analysis"
	ProcessorDeliveryAnalysis  = "delivery_analysis"
	ProcessorEfficiencyCalc    = "efficiency_calc"
	ProcessorRouteOptimization = "route_optimization"
	ProcessorCostAnalysis      = "cost_analysis"
)
