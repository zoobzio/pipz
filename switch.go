package pipz

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/zoobzio/hookz"
	"github.com/zoobzio/metricz"
	"github.com/zoobzio/tracez"
)

// Observability constants for the Switch connector.
const (
	// Metrics.
	SwitchProcessedTotal = metricz.Key("switch.processed.total")
	SwitchSuccessesTotal = metricz.Key("switch.successes.total")
	SwitchRoutedTotal    = metricz.Key("switch.routed.total")
	SwitchUnroutedTotal  = metricz.Key("switch.unrouted.total")
	SwitchDurationMs     = metricz.Key("switch.duration.ms")

	// Spans.
	SwitchProcessSpan = tracez.Key("switch.process")

	// Tags.
	SwitchTagRouteKey = tracez.Tag("switch.route_key")
	SwitchTagRouted   = tracez.Tag("switch.routed")
	SwitchTagSuccess  = tracez.Tag("switch.success")
	SwitchTagError    = tracez.Tag("switch.error")

	// Hook event keys.
	SwitchEventRouted   = hookz.Key("switch.routed")
	SwitchEventUnrouted = hookz.Key("switch.unrouted")
)

// SwitchEvent represents a switch routing event.
// This is emitted via hookz when routing decisions are made,
// providing visibility into which path was taken or if no route was found.
type SwitchEvent[K comparable] struct {
	Name          Name          // Connector name
	RouteKey      K             // The key returned by the condition
	ProcessorName Name          // Name of the processor that was routed to (if any)
	Routed        bool          // Whether a route was found
	Success       bool          // Whether the processor succeeded (if routed)
	Error         error         // Error from processor (if failed)
	Duration      time.Duration // How long the processor took (if routed)
	Timestamp     time.Time     // When the event occurred
}

// Condition determines routing based on input data.
// Returns a route key of any comparable type for multi-way branching.
//
// Using generic keys instead of strings enables type-safe routing
// beyond simple string matching. Define custom types for your routes:
//
//	type PaymentRoute string
//	const (
//	    RouteStandard   PaymentRoute = "standard"
//	    RouteHighValue  PaymentRoute = "high_value"
//	    RouteCrypto     PaymentRoute = "crypto"
//	    RouteDefault    PaymentRoute = "default"
//	)
//
// Common patterns include routing by:
//   - Typed enums for business states
//   - Integer codes for priority levels
//   - Custom types for domain concepts
type Condition[T any, K comparable] func(context.Context, T) K

