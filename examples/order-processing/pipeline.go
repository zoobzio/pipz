package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/zoobzio/pipz"
)

// ============================================================================.
// ORDER PROCESSING PIPELINE - DEMONSTRATION EXAMPLE.
// ============================================================================.
// This example shows how to build a production-grade e-commerce order.
// processing system that evolves from a simple MVP to a sophisticated.
// system with fraud detection, inventory management, and notifications.
//
// WARNING: This uses mock services for demonstration. In production, you.
// would integrate with real payment providers, inventory systems, etc.
//
// The example demonstrates:.
// - State evolution as order moves through pipeline.
// - Concurrent side effects for notifications.
// - Smart routing based on fraud risk.
// - Compensation patterns for failures.
// - Real business metrics and impact.

// ============================================================================.
// SPRINT 1: MVP - Just Take Payment! (Week 1).
// ============================================================================.
// CEO: "We need to start taking orders TODAY! Just make it work!".
// Dev: "But what about inventory? Fraud? Notifications?".
// CEO: "We'll add that later. Ship it!".
//
// Requirements:.
// - Validate the order.
// - Charge payment.
// - Save to database.
//
// Dev at 6 PM: "Done! Going home early for once... 🎉".

var (
	// ValidateOrder ensures the order is valid.
	// Added: Sprint 1 - Basic validation.
	ValidateOrder = pipz.Apply(ProcessorValidateOrder, func(_ context.Context, order Order) (Order, error) {
		if len(order.Items) == 0 {
			return order, errors.New("order has no items")
		}

		if order.TotalAmount <= 0 {
			return order, errors.New("invalid order amount")
		}

		order.Status = StatusValidated
		order.ProcessingLog = append(order.ProcessingLog, "order validated")
		order.Timestamps["validated"] = time.Now()

		return order, nil
	})

	// ChargePayment processes payment for the order.
	// Added: Sprint 1 - Core requirement.
	ChargePayment = pipz.Apply(ProcessorChargePayment, func(ctx context.Context, order Order) (Order, error) {
		order.Status = StatusPaymentProcessing

		paymentID, err := PaymentGateway.Charge(ctx, order)
		if err != nil {
			order.Status = StatusCanceled
			return order, fmt.Errorf("payment failed: %w", err)
		}

		order.PaymentID = paymentID
		order.Status = StatusPaid
		order.ProcessingLog = append(order.ProcessingLog, fmt.Sprintf("payment processed: %s", paymentID))
		order.Timestamps["paid"] = time.Now()

		return order, nil
	})

	// SaveOrder persists the order to database.
	// Added: Sprint 1 - Need to record orders.
	SaveOrder = pipz.Effect(ProcessorSaveOrder, func(ctx context.Context, order Order) error {
		order.Timestamps["saved"] = time.Now()
		return OrderDB.Save(ctx, order)
	})
)

// ============================================================================.
// SPRINT 2: Inventory Crisis (Week 2).
// ============================================================================.
// Support: "We're getting angry calls! People paid but items are out of stock!".
// Finance: "We had to refund $50k last week for oversold items!".
// Dev: "I told you we needed inventory management... 😤".
//
// THE MATH: $50k in refunds × 3% processing fee = $1,500 lost per week.
// Plus: Angry customers, bad reviews, support costs.
//
// Requirements:.
// - Check inventory before payment.
// - Reserve items during checkout.
// - Release if payment fails.

