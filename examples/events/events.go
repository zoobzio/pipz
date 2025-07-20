// Package events demonstrates using pipz for high-throughput event processing
// with routing, deduplication, and error handling.
package events

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/zoobzio/pipz"
)

// Event represents a generic event in the system
type Event struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"`    // e.g., "user.created", "order.placed"
	Source    string                 `json:"source"`  // e.g., "api", "webhook", "internal"
	Version   string                 `json:"version"` // Schema version
	Timestamp time.Time              `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`     // Event payload
	Metadata  map[string]string      `json:"metadata"` // Headers, correlation IDs, etc.

	// Processing context
	Attempts    int       `json:"attempts"`
	ProcessedAt time.Time `json:"processed_at,omitempty"`
	Status      string    `json:"status"` // "pending", "processed", "failed"
	Error       string    `json:"error,omitempty"`
}

// Common event types
const (
	EventUserCreated    = "user.created"
	EventUserUpdated    = "user.updated"
	EventUserDeleted    = "user.deleted"
	EventOrderPlaced    = "order.placed"
	EventOrderCancelled = "order.cancelled"
	EventPaymentFailed  = "payment.failed"
	EventPaymentSuccess = "payment.success"
)

// EventDeduplicator handles event deduplication
type EventDeduplicator struct {
	processed map[string]time.Time
	mu        sync.RWMutex
}

func NewEventDeduplicator() *EventDeduplicator {
	return &EventDeduplicator{
		processed: make(map[string]time.Time),
	}
}

func (d *EventDeduplicator) IsProcessed(eventID, eventType string) (time.Time, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	key := fmt.Sprintf("%s:%s", eventID, eventType)
	t, exists := d.processed[key]
	return t, exists
}

func (d *EventDeduplicator) MarkProcessed(eventID, eventType string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	key := fmt.Sprintf("%s:%s", eventID, eventType)
	d.processed[key] = time.Now()
}

// Processing results storage (in production, use proper storage)
var (
	deduplicator    = NewEventDeduplicator()
	auditLog        = &sync.Map{} // Audit trail
	deadLetterQueue = &sync.Map{} // Failed events
	eventMetrics    = &EventMetrics{
		Processed: make(map[string]int64),
		Failed:    make(map[string]int64),
		mu:        &sync.RWMutex{},
	}
)

// EventMetrics tracks processing statistics
type EventMetrics struct {
	Processed map[string]int64
	Failed    map[string]int64
	mu        *sync.RWMutex
}

// Event processors

// ValidateSchema ensures the event has required fields and valid structure
func ValidateSchema(_ context.Context, event Event) error {
	// Basic validation
	if event.ID == "" {
		return errors.New("event ID is required")
	}
	if event.Type == "" {
		return errors.New("event type is required")
	}
	if event.Timestamp.IsZero() {
		return errors.New("event timestamp is required")
	}

	// Version-specific validation
	switch event.Version {
	case "1.0":
		return validateV1Schema(event)
	case "2.0":
		return validateV2Schema(event)
	default:
		return fmt.Errorf("unsupported event version: %s", event.Version)
	}
}

func validateV1Schema(event Event) error {
	// V1 requires certain fields based on event type
	switch event.Type {
	case EventUserCreated:
		if _, ok := event.Data["email"]; !ok {
			return errors.New("user.created event requires email field")
		}
	case EventOrderPlaced:
		if _, ok := event.Data["total"]; !ok {
			return errors.New("order.placed event requires total field")
		}
	}
	return nil
}

func validateV2Schema(event Event) error {
	// V2 has stricter requirements
	if event.Source == "" {
		return errors.New("v2 events require source field")
	}
	return validateV1Schema(event) // V2 includes V1 validation
}

// DeduplicateEvent prevents processing the same event multiple times
func DeduplicateEvent(_ context.Context, event Event) (Event, error) {
	// Check if already processed
	if processedTime, exists := deduplicator.IsProcessed(event.ID, event.Type); exists {
		return event, fmt.Errorf("event already processed at %v", processedTime)
	}

	// Mark as processed
	deduplicator.MarkProcessed(event.ID, event.Type)

	return event, nil
}

// EnrichEvent adds additional context to events
func EnrichEvent(_ context.Context, event Event) Event {
	// Add processing metadata
	if event.Metadata == nil {
		event.Metadata = make(map[string]string)
	}

	event.Metadata["processor_id"] = "pipz-event-processor"
	event.Metadata["processed_region"] = "us-west-2"
	event.Metadata["pipeline_version"] = "1.0.0"

	// Enrich based on event type
	switch event.Type {
	case EventUserCreated:
		// Could fetch user details from database
		event.Data["enriched"] = true
		event.Data["account_type"] = "standard"

	case EventOrderPlaced:
		// Could calculate shipping estimates
		if total, ok := event.Data["total"].(float64); ok {
			event.Data["shipping_method"] = "standard"
			if total > 100 {
				event.Data["shipping_method"] = "express"
			}
		}
	}

	return event
}