// Switch routes to different processors based on condition result.
// Switch enables conditional processing where the path taken depends
// on the input data. The condition function examines the data and
// returns a route key that determines which processor to use.
//
// The key type K must be comparable (can be used as map key). This enables
// type-safe routing with custom types, avoiding magic strings. If no route
// exists for the returned key, the input passes through unchanged.
//
// Switch is perfect for:
//   - Type-based processing with enum safety
//   - Status-based workflows with defined states
//   - Region-specific logic with typed regions
//   - Priority handling with numeric levels
//   - A/B testing with experiment types
//   - Dynamic routing tables that change at runtime
//   - Feature flag controlled processing paths
//
// Example with type-safe keys:
//
//	type PaymentRoute string
//	const (
//	    RouteStandard   PaymentRoute = "standard"
//	    RouteHighValue  PaymentRoute = "high_value"
//	    RouteCrypto     PaymentRoute = "crypto"
//	)
//
//	switch := pipz.NewSwitch(
//	    func(ctx context.Context, p Payment) PaymentRoute {
//	        if p.Amount > 10000 {
//	            return RouteHighValue
//	        } else if p.Method == "crypto" {
//	            return RouteCrypto
//	        }
//	        return RouteStandard
//	    },
//	)
//	switch.AddRoute(RouteStandard, standardProcessor)
//	switch.AddRoute(RouteHighValue, highValueProcessor)
//	switch.AddRoute(RouteCrypto, cryptoProcessor)
//
// # Observability
//
// Switch provides comprehensive observability through metrics, tracing, and events:
//
// Metrics:
//   - switch.processed.total: Counter of switch operations
//   - switch.successes.total: Counter of successful routing
//   - switch.routed.total: Counter of requests that found a route
//   - switch.unrouted.total: Counter of requests with no matching route
//   - switch.duration.ms: Gauge of routing and processing duration
//
// Traces:
//   - switch.process: Span for switch operation including routing decision
//
// Events (via hooks):
//   - switch.routed: Fired when a route is found and executed
//   - switch.unrouted: Fired when no route matches
//
// Example with hooks:
//
//	type PriorityLevel int
//	const (
//	    PriorityHigh   PriorityLevel = 1
//	    PriorityMedium PriorityLevel = 2
//	    PriorityLow    PriorityLevel = 3
//	)
//
//	prioritySwitch := pipz.NewSwitch("priority-router",
//	    func(ctx context.Context, task Task) PriorityLevel {
//	        return task.Priority
//	    },
//	)
//
//	// Monitor routing patterns
//	prioritySwitch.OnRouted(func(ctx context.Context, event SwitchEvent[PriorityLevel]) error {
//	    metrics.Inc("tasks.routed", fmt.Sprintf("priority_%d", event.RouteKey))
//	    log.Debug("Task routed to %s (priority %d)", event.ProcessorName, event.RouteKey)
//	    return nil
//	})
//
//	// Alert on unhandled priorities
//	prioritySwitch.OnUnrouted(func(ctx context.Context, event SwitchEvent[PriorityLevel]) error {
//	    alert.Warn("No handler for priority level %d", event.RouteKey)
//	    return nil
//	})
type Switch[T any, K comparable] struct {
	condition Condition[T, K]
	routes    map[K]Chainable[T]
	name      Name
	mu        sync.RWMutex
	metrics   *metricz.Registry
	tracer    *tracez.Tracer
	hooks     *hookz.Hooks[SwitchEvent[K]]
}

// NewSwitch creates a new Switch connector with the given condition function.
func NewSwitch[T any, K comparable](name Name, condition Condition[T, K]) *Switch[T, K] {
	// Initialize observability
	metrics := metricz.New()
	metrics.Counter(SwitchProcessedTotal)
	metrics.Counter(SwitchSuccessesTotal)
	metrics.Counter(SwitchRoutedTotal)
	metrics.Counter(SwitchUnroutedTotal)
	metrics.Gauge(SwitchDurationMs)

	return &Switch[T, K]{
		name:      name,
		condition: condition,
		routes:    make(map[K]Chainable[T]),
		metrics:   metrics,
		tracer:    tracez.New(),
		hooks:     hookz.New[SwitchEvent[K]](),
	}
}

// Process implements the Chainable interface.
// If no route matches the condition result, the input is returned unchanged.
func (s *Switch[T, K]) Process(ctx context.Context, data T) (result T, err error) {
	defer recoverFromPanic(&result, &err, s.name, data)

	// Track metrics
	s.metrics.Counter(SwitchProcessedTotal).Inc()
	start := time.Now()

	// Start main span
	ctx, span := s.tracer.StartSpan(ctx, SwitchProcessSpan)
	defer func() {
		// Record duration
		elapsed := time.Since(start)
		s.metrics.Gauge(SwitchDurationMs).Set(float64(elapsed.Milliseconds()))

		// Set success status
		if err == nil {
			span.SetTag(SwitchTagSuccess, "true")
			s.metrics.Counter(SwitchSuccessesTotal).Inc()
		} else {
			span.SetTag(SwitchTagSuccess, "false")
			span.SetTag(SwitchTagError, err.Error())
		}
		span.Finish()
	}()

	s.mu.RLock()
	defer s.mu.RUnlock()

	route := s.condition(ctx, data)
	span.SetTag(SwitchTagRouteKey, fmt.Sprintf("%v", route))

	processor, exists := s.routes[route]
	if !exists {
		// No route found - pass through
		span.SetTag(SwitchTagRouted, "false")
		s.metrics.Counter(SwitchUnroutedTotal).Inc()

		// Emit unrouted event
		_ = s.hooks.Emit(ctx, SwitchEventUnrouted, SwitchEvent[K]{ //nolint:errcheck
			Name:      s.name,
			RouteKey:  route,
			Routed:    false,
			Timestamp: time.Now(),
		})

		return data, nil
	}

	// Route found
	span.SetTag(SwitchTagRouted, "true")
	s.metrics.Counter(SwitchRoutedTotal).Inc()

	processorStart := time.Now()
	result, err = processor.Process(ctx, data)
	processorDuration := time.Since(processorStart)

	// Emit routed event
	_ = s.hooks.Emit(ctx, SwitchEventRouted, SwitchEvent[K]{ //nolint:errcheck
		Name:          s.name,
		RouteKey:      route,
		ProcessorName: processor.Name(),
		Routed:        true,
		Success:       err == nil,
		Error:         err,
		Duration:      processorDuration,
		Timestamp:     time.Now(),
	})
	if err != nil {
		var pipeErr *Error[T]
		if errors.As(err, &pipeErr) {
			// Prepend this switch's name to the path
			pipeErr.Path = append([]Name{s.name}, pipeErr.Path...)
			return result, pipeErr
		}
		// Handle non-pipeline errors by wrapping them
		return result, &Error[T]{
			Timestamp: time.Now(),
			InputData: data,
			Err:       err,
			Path:      []Name{s.name},
		}
	}
	return result, nil
}

