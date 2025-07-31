package main

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

// Mock repositories and services for the example.

// VehicleRepository provides vehicle data.
type VehicleRepository struct {
	vehicles map[string]Vehicle
	mu       sync.RWMutex
}

func NewVehicleRepository() *VehicleRepository {
	repo := &VehicleRepository{
		vehicles: make(map[string]Vehicle),
	}

	// Seed with test data.
	repo.vehicles["TRUCK-001"] = Vehicle{
		ID:           "TRUCK-001",
		Type:         "truck",
		Make:         "Freightliner",
		Model:        "Cascadia",
		Year:         2022,
		LicensePlate: "ABC-1234",
		VIN:          "1FUJGLDR2PLBC1234",
	}

	repo.vehicles["VAN-002"] = Vehicle{
		ID:           "VAN-002",
		Type:         "van",
		Make:         "Mercedes",
		Model:        "Sprinter",
		Year:         2023,
		LicensePlate: "XYZ-5678",
		VIN:          "WD3PE8CC5P9123456",
	}

	repo.vehicles["SEDAN-003"] = Vehicle{
		ID:           "SEDAN-003",
		Type:         "sedan",
		Make:         "Toyota",
		Model:        "Camry",
		Year:         2023,
		LicensePlate: "DEF-9012",
		VIN:          "4T1B11HK5PU123456",
	}

	return repo
}

func (r *VehicleRepository) Get(id string) Vehicle {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if v, exists := r.vehicles[id]; exists {
		return v
	}

	// Return a default vehicle if not found.
	return Vehicle{
		ID:   id,
		Type: "unknown",
		Make: "Unknown",
	}
}

// DriverRepository provides driver data.
type DriverRepository struct {
	drivers       map[string]Driver
	vehicleDriver map[string]string // vehicle ID -> driver ID
	mu            sync.RWMutex
}

func NewDriverRepository() *DriverRepository {
	repo := &DriverRepository{
		drivers:       make(map[string]Driver),
		vehicleDriver: make(map[string]string),
	}

	// Seed with test data.
	repo.drivers["DRIVER-001"] = Driver{
		ID:            "DRIVER-001",
		Name:          "John Smith",
		LicenseNumber: "D123456789",
		LicenseExpiry: time.Now().Add(365 * 24 * time.Hour),
		EmployeeID:    "EMP-001",
	}

	repo.drivers["DRIVER-002"] = Driver{
		ID:            "DRIVER-002",
		Name:          "Jane Doe",
		LicenseNumber: "D987654321",
		LicenseExpiry: time.Now().Add(180 * 24 * time.Hour),
		EmployeeID:    "EMP-002",
	}

	repo.drivers["DRIVER-003"] = Driver{
		ID:            "DRIVER-003",
		Name:          "Mike Johnson",
		LicenseNumber: "D456789123",
		LicenseExpiry: time.Now().Add(30 * 24 * time.Hour), // Expiring soon!
		EmployeeID:    "EMP-003",
	}

	// Map vehicles to drivers.
	repo.vehicleDriver["TRUCK-001"] = "DRIVER-001"
	repo.vehicleDriver["VAN-002"] = "DRIVER-002"
	repo.vehicleDriver["SEDAN-003"] = "DRIVER-003"

	return repo
}

func (r *DriverRepository) Get(id string) Driver {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if d, exists := r.drivers[id]; exists {
		return d
	}

	return Driver{
		ID:   id,
		Name: "Unknown Driver",
	}
}

func (r *DriverRepository) GetByVehicle(vehicleID string) Driver {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if driverID, exists := r.vehicleDriver[vehicleID]; exists {
		return r.Get(driverID)
	}

	return Driver{
		ID:   "UNKNOWN",
		Name: "No Driver Assigned",
	}
}

// MaintenanceRepository provides maintenance history.
type MaintenanceRepository struct {
	history map[string][]ServiceRecord
	mu      sync.RWMutex
}

func NewMaintenanceRepository() *MaintenanceRepository {
	repo := &MaintenanceRepository{
		history: make(map[string][]ServiceRecord),
	}

	// Seed with test data.
	repo.history["TRUCK-001"] = []ServiceRecord{
		{
			Date:          time.Now().Add(-90 * 24 * time.Hour),
			Mileage:       145000,
			Type:          "oil_change",
			Cost:          150.00,
			ServiceCenter: "Fleet Service Center",
			Technician:    "Tom",
			Notes:         "Regular maintenance",
		},
		{
			Date:          time.Now().Add(-30 * 24 * time.Hour),
			Mileage:       155000,
			Type:          "tire_rotation",
			Cost:          80.00,
			ServiceCenter: "Fleet Service Center",
			Technician:    "Jerry",
			Notes:         "All tires in good condition",
		},
	}

	repo.history["VAN-002"] = []ServiceRecord{
		{
			Date:          time.Now().Add(-45 * 24 * time.Hour),
			Mileage:       67000,
			Type:          "brake_service",
			Cost:          350.00,
			ServiceCenter: "Quick Stop Auto",
			Technician:    "Mike",
			Notes:         "Replaced front brake pads",
		},
	}

	return repo
}

