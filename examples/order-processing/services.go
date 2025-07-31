package main

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"time"
)

// ============================================================================.
// MOCK SERVICES FOR DEMONSTRATION PURPOSES ONLY.
// ============================================================================.
// These are simplified mock implementations to demonstrate the pipz library.
// In production, you would integrate with real payment providers (Stripe, etc.),.
// inventory systems (SAP, etc.), and notification services (SendGrid, Twilio).
//
// The mock services simulate:.
// - Variable latency and response times.
// - Service failures and timeouts.
// - Different error scenarios.
// - Realistic business logic.
//
// DO NOT use these mock services in production!.

// ServiceBehavior controls how mock services behave for testing.
// Dev: "We discovered these patterns from real incidents!".
type ServiceBehavior int

const (
	BehaviorNormal       ServiceBehavior = iota
	BehaviorSlow                         // Still works but 3x slower (payment provider issues).
	BehaviorTimeout                      // Hangs forever (network partition).
	BehaviorError                        // Always fails (service down).
	BehaviorIntermittent                 // 30% failure rate (degraded service).
	BehaviorThrottle                     // Rate limited responses.
)

// TestMode enables fast responses for testing.
var TestMode = false

// PaymentService handles payment processing.
//
//nolint:govet // demo struct, alignment not critical.
type PaymentService struct {
	Name     string
	Behavior ServiceBehavior
	mu       sync.Mutex

	// Track some basic metrics.
	attemptCount int
	successCount int
	totalRevenue float64
}

// Charge attempts to charge the customer for an order.
func (p *PaymentService) Charge(ctx context.Context, order Order) (string, error) {
	p.mu.Lock()
	p.attemptCount++
	p.mu.Unlock()

	// Check behavior.
	switch p.Behavior {
	case BehaviorTimeout:
		<-ctx.Done()
		return "", ctx.Err()

	case BehaviorError:
		return "", errors.New("payment service unavailable")

	case BehaviorIntermittent:
		if rand.Float32() < 0.3 { //nolint:gosec // mock service uses weak random.
			return "", errors.New("payment gateway timeout")
		}

	case BehaviorThrottle:
		// Simulate rate limiting.
		if p.attemptCount%10 == 0 {
			return "", errors.New("rate limit exceeded")
		}
	}

	// Simulate processing time.
	baseLatency := 2 * time.Second
	if TestMode {
		baseLatency = 20 * time.Millisecond
	} else if p.Behavior == BehaviorSlow {
		baseLatency = 6 * time.Second
	}

	select {
	case <-time.After(baseLatency + time.Duration(rand.Intn(500))*time.Millisecond): //nolint:gosec // mock service uses weak random.
		// Success.
		p.mu.Lock()
		p.successCount++
		p.totalRevenue += order.TotalAmount
		p.mu.Unlock()

		// Generate payment ID.
		paymentID := fmt.Sprintf("PAY-%s-%d", order.ID, time.Now().Unix())
		return paymentID, nil

	case <-ctx.Done():
		return "", ctx.Err()
	}
}

// Refund reverses a payment.
func (p *PaymentService) Refund(ctx context.Context, _ string, amount float64) error {
	// Simulate refund processing.
	if p.Behavior == BehaviorError {
		return errors.New("refund service unavailable")
	}

	latency := 1 * time.Second
	if TestMode {
		latency = 10 * time.Millisecond
	}

	select {
	case <-time.After(latency):
		p.mu.Lock()
		p.totalRevenue -= amount
		p.mu.Unlock()
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// InventoryService manages product inventory.
//
//nolint:govet // demo struct, alignment not critical.
type InventoryService struct {
	Name     string
	Behavior ServiceBehavior
	mu       sync.RWMutex

	// Simple inventory tracking.
	stock        map[string]int
	reservations map[string]map[string]int // reservationID -> productID -> quantity.
}

// Reserve attempts to reserve inventory for an order.
func (i *InventoryService) Reserve(ctx context.Context, order Order) (string, error) {
	i.mu.Lock()
	defer i.mu.Unlock()

	// Check behavior.
	if i.Behavior == BehaviorError {
		return "", errors.New("inventory service down")
	}

	// Simulate processing.
	latency := 300 * time.Millisecond
	if TestMode {
		latency = 5 * time.Millisecond
	} else if i.Behavior == BehaviorSlow {
		latency = 2 * time.Second
	}

	select {
	case <-time.After(latency):
		// Check stock for all items.
		for _, item := range order.Items {
			available := i.stock[item.ProductID]
			if available < item.Quantity {
				return "", fmt.Errorf("insufficient stock for %s: need %d, have %d",
					item.ProductName, item.Quantity, available)
			}
		}

		// Reserve the items.
		reservationID := fmt.Sprintf("RES-%s-%d", order.ID, time.Now().Unix())
		i.reservations[reservationID] = make(map[string]int)

		for _, item := range order.Items {
			i.stock[item.ProductID] -= item.Quantity
			i.reservations[reservationID][item.ProductID] = item.Quantity
		}

		return reservationID, nil

	case <-ctx.Done():
		return "", ctx.Err()
	}
}

// Confirm converts a reservation into a permanent allocation.
func (i *InventoryService) Confirm(_ context.Context, reservationID string) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	if _, exists := i.reservations[reservationID]; !exists {
		return fmt.Errorf("reservation %s not found", reservationID)
	}

	// Simply remove from reservations (already deducted from stock).
	delete(i.reservations, reservationID)
	return nil
}

// Release cancels a reservation and returns items to stock.
func (i *InventoryService) Release(_ context.Context, reservationID string) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	reservation, exists := i.reservations[reservationID]
	if !exists {
		return nil // Already released or confirmed.
	}

	// Return items to stock.
	for productID, quantity := range reservation {
		i.stock[productID] += quantity
	}

	delete(i.reservations, reservationID)
	return nil
}

