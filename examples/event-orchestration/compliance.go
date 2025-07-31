package main

import (
	"context"
	"fmt"
	"time"

	"github.com/zoobzio/pipz"
)

// Compliance domain - owns ComplianceCheck processing.

// ComplianceCheck ensures regulatory compliance.
type ComplianceCheck struct { //nolint:govet // Demo struct
	Driver  Driver
	Vehicle Vehicle
	Date    time.Time

	// Hours of Service (HOS).
	DrivingHours float64
	OnDutyHours  float64
	RestPeriods  []RestPeriod
	Last7Days    []DailyLog
	Last14Days   []DailyLog

	// Certifications.
	DriverLicense  LicenseStatus
	VehiclePermits []Permit
	Insurance      InsurancePolicy

	// Violations.
	RecentViolations []Violation

	// Results (populated by pipeline).
	ProcessingResult ComplianceResult
}

// Clone implements Cloner[ComplianceCheck].
func (c ComplianceCheck) Clone() ComplianceCheck {
	clone := c

	// Deep copy slices.
	if c.RestPeriods != nil {
		clone.RestPeriods = make([]RestPeriod, len(c.RestPeriods))
		copy(clone.RestPeriods, c.RestPeriods)
	}
	if c.Last7Days != nil {
		clone.Last7Days = make([]DailyLog, len(c.Last7Days))
		copy(clone.Last7Days, c.Last7Days)
	}
	if c.Last14Days != nil {
		clone.Last14Days = make([]DailyLog, len(c.Last14Days))
		copy(clone.Last14Days, c.Last14Days)
	}
	if c.VehiclePermits != nil {
		clone.VehiclePermits = make([]Permit, len(c.VehiclePermits))
		copy(clone.VehiclePermits, c.VehiclePermits)
	}
	if c.RecentViolations != nil {
		clone.RecentViolations = make([]Violation, len(c.RecentViolations))
		copy(clone.RecentViolations, c.RecentViolations)
	}

	// Deep copy complex types.
	if c.DriverLicense.Endorsements != nil {
		clone.DriverLicense.Endorsements = make([]string, len(c.DriverLicense.Endorsements))
		copy(clone.DriverLicense.Endorsements, c.DriverLicense.Endorsements)
	}
	if c.DriverLicense.Restrictions != nil {
		clone.DriverLicense.Restrictions = make([]string, len(c.DriverLicense.Restrictions))
		copy(clone.DriverLicense.Restrictions, c.DriverLicense.Restrictions)
	}

	return clone
}

// RestPeriod tracks driver rest.
type RestPeriod struct {
	Start    time.Time
	End      time.Time
	Location string
	Type     string // "short_break", "daily_rest", "weekly_rest"
}

// DailyLog for HOS tracking.
type DailyLog struct {
	Date         time.Time
	DrivingHours float64
	OnDutyHours  float64
	RestHours    float64
}

// LicenseStatus tracks license validity.
type LicenseStatus struct {
	Number       string
	Class        string
	Expiry       time.Time
	Endorsements []string
	Restrictions []string
	Valid        bool
}

// Permit for special cargo or routes.
type Permit struct {
	Type   string
	Number string
	Expiry time.Time
	States []string
}

// InsurancePolicy details.
type InsurancePolicy struct { //nolint:govet // Demo struct
	Provider     string
	PolicyNumber string
	Coverage     float64
	Expiry       time.Time
}

// Violation record.
type Violation struct {
	Date        time.Time
	Type        string
	Description string
	Fine        float64
	Points      int
}

// ComplianceResult after compliance check.
type ComplianceResult struct { //nolint:govet // Demo struct
	Compliant       bool
	HOSRemaining    float64 // Hours until mandatory rest
	Violations      []string
	ExpiringItems   map[string]time.Time
	RequiredActions []string
}

// Internal pipeline - not exported!.
var compliancePipeline *pipz.Sequence[ComplianceCheck]

