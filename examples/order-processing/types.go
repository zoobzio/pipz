package main

import (
	"fmt"
	"time"
)

// Constants for repeated strings to satisfy goconst linter.
const (
	// Customer tier constants.
	TierBronze   = "bronze"
	TierSilver   = "silver"
	TierGold     = "gold"
	TierPlatinum = "platinum"

	// Risk level constants.
	RiskTrusted = "trusted"
	RiskHigh    = "high_risk"
	RiskMedium  = "medium_risk"
	RiskLow     = "low_risk"

	// Status text constants.
	StatusCanceledText = "canceled" // US spelling.
)

// Order represents an e-commerce order flowing through our system.
//
//nolint:govet // demo struct, alignment not critical.
type Order struct {
	// Basic order info.
	ID          string
	CustomerID  string
	Items       []OrderItem
	TotalAmount float64

	// State that evolves through the pipeline.
	Status         OrderStatus
	PaymentID      string  // Set after successful payment.
	ReservationID  string  // Set after inventory reservation.
	TrackingNumber string  // Set after shipping.
	FraudScore     float64 // Set during risk assessment.

	// Processing metadata.
	CreatedAt     time.Time
	ProcessingLog []string
	Timestamps    map[string]time.Time // Track when each stage completed.

	// Customer info (enriched during processing).
	CustomerTier   string // "bronze", "silver", "gold", "platinum".
	PreviousOrders int
	IsFirstOrder   bool
}

// Clone implements Cloner for concurrent processing.
// IMPORTANT: Deep copy all mutable fields to prevent race conditions.
func (o Order) Clone() Order {
	// Dev: "Found this out the hard way when concurrent notifications.
	// were all modifying the same ProcessingLog slice!".

	// Deep copy items.
	items := make([]OrderItem, len(o.Items))
	copy(items, o.Items)

	// Deep copy processing log.
	log := make([]string, len(o.ProcessingLog))
	copy(log, o.ProcessingLog)

	// Deep copy timestamps.
	timestamps := make(map[string]time.Time, len(o.Timestamps))
	for k, v := range o.Timestamps {
		timestamps[k] = v
	}

	return Order{
		ID:             o.ID,
		CustomerID:     o.CustomerID,
		Items:          items,
		TotalAmount:    o.TotalAmount,
		Status:         o.Status,
		PaymentID:      o.PaymentID,
		ReservationID:  o.ReservationID,
		TrackingNumber: o.TrackingNumber,
		FraudScore:     o.FraudScore,
		CreatedAt:      o.CreatedAt,
		ProcessingLog:  log,
		Timestamps:     timestamps,
		CustomerTier:   o.CustomerTier,
		PreviousOrders: o.PreviousOrders,
		IsFirstOrder:   o.IsFirstOrder,
	}
}

// OrderItem represents a single item in an order.
//
//nolint:govet // demo struct, alignment not critical.
type OrderItem struct {
	ProductID   string
	ProductName string
	Quantity    int
	Price       float64
}

// OrderStatus represents the current state of an order.
type OrderStatus int

const (
	StatusPending OrderStatus = iota
	StatusValidated
	StatusInventoryReserved
	StatusPaymentProcessing
	StatusPaid
	StatusFulfilling
	StatusShipped
	StatusDelivered
	StatusCanceled
	StatusRefunded
)

func (s OrderStatus) String() string {
	switch s {
	case StatusPending:
		return "pending"
	case StatusValidated:
		return "validated"
	case StatusInventoryReserved:
		return "inventory_reserved"
	case StatusPaymentProcessing:
		return "payment_processing"
	case StatusPaid:
		return "paid"
	case StatusFulfilling:
		return "fulfilling"
	case StatusShipped:
		return "shipped"
	case StatusDelivered:
		return "delivered"
	case StatusCanceled:
		return StatusCanceledText
	case StatusRefunded:
		return "refunded"
	default:
		return "unknown"
	}
}

