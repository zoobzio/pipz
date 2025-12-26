package pipz

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/zoobzio/capitan"
)

// Condition determines routing based on input data.
// Returns a route key string for multi-way branching.
//
// Define string constants for type-safe routing:
//
//	const (
//	    RouteStandard   = "standard"
//	    RouteHighValue  = "high_value"
//	    RouteCrypto     = "crypto"
//	)
//
// Common patterns include routing by:
//   - Status strings for workflow states
//   - Region identifiers
//   - Priority levels as strings
//   - Feature flag names
type Condition[T any] func(context.Context, T) string

// Switch routes to different processors based on condition result.
// Switch enables conditional processing where the path taken depends
// on the input data. The condition function examines the data and
// returns a route key that determines which processor to use.
//
// If no route exists for the returned key, the input passes through unchanged.
//
// Switch is perfect for:
//   - Status-based workflows with defined states
//   - Region-specific logic
//   - Priority handling
//   - A/B testing with experiment names
//   - Dynamic routing tables that change at runtime
//   - Feature flag controlled processing paths
//
// Example:
//
//	const (
//	    RouteStandard   = "standard"
//	    RouteHighValue  = "high_value"
//	    RouteCrypto     = "crypto"
//	)
//
//	router := pipz.NewSwitch(SwitchID,
//	    func(ctx context.Context, p Payment) string {
//	        if p.Amount > 10000 {
//	            return RouteHighValue
//	        } else if p.Method == "crypto" {
//	            return RouteCrypto
//	        }
//	        return RouteStandard
//	    },
//	)
//	router.AddRoute(RouteStandard, standardProcessor)
//	router.AddRoute(RouteHighValue, highValueProcessor)
//	router.AddRoute(RouteCrypto, cryptoProcessor)
type Switch[T any] struct {
	condition Condition[T]
	routes    map[string]Chainable[T]
	identity  Identity
	mu        sync.RWMutex
	closeOnce sync.Once
	closeErr  error
}

// NewSwitch creates a new Switch connector with the given condition function.
func NewSwitch[T any](identity Identity, condition Condition[T]) *Switch[T] {
	return &Switch[T]{
		identity:  identity,
		condition: condition,
		routes:    make(map[string]Chainable[T]),
	}
}

// Process implements the Chainable interface.
// If no route matches the condition result, the input is returned unchanged.
func (s *Switch[T]) Process(ctx context.Context, data T) (result T, err error) {
	defer recoverFromPanic(&result, &err, s.identity, data)

	s.mu.RLock()
	defer s.mu.RUnlock()

	route := s.condition(ctx, data)

	processor, exists := s.routes[route]

	// Emit routed signal
	capitan.Info(ctx, SignalSwitchRouted,
		FieldName.Field(s.identity.Name()),
		FieldIdentityID.Field(s.identity.ID().String()),
		FieldRouteKey.Field(route),
		FieldMatched.Field(exists),
	)

	if !exists {
		// No route found - pass through
		return data, nil
	}

	// Route found - execute processor
	result, err = processor.Process(ctx, data)
	if err != nil {
		var pipeErr *Error[T]
		if errors.As(err, &pipeErr) {
			// Prepend this switch's identity to the path
			pipeErr.Path = append([]Identity{s.identity}, pipeErr.Path...)
			return result, pipeErr
		}
		// Handle non-pipeline errors by wrapping them
		return result, &Error[T]{
			Timestamp: time.Now(),
			InputData: data,
			Err:       err,
			Path:      []Identity{s.identity},
		}
	}
	return result, nil
}

// AddRoute adds or updates a route in the switch.
func (s *Switch[T]) AddRoute(key string, processor Chainable[T]) *Switch[T] {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.routes[key] = processor
	return s
}

// RemoveRoute removes a route from the switch.
func (s *Switch[T]) RemoveRoute(key string) *Switch[T] {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.routes, key)
	return s
}

// SetCondition updates the condition function.
func (s *Switch[T]) SetCondition(condition Condition[T]) *Switch[T] {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.condition = condition
	return s
}

// Routes returns a copy of the current routes map.
func (s *Switch[T]) Routes() map[string]Chainable[T] {
	s.mu.RLock()
	defer s.mu.RUnlock()

	routes := make(map[string]Chainable[T], len(s.routes))
	for k, v := range s.routes {
		routes[k] = v
	}
	return routes
}

// HasRoute checks if a route exists for the given key.
func (s *Switch[T]) HasRoute(key string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, exists := s.routes[key]
	return exists
}

// ClearRoutes removes all routes from the switch.
func (s *Switch[T]) ClearRoutes() *Switch[T] {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.routes = make(map[string]Chainable[T])
	return s
}

// SetRoutes replaces all routes in the switch atomically.
func (s *Switch[T]) SetRoutes(routes map[string]Chainable[T]) *Switch[T] {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.routes = make(map[string]Chainable[T], len(routes))
	for k, v := range routes {
		s.routes[k] = v
	}
	return s
}

// Identity returns the identity of this connector.
func (s *Switch[T]) Identity() Identity {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.identity
}

// Schema returns a Node representing this connector in the pipeline schema.
func (s *Switch[T]) Schema() Node {
	s.mu.RLock()
	defer s.mu.RUnlock()

	routes := make(map[string]Node, len(s.routes))
	for key, proc := range s.routes {
		routes[key] = proc.Schema()
	}

	return Node{
		Identity: s.identity,
		Type:     "switch",
		Flow:     SwitchFlow{Routes: routes},
	}
}

// Close gracefully shuts down the connector and all its route processors.
// Close is idempotent - multiple calls return the same result.
func (s *Switch[T]) Close() error {
	s.closeOnce.Do(func() {
		s.mu.RLock()
		defer s.mu.RUnlock()

		var errs []error
		for _, processor := range s.routes {
			if err := processor.Close(); err != nil {
				errs = append(errs, err)
			}
		}
		s.closeErr = errors.Join(errs...)
	})
	return s.closeErr
}
