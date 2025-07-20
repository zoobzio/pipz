package main

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/zoobzio/pipz"
	"github.com/zoobzio/pipz/examples/events"
)

// EventsExample implements the Example interface for event processing
type EventsExample struct{}

func (e *EventsExample) Name() string {
	return "events"
}

func (e *EventsExample) Description() string {
	return "Event routing and deduplication with real-time processing"
}

func (e *EventsExample) Demo(ctx context.Context) error {
	fmt.Println("\n" + colorCyan + "═══ EVENT PROCESSING EXAMPLE ═══" + colorReset)
	fmt.Println("\nThis example demonstrates:")
	fmt.Println("• Event routing based on type")
	fmt.Println("• Deduplication to prevent duplicate processing")
	fmt.Println("• Schema validation and enrichment")
	fmt.Println("• Dead letter queue for failed events")
	fmt.Println("• Audit logging and metrics")

	waitForEnter()

	// Show the problem
	fmt.Println("\n" + colorYellow + "The Problem:" + colorReset)
	fmt.Println("Event processing systems quickly become complex:")
	fmt.Println(colorGray + `
func processEvent(event Event) error {
    // Check if we've seen this event
    dedupKey := fmt.Sprintf("%s:%s", event.Source, event.ID)
    if seen, _ := cache.Get(dedupKey); seen != nil {
        metrics.Inc("events.duplicate")
        return nil // Skip duplicates
    }
    
    // Store in dedup cache
    cache.Set(dedupKey, true, 24*time.Hour)
    
    // Route based on event type
    switch event.Type {
    case "user.created", "user.updated":
        if err := handleUserEvent(event); err != nil {
            metrics.Inc("events.user.failed")
            if isRetryable(err) {
                return fmt.Errorf("retryable: %w", err)
            }
            sendToDeadLetter(event, err)
            return err
        }
        metrics.Inc("events.user.success")
        
    case "order.placed", "order.shipped":
        if err := handleOrderEvent(event); err != nil {
            metrics.Inc("events.order.failed")
            // Similar error handling...
        }
        
    default:
        metrics.Inc("events.unknown")
        return sendToDeadLetter(event, errors.New("unknown type"))
    }
    
    return nil
}` + colorReset)

	waitForEnter()

	// Show the solution
	fmt.Println("\n" + colorGreen + "The pipz Solution:" + colorReset)
	fmt.Println("Use the real events example with proper types:")
	fmt.Println(colorGray + `
// Import the real events package
import "github.com/zoobzio/pipz/examples/events"

// Use the actual Event type and processors
eventPipeline := events.CreateEventPipeline()

// Or for batch processing
batchPipeline := events.CreateBatchEventPipeline()

// Process events with full deduplication and routing
result, err := eventPipeline.Process(ctx, event)` + colorReset)

	waitForEnter()

	// Run interactive demo
	fmt.Println("\n" + colorPurple + "Let's process some events!" + colorReset)
	return e.runInteractive(ctx)
}

