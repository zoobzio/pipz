# Shipping Fulfillment Example

This example demonstrates pipz's adapter pattern and the Contest connector for e-commerce shipping fulfillment.

## Overview

The example shows how pipz can orchestrate multiple shipping providers (FedEx, UPS, USPS, etc.) through a simple adapter interface. External teams only need to implement three methods, while pipz handles:

- Concurrent rate shopping with Contest
- Optimal provider selection
- Fallback handling  
- Timeout management
- Comprehensive error tracking

## Key Features

1. **Simple Provider Interface**: Shipping providers implement just 3 methods
2. **Contest Pattern**: Find the cheapest acceptable rate quickly
3. **Progressive Enhancement**: Add providers without changing core logic
4. **Production-Ready**: Includes retry, timeout, and error handling

## Running the Example

```bash
go run .
```

The example simulates:
- Multiple shipping providers with varying rates and speeds
- Contest-based rate shopping (first acceptable rate wins)
- Label generation with the selected provider
- Comprehensive metrics and reporting

## Architecture

```
ShippingProvider Interface
    ├── GetRates()      - Quote shipping rates
    ├── CreateLabel()   - Generate shipping label
    └── GetTracking()   - Track shipment

Contest Connector
    ├── Runs all providers in parallel
    ├── Returns first rate meeting criteria
    └── Cancels slower providers

Pipeline Flow
    1. Validate shipment
    2. Contest: Find best rate from providers
    3. Create label with winning provider
    4. Dispatch shipment
```

## Adapter Pattern Benefits

External teams can integrate easily:

```go
type MyShippingProvider struct {
    apiKey string
}

func (p *MyShippingProvider) GetRates(ctx context.Context, shipment Shipment) ([]Rate, error) {
    // Simple rate calculation
    return []Rate{{Cost: 15.99, EstimatedDays: 3}}, nil
}

func (p *MyShippingProvider) CreateLabel(ctx context.Context, shipment Shipment, rate Rate) (*Label, error) {
    // Generate label
    return &Label{TrackingNumber: "123456"}, nil
}

func (p *MyShippingProvider) GetTracking(ctx context.Context, trackingNumber string) (*TrackingInfo, error) {
    // Return tracking info
    return &TrackingInfo{Status: TrackingInTransit}, nil
}
```

That's it! Pipz handles the rest.

## Contest vs Race

This example uses Contest instead of Race because:

- **Race**: Returns first successful result (any rate)
- **Contest**: Returns first result meeting criteria (acceptable rate)

With Contest, we can define "acceptable" as cheapest rate under $50 that delivers in 3 days or less.