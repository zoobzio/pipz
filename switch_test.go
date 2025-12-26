package pipz

import (
	"context"
	"errors"
	"testing"

	"github.com/zoobzio/capitan"
)

func TestSwitch(t *testing.T) {
	t.Run("Basic Routing", func(t *testing.T) {
		// Create processors for routes
		double := Transform(NewIdentity("double", ""), func(_ context.Context, n int) int {
			return n * 2
		})
		triple := Transform(NewIdentity("triple", ""), func(_ context.Context, n int) int {
			return n * 3
		})

		// Create condition function
		condition := func(_ context.Context, n int) string {
			if n%2 == 0 {
				return "even"
			}
			return "odd"
		}

		// Create switch
		sw := NewSwitch(NewIdentity("test-switch", ""), condition)
		sw.AddRoute("even", double)
		sw.AddRoute("odd", triple)

		// Test even number
		result, err := sw.Process(context.Background(), 4)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != 8 {
			t.Errorf("expected 8, got %d", result)
		}

		// Test odd number
		result, err = sw.Process(context.Background(), 3)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != 9 {
			t.Errorf("expected 9, got %d", result)
		}
	})

	t.Run("No Route Passes Through", func(t *testing.T) {
		condition := func(_ context.Context, _ int) string {
			return "nonexistent"
		}

		sw := NewSwitch(NewIdentity("test-switch", ""), condition)
		sw.AddRoute("other", Transform(NewIdentity("noop", ""), func(_ context.Context, n int) int {
			return n + 100
		}))

		// Should pass through unchanged
		result, err := sw.Process(context.Background(), 42)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != 42 {
			t.Errorf("expected 42 (passthrough), got %d", result)
		}
	})

	t.Run("Name Method", func(t *testing.T) {
		sw := NewSwitch(NewIdentity("my-switch", ""), func(_ context.Context, _ int) string {
			return "test"
		})

		if sw.Identity().Name() != "my-switch" {
			t.Errorf("expected 'my-switch', got %q", sw.Identity().Name())
		}
	})

	t.Run("Route Management", func(t *testing.T) {
		sw := NewSwitch(NewIdentity("test", ""), func(_ context.Context, _ int) string {
			return "route1"
		})

		processor := Transform(NewIdentity("test", ""), func(_ context.Context, n int) int { return n })

		// Add route
		sw.AddRoute("route1", processor)
		if !sw.HasRoute("route1") {
			t.Error("route1 should exist")
		}

		// Remove route
		sw.RemoveRoute("route1")
		if sw.HasRoute("route1") {
			t.Error("route1 should not exist")
		}

		// Test route clearing
		sw.AddRoute("route1", processor)
		sw.AddRoute("route2", processor)
		sw.ClearRoutes()
		if sw.HasRoute("route1") || sw.HasRoute("route2") {
			t.Error("routes should be cleared")
		}
	})

	t.Run("SetCondition Method", func(t *testing.T) {
		// Initial condition
		initialCondition := func(_ context.Context, n int) string {
			if n > 0 {
				return "positive"
			}
			return "negative"
		}

		sw := NewSwitch(NewIdentity("test", ""), initialCondition)
		sw.AddRoute("positive", Transform(NewIdentity("double", ""), func(_ context.Context, n int) int { return n * 2 }))
		sw.AddRoute("negative", Transform(NewIdentity("negate", ""), func(_ context.Context, n int) int { return -n }))

		// Test with initial condition
		result, err := sw.Process(context.Background(), 5)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != 10 {
			t.Errorf("expected 10, got %d", result)
		}

		// Change condition
		newCondition := func(_ context.Context, n int) string {
			if n%2 == 0 {
				return "even"
			}
			return "odd"
		}
		sw.SetCondition(newCondition)

		// Test with new condition - should pass through since no even/odd routes
		result, err = sw.Process(context.Background(), 5)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != 5 {
			t.Errorf("expected 5 (passthrough), got %d", result)
		}
	})

	t.Run("Routes Method", func(t *testing.T) {
		sw := NewSwitch(NewIdentity("test", ""), func(_ context.Context, _ int) string { return "default" })

		p1 := Transform(NewIdentity("p1", ""), func(_ context.Context, n int) int { return n })
		p2 := Transform(NewIdentity("p2", ""), func(_ context.Context, n int) int { return n })
		p3 := Transform(NewIdentity("p3", ""), func(_ context.Context, n int) int { return n })

		sw.AddRoute("route1", p1)
		sw.AddRoute("route2", p2)
		sw.AddRoute("route3", p3)

		routes := sw.Routes()
		if len(routes) != 3 {
			t.Errorf("expected 3 routes, got %d", len(routes))
		}

		// Check that all routes are present
		expectedRoutes := map[string]bool{"route1": false, "route2": false, "route3": false}
		for route := range routes {
			if _, ok := expectedRoutes[route]; !ok {
				t.Errorf("unexpected route: %s", route)
			}
			expectedRoutes[route] = true
		}
		for route, found := range expectedRoutes {
			if !found {
				t.Errorf("missing route: %s", route)
			}
		}
	})

	t.Run("SetRoutes Method", func(t *testing.T) {
		sw := NewSwitch(NewIdentity("test", ""), func(_ context.Context, _ int) string { return "default" })

		p1 := Transform(NewIdentity("p1", ""), func(_ context.Context, n int) int { return n + 1 })
		p2 := Transform(NewIdentity("p2", ""), func(_ context.Context, n int) int { return n + 2 })
		p3 := Transform(NewIdentity("p3", ""), func(_ context.Context, n int) int { return n + 3 })

		// Set initial routes
		initialRoutes := map[string]Chainable[int]{
			"a": p1,
			"b": p2,
		}
		sw.SetRoutes(initialRoutes)

		if !sw.HasRoute("a") || !sw.HasRoute("b") {
			t.Error("initial routes not set correctly")
		}
		if sw.HasRoute("c") {
			t.Error("unexpected route 'c'")
		}

		// Replace with new routes
		newRoutes := map[string]Chainable[int]{
			"x": p2,
			"y": p3,
			"z": p1,
		}
		sw.SetRoutes(newRoutes)

		// Old routes should be gone
		if sw.HasRoute("a") || sw.HasRoute("b") {
			t.Error("old routes should be removed")
		}

		// New routes should exist
		if !sw.HasRoute("x") || !sw.HasRoute("y") || !sw.HasRoute("z") {
			t.Error("new routes not set correctly")
		}

		// Verify routes work
		sw.SetCondition(func(_ context.Context, _ int) string { return "y" })
		result, err := sw.Process(context.Background(), 10)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != 13 { // 10 + 3 from p3
			t.Errorf("expected 13, got %d", result)
		}
	})

	t.Run("Route Processor Error", func(t *testing.T) {
		// Create a processor that returns an error
		errorProc := Apply(NewIdentity("error-proc", ""), func(_ context.Context, _ int) (int, error) {
			return 0, errors.New("processor failed")
		})
		successProc := Transform(NewIdentity("success", ""), func(_ context.Context, n int) int {
			return n * 2
		})

		condition := func(_ context.Context, n int) string {
			if n > 0 {
				return "error"
			}
			return "success"
		}

		sw := NewSwitch(NewIdentity("error-switch", ""), condition)
		sw.AddRoute("error", errorProc)
		sw.AddRoute("success", successProc)

		// Test error route
		_, err := sw.Process(context.Background(), 5)
		if err == nil {
			t.Fatal("expected error from error route")
		}

		// Check error path includes switch name
		var pipeErr *Error[int]
		if errors.As(err, &pipeErr) {
			expectedPath := []string{"error-switch", "error-proc"}
			if len(pipeErr.Path) != len(expectedPath) {
				t.Errorf("expected error path length %d, got %d", len(expectedPath), len(pipeErr.Path))
			}
			for i, exp := range expectedPath {
				if pipeErr.Path[i].Name() != exp {
					t.Errorf("expected error path[%d] %q, got %q", i, exp, pipeErr.Path[i].Name())
				}
			}
		} else {
			t.Error("expected error to be of type *pipz.Error[int]")
		}
	})

	t.Run("Switch panic recovery", func(t *testing.T) {
		// Create a switch with panic in condition
		panicSwitch := NewSwitch(NewIdentity("panic_switch", ""), func(_ context.Context, _ string) string {
			panic("switch condition panic")
		})

		result, err := panicSwitch.Process(context.Background(), "test")

		if result != "" {
			t.Errorf("expected empty string, got %q", result)
		}

		var pipzErr *Error[string]
		if !errors.As(err, &pipzErr) {
			t.Fatal("expected pipz.Error")
		}

		if pipzErr.Path[0].Name() != "panic_switch" {
			t.Errorf("expected path to start with 'panic_switch', got %v", pipzErr.Path)
		}

		if pipzErr.InputData != "test" {
			t.Errorf("expected input data 'test', got %q", pipzErr.InputData)
		}
	})

}

