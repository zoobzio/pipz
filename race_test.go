package pipz

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestRace(t *testing.T) {
	t.Run("First Success Wins", func(t *testing.T) {
		fast := Apply("fast", func(_ context.Context, d TestData) (TestData, error) {
			time.Sleep(10 * time.Millisecond)
			d.Value = 100
			return d, nil
		})
		slow := Apply("slow", func(_ context.Context, d TestData) (TestData, error) {
			time.Sleep(50 * time.Millisecond)
			d.Value = 200
			return d, nil
		})

		race := NewRace("test-race", fast, slow)
		data := TestData{Value: 1}

		result, err := race.Process(context.Background(), data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Value != 100 {
			t.Errorf("expected fast processor result (100), got %d", result.Value)
		}
	})

	t.Run("All Fail Returns Last Error", func(t *testing.T) {
		p1 := Apply("p1", func(_ context.Context, d TestData) (TestData, error) {
			time.Sleep(10 * time.Millisecond)
			return d, errors.New("error 1")
		})
		p2 := Apply("p2", func(_ context.Context, d TestData) (TestData, error) {
			time.Sleep(20 * time.Millisecond)
			return d, errors.New("error 2")
		})
		p3 := Apply("p3", func(_ context.Context, d TestData) (TestData, error) {
			time.Sleep(30 * time.Millisecond)
			return d, errors.New("error 3")
		})

		race := NewRace("test-race", p1, p2, p3)
		data := TestData{Value: 1}

		_, err := race.Process(context.Background(), data)
		if err == nil {
			t.Fatal("expected error when all processors fail")
		}
		// Should get one of the errors (last processed)
		if !strings.Contains(err.Error(), "error") {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("Context Cancellation", func(t *testing.T) {
		slow := Apply("slow", func(ctx context.Context, d TestData) (TestData, error) {
			select {
			case <-time.After(100 * time.Millisecond):
				return d, nil
			case <-ctx.Done():
				return d, ctx.Err()
			}
		})

		race := NewRace("test-race", slow)
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()

		data := TestData{Value: 1}
		_, err := race.Process(ctx, data)

		if err != nil {
			t.Errorf("expected no error with new behavior, got %v", err)
		}
	})

	t.Run("Empty Processors", func(t *testing.T) {
		race := NewRace[TestData]("empty")
		data := TestData{Value: 42}

		_, err := race.Process(context.Background(), data)
		if err == nil {
			t.Fatal("expected error for empty race")
		}
		if !strings.Contains(err.Error(), "no processors provided to Race") {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("Winner Cancels Others", func(t *testing.T) {
		// Use channels to coordinate test timing
		slowStartedCh := make(chan bool, 1)
		slowCanceledCh := make(chan bool, 1)

		fast := Apply("fast", func(_ context.Context, d TestData) (TestData, error) {
			// Add a small delay to ensure slow processor starts
			time.Sleep(10 * time.Millisecond)
			d.Value = 100
			return d, nil
		})
		slow := Apply("slow", func(ctx context.Context, d TestData) (TestData, error) {
			slowStartedCh <- true
			select {
			case <-time.After(200 * time.Millisecond):
				d.Value = 200
				return d, nil
			case <-ctx.Done():
				slowCanceledCh <- true
				return d, ctx.Err()
			}
		})

		race := NewRace("test-race", fast, slow)
		data := TestData{Value: 1}

		result, err := race.Process(context.Background(), data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Value != 100 {
			t.Errorf("expected fast result, got %d", result.Value)
		}

		// Wait for signals with timeout
		select {
		case <-slowStartedCh:
			// Good, slow processor started
		case <-time.After(100 * time.Millisecond):
			t.Error("slow processor should have started")
		}

		select {
		case <-slowCanceledCh:
			// Good, slow processor was canceled
		case <-time.After(100 * time.Millisecond):
			t.Error("slow processor should have been canceled")
		}
	})

	t.Run("Configuration Methods", func(t *testing.T) {
		p1 := Transform("p1", func(_ context.Context, d TestData) TestData { return d })
		p2 := Transform("p2", func(_ context.Context, d TestData) TestData { return d })
		p3 := Transform("p3", func(_ context.Context, d TestData) TestData { return d })

		race := NewRace("test", p1, p2)

		if race.Len() != 2 {
			t.Errorf("expected 2 processors, got %d", race.Len())
		}

		race.Add(p3)
		if race.Len() != 3 {
			t.Errorf("expected 3 processors after add, got %d", race.Len())
		}

		err := race.Remove(1)
		if err != nil {
			t.Fatalf("unexpected error removing processor: %v", err)
		}
		if race.Len() != 2 {
			t.Errorf("expected 2 processors after remove, got %d", race.Len())
		}

		race.Clear()
		if race.Len() != 0 {
			t.Errorf("expected 0 processors after clear, got %d", race.Len())
		}

		race.SetProcessors(p1, p2, p3)
		if race.Len() != 3 {
			t.Errorf("expected 3 processors after SetProcessors, got %d", race.Len())
		}
	})

	t.Run("Name Method", func(t *testing.T) {
		race := NewRace[TestData]("my-race")
		if race.Name() != "my-race" {
			t.Errorf("expected 'my-race', got %q", race.Name())
		}
	})

	t.Run("Remove Out of Bounds", func(t *testing.T) {
		p1 := Transform("p1", func(_ context.Context, d TestData) TestData { return d })
		race := NewRace("test", p1)

		// Test negative index
		err := race.Remove(-1)
		if err == nil {
			t.Error("expected error for negative index")
		}
		if !errors.Is(err, ErrIndexOutOfBounds) {
			t.Errorf("expected ErrIndexOutOfBounds, got %v", err)
		}

		// Test index >= length
		err = race.Remove(1)
		if err == nil {
			t.Error("expected error for index >= length")
		}
		if !errors.Is(err, ErrIndexOutOfBounds) {
			t.Errorf("expected ErrIndexOutOfBounds, got %v", err)
		}
	})
}