// Processor and Connector names as constants.
const (
	// Order processing stages.
	ProcessorValidateOrder       = "validate_order"
	ProcessorEnrichCustomer      = "enrich_customer"
	ProcessorCalculateFraudScore = "calculate_fraud_score"
	ProcessorManualReview        = "manual_review"
	ProcessorReserveInventory    = "reserve_inventory"
	ProcessorChargePayment       = "charge_payment"
	ProcessorConfirmInventory    = "confirm_inventory"
	ProcessorReleaseInventory    = "release_inventory"
	ProcessorSaveOrder           = "save_order"
	ProcessorQueueForShipping    = "queue_shipping"

	// Notification processors.
	ProcessorSendOrderEmail         = "send_order_email"
	ProcessorSendOrderSMS           = "send_order_sms"
	ProcessorUpdateAnalytics        = "update_analytics"
	ProcessorNotifyWarehouse        = "notify_warehouse"
	ProcessorUpdateLoyaltyPoints    = "update_loyalty"
	ProcessorTriggerRecommendations = "trigger_recommendations"
	ProcessorNotifyPaymentIssue     = "notify_payment_issue"
	ProcessorAdditionalVerification = "additional_verification"
	ProcessorSaveAsPending          = "save_pending"

	// Error handling processors.
	ProcessorOrderFailureHandler = "order_failure_handler"
	ProcessorInventoryCleanup    = "inventory_cleanup"
	ProcessorHighRiskHandler     = "high_risk_handler"
	ProcessorPaymentErrorHandler = "payment_error_handler"
	ProcessorLogError            = "log_error"
	ProcessorRefundPayment       = "refund_payment"

	// Pipeline names.
	PipelineOrderProcessing   = "order_processing"
	PipelineHighRisk          = "high_risk_orders"
	PipelineMediumRisk        = "medium_risk_orders"
	PipelineLowRisk           = "low_risk_orders"
	PipelineNotifications     = "notifications"
	PipelineInventoryRollback = "inventory_rollback"

	// Connector names.
	ConnectorRiskRouter            = "risk_based_router"
	ConnectorPaymentWithFallback   = "payment_with_fallback"
	ConnectorNotificationBroadcast = "notification_broadcast"
	ConnectorInventoryTimeout      = "inventory_timeout"
	ConnectorPaymentRetry          = "payment_retry"
	ConnectorCompleteNotifications = "complete_notifications"
)

// MetricsSummary tracks order processing metrics.
//
//nolint:govet // demo struct, alignment not critical.
type MetricsSummary struct {
	TotalOrders        int
	SuccessfulOrders   int
	FailedOrders       int
	TotalRevenue       float64
	AverageOrderValue  float64
	AverageProcessTime time.Duration
	FraudPrevented     float64
	InventoryAccuracy  float64

	// Breakdown by status.
	StatusBreakdown map[OrderStatus]int

	// Performance metrics.
	FastestOrder time.Duration
	SlowestOrder time.Duration

	// Service health.
	PaymentSuccessRate float64
	InventoryHitRate   float64
}

// GenerateReport creates a formatted metrics report.
func (m MetricsSummary) GenerateReport() string {
	report := "\n=== Order Processing Metrics ===\n"
	report += fmt.Sprintf("Total Orders: %d\n", m.TotalOrders)
	report += fmt.Sprintf("Success Rate: %.1f%% (%d succeeded)\n",
		float64(m.SuccessfulOrders)/float64(m.TotalOrders)*100, m.SuccessfulOrders)
	report += fmt.Sprintf("Total Revenue: $%.2f\n", m.TotalRevenue)
	report += fmt.Sprintf("Average Order Value: $%.2f\n", m.AverageOrderValue)
	report += fmt.Sprintf("Average Process Time: %v\n", m.AverageProcessTime)

	if m.FraudPrevented > 0 {
		report += fmt.Sprintf("Fraud Prevented: $%.2f\n", m.FraudPrevented)
	}

	report += "\nPerformance:\n"
	report += fmt.Sprintf("  Fastest: %v\n", m.FastestOrder)
	report += fmt.Sprintf("  Slowest: %v\n", m.SlowestOrder)

	report += "\nService Health:\n"
	report += fmt.Sprintf("  Payment Success: %.1f%%\n", m.PaymentSuccessRate*100)
	report += fmt.Sprintf("  Inventory Accuracy: %.1f%%\n", m.InventoryAccuracy*100)

	return report
}
