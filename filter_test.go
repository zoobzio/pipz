package pipz

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/zoobzio/tracez"
)

func TestFilter_NewFilter(t *testing.T) {
	condition := func(_ context.Context, data int) bool { return data > 5 }
	processor := Transform("double", func(_ context.Context, data int) int { return data * 2 })

	filter := NewFilter("test-filter", condition, processor)

	if filter.Name() != "test-filter" {
		t.Errorf("Expected name 'test-filter', got %s", filter.Name())
	}

	if filter.Condition() == nil {
		t.Error("Expected condition to be set")
	}

	if filter.Processor() == nil {
		t.Error("Expected processor to be set")
	}
}

func TestFilter_Process_ConditionTrue(t *testing.T) {
	condition := func(_ context.Context, data int) bool { return data > 5 }
	processor := Transform("double", func(_ context.Context, data int) int { return data * 2 })
	filter := NewFilter("test-filter", condition, processor)

	result, err := filter.Process(context.Background(), 10)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if result != 20 {
		t.Errorf("Expected result 20, got %d", result)
	}
}

func TestFilter_Process_ConditionFalse(t *testing.T) {
	condition := func(_ context.Context, data int) bool { return data > 5 }
	processor := Transform("double", func(_ context.Context, data int) int { return data * 2 })
	filter := NewFilter("test-filter", condition, processor)

	result, err := filter.Process(context.Background(), 3)

	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if result != 3 {
		t.Errorf("Expected result 3 (unchanged), got %d", result)
	}
}

func TestFilter_Process_ProcessorError(t *testing.T) {
	condition := func(_ context.Context, data string) bool { return len(data) > 3 }
	processor := Apply("fail", func(_ context.Context, _ string) (string, error) {
		return "", errors.New("processing failed")
	})
	filter := NewFilter("test-filter", condition, processor)

	result, err := filter.Process(context.Background(), "test")

	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	if result != "" {
		t.Errorf("Expected zero value on error, got %s", result)
	}

	var pipeErr *Error[string]
	if errors.As(err, &pipeErr) {
		if len(pipeErr.Path) != 2 {
			t.Errorf("Expected error path length 2, got %d", len(pipeErr.Path))
		}

		if pipeErr.Path[0] != "test-filter" {
			t.Errorf("Expected first path element 'test-filter', got %s", pipeErr.Path[0])
		}

		if pipeErr.Path[1] != "fail" {
			t.Errorf("Expected second path element 'fail', got %s", pipeErr.Path[1])
		}
	} else {
		t.Error("Expected error to be of type *pipz.Error[string]")
	}
}

func TestFilter_SetCondition(t *testing.T) {
	filter := NewFilter("test", func(_ context.Context, data int) bool { return data > 5 },
		Transform("noop", func(_ context.Context, data int) int { return data }))

	newCondition := func(_ context.Context, data int) bool { return data < 5 }
	filter.SetCondition(newCondition)

	// Test that new condition is applied
	result, err := filter.Process(context.Background(), 3)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if result != 3 {
		t.Errorf("Expected processing to occur with new condition, got %d", result)
	}

	result, err = filter.Process(context.Background(), 7)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if result != 7 {
		t.Errorf("Expected no processing with new condition, got %d", result)
	}
}

func TestFilter_SetProcessor(t *testing.T) {
	filter := NewFilter("test", func(_ context.Context, data int) bool { return data > 5 },
		Transform("old", func(_ context.Context, data int) int { return data * 2 }))

	newProcessor := Transform("new", func(_ context.Context, data int) int { return data * 3 })
	filter.SetProcessor(newProcessor)

	result, err := filter.Process(context.Background(), 10)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
	if result != 30 {
		t.Errorf("Expected new processor result 30, got %d", result)
	}
}

func TestFilter_ConcurrentAccess(t *testing.T) {
	filter := NewFilter("concurrent-test",
		func(_ context.Context, data int) bool { return data > 0 },
		Transform("increment", func(_ context.Context, data int) int { return data + 1 }))

	done := make(chan bool)

	// Start multiple goroutines processing
	for i := 0; i < 10; i++ {
		go func(val int) {
			defer func() { done <- true }()

			result, err := filter.Process(context.Background(), val)
			if err != nil {
				t.Errorf("Goroutine %d: unexpected error %v", val, err)
				return
			}

			// Since condition and processor may change during execution,
			// just verify that we get a reasonable result (either original or processed)
			if result != val && result != val+1 && result != val*2 {
				t.Errorf("Goroutine %d: unexpected result %d for input %d", val, result, val)
			}
		}(i + 1)
	}

	// Start goroutines updating the filter
	go func() {
		defer func() { done <- true }()
		filter.SetCondition(func(_ context.Context, data int) bool { return data > 5 })
	}()

	go func() {
		defer func() { done <- true }()
		filter.SetProcessor(Transform("double", func(_ context.Context, data int) int { return data * 2 }))
	}()

	// Wait for all goroutines
	for i := 0; i < 12; i++ {
		<-done
	}
}