func TestSwitchClose(t *testing.T) {
	t.Run("Closes All Routes", func(t *testing.T) {
		p1 := newTrackingProcessor[int](NewIdentity("p1", ""))
		p2 := newTrackingProcessor[int](NewIdentity("p2", ""))

		sw := NewSwitch(NewIdentity("test", ""), func(_ context.Context, _ int) string { return "a" })
		sw.AddRoute("a", p1)
		sw.AddRoute("b", p2)

		err := sw.Close()

		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if p1.CloseCalls() != 1 || p2.CloseCalls() != 1 {
			t.Error("expected all routes to be closed")
		}
	})

	t.Run("Aggregates Errors", func(t *testing.T) {
		p1 := newTrackingProcessor[int](NewIdentity("p1", "")).WithCloseError(errors.New("p1 error"))
		p2 := newTrackingProcessor[int](NewIdentity("p2", "")).WithCloseError(errors.New("p2 error"))

		sw := NewSwitch(NewIdentity("test", ""), func(_ context.Context, _ int) string { return "a" })
		sw.AddRoute("a", p1)
		sw.AddRoute("b", p2)

		err := sw.Close()

		if err == nil {
			t.Error("expected error")
		}
		if p1.CloseCalls() != 1 || p2.CloseCalls() != 1 {
			t.Error("expected all routes to be closed")
		}
	})

	t.Run("Idempotency", func(t *testing.T) {
		p := newTrackingProcessor[int](NewIdentity("p", ""))
		sw := NewSwitch(NewIdentity("test", ""), func(_ context.Context, _ int) string { return "a" })
		sw.AddRoute("a", p)

		_ = sw.Close()
		_ = sw.Close()

		if p.CloseCalls() != 1 {
			t.Errorf("expected 1 close call, got %d", p.CloseCalls())
		}
	})
}

