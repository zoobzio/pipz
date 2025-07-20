package events

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

func BenchmarkEventPipeline(b *testing.B) {
	pipeline := CreateEventPipeline()
	ctx := context.Background()

	// Pre-create events for different scenarios
	userEvent := Event{
		ID:        "bench-user",
		Type:      EventUserCreated,
		Version:   "1.0",
		Source:    "api",
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"email": "user@example.com",
			"name":  "Test User",
		},
	}

	orderEvent := Event{
		ID:        "bench-order",
		Type:      EventOrderPlaced,
		Version:   "1.0",
		Source:    "web",
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"total":    99.99,
			"items":    3,
			"currency": "USD",
		},
	}

	b.Run("UserEvent", func(b *testing.B) {
		// Clear dedup cache periodically
		deduplicator = NewEventDeduplicator()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// Unique ID to avoid dedup
			userEvent.ID = fmt.Sprintf("bench-user-%d", i)
			_, err := pipeline.Process(ctx, userEvent)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("OrderEvent", func(b *testing.B) {
		deduplicator = NewEventDeduplicator()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			orderEvent.ID = fmt.Sprintf("bench-order-%d", i)
			_, err := pipeline.Process(ctx, orderEvent)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("WithDeduplication", func(b *testing.B) {
		// Test deduplication overhead
		dedupEvent := Event{
			ID:        "dedup-bench", // Same ID
			Type:      EventUserCreated,
			Version:   "1.0",
			Timestamp: time.Now(),
			Data:      map[string]interface{}{"email": "test@example.com"},
		}

		// First process should succeed
		pipeline.Process(ctx, dedupEvent)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// All subsequent attempts should be rejected
			pipeline.Process(ctx, dedupEvent)
		}
	})
}

func BenchmarkBatchPipeline(b *testing.B) {
	pipeline := CreateBatchEventPipeline()
	ctx := context.Background()

	// Create batches of different sizes
	createBatch := func(size int) []Event {
		events := make([]Event, size)
		for i := 0; i < size; i++ {
			events[i] = Event{
				ID:        fmt.Sprintf("batch-%d-%d", size, i),
				Type:      EventUserCreated,
				Version:   "1.0",
				Timestamp: time.Now(),
				Data:      map[string]interface{}{"email": fmt.Sprintf("user%d@example.com", i)},
			}
		}
		return events
	}

	benchmarks := []int{10, 100, 1000}

	for _, size := range benchmarks {
		b.Run(fmt.Sprintf("BatchSize_%d", size), func(b *testing.B) {
			batch := createBatch(size)
			deduplicator = NewEventDeduplicator() // Clear cache

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				// Update IDs to avoid dedup
				for j := range batch {
					batch[j].ID = fmt.Sprintf("batch-%d-%d-%d", size, i, j)
				}

				_, err := pipeline.Process(ctx, batch)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkRouting(b *testing.B) {
	pipeline := CreateEventPipeline()
	ctx := context.Background()

	// Different event types to test routing performance
	eventTypes := []struct {
		name  string
		event Event
	}{
		{
			name: "UserRoute",
			event: Event{
				Type:      EventUserCreated,
				Version:   "1.0",
				Timestamp: time.Now(),
				Data:      map[string]interface{}{"email": "test@example.com"},
			},
		},
		{
			name: "OrderRoute",
			event: Event{
				Type:      EventOrderPlaced,
				Version:   "1.0",
				Timestamp: time.Now(),
				Data:      map[string]interface{}{"total": 50.0},
			},
		},
		{
			name: "PaymentRoute",
			event: Event{
				Type:      EventPaymentSuccess,
				Version:   "1.0",
				Timestamp: time.Now(),
				Data:      map[string]interface{}{"order_id": "12345"},
			},
		},
	}

	for _, et := range eventTypes {
		b.Run(et.name, func(b *testing.B) {
			deduplicator = NewEventDeduplicator()

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				et.event.ID = fmt.Sprintf("route-%s-%d", et.name, i)
				_, err := pipeline.Process(ctx, et.event)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkPriorityRouting(b *testing.B) {
	pipeline := CreatePriorityEventPipeline()
	ctx := context.Background()

	b.Run("HighPriority", func(b *testing.B) {
		event := Event{
			Type:      EventPaymentFailed,
			Version:   "1.0",
			Timestamp: time.Now(),
		}
		deduplicator = NewEventDeduplicator()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			event.ID = fmt.Sprintf("high-%d", i)
			_, err := pipeline.Process(ctx, event)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("NormalPriority", func(b *testing.B) {
		event := Event{
			Type:      EventUserUpdated,
			Version:   "1.0",
			Timestamp: time.Now(),
		}
		deduplicator = NewEventDeduplicator()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			event.ID = fmt.Sprintf("normal-%d", i)
			_, err := pipeline.Process(ctx, event)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkIndividualStages(b *testing.B) {
	ctx := context.Background()

	event := Event{
		ID:        "stage-bench",
		Type:      EventUserCreated,
		Version:   "1.0",
		Timestamp: time.Now(),
		Data:      map[string]interface{}{"email": "test@example.com"},
	}

	b.Run("Validation", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			err := ValidateSchema(ctx, event)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("Deduplication", func(b *testing.B) {
		deduplicator = NewEventDeduplicator()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			event.ID = fmt.Sprintf("dedup-%d", i)
			_, err := DeduplicateEvent(ctx, event)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("Enrichment", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = EnrichEvent(ctx, event)
		}
	})

	b.Run("Handler", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := HandleUserEvent(ctx, event)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkConcurrentEvents(b *testing.B) {
	pipeline := CreateEventPipeline()
	ctx := context.Background()

	// Test concurrent event processing
	concurrencyLevels := []int{1, 10, 100}

	for _, level := range concurrencyLevels {
		b.Run(fmt.Sprintf("Concurrency_%d", level), func(b *testing.B) {
			deduplicator = NewEventDeduplicator()

			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				i := 0
				for pb.Next() {
					event := Event{
						ID:        fmt.Sprintf("concurrent-%d-%d", level, i),
						Type:      EventUserCreated,
						Version:   "1.0",
						Timestamp: time.Now(),
						Data:      map[string]interface{}{"email": fmt.Sprintf("user%d@example.com", i)},
					}

					_, err := pipeline.Process(ctx, event)
					if err != nil && !strings.Contains(err.Error(), "already processed") {
						b.Fatal(err)
					}
					i++
				}
			})
		})
	}
}

var globalEvent Event // Prevent compiler optimizations

func BenchmarkMemoryAllocation(b *testing.B) {
	pipeline := CreateEventPipeline()
	ctx := context.Background()

	b.Run("EventProcessing", func(b *testing.B) {
		deduplicator = NewEventDeduplicator()

		b.ReportAllocs()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			event := Event{
				ID:        fmt.Sprintf("mem-%d", i),
				Type:      EventUserCreated,
				Version:   "1.0",
				Timestamp: time.Now(),
				Data: map[string]interface{}{
					"email": "test@example.com",
					"name":  "Test User",
					"age":   30,
				},
				Metadata: map[string]string{
					"source": "api",
					"ip":     "192.168.1.1",
				},
			}

			result, err := pipeline.Process(ctx, event)
			if err != nil {
				b.Fatal(err)
			}
			globalEvent = result // Prevent optimization
		}
	})
}