// InitializeCompliance sets up the compliance domain pipeline.
func InitializeCompliance() {
	compliancePipeline = pipz.NewSequence[ComplianceCheck](PipelineComplianceCheck)

	// Step 1: Check Hours of Service (HOS).
	checkHOS := pipz.Apply(ProcessorHOSCalculation, func(_ context.Context, check ComplianceCheck) (ComplianceCheck, error) {
		// Initialize result.
		check.ProcessingResult = ComplianceResult{
			Compliant:       true,
			Violations:      []string{},
			ExpiringItems:   make(map[string]time.Time),
			RequiredActions: []string{},
		}

		// Federal HOS rules (simplified):.
		// - Max 11 hours driving in 14-hour window.
		// - Max 70 hours in 8 days.
		// - Required 30-min break after 8 hours.

		fmt.Printf("   🕐 Today's hours: %.1fh driving, %.1fh on-duty\n",
			check.DrivingHours, check.OnDutyHours)

		// Check daily limits.
		if check.DrivingHours > 11 {
			fmt.Printf("   ❌ VIOLATION: Exceeded 11-hour driving limit!\n")
			check.ProcessingResult.Compliant = false
			check.ProcessingResult.Violations = append(check.ProcessingResult.Violations, "Exceeded daily driving hours")
			check.ProcessingResult.RequiredActions = append(check.ProcessingResult.RequiredActions, "Stop driving immediately")
			check.ProcessingResult.HOSRemaining = 0
		} else {
			check.ProcessingResult.HOSRemaining = 11.0 - check.DrivingHours
			if check.DrivingHours > 10 {
				fmt.Printf("   ⚠️  Warning: Approaching 11-hour driving limit (%.1fh remaining)\n",
					check.ProcessingResult.HOSRemaining)
				check.ProcessingResult.RequiredActions = append(check.ProcessingResult.RequiredActions,
					"Plan for rest within 2 hours")
			}
		}

		if check.OnDutyHours > 14 {
			fmt.Printf("   ❌ VIOLATION: Exceeded 14-hour on-duty limit!\n")
			check.ProcessingResult.Compliant = false
			check.ProcessingResult.Violations = append(check.ProcessingResult.Violations, "Exceeded on-duty hours")
		}

		// Check 8-day total.
		totalHours := check.DrivingHours
		for _, day := range check.Last7Days {
			totalHours += day.DrivingHours
		}

		if totalHours > 70 {
			fmt.Printf("   ❌ VIOLATION: Exceeded 70-hour/8-day limit (%.1fh)!\n", totalHours)
			check.ProcessingResult.Compliant = false
			check.ProcessingResult.Violations = append(check.ProcessingResult.Violations, "Exceeded 8-day hours limit")
		} else {
			fmt.Printf("   ✓ 8-day total: %.1fh of 70h allowed\n", totalHours)
		}

		// Check rest periods.
		lastBreak := time.Now()
		for _, rest := range check.RestPeriods {
			if rest.Type == "short_break" && rest.End.After(lastBreak) {
				lastBreak = rest.End
			}
		}

		hoursSinceBreak := time.Since(lastBreak).Hours()
		if hoursSinceBreak > 8 {
			fmt.Printf("   ❌ VIOLATION: No 30-minute break in last 8 hours!\n")
			check.ProcessingResult.Compliant = false
			check.ProcessingResult.Violations = append(check.ProcessingResult.Violations, "Missing required break")
		}

		return check, nil
	})

	// Step 2: Verify licenses and certifications.
	verifyLicenses := pipz.Apply(ProcessorLicenseVerify, func(_ context.Context, check ComplianceCheck) (ComplianceCheck, error) {
		// Check driver's license.
		daysUntilExpiry := time.Until(check.DriverLicense.Expiry).Hours() / 24

		if daysUntilExpiry < 0 {
			fmt.Printf("   ❌ EXPIRED: Driver's license expired %d days ago!\n", int(-daysUntilExpiry))
			check.ProcessingResult.Compliant = false
			check.ProcessingResult.Violations = append(check.ProcessingResult.Violations, "Expired driver's license")
			check.ProcessingResult.RequiredActions = append(check.ProcessingResult.RequiredActions,
				"Renew driver's license immediately")
		} else if daysUntilExpiry < 30 {
			fmt.Printf("   ⚠️  Driver's license expires in %d days\n", int(daysUntilExpiry))
			check.ProcessingResult.ExpiringItems["Driver's License"] = check.DriverLicense.Expiry
			check.ProcessingResult.RequiredActions = append(check.ProcessingResult.RequiredActions,
				"Schedule license renewal")
		}

		// Check required endorsements.
		requiredEndorsements := []string{}
		if check.Vehicle.Type == "truck" {
			requiredEndorsements = append(requiredEndorsements, "CDL-A")
		}

		hasRequiredEndorsements := true
		for _, required := range requiredEndorsements {
			found := false
			for _, endorsement := range check.DriverLicense.Endorsements {
				if endorsement == required {
					found = true
					break
				}
			}
			if !found {
				fmt.Printf("   ❌ Missing required endorsement: %s\n", required)
				check.ProcessingResult.Compliant = false
				check.ProcessingResult.Violations = append(check.ProcessingResult.Violations,
					fmt.Sprintf("Missing %s endorsement", required))
				hasRequiredEndorsements = false
			}
		}

		if hasRequiredEndorsements && daysUntilExpiry > 30 {
			fmt.Printf("   ✓ Driver license valid with all endorsements\n")
		}

		return check, nil
	})

	// Step 3: Check vehicle permits.
	checkPermits := pipz.Apply(ProcessorPermitCheck, func(_ context.Context, check ComplianceCheck) (ComplianceCheck, error) {
		// Check each permit.
		expiringPermits := []string{}
		expiredPermits := []string{}

		for _, permit := range check.VehiclePermits {
			daysUntilExpiry := time.Until(permit.Expiry).Hours() / 24

			if daysUntilExpiry < 0 {
				expiredPermits = append(expiredPermits, permit.Type)
				check.ProcessingResult.Compliant = false
				check.ProcessingResult.Violations = append(check.ProcessingResult.Violations,
					fmt.Sprintf("Expired %s permit", permit.Type))
			} else if daysUntilExpiry < 30 {
				expiringPermits = append(expiringPermits,
					fmt.Sprintf("%s (expires in %d days)", permit.Type, int(daysUntilExpiry)))
				check.ProcessingResult.ExpiringItems[permit.Type+" Permit"] = permit.Expiry
			}
		}

		if len(expiredPermits) > 0 {
			fmt.Printf("   ❌ EXPIRED PERMITS: %v\n", expiredPermits)
		}
		if len(expiringPermits) > 0 {
			fmt.Printf("   ⚠️  Expiring permits: %v\n", expiringPermits)
		}

		// Check insurance.
		insuranceDays := time.Until(check.Insurance.Expiry).Hours() / 24
		switch {
		case insuranceDays < 0:
			fmt.Printf("   ❌ INSURANCE EXPIRED!\n")
			check.ProcessingResult.Compliant = false
			check.ProcessingResult.Violations = append(check.ProcessingResult.Violations, "Expired insurance")
			check.ProcessingResult.RequiredActions = append(check.ProcessingResult.RequiredActions,
				"Renew insurance policy")
		case insuranceDays < 30:
			fmt.Printf("   ⚠️  Insurance expires in %d days\n", int(insuranceDays))
			check.ProcessingResult.ExpiringItems["Insurance Policy"] = check.Insurance.Expiry
			check.ProcessingResult.RequiredActions = append(check.ProcessingResult.RequiredActions,
				"Contact insurance provider")
		default:
			fmt.Printf("   ✓ Insurance valid ($%.0f coverage)\n", check.Insurance.Coverage)
		}

		return check, nil
	})

	// Step 4: Review recent violations.
	reviewViolations := pipz.Apply(ProcessorViolationReview, func(_ context.Context, check ComplianceCheck) (ComplianceCheck, error) {
		if len(check.RecentViolations) == 0 {
			fmt.Printf("   ✓ No recent violations\n")
			return check, nil
		}

		// Count violations by type.
		violationCounts := make(map[string]int)
		totalFines := 0.0
		totalPoints := 0

		for _, violation := range check.RecentViolations {
			violationCounts[violation.Type]++
			totalFines += violation.Fine
			totalPoints += violation.Points
		}

		fmt.Printf("   ⚖️  Recent violations: %d total\n", len(check.RecentViolations))
		for vType, count := range violationCounts {
			fmt.Printf("      - %s: %d times\n", vType, count)
		}
		fmt.Printf("   💰 Total fines: $%.2f\n", totalFines)
		fmt.Printf("   📊 Total points: %d\n", totalPoints)

		// Check if too many points.
		if totalPoints >= 12 {
			fmt.Printf("   ❌ CRITICAL: License suspension risk!\n")
			check.ProcessingResult.RequiredActions = append(check.ProcessingResult.RequiredActions,
				"Review safety practices immediately")
		} else if totalPoints >= 8 {
			fmt.Printf("   ⚠️  Warning: High point total\n")
		}

		return check, nil
	})

	// Step 5: Generate compliance report.
	generateReport := pipz.Apply(ProcessorComplianceReport, func(_ context.Context, check ComplianceCheck) (ComplianceCheck, error) {
		// Summary of all compliance areas.
		fmt.Printf("   📋 Compliance Summary for %s:\n", check.Driver.Name)

		// Calculate overall compliance score.
		score := 100.0
		deductions := []string{}

		// HOS deductions.
		if check.DrivingHours > 11 {
			score -= 20
			deductions = append(deductions, "HOS violation (-20)")
		}

		// License deductions.
		if time.Until(check.DriverLicense.Expiry) < 0 {
			score -= 30
			deductions = append(deductions, "Expired license (-30)")
		}

		// Violation deductions.
		if len(check.RecentViolations) > 0 {
			score -= float64(len(check.RecentViolations)) * 5
			deductions = append(deductions,
				fmt.Sprintf("%d violations (-%d)", len(check.RecentViolations), len(check.RecentViolations)*5))
		}

		if score < 0 {
			score = 0
		}

		fmt.Printf("   📊 Compliance Score: %.0f/100\n", score)
		if len(deductions) > 0 {
			fmt.Printf("   Deductions: %v\n", deductions)
		}

		return check, nil
	})

	// Register all steps.
	compliancePipeline.Register(
		checkHOS,
		verifyLicenses,
		checkPermits,
		reviewViolations,
		generateReport,
	)
}

// ProcessComplianceCheck is the public API for the compliance domain.
func ProcessComplianceCheck(ctx context.Context, check ComplianceCheck) (ComplianceCheck, *pipz.Error[ComplianceCheck]) {
	// Process through internal pipeline.
	return compliancePipeline.Process(ctx, check)
}

// Processor names.
const (
	PipelineComplianceCheck   = "compliance_check"
	ProcessorHOSCalculation   = "hos_calculation"
	ProcessorLicenseVerify    = "license_verify"
	ProcessorPermitCheck      = "permit_check"
	ProcessorViolationReview  = "violation_review"
	ProcessorComplianceReport = "compliance_report"
)