var (
	// ReserveInventory holds items during checkout.
	// Added: Sprint 2 - Prevent overselling.
	ReserveInventory = pipz.Apply(ProcessorReserveInventory, func(ctx context.Context, order Order) (Order, error) {
		reservationID, err := Inventory.Reserve(ctx, order)
		if err != nil {
			order.Status = StatusCanceled
			return order, fmt.Errorf("inventory reservation failed: %w", err)
		}

		order.ReservationID = reservationID
		order.Status = StatusInventoryReserved
		order.ProcessingLog = append(order.ProcessingLog, fmt.Sprintf("inventory reserved: %s", reservationID))
		order.Timestamps["inventory_reserved"] = time.Now()

		return order, nil
	})

	// ConfirmInventory converts reservation to allocation.
	// Added: Sprint 2 - Finalize inventory after payment.
	ConfirmInventory = pipz.Effect(ProcessorConfirmInventory, func(ctx context.Context, order Order) error {
		if order.ReservationID == "" {
			return nil // No reservation to confirm.
		}

		err := Inventory.Confirm(ctx, order.ReservationID)
		if err != nil {
			return fmt.Errorf("inventory confirmation failed: %w", err)
		}

		order.ProcessingLog = append(order.ProcessingLog, "inventory confirmed")
		return nil
	})

	// ReleaseInventory returns items to stock on failure.
	// Added: Sprint 2 - Cleanup on payment failure.
	ReleaseInventory = pipz.Effect(ProcessorReleaseInventory, func(ctx context.Context, order Order) error {
		if order.ReservationID == "" {
			return nil // No reservation to release.
		}

		err := Inventory.Release(ctx, order.ReservationID)
		if err != nil {
			// Log but don't fail - inventory will timeout eventually.
			order.ProcessingLog = append(order.ProcessingLog, fmt.Sprintf("inventory release failed: %v", err))
		} else {
			order.ProcessingLog = append(order.ProcessingLog, "inventory released")
		}

		return nil
	})
)

// ============================================================================.
// SPRINT 3: Customer Communications (Week 3).
// ============================================================================.
// Support Manager: "Customers keep calling! 'Where's my order?' 'Did payment go through?'".
// Marketing: "We need order confirmations for customer satisfaction!".
// CEO: "And I want analytics on everything!".
//
// Requirements:.
// - Email confirmations.
// - SMS for high-value orders.
// - Analytics tracking.
// - Warehouse notifications.
//
// Dev: "Great, now I need to integrate with 4 different services... ☕".

var (
	// SendOrderEmail sends confirmation email.
	// Added: Sprint 3 - Customer communication.
	SendOrderEmail = pipz.Effect(ProcessorSendOrderEmail, func(ctx context.Context, order Order) error {
		return EmailService.Send(ctx, order)
	})

	// SendOrderSMS sends SMS for high-value orders.
	// Added: Sprint 3 - Premium customer experience.
	SendOrderSMS = pipz.Effect(ProcessorSendOrderSMS, func(ctx context.Context, order Order) error {
		// Only for high-value orders or premium customers.
		if order.TotalAmount < 500 && order.CustomerTier != TierPlatinum {
			return nil
		}
		return SMSService.Send(ctx, order)
	})

	// UpdateAnalytics tracks order metrics.
	// Added: Sprint 3 - Business intelligence.
	UpdateAnalytics = pipz.Effect(ProcessorUpdateAnalytics, func(ctx context.Context, order Order) error {
		return AnalyticsService.Send(ctx, order)
	})

	// NotifyWarehouse queues order for fulfillment.
	// Added: Sprint 3 - Start shipping process.
	NotifyWarehouse = pipz.Effect(ProcessorNotifyWarehouse, func(ctx context.Context, order Order) error {
		return WarehouseService.Send(ctx, order)
	})

	// NotificationBroadcast sends all notifications in parallel.
	// Added: Sprint 3 - Don't make customer wait for emails!.
	// Before: Sequential notifications = 2.5s total.
	// After: Concurrent notifications = 1s (fastest one).
	// Performance gain: 60% faster checkout!.
	NotificationBroadcast = pipz.NewConcurrent[Order](
		ConnectorNotificationBroadcast,
		SendOrderEmail,  // ~1s.
		SendOrderSMS,    // ~500ms.
		UpdateAnalytics, // ~200ms.
		NotifyWarehouse, // ~300ms.
	)
	// Dev: "This is why I love pipz - parallel notifications with one line!".
)

