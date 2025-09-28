package benchmarks

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/zoobzio/pipz"
)

// ClonableInt is a simple integer type that implements Clone for benchmarking..
type ClonableInt int

func (c ClonableInt) Clone() ClonableInt {
	return c
}

// ShoppingCart represents a user's shopping cart - tests deep cloning with slices and maps..
type ShoppingCart struct {
	UserID   int64
	Items    []CartItem
	Total    float64
	Updated  time.Time
	Metadata map[string]interface{}
}

func (sc ShoppingCart) Clone() ShoppingCart {
	// Deep copy required for Items slice and Metadata map
	clonedItems := make([]CartItem, len(sc.Items))
	copy(clonedItems, sc.Items)

	clonedMetadata := make(map[string]interface{})
	for k, v := range sc.Metadata {
		clonedMetadata[k] = v
	}

	return ShoppingCart{
		UserID:   sc.UserID,
		Items:    clonedItems,
		Total:    sc.Total,
		Updated:  sc.Updated,
		Metadata: clonedMetadata,
	}
}

type CartItem struct {
	ProductID string
	Quantity  int
	Price     float64
}

// BenchmarkPipelineLength measures how pipeline length affects performance.
func BenchmarkPipelineLength(b *testing.B) {
	ctx := context.Background()
	data := ShoppingCart{
		UserID:  12345,
		Items:   []CartItem{{ProductID: "ITEM-001", Quantity: 1, Price: 29.99}},
		Total:   29.99,
		Updated: time.Now(),
		Metadata: map[string]interface{}{
			"source":  "web",
			"session": "sess-123",
		},
	}

	lengths := []int{1, 2, 5, 10, 25, 50, 100}

	for _, length := range lengths {
		b.Run("Length_"+strconv.Itoa(length), func(b *testing.B) {
			// Create pipeline with specified length
			processors := make([]pipz.Chainable[ShoppingCart], length)
			for i := 0; i < length; i++ {
				processors[i] = pipz.Transform(pipz.Name("step"+strconv.Itoa(i)),
					func(_ context.Context, cart ShoppingCart) ShoppingCart {
						cart.Total += 0.01 // Add small processing fee
						return cart
					})
			}

			pipeline := pipz.NewSequence("pipeline", processors...)

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				result, err := pipeline.Process(ctx, data)
				if err != nil {
					b.Fatal(err)
				}
				_ = result
			}
		})
	}
}

