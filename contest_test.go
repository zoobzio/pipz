package pipz

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/zoobzio/capitan"
)

func TestContest(t *testing.T) {
	t.Run("First Meeting Condition Wins", func(t *testing.T) {
		// Condition: value must be > 150
		condition := func(_ context.Context, d TestData) bool {
			return d.Value > 150
		}

		fast := Apply(NewIdentity("fast", ""), func(_ context.Context, d TestData) (TestData, error) {
			time.Sleep(10 * time.Millisecond)
			d.Value = 100 // Doesn't meet condition
			return d, nil
		})
		medium := Apply(NewIdentity("medium", ""), func(_ context.Context, d TestData) (TestData, error) {
			time.Sleep(30 * time.Millisecond)
			d.Value = 200 // Meets condition
			return d, nil
		})
		slow := Apply(NewIdentity("slow", ""), func(_ context.Context, d TestData) (TestData, error) {
			time.Sleep(50 * time.Millisecond)
			d.Value = 300 // Also meets condition but slower
			return d, nil
		})

		contest := NewContest(NewIdentity("test-contest", ""), condition, fast, medium, slow)
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

		p1 := Apply(NewIdentity("p1", ""), func(_ context.Context, d TestData) (TestData, error) {
			d.Value = 100
			return d, nil
		})
		p2 := Apply(NewIdentity("p2", ""), func(_ context.Context, d TestData) (TestData, error) {
			d.Value = 200
			return d, nil
		})
		p3 := Apply(NewIdentity("p3", ""), func(_ context.Context, d TestData) (TestData, error) {
			d.Value = 300
			return d, nil
		})

		contest := NewContest(NewIdentity("test-contest", ""), condition, p1, p2, p3)
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

		p1 := Apply(NewIdentity("p1", ""), func(_ context.Context, d TestData) (TestData, error) {
			return d, errors.New("error 1")
		})
		p2 := Apply(NewIdentity("p2", ""), func(_ context.Context, d TestData) (TestData, error) {
			return d, errors.New("error 2")
		})
		p3 := Apply(NewIdentity("p3", ""), func(_ context.Context, d TestData) (TestData, error) {
			return d, errors.New("error 3")
		})

		contest := NewContest(NewIdentity("test-contest", ""), condition, p1, p2, p3)
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

		failing := Apply(NewIdentity("failing", ""), func(_ context.Context, d TestData) (TestData, error) {
			time.Sleep(5 * time.Millisecond)
			return d, errors.New("service unavailable")
		})
		wrongValue := Apply(NewIdentity("wrong-value", ""), func(_ context.Context, d TestData) (TestData, error) {
			time.Sleep(10 * time.Millisecond)
			d.Value = 100 // Not divisible by 3
			return d, nil
		})
		correct := Apply(NewIdentity("correct", ""), func(_ context.Context, d TestData) (TestData, error) {
			time.Sleep(20 * time.Millisecond)
			d.Value = 99 // Divisible by 3
			return d, nil
		})

		contest := NewContest(NewIdentity("test-contest", ""), condition, failing, wrongValue, correct)
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

		slow := Apply(NewIdentity("slow", ""), func(ctx context.Context, d TestData) (TestData, error) {
			select {
			case <-time.After(100 * time.Millisecond):
				d.Value = 200
				return d, nil
			case <-ctx.Done():
				return d, ctx.Err()
			}
		})

		contest := NewContest(NewIdentity("test-contest", ""), condition, slow)
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

		contest := NewContest[TestData](NewIdentity("empty", ""), condition)
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
		p1 := Transform(NewIdentity("p1", ""), func(_ context.Context, d TestData) TestData { return d })
		contest := NewContest[TestData](NewIdentity("no-condition", ""), nil, p1)
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

		fast := Apply(NewIdentity("fast", ""), func(_ context.Context, d TestData) (TestData, error) {
			// Small delay to ensure slow processor starts
			time.Sleep(10 * time.Millisecond)
			d.Value = 100 // Meets condition
			return d, nil
		})
		slow := Apply(NewIdentity("slow", ""), func(ctx context.Context, d TestData) (TestData, error) {
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

		contest := NewContest(NewIdentity("test-contest", ""), condition, fast, slow)
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

		p1 := Transform(NewIdentity("p1", ""), func(_ context.Context, d TestData) TestData { return d })
		p2 := Transform(NewIdentity("p2", ""), func(_ context.Context, d TestData) TestData { return d })
		p3 := Transform(NewIdentity("p3", ""), func(_ context.Context, d TestData) TestData { return d })

		contest := NewContest(NewIdentity("test", ""), condition, p1, p2)

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

		p1 := Apply(NewIdentity("p1", ""), func(_ context.Context, d TestData) (TestData, error) {
			d.Value = 50 // Doesn't meet initial condition
			return d, nil
		})

		contest := NewContest(NewIdentity("test", ""), initialCondition, p1)
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
		contest := NewContest[TestData](NewIdentity("my-contest", ""), condition)
		if contest.Identity().Name() != "my-contest" {
			t.Errorf("expected 'my-contest', got %q", contest.Identity().Name())
		}
	})

	t.Run("Remove Out of Bounds", func(t *testing.T) {
		condition := func(_ context.Context, _ TestData) bool { return true }
		p1 := Transform(NewIdentity("p1", ""), func(_ context.Context, d TestData) TestData { return d })
		contest := NewContest(NewIdentity("test", ""), condition, p1)

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

		slow := Apply(NewIdentity("slow", ""), func(_ context.Context, d TestData) (TestData, error) {
			time.Sleep(30 * time.Millisecond)
			d.Value = 75 // Only acceptable if running out of time
			return d, nil
		})
		slower := Apply(NewIdentity("slower", ""), func(_ context.Context, d TestData) (TestData, error) {
			time.Sleep(60 * time.Millisecond)
			d.Value = 150 // Always acceptable
			return d, nil
		})

		contest := NewContest(NewIdentity("deadline-aware", ""), condition, slow, slower)
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
		processor := Apply(NewIdentity("processor", ""), func(_ context.Context, d TestData) (TestData, error) {
			d.Value = 100
			return d, nil
		})

		contest := NewContest(NewIdentity("panic_contest", ""), panicCondition, processor)
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

		if pipzErr.Path[0].Name() != "panic_contest" {
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
		panicProcessor := Apply(NewIdentity("panic_processor", ""), func(_ context.Context, _ TestData) (TestData, error) {
			panic("contest processor panic")
		})
		normalProcessor := Apply(NewIdentity("normal_processor", ""), func(_ context.Context, d TestData) (TestData, error) {
			d.Value = 100
			return d, nil
		})

		contest := NewContest(NewIdentity("panic_contest", ""), condition, panicProcessor, normalProcessor)
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
}

func TestContestClose(t *testing.T) {
	t.Run("Closes All Children", func(t *testing.T) {
		p1 := newTrackingProcessor[TestData](NewIdentity("p1", ""))
		p2 := newTrackingProcessor[TestData](NewIdentity("p2", ""))

		c := NewContest(NewIdentity("test", ""), func(_ context.Context, _ TestData) bool { return true }, p1, p2)
		err := c.Close()

		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if p1.CloseCalls() != 1 || p2.CloseCalls() != 1 {
			t.Error("expected all processors to be closed")
		}
	})

	t.Run("Aggregates Errors", func(t *testing.T) {
		p1 := newTrackingProcessor[TestData](NewIdentity("p1", "")).WithCloseError(errors.New("p1 error"))
		p2 := newTrackingProcessor[TestData](NewIdentity("p2", "")).WithCloseError(errors.New("p2 error"))

		c := NewContest(NewIdentity("test", ""), func(_ context.Context, _ TestData) bool { return true }, p1, p2)
		err := c.Close()

		if err == nil {
			t.Error("expected error")
		}
		if p1.CloseCalls() != 1 || p2.CloseCalls() != 1 {
			t.Error("expected all processors to be closed")
		}
	})

	t.Run("Idempotency", func(t *testing.T) {
		p := newTrackingProcessor[TestData](NewIdentity("p", ""))
		c := NewContest(NewIdentity("test", ""), func(_ context.Context, _ TestData) bool { return true }, p)

		_ = c.Close()
		_ = c.Close()

		if p.CloseCalls() != 1 {
			t.Errorf("expected 1 close call, got %d", p.CloseCalls())
		}
	})
}

func TestContestSignals(t *testing.T) {
	t.Run("Emits Winner Signal When Condition Met", func(t *testing.T) {
		var signalReceived bool
		var signalName string
		var signalWinnerName string
		var signalDuration float64

		listener := capitan.Hook(SignalContestWinner, func(_ context.Context, e *capitan.Event) {
			signalReceived = true
			signalName, _ = FieldName.From(e)
			signalWinnerName, _ = FieldWinnerName.From(e)
			signalDuration, _ = FieldDuration.From(e)
		})
		defer listener.Close()

		// Condition: value must be > 50
		condition := func(_ context.Context, d TestData) bool {
			return d.Value > 50
		}

		winner := Transform(NewIdentity("the-winner", ""), func(_ context.Context, d TestData) TestData {
			d.Value = 100
			return d
		})
		loser := Transform(NewIdentity("the-loser", ""), func(_ context.Context, d TestData) TestData {
			d.Value = 10 // Doesn't meet condition
			return d
		})

		contest := NewContest(NewIdentity("signal-test-contest", ""), condition, winner, loser)
		data := TestData{Value: 5}

		_, err := contest.Process(context.Background(), data)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if err := listener.Drain(context.Background()); err != nil {
			t.Fatalf("drain failed: %v", err)
		}

		if !signalReceived {
			t.Error("expected signal to be received")
		}
		if signalName != "signal-test-contest" {
			t.Errorf("expected name 'signal-test-contest', got %q", signalName)
		}
		if signalWinnerName != "the-winner" {
			t.Errorf("expected winner_name 'the-winner', got %q", signalWinnerName)
		}
		if signalDuration <= 0 {
			t.Error("expected positive duration")
		}
	})

	t.Run("Does Not Emit Signal When No Winner", func(t *testing.T) {
		var signalReceived bool

		listener := capitan.Hook(SignalContestWinner, func(_ context.Context, _ *capitan.Event) {
			signalReceived = true
		})
		defer listener.Close()

		// Condition that can never be met
		condition := func(_ context.Context, d TestData) bool {
			return d.Value > 1000
		}

		p1 := Transform(NewIdentity("p1", ""), func(_ context.Context, d TestData) TestData {
			d.Value = 10
			return d
		})
		p2 := Transform(NewIdentity("p2", ""), func(_ context.Context, d TestData) TestData {
			d.Value = 20
			return d
		})

		contest := NewContest(NewIdentity("signal-no-winner", ""), condition, p1, p2)
		data := TestData{Value: 5}

		_, err := contest.Process(context.Background(), data)

		if err == nil {
			t.Fatal("expected error when no winner")
		}

		if err := listener.Drain(context.Background()); err != nil {
			t.Fatalf("drain failed: %v", err)
		}

		if signalReceived {
			t.Error("signal should not be emitted when no winner")
		}
	})

	t.Run("Schema", func(t *testing.T) {
		proc1 := Transform(NewIdentity("proc1", ""), func(_ context.Context, d TestData) TestData { return d })
		proc2 := Transform(NewIdentity("proc2", ""), func(_ context.Context, d TestData) TestData { return d })
		condition := func(_ context.Context, _ TestData) bool { return true }

		contest := NewContest(NewIdentity("test-contest", "Contest connector"), condition, proc1, proc2)

		schema := contest.Schema()

		if schema.Identity.Name() != "test-contest" {
			t.Errorf("Schema Identity.Name() = %v, want %v", schema.Identity.Name(), "test-contest")
		}
		if schema.Type != "contest" {
			t.Errorf("Schema Type = %v, want %v", schema.Type, "contest")
		}

		flow, ok := ContestKey.From(schema)
		if !ok {
			t.Fatal("Expected ContestFlow")
		}
		if len(flow.Competitors) != 2 {
			t.Errorf("len(Flow.Competitors) = %d, want 2", len(flow.Competitors))
		}
	})
}