// ============================================================================.
// SPRINT 4: Fraud Detection (Week 4).
// ============================================================================.
// 3 AM CALL: "We just lost $50k to stolen credit cards!".
// CEO: "How did this happen?!".
// Dev: "Remember when you said 'we'll add fraud detection later'? It's later.".
//
// THE MATH: $50k loss + $35 chargeback fee × 1,400 orders = $99k total loss.
//
// Requirements:.
// - Risk scoring for all orders.
// - Manual review for high risk.
// - Auto-approval for trusted customers.
// - Different processing paths based on risk.

var (
	// EnrichCustomer adds customer history to order.
	// Added: Sprint 4 - Need customer data for risk scoring.
	EnrichCustomer = pipz.Apply(ProcessorEnrichCustomer, func(ctx context.Context, order Order) (Order, error) {
		tier, previousOrders, err := CustomerAPI.GetCustomerInfo(ctx, order.CustomerID)
		if err != nil {
			// Don't fail order for this - just use defaults.
			order.CustomerTier = "bronze"
			order.PreviousOrders = 0
			if !order.IsFirstOrder { // Only set if not already set.
				order.IsFirstOrder = true
			}
		} else {
			order.CustomerTier = tier
			order.PreviousOrders = previousOrders
			// For testing, preserve explicitly set IsFirstOrder.
			if !order.IsFirstOrder && previousOrders == 0 {
				order.IsFirstOrder = true
			}
		}

		order.ProcessingLog = append(order.ProcessingLog,
			fmt.Sprintf("customer enriched: %s tier, %d previous orders", order.CustomerTier, order.PreviousOrders))

		return order, nil
	})

	// CalculateFraudScore assesses order risk.
	// Added: Sprint 4 - Core fraud detection.
	CalculateFraudScore = pipz.Apply(ProcessorCalculateFraudScore, func(ctx context.Context, order Order) (Order, error) {
		score, err := FraudDetection.CalculateRisk(ctx, order)
		if err != nil {
			// Default to medium risk if service fails.
			order.FraudScore = 0.5
		} else {
			order.FraudScore = score
		}

		order.ProcessingLog = append(order.ProcessingLog, fmt.Sprintf("fraud score: %.2f", order.FraudScore))

		// Debug logging.
		if TestMode && order.FraudScore >= 0.8 {
			fmt.Printf("[DEBUG] High risk order detected: ID=%s, Score=%.2f\n", order.ID, order.FraudScore)
		}

		return order, nil
	})

	// RiskBasedRouter routes orders based on fraud risk.
	// Added: Sprint 4 - Different paths for different risks.
	RiskBasedRouter = pipz.NewSwitch[Order](
		ConnectorRiskRouter,
		func(_ context.Context, order Order) string {
			// Platinum customers get fast track.
			if order.CustomerTier == TierPlatinum && order.FraudScore < 0.9 {
				return RiskTrusted
			}

			// Route based on risk score.
			switch {
			case order.FraudScore >= 0.8:
				return RiskHigh
			case order.FraudScore > 0.5:
				return RiskMedium
			default:
				return RiskLow
			}
		},
	)

	// ManualReview flags order for human review.
	// Added: Sprint 4 - High risk orders need human approval.
	ManualReview = pipz.Apply(ProcessorManualReview, func(_ context.Context, order Order) (Order, error) {
		order.Status = StatusCanceled
		order.ProcessingLog = append(order.ProcessingLog, "flagged for manual review")
		order.Timestamps["canceled"] = time.Now()
		// In real system, this would queue for review.
		return order, errors.New("order requires manual review")
	})

	// AdditionalVerification adds extra checks.
	// Added: Sprint 4 - Medium risk gets extra verification.
	AdditionalVerification = pipz.Apply(ProcessorAdditionalVerification, func(_ context.Context, order Order) (Order, error) {
		// In real system: 3DS, CVV check, address verification.
		order.ProcessingLog = append(order.ProcessingLog, "additional verification completed")
		return order, nil
	})
)