// BenchmarkBranchingPatterns measures different branching and parallel patterns.
func BenchmarkBranchingPatterns(b *testing.B) {
	ctx := context.Background()
	data := ClonableInt(42)

	// Simple processors for composition
	double := pipz.Transform("double", func(_ context.Context, n ClonableInt) ClonableInt { return n * 2 })
	add10 := pipz.Transform("add10", func(_ context.Context, n ClonableInt) ClonableInt { return n + 10 })
	subtract5 := pipz.Transform("subtract5", func(_ context.Context, n ClonableInt) ClonableInt { return n - 5 })
	multiply3 := pipz.Transform("multiply3", func(_ context.Context, n ClonableInt) ClonableInt { return n * 3 })

	b.Run("Sequential_Chain", func(b *testing.B) {
		pipeline := pipz.NewSequence("sequential",
			double, add10, subtract5, multiply3,
		)

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			result, err := pipeline.Process(ctx, data)
			if err != nil {
				b.Fatal(err)
			}
			_ = result
		}
	})

	b.Run("Concurrent_Four_Branches", func(b *testing.B) {
		pipeline := pipz.NewConcurrent("concurrent",
			double, add10, subtract5, multiply3,
		)

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			result, err := pipeline.Process(ctx, data)
			if err != nil {
				b.Fatal(err)
			}
			_ = result
		}
	})

	b.Run("Race_Four_Branches", func(b *testing.B) {
		pipeline := pipz.NewRace("race",
			pipz.Transform("fast", func(_ context.Context, n ClonableInt) ClonableInt { return n * 2 }),
			pipz.Transform("slow1", func(_ context.Context, n ClonableInt) ClonableInt {
				time.Sleep(time.Microsecond)
				return n + 10
			}),
			pipz.Transform("slow2", func(_ context.Context, n ClonableInt) ClonableInt {
				time.Sleep(2 * time.Microsecond)
				return n - 5
			}),
			pipz.Transform("slowest", func(_ context.Context, n ClonableInt) ClonableInt {
				time.Sleep(3 * time.Microsecond)
				return n * 3
			}),
		)

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			result, err := pipeline.Process(ctx, data)
			if err != nil {
				b.Fatal(err)
			}
			_ = result
		}
	})

	b.Run("Contest_First_Acceptable", func(b *testing.B) {
		pipeline := pipz.NewContest("contest",
			func(_ context.Context, n ClonableInt) bool { return n > 100 },                                  // Accept results > 100
			pipz.Transform("small", func(_ context.Context, n ClonableInt) ClonableInt { return n + 5 }),    // Won't meet condition
			pipz.Transform("medium", func(_ context.Context, n ClonableInt) ClonableInt { return n + 50 }),  // Won't meet condition
			pipz.Transform("large", func(_ context.Context, n ClonableInt) ClonableInt { return n + 100 }),  // Will meet condition
			pipz.Transform("xlarge", func(_ context.Context, n ClonableInt) ClonableInt { return n + 200 }), // Would meet but slower
		)

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			result, err := pipeline.Process(ctx, data)
			if err != nil {
				b.Fatal(err)
			}
			_ = result
		}
	})

	b.Run("Nested_Sequential_In_Concurrent", func(b *testing.B) {
		// Create two sequential pipelines to run concurrently
		seq1 := pipz.NewSequence("seq1", double, add10)
		seq2 := pipz.NewSequence("seq2", subtract5, multiply3)

		pipeline := pipz.NewConcurrent("nested", seq1, seq2)

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			result, err := pipeline.Process(ctx, data)
			if err != nil {
				b.Fatal(err)
			}
			_ = result
		}
	})

	b.Run("Nested_Concurrent_In_Sequential", func(b *testing.B) {
		// Sequential pipeline with concurrent step in the middle
		concurrent := pipz.NewConcurrent("middle", double, add10)

		pipeline := pipz.NewSequence("nested",
			subtract5,
			concurrent,
			multiply3,
		)

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			result, err := pipeline.Process(ctx, data)
			if err != nil {
				b.Fatal(err)
			}
			_ = result
		}
	})
}

