package events

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/zoobzio/pipz"
)

func TestValidateSchema(t *testing.T) {
	tests := []struct {
		name      string
		event     Event
		wantError bool
		errorMsg  string
	}{
		{
			name: "Valid V1 event",
			event: Event{
				ID:        "test-1",
				Type:      EventUserCreated,
				Version:   "1.0",
				Timestamp: time.Now(),
				Data:      map[string]interface{}{"email": "test@example.com"},
			},
			wantError: false,
		},
		{
			name: "Missing ID",
			event: Event{
				Type:      EventUserCreated,
				Version:   "1.0",
				Timestamp: time.Now(),
			},
			wantError: true,
			errorMsg:  "event ID is required",
		},
		{
			name: "Missing required field for event type",
			event: Event{
				ID:        "test-2",
				Type:      EventUserCreated,
				Version:   "1.0",
				Timestamp: time.Now(),
				Data:      map[string]interface{}{}, // Missing email
			},
			wantError: true,
			errorMsg:  "email field",
		},
		{
			name: "V2 requires source",
			event: Event{
				ID:        "test-3",
				Type:      EventUserCreated,
				Version:   "2.0",
				Timestamp: time.Now(),
				Data:      map[string]interface{}{"email": "test@example.com"},
				// Missing Source
			},
			wantError: true,
			errorMsg:  "source field",
		},
		{
			name: "Valid V2 event",
			event: Event{
				ID:        "test-4",
				Type:      EventUserCreated,
				Version:   "2.0",
				Source:    "api",
				Timestamp: time.Now(),
				Data:      map[string]interface{}{"email": "test@example.com"},
			},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSchema(context.Background(), tt.event)
			if (err != nil) != tt.wantError {
				t.Errorf("ValidateSchema() error = %v, wantError %v", err, tt.wantError)
			}
			if err != nil && tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
				t.Errorf("Expected error containing '%s', got: %v", tt.errorMsg, err)
			}
		})
	}
}

func TestDeduplicateEvent(t *testing.T) {
	// Clear the deduplication cache
	processedEvents = &sync.Map{}

	event := Event{
		ID:        "dedup-test-1",
		Type:      EventUserCreated,
		Timestamp: time.Now(),
	}

	ctx := context.Background()

	// First call should succeed
	_, err := DeduplicateEvent(ctx, event)
	if err != nil {
		t.Errorf("First deduplication should succeed, got error: %v", err)
	}

	// Second call should fail
	_, err = DeduplicateEvent(ctx, event)
	if err == nil {
		t.Error("Second deduplication should fail")
	}
	if !strings.Contains(err.Error(), "already processed") {
		t.Errorf("Expected 'already processed' error, got: %v", err)
	}

	// Different event should succeed
	event2 := Event{
		ID:        "dedup-test-2",
		Type:      EventUserCreated,
		Timestamp: time.Now(),
	}
	_, err = DeduplicateEvent(ctx, event2)
	if err != nil {
		t.Errorf("Different event should not be deduplicated, got error: %v", err)
	}
}

func TestEnrichEvent(t *testing.T) {
	tests := []struct {
		name           string
		event          Event
		expectedFields map[string]interface{}
	}{
		{
			name: "User event enrichment",
			event: Event{
				Type: EventUserCreated,
				Data: map[string]interface{}{"email": "test@example.com"},
			},
			expectedFields: map[string]interface{}{
				"enriched":     true,
				"account_type": "standard",
			},
		},
		{
			name: "Order event with low total",
			event: Event{
				Type: EventOrderPlaced,
				Data: map[string]interface{}{"total": 50.0},
			},
			expectedFields: map[string]interface{}{
				"shipping_method": "standard",
			},
		},
		{
			name: "Order event with high total",
			event: Event{
				Type: EventOrderPlaced,
				Data: map[string]interface{}{"total": 150.0},
			},
			expectedFields: map[string]interface{}{
				"shipping_method": "express",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enriched := EnrichEvent(context.Background(), tt.event)

			// Check metadata was added
			if enriched.Metadata["processor_id"] == "" {
				t.Error("Expected processor_id in metadata")
			}

			// Check expected fields
			for field, expected := range tt.expectedFields {
				if actual := enriched.Data[field]; actual != expected {
					t.Errorf("Expected %s=%v, got %v", field, expected, actual)
				}
			}
		})
	}
}