// ============================================================================.
// SPRINT 5: Payment Resilience (Week 5).
// ============================================================================.
// Black Friday: "Payment provider is down! We're losing $10k per minute!".
// Dev: "Good thing I built that fallback handler last sprint... oh wait, I didn't 😅".
//
// Requirements:.
// - Retry failed payments.
// - Clean up inventory on failure.
// - Don't lose orders during outages.
// - Graceful degradation.

var (
	// SaveAsPending saves order for later retry.
	// Added: Sprint 5 - Don't lose orders during outages.
	SaveAsPending = pipz.Apply(ProcessorSaveAsPending, func(ctx context.Context, order Order) (Order, error) {
		order.Status = StatusPending
		order.ProcessingLog = append(order.ProcessingLog, "saved for retry")
		err := OrderDB.Save(ctx, order)
		if err != nil {
			return order, err
		}
		// Return error to stop pipeline after saving.
		return order, errors.New("payment failed - order saved for retry")
	})

	// NotifyPaymentIssue informs customer of payment problem.
	// Added: Sprint 5 - Customer communication on failure.
	NotifyPaymentIssue = pipz.Effect(ProcessorNotifyPaymentIssue, func(ctx context.Context, order Order) error {
		// Send email about payment issue.
		return EmailService.Send(ctx, order)
	})

	// PaymentRetry adds retry logic to payment.
	// Added: Sprint 5 - Handle transient failures.
	// Stats: 65% of failed payments succeed on retry!.
	PaymentRetry = pipz.NewRetry(
		ConnectorPaymentRetry,
		ChargePayment,
		3, // 3 attempts total.
	)

	// PaymentFailureHandler handles payment failures gracefully.
	// Added: Sprint 5 - Clean up on payment failure.
	PaymentFailureHandler = pipz.NewSequence[Order](
		"payment_failure_handler",
		ReleaseInventory,   // Return items to stock.
		NotifyPaymentIssue, // Email customer.
		SaveAsPending,      // Save for manual retry.
	)

	// ChargeWithFallback provides payment resilience.
	// Added: Sprint 5 - Complete payment handling.
	ChargeWithFallback = pipz.NewFallback(
		ConnectorPaymentWithFallback,
		PaymentRetry,
		PaymentFailureHandler,
	)
)

// ============================================================================.
// SPRINT 6: Enterprise Features (Week 6).
// ============================================================================.
// CEO: "We're going enterprise! I need loyalty points, B2B support, and.
//       real-time recommendations!".
// Dev: "So basically rebuild everything?".
// CEO: "No, just add it to what we have!".
// Dev: "That's... actually possible with pipz! 🤯".
//
// Requirements:.
// - Loyalty point calculation.
// - B2B order handling.
// - AI recommendations.
// - Complete audit trail.
// - 99.99% uptime.