// BenchmarkComplexWorkflows measures realistic complex pipeline compositions.
func BenchmarkComplexWorkflows(b *testing.B) {
	ctx := context.Background()

	b.Run("API_Client_Workflow", func(b *testing.B) {
		type Request struct {
			URL     string
			Data    int
			Retries int
		}

		// Simulate a complete API client workflow
		pipeline := pipz.NewSequence[Request]("api-client",
			// Validate request
			pipz.Apply("validate", func(_ context.Context, req Request) (Request, error) {
				if req.URL == "" {
					return req, errors.New("URL required")
				}
				return req, nil
			}),

			// Add authentication
			pipz.Transform("auth", func(_ context.Context, req Request) Request {
				req.URL += "?token=auth_token"
				return req
			}),

			// Rate limiting
			pipz.NewRateLimiter[Request]("rate-limit", 1000.0, 100),

			// Circuit breaker + retry
			pipz.NewCircuitBreaker[Request]("circuit-breaker",
				pipz.NewRetry[Request]("retry",
					// Simulate API call
					pipz.Apply("api-call", func(_ context.Context, req Request) (Request, error) {
						// Simulate occasional failures
						if req.Data%10 == 0 {
							return req, errors.New("API error")
						}
						req.Data *= 2 // Simulate response processing
						return req, nil
					}),
					3,
				),
				5,
				time.Minute,
			),

			// Process response
			pipz.Transform("process", func(_ context.Context, req Request) Request {
				req.Data += 100
				return req
			}),
		)

		request := Request{URL: "https://api.example.com/data", Data: 42}

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			request.Data = i // Vary data to avoid consistent failures
			result, err := pipeline.Process(ctx, request)
			if err != nil {
				// Some failures expected due to simulated API errors
				continue
			}
			_ = result
		}
	})

	b.Run("Event_Processing_Workflow", func(b *testing.B) {
		type Event struct {
			ID       string
			Type     string
			Priority int
			Data     map[string]interface{}
		}

		var processedCount int64

		// Complex event processing pipeline
		pipeline := pipz.NewSequence[Event]("event-processing",
			// Validate event
			pipz.Apply("validate", func(_ context.Context, event Event) (Event, error) {
				if event.ID == "" {
					return event, errors.New("event ID required")
				}
				return event, nil
			}),

			// Filter by type
			pipz.NewFilter[Event]("type-filter",
				func(_ context.Context, event Event) bool {
					return event.Type != "ignored"
				},
				pipz.Transform("mark-accepted", func(_ context.Context, event Event) Event {
					if event.Data == nil {
						event.Data = make(map[string]interface{})
					}
					event.Data["accepted"] = true
					return event
				}),
			),

			// Route by priority
			pipz.NewSwitch[Event, string]("priority-router",
				func(_ context.Context, event Event) string {
					if event.Priority > 9 {
						return "critical"
					} else if event.Priority > 7 {
						return "high"
					}
					return "normal"
				},
			).AddRoute("critical",
				pipz.Transform("critical", func(_ context.Context, event Event) Event {
					if event.Data == nil {
						event.Data = make(map[string]interface{})
					}
					event.Data["processing"] = "critical"
					return event
				}),
			).AddRoute("high",
				pipz.Transform("high-priority", func(_ context.Context, event Event) Event {
					if event.Data == nil {
						event.Data = make(map[string]interface{})
					}
					event.Data["processing"] = "high_priority"
					return event
				}),
			).AddRoute("normal",
				pipz.Transform("normal", func(_ context.Context, event Event) Event {
					if event.Data == nil {
						event.Data = make(map[string]interface{})
					}
					event.Data["processing"] = "normal"
					return event
				}),
			),

			// Parallel enrichment
			pipz.NewSequence[Event]("enrichment",
				// Add timestamp
				pipz.Transform("timestamp", func(_ context.Context, event Event) Event {
					if event.Data == nil {
						event.Data = make(map[string]interface{})
					}
					event.Data["timestamp"] = time.Now().Unix()
					return event
				}),

				// Add metadata
				pipz.Transform("metadata", func(_ context.Context, event Event) Event {
					if event.Data == nil {
						event.Data = make(map[string]interface{})
					}
					event.Data["metadata"] = map[string]interface{}{
						"processed_by": "pipz",
						"version":      "1.0",
					}
					return event
				}),
			),

			// Final processing
			pipz.Transform("finalize", func(_ context.Context, event Event) Event {
				atomic.AddInt64(&processedCount, 1)
				if event.Data == nil {
					event.Data = make(map[string]interface{})
				}
				event.Data["final"] = true
				return event
			}),
		)

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			event := Event{
				ID:       "event-" + strconv.Itoa(i),
				Type:     "user_action",
				Priority: i % 11, // Vary priority 0-10
				Data:     make(map[string]interface{}),
			}

			result, err := pipeline.Process(ctx, event)
			if err != nil {
				b.Fatal(err)
			}
			_ = result
		}
	})

	b.Run("Data_Transformation_Workflow", func(b *testing.B) {
		type Record struct {
			ID    int
			Name  string
			Email string
			Score float64
			Tags  []string
			Valid bool
		}

		// Complex data transformation pipeline
		pipeline := pipz.NewSequence[Record]("data-transform",
			// Validation with fallback
			pipz.NewFallback[Record]("validation",
				// Strict validation
				pipz.Apply("strict-validate", func(_ context.Context, r Record) (Record, error) {
					if r.Name == "" || r.Email == "" || r.Score < 0 {
						return r, errors.New("strict validation failed")
					}
					r.Valid = true
					return r, nil
				}),
				// Lenient validation
				pipz.Apply("lenient-validate", func(_ context.Context, r Record) (Record, error) {
					if r.ID <= 0 {
						return r, errors.New("ID required")
					}
					r.Valid = false // Mark as needing cleanup
					return r, nil
				}),
			),

			// Conditional cleanup
			pipz.NewFilter[Record]("needs-cleanup",
				func(_ context.Context, r Record) bool { return !r.Valid },
				pipz.NewSequence[Record]("cleanup",
					// Fix name
					pipz.Transform("fix-name", func(_ context.Context, r Record) Record {
						if r.Name == "" {
							r.Name = "Unknown-" + strconv.Itoa(r.ID)
						}
						return r
					}),
					// Fix email
					pipz.Transform("fix-email", func(_ context.Context, r Record) Record {
						if r.Email == "" || !strings.Contains(r.Email, "@") {
							r.Email = "user" + strconv.Itoa(r.ID) + "@example.com"
						}
						return r
					}),
					// Fix score
					pipz.Transform("fix-score", func(_ context.Context, r Record) Record {
						if r.Score < 0 {
							r.Score = 0
						}
						return r
					}),
				),
			),

			// Normalization
			pipz.Transform("normalize", func(_ context.Context, r Record) Record {
				r.Name = strings.TrimSpace(strings.ToTitle(r.Name))
				r.Email = strings.ToLower(strings.TrimSpace(r.Email))
				return r
			}),

			// Enrichment with race condition (fastest wins)
			pipz.NewSequence[Record]("enrichment-race",
				// Fast local enrichment
				pipz.Transform("local-enrich", func(_ context.Context, r Record) Record {
					r.Tags = append(r.Tags, "local", "fast")
					return r
				}),
				// Slower database enrichment
				pipz.Transform("db-enrich", func(_ context.Context, r Record) Record {
					time.Sleep(time.Microsecond) // Simulate DB delay
					r.Tags = append(r.Tags, "database", "complete")
					r.Score *= 1.1 // DB has more accurate scoring
					return r
				}),
			),

			// Final scoring
			pipz.Transform("final-score", func(_ context.Context, r Record) Record {
				// Boost score based on tags
				for _, tag := range r.Tags {
					if tag == "complete" {
						r.Score *= 1.2
					}
				}
				return r
			}),
		)

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			record := Record{
				ID:    i + 1,
				Name:  "User " + strconv.Itoa(i),
				Email: "user" + strconv.Itoa(i) + "@test.com",
				Score: float64(i%100 - 10), // Some negative scores to trigger cleanup
				Tags:  make([]string, 0),
				Valid: false,
			}

			// Occasionally create invalid records
			if i%10 == 0 {
				record.Name = ""
				record.Email = "invalid-email"
			}

			result, err := pipeline.Process(ctx, record)
			if err != nil {
				b.Fatal(err)
			}
			_ = result
		}
	})
}

