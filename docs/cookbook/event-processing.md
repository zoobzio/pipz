# Recipe: Event Processing System

Build scalable event processing pipelines with routing, enrichment, and error handling.

## Problem

You need to process events from multiple sources with:
- Different event types requiring different processing
- Event enrichment from multiple data sources
- Guaranteed delivery to multiple destinations
- Dead letter queue for failed events
- Real-time and batch processing modes

## Solution

Create a flexible event processing system:

```go
// Event structure
type Event struct {
    ID        string                 `json:"id"`
    Type      string                 `json:"type"`
    Source    string                 `json:"source"`
    Timestamp time.Time              `json:"timestamp"`
    Data      map[string]interface{} `json:"data"`
    Metadata  map[string]string      `json:"metadata"`
}

// Implement Cloner for parallel processing
func (e Event) Clone() Event {
    data := make(map[string]interface{}, len(e.Data))
    for k, v := range e.Data {
        data[k] = v
    }
    
    metadata := make(map[string]string, len(e.Metadata))
    for k, v := range e.Metadata {
        metadata[k] = v
    }
    
    return Event{
        ID:        e.ID,
        Type:      e.Type,
        Source:    e.Source,
        Timestamp: e.Timestamp,
        Data:      data,
        Metadata:  metadata,
    }
}

// Main event processing pipeline
func createEventProcessor() pipz.Chainable[Event] {
    return pipz.NewSequence[Event]("event-processor",
        // 1. Validate event structure
        pipz.Apply("validate", validateEvent),
        
        // 2. Enrich with context
        pipz.Transform("add-metadata", func(ctx context.Context, e Event) Event {
            e.Metadata["processed_at"] = time.Now().Format(time.RFC3339)
            e.Metadata["processor_version"] = "1.0.0"
            if traceID := ctx.Value("trace_id"); traceID != nil {
                e.Metadata["trace_id"] = traceID.(string)
            }
            return e
        }),
        
        // 3. Route by event type
        pipz.Switch[Event]("route-by-type",
            func(ctx context.Context, e Event) string {
                return e.Type
            },
        ).
        AddRoute("user.created", processUserCreated).
        AddRoute("order.placed", processOrderPlaced).
        AddRoute("payment.received", processPaymentReceived).
        AddDefault(processUnknownEvent),
        
        // 4. Distribute to destinations
        pipz.NewConcurrent[Event]("distribute",
            pipz.Effect("kafka", publishToKafka),
            pipz.Effect("database", saveToDatabase),
            pipz.Effect("analytics", sendToAnalytics),
            pipz.Scaffold("notifications",  // Fire-and-forget
                pipz.Effect("email", sendEmailNotification),
                pipz.Effect("slack", sendSlackNotification),
            ),
        ),
    )
}
```

## Event Type Processors

```go
// User created event processing
var processUserCreated = pipz.NewSequence[Event]("user-created",
    pipz.Apply("extract-user", func(ctx context.Context, e Event) (Event, error) {
        // Validate user data exists
        if _, ok := e.Data["user_id"]; !ok {
            return e, errors.New("missing user_id")
        }
        return e, nil
    }),
    
    pipz.Enrich("user-enrichment", func(ctx context.Context, e Event) (Event, error) {
        // Enrich with user profile
        userID := e.Data["user_id"].(string)
        profile, err := userService.GetProfile(ctx, userID)
        if err != nil {
            log.Printf("Failed to enrich user %s: %v", userID, err)
            return e, err // Enrich logs but doesn't fail
        }
        e.Data["profile"] = profile
        return e, nil
    }),
    
    pipz.NewConcurrent[Event]("user-workflows",
        pipz.Effect("crm", updateCRM),
        pipz.Effect("welcome", sendWelcomeEmail),
        pipz.Effect("onboarding", scheduleOnboarding),
    ),
)

// Order placed event processing
var processOrderPlaced = pipz.NewSequence[Event]("order-placed",
    pipz.Apply("validate-order", validateOrderData),
    
    pipz.NewConcurrent[Event]("order-processing",
        pipz.Apply("inventory", updateInventory),
        pipz.Apply("payment", processPayment),
        pipz.Effect("shipping", createShippingLabel),
    ),
    
    pipz.Apply("confirmation", sendOrderConfirmation),
)
```

## Stream Processing Patterns

### Windowing and Batching

```go
type EventBatcher struct {
    window   time.Duration
    maxBatch int
    events   []Event
    mu       sync.Mutex
    flush    chan struct{}
}

func NewEventBatcher(window time.Duration, maxBatch int) *EventBatcher {
    b := &EventBatcher{
        window:   window,
        maxBatch: maxBatch,
        events:   make([]Event, 0, maxBatch),
        flush:    make(chan struct{}, 1),
    }
    go b.runFlusher()
    return b
}

func (b *EventBatcher) Process(ctx context.Context, e Event) (Event, error) {
    b.mu.Lock()
    b.events = append(b.events, e)
    shouldFlush := len(b.events) >= b.maxBatch
    b.mu.Unlock()
    
    if shouldFlush {
        select {
        case b.flush <- struct{}{}:
        default:
        }
    }
    
    return e, nil
}

func (b *EventBatcher) runFlusher() {
    ticker := time.NewTicker(b.window)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            b.flushBatch()
        case <-b.flush:
            b.flushBatch()
        }
    }
}

func (b *EventBatcher) flushBatch() {
    b.mu.Lock()
    if len(b.events) == 0 {
        b.mu.Unlock()
        return
    }
    
    batch := b.events
    b.events = make([]Event, 0, b.maxBatch)
    b.mu.Unlock()
    
    // Process batch
    processBatch(batch)
}
```