// NotificationService handles various customer notifications.
//
//nolint:govet // demo struct, alignment not critical.
type NotificationService struct {
	Name     string
	Type     string // "email", "sms", "analytics", etc.
	Behavior ServiceBehavior
	mu       sync.Mutex

	sentCount int
}

// Send sends a notification (fire and forget).
func (n *NotificationService) Send(ctx context.Context, _ Order) error {
	// Notification services are often fire-and-forget.
	// We don't want them to block order processing.

	if n.Behavior == BehaviorError {
		return fmt.Errorf("%s service unavailable", n.Type)
	}

	// Simulate sending.
	latency := 500 * time.Millisecond
	switch {
	case TestMode:
		latency = 5 * time.Millisecond
	case n.Type == "email":
		latency = 1 * time.Second // Email is slower.
	case n.Behavior == BehaviorSlow:
		latency = 3 * time.Second
	}

	// Add some randomness.
	latency += time.Duration(rand.Intn(200)) * time.Millisecond //nolint:gosec // mock service uses weak random.

	select {
	case <-time.After(latency):
		n.mu.Lock()
		n.sentCount++
		n.mu.Unlock()
		return nil

	case <-ctx.Done():
		return ctx.Err()
	}
}

// DatabaseService simulates order database.
//
//nolint:govet // demo struct, alignment not critical.
type DatabaseService struct {
	Name     string
	Behavior ServiceBehavior
	mu       sync.RWMutex

	orders map[string]Order
}

// Save stores an order in the database.
func (d *DatabaseService) Save(ctx context.Context, order Order) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.Behavior == BehaviorError {
		return errors.New("database connection failed")
	}

	// Simulate write latency.
	latency := 100 * time.Millisecond
	if TestMode {
		latency = 2 * time.Millisecond
	} else if d.Behavior == BehaviorSlow {
		latency = 1 * time.Second
	}

	select {
	case <-time.After(latency):
		d.orders[order.ID] = order
		return nil

	case <-ctx.Done():
		return ctx.Err()
	}
}

// CustomerService provides customer information.
//
//nolint:govet // demo struct, alignment not critical.
type CustomerService struct {
	Name     string
	Behavior ServiceBehavior
}

// GetCustomerInfo enriches order with customer data.
func (c *CustomerService) GetCustomerInfo(ctx context.Context, customerID string) (tier string, previousOrders int, err error) {
	if c.Behavior == BehaviorError {
		return "", 0, errors.New("customer service unavailable")
	}

	// Simulate lookup.
	latency := 200 * time.Millisecond
	if TestMode {
		latency = 5 * time.Millisecond
	}

	select {
	case <-time.After(latency):
		// Mock customer tiers based on ID.
		hash := 0
		for _, ch := range customerID {
			hash += int(ch)
		}

		switch hash % 4 {
		case 0:
			return TierBronze, hash % 5, nil
		case 1:
			return TierSilver, (hash % 10) + 5, nil
		case 2:
			return TierGold, (hash % 20) + 10, nil
		default:
			return TierPlatinum, (hash % 30) + 20, nil
		}

	case <-ctx.Done():
		return "", 0, ctx.Err()
	}
}

// FraudService calculates fraud risk scores.
//
//nolint:govet // demo struct, alignment not critical.
type FraudService struct {
	Name     string
	Behavior ServiceBehavior
}

// CalculateRisk returns a fraud risk score (0.0 - 1.0).
func (f *FraudService) CalculateRisk(ctx context.Context, order Order) (float64, error) {
	if f.Behavior == BehaviorError {
		// Default to medium risk if service is down.
		return 0.5, nil
	}

	// Simulate calculation.
	latency := 500 * time.Millisecond
	if TestMode {
		latency = 10 * time.Millisecond
	}

	select {
	case <-time.After(latency):
		// Mock risk calculation.
		score := 0.1 // Base score.

		// High-value orders are riskier.
		if order.TotalAmount > 1000 {
			score += 0.3
		} else if order.TotalAmount > 500 {
			score += 0.2
		}

		// First-time customers are riskier.
		if order.IsFirstOrder {
			score += 0.2
		}

		// Bulk orders are suspicious.
		bulkFound := false
		for _, item := range order.Items {
			if item.Quantity > 10 {
				score += 0.2
				bulkFound = true
				break
			}
		}

		// Debug logging.
		if TestMode {
			fmt.Printf("[DEBUG] Fraud calculation for %s: Base=0.1, Amount=%.2f, FirstOrder=%v, BulkOrder=%v => Score=%.2f\n",
				order.ID, order.TotalAmount, order.IsFirstOrder, bulkFound, score)
		}

		// Cap at 1.0.
		if score > 1.0 {
			score = 1.0
		}

		return score, nil

	case <-ctx.Done():
		return 0.5, ctx.Err()
	}
}

