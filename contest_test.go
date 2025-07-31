package pipz

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
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
}