// AddRoute adds or updates a route in the switch.
func (s *Switch[T, K]) AddRoute(key K, processor Chainable[T]) *Switch[T, K] {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.routes[key] = processor
	return s
}

// RemoveRoute removes a route from the switch.
func (s *Switch[T, K]) RemoveRoute(key K) *Switch[T, K] {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.routes, key)
	return s
}

// SetCondition updates the condition function.
func (s *Switch[T, K]) SetCondition(condition Condition[T, K]) *Switch[T, K] {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.condition = condition
	return s
}

// Routes returns a copy of the current routes map.
func (s *Switch[T, K]) Routes() map[K]Chainable[T] {
	s.mu.RLock()
	defer s.mu.RUnlock()

	routes := make(map[K]Chainable[T], len(s.routes))
	for k, v := range s.routes {
		routes[k] = v
	}
	return routes
}

// HasRoute checks if a route exists for the given key.
func (s *Switch[T, K]) HasRoute(key K) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, exists := s.routes[key]
	return exists
}

// ClearRoutes removes all routes from the switch.
func (s *Switch[T, K]) ClearRoutes() *Switch[T, K] {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.routes = make(map[K]Chainable[T])
	return s
}

// SetRoutes replaces all routes in the switch atomically.
func (s *Switch[T, K]) SetRoutes(routes map[K]Chainable[T]) *Switch[T, K] {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.routes = make(map[K]Chainable[T], len(routes))
	for k, v := range routes {
		s.routes[k] = v
	}
	return s
}

// Name returns the name of this connector.
func (s *Switch[T, K]) Name() Name {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.name
}

// Metrics returns the metrics registry for this connector.
func (s *Switch[T, K]) Metrics() *metricz.Registry {
	return s.metrics
}

// Tracer returns the tracer for this connector.
func (s *Switch[T, K]) Tracer() *tracez.Tracer {
	return s.tracer
}

// Close gracefully shuts down observability components.
func (s *Switch[T, K]) Close() error {
	if s.tracer != nil {
		s.tracer.Close()
	}
	s.hooks.Close()
	return nil
}

// OnRouted registers a handler for when a route is found and executed.
// The handler is called asynchronously after the processor completes,
// providing visibility into successful routing decisions.
func (s *Switch[T, K]) OnRouted(handler func(context.Context, SwitchEvent[K]) error) error {
	_, err := s.hooks.Hook(SwitchEventRouted, handler)
	return err
}

// OnUnrouted registers a handler for when no route matches the condition.
// The handler is called asynchronously when data passes through unchanged,
// useful for monitoring unhandled cases.
func (s *Switch[T, K]) OnUnrouted(handler func(context.Context, SwitchEvent[K]) error) error {
	_, err := s.hooks.Hook(SwitchEventUnrouted, handler)
	return err
}