func TestEventHandlers(t *testing.T) {
	ctx := context.Background()

	t.Run("User handler", func(t *testing.T) {
		events := []Event{
			{ID: "u1", Type: EventUserCreated, Data: map[string]interface{}{"email": "new@example.com"}},
			{ID: "u2", Type: EventUserUpdated},
			{ID: "u3", Type: EventUserDeleted},
		}

		for _, event := range events {
			result, err := HandleUserEvent(ctx, event)
			if err != nil {
				t.Errorf("HandleUserEvent() failed for %s: %v", event.Type, err)
			}
			if result.Status != "processed" {
				t.Errorf("Expected status 'processed', got %s", result.Status)
			}
		}

		// Test unknown event type
		unknownEvent := Event{Type: "user.unknown"}
		_, err := HandleUserEvent(ctx, unknownEvent)
		if err == nil {
			t.Error("Expected error for unknown user event type")
		}
	})

	t.Run("Order handler", func(t *testing.T) {
		// Test successful order
		order := Event{
			ID:   "o1",
			Type: EventOrderPlaced,
			Data: map[string]interface{}{"total": 99.99},
		}
		result, err := HandleOrderEvent(ctx, order)
		if err != nil {
			t.Errorf("HandleOrderEvent() failed: %v", err)
		}
		if result.Status != "processed" {
			t.Errorf("Expected status 'processed', got %s", result.Status)
		}

		// Test error case
		errorOrder := Event{
			ID:   "error-test",
			Type: EventOrderPlaced,
			Data: map[string]interface{}{"total": 50.0},
		}
		_, err = HandleOrderEvent(ctx, errorOrder)
		if err == nil {
			t.Error("Expected error for error-test order")
		}
	})
}

func TestCreateEventPipeline(t *testing.T) {
	pipeline := CreateEventPipeline()
	ctx := context.Background()

	// Clear state
	processedEvents = &sync.Map{}
	eventMetrics = &EventMetrics{
		Processed: make(map[string]int64),
		Failed:    make(map[string]int64),
		mu:        &sync.RWMutex{},
	}

	t.Run("Successful user event", func(t *testing.T) {
		event := Event{
			ID:        "pipeline-test-1",
			Type:      EventUserCreated,
			Version:   "1.0",
			Timestamp: time.Now(),
			Data:      map[string]interface{}{"email": "test@example.com"},
		}

		result, err := pipeline.Process(ctx, event)
		if err != nil {
			t.Fatalf("Pipeline failed: %v", err)
		}

		if result.Status != "processed" {
			t.Errorf("Expected status 'processed', got %s", result.Status)
		}

		// Check enrichment
		if result.Data["enriched"] != true {
			t.Error("Expected event to be enriched")
		}

		// Check audit log
		found := false
		auditLog.Range(func(key, value interface{}) bool {
			if entry, ok := value.(map[string]interface{}); ok {
				if entry["event_id"] == event.ID {
					found = true
					return false
				}
			}
			return true
		})
		if !found {
			t.Error("Event not found in audit log")
		}
	})

	t.Run("Duplicate event rejected", func(t *testing.T) {
		event := Event{
			ID:        "pipeline-test-1", // Same ID as above
			Type:      EventUserCreated,
			Version:   "1.0",
			Timestamp: time.Now(),
			Data:      map[string]interface{}{"email": "test@example.com"},
		}

		_, err := pipeline.Process(ctx, event)
		if err == nil {
			t.Error("Expected error for duplicate event")
		}
		if !strings.Contains(err.Error(), "already processed") {
			t.Errorf("Expected 'already processed' error, got: %v", err)
		}
	})

	t.Run("Invalid event rejected", func(t *testing.T) {
		event := Event{
			// Missing required fields
			Type: EventUserCreated,
		}

		_, err := pipeline.Process(ctx, event)
		if err == nil {
			t.Error("Expected error for invalid event")
		}
	})

	t.Run("Unknown event type", func(t *testing.T) {
		event := Event{
			ID:        "unknown-test",
			Type:      "unknown.event",
			Version:   "1.0",
			Timestamp: time.Now(),
		}

		_, err := pipeline.Process(ctx, event)
		if err == nil {
			t.Error("Expected error for unknown event type")
		}
	})
}

func TestBatchEventPipeline(t *testing.T) {
	pipeline := CreateBatchEventPipeline()
	ctx := context.Background()

	// Clear state
	processedEvents = &sync.Map{}

	events := []Event{
		{
			ID:        "batch-1",
			Type:      EventUserCreated,
			Version:   "1.0",
			Timestamp: time.Now(),
			Data:      map[string]interface{}{"email": "user1@example.com"},
		},
		{
			// Invalid event (missing ID)
			Type:      EventUserCreated,
			Version:   "1.0",
			Timestamp: time.Now(),
		},
		{
			ID:        "batch-3",
			Type:      EventOrderPlaced,
			Version:   "1.0",
			Timestamp: time.Now(),
			Data:      map[string]interface{}{"total": 75.0},
		},
	}

	processed, err := pipeline.Process(ctx, events)
	if err != nil {
		t.Fatalf("Batch pipeline failed: %v", err)
	}

	// Should process 2 out of 3 events (one is invalid)
	if len(processed) != 2 {
		t.Errorf("Expected 2 processed events, got %d", len(processed))
	}

	// Check that invalid event went to DLQ
	dlqCount := 0
	deadLetterQueue.Range(func(key, value interface{}) bool {
		dlqCount++
		return true
	})
	if dlqCount == 0 {
		t.Error("Expected invalid event in dead letter queue")
	}
}

