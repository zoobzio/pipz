// Package workflow demonstrates using pipz for complex multi-stage business workflows
// with parallel execution, conditional routing, and compensating transactions.
package workflow

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/zoobzio/pipz"
)

// Order represents an e-commerce order flowing through the system
type Order struct {
	ID         string
	CustomerID string
	Items      []OrderItem

	// Order metadata
	Type          OrderType // express, standard, digital
	PaymentMethod PaymentMethod
	ShippingAddr  Address

	// Pricing
	Subtotal     float64
	Tax          float64
	ShippingCost float64
	Total        float64

	// State accumulated through pipeline
	Status            OrderStatus
	InventoryReserved bool
	ReservationIDs    []string
	FraudScore        float64
	FraudCheckPassed  bool
	PaymentID         string
	PaymentStatus     PaymentStatus
	ShippingCarrier   string
	ShippingLabel     string
	TrackingNumber    string

	// Timestamps
	CreatedAt   time.Time
	ValidatedAt time.Time
	PaidAt      time.Time
	ShippedAt   time.Time
	CompletedAt time.Time

	// Error tracking
	Errors []OrderError
}

type OrderItem struct {
	ProductID string
	SKU       string
	Name      string
	Quantity  int
	Price     float64
}

type OrderType string

const (
	OrderTypeExpress  OrderType = "express"
	OrderTypeStandard OrderType = "standard"
	OrderTypeDigital  OrderType = "digital"
)

type OrderStatus string

const (
	StatusPending    OrderStatus = "pending"
	StatusValidated  OrderStatus = "validated"
	StatusProcessing OrderStatus = "processing"
	StatusPaid       OrderStatus = "paid"
	StatusShipped    OrderStatus = "shipped"
	StatusCompleted  OrderStatus = "completed"
	StatusCancelled  OrderStatus = "cancelled"
	StatusFailed     OrderStatus = "failed"
)

type PaymentMethod string

const (
	PaymentCard   PaymentMethod = "card"
	PaymentWallet PaymentMethod = "wallet"
	PaymentBank   PaymentMethod = "bank"
)

type PaymentStatus string

const (
	PaymentPending  PaymentStatus = "pending"
	PaymentSuccess  PaymentStatus = "success"
	PaymentFailed   PaymentStatus = "failed"
	PaymentRefunded PaymentStatus = "refunded"
)

type Address struct {
	Street  string
	City    string
	State   string
	Zip     string
	Country string
}

type OrderError struct {
	Stage     string
	Error     string
	Timestamp time.Time
}

// Services (mock implementations)

// InventoryService manages product inventory
type InventoryService struct {
	stock    sync.Map // productID -> quantity
	reserved sync.Map // reservationID -> reservation
}

type Reservation struct {
	ID        string
	OrderID   string
	ProductID string
	Quantity  int
	ExpiresAt time.Time
}

func NewInventoryService() *InventoryService {
	svc := &InventoryService{}
	// Mock inventory
	svc.stock.Store("PROD-001", 100)
	svc.stock.Store("PROD-002", 50)
	svc.stock.Store("PROD-003", 200)
	svc.stock.Store("DIGITAL-001", 999999) // Unlimited digital goods
	return svc
}

func (s *InventoryService) CheckAvailability(items []OrderItem) (bool, []string) {
	var unavailable []string

	for _, item := range items {
		if stock, ok := s.stock.Load(item.ProductID); ok {
			if stock.(int) < item.Quantity {
				unavailable = append(unavailable, fmt.Sprintf("%s (need %d, have %d)",
					item.Name, item.Quantity, stock.(int)))
			}
		} else {
			unavailable = append(unavailable, fmt.Sprintf("%s (not found)", item.Name))
		}
	}

	return len(unavailable) == 0, unavailable
}

