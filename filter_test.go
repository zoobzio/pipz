package pipz

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/zoobzio/capitan"
)

func TestFilter_NewFilter(t *testing.T) {
	condition := func(_ context.Context, data int) bool { return data > 5 }
	processor := Transform(NewIdentity("double", ""), func(_ context.Context, data int) int { return data * 2 })

	filter := NewFilter(NewIdentity("test-filter", "test filter"), condition, processor)

	if filter.Identity().Name() != "test-filter" {
		t.Errorf("Expected name 'test-filter', got %s", filter.Identity().Name())
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
	processor := Transform(NewIdentity("double", ""), func(_ context.Context, data int) int { return data * 2 })
	filter := NewFilter(NewIdentity("test-filter", "test filter"), condition, processor)

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
	processor := Transform(NewIdentity("double", ""), func(_ context.Context, data int) int { return data * 2 })
	filter := NewFilter(NewIdentity("test-filter", "test filter"), condition, processor)

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
	processor := Apply(NewIdentity("fail", ""), func(_ context.Context, _ string) (string, error) {
		return "", errors.New("processing failed")
	})
	filter := NewFilter(NewIdentity("test-filter", "test filter"), condition, processor)

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

		if pipeErr.Path[0].Name() != "test-filter" {
			t.Errorf("Expected first path element 'test-filter', got %s", pipeErr.Path[0])
		}

		if pipeErr.Path[1].Name() != "fail" {
			t.Errorf("Expected second path element 'fail', got %s", pipeErr.Path[1])
		}
	} else {
		t.Error("Expected error to be of type *pipz.Error[string]")
	}
}

func TestFilter_SetCondition(t *testing.T) {
	filter := NewFilter(NewIdentity("test", ""), func(_ context.Context, data int) bool { return data > 5 },
		Transform(NewIdentity("noop", ""), func(_ context.Context, data int) int { return data }))

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
	filter := NewFilter(NewIdentity("test", ""), func(_ context.Context, data int) bool { return data > 5 },
		Transform(NewIdentity("old", ""), func(_ context.Context, data int) int { return data * 2 }))

	newProcessor := Transform(NewIdentity("new", ""), func(_ context.Context, data int) int { return data * 3 })
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
	filter := NewFilter(NewIdentity("concurrent-test", "concurrent test filter"),
		func(_ context.Context, data int) bool { return data > 0 },
		Transform(NewIdentity("increment", ""), func(_ context.Context, data int) int { return data + 1 }))

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
		filter.SetProcessor(Transform(NewIdentity("double", ""), func(_ context.Context, data int) int { return data * 2 }))
	}()

	// Wait for all goroutines
	for i := 0; i < 12; i++ {
		<-done
	}
}