var (
	// UpdateLoyaltyPoints calculates and assigns points.
	// Added: Sprint 6 - Customer retention.
	UpdateLoyaltyPoints = pipz.Effect(ProcessorUpdateLoyaltyPoints, func(_ context.Context, order Order) error {
		// Simulate loyalty point calculation.
		points := int(order.TotalAmount * 10) // 10 points per dollar.

		// Premium customers get bonus.
		if order.CustomerTier == TierPlatinum {
			points = int(float64(points) * 1.5)
		}

		order.ProcessingLog = append(order.ProcessingLog, fmt.Sprintf("loyalty points: +%d", points))
		return nil
	})

	// TriggerRecommendations starts recommendation engine.
	// Added: Sprint 6 - AI-powered upsell.
	TriggerRecommendations = pipz.Effect(ProcessorTriggerRecommendations, func(_ context.Context, order Order) error {
		// In real system: ML model for recommendations.
		order.ProcessingLog = append(order.ProcessingLog, "recommendations queued")
		return nil
	})

	// QueueForShipping initiates fulfillment.
	// Added: Sprint 6 - Start shipping process.
	QueueForShipping = pipz.Effect(ProcessorQueueForShipping, func(_ context.Context, order Order) error {
		order.Status = StatusFulfilling
		order.TrackingNumber = fmt.Sprintf("TRACK-%s-%d", order.ID, time.Now().Unix())
		order.ProcessingLog = append(order.ProcessingLog, fmt.Sprintf("queued for shipping: %s", order.TrackingNumber))
		order.Timestamps["fulfilling"] = time.Now()
		return nil
	})

	// CompleteNotifications includes all notification types.
	// Added: Sprint 6 - Comprehensive notifications.
	CompleteNotifications = pipz.NewConcurrent[Order](
		ConnectorCompleteNotifications,
		SendOrderEmail,
		SendOrderSMS,
		UpdateAnalytics,
		NotifyWarehouse,
		UpdateLoyaltyPoints,
		TriggerRecommendations,
	)

	// OrderFailureHandler provides comprehensive error handling.
	// Added: Sprint 6 - Production-grade error recovery.
	OrderFailureHandler = pipz.Apply(ProcessorOrderFailureHandler, func(ctx context.Context, pipeErr *pipz.Error[Order]) (*pipz.Error[Order], error) {
		// Log the error.
		fmt.Printf("[ERROR] Order %s failed: %v at %v\n",
			pipeErr.InputData.ID, pipeErr.Err, pipeErr.Path)

		// Clean up based on order state.
		if pipeErr.InputData.ReservationID != "" {
			_ = Inventory.Release(ctx, pipeErr.InputData.ReservationID) //nolint:errcheck // best effort cleanup.
		}

		// Set status based on error type.
		if strings.Contains(pipeErr.Error(), "database") {
			// Database errors - save as pending for retry.
			pipeErr.InputData.Status = StatusPending
		} else {
			// Other errors - cancel the order.
			pipeErr.InputData.Status = StatusCanceled
		}

		pipeErr.InputData.Timestamps["failed"] = time.Now()
		pipeErr.InputData.ProcessingLog = append(pipeErr.InputData.ProcessingLog,
			fmt.Sprintf("pipeline failed: %v", pipeErr.Err))

		// Try to save failed order for analysis (skip if database is down).
		if !strings.Contains(pipeErr.Error(), "database") {
			_ = OrderDB.Save(ctx, pipeErr.InputData) //nolint:errcheck // best effort logging.
		}

		return pipeErr, nil
	})
)

// ============================================================================.
// PIPELINE DEFINITIONS.
// ============================================================================.

// OrderPipeline is our main pipeline that evolves through sprints.
var OrderPipeline = pipz.NewSequence[Order](
	PipelineOrderProcessing,
	ValidateOrder,
	ChargePayment,
	SaveOrder,
)

// Pipeline configurations for different risk levels.
var (
	// HighRiskPipeline for suspicious orders.
	HighRiskPipeline = pipz.NewSequence[Order](
		PipelineHighRisk,
		ManualReview, // Stop here for manual review.
	)

	// MediumRiskPipeline for orders needing extra verification.
	MediumRiskPipeline = pipz.NewSequence[Order](
		PipelineMediumRisk,
		AdditionalVerification,
		ReserveInventory,
		ChargeWithFallback,
		ConfirmInventory,
		SaveOrder,
	)

	// LowRiskPipeline for trusted customers.
	LowRiskPipeline = pipz.NewSequence[Order](
		PipelineLowRisk,
		ReserveInventory,
		ChargeWithFallback,
		ConfirmInventory,
		SaveOrder,
	)
)

// ResetPipeline resets to MVP state.
func ResetPipeline() {
	OrderPipeline.Clear()
	OrderPipeline.Register(
		ValidateOrder,
		ChargePayment,
		SaveOrder,
	)
}

