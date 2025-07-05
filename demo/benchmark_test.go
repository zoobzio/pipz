package main

import (
	"testing"
	"pipz"
	"pipz/demo/processors"
)

// BenchmarkSecurityPipeline measures the performance of the security audit pipeline
func BenchmarkSecurityPipeline(b *testing.B) {
	// Register the pipeline once
	const key processors.SecurityKey = "bench-v1"
	contract := pipz.GetContract[processors.SecurityKey, processors.AuditableData](key)
	contract.Register(
		processors.Adapt(processors.CheckPermissions),
		processors.Adapt(processors.LogAccess),
		processors.Adapt(processors.RedactSensitive),
		processors.Adapt(processors.TrackCompliance),
	)
	
	// Test data
	data := processors.AuditableData{
		Data: &processors.User{
			Name:    "John Doe",
			Email:   "john@example.com",
			SSN:     "123-45-6789",
			IsAdmin: false,
		},
		UserID: "user-123",
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		contract.Process(data)
	}
}

// BenchmarkValidationPipeline measures validation performance
func BenchmarkValidationPipeline(b *testing.B) {
	type ValidationKey string
	contract := pipz.GetContract[ValidationKey, processors.Order](ValidationKey("bench"))
	contract.Register(
		processors.Adapt(processors.ValidateOrderID),
		processors.Adapt(processors.ValidateItems),
		processors.Adapt(processors.ValidateQuantities),
		processors.Adapt(processors.ValidateTotals),
		processors.Adapt(processors.ValidateShipping),
		processors.Adapt(processors.EnrichWithMetadata),
	)
	
	order := processors.Order{
		ID:         "ORD-12345",
		CustomerID: "CUST-67890",
		Items: []processors.OrderItem{
			{SKU: "LAPTOP-001", Quantity: 2, Price: 1299.99},
			{SKU: "MOUSE-002", Quantity: 2, Price: 49.99},
		},
		Total: 2699.96,
		ShippingAddress: processors.Address{
			Street:  "123 Main St",
			City:    "San Francisco",
			State:   "CA",
			ZipCode: "94105",
		},
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		contract.Process(order)
	}
}

// BenchmarkTransformPipeline measures data transformation performance
func BenchmarkTransformPipeline(b *testing.B) {
	type TransformKey string
	contract := pipz.GetContract[TransformKey, processors.TransformContext](TransformKey("bench"))
	contract.Register(
		processors.AdaptTransform(processors.ParseRecord),
		processors.AdaptTransform(processors.ValidateRecord),
		processors.AdaptTransform(processors.NormalizeRecord),
		processors.AdaptTransform(processors.ConvertToModel),
	)
	
	ctx := processors.TransformContext{
		SourceFormat: "csv",
		TargetFormat: "model",
		Record: map[string]string{
			"name":  "john doe",
			"email": "JOHN.DOE@EXAMPLE.COM",
			"phone": "415-555-0123",
		},
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		contract.Process(ctx)
	}
}

// BenchmarkPaymentPipeline measures payment processing performance
func BenchmarkPaymentPipeline(b *testing.B) {
	type PaymentKey string
	contract := pipz.GetContract[PaymentKey, processors.Payment](PaymentKey("bench"))
	contract.Register(
		processors.AdaptPayment(processors.ValidatePayment),
		processors.AdaptPayment(processors.CheckFraud),
		processors.AdaptPayment(processors.ChargeCard),
		processors.AdaptPayment(processors.RecordTransaction),
	)
	
	payment := processors.Payment{
		Amount:     99.99,
		Currency:   "USD",
		CardNumber: "4242424242424242",
		CardHolder: "John Doe",
		MerchantID: "merchant-123",
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		contract.Process(payment)
	}
}

// BenchmarkTypeUniverseAccess measures the overhead of type universe isolation
func BenchmarkTypeUniverseAccess(b *testing.B) {
	// Create 3 different type universes
	type TenantAKey string
	type TenantBKey string
	type TenantCKey string
	
	// Simple test data
	type Data struct {
		Value int
	}
	
	processor := func(d Data) ([]byte, error) {
		d.Value++
		return pipz.Encode(d)
	}
	
	// Register in each universe
	contractA := pipz.GetContract[TenantAKey, Data](TenantAKey("v1"))
	contractA.Register(processor)
	
	contractB := pipz.GetContract[TenantBKey, Data](TenantBKey("v1"))
	contractB.Register(processor)
	
	contractC := pipz.GetContract[TenantCKey, Data](TenantCKey("v1"))
	contractC.Register(processor)
	
	data := Data{Value: 0}
	
	b.Run("UniverseA", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			contractA.Process(data)
		}
	})
	
	b.Run("UniverseB", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			contractB.Process(data)
		}
	})
	
	b.Run("UniverseC", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			contractC.Process(data)
		}
	})
}