func (r *MaintenanceRepository) GetHistory(vehicleID string) []ServiceRecord {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if history, exists := r.history[vehicleID]; exists {
		// Return a copy.
		result := make([]ServiceRecord, len(history))
		copy(result, history)
		return result
	}

	return []ServiceRecord{}
}

// RouteRepository provides planned routes.
type RouteRepository struct {
	routes map[string]Route
	mu     sync.RWMutex
}

func NewRouteRepository() *RouteRepository {
	repo := &RouteRepository{
		routes: make(map[string]Route),
	}

	// Seed with test routes.
	repo.routes["ROUTE-NORTH"] = Route{
		ID:       "ROUTE-NORTH",
		Name:     "Northern Distribution",
		Distance: 250.5,
		Duration: 4 * time.Hour,
		Waypoints: []GPSCoordinate{
			{Latitude: 40.7128, Longitude: -74.0060}, // NYC
			{Latitude: 41.8781, Longitude: -87.6298}, // Chicago
		},
	}

	repo.routes["ROUTE-LOCAL"] = Route{
		ID:       "ROUTE-LOCAL",
		Name:     "Local Deliveries",
		Distance: 45.2,
		Duration: 2 * time.Hour,
		Waypoints: []GPSCoordinate{
			{Latitude: 40.7128, Longitude: -74.0060},
			{Latitude: 40.7580, Longitude: -73.9855},
			{Latitude: 40.7489, Longitude: -73.9680},
		},
	}

	return repo
}

func (r *RouteRepository) GetActiveRoute(vehicleID string) Route {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Simple logic - trucks get long routes, others get local.
	vehicle := vehicleRepo.Get(vehicleID)
	if vehicle.Type == "truck" {
		return r.routes["ROUTE-NORTH"]
	}

	return r.routes["ROUTE-LOCAL"]
}

// Mock external services.

// VideoService simulates dash cam footage retrieval.
type VideoService struct{}

func (*VideoService) GetVideoURL(vehicleID string, timestamp time.Time) string {
	// Simulate video storage URL.
	return fmt.Sprintf("https://dashcam.fleet.com/video/%s/%d", vehicleID, timestamp.Unix())
}

// WeatherService provides weather conditions.
type WeatherService struct{}

func (*WeatherService) GetConditions(_, _ float64) WeatherConditions {
	// Randomly return different conditions.
	conditions := []string{"clear", "rain", "fog", "snow"}
	return WeatherConditions{
		Type:        conditions[rand.Intn(len(conditions))], //nolint:gosec // Mock data generation
		Visibility:  float64(rand.Intn(10) + 1),             //nolint:gosec // Mock data generation
		Temperature: float64(rand.Intn(30) + 50),            //nolint:gosec // Mock data generation
		WindSpeed:   float64(rand.Intn(20)),                 //nolint:gosec // Mock data generation
	}
}

// TrafficService provides traffic data.
type TrafficService struct{}

func (*TrafficService) GetTraffic(_, _ float64) TrafficConditions {
	levels := []string{"light", "moderate", "heavy"}
	return TrafficConditions{
		Level:    levels[rand.Intn(len(levels))], //nolint:gosec // Mock data generation
		AvgSpeed: float64(rand.Intn(40) + 20),    //nolint:gosec // Mock data generation
	}
}

// NotificationService sends alerts.
type NotificationService struct {
	sentAlerts []string
	mu         sync.Mutex
}

func NewNotificationService() *NotificationService {
	return &NotificationService{
		sentAlerts: make([]string, 0),
	}
}

func (n *NotificationService) SendAlert(recipient, message string) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	alert := fmt.Sprintf("[%s] To: %s - %s", time.Now().Format("15:04:05"), recipient, message)
	n.sentAlerts = append(n.sentAlerts, alert)

	// Simulate sending.
	fmt.Printf("📨 ALERT: %s\n", alert)
	return nil
}

func (n *NotificationService) GetSentAlerts() []string {
	n.mu.Lock()
	defer n.mu.Unlock()

	result := make([]string, len(n.sentAlerts))
	copy(result, n.sentAlerts)
	return result
}

// Global instances.
var (
	vehicleRepo         *VehicleRepository
	driverRepo          *DriverRepository
	maintenanceRepo     *MaintenanceRepository
	routeRepo           *RouteRepository
	videoService        *VideoService
	weatherService      *WeatherService
	trafficService      *TrafficService
	notificationService *NotificationService
)

func InitializeMockServices() {
	vehicleRepo = NewVehicleRepository()
	driverRepo = NewDriverRepository()
	maintenanceRepo = NewMaintenanceRepository()
	routeRepo = NewRouteRepository()
	videoService = &VideoService{}
	weatherService = &WeatherService{}
	trafficService = &TrafficService{}
	notificationService = NewNotificationService()
}
