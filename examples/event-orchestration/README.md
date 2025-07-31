# Fleet Management Event Orchestration

This example demonstrates how pipz enables microservice-like architecture within a monolith through event-driven orchestration of domain-specific pipelines.

## The Story: From Chaos to Coordination

### Sprint 1: MVP - Just Track Vehicles
"We need to know where our trucks are!"
- Simple location tracking
- Everything processes FleetEvent directly
- All logic mixed together in one handler

**The code:**
```go
func handleLocationUpdate(event FleetEvent) {
    // 500 lines of tangled logic...
    updateDatabase(event.VehicleID, event.Data["lat"], event.Data["lng"])
    checkSpeed(event.Data["speed"])
    calculateFuel(event.Data["mileage"])
    checkMaintenance(event.VehicleID)
    // etc...
}
```

### Sprint 3: The 3 AM Wake-up Call
**Crisis:** "Why didn't we know the driver was speeding for 2 hours?"  
**Support:** "The system knew! But the alert was buried in the maintenance check code..."  
**Fleet Manager:** "And why did the engine seize? We track maintenance!"  
**Dev:** "Well, the maintenance alert is only checked during location updates..."

### Sprint 5: Domain Separation - Each Team Owns Their Pipeline
**Solution:** Break into domain pipelines with their own types!

```go
// Safety team owns SafetyIncident and SafetyPipeline
type SafetyIncident struct {
    Vehicle     Vehicle
    Driver      Driver
    Severity    int
    VideoURL    string
}

// Maintenance team owns MaintenanceCheck and MaintenancePipeline  
type MaintenanceCheck struct {
    Vehicle        Vehicle
    ServiceHistory []ServiceRecord
    CurrentMileage int
}

// Event handlers BRIDGE between event and domain types
eventRouter.AddRoute("harsh.braking", safetyBridge)
```

**Result:** Teams can work independently, test in isolation, and evolve their domains!

### Sprint 7: The Cascade Effect
**New requirement:** "A harsh braking event needs to check EVERYTHING"
- Driver safety score (Safety domain)
- Dash cam footage (Safety domain)  
- Brake wear status (Maintenance domain)
- Is vehicle in construction zone? (Routing domain)
- Hours of service compliance (Compliance domain)

**Solution:** Concurrent domain processing!

### Sprint 9: Cross-Domain Intelligence
**Fleet Manager:** "Vehicle 42 has 3 safety incidents AND overdue maintenance - that's a liability!"
**Solution:** Risk Assessment pipeline that reads results from all domains

### Sprint 11: Predictive Operations
**CEO:** "Can we prevent problems instead of reacting?"
**Solution:** Pattern analysis across domains to predict:
- Maintenance needs based on driving patterns
- Safety risks based on route + driver history
- Optimal routes based on vehicle condition

## Key Architecture Patterns

### 1. Type Boundaries
Each domain has its own types - no shared structs!
```go
// ❌ Bad: Shared "god" struct
type Vehicle struct {
    // 50 fields for all domains...
}

// ✅ Good: Domain-specific types
type SafetyVehicle struct {
    ID           string
    SafetyScore  float64
    Incidents    []Incident
}

type MaintenanceVehicle struct {
    ID              string
    LastServiceDate time.Time
    NextServiceMile int
}
```

### 2. Bridge Pattern
Event handlers translate between event and domain types:
```go
func safetyBridge(ctx context.Context, event FleetEvent) (FleetEvent, error) {
    // Fetch domain data
    vehicle := vehicleRepo.GetForSafety(event.VehicleID)
    
    // Create domain type
    incident := SafetyIncident{Vehicle: vehicle, ...}
    
    // Run domain pipeline
    result, err := SafetyPipeline.Process(ctx, incident)
    
    // Bridge results back
    event.Data["safety_score"] = result.Score
    return event, err
}
```

### 3. Pipeline Encapsulation
Domains hide their pipelines:
```go
// safety/pipeline.go
var pipeline *pipz.Sequence[SafetyIncident] // lowercase - not exported!

func ProcessSafetyIncident(ctx context.Context, incident SafetyIncident) (SafetyResult, error) {
    return pipeline.Process(ctx, incident)
}
```

## Running the Example

```bash
# Run full demo showing evolution
go run .

# Run specific sprint
go run . -sprint=7
```

## Benefits Demonstrated

1. **True Domain Isolation**: Each team owns their types and logic
2. **Independent Evolution**: Add fields to SafetyIncident without touching Maintenance
3. **Testability**: Test domain pipelines without event infrastructure
4. **Scalability**: Add new domains by adding new event routes
5. **Type Safety**: Compiler ensures bridge functions handle type conversion
6. **Performance**: Concurrent processing when domains are independent

## This is Microservices... in a Monolith!

Each domain pipeline is like a microservice:
- Own data types (like API contracts)
- Own business logic (like service code)
- Own storage access (like service databases)
- Communicate through events (like message queues)

But with monolith benefits:
- Single deployment
- No network calls
- Shared libraries
- Atomic transactions when needed