// Event handlers for different event types

// HandleUserEvent processes user-related events
func HandleUserEvent(_ context.Context, event Event) (Event, error) {
	switch event.Type {
	case EventUserCreated:
		// Simulate user creation processing
		fmt.Printf("[USER] Creating user account for %v\n", event.Data["email"])
		// Could trigger welcome email, create profile, etc.

	case EventUserUpdated:
		fmt.Printf("[USER] Updating user %s\n", event.ID)
		// Could sync with external systems

	case EventUserDeleted:
		fmt.Printf("[USER] Deleting user %s\n", event.ID)
		// Could trigger cleanup workflows

	default:
		return event, fmt.Errorf("unknown user event type: %s", event.Type)
	}

	event.Status = "processed"
	event.ProcessedAt = time.Now()
	return event, nil
}

// HandleOrderEvent processes order-related events
func HandleOrderEvent(_ context.Context, event Event) (Event, error) {
	switch event.Type {
	case EventOrderPlaced:
		fmt.Printf("[ORDER] Processing order %s for $%.2f\n", event.ID, event.Data["total"])
		// Could trigger inventory check, payment processing, etc.

		// Simulate occasional failures
		if event.ID == "error-test" {
			return event, errors.New("order processing failed")
		}

	case EventOrderCancelled:
		fmt.Printf("[ORDER] Cancelling order %s\n", event.ID)
		// Could trigger refunds, restock inventory, etc.

	default:
		return event, fmt.Errorf("unknown order event type: %s", event.Type)
	}

	event.Status = "processed"
	event.ProcessedAt = time.Now()
	return event, nil
}

// HandlePaymentEvent processes payment-related events
func HandlePaymentEvent(_ context.Context, event Event) (Event, error) {
	switch event.Type {
	case EventPaymentFailed:
		fmt.Printf("[PAYMENT] Payment failed for order %s\n", event.Data["order_id"])
		// Could trigger retry logic, customer notification

	case EventPaymentSuccess:
		fmt.Printf("[PAYMENT] Payment successful for order %s\n", event.Data["order_id"])
		// Could trigger fulfillment workflow

	default:
		return event, fmt.Errorf("unknown payment event type: %s", event.Type)
	}

	event.Status = "processed"
	event.ProcessedAt = time.Now()
	return event, nil
}

// LogAuditTrail records all events for compliance
func LogAuditTrail(_ context.Context, event Event) error {
	auditEntry := map[string]interface{}{
		"event_id":     event.ID,
		"event_type":   event.Type,
		"processed_at": time.Now(),
		"status":       event.Status,
		"metadata":     event.Metadata,
	}

	auditKey := fmt.Sprintf("%s:%d", event.ID, time.Now().UnixNano())
	auditLog.Store(auditKey, auditEntry)

	// Update metrics
	eventMetrics.mu.Lock()
	if event.Status == "processed" {
		eventMetrics.Processed[event.Type]++
	} else {
		eventMetrics.Failed[event.Type]++
	}
	eventMetrics.mu.Unlock()

	return nil
}

// SendToDeadLetter stores failed events for manual review
func SendToDeadLetter(_ context.Context, event Event, err error) {
	event.Status = "failed"
	event.Error = err.Error()
	event.ProcessedAt = time.Now()

	dlqKey := fmt.Sprintf("%s:%d", event.ID, time.Now().UnixNano())
	deadLetterQueue.Store(dlqKey, event)

	fmt.Printf("[DLQ] Event %s sent to dead letter queue: %v\n", event.ID, err)
}