func TestFilter_WithTimeout(t *testing.T) {
	condition := func(_ context.Context, data int) bool { return data > 0 }
	processor := Apply("slow", func(ctx context.Context, data int) (int, error) {
		select {
		case <-time.After(100 * time.Millisecond):
			return data * 2, nil
		case <-ctx.Done():
			return data, ctx.Err()
		}
	})
	filter := NewFilter("timeout-test", condition, processor)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	result, err := filter.Process(ctx, 5)

	if err == nil {
		t.Fatal("Expected timeout error, got nil")
	}

	if result != 0 {
		t.Errorf("Expected zero value on timeout, got %d", result)
	}

	var pipeErr *Error[int]
	if errors.As(err, &pipeErr) {
		if !pipeErr.Timeout {
			t.Error("Expected timeout flag to be true")
		}
	} else {
		t.Error("Expected error to be of type *pipz.Error[int]")
	}
}

func TestFilter_WithCancellation(t *testing.T) {
	condition := func(_ context.Context, data int) bool { return data > 0 }
	processor := Apply("cancelable", func(ctx context.Context, data int) (int, error) {
		select {
		case <-time.After(100 * time.Millisecond):
			return data * 2, nil
		case <-ctx.Done():
			return data, ctx.Err()
		}
	})
	filter := NewFilter("cancel-test", condition, processor)

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	result, err := filter.Process(ctx, 5)

	if err == nil {
		t.Fatal("Expected cancellation error, got nil")
	}

	if result != 0 {
		t.Errorf("Expected zero value on cancellation, got %d", result)
	}

	var pipeErr *Error[int]
	if errors.As(err, &pipeErr) {
		if !pipeErr.Canceled {
			t.Error("Expected canceled flag to be true")
		}
	} else {
		t.Error("Expected error to be of type *pipz.Error[int]")
	}
}

func TestFilter_FeatureFlagExample(t *testing.T) {
	type User struct {
		ID          string
		Data        string
		BetaEnabled bool
	}

	betaProcessor := Transform("beta-feature", func(_ context.Context, user User) User {
		user.Data = "BETA:" + user.Data
		return user
	})

	featureFlag := NewFilter("feature-flag",
		func(_ context.Context, user User) bool {
			return user.BetaEnabled
		},
		betaProcessor,
	)

	// Test beta user
	betaUser := User{ID: "1", BetaEnabled: true, Data: "content"}
	result, err := featureFlag.Process(context.Background(), betaUser)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if result.Data != "BETA:content" {
		t.Errorf("Expected 'BETA:content', got %s", result.Data)
	}

	// Test regular user
	regularUser := User{ID: "2", BetaEnabled: false, Data: "content"}
	result, err = featureFlag.Process(context.Background(), regularUser)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if result.Data != "content" {
		t.Errorf("Expected 'content', got %s", result.Data)
	}
}

func TestFilter_ConditionalValidationExample(t *testing.T) {
	type Order struct {
		ID           string
		CustomerTier string
		Amount       float64
		Validated    bool
	}

	premiumValidation := Apply("premium-validation", func(_ context.Context, order Order) (Order, error) {
		if order.Amount > 10000 {
			return order, errors.New("amount exceeds premium limit")
		}
		order.Validated = true
		return order, nil
	})

	validatePremium := NewFilter("premium-filter",
		func(_ context.Context, order Order) bool {
			return order.CustomerTier == "premium"
		},
		premiumValidation,
	)

	// Test premium order
	premiumOrder := Order{ID: "1", CustomerTier: "premium", Amount: 5000}
	result, err := validatePremium.Process(context.Background(), premiumOrder)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !result.Validated {
		t.Error("Expected premium order to be validated")
	}

	// Test standard order
	standardOrder := Order{ID: "2", CustomerTier: "standard", Amount: 5000}
	result, err = validatePremium.Process(context.Background(), standardOrder)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if result.Validated {
		t.Error("Expected standard order to not be validated")
	}

	// Test premium order with validation error
	expensiveOrder := Order{ID: "3", CustomerTier: "premium", Amount: 15000}
	_, err = validatePremium.Process(context.Background(), expensiveOrder)
	if err == nil {
		t.Fatal("Expected validation error for expensive premium order")
	}
	var pipeErr *Error[Order]
	if errors.As(err, &pipeErr) {
		if pipeErr.Err.Error() != "amount exceeds premium limit" {
			t.Errorf("Expected specific error message, got %v", pipeErr.Err)
		}
	} else {
		t.Error("Expected error to be of type *pipz.Error[Order]")
	}
}