func (s *InventoryService) Reserve(orderID string, items []OrderItem) ([]string, error) {
	var reservationIDs []string

	// First check if all items are available
	available, unavailable := s.CheckAvailability(items)
	if !available {
		return nil, fmt.Errorf("items unavailable: %v", unavailable)
	}

	// Reserve items
	for _, item := range items {
		reservation := Reservation{
			ID:        fmt.Sprintf("RES-%s-%d", orderID, len(reservationIDs)),
			OrderID:   orderID,
			ProductID: item.ProductID,
			Quantity:  item.Quantity,
			ExpiresAt: time.Now().Add(30 * time.Minute),
		}

		// Decrease stock
		if stock, ok := s.stock.Load(item.ProductID); ok {
			newStock := stock.(int) - item.Quantity
			s.stock.Store(item.ProductID, newStock)
		}

		s.reserved.Store(reservation.ID, reservation)
		reservationIDs = append(reservationIDs, reservation.ID)
	}

	return reservationIDs, nil
}

func (s *InventoryService) ReleaseReservations(reservationIDs []string) error {
	for _, id := range reservationIDs {
		if res, ok := s.reserved.Load(id); ok {
			reservation := res.(Reservation)

			// Return stock
			if stock, ok := s.stock.Load(reservation.ProductID); ok {
				newStock := stock.(int) + reservation.Quantity
				s.stock.Store(reservation.ProductID, newStock)
			}

			s.reserved.Delete(id)
		}
	}
	return nil
}

// FraudService performs fraud detection
type FraudService struct {
	blacklist sync.Map
}

func NewFraudService() *FraudService {
	svc := &FraudService{}
	// Mock blacklist
	svc.blacklist.Store("FRAUD-USER-001", true)
	svc.blacklist.Store("127.0.0.1", true)
	return svc
}

func (s *FraudService) CheckFraud(order Order) (float64, error) {
	// Simulate fraud check
	time.Sleep(50 * time.Millisecond)

	score := 0.0

	// Check blacklist
	if _, blacklisted := s.blacklist.Load(order.CustomerID); blacklisted {
		score += 0.9
	}

	// High value orders
	if order.Total > 5000 {
		score += 0.3
	} else if order.Total > 1000 {
		score += 0.1
	}

	// New customer
	if rand.Float64() < 0.2 { // 20% are "new" customers
		score += 0.2
	}

	// Multiple items
	if len(order.Items) > 5 {
		score += 0.1
	}

	return math.Min(score, 1.0), nil
}

// PaymentService processes payments
type PaymentService struct {
	transactions sync.Map
}

func NewPaymentService() *PaymentService {
	return &PaymentService{}
}

func (s *PaymentService) ProcessPayment(order Order) (string, error) {
	// Simulate payment processing
	time.Sleep(100 * time.Millisecond)

	// Random failures for testing
	if rand.Float64() < 0.1 { // 10% failure rate
		return "", errors.New("payment declined: insufficient funds")
	}

	paymentID := fmt.Sprintf("PAY-%s-%d", order.ID, time.Now().Unix())
	s.transactions.Store(paymentID, map[string]interface{}{
		"orderID": order.ID,
		"amount":  order.Total,
		"status":  PaymentSuccess,
		"time":    time.Now(),
	})

	return paymentID, nil
}

func (s *PaymentService) RefundPayment(paymentID string) error {
	if tx, ok := s.transactions.Load(paymentID); ok {
		transaction := tx.(map[string]interface{})
		transaction["status"] = PaymentRefunded
		transaction["refundedAt"] = time.Now()
		s.transactions.Store(paymentID, transaction)
		return nil
	}
	return fmt.Errorf("payment %s not found", paymentID)
}

// ShippingService handles shipping
type ShippingService struct{}

func NewShippingService() *ShippingService {
	return &ShippingService{}
}

func (s *ShippingService) GetCarrier(orderType OrderType) string {
	switch orderType {
	case OrderTypeExpress:
		return "FedEx Priority"
	case OrderTypeStandard:
		return "USPS Ground"
	default:
		return "Digital Delivery"
	}
}