// EnableInventoryManagement adds inventory control.
// Sprint 2: "No more overselling!".
func EnableInventoryManagement() {
	// Insert inventory reservation before payment.
	OrderPipeline.Clear()

	// Create a pipeline with error handling for inventory release.
	inventoryPipeline := pipz.NewHandle(
		"inventory_managed_pipeline",
		pipz.NewSequence[Order](
			"inner_inventory_pipeline",
			ValidateOrder,
			ReserveInventory,
			ChargePayment,
			ConfirmInventory,
			SaveOrder,
		),
		pipz.Apply(ProcessorInventoryCleanup, func(ctx context.Context, pipeErr *pipz.Error[Order]) (*pipz.Error[Order], error) {
			// Release inventory on failure.
			if pipeErr.InputData.ReservationID != "" {
				_ = Inventory.Release(ctx, pipeErr.InputData.ReservationID) //nolint:errcheck // best effort cleanup.
				pipeErr.InputData.ProcessingLog = append(pipeErr.InputData.ProcessingLog, "inventory released on failure")
			}
			return pipeErr, nil
		}),
	)

	OrderPipeline.Register(inventoryPipeline)
}

// EnableNotifications adds customer communications.
// Sprint 3: "Keep customers informed!".
func EnableNotifications() {
	// Need to rebuild pipeline with notifications.
	OrderPipeline.Clear()

	inventoryPipeline := pipz.NewHandle(
		"inventory_managed_pipeline",
		pipz.NewSequence[Order](
			"inner_inventory_pipeline",
			ValidateOrder,
			ReserveInventory,
			ChargePayment,
			ConfirmInventory,
			SaveOrder,
			NotificationBroadcast, // Add notifications here.
		),
		pipz.Apply(ProcessorInventoryCleanup, func(ctx context.Context, pipeErr *pipz.Error[Order]) (*pipz.Error[Order], error) {
			// Release inventory on failure.
			if pipeErr.InputData.ReservationID != "" {
				_ = Inventory.Release(ctx, pipeErr.InputData.ReservationID) //nolint:errcheck // best effort cleanup.
				pipeErr.InputData.ProcessingLog = append(pipeErr.InputData.ProcessingLog, "inventory released on failure")
			}
			return pipeErr, nil
		}),
	)

	OrderPipeline.Register(inventoryPipeline)
}

// EnableFraudDetection adds risk-based processing.
// Sprint 4: "Stop the fraud!".
func EnableFraudDetection() {
	OrderPipeline.Clear()

	// Create a fresh router with routes.
	router := pipz.NewSwitch[Order](
		ConnectorRiskRouter,
		func(_ context.Context, order Order) string {
			// Platinum customers get fast track.
			if order.CustomerTier == TierPlatinum && order.FraudScore < 0.9 {
				return RiskTrusted
			}

			// Route based on risk score.
			var route string
			switch {
			case order.FraudScore >= 0.8:
				route = RiskHigh
			case order.FraudScore > 0.5:
				route = RiskMedium
			default:
				route = RiskLow
			}

			// Debug logging.
			if TestMode {
				fmt.Printf("[DEBUG] Routing order %s: Score=%.2f, Route=%s\n", order.ID, order.FraudScore, route)
			}

			return route
		},
	)

	// Must add routes before registering.
	router = router.
		AddRoute(RiskHigh, HighRiskPipeline).
		AddRoute(RiskMedium, MediumRiskPipeline).
		AddRoute(RiskLow, LowRiskPipeline).
		AddRoute(RiskTrusted, LowRiskPipeline)

	// Wrap the entire fraud detection pipeline with error handling.
	fraudPipeline := pipz.NewHandle(
		"fraud_detection_with_error_handling",
		pipz.NewSequence[Order](
			"fraud_detection_inner",
			ValidateOrder,
			EnrichCustomer,
			CalculateFraudScore,
			router,
		),
		pipz.Apply(ProcessorPaymentErrorHandler, func(ctx context.Context, pipeErr *pipz.Error[Order]) (*pipz.Error[Order], error) {
			// Handle payment failures by saving for retry.
			if strings.Contains(pipeErr.Error(), "payment") {
				pipeErr.InputData.Status = StatusPending
				pipeErr.InputData.ProcessingLog = append(pipeErr.InputData.ProcessingLog, "saved for retry")

				// Save the order with pending status.
				if saveErr := OrderDB.Save(ctx, pipeErr.InputData); saveErr != nil {
					// If save fails, keep original error but log the save failure.
					pipeErr.InputData.ProcessingLog = append(pipeErr.InputData.ProcessingLog, fmt.Sprintf("save failed: %v", saveErr))
				}
			}
			// Handle manual review cases - preserve existing status and fraud score.
			if strings.Contains(pipeErr.Error(), "manual review") {
				// Set status to canceled as ManualReview intended.
				pipeErr.InputData.Status = StatusCanceled
				pipeErr.InputData.ProcessingLog = append(pipeErr.InputData.ProcessingLog, "order canceled pending manual review")
			}
			return pipeErr, nil
		}),
	)

	OrderPipeline.Register(fraudPipeline)

	// Debug logging.
	if TestMode {
		fmt.Println("[DEBUG] Fraud detection enabled with risk-based routing")
	}
}