func TestFilter_Observability(t *testing.T) {
	t.Run("Metrics and Spans", func(t *testing.T) {
		condition := func(_ context.Context, data int) bool { return data > 5 }
		processor := Transform("double", func(_ context.Context, data int) int { return data * 2 })

		filter := NewFilter("test-filter", condition, processor)
		defer filter.Close()

		// Verify observability components are initialized
		if filter.Metrics() == nil {
			t.Error("expected metrics registry to be initialized")
		}
		if filter.Tracer() == nil {
			t.Error("expected tracer to be initialized")
		}

		// Capture spans using the callback API
		var spans []tracez.Span
		filter.Tracer().OnSpanComplete(func(span tracez.Span) {
			spans = append(spans, span)
		})

		// Process items that pass the filter
		result, err := filter.Process(context.Background(), 10)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result != 20 {
			t.Errorf("expected 20, got %d", result)
		}

		// Process items that don't pass the filter
		result, err = filter.Process(context.Background(), 3)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result != 3 {
			t.Errorf("expected 3, got %d", result)
		}

		// Process another item that passes
		result, err = filter.Process(context.Background(), 8)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result != 16 {
			t.Errorf("expected 16, got %d", result)
		}

		// Verify metrics
		processedTotal := filter.Metrics().Counter(FilterProcessedTotal).Value()
		if processedTotal != 3 {
			t.Errorf("expected 3 processed items, got %f", processedTotal)
		}

		passedTotal := filter.Metrics().Counter(FilterPassedTotal).Value()
		if passedTotal != 2 {
			t.Errorf("expected 2 passed items, got %f", passedTotal)
		}

		skippedTotal := filter.Metrics().Counter(FilterSkippedTotal).Value()
		if skippedTotal != 1 {
			t.Errorf("expected 1 skipped item, got %f", skippedTotal)
		}

		// Verify spans were captured
		if len(spans) != 3 {
			t.Errorf("expected 3 spans, got %d", len(spans))
		}

		// Check span tags
		for i, span := range spans {
			if span.Name != FilterProcessSpan {
				t.Errorf("span %d: expected name %s, got %s", i, FilterProcessSpan, span.Name)
			}

			if _, ok := span.Tags[FilterTagConditionMet]; !ok {
				t.Errorf("span %d: missing condition_met tag", i)
			}

			if _, ok := span.Tags[FilterTagSuccess]; !ok {
				t.Errorf("span %d: missing success tag", i)
			}
		}
	})

	t.Run("Hooks fire on filter events", func(t *testing.T) {
		condition := func(_ context.Context, data int) bool { return data > 5 }
		processor := Transform("double", func(_ context.Context, data int) int { return data * 2 })

		filter := NewFilter("test-filter", condition, processor)
		defer filter.Close()

		var passedEvents []FilterEvent
		var skippedEvents []FilterEvent
		var mu sync.Mutex

		// Register hooks
		filter.OnPassed(func(_ context.Context, event FilterEvent) error {
			mu.Lock()
			passedEvents = append(passedEvents, event)
			mu.Unlock()
			return nil
		})

		filter.OnSkipped(func(_ context.Context, event FilterEvent) error {
			mu.Lock()
			skippedEvents = append(skippedEvents, event)
			mu.Unlock()
			return nil
		})

		// Test condition true (passed)
		result, err := filter.Process(context.Background(), 10)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if result != 20 {
			t.Errorf("Expected result 20, got %d", result)
		}

		// Test condition false (skipped)
		result, err = filter.Process(context.Background(), 3)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if result != 3 {
			t.Errorf("Expected result 3 (unchanged), got %d", result)
		}

		// Wait for async hooks
		time.Sleep(50 * time.Millisecond)

		mu.Lock()
		passedCount := len(passedEvents)
		skippedCount := len(skippedEvents)

		// Check passed event
		if passedCount != 1 {
			t.Errorf("Expected 1 passed event, got %d", passedCount)
		}

		if passedCount > 0 {
			event := passedEvents[0]
			if !event.ConditionMet {
				t.Error("Expected condition met for passed event")
			}
			if !event.Passed {
				t.Error("Expected passed=true for passed event")
			}
			if event.Skipped {
				t.Error("Expected skipped=false for passed event")
			}
			if !event.Success {
				t.Error("Expected success=true for passed event")
			}
			if event.ProcessorName != "double" {
				t.Errorf("Expected processor name 'double', got %s", event.ProcessorName)
			}
			if event.Duration <= 0 {
				t.Error("Expected positive duration for passed event")
			}
		}

		// Check skipped event
		if skippedCount != 1 {
			t.Errorf("Expected 1 skipped event, got %d", skippedCount)
		}

		if skippedCount > 0 {
			event := skippedEvents[0]
			if event.ConditionMet {
				t.Error("Expected condition not met for skipped event")
			}
			if event.Passed {
				t.Error("Expected passed=false for skipped event")
			}
			if !event.Skipped {
				t.Error("Expected skipped=true for skipped event")
			}
		}
		mu.Unlock()
	})

	t.Run("Hook fires on processor failure", func(t *testing.T) {
		condition := func(_ context.Context, data int) bool { return data > 5 }
		processor := Apply("failing", func(_ context.Context, _ int) (int, error) {
			return 0, errors.New("processor error")
		})

		filter := NewFilter("test-filter", condition, processor)
		defer filter.Close()

		var passedEvents []FilterEvent
		var mu sync.Mutex

		filter.OnPassed(func(_ context.Context, event FilterEvent) error {
			mu.Lock()
			passedEvents = append(passedEvents, event)
			mu.Unlock()
			return nil
		})

		// Process with condition true, but processor fails
		_, err := filter.Process(context.Background(), 10)
		if err == nil {
			t.Error("Expected error from processor")
		}

		// Wait for async hook
		time.Sleep(50 * time.Millisecond)

		mu.Lock()
		if len(passedEvents) != 1 {
			t.Errorf("Expected 1 passed event, got %d", len(passedEvents))
		}

		if len(passedEvents) > 0 {
			event := passedEvents[0]
			if event.Success {
				t.Error("Expected success=false for failed processor")
			}
			if event.Error == nil {
				t.Error("Expected error to be set")
			}
			if event.Duration <= 0 {
				t.Error("Expected positive duration even for failed processor")
			}
		}
		mu.Unlock()
	})
}

