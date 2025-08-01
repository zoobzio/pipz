package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/zoobzio/pipz"
)

// TraceID is a custom type for our trace context key.
type TraceID string

const traceIDKey TraceID = "traceID"

// Data represents our example data type.
type Data struct {
	Message string
	Value   int
}

// Clone implements the Cloner interface.
func (d Data) Clone() Data {
	return Data{
		Message: d.Message,
		Value:   d.Value,
	}
}

// withTraceID adds a trace ID to the context.
func withTraceID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, traceIDKey, id)
}

// getTraceID extracts the trace ID from context.
func getTraceID(ctx context.Context) string {
	if id, ok := ctx.Value(traceIDKey).(string); ok {
		return id
	}
	return "no-trace"
}

func main() {
	// Create processors that log their trace ID
	logWithTrace := pipz.Effect("log-trace", func(ctx context.Context, d Data) error {
		traceID := getTraceID(ctx)
		log.Printf("[%s] Processing: %s", traceID, d.Message)
		time.Sleep(100 * time.Millisecond)
		return nil
	})

	sendMetrics := pipz.Effect("send-metrics", func(ctx context.Context, d Data) error {
		traceID := getTraceID(ctx)
		log.Printf("[%s] Sending metrics for value: %d", traceID, d.Value)
		time.Sleep(150 * time.Millisecond)
		return nil
	})

	updateCache := pipz.Effect("update-cache", func(ctx context.Context, d Data) error {
		traceID := getTraceID(ctx)
		log.Printf("[%s] Updating cache with: %s", traceID, d.Message)
		time.Sleep(200 * time.Millisecond)
		return nil
	})

	// Example 1: Concurrent preserves trace context and waits
	fmt.Println("=== Concurrent Example (waits for all) ===")
	concurrent := pipz.NewConcurrent("traced-concurrent", logWithTrace, sendMetrics, updateCache)

	ctx := withTraceID(context.Background(), "trace-123")
	data := Data{Value: 42, Message: "Hello, World!"}

	start := time.Now()
	_, err := concurrent.Process(ctx, data)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Concurrent completed in: %v\n\n", time.Since(start))

	// Example 2: Scaffold preserves trace context but doesn't wait
	fmt.Println("=== Scaffold Example (fire-and-forget) ===")
	scaffold := pipz.NewScaffold("async-scaffold", logWithTrace, sendMetrics, updateCache)

	ctx2 := withTraceID(context.Background(), "trace-456")
	data2 := Data{Value: 99, Message: "Async processing"}

	start2 := time.Now()
	_, err = scaffold.Process(ctx2, data2)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Scaffold returned in: %v\n", time.Since(start2))
	fmt.Println("(Background processors still running...)")

	// Wait a bit to see background logs
	time.Sleep(300 * time.Millisecond)

	// Example 3: Race with cancellation
	fmt.Println("\n=== Race Example (first wins, cancels others) ===")
	fastProcessor := pipz.Effect("fast", func(ctx context.Context, _ Data) error {
		traceID := getTraceID(ctx)
		log.Printf("[%s] Fast processor starting", traceID)
		time.Sleep(50 * time.Millisecond)
		log.Printf("[%s] Fast processor done", traceID)
		return nil
	})

	slowProcessor := pipz.Effect("slow", func(ctx context.Context, _ Data) error {
		traceID := getTraceID(ctx)
		log.Printf("[%s] Slow processor starting", traceID)
		select {
		case <-time.After(500 * time.Millisecond):
			log.Printf("[%s] Slow processor completed (shouldn't happen)", traceID)
		case <-ctx.Done():
			log.Printf("[%s] Slow processor canceled", traceID)
		}
		return nil
	})

	race := pipz.NewRace("traced-race", fastProcessor, slowProcessor)
	ctx3 := withTraceID(context.Background(), "trace-789")
	data3 := Data{Value: 100, Message: "Race condition"}

	start3 := time.Now()
	_, err = race.Process(ctx3, data3)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Race completed in: %v\n", time.Since(start3))

	// Wait to see cancellation
	time.Sleep(100 * time.Millisecond)
}
