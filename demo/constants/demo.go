// Package constants defines common values used across all demos.
package constants

import "time"

// Timing constants for demo animations and simulated operations.
const (
	// DemoSleepShort is used for brief pauses between demo sections
	DemoSleepShort = 1 * time.Second
	
	// DemoSleepMedium is used for longer pauses to let users read
	DemoSleepMedium = 2 * time.Second
	
	// SimulatedFastOperation represents a quick operation (e.g., validation)
	SimulatedFastOperation = 10 * time.Millisecond
	
	// SimulatedMediumOperation represents a moderate operation (e.g., API call)
	SimulatedMediumOperation = 20 * time.Millisecond
	
	// SimulatedSlowOperation represents a slow operation (e.g., external service)
	SimulatedSlowOperation = 50 * time.Millisecond
)

// Business logic limits and thresholds.
const (
	// StandardTransactionLimit is the maximum transaction amount for standard merchants
	StandardTransactionLimit = 10000
	
	// PremiumTransactionLimit is the maximum transaction amount for premium merchants
	PremiumTransactionLimit = 100000
	
	// StandardVelocityLimit is transactions per hour for standard merchants
	StandardVelocityLimit = 10
	
	// PremiumVelocityLimit is transactions per hour for premium merchants
	PremiumVelocityLimit = 100
	
	// MinimumAge is the minimum age for user registration
	MinimumAge = 18
	
	// MaximumAge is the maximum age for validation
	MaximumAge = 150
)

// ANSI escape codes for terminal control.
const (
	// ClearScreen clears the terminal screen
	ClearScreen = "\033[H\033[2J"
	
	// MoveCursorUp moves cursor up N lines (use with fmt.Sprintf)
	MoveCursorUp = "\033[%dA"
	
	// ClearLine clears the current line
	ClearLine = "\033[2K"
)