func (s *ShippingService) GenerateLabel(order Order) (string, string, error) {
	// Simulate API call
	time.Sleep(50 * time.Millisecond)

	label := fmt.Sprintf("LABEL-%s-%d", order.ID, time.Now().Unix())
	tracking := fmt.Sprintf("TRACK-%s-%d", order.ID, time.Now().Unix())

	return label, tracking, nil
}

// Pipeline Processors

// ValidateOrder ensures the order is valid
func ValidateOrder(_ context.Context, order Order) (Order, error) {
	// Check required fields
	if order.CustomerID == "" {
		return order, errors.New("missing customer ID")
	}

	if len(order.Items) == 0 {
		return order, errors.New("order has no items")
	}

	// Validate shipping address for physical orders
	if order.Type != OrderTypeDigital {
		if order.ShippingAddr.Street == "" || order.ShippingAddr.City == "" {
			return order, errors.New("invalid shipping address")
		}
	}

	// Calculate totals
	subtotal := 0.0
	for _, item := range order.Items {
		if item.Quantity <= 0 {
			return order, fmt.Errorf("invalid quantity for item %s", item.Name)
		}
		if item.Price <= 0 {
			return order, fmt.Errorf("invalid price for item %s", item.Name)
		}
		subtotal += float64(item.Quantity) * item.Price
	}

	order.Subtotal = subtotal
	order.Tax = subtotal * 0.08 // 8% tax

	// Shipping cost
	switch order.Type {
	case OrderTypeExpress:
		order.ShippingCost = 25.0
	case OrderTypeStandard:
		order.ShippingCost = 5.0
	case OrderTypeDigital:
		order.ShippingCost = 0.0
	}

	order.Total = order.Subtotal + order.Tax + order.ShippingCost
	order.Status = StatusValidated
	order.ValidatedAt = time.Now()

	return order, nil
}

// ParallelChecks runs inventory and fraud checks concurrently
func ParallelChecks(inventory *InventoryService, fraud *FraudService) func(context.Context, Order) (Order, error) {
	return func(_ context.Context, order Order) (Order, error) {
		var inventoryErr, fraudErr error
		var reservationIDs []string
		var fraudScore float64

		var wg sync.WaitGroup
		wg.Add(2)

		// Inventory check and reservation
		go func() {
			defer wg.Done()
			reservationIDs, inventoryErr = inventory.Reserve(order.ID, order.Items)
			if inventoryErr == nil {
				order.InventoryReserved = true
				order.ReservationIDs = reservationIDs
			}
		}()

		// Fraud check
		go func() {
			defer wg.Done()
			fraudScore, fraudErr = fraud.CheckFraud(order)
			order.FraudScore = fraudScore
		}()

		wg.Wait()

		// Check results
		if inventoryErr != nil {
			return order, fmt.Errorf("inventory check failed: %w", inventoryErr)
		}

		if fraudErr != nil {
			// Release inventory if fraud check failed
			inventory.ReleaseReservations(order.ReservationIDs)
			order.InventoryReserved = false
			order.ReservationIDs = nil
			return order, fmt.Errorf("fraud check failed: %w", fraudErr)
		}

		// Evaluate fraud score
		if order.FraudScore > 0.7 {
			// High fraud risk - cancel order
			inventory.ReleaseReservations(order.ReservationIDs)
			order.InventoryReserved = false
			order.ReservationIDs = nil
			order.FraudCheckPassed = false
			return order, fmt.Errorf("order flagged as fraudulent (score: %.2f)", order.FraudScore)
		}

		order.FraudCheckPassed = true
		order.Status = StatusProcessing
		return order, nil
	}
}