func (e *EventsExample) runInteractive(ctx context.Context) error {
	// Create pipelines using the real events example
	eventPipeline := events.CreateEventPipeline()
	batchPipeline := events.CreateBatchEventPipeline()
	priorityPipeline := events.CreatePriorityEventPipeline()

	// Test scenarios
	scenarios := []struct {
		name    string
		events  []events.Event
		useBatch bool
		usePriority bool
	}{
		{
			name: "User Registration Flow",
			events: []events.Event{
				{
					ID:        events.GenerateEventID("user.created"),
					Type:      events.EventUserCreated,
					Source:    "auth-service",
					Version:   "1.0",
					Timestamp: time.Now(),
					Data: map[string]interface{}{
						"user_id": "user_123",
						"email":   "newuser@example.com",
						"plan":    "premium",
					},
					Metadata: map[string]string{
						"correlation_id": "reg_flow_001",
						"user_agent":     "mobile_app",
					},
					Status: "pending",
				},
				{
					ID:        events.GenerateEventID("user.updated"),
					Type:      events.EventUserUpdated,
					Source:    "profile-service",
					Version:   "1.0",
					Timestamp: time.Now().Add(1 * time.Second),
					Data: map[string]interface{}{
						"user_id": "user_123",
						"field":   "email_verified",
						"value":   true,
					},
					Metadata: map[string]string{
						"correlation_id": "reg_flow_001",
					},
					Status: "pending",
				},
			},
		},
		{
			name: "Duplicate Event (Deduplication Test)",
			events: []events.Event{
				{
					ID:        "duplicate_test_001", // Use fixed ID for testing
					Type:      events.EventUserCreated,
					Source:    "auth-service",
					Version:   "1.0",
					Timestamp: time.Now(),
					Data: map[string]interface{}{
						"user_id": "user_456",
						"email":   "duplicate@example.com",
					},
					Status: "pending",
				},
				{
					ID:        "duplicate_test_001", // Same ID - should be deduplicated
					Type:      events.EventUserCreated,
					Source:    "auth-service",
					Version:   "1.0",
					Timestamp: time.Now().Add(500 * time.Millisecond),
					Data: map[string]interface{}{
						"user_id": "user_456",
						"email":   "duplicate@example.com",
					},
					Status: "pending",
				},
			},
		},
		{
			name: "Order Processing Chain",
			events: []events.Event{
				{
					ID:        events.GenerateEventID("order.placed"),
					Type:      events.EventOrderPlaced,
					Source:    "order-service",
					Version:   "1.0",
					Timestamp: time.Now(),
					Data: map[string]interface{}{
						"order_id":    "order_789",
						"customer_id": "user_123",
						"amount":      299.99,
						"currency":    "USD",
					},
					Metadata: map[string]string{
						"correlation_id": "order_flow_001",
					},
					Status: "pending",
				},
				{
					ID:        events.GenerateEventID("payment.success"),
					Type:      events.EventPaymentSuccess,
					Source:    "payment-service",
					Version:   "1.0",
					Timestamp: time.Now().Add(2 * time.Second),
					Data: map[string]interface{}{
						"order_id":       "order_789",
						"payment_id":     "pay_123",
						"amount":         299.99,
						"payment_method": "card",
					},
					Metadata: map[string]string{
						"correlation_id": "order_flow_001",
					},
					Status: "pending",
				},
			},
		},
		{
			name:     "Batch Processing Test",
			useBatch: true,
			events: []events.Event{
				{
					ID:        events.GenerateEventID("user.created"),
					Type:      events.EventUserCreated,
					Source:    "batch-import",
					Version:   "1.0",
					Timestamp: time.Now(),
					Data:      map[string]interface{}{"user_id": "batch_user_1"},
					Status:    "pending",
				},
				{
					ID:        events.GenerateEventID("user.created"),
					Type:      events.EventUserCreated,
					Source:    "batch-import",
					Version:   "1.0",
					Timestamp: time.Now(),
					Data:      map[string]interface{}{"user_id": "batch_user_2"},
					Status:    "pending",
				},
				{
					ID:        events.GenerateEventID("user.created"),
					Type:      events.EventUserCreated,
					Source:    "batch-import",
					Version:   "1.0",
					Timestamp: time.Now(),
					Data:      map[string]interface{}{"user_id": "batch_user_3"},
					Status:    "pending",
				},
			},
		},
		{
			name:        "Priority Processing",
			usePriority: true,
			events: []events.Event{
				{
					ID:        events.GenerateEventID("payment.failed"),
					Type:      events.EventPaymentFailed,
					Source:    "payment-service",
					Version:   "1.0",
					Timestamp: time.Now(),
					Data: map[string]interface{}{
						"order_id": "urgent_order_001",
						"reason":   "insufficient_funds",
					},
					Metadata: map[string]string{
						"priority": "high",
					},
					Status: "pending",
				},
			},
		},
	}

	// Process scenarios
	for i, scenario := range scenarios {
		fmt.Printf("\n%s═══ Scenario %d: %s ═══%s\n",
			colorWhite, i+1, scenario.name, colorReset)

		if scenario.useBatch {
			// Batch processing
			fmt.Printf("\nProcessing %d events as a batch:\n", len(scenario.events))
			for j, event := range scenario.events {
				fmt.Printf("  %d. %s (ID: %s)\n", j+1, event.Type, event.ID)
			}

			fmt.Printf("\n%sProcessing batch...%s\n", colorYellow, colorReset)
			start := time.Now()

			result, err := batchPipeline.Process(ctx, scenario.events)
			duration := time.Since(start)

			if err != nil {
				fmt.Printf("\n%s❌ Batch Processing Failed%s\n", colorRed, colorReset)
				fmt.Printf("  Error: %s\n", err.Error())
			} else {
				fmt.Printf("\n%s✅ Batch Processed Successfully%s\n", colorGreen, colorReset)
				fmt.Printf("  Events processed: %d\n", len(result))
				fmt.Printf("  Processing time: %v\n", duration)
				fmt.Printf("  Throughput: %.0f events/sec\n", 
					float64(len(result))/duration.Seconds())
			}
		} else {
			// Single event processing
			var currentPipeline pipz.Chainable[events.Event]
			if scenario.usePriority {
				currentPipeline = priorityPipeline
				fmt.Printf("\nUsing priority processing pipeline\n")
			} else {
				currentPipeline = eventPipeline
			}

			for j, event := range scenario.events {
				fmt.Printf("\nProcessing Event %d: %s from %s (ID: %s)\n", 
					j+1, event.Type, event.Source, event.ID)

				start := time.Now()
				result, err := currentPipeline.Process(ctx, event)
				duration := time.Since(start)

				if err != nil {
					if strings.Contains(err.Error(), "duplicate") {
						fmt.Printf("  %s⚠️  Duplicate detected: %s%s\n", 
							colorYellow, event.ID, colorReset)
					} else {
						fmt.Printf("  %s❌ Failed: %s%s\n", 
							colorRed, err.Error(), colorReset)
					}
				} else {
					fmt.Printf("  %s✅ Processed: %s → %s (%v)%s\n",
						colorGreen, event.Type, result.Status, duration, colorReset)
					
					// Show enrichment
					if result.Metadata["processed_by"] != "" {
						fmt.Printf("    %s→ Enriched with processor: %s%s\n",
							colorGray, result.Metadata["processed_by"], colorReset)
					}
				}

				if j < len(scenario.events)-1 {
					time.Sleep(200 * time.Millisecond) // Small delay between events
				}
			}
		}

		if i < len(scenarios)-1 {
			waitForEnter()
		}
	}

	// Show statistics using the real metrics from the events example
	fmt.Printf("\n%s═══ EVENT PROCESSING STATISTICS ═══%s\n", colorCyan, colorReset)
	metrics := events.GetMetrics()
	
	if processed, ok := metrics["processed"].(map[string]int64); ok {
		fmt.Printf("\nProcessed Events by Type:\n")
		for eventType, count := range processed {
			fmt.Printf("  %s: %d\n", eventType, count)
		}
	}
	
	if failed, ok := metrics["failed"].(map[string]int64); ok && len(failed) > 0 {
		fmt.Printf("\nFailed Events by Type:\n")
		for eventType, count := range failed {
			fmt.Printf("  %s: %d\n", eventType, count)
		}
	}
	
	if total, ok := metrics["total_processed"].(int64); ok {
		fmt.Printf("\nTotal Events Processed: %d\n", total)
	}

	return nil
}

func (e *EventsExample) Benchmark(b *testing.B) error {
	return nil
}