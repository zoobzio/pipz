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

	t.Run("Race processor panic recovery", func(t *testing.T) {
		panicProcessor := Apply("panic_processor", func(_ context.Context, _ TestData) (TestData, error) {
			panic("race processor panic")
		})
		successProcessor := Apply("success_processor", func(_ context.Context, d TestData) (TestData, error) {
			time.Sleep(10 * time.Millisecond) // Small delay to ensure panic happens first
			d.Value = 100
			return d, nil
		})

		race := NewRace("panic_race", panicProcessor, successProcessor)
		data := TestData{Value: 42}

		result, err := race.Process(context.Background(), data)

		// Success processor should win
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.Value != 100 {
			t.Errorf("expected success processor result 100, got %d", result.Value)
		}
	})

	t.Run("All race processors panic", func(t *testing.T) {
		panic1 := Apply("panic1", func(_ context.Context, _ TestData) (TestData, error) {
			panic("first processor panic")
		})
		panic2 := Apply("panic2", func(_ context.Context, _ TestData) (TestData, error) {
			panic("second processor panic")
		})

		race := NewRace("all_panic_race", panic1, panic2)
		data := TestData{Value: 42}

		result, err := race.Process(context.Background(), data)

		if result.Value != 42 {
			t.Errorf("expected original input value 42, got %d", result.Value)
		}

		var pipzErr *Error[TestData]
		if !errors.As(err, &pipzErr) {
			t.Fatal("expected pipz.Error")
		}

		if pipzErr.Path[0] != "all_panic_race" {
			t.Errorf("expected path to start with 'all_panic_race', got %v", pipzErr.Path)
		}

		if pipzErr.InputData.Value != 42 {
			t.Errorf("expected input data value 42, got %d", pipzErr.InputData.Value)
		}
	})

	t.Run("Observability", func(t *testing.T) {
		t.Run("Metrics and Spans", func(t *testing.T) {
			// Create processors with different speeds
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
			verySlow := Apply("very-slow", func(_ context.Context, d TestData) (TestData, error) {
				time.Sleep(100 * time.Millisecond)
				d.Value = 300
				return d, nil
			})

			race := NewRace("test-race", fast, slow, verySlow)
			defer race.Close()

			// Verify observability components are initialized
			if race.Metrics() == nil {
				t.Error("expected metrics registry to be initialized")
			}
			if race.Tracer() == nil {
				t.Error("expected tracer to be initialized")
			}

			// Capture spans using the callback API
			var spans []tracez.Span
			var spanMu sync.Mutex
			race.Tracer().OnSpanComplete(func(span tracez.Span) {
				spanMu.Lock()
				spans = append(spans, span)
				spanMu.Unlock()
			})

			// Process - fast should win
			testData := TestData{Value: 42}
			result, err := race.Process(context.Background(), testData)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if result.Value != 100 {
				t.Errorf("expected fast processor result (100), got %d", result.Value)
			}

			// Wait for spans to be collected (including canceled ones)
			time.Sleep(120 * time.Millisecond) // Wait longer than the slowest processor

			// Verify metrics
			processedTotal := race.Metrics().Counter(RaceProcessedTotal).Value()
			if processedTotal != 1 {
				t.Errorf("expected 1 processed item, got %f", processedTotal)
			}

			successesTotal := race.Metrics().Counter(RaceSuccessesTotal).Value()
			if successesTotal != 1 {
				t.Errorf("expected 1 success, got %f", successesTotal)
			}

			winsTotal := race.Metrics().Counter(RaceWinsTotal).Value()
			if winsTotal != 1 {
				t.Errorf("expected 1 win, got %f", winsTotal)
			}

			processorCount := race.Metrics().Gauge(RaceProcessorCount).Value()
			if processorCount != 3 {
				t.Errorf("expected processor count 3, got %f", processorCount)
			}

			// Check duration was recorded
			duration := race.Metrics().Gauge(RaceDurationMs).Value()
			if duration < 10 { // Should be at least 10ms (fast processor time)
				t.Errorf("expected duration >= 10ms, got %f", duration)
			}

			// Verify spans were captured (at minimum 1 main span and the winner processor span)
			// Note: other processor spans might be canceled before completing
			spanMu.Lock()
			spanCount := len(spans)
			spanMu.Unlock()

			if spanCount < 2 {
				t.Errorf("expected at least 2 spans (1 main + winner processor), got %d", spanCount)
			}

			// Check span details
			spanMu.Lock()
			for _, span := range spans {
				if span.Name == RaceProcessSpan {
					// Main span should have processor count and winner
					if _, ok := span.Tags[RaceTagProcessorCount]; !ok {
						t.Error("main span missing processor_count tag")
					}
					if _, ok := span.Tags[RaceTagWinner]; !ok {
						t.Error("main span missing winner tag")
					}
				} else if span.Name == RaceProcessorSpan {
					// Processor spans should have processor name
					if _, ok := span.Tags[RaceTagProcessorName]; !ok {
						t.Error("processor span missing processor_name tag")
					}
				}
			}
			spanMu.Unlock()
		})

		t.Run("All Failed Metrics", func(t *testing.T) {
			// Create processors that all fail
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

			race := NewRace("test-race-fail", fail1, fail2, fail3)
			defer race.Close()

			testData := TestData{Value: 42}
			_, err := race.Process(context.Background(), testData)
			if err == nil {
				t.Fatal("expected error when all processors fail")
			}

			// Check metrics for all failed case
			processedTotal := race.Metrics().Counter(RaceProcessedTotal).Value()
			if processedTotal != 1 {
				t.Errorf("expected 1 processed item, got %f", processedTotal)
			}

			successesTotal := race.Metrics().Counter(RaceSuccessesTotal).Value()
			if successesTotal != 0 {
				t.Errorf("expected 0 successes, got %f", successesTotal)
			}

			allFailedTotal := race.Metrics().Counter(RaceAllFailedTotal).Value()
			if allFailedTotal != 1 {
				t.Errorf("expected 1 all_failed count, got %f", allFailedTotal)
			}

			winsTotal := race.Metrics().Counter(RaceWinsTotal).Value()
			if winsTotal != 0 {
				t.Errorf("expected 0 wins when all fail, got %f", winsTotal)
			}
		})

		t.Run("Empty Processors Metrics", func(t *testing.T) {
			race := NewRace[TestData]("empty-race")
			defer race.Close()

			testData := TestData{Value: 10}
			_, err := race.Process(context.Background(), testData)
			if err == nil {
				t.Error("expected error for empty processor list")
			}

			// Check metrics for empty processor list
			processedTotal := race.Metrics().Counter(RaceProcessedTotal).Value()
			if processedTotal != 1 {
				t.Errorf("expected 1 processed item, got %f", processedTotal)
			}

			processorCount := race.Metrics().Gauge(RaceProcessorCount).Value()
			if processorCount != 0 {
				t.Errorf("expected processor count 0, got %f", processorCount)
			}
		})

		t.Run("Hooks fire on race events", func(t *testing.T) {
			// Test winner event
			fast := Apply("fast", func(_ context.Context, d TestData) (TestData, error) {
				time.Sleep(5 * time.Millisecond)
				d.Value = 100
				return d, nil
			})
			slow := Apply("slow", func(_ context.Context, d TestData) (TestData, error) {
				time.Sleep(20 * time.Millisecond)
				d.Value = 200
				return d, nil
			})

			race := NewRace("test-hooks", fast, slow)
			defer race.Close()

			var winnerEvents []RaceEvent
			var mu sync.Mutex
			race.OnWinner(func(_ context.Context, event RaceEvent) error {
				mu.Lock()
				winnerEvents = append(winnerEvents, event)
				mu.Unlock()
				return nil
			})

			data := TestData{Value: 1}
			result, err := race.Process(context.Background(), data)
			if err != nil {
				t.Errorf("expected success, got error: %v", err)
			}
			if result.Value != 100 {
				t.Errorf("expected fast processor to win with 100, got %d", result.Value)
			}

			// Wait for async hook
			time.Sleep(10 * time.Millisecond)

			mu.Lock()
			winnerCount := len(winnerEvents)
			var winner Name
			var processorCount int
			var allFailed bool
			if len(winnerEvents) > 0 {
				event := winnerEvents[0]
				winner = event.Winner
				processorCount = event.ProcessorCount
				allFailed = event.AllFailed
			}
			mu.Unlock()

			if winnerCount != 1 {
				t.Errorf("expected 1 winner event, got %d", winnerCount)
			}

			if winnerCount > 0 {
				if winner != "fast" {
					t.Errorf("expected winner 'fast', got %s", winner)
				}
				if processorCount != 2 {
					t.Errorf("expected 2 processors, got %d", processorCount)
				}
				if allFailed {
					t.Error("expected AllFailed=false")
				}
			}
		})

		t.Run("All failed hook fires", func(t *testing.T) {
			fail1 := Apply("fail1", func(_ context.Context, d TestData) (TestData, error) {
				return d, errors.New("error 1")
			})
			fail2 := Apply("fail2", func(_ context.Context, d TestData) (TestData, error) {
				time.Sleep(5 * time.Millisecond) // Delay to ensure both run
				return d, errors.New("error 2")
			})

			race := NewRace("test-all-failed", fail1, fail2)
			defer race.Close()

			var allFailedEvents []RaceEvent
			var mu sync.Mutex
			race.OnAllFailed(func(_ context.Context, event RaceEvent) error {
				mu.Lock()
				allFailedEvents = append(allFailedEvents, event)
				mu.Unlock()
				return nil
			})

			data := TestData{Value: 1}
			_, err := race.Process(context.Background(), data)
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