// ProcessPayment handles payment with automatic rollback on failure
func ProcessPayment(payment *PaymentService, inventory *InventoryService) func(context.Context, Order) (Order, error) {
	return func(_ context.Context, order Order) (Order, error) {
		paymentID, err := payment.ProcessPayment(order)
		if err != nil {
			// Payment failed - release inventory
			if order.InventoryReserved {
				inventory.ReleaseReservations(order.ReservationIDs)
				order.InventoryReserved = false
			}
			order.PaymentStatus = PaymentFailed
			order.Errors = append(order.Errors, OrderError{
				Stage:     "payment",
				Error:     err.Error(),
				Timestamp: time.Now(),
			})
			return order, fmt.Errorf("payment processing failed: %w", err)
		}

		order.PaymentID = paymentID
		order.PaymentStatus = PaymentSuccess
		order.Status = StatusPaid
		order.PaidAt = time.Now()

		return order, nil
	}
}

// Fulfillment handlers for different order types

func FulfillExpressOrder(shipping *ShippingService) func(context.Context, Order) (Order, error) {
	return func(_ context.Context, order Order) (Order, error) {
		// Skip logging in benchmarks
		if order.ID != "bench" {
			fmt.Printf("[EXPRESS] Fast-tracking order %s\n", order.ID)
		}

		order.ShippingCarrier = shipping.GetCarrier(OrderTypeExpress)

		// Priority processing
		label, tracking, err := shipping.GenerateLabel(order)
		if err != nil {
			return order, fmt.Errorf("failed to generate express label: %w", err)
		}

		order.ShippingLabel = label
		order.TrackingNumber = tracking

		// Alert warehouse for priority pick
		if order.ID != "bench" {
			fmt.Printf("[EXPRESS] Priority pick requested for order %s\n", order.ID)
		}

		return order, nil
	}
}

func FulfillStandardOrder(shipping *ShippingService) func(context.Context, Order) (Order, error) {
	return func(_ context.Context, order Order) (Order, error) {
		if order.ID != "bench" {
			fmt.Printf("[STANDARD] Processing order %s\n", order.ID)
		}

		order.ShippingCarrier = shipping.GetCarrier(OrderTypeStandard)

		// Standard processing
		label, tracking, err := shipping.GenerateLabel(order)
		if err != nil {
			return order, fmt.Errorf("failed to generate label: %w", err)
		}

		order.ShippingLabel = label
		order.TrackingNumber = tracking

		return order, nil
	}
}

func FulfillDigitalOrder(_ context.Context, order Order) (Order, error) {
	if order.ID != "bench" {
		fmt.Printf("[DIGITAL] Processing digital order %s\n", order.ID)
	}

	// Generate licenses or download links
	for _, item := range order.Items {
		if order.ID != "bench" {
			fmt.Printf("[DIGITAL] Generating license for %s\n", item.Name)
		}
		// In real implementation, would generate actual licenses
	}

	order.ShippingCarrier = "Digital Delivery"
	order.TrackingNumber = "DIGITAL-" + order.ID

	// Mark as shipped immediately for digital goods
	order.Status = StatusShipped
	order.ShippedAt = time.Now()

	return order, nil
}

// FinalizeOrder performs final order completion tasks
func FinalizeOrder(inventory *InventoryService) func(context.Context, Order) (Order, error) {
	return func(_ context.Context, order Order) (Order, error) {
		// Update inventory (convert reservations to actual deductions)
		// In a real system, this would finalize the inventory changes

		if order.Type != OrderTypeDigital {
			order.Status = StatusShipped
			order.ShippedAt = time.Now()
		}

		order.Status = StatusCompleted
		order.CompletedAt = time.Now()

		if order.ID != "bench" {
			fmt.Printf("[COMPLETE] Order %s completed successfully\n", order.ID)
		}

		return order, nil
	}
}