// CreateEventPipeline creates the main event processing pipeline
func CreateEventPipeline() pipz.Chainable[Event] {
	// Define routing logic based on event type
	routeByEventType := func(_ context.Context, event Event) string {
		// Extract major event category
		parts := strings.Split(event.Type, ".")
		if len(parts) > 0 {
			return parts[0] // "user", "order", "payment"
		}
		return "unknown"
	}

	// Create specialized handlers for each event category
	userHandler := pipz.Sequential(
		pipz.Apply("handle_user", HandleUserEvent),
		pipz.Effect("notify_user_service", func(_ context.Context, e Event) error {
			// Could publish to user service message queue
			return nil
		}),
	)

	orderHandler := pipz.Sequential(
		pipz.Apply("handle_order", HandleOrderEvent),
		pipz.Effect("update_inventory", func(_ context.Context, e Event) error {
			// Could update inventory system
			return nil
		}),
	)

	paymentHandler := pipz.RetryWithBackoff(
		pipz.Apply("handle_payment", HandlePaymentEvent),
		3,           // Retry payment events up to 3 times
		time.Second, // Start with 1 second backoff
	)

	// Build the main pipeline
	return pipz.Sequential(
		// Validation and deduplication
		pipz.Effect("validate_schema", ValidateSchema),
		pipz.Apply("deduplicate", DeduplicateEvent),
		pipz.Transform("enrich", EnrichEvent),

		// Route to appropriate handler with timeout protection
		pipz.Timeout(
			pipz.Switch(routeByEventType, map[string]pipz.Chainable[Event]{
				"user":    userHandler,
				"order":   orderHandler,
				"payment": paymentHandler,
				"default": pipz.Apply("unknown_handler", func(_ context.Context, e Event) (Event, error) {
					return e, fmt.Errorf("no handler for event type: %s", e.Type)
				}),
			}),
			5*time.Second, // 5 second timeout for all handlers
		),

		// Audit logging (always runs, even on failure)
		pipz.Effect("audit_log", LogAuditTrail),
	)
}

// CreateBatchEventPipeline processes events in batches for efficiency
func CreateBatchEventPipeline() pipz.Chainable[[]Event] {
	return pipz.Sequential(
		// Validate all events in batch
		pipz.Apply("validate_batch", func(ctx context.Context, events []Event) ([]Event, error) {
			validEvents := make([]Event, 0, len(events))
			for _, event := range events {
				if err := ValidateSchema(ctx, event); err != nil {
					SendToDeadLetter(ctx, event, err)
					continue
				}
				validEvents = append(validEvents, event)
			}
			return validEvents, nil
		}),

		// Process events individually but track batch metrics
		pipz.Apply("process_batch", func(ctx context.Context, events []Event) ([]Event, error) {
			pipeline := CreateEventPipeline()
			processed := make([]Event, 0, len(events))

			for _, event := range events {
				result, err := pipeline.Process(ctx, event)
				if err != nil {
					SendToDeadLetter(ctx, event, err)
					continue
				}
				processed = append(processed, result)
			}

			fmt.Printf("[BATCH] Processed %d/%d events successfully\n",
				len(processed), len(events))
			return processed, nil
		}),
	)
}

// CreatePriorityEventPipeline handles high-priority events differently
func CreatePriorityEventPipeline() pipz.Chainable[Event] {
	// Determine event priority
	checkPriority := func(_ context.Context, event Event) string {
		// Payment events are always high priority
		if strings.HasPrefix(event.Type, "payment.") {
			return "high"
		}

		// Check metadata for priority flag
		if priority, ok := event.Metadata["priority"]; ok {
			return priority
		}

		// Check data fields for VIP customers
		if vip, ok := event.Data["is_vip"].(bool); ok && vip {
			return "high"
		}

		return "normal"
	}

	// High priority pipeline - no retries, fail fast
	highPriorityPipeline := pipz.Sequential(
		pipz.Effect("validate_schema", ValidateSchema),
		pipz.Transform("enrich", EnrichEvent),
		pipz.Timeout(
			CreateEventPipeline(),
			1*time.Second, // Tight timeout for high priority
		),
	)

	// Normal priority pipeline - with retries
	normalPriorityPipeline := CreateEventPipeline()

	return pipz.Switch(checkPriority, map[string]pipz.Chainable[Event]{
		"high":   highPriorityPipeline,
		"normal": normalPriorityPipeline,
	})
}

// Utility functions

// GetMetrics returns current processing metrics
func GetMetrics() map[string]interface{} {
	eventMetrics.mu.RLock()
	defer eventMetrics.mu.RUnlock()

	totalProcessed := int64(0)
	totalFailed := int64(0)

	for _, count := range eventMetrics.Processed {
		totalProcessed += count
	}
	for _, count := range eventMetrics.Failed {
		totalFailed += count
	}

	return map[string]interface{}{
		"total_processed": totalProcessed,
		"total_failed":    totalFailed,
		"by_type": map[string]interface{}{
			"processed": eventMetrics.Processed,
			"failed":    eventMetrics.Failed,
		},
		"success_rate": float64(totalProcessed) / float64(totalProcessed+totalFailed) * 100,
	}
}

// GenerateEventID creates a unique event ID
func GenerateEventID(eventType string) string {
	data := fmt.Sprintf("%s:%d:%s", eventType, time.Now().UnixNano(), randomString(8))
	hash := md5.Sum([]byte(data))
	return hex.EncodeToString(hash[:])[:16]
}

func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
	}
	return string(b)
}