### Event Deduplication

```go
type Deduplicator struct {
    seen  map[string]time.Time
    ttl   time.Duration
    mu    sync.RWMutex
}

func NewDeduplicator(ttl time.Duration) *Deduplicator {
    d := &Deduplicator{
        seen: make(map[string]time.Time),
        ttl:  ttl,
    }
    go d.cleanup()
    return d
}

func (d *Deduplicator) Process(ctx context.Context, e Event) (Event, error) {
    d.mu.Lock()
    defer d.mu.Unlock()
    
    if lastSeen, exists := d.seen[e.ID]; exists {
        if time.Since(lastSeen) < d.ttl {
            return e, fmt.Errorf("duplicate event %s", e.ID)
        }
    }
    
    d.seen[e.ID] = time.Now()
    return e, nil
}

// Use in pipeline
pipeline := pipz.NewSequence[Event]("deduplicated",
    pipz.Apply("dedup", deduplicator.Process),
    processEvent,
)
```

## Error Handling and DLQ

```go
// Dead Letter Queue for failed events
type DeadLetterQueue struct {
    storage    EventStorage
    maxRetries int
}

func (dlq *DeadLetterQueue) Process(ctx context.Context, e Event) (Event, error) {
    retries := 0
    if r, ok := e.Metadata["retry_count"]; ok {
        retries, _ = strconv.Atoi(r)
    }
    
    if retries >= dlq.maxRetries {
        // Move to DLQ
        e.Metadata["dlq_reason"] = "max_retries_exceeded"
        e.Metadata["dlq_timestamp"] = time.Now().Format(time.RFC3339)
        return e, dlq.storage.SaveToDLQ(ctx, e)
    }
    
    // Increment retry count
    e.Metadata["retry_count"] = strconv.Itoa(retries + 1)
    return e, nil
}

// Wrap processor with DLQ
processorWithDLQ := pipz.NewHandle("with-dlq",
    eventProcessor,
    pipz.NewSequence[*pipz.Error[Event]]("error-handler",
        pipz.Apply("check-retries", func(ctx context.Context, err *pipz.Error[Event]) (*pipz.Error[Event], error) {
            event := err.State
            return err, dlq.Process(ctx, event)
        }),
        pipz.Effect("alert", func(ctx context.Context, err *pipz.Error[Event]) error {
            if err.State.Metadata["retry_count"] == strconv.Itoa(dlq.maxRetries) {
                alerting.SendCritical("Event moved to DLQ", err.State)
            }
            return nil
        }),
    ),
)
```

## Event Sourcing Pattern

```go
// Event store with replay capability
type EventStore struct {
    events []Event
    mu     sync.RWMutex
}

func (es *EventStore) Append(ctx context.Context, e Event) (Event, error) {
    es.mu.Lock()
    defer es.mu.Unlock()
    
    e.Metadata["sequence"] = strconv.Itoa(len(es.events))
    es.events = append(es.events, e)
    return e, nil
}

func (es *EventStore) Replay(ctx context.Context, from time.Time, processor pipz.Chainable[Event]) error {
    es.mu.RLock()
    events := make([]Event, len(es.events))
    copy(events, es.events)
    es.mu.RUnlock()
    
    for _, event := range events {
        if event.Timestamp.After(from) {
            if _, err := processor.Process(ctx, event); err != nil {
                log.Printf("Failed to replay event %s: %v", event.ID, err)
            }
        }
    }
    return nil
}

// Create event-sourced pipeline
eventSourced := pipz.NewSequence[Event]("event-sourced",
    pipz.Apply("store", eventStore.Append),
    processEvent,
)
```

## Monitoring and Observability

```go
// Event metrics collector
var eventMetrics = pipz.Effect("metrics", func(ctx context.Context, e Event) error {
    labels := map[string]string{
        "type":   e.Type,
        "source": e.Source,
    }
    
    metrics.Increment("events.processed", labels)
    
    // Measure processing lag
    lag := time.Since(e.Timestamp)
    metrics.RecordHistogram("events.lag_ms", lag.Milliseconds(), labels)
    
    // Track event size
    size := len(fmt.Sprintf("%v", e.Data))
    metrics.RecordHistogram("events.size_bytes", size, labels)
    
    return nil
})

// Add to pipeline
monitoredPipeline := pipz.NewSequence[Event]("monitored",
    eventMetrics,
    eventProcessor,
)
```

## Testing Event Processing

```go
func TestEventRouting(t *testing.T) {
    processor := createEventProcessor()
    
    tests := []struct {
        name  string
        event Event
        check func(t *testing.T, result Event)
    }{
        {
            name: "user created event",
            event: Event{
                Type: "user.created",
                Data: map[string]interface{}{
                    "user_id": "123",
                    "email":   "test@example.com",
                },
            },
            check: func(t *testing.T, result Event) {
                assert.Contains(t, result.Metadata, "processed_at")
                assert.NotNil(t, result.Data["profile"])
            },
        },
        {
            name: "unknown event type",
            event: Event{
                Type: "unknown.event",
                Data: map[string]interface{}{},
            },
            check: func(t *testing.T, result Event) {
                assert.Equal(t, "unknown", result.Metadata["handler"])
            },
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result, err := processor.Process(context.Background(), tt.event)
            assert.NoError(t, err)
            tt.check(t, result)
        })
    }
}
```

## See Also

- [Switch Reference](../reference/connectors/switch.md)
- [Concurrent Reference](../reference/connectors/concurrent.md)
- [Scaffold Reference](../reference/processors/scaffold.md)
- [Handle Reference](../reference/processors/handle.md)