// SendNotifications handles customer notifications
func SendNotifications(_ context.Context, order Order) error {
	if order.ID == "bench" {
		return nil
	}

	// Send order confirmation
	fmt.Printf("[NOTIFY] Sending order confirmation for %s to customer %s\n",
		order.ID, order.CustomerID)

	// Send shipping notification if applicable
	if order.TrackingNumber != "" {
		fmt.Printf("[NOTIFY] Shipping notification sent. Tracking: %s\n",
			order.TrackingNumber)
	}

	// Digital delivery
	if order.Type == OrderTypeDigital {
		fmt.Printf("[NOTIFY] Digital delivery email sent with download links\n")
	}

	return nil
}

// RecordMetrics tracks order processing metrics
func RecordMetrics(_ context.Context, order Order) error {
	processingTime := order.CompletedAt.Sub(order.CreatedAt)

	if order.ID != "bench" {
		fmt.Printf("[METRICS] Order %s processed in %v\n", order.ID, processingTime)
		fmt.Printf("[METRICS] Type: %s, Total: $%.2f, Items: %d\n",
			order.Type, order.Total, len(order.Items))
	}

	// In real implementation, would send to metrics system
	UpdateStats(order)

	return nil
}

// Pipeline Builders

// CreateOrderPipeline creates the main order processing pipeline
func CreateOrderPipeline(inventory *InventoryService, fraud *FraudService,
	payment *PaymentService, shipping *ShippingService) pipz.Chainable[Order] {

	// Route by order type
	routeByType := func(ctx context.Context, order Order) string {
		return string(order.Type)
	}

	// Fulfillment handlers
	fulfillmentHandlers := map[string]pipz.Chainable[Order]{
		string(OrderTypeExpress):  pipz.Apply("fulfill_express", FulfillExpressOrder(shipping)),
		string(OrderTypeStandard): pipz.Apply("fulfill_standard", FulfillStandardOrder(shipping)),
		string(OrderTypeDigital):  pipz.Apply("fulfill_digital", FulfillDigitalOrder),
	}

	return pipz.Sequential(
		// Validation
		pipz.Apply("validate_order", ValidateOrder),

		// Parallel checks (inventory + fraud)
		pipz.Apply("parallel_checks", ParallelChecks(inventory, fraud)),

		// Payment processing with rollback
		pipz.Apply("process_payment", ProcessPayment(payment, inventory)),

		// Route to appropriate fulfillment
		pipz.Switch(routeByType, fulfillmentHandlers),

		// Finalize order
		pipz.Apply("finalize_order", FinalizeOrder(inventory)),

		// Side effects
		pipz.Effect("send_notifications", SendNotifications),
		pipz.Effect("record_metrics", RecordMetrics),
	)
}

// CreateCompensatingPipeline handles order cancellation/rollback
func CreateCompensatingPipeline(inventory *InventoryService, payment *PaymentService) pipz.Chainable[Order] {
	return pipz.Sequential(
		// Release inventory if reserved
		pipz.Apply("release_inventory", func(ctx context.Context, order Order) (Order, error) {
			if order.InventoryReserved && len(order.ReservationIDs) > 0 {
				err := inventory.ReleaseReservations(order.ReservationIDs)
				if err != nil {
					return order, fmt.Errorf("failed to release inventory: %w", err)
				}
				order.InventoryReserved = false
				fmt.Printf("[ROLLBACK] Released inventory reservations for order %s\n", order.ID)
			}
			return order, nil
		}),

		// Refund payment if processed
		pipz.Apply("refund_payment", func(ctx context.Context, order Order) (Order, error) {
			if order.PaymentStatus == PaymentSuccess && order.PaymentID != "" {
				err := payment.RefundPayment(order.PaymentID)
				if err != nil {
					return order, fmt.Errorf("failed to refund payment: %w", err)
				}
				order.PaymentStatus = PaymentRefunded
				fmt.Printf("[ROLLBACK] Refunded payment %s for order %s\n",
					order.PaymentID, order.ID)
			}
			return order, nil
		}),

		// Update status
		pipz.Apply("mark_cancelled", func(ctx context.Context, order Order) (Order, error) {
			order.Status = StatusCancelled
			order.CompletedAt = time.Now()
			fmt.Printf("[ROLLBACK] Order %s cancelled\n", order.ID)
			return order, nil
		}),

		// Notify customer
		pipz.Effect("notify_cancellation", func(ctx context.Context, order Order) error {
			fmt.Printf("[NOTIFY] Sending cancellation notice for order %s\n", order.ID)
			return nil
		}),
	)
}

// Statistics

type OrderStats struct {
	Total          int64
	ByType         map[OrderType]int64
	ByStatus       map[OrderStatus]int64
	TotalRevenue   float64
	AverageValue   float64
	ProcessingTime time.Duration
	mu             sync.RWMutex
}

var globalStats = &OrderStats{
	ByType:   make(map[OrderType]int64),
	ByStatus: make(map[OrderStatus]int64),
}

func UpdateStats(order Order) {
	globalStats.mu.Lock()
	defer globalStats.mu.Unlock()

	globalStats.Total++
	globalStats.ByType[order.Type]++
	globalStats.ByStatus[order.Status]++
	globalStats.TotalRevenue += order.Total
	globalStats.AverageValue = globalStats.TotalRevenue / float64(globalStats.Total)

	if !order.CompletedAt.IsZero() && !order.CreatedAt.IsZero() {
		processingTime := order.CompletedAt.Sub(order.CreatedAt)
		// Simple average (in production, use weighted average)
		globalStats.ProcessingTime = time.Duration((int64(globalStats.ProcessingTime)*(globalStats.Total-1) +
			int64(processingTime)) / globalStats.Total)
	}
}

func GetStats() OrderStats {
	globalStats.mu.RLock()
	defer globalStats.mu.RUnlock()

	return OrderStats{
		Total:          globalStats.Total,
		ByType:         copyTypeMap(globalStats.ByType),
		ByStatus:       copyStatusMap(globalStats.ByStatus),
		TotalRevenue:   globalStats.TotalRevenue,
		AverageValue:   globalStats.AverageValue,
		ProcessingTime: globalStats.ProcessingTime,
	}
}

func copyTypeMap(m map[OrderType]int64) map[OrderType]int64 {
	copy := make(map[OrderType]int64)
	for k, v := range m {
		copy[k] = v
	}
	return copy
}

func copyStatusMap(m map[OrderStatus]int64) map[OrderStatus]int64 {
	copy := make(map[OrderStatus]int64)
	for k, v := range m {
		copy[k] = v
	}
	return copy
}

// Helper to create test orders
func CreateTestOrder(orderType OrderType, customerID string) Order {
	order := Order{
		ID:            fmt.Sprintf("ORD-%d", time.Now().UnixNano()),
		CustomerID:    customerID,
		Type:          orderType,
		PaymentMethod: PaymentCard,
		CreatedAt:     time.Now(),
		Status:        StatusPending,
		ShippingAddr: Address{
			Street:  "123 Main St",
			City:    "San Francisco",
			State:   "CA",
			Zip:     "94105",
			Country: "USA",
		},
	}

	// Add items based on type
	switch orderType {
	case OrderTypeDigital:
		order.Items = []OrderItem{
			{ProductID: "DIGITAL-001", SKU: "DIG-001", Name: "Software License", Quantity: 1, Price: 99.99},
		}
	case OrderTypeExpress:
		order.Items = []OrderItem{
			{ProductID: "PROD-001", SKU: "SKU-001", Name: "Premium Widget", Quantity: 2, Price: 149.99},
			{ProductID: "PROD-002", SKU: "SKU-002", Name: "Deluxe Gadget", Quantity: 1, Price: 299.99},
		}
	default:
		order.Items = []OrderItem{
			{ProductID: "PROD-001", SKU: "SKU-001", Name: "Standard Widget", Quantity: 1, Price: 49.99},
			{ProductID: "PROD-003", SKU: "SKU-003", Name: "Basic Tool", Quantity: 3, Price: 19.99},
		}
	}

	return order
}