func TestPriorityEventPipeline(t *testing.T) {
	pipeline := CreatePriorityEventPipeline()
	ctx := context.Background()

	tests := []struct {
		name       string
		event      Event
		expectFast bool
	}{
		{
			name: "Payment event (high priority)",
			event: Event{
				ID:        "priority-1",
				Type:      EventPaymentFailed,
				Version:   "1.0",
				Timestamp: time.Now(),
			},
			expectFast: true,
		},
		{
			name: "VIP customer event",
			event: Event{
				ID:        "priority-2",
				Type:      EventOrderPlaced,
				Version:   "1.0",
				Timestamp: time.Now(),
				Data: map[string]interface{}{
					"total":  100.0,
					"is_vip": true,
				},
			},
			expectFast: true,
		},
		{
			name: "Normal event",
			event: Event{
				ID:        "priority-3",
				Type:      EventUserUpdated,
				Version:   "1.0",
				Timestamp: time.Now(),
			},
			expectFast: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear dedup cache for each test
			processedEvents.Delete(tt.event.ID + ":" + tt.event.Type)

			start := time.Now()
			_, err := pipeline.Process(ctx, tt.event)
			duration := time.Since(start)

			if err != nil {
				t.Errorf("Pipeline failed: %v", err)
			}

			// High priority should complete faster due to tighter timeout
			// This is a simplified check - in real tests you'd mock the handlers
			t.Logf("%s completed in %v", tt.name, duration)
		})
	}
}

func TestEventMetrics(t *testing.T) {
	// Reset metrics
	eventMetrics = &EventMetrics{
		Processed: make(map[string]int64),
		Failed:    make(map[string]int64),
		mu:        &sync.RWMutex{},
	}

	pipeline := CreateEventPipeline()
	ctx := context.Background()

	// Process some events
	events := []Event{
		{ID: "m1", Type: EventUserCreated, Version: "1.0", Timestamp: time.Now(),
			Data: map[string]interface{}{"email": "test@example.com"}},
		{ID: "m2", Type: EventOrderPlaced, Version: "1.0", Timestamp: time.Now(),
			Data: map[string]interface{}{"total": 50.0}},
		{ID: "m3", Type: EventUserCreated, Version: "1.0", Timestamp: time.Now(),
			Data: map[string]interface{}{"email": "test2@example.com"}},
	}

	for _, event := range events {
		// Clear dedup for testing
		processedEvents.Delete(event.ID + ":" + event.Type)
		pipeline.Process(ctx, event)
	}

	// Check metrics
	metrics := GetMetrics()

	if total, ok := metrics["total_processed"].(int64); !ok || total != 3 {
		t.Errorf("Expected 3 processed events, got %v", metrics["total_processed"])
	}

	if rate, ok := metrics["success_rate"].(float64); !ok || rate != 100.0 {
		t.Errorf("Expected 100%% success rate, got %v", metrics["success_rate"])
	}

	// Check breakdown by type
	if byType, ok := metrics["by_type"].(map[string]interface{}); ok {
		if processed, ok := byType["processed"].(map[string]int64); ok {
			if processed[EventUserCreated] != 2 {
				t.Errorf("Expected 2 user.created events, got %d", processed[EventUserCreated])
			}
			if processed[EventOrderPlaced] != 1 {
				t.Errorf("Expected 1 order.placed event, got %d", processed[EventOrderPlaced])
			}
		}
	}
}

// Mock a slow handler for timeout testing
func slowHandler(ctx context.Context, event Event) (Event, error) {
	select {
	case <-time.After(10 * time.Second):
		return event, nil
	case <-ctx.Done():
		return event, ctx.Err()
	}
}

func TestPipelineTimeout(t *testing.T) {
	// Create a custom pipeline with a slow handler
	pipeline := pipz.Sequential(
		pipz.Validate("validate", ValidateSchema),
		pipz.Timeout(
			pipz.Apply("slow_handler", slowHandler),
			100*time.Millisecond, // Short timeout
		),
	)

	event := Event{
		ID:        "timeout-test",
		Type:      EventUserCreated,
		Version:   "1.0",
		Timestamp: time.Now(),
		Data:      map[string]interface{}{"email": "test@example.com"},
	}

	ctx := context.Background()
	_, err := pipeline.Process(ctx, event)

	if err == nil {
		t.Error("Expected timeout error")
	}
	if !strings.Contains(err.Error(), "timeout") {
		t.Errorf("Expected timeout error, got: %v", err)
	}
}
