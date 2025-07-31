package pipz

import (
	"context"
	"errors"
	"testing"
)

func TestEnrich(t *testing.T) {
	t.Run("Enrich Success", func(t *testing.T) {
		type Order struct {
			ID           string
			CustomerID   string
			CustomerName string
		}

		// Simulate successful enrichment
		const addCustomerName Name = "add_customer_name"
		enricher := Enrich(addCustomerName, func(_ context.Context, o Order) (Order, error) {
			// Simulate DB lookup
			if o.CustomerID == "123" {
				o.CustomerName = "Alice Smith"
			}
			return o, nil
		})

		order := Order{ID: "order-1", CustomerID: "123"}
		result, err := enricher.Process(context.Background(), order)
		if err != nil {
			t.Fatalf("enrich should not fail: %v", err)
		}
		if result.CustomerName != "Alice Smith" {
			t.Errorf("expected customer name to be added")
		}
	})

	t.Run("Enrich Failure Returns Original", func(t *testing.T) {
		type Product struct {
			ID    string
			Name  string
			Price float64
		}

		// Simulate enrichment that fails
		const addPrice Name = "add_price"
		enricher := Enrich(addPrice, func(_ context.Context, p Product) (Product, error) {
			// Simulate external service failure
			return p, errors.New("price service unavailable")
		})

		product := Product{ID: "prod-1", Name: "Widget"}
		result, err := enricher.Process(context.Background(), product)
		if err != nil {
			t.Fatalf("enrich should not propagate error: %v", err)
		}
		if result != product {
			t.Errorf("expected original product on enrichment failure")
		}
	})

	t.Run("Enrich Best Effort", func(t *testing.T) {
		type Event struct {
			ID       string
			Type     string
			Location string
			Weather  string
		}

		callCount := 0
		const addWeather Name = "add_weather"
		weatherService := Enrich(addWeather, func(_ context.Context, e Event) (Event, error) {
			callCount++
			// Fail every other call
			if callCount%2 == 0 {
				return e, errors.New("weather service timeout")
			}
			e.Weather = "sunny"
			return e, nil
		})

		// First call succeeds
		event1 := Event{ID: "1", Type: "outdoor", Location: "park"}
		result1, err := weatherService.Process(context.Background(), event1)
		if err != nil {
			t.Fatalf("unexpected error on first call: %v", err)
		}
		if result1.Weather != "sunny" {
			t.Error("expected weather to be added on success")
		}

		// Second call fails but returns original
		event2 := Event{ID: "2", Type: "outdoor", Location: "beach"}
		result2, err := weatherService.Process(context.Background(), event2)
		if err != nil {
			t.Errorf("unexpected error: %v (Enrich should swallow errors)", err)
		}
		if result2.Weather != "" {
			t.Error("expected no weather on failure")
		}

		if callCount != 2 {
			t.Errorf("enrichment function should be called for each invocation")
		}
	})
}