// Mock service instances - FOR DEMONSTRATION ONLY.
var (
	// Payment service.
	PaymentGateway = &PaymentService{
		Name: "stripe",
	}

	// Inventory service with some mock stock.
	Inventory = &InventoryService{
		Name: "inventory",
		stock: map[string]int{
			"LAPTOP-001":     50,
			"PHONE-001":      100,
			"TABLET-001":     75,
			"WATCH-001":      200,
			"HEADPHONES-001": 150,
		},
		reservations: make(map[string]map[string]int),
	}

	// Notification services.
	EmailService = &NotificationService{
		Name: "sendgrid",
		Type: "email",
	}

	SMSService = &NotificationService{
		Name: "twilio",
		Type: "sms",
	}

	AnalyticsService = &NotificationService{
		Name: "segment",
		Type: "analytics",
	}

	WarehouseService = &NotificationService{
		Name: "warehouse",
		Type: "fulfillment",
	}

	// Database.
	OrderDB = &DatabaseService{
		Name:   "postgres",
		orders: make(map[string]Order),
	}

	// Customer service.
	CustomerAPI = &CustomerService{
		Name: "customer-api",
	}

	// Fraud detection.
	FraudDetection = &FraudService{
		Name: "fraud-guard",
	}

	// Metrics collector.
	MetricsCollector = &OrderMetrics{
		orders: make([]Order, 0),
	}
)

// OrderMetrics tracks order processing metrics.
//
//nolint:govet // demo struct, alignment not critical.
type OrderMetrics struct {
	mu     sync.Mutex
	orders []Order
}

func (m *OrderMetrics) Record(order Order) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.orders = append(m.orders, order)
}

func (m *OrderMetrics) GetSummary() MetricsSummary {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.orders) == 0 {
		return MetricsSummary{}
	}

	summary := MetricsSummary{
		TotalOrders:     len(m.orders),
		StatusBreakdown: make(map[OrderStatus]int),
	}

	var totalRevenue float64
	var totalProcessTime time.Duration
	var successCount int

	for i := range m.orders {
		summary.StatusBreakdown[m.orders[i].Status]++

		if m.orders[i].Status == StatusPaid || m.orders[i].Status == StatusShipped || m.orders[i].Status == StatusFulfilling {
			successCount++
			totalRevenue += m.orders[i].TotalAmount
		}

		// Calculate process time.
		if endTime, exists := m.orders[i].Timestamps["completed"]; exists {
			processTime := endTime.Sub(m.orders[i].CreatedAt)
			totalProcessTime += processTime

			if i == 0 || processTime < summary.FastestOrder {
				summary.FastestOrder = processTime
			}
			if i == 0 || processTime > summary.SlowestOrder {
				summary.SlowestOrder = processTime
			}
		}

		// Track fraud prevented.
		if m.orders[i].FraudScore > 0.8 && m.orders[i].Status == StatusCanceled {
			summary.FraudPrevented += m.orders[i].TotalAmount
		}
	}

	summary.SuccessfulOrders = successCount
	summary.FailedOrders = summary.TotalOrders - successCount
	summary.TotalRevenue = totalRevenue

	if successCount > 0 {
		summary.AverageOrderValue = totalRevenue / float64(successCount)
		summary.AverageProcessTime = totalProcessTime / time.Duration(successCount)
	}

	// Calculate service health.
	summary.PaymentSuccessRate = float64(PaymentGateway.successCount) / float64(PaymentGateway.attemptCount)

	// Inventory accuracy (simplified).
	summary.InventoryAccuracy = 0.95 // Mock 95% accuracy.

	return summary
}

func (m *OrderMetrics) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.orders = make([]Order, 0)
}

// Helper functions to control service behaviors.
func SetAllServicesNormal() {
	PaymentGateway.Behavior = BehaviorNormal
	Inventory.Behavior = BehaviorNormal
	EmailService.Behavior = BehaviorNormal
	SMSService.Behavior = BehaviorNormal
	AnalyticsService.Behavior = BehaviorNormal
	WarehouseService.Behavior = BehaviorNormal
	OrderDB.Behavior = BehaviorNormal
	CustomerAPI.Behavior = BehaviorNormal
	FraudDetection.Behavior = BehaviorNormal
}

// ResetInventory restores inventory to initial state.
func ResetInventory() {
	Inventory.mu.Lock()
	defer Inventory.mu.Unlock()

	Inventory.stock = map[string]int{
		"LAPTOP-001":     50,
		"PHONE-001":      100,
		"TABLET-001":     75,
		"WATCH-001":      200,
		"HEADPHONES-001": 150,
	}
	Inventory.reservations = make(map[string]map[string]int)
}