func TestSwitchSignals(t *testing.T) {
	t.Run("Emits Routed Signal With Match", func(t *testing.T) {
		var signalReceived bool
		var signalName string
		var signalRouteKey string
		var signalMatched bool

		listener := capitan.Hook(SignalSwitchRouted, func(_ context.Context, e *capitan.Event) {
			signalReceived = true
			signalName, _ = FieldName.From(e)
			signalRouteKey, _ = FieldRouteKey.From(e)
			signalMatched, _ = FieldMatched.From(e)
		})
		defer listener.Close()

		sw := NewSwitch(NewIdentity("signal-test-switch", ""), func(_ context.Context, n int) string {
			if n > 10 {
				return "high"
			}
			return "low"
		})
		sw.AddRoute("high", Transform(NewIdentity("double", ""), func(_ context.Context, n int) int { return n * 2 }))
		sw.AddRoute("low", Transform(NewIdentity("half", ""), func(_ context.Context, n int) int { return n / 2 }))

		_, err := sw.Process(context.Background(), 20)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if err := listener.Drain(context.Background()); err != nil {
			t.Fatalf("drain failed: %v", err)
		}

		if !signalReceived {
			t.Error("expected signal to be received")
		}
		if signalName != "signal-test-switch" {
			t.Errorf("expected name 'signal-test-switch', got %q", signalName)
		}
		if signalRouteKey != "high" {
			t.Errorf("expected route_key 'high', got %q", signalRouteKey)
		}
		if !signalMatched {
			t.Error("expected matched to be true")
		}
	})

	t.Run("Emits Routed Signal Without Match", func(t *testing.T) {
		var signalMatched bool
		var signalReceived bool

		listener := capitan.Hook(SignalSwitchRouted, func(_ context.Context, e *capitan.Event) {
			signalReceived = true
			signalMatched, _ = FieldMatched.From(e)
		})
		defer listener.Close()

		sw := NewSwitch(NewIdentity("signal-no-match", ""), func(_ context.Context, _ int) string {
			return "unknown"
		})
		sw.AddRoute("known", Transform(NewIdentity("double", ""), func(_ context.Context, n int) int { return n * 2 }))

		_, err := sw.Process(context.Background(), 5)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if err := listener.Drain(context.Background()); err != nil {
			t.Fatalf("drain failed: %v", err)
		}

		if !signalReceived {
			t.Error("expected signal to be received")
		}
		if signalMatched {
			t.Error("expected matched to be false")
		}
	})

	t.Run("Schema", func(t *testing.T) {
		proc1 := Transform(NewIdentity("route-a", ""), func(_ context.Context, n int) int { return n })
		proc2 := Transform(NewIdentity("route-b", ""), func(_ context.Context, n int) int { return n })
		condition := func(_ context.Context, _ int) string { return "a" }

		sw := NewSwitch(NewIdentity("test-switch", "Switch connector"), condition)
		sw.AddRoute("a", proc1)
		sw.AddRoute("b", proc2)

		schema := sw.Schema()

		if schema.Identity.Name() != "test-switch" {
			t.Errorf("Schema Identity.Name() = %v, want %v", schema.Identity.Name(), "test-switch")
		}
		if schema.Type != "switch" {
			t.Errorf("Schema Type = %v, want %v", schema.Type, "switch")
		}

		flow, ok := SwitchKey.From(schema)
		if !ok {
			t.Fatal("Expected SwitchFlow")
		}
		if len(flow.Routes) != 2 {
			t.Errorf("len(Flow.Routes) = %d, want 2", len(flow.Routes))
		}
	})
}