func TestFilter_ChainableComposition(t *testing.T) {
	// Test that Filter can be used in sequences and other connectors
	doubler := Transform("double", func(_ context.Context, data int) int { return data * 2 })
	filter := NewFilter("even-only",
		func(_ context.Context, data int) bool { return data%2 == 0 },
		doubler)

	adder := Transform("add-ten", func(_ context.Context, data int) int { return data + 10 })

	sequence := NewSequence("test-sequence", filter, adder)

	// Test even number (filter applies)
	result, err := sequence.Process(context.Background(), 4)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	expected := 18 // 4 * 2 + 10
	if result != expected {
		t.Errorf("Expected %d, got %d", expected, result)
	}

	// Test odd number (filter skips)
	result, err = sequence.Process(context.Background(), 5)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if result != 15 { // 5 + 10 = 15
		t.Errorf("Expected 15, got %d", result)
	}

	t.Run("Filter condition panic recovery", func(t *testing.T) {
		panicCondition := func(_ context.Context, _ int) bool {
			panic("filter condition panic")
		}
		processor := Transform("processor", func(_ context.Context, data int) int { return data * 2 })

		filter := NewFilter("panic_filter", panicCondition, processor)
		result, err := filter.Process(context.Background(), 42)

		if result != 0 {
			t.Errorf("expected zero value 0, got %d", result)
		}

		var pipzErr *Error[int]
		if !errors.As(err, &pipzErr) {
			t.Fatal("expected pipz.Error")
		}

		if pipzErr.Path[0] != "panic_filter" {
			t.Errorf("expected path to start with 'panic_filter', got %v", pipzErr.Path)
		}

		if pipzErr.InputData != 42 {
			t.Errorf("expected input data 42, got %d", pipzErr.InputData)
		}
	})

	t.Run("Filter processor panic recovery", func(t *testing.T) {
		condition := func(_ context.Context, _ int) bool { return true }
		panicProcessor := Transform("panic_processor", func(_ context.Context, _ int) int {
			panic("filter processor panic")
		})

		filter := NewFilter("panic_filter", condition, panicProcessor)
		result, err := filter.Process(context.Background(), 42)

		if result != 0 {
			t.Errorf("expected zero value 0, got %d", result)
		}

		var pipzErr *Error[int]
		if !errors.As(err, &pipzErr) {
			t.Fatal("expected pipz.Error")
		}

		if pipzErr.Path[0] != "panic_filter" {
			t.Errorf("expected path to start with 'panic_filter', got %v", pipzErr.Path)
		}

		if pipzErr.InputData != 42 {
			t.Errorf("expected input data 42, got %d", pipzErr.InputData)
		}
	})
}
