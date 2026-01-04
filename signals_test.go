package pipz

import "testing"

// TestSignalsInitialized verifies all signals are properly initialized.
// This file tests declaration-only code in signals.go.
func TestSignalsInitialized(t *testing.T) {
	signals := []struct {
		name   string
		signal any
	}{
		{"CircuitBreakerOpened", SignalCircuitBreakerOpened},
		{"CircuitBreakerClosed", SignalCircuitBreakerClosed},
		{"CircuitBreakerHalfOpen", SignalCircuitBreakerHalfOpen},
		{"CircuitBreakerRejected", SignalCircuitBreakerRejected},
		{"RateLimiterThrottled", SignalRateLimiterThrottled},
		{"RateLimiterDropped", SignalRateLimiterDropped},
		{"RateLimiterAllowed", SignalRateLimiterAllowed},
		{"WorkerPoolSaturated", SignalWorkerPoolSaturated},
		{"WorkerPoolAcquired", SignalWorkerPoolAcquired},
		{"WorkerPoolReleased", SignalWorkerPoolReleased},
		{"RetryAttemptStart", SignalRetryAttemptStart},
		{"RetryAttemptFail", SignalRetryAttemptFail},
		{"RetryExhausted", SignalRetryExhausted},
		{"FallbackAttempt", SignalFallbackAttempt},
		{"FallbackFailed", SignalFallbackFailed},
		{"TimeoutTriggered", SignalTimeoutTriggered},
		{"BackoffWaiting", SignalBackoffWaiting},
		{"SequenceCompleted", SignalSequenceCompleted},
		{"ConcurrentCompleted", SignalConcurrentCompleted},
		{"RaceWinner", SignalRaceWinner},
		{"ContestWinner", SignalContestWinner},
		{"ScaffoldDispatched", SignalScaffoldDispatched},
		{"SwitchRouted", SignalSwitchRouted},
		{"FilterEvaluated", SignalFilterEvaluated},
		{"HandleErrorHandled", SignalHandleErrorHandled},
	}

	for _, s := range signals {
		if s.signal == nil {
			t.Errorf("Signal %s is nil", s.name)
		}
	}
}

// TestFieldKeysInitialized verifies all field keys are properly initialized.
func TestFieldKeysInitialized(t *testing.T) {
	fields := []struct {
		name string
		key  any
	}{
		{"IdentityID", FieldIdentityID},
		{"Name", FieldName},
		{"Error", FieldError},
		{"Timestamp", FieldTimestamp},
		{"State", FieldState},
		{"Failures", FieldFailures},
		{"Successes", FieldSuccesses},
		{"FailureThreshold", FieldFailureThreshold},
		{"SuccessThreshold", FieldSuccessThreshold},
		{"ResetTimeout", FieldResetTimeout},
		{"Generation", FieldGeneration},
		{"LastFailTime", FieldLastFailTime},
		{"Rate", FieldRate},
		{"Burst", FieldBurst},
		{"Tokens", FieldTokens},
		{"Mode", FieldMode},
		{"WaitTime", FieldWaitTime},
		{"WorkerCount", FieldWorkerCount},
		{"ActiveWorkers", FieldActiveWorkers},
		{"Attempt", FieldAttempt},
		{"MaxAttempts", FieldMaxAttempts},
		{"ProcessorIndex", FieldProcessorIndex},
		{"ProcessorName", FieldProcessorName},
		{"Duration", FieldDuration},
		{"Delay", FieldDelay},
		{"NextDelay", FieldNextDelay},
		{"ProcessorCount", FieldProcessorCount},
		{"ErrorCount", FieldErrorCount},
		{"WinnerName", FieldWinnerName},
		{"RouteKey", FieldRouteKey},
		{"Matched", FieldMatched},
		{"Passed", FieldPassed},
	}

	for _, f := range fields {
		if f.key == nil {
			t.Errorf("Field key %s is nil", f.name)
		}
	}
}