// BenchmarkDynamicPipelines measures runtime modification performance.
func BenchmarkDynamicPipelines(b *testing.B) {
	ctx := context.Background()

	b.Run("Static_Pipeline", func(b *testing.B) {
		// Static pipeline for comparison
		pipeline := pipz.NewSequence("static",
			pipz.Transform("step1", func(_ context.Context, n ClonableInt) ClonableInt { return n + 1 }),
			pipz.Transform("step2", func(_ context.Context, n ClonableInt) ClonableInt { return n * 2 }),
			pipz.Transform("step3", func(_ context.Context, n ClonableInt) ClonableInt { return n - 5 }),
		)

		data := ClonableInt(42)

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			result, err := pipeline.Process(ctx, data)
			if err != nil {
				b.Fatal(err)
			}
			_ = result
		}
	})

	b.Run("Dynamic_Pipeline_No_Changes", func(b *testing.B) {
		// Dynamic pipeline but no actual changes during benchmark
		seq := pipz.NewSequence[ClonableInt]("dynamic")
		seq.Register(
			pipz.Transform("step1", func(_ context.Context, n ClonableInt) ClonableInt { return n + 1 }),
			pipz.Transform("step2", func(_ context.Context, n ClonableInt) ClonableInt { return n * 2 }),
			pipz.Transform("step3", func(_ context.Context, n ClonableInt) ClonableInt { return n - 5 }),
		)

		data := ClonableInt(42)

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			result, err := seq.Process(ctx, data)
			if err != nil {
				b.Fatal(err)
			}
			_ = result
		}
	})

	b.Run("Dynamic_Pipeline_With_Modifications", func(b *testing.B) {
		seq := pipz.NewSequence[ClonableInt]("dynamic-mod")
		seq.Register(pipz.Transform("step1", func(_ context.Context, n ClonableInt) ClonableInt { return n + 1 }))

		data := ClonableInt(42)

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			// Occasionally modify the pipeline
			if i%100 == 0 {
				if seq.Len() == 1 {
					seq.Register(pipz.Transform("step2", func(_ context.Context, n ClonableInt) ClonableInt { return n * 2 }))
				} else {
					_ = seq.Remove("step2") //nolint:errcheck // Return value ignored in benchmark
				}
			}

			result, err := seq.Process(ctx, data)
			if err != nil {
				b.Fatal(err)
			}
			_ = result
		}
	})

	b.Run("Pipeline_Modification_Operations", func(b *testing.B) {
		seq := pipz.NewSequence[ClonableInt]("mod-ops")

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			stepName := "step-" + strconv.Itoa(i%10)
			processor := pipz.Transform(pipz.Name(stepName), func(_ context.Context, n ClonableInt) ClonableInt { return n + 1 })

			switch i % 4 {
			case 0:
				seq.Register(processor)
			case 1:
				if seq.Len() > 0 {
					_ = seq.Remove(pipz.Name(stepName)) //nolint:errcheck // Return value ignored in benchmark
				}
			case 2:
				if seq.Len() > 0 {
					_ = seq.Replace(pipz.Name(stepName), processor) //nolint:errcheck // Return value ignored in benchmark
				}
			case 3:
				if seq.Len() > 1 {
					_ = seq.After(seq.Names()[0], processor) //nolint:errcheck // Return value ignored in benchmark
				}
			}
		}
	})
}

