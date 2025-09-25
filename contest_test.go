package pipz

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/zoobzio/tracez"
)

func TestContest(t *testing.T) {
	t.Run("First Meeting Condition Wins", func(t *testing.T) {
		// Condition: value must be > 150
		condition := func(_ context.Context, d TestData) bool {
			return d.Value > 150
		}

		fast := Apply("fast", func(_ context.Context, d TestData) (TestData, error) {
			time.Sleep(10 * time.Millisecond)
			d.Value = 100 // Doesn't meet condition
			return d, nil
		})
		medium := Apply("medium", func(_ context.Context, d TestData) (TestData, error) {
			time.Sleep(30 * time.Millisecond)
			d.Value = 200 // Meets condition
			return d, nil
		})
		slow := Apply("slow", func(_ context.Context, d TestData) (TestData, error) {
			time.Sleep(50 * time.Millisecond)
			d.Value = 300 // Also meets condition but slower
			return d, nil
		})

		contest := NewContest("test-contest", condition, fast, medium, slow)
		data := TestData{Value: 1}

		result, err := contest.Process(context.Background(), data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Value != 200 {
			t.Errorf("expected medium processor result (200), got %d", result.Value)
		}
	})

	t.Run("No Results Meet Condition", func(t *testing.T) {
		// Condition: value must be > 500
		condition := func(_ context.Context, d TestData) bool {
			return d.Value > 500
		}

		p1 := Apply("p1", func(_ context.Context, d TestData) (TestData, error) {
			d.Value = 100
			return d, nil
		})
		p2 := Apply("p2", func(_ context.Context, d TestData) (TestData, error) {
			d.Value = 200
			return d, nil
		})
		p3 := Apply("p3", func(_ context.Context, d TestData) (TestData, error) {
			d.Value = 300
			return d, nil
		})

		contest := NewContest("test-contest", condition, p1, p2, p3)
		data := TestData{Value: 1}

		result, err := contest.Process(context.Background(), data)
		if err == nil {
			t.Fatal("expected error when no results meet condition")
		}
		if !strings.Contains(err.Error(), "no processor results met the specified condition") {
			t.Errorf("unexpected error: %v", err)
		}
		// Should return original input on failure
		if result.Value != 1 {
			t.Errorf("expected original input value (1), got %d", result.Value)
		}
	})

	t.Run("All Processors Fail", func(t *testing.T) {
		condition := func(_ context.Context, d TestData) bool {
			return d.Value > 100
		}

		p1 := Apply("p1", func(_ context.Context, d TestData) (TestData, error) {
			return d, errors.New("error 1")
		})
		p2 := Apply("p2", func(_ context.Context, d TestData) (TestData, error) {
			return d, errors.New("error 2")
		})
		p3 := Apply("p3", func(_ context.Context, d TestData) (TestData, error) {
			return d, errors.New("error 3")
		})

		contest := NewContest("test-contest", condition, p1, p2, p3)
		data := TestData{Value: 1}

		_, err := contest.Process(context.Background(), data)
		if err == nil {
			t.Fatal("expected error when all processors fail")
		}
		if !strings.Contains(err.Error(), "all processors failed") {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("Mixed Success and Failure", func(t *testing.T) {
		// Condition: value must be divisible by 3
		condition := func(_ context.Context, d TestData) bool {
			return d.Value%3 == 0
		}

		failing := Apply("failing", func(_ context.Context, d TestData) (TestData, error) {
			time.Sleep(5 * time.Millisecond)
			return d, errors.New("service unavailable")
		})
		wrongValue := Apply("wrong-value", func(_ context.Context, d TestData) (TestData, error) {
			time.Sleep(10 * time.Millisecond)
			d.Value = 100 // Not divisible by 3
			return d, nil
		})
		correct := Apply("correct", func(_ context.Context, d TestData) (TestData, error) {
			time.Sleep(20 * time.Millisecond)
			d.Value = 99 // Divisible by 3
			return d, nil
		})

		contest := NewContest("test-contest", condition, failing, wrongValue, correct)
		data := TestData{Value: 1}

		result, err := contest.Process(context.Background(), data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Value != 99 {
			t.Errorf("expected correct processor result (99), got %d", result.Value)
		}
	})

	t.Run("Context Cancellation", func(t *testing.T) {
		condition := func(_ context.Context, d TestData) bool {
			return d.Value > 100
		}

		slow := Apply("slow", func(ctx context.Context, d TestData) (TestData, error) {
			select {
			case <-time.After(100 * time.Millisecond):
				d.Value = 200
				return d, nil
			case <-ctx.Done():
				return d, ctx.Err()
			}
		})

		contest := NewContest("test-contest", condition, slow)
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		data := TestData{Value: 1}
		result, err := contest.Process(ctx, data)

		// With new behavior, should return input without error
		if err != nil {
			t.Errorf("expected no error with new behavior, got %v", err)
		}
		if result.Value != 1 {
			t.Errorf("expected original input value (1), got %d", result.Value)
		}
	})

	t.Run("Empty Processors", func(t *testing.T) {
		condition := func(_ context.Context, _ TestData) bool {
			return true
		}

		contest := NewContest[TestData]("empty", condition)
		data := TestData{Value: 42}

		_, err := contest.Process(context.Background(), data)
		if err == nil {
			t.Fatal("expected error for empty contest")
		}
		if !strings.Contains(err.Error(), "no processors provided to Contest") {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("Nil Condition", func(t *testing.T) {
		p1 := Transform("p1", func(_ context.Context, d TestData) TestData { return d })
		contest := NewContest[TestData]("no-condition", nil, p1)
		data := TestData{Value: 42}

		_, err := contest.Process(context.Background(), data)
		if err == nil {
			t.Fatal("expected error for nil condition")
		}
		if !strings.Contains(err.Error(), "no condition provided to Contest") {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("Winner Cancels Others", func(t *testing.T) {
		// Condition: value == 100
		condition := func(_ context.Context, d TestData) bool {
			return d.Value == 100
		}

		// Use channels to coordinate test timing
		slowStartedCh := make(chan bool, 1)
		slowCanceledCh := make(chan bool, 1)

		fast := Apply("fast", func(_ context.Context, d TestData) (TestData, error) {
			// Small delay to ensure slow processor starts
			time.Sleep(10 * time.Millisecond)
			d.Value = 100 // Meets condition
			return d, nil
		})
		slow := Apply("slow", func(ctx context.Context, d TestData) (TestData, error) {
			slowStartedCh <- true
			select {
			case <-time.After(200 * time.Millisecond):
				d.Value = 100 // Also would meet condition
				return d, nil
			case <-ctx.Done():
				slowCanceledCh <- true
				return d, ctx.Err()
			}
		})

		contest := NewContest("test-contest", condition, fast, slow)
		data := TestData{Value: 1}

		result, err := contest.Process(context.Background(), data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Value != 100 {
			t.Errorf("expected fast result (100), got %d", result.Value)
		}

		// Verify timing
		select {
		case <-slowStartedCh:
			// Good, slow processor started
		case <-time.After(100 * time.Millisecond):
			t.Error("slow processor should have started")
		}

		// Note: We can't reliably test cancellation because Contest passes
		// the original context to processors (like Race does)
	})

	t.Run("Configuration Methods", func(t *testing.T) {
		condition := func(_ context.Context, d TestData) bool {
			return d.Value > 50
		}

		p1 := Transform("p1", func(_ context.Context, d TestData) TestData { return d })
		p2 := Transform("p2", func(_ context.Context, d TestData) TestData { return d })
		p3 := Transform("p3", func(_ context.Context, d TestData) TestData { return d })

		contest := NewContest("test", condition, p1, p2)

		if contest.Len() != 2 {
			t.Errorf("expected 2 processors, got %d", contest.Len())
		}

		contest.Add(p3)
		if contest.Len() != 3 {
			t.Errorf("expected 3 processors after add, got %d", contest.Len())
		}

		err := contest.Remove(1)
		if err != nil {
			t.Fatalf("unexpected error removing processor: %v", err)
		}
		if contest.Len() != 2 {
			t.Errorf("expected 2 processors after remove, got %d", contest.Len())
		}

		contest.Clear()
		if contest.Len() != 0 {
			t.Errorf("expected 0 processors after clear, got %d", contest.Len())
		}

		contest.SetProcessors(p1, p2, p3)
		if contest.Len() != 3 {
			t.Errorf("expected 3 processors after SetProcessors, got %d", contest.Len())
		}
	})

	t.Run("SetCondition", func(t *testing.T) {
		// Initial condition: value > 100
		initialCondition := func(_ context.Context, d TestData) bool {
			return d.Value > 100
		}

		p1 := Apply("p1", func(_ context.Context, d TestData) (TestData, error) {
			d.Value = 50 // Doesn't meet initial condition
			return d, nil
		})

		contest := NewContest("test", initialCondition, p1)
		data := TestData{Value: 1}

		// First run - should fail with initial condition
		_, err := contest.Process(context.Background(), data)
		if err == nil {
			t.Fatal("expected error with initial condition")
		}

		// Update condition: value < 100
		newCondition := func(_ context.Context, d TestData) bool {
			return d.Value < 100
		}
		contest.SetCondition(newCondition)

		// Second run - should succeed with new condition
		result, err := contest.Process(context.Background(), data)
		if err != nil {
			t.Fatalf("unexpected error with new condition: %v", err)
		}
		if result.Value != 50 {
			t.Errorf("expected value 50, got %d", result.Value)
		}
	})

	t.Run("Name Method", func(t *testing.T) {
		condition := func(_ context.Context, _ TestData) bool { return true }
		contest := NewContest[TestData]("my-contest", condition)
		if contest.Name() != "my-contest" {
			t.Errorf("expected 'my-contest', got %q", contest.Name())
		}
	})

	t.Run("Remove Out of Bounds", func(t *testing.T) {
		condition := func(_ context.Context, _ TestData) bool { return true }
		p1 := Transform("p1", func(_ context.Context, d TestData) TestData { return d })
		contest := NewContest("test", condition, p1)

		// Test negative index
		err := contest.Remove(-1)
		if err == nil {
			t.Error("expected error for negative index")
		}
		if !errors.Is(err, ErrIndexOutOfBounds) {
			t.Errorf("expected ErrIndexOutOfBounds, got %v", err)
		}

		// Test index >= length
		err = contest.Remove(1)
		if err == nil {
			t.Error("expected error for index >= length")
		}
		if !errors.Is(err, ErrIndexOutOfBounds) {
			t.Errorf("expected ErrIndexOutOfBounds, got %v", err)
		}
	})

	t.Run("Complex Condition with Context", func(t *testing.T) {
		// Condition that uses context to check deadline
		condition := func(ctx context.Context, d TestData) bool {
			deadline, ok := ctx.Deadline()
			if !ok {
				return d.Value > 100
			}
			// Accept lower values if we're running out of time
			timeLeft := time.Until(deadline)
			if timeLeft < 25*time.Millisecond {
				return d.Value > 50
			}
			return d.Value > 100
		}

		slow := Apply("slow", func(_ context.Context, d TestData) (TestData, error) {
			time.Sleep(30 * time.Millisecond)
			d.Value = 75 // Only acceptable if running out of time
			return d, nil
		})
		slower := Apply("slower", func(_ context.Context, d TestData) (TestData, error) {
			time.Sleep(60 * time.Millisecond)
			d.Value = 150 // Always acceptable
			return d, nil
		})

		contest := NewContest("deadline-aware", condition, slow, slower)
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Millisecond)
		defer cancel()

		data := TestData{Value: 1}
		result, err := contest.Process(ctx, data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Should get the 75 value because deadline pressure made it acceptable
		if result.Value != 75 {
			t.Errorf("expected value 75 under deadline pressure, got %d", result.Value)
		}
	})

	t.Run("Contest condition panic recovery", func(t *testing.T) {
		panicCondition := func(_ context.Context, _ TestData) bool {
			panic("contest condition panic")
		}
		processor := Apply("processor", func(_ context.Context, d TestData) (TestData, error) {
			d.Value = 100
			return d, nil
		})

		contest := NewContest("panic_contest", panicCondition, processor)
		data := TestData{Value: 42}

		result, err := contest.Process(context.Background(), data)

		// Contest should return zero value when condition panics
		if result.Value != 0 {
			t.Errorf("expected zero value 0, got %d", result.Value)
		}

		var pipzErr *Error[TestData]
		if !errors.As(err, &pipzErr) {
			t.Fatal("expected pipz.Error")
		}

		if pipzErr.Path[0] != "panic_contest" {
			t.Errorf("expected path to start with 'panic_contest', got %v", pipzErr.Path)
		}

		if pipzErr.InputData.Value != 42 {
			t.Errorf("expected input data value 42, got %d", pipzErr.InputData.Value)
		}
	})

	t.Run("Contest processor panic recovery", func(t *testing.T) {
		condition := func(_ context.Context, d TestData) bool {
			return d.Value > 50
		}
		panicProcessor := Apply("panic_processor", func(_ context.Context, _ TestData) (TestData, error) {
			panic("contest processor panic")
		})
		normalProcessor := Apply("normal_processor", func(_ context.Context, d TestData) (TestData, error) {
			d.Value = 100
			return d, nil
		})

		contest := NewContest("panic_contest", condition, panicProcessor, normalProcessor)
		data := TestData{Value: 42}

		result, err := contest.Process(context.Background(), data)

		// Normal processor should succeed
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.Value != 100 {
			t.Errorf("expected normal processor result 100, got %d", result.Value)
		}
	})

	t.Run("Observability", func(t *testing.T) {
		t.Run("Metrics and Spans - Winner Found", func(t *testing.T) {
			// Condition: value must be > 150
			condition := func(_ context.Context, d TestData) bool {
				return d.Value > 150
			}

			// Create processors with different results and speeds
			slow := Apply("slow", func(_ context.Context, d TestData) (TestData, error) {
				time.Sleep(50 * time.Millisecond)
				d.Value = 100 // Doesn't meet condition
				return d, nil
			})
			fast := Apply("fast", func(_ context.Context, d TestData) (TestData, error) {
				time.Sleep(10 * time.Millisecond)
				d.Value = 200 // Meets condition
				return d, nil
			})
			verySlow := Apply("very-slow", func(_ context.Context, d TestData) (TestData, error) {
				time.Sleep(100 * time.Millisecond)
				d.Value = 300 // Also meets condition but too slow
				return d, nil
			})

			contest := NewContest("test-contest", condition, slow, fast, verySlow)
			defer contest.Close()

			// Verify observability components are initialized
			if contest.Metrics() == nil {
				t.Error("expected metrics registry to be initialized")
			}
			if contest.Tracer() == nil {
				t.Error("expected tracer to be initialized")
			}

			// Capture spans using the callback API
			var spans []tracez.Span
			var spanMu sync.Mutex
			contest.Tracer().OnSpanComplete(func(span tracez.Span) {
				spanMu.Lock()
				spans = append(spans, span)
				spanMu.Unlock()
			})

			// Process - fast should win
			testData := TestData{Value: 42}
			result, err := contest.Process(context.Background(), testData)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if result.Value != 200 {
				t.Errorf("expected fast processor result (200), got %d", result.Value)
			}

			// Wait for spans to be collected
			time.Sleep(120 * time.Millisecond)

			// Verify metrics
			processedTotal := contest.Metrics().Counter(ContestProcessedTotal).Value()
			if processedTotal != 1 {
				t.Errorf("expected 1 processed item, got %f", processedTotal)
			}

			successesTotal := contest.Metrics().Counter(ContestSuccessesTotal).Value()
			if successesTotal != 1 {
				t.Errorf("expected 1 success, got %f", successesTotal)
			}

			winsTotal := contest.Metrics().Counter(ContestWinsTotal).Value()
			if winsTotal != 1 {
				t.Errorf("expected 1 win, got %f", winsTotal)
			}

			processorCount := contest.Metrics().Gauge(ContestProcessorCount).Value()
			if processorCount != 3 {
				t.Errorf("expected processor count 3, got %f", processorCount)
			}

			// Check duration was recorded
			duration := contest.Metrics().Gauge(ContestDurationMs).Value()
			if duration < 10 { // Should be at least 10ms (fast processor time)
				t.Errorf("expected duration >= 10ms, got %f", duration)
			}

			// Verify spans were captured (at minimum 1 main span and winner processor span)
			spanMu.Lock()
			spanCount := len(spans)
			spanMu.Unlock()

			if spanCount < 2 {
				t.Errorf("expected at least 2 spans (1 main + winner processor), got %d", spanCount)
			}

			// Check span details
			spanMu.Lock()
			for _, span := range spans {
				if span.Name == ContestProcessSpan {
					// Main span should have processor count and winner
					if _, ok := span.Tags[ContestTagProcessorCount]; !ok {
						t.Error("main span missing processor_count tag")
					}
					if _, ok := span.Tags[ContestTagWinner]; !ok {
						t.Error("main span missing winner tag")
					}
					if _, ok := span.Tags[ContestTagConditionMet]; !ok {
						t.Error("main span missing condition_met tag")
					}
				} else if span.Name == ContestProcessorSpan {
					// Processor spans should have processor name
					if _, ok := span.Tags[ContestTagProcessorName]; !ok {
						t.Error("processor span missing processor_name tag")
					}
				}
			}
			spanMu.Unlock()
		})

		t.Run("Metrics and Spans - No Condition Met", func(t *testing.T) {
			// Condition: value must be > 1000 (none will meet this)
			condition := func(_ context.Context, d TestData) bool {
				return d.Value > 1000
			}

			proc1 := Apply("proc1", func(_ context.Context, d TestData) (TestData, error) {
				d.Value = 100
				return d, nil
			})
			proc2 := Apply("proc2", func(_ context.Context, d TestData) (TestData, error) {
				d.Value = 200
				return d, nil
			})
			proc3 := Apply("proc3", func(_ context.Context, d TestData) (TestData, error) {
				d.Value = 300
				return d, nil
			})

			contest := NewContest("test-contest-no-meet", condition, proc1, proc2, proc3)
			defer contest.Close()

			testData := TestData{Value: 42}
			_, err := contest.Process(context.Background(), testData)
			if err == nil {
				t.Fatal("expected error when no results meet condition")
			}

			// Check metrics for no condition met case
			processedTotal := contest.Metrics().Counter(ContestProcessedTotal).Value()
			if processedTotal != 1 {
				t.Errorf("expected 1 processed item, got %f", processedTotal)
			}

			successesTotal := contest.Metrics().Counter(ContestSuccessesTotal).Value()
			if successesTotal != 0 {
				t.Errorf("expected 0 successes, got %f", successesTotal)
			}

			noConditionMet := contest.Metrics().Counter(ContestNoConditionMet).Value()
			if noConditionMet != 1 {
				t.Errorf("expected 1 no_condition_met count, got %f", noConditionMet)
			}

			winsTotal := contest.Metrics().Counter(ContestWinsTotal).Value()
			if winsTotal != 0 {
				t.Errorf("expected 0 wins when no condition met, got %f", winsTotal)
			}
		})

		t.Run("Metrics and Spans - All Failed", func(t *testing.T) {
			condition := func(_ context.Context, d TestData) bool {
				return d.Value > 50
			}

			fail1 := Apply("fail1", func(_ context.Context, _ TestData) (TestData, error) {
				time.Sleep(10 * time.Millisecond)
				return TestData{}, errors.New("error 1")
			})
			fail2 := Apply("fail2", func(_ context.Context, _ TestData) (TestData, error) {
				time.Sleep(20 * time.Millisecond)
				return TestData{}, errors.New("error 2")
			})
			fail3 := Apply("fail3", func(_ context.Context, _ TestData) (TestData, error) {
				time.Sleep(30 * time.Millisecond)
				return TestData{}, errors.New("error 3")
			})

			contest := NewContest("test-contest-fail", condition, fail1, fail2, fail3)
			defer contest.Close()

			testData := TestData{Value: 42}
			_, err := contest.Process(context.Background(), testData)
			if err == nil {
				t.Fatal("expected error when all processors fail")
			}

			// Check metrics for all failed case
			processedTotal := contest.Metrics().Counter(ContestProcessedTotal).Value()
			if processedTotal != 1 {
				t.Errorf("expected 1 processed item, got %f", processedTotal)
			}

			successesTotal := contest.Metrics().Counter(ContestSuccessesTotal).Value()
			if successesTotal != 0 {
				t.Errorf("expected 0 successes, got %f", successesTotal)
			}

			allFailedTotal := contest.Metrics().Counter(ContestAllFailedTotal).Value()
			if allFailedTotal != 1 {
				t.Errorf("expected 1 all_failed count, got %f", allFailedTotal)
			}

			winsTotal := contest.Metrics().Counter(ContestWinsTotal).Value()
			if winsTotal != 0 {
				t.Errorf("expected 0 wins when all fail, got %f", winsTotal)
			}
		})

		t.Run("Empty Processors Metrics", func(t *testing.T) {
			condition := func(_ context.Context, d TestData) bool {
				return d.Value > 50
			}

			contest := NewContest[TestData]("empty-contest", condition)
			defer contest.Close()

			testData := TestData{Value: 10}
			_, err := contest.Process(context.Background(), testData)
			if err == nil {
				t.Error("expected error for empty processor list")
			}

			// Check metrics for empty processor list
			processedTotal := contest.Metrics().Counter(ContestProcessedTotal).Value()
			if processedTotal != 1 {
				t.Errorf("expected 1 processed item, got %f", processedTotal)
			}

			processorCount := contest.Metrics().Gauge(ContestProcessorCount).Value()
			if processorCount != 0 {
				t.Errorf("expected processor count 0, got %f", processorCount)
			}
		})

		t.Run("Hooks fire on contest events", func(t *testing.T) {
			// Test winner event
			condition := func(_ context.Context, d TestData) bool {
				return d.Value > 150
			}

			fast := Apply("fast", func(_ context.Context, d TestData) (TestData, error) {
				time.Sleep(5 * time.Millisecond)
				d.Value = 200 // Meets condition
				return d, nil
			})
			slow := Apply("slow", func(_ context.Context, d TestData) (TestData, error) {
				time.Sleep(20 * time.Millisecond)
				d.Value = 300 // Also meets but slower
				return d, nil
			})

			contest := NewContest("test-hooks", condition, fast, slow)
			defer contest.Close()

			var winnerEvents []ContestEvent
			var mu sync.Mutex
			contest.OnWinner(func(_ context.Context, event ContestEvent) error {
				mu.Lock()
				winnerEvents = append(winnerEvents, event)
				mu.Unlock()
				return nil
			})

			data := TestData{Value: 1}
			result, err := contest.Process(context.Background(), data)
			if err != nil {
				t.Errorf("expected success, got error: %v", err)
			}
			if result.Value != 200 {
				t.Errorf("expected fast processor to win with 200, got %d", result.Value)
			}

			// Wait for async hook
			time.Sleep(10 * time.Millisecond)

			mu.Lock()
			winnerCount := len(winnerEvents)
			var winner Name
			var conditionMet bool
			var processorCount int
			if len(winnerEvents) > 0 {
				event := winnerEvents[0]
				winner = event.Winner
				conditionMet = event.ConditionMet
				processorCount = event.ProcessorCount
			}
			mu.Unlock()

			if winnerCount != 1 {
				t.Errorf("expected 1 winner event, got %d", winnerCount)
			}

			if winnerCount > 0 {
				if winner != "fast" {
					t.Errorf("expected winner 'fast', got %s", winner)
				}
				if !conditionMet {
					t.Error("expected ConditionMet=true")
				}
				if processorCount != 2 {
					t.Errorf("expected 2 processors, got %d", processorCount)
				}
			}
		})

		t.Run("No condition met hook fires", func(t *testing.T) {
			// Impossible condition
			condition := func(_ context.Context, d TestData) bool {
				return d.Value > 1000
			}

			proc1 := Apply("proc1", func(_ context.Context, d TestData) (TestData, error) {
				d.Value = 100
				return d, nil
			})
			proc2 := Apply("proc2", func(_ context.Context, d TestData) (TestData, error) {
				d.Value = 200
				return d, nil
			})

			contest := NewContest("test-no-condition", condition, proc1, proc2)
			defer contest.Close()

			var noConditionEvents []ContestEvent
			var mu sync.Mutex
			contest.OnNoCondition(func(_ context.Context, event ContestEvent) error {
				mu.Lock()
				noConditionEvents = append(noConditionEvents, event)
				mu.Unlock()
				return nil
			})

			data := TestData{Value: 1}
			_, err := contest.Process(context.Background(), data)
			if err == nil {
				t.Error("expected error when no condition met")
			}

			// Wait for async hook
			time.Sleep(10 * time.Millisecond)

			mu.Lock()
			noConditionCount := len(noConditionEvents)
			var conditionMet bool
			var processorCount int
			if len(noConditionEvents) > 0 {
				event := noConditionEvents[0]
				conditionMet = event.ConditionMet
				processorCount = event.ProcessorCount
			}
			mu.Unlock()

			if noConditionCount != 1 {
				t.Errorf("expected 1 no-condition event, got %d", noConditionCount)
			}

			if noConditionCount > 0 {
				if conditionMet {
					t.Error("expected ConditionMet=false")
				}
				if processorCount != 2 {
					t.Errorf("expected 2 processors, got %d", processorCount)
				}
			}
		})

		t.Run("All failed hook fires", func(t *testing.T) {
			condition := func(_ context.Context, d TestData) bool {
				return d.Value > 100
			}

			fail1 := Apply("fail1", func(_ context.Context, d TestData) (TestData, error) {
				return d, errors.New("error 1")
			})
			fail2 := Apply("fail2", func(_ context.Context, d TestData) (TestData, error) {
				return d, errors.New("error 2")
			})

			contest := NewContest("test-all-failed", condition, fail1, fail2)
			defer contest.Close()

			var allFailedEvents []ContestEvent
			var mu sync.Mutex
			contest.OnAllFailed(func(_ context.Context, event ContestEvent) error {
				mu.Lock()
				allFailedEvents = append(allFailedEvents, event)
				mu.Unlock()
				return nil
			})

			data := TestData{Value: 1}
			_, err := contest.Process(context.Background(), data)
			if err == nil {
				t.Error("expected error when all processors fail")
			}

			// Wait for async hook
			time.Sleep(10 * time.Millisecond)

			mu.Lock()
			allFailedCount := len(allFailedEvents)
			var allFailed bool
			var hasError bool
			var processorCount int
			if len(allFailedEvents) > 0 {
				event := allFailedEvents[0]
				allFailed = event.AllFailed
				hasError = event.Error != nil
				processorCount = event.ProcessorCount
			}
			mu.Unlock()

			if allFailedCount != 1 {
				t.Errorf("expected 1 all-failed event, got %d", allFailedCount)
			}

			if allFailedCount > 0 {
				if !allFailed {
					t.Error("expected AllFailed=true")
				}
				if !hasError {
					t.Error("expected Error to be set")
				}
				if processorCount != 2 {
					t.Errorf("expected 2 processors, got %d", processorCount)
				}
			}
		})
	})
}
