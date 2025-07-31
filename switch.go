package pipz

import (
	"context"
	"sync"
)

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
type Switch[T any, K comparable] struct {
	condition Condition[T, K]
	routes    map[K]Chainable[T]
	name      Name
	mu        sync.RWMutex
}

// NewSwitch creates a new Switch connector with the given condition function.
func NewSwitch[T any, K comparable](name Name, condition Condition[T, K]) *Switch[T, K] {
	return &Switch[T, K]{
		name:      name,
		condition: condition,
		routes:    make(map[K]Chainable[T]),
	}
}

// Process implements the Chainable interface.
// If no route matches the condition result, the input is returned unchanged.
func (s *Switch[T, K]) Process(ctx context.Context, data T) (T, *Error[T]) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	route := s.condition(ctx, data)
	processor, exists := s.routes[route]
	if !exists {
		return data, nil
	}
	result, err := processor.Process(ctx, data)
	if err != nil {
		// Prepend this switch's name to the path
		err.Path = append([]Name{s.name}, err.Path...)
	}
	return result, err
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