func TestFilter_WithTimeout(t *testing.T) {
	condition := func(_ context.Context, data int) bool { return data > 0 }
	processor := Apply(NewIdentity("slow", ""), func(ctx context.Context, data int) (int, error) {
		select {
		case <-time.After(100 * time.Millisecond):
			return data * 2, nil
		case <-ctx.Done():
			return data, ctx.Err()
		}
	})
	filter := NewFilter(NewIdentity("timeout-test", "timeout test filter"), condition, processor)

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
	processor := Apply(NewIdentity("cancelable", ""), func(ctx context.Context, data int) (int, error) {
		select {
		case <-time.After(100 * time.Millisecond):
			return data * 2, nil
		case <-ctx.Done():
			return data, ctx.Err()
		}
	})
	filter := NewFilter(NewIdentity("cancel-test", "cancel test filter"), condition, processor)

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

	betaProcessor := Transform(NewIdentity("beta-feature", ""), func(_ context.Context, user User) User {
		user.Data = "BETA:" + user.Data
		return user
	})

	featureFlag := NewFilter(NewIdentity("feature-flag", "feature flag filter"),
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

	premiumValidation := Apply(NewIdentity("premium-validation", ""), func(_ context.Context, order Order) (Order, error) {
		if order.Amount > 10000 {
			return order, errors.New("amount exceeds premium limit")
		}
		order.Validated = true
		return order, nil
	})

	validatePremium := NewFilter(NewIdentity("premium-filter", "premium filter"),
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

func TestFilter_ChainableComposition(t *testing.T) {
	// Test that Filter can be used in sequences and other connectors
	doubler := Transform(NewIdentity("double", ""), func(_ context.Context, data int) int { return data * 2 })
	filter := NewFilter(NewIdentity("even-only", "even only filter"),
		func(_ context.Context, data int) bool { return data%2 == 0 },
		doubler)

	adder := Transform(NewIdentity("add-ten", ""), func(_ context.Context, data int) int { return data + 10 })

	sequence := NewSequence(NewIdentity("test-sequence", "test sequence"), filter, adder)

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
		processor := Transform(NewIdentity("processor", ""), func(_ context.Context, data int) int { return data * 2 })

		filter := NewFilter(NewIdentity("panic_filter", "panic filter"), panicCondition, processor)
		result, err := filter.Process(context.Background(), 42)

		if result != 0 {
			t.Errorf("expected zero value 0, got %d", result)
		}

		var pipzErr *Error[int]
		if !errors.As(err, &pipzErr) {
			t.Fatal("expected pipz.Error")
		}

		if pipzErr.Path[0].Name() != "panic_filter" {
			t.Errorf("expected path to start with 'panic_filter', got %v", pipzErr.Path)
		}

		if pipzErr.InputData != 42 {
			t.Errorf("expected input data 42, got %d", pipzErr.InputData)
		}
	})

	t.Run("Filter processor panic recovery", func(t *testing.T) {
		condition := func(_ context.Context, _ int) bool { return true }
		panicProcessor := Transform(NewIdentity("panic_processor", ""), func(_ context.Context, _ int) int {
			panic("filter processor panic")
		})

		filter := NewFilter(NewIdentity("panic_filter", "panic filter"), condition, panicProcessor)
		result, err := filter.Process(context.Background(), 42)

		if result != 0 {
			t.Errorf("expected zero value 0, got %d", result)
		}

		var pipzErr *Error[int]
		if !errors.As(err, &pipzErr) {
			t.Fatal("expected pipz.Error")
		}

		if pipzErr.Path[0].Name() != "panic_filter" {
			t.Errorf("expected path to start with 'panic_filter', got %v", pipzErr.Path)
		}

		if pipzErr.InputData != 42 {
			t.Errorf("expected input data 42, got %d", pipzErr.InputData)
		}
	})
}

func TestFilterClose(t *testing.T) {
	t.Run("Closes Child Processor", func(t *testing.T) {
		p := newTrackingProcessor[int](NewIdentity("p", ""))

		f := NewFilter(NewIdentity("test", ""), func(_ context.Context, _ int) bool { return true }, p)
		err := f.Close()

		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if p.CloseCalls() != 1 {
			t.Errorf("expected 1 close call, got %d", p.CloseCalls())
		}
	})

	t.Run("Propagates Close Error", func(t *testing.T) {
		p := newTrackingProcessor[int](NewIdentity("p", "")).WithCloseError(errors.New("close error"))

		f := NewFilter(NewIdentity("test", ""), func(_ context.Context, _ int) bool { return true }, p)
		err := f.Close()

		if err == nil {
			t.Error("expected error")
		}
		if p.CloseCalls() != 1 {
			t.Errorf("expected 1 close call, got %d", p.CloseCalls())
		}
	})

	t.Run("Idempotency", func(t *testing.T) {
		p := newTrackingProcessor[int](NewIdentity("p", ""))
		f := NewFilter(NewIdentity("test", ""), func(_ context.Context, _ int) bool { return true }, p)

		_ = f.Close()
		_ = f.Close()

		if p.CloseCalls() != 1 {
			t.Errorf("expected 1 close call, got %d", p.CloseCalls())
		}
	})
}

func TestFilterSignals(t *testing.T) {
	t.Run("Emits Evaluated Signal When Passed", func(t *testing.T) {
		var signalReceived bool
		var signalName string
		var signalPassed bool

		listener := capitan.Hook(SignalFilterEvaluated, func(_ context.Context, e *capitan.Event) {
			signalReceived = true
			signalName, _ = FieldName.From(e)
			signalPassed, _ = FieldPassed.From(e)
		})
		defer listener.Close()

		filter := NewFilter(NewIdentity("signal-test-filter", "signal test filter"),
			func(_ context.Context, n int) bool { return n > 5 },
			Transform(NewIdentity("double", ""), func(_ context.Context, n int) int { return n * 2 }),
		)

		_, err := filter.Process(context.Background(), 10)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if err := listener.Drain(context.Background()); err != nil {
			t.Fatalf("drain failed: %v", err)
		}

		if !signalReceived {
			t.Error("expected signal to be received")
		}
		if signalName != "signal-test-filter" {
			t.Errorf("expected name 'signal-test-filter', got %q", signalName)
		}
		if !signalPassed {
			t.Error("expected passed to be true")
		}
	})

	t.Run("Emits Evaluated Signal When Not Passed", func(t *testing.T) {
		var signalReceived bool
		var signalPassed bool

		listener := capitan.Hook(SignalFilterEvaluated, func(_ context.Context, e *capitan.Event) {
			signalReceived = true
			signalPassed, _ = FieldPassed.From(e)
		})
		defer listener.Close()

		filter := NewFilter(NewIdentity("signal-skip-filter", "signal skip filter"),
			func(_ context.Context, n int) bool { return n > 100 },
			Transform(NewIdentity("double", ""), func(_ context.Context, n int) int { return n * 2 }),
		)

		_, err := filter.Process(context.Background(), 5)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if err := listener.Drain(context.Background()); err != nil {
			t.Fatalf("drain failed: %v", err)
		}

		if !signalReceived {
			t.Error("expected signal to be received")
		}
		if signalPassed {
			t.Error("expected passed to be false")
		}
	})

	t.Run("Schema", func(t *testing.T) {
		proc := Transform(NewIdentity("inner-proc", ""), func(_ context.Context, n int) int { return n })
		condition := func(_ context.Context, n int) bool { return n > 0 }

		filter := NewFilter(NewIdentity("test-filter", "Filter connector"), condition, proc)

		schema := filter.Schema()

		if schema.Identity.Name() != "test-filter" {
			t.Errorf("Schema Identity.Name() = %v, want %v", schema.Identity.Name(), "test-filter")
		}
		if schema.Type != "filter" {
			t.Errorf("Schema Type = %v, want %v", schema.Type, "filter")
		}

		flow, ok := FilterKey.From(schema)
		if !ok {
			t.Fatal("Expected FilterFlow")
		}
		if flow.Processor.Identity.Name() != "inner-proc" {
			t.Errorf("Flow.Processor.Identity.Name() = %v, want %v", flow.Processor.Identity.Name(), "inner-proc")
		}
	})
}