// BenchmarkScalabilityPatterns measures how different patterns scale with load.
func BenchmarkScalabilityPatterns(b *testing.B) {
	ctx := context.Background()

	// Test with different numbers of concurrent processors
	concurrencyLevels := []int{2, 4, 8, 16, 32}

	for _, concurrency := range concurrencyLevels {
		b.Run("Concurrent_"+strconv.Itoa(concurrency)+"_Processors", func(b *testing.B) {
			processors := make([]pipz.Chainable[ClonableInt], concurrency)
			for i := 0; i < concurrency; i++ {
				processors[i] = pipz.Transform(pipz.Name("proc"+strconv.Itoa(i)),
					func(_ context.Context, n ClonableInt) ClonableInt {
						// Add small delay to simulate work
						for j := 0; j < 100; j++ {
							n ^= ClonableInt(j) // Some CPU work
						}
						return n + 1
					})
			}

			pipeline := pipz.NewConcurrent("concurrent", processors...)
			data := ClonableInt(42)

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				result, err := pipeline.Process(ctx, data)
				if err != nil {
					b.Fatal(err)
				}
				_ = result
			}
		})
	}

	// Test with different numbers of fallback levels
	fallbackLevels := []int{1, 2, 4, 8}

	for _, levels := range fallbackLevels {
		b.Run("Fallback_"+strconv.Itoa(levels)+"_Levels", func(b *testing.B) {
			var pipeline pipz.Chainable[ClonableInt] = pipz.Apply("final", func(_ context.Context, n ClonableInt) (ClonableInt, error) {
				return n + 1, nil
			})

			// Build nested fallbacks
			for i := levels - 1; i >= 0; i-- {
				failing := pipz.Apply(pipz.Name("fail"+strconv.Itoa(i)), func(_ context.Context, _ ClonableInt) (ClonableInt, error) {
					return 0, errors.New("simulated failure")
				})
				pipeline = pipz.NewFallback(pipz.Name("fallback"+strconv.Itoa(i)), failing, pipeline)
			}

			data := ClonableInt(42)

			b.ResetTimer()
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				result, err := pipeline.Process(ctx, data)
				if err != nil {
					b.Fatal(err)
				}
				_ = result
			}
		})
	}
}