// EnablePaymentResilience adds payment failure handling.
// Sprint 5: "Never lose an order!".
func EnablePaymentResilience() {
	// Already included in risk pipelines via ChargeWithFallback.
	// This is more about updating the existing pipelines.
	EnableFraudDetection() // Rebuild with resilient payment.
}

// EnableEnterpriseFeatures adds all production features.
// Sprint 6: "Enterprise ready!".
func EnableEnterpriseFeatures() {
	// Create a fresh router.
	router := pipz.NewSwitch[Order](
		ConnectorRiskRouter,
		func(_ context.Context, order Order) string {
			// Platinum customers get fast track.
			if order.CustomerTier == TierPlatinum && order.FraudScore < 0.9 {
				return RiskTrusted
			}

			// Route based on risk score.
			switch {
			case order.FraudScore >= 0.8:
				return RiskHigh
			case order.FraudScore > 0.5:
				return RiskMedium
			default:
				return RiskLow
			}
		},
	).
		AddRoute(RiskHigh, HighRiskPipeline).
		AddRoute(RiskMedium, MediumRiskPipeline).
		AddRoute(RiskLow, LowRiskPipeline).
		AddRoute(RiskTrusted, LowRiskPipeline)

	// Full production pipeline with all features.
	ProductionPipeline := pipz.NewHandle(
		"production_order_pipeline",
		pipz.NewSequence[Order](
			PipelineOrderProcessing,
			ValidateOrder,
			EnrichCustomer,
			CalculateFraudScore,
			router,
		),
		OrderFailureHandler,
	)

	// Replace the main pipeline.
	OrderPipeline.Clear()
	OrderPipeline.Register(ProductionPipeline)
}

// ProcessOrder handles an order through the pipeline.
func ProcessOrder(ctx context.Context, order Order) (Order, error) {
	// Set defaults.
	if order.ID == "" {
		order.ID = fmt.Sprintf("ORD-%d", time.Now().UnixNano())
	}
	if order.CreatedAt.IsZero() {
		order.CreatedAt = time.Now()
	}
	if order.ProcessingLog == nil {
		order.ProcessingLog = make([]string, 0)
	}
	if order.Timestamps == nil {
		order.Timestamps = make(map[string]time.Time)
	}

	// Process through pipeline.
	startTime := time.Now()
	result, err := OrderPipeline.Process(ctx, order)

	// Now ensure we have initialized maps on the actual result.
	if result.Timestamps == nil {
		result.Timestamps = make(map[string]time.Time)
	}
	if result.ProcessingLog == nil {
		result.ProcessingLog = make([]string, 0)
	}

	// Record completion time.
	result.Timestamps["completed"] = time.Now()
	processingTime := time.Since(startTime)
	result.ProcessingLog = append(result.ProcessingLog,
		fmt.Sprintf("total processing time: %v", processingTime))

	// Record metrics.
	MetricsCollector.Record(result)

	// Return pipz error directly - rich debugging context available immediately.
	return result, err
}
