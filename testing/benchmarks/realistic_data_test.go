package benchmarks

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/zoobzio/pipz"
)

// BenchmarkRealisticDataProcessing compares performance with realistic data structures
// vs simple integers to show real-world performance characteristics.
func BenchmarkRealisticDataProcessing(b *testing.B) {
	ctx := context.Background()

	// Create sample realistic data
	user := User{
		ID:    12345,
		Name:  "John Doe",
		Email: "john.doe@example.com",
		Age:   30,
		Metadata: map[string]interface{}{
			"department": "engineering",
			"level":      "senior",
			"projects":   []string{"pipz", "streamz"},
		},
	}

	order := Order{
		ID:         "ORD-2024-001",
		CustomerID: 12345,
		Items: []OrderItem{
			{ProductID: "PROD-001", Quantity: 2, Price: 29.99},
			{ProductID: "PROD-002", Quantity: 1, Price: 149.99},
		},
		Total:     209.97,
		Status:    "pending",
		Timestamp: time.Now(),
		Shipping: ShippingInfo{
			Address: "123 Main St",
			City:    "San Francisco",
			State:   "CA",
			ZipCode: "94105",
			Country: "US",
			Method:  "standard",
			Cost:    9.99,
		},
	}

	b.Run("User_Processing_Simple_Integer", func(b *testing.B) {
		// Simple integer processing for comparison
		pipeline := pipz.NewSequence("user-simple",
			pipz.Transform("validate", func(_ context.Context, n ClonableInt) ClonableInt {
				if n <= 0 {
					return 0
				}
				return n
			}),
			pipz.Transform("enrich", func(_ context.Context, n ClonableInt) ClonableInt {
				return n * 2
			}),
			pipz.Transform("format", func(_ context.Context, n ClonableInt) ClonableInt {
				return n + 100
			}),
		)

		data := ClonableInt(42)
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			result, err := pipeline.Process(ctx, data)
			if err != nil {
				b.Fatal(err)
			}
			_ = result
		}
	})

	b.Run("User_Processing_Realistic_Data", func(b *testing.B) {
		// Realistic user processing pipeline
		pipeline := pipz.NewSequence("user-realistic",
			pipz.Apply("validate", func(_ context.Context, u User) (User, error) {
				if u.ID <= 0 {
					return u, errors.New("invalid user ID")
				}
				if u.Email == "" {
					return u, errors.New("email required")
				}
				return u, nil
			}),
			pipz.Transform("enrich", func(_ context.Context, u User) User {
				// Simulate enrichment with metadata
				u.Metadata["processing_timestamp"] = time.Now().Unix()
				u.Metadata["enriched"] = true
				return u
			}),
			pipz.Transform("format", func(_ context.Context, u User) User {
				// Simulate formatting operations
				u.Name = strings.ToTitle(u.Name)
				u.Email = strings.ToLower(u.Email)
				return u
			}),
		)

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			result, err := pipeline.Process(ctx, user)
			if err != nil {
				b.Fatal(err)
			}
			_ = result
		}
	})

	b.Run("Order_Processing_Simple_Integer", func(b *testing.B) {
		// Simple integer processing for comparison
		pipeline := pipz.NewSequence("order-simple",
			pipz.Apply("validate", func(_ context.Context, n ClonableInt) (ClonableInt, error) {
				if n <= 0 {
					return 0, errors.New("invalid value")
				}
				return n, nil
			}),
			pipz.Transform("calculate", func(_ context.Context, n ClonableInt) ClonableInt {
				return n * 2
			}),
			pipz.Transform("finalize", func(_ context.Context, n ClonableInt) ClonableInt {
				return n + 100
			}),
		)

		data := ClonableInt(42)
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			result, err := pipeline.Process(ctx, data)
			if err != nil {
				b.Fatal(err)
			}
			_ = result
		}
	})

	b.Run("Order_Processing_Realistic_Data", func(b *testing.B) {
		// Realistic order processing pipeline
		pipeline := pipz.NewSequence("order-realistic",
			pipz.Apply("validate", func(_ context.Context, o Order) (Order, error) {
				if o.ID == "" {
					return o, errors.New("order ID required")
				}
				if len(o.Items) == 0 {
					return o, errors.New("order must have items")
				}
				if o.CustomerID <= 0 {
					return o, errors.New("valid customer ID required")
				}
				return o, nil
			}),
			pipz.Transform("calculate_total", func(_ context.Context, o Order) Order {
				// Recalculate total from items
				total := 0.0
				for _, item := range o.Items {
					total += item.Price * float64(item.Quantity)
				}
				// Add shipping cost
				total += o.Shipping.Cost
				o.Total = total
				return o
			}),
			pipz.Transform("update_status", func(_ context.Context, o Order) Order {
				if o.Total > 100.0 {
					o.Status = "approved"
				} else {
					o.Status = "pending_review"
				}
				return o
			}),
		)

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			result, err := pipeline.Process(ctx, order)
			if err != nil {
				b.Fatal(err)
			}
			_ = result
		}
	})

	b.Run("Concurrent_Processing_Simple_Integer", func(b *testing.B) {
		// Simple concurrent processing
		concurrent := pipz.NewConcurrent("concurrent-simple",
			pipz.Transform("proc1", func(_ context.Context, n ClonableInt) ClonableInt { return n * 2 }),
			pipz.Transform("proc2", func(_ context.Context, n ClonableInt) ClonableInt { return n + 10 }),
			pipz.Transform("proc3", func(_ context.Context, n ClonableInt) ClonableInt { return n - 5 }),
		)

		data := ClonableInt(42)
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			results, err := concurrent.Process(ctx, data)
			if err != nil {
				b.Fatal(err)
			}
			_ = results
		}
	})

	b.Run("Concurrent_Processing_Realistic_Data", func(b *testing.B) {
		// Realistic concurrent processing
		concurrent := pipz.NewConcurrent("concurrent-realistic",
			pipz.Transform("validate_address", func(_ context.Context, u User) User {
				// Simulate address validation
				if u.Metadata == nil {
					u.Metadata = make(map[string]interface{})
				}
				u.Metadata["address_validated"] = true
				return u
			}),
			pipz.Transform("check_permissions", func(_ context.Context, u User) User {
				// Simulate permission check
				if u.Metadata == nil {
					u.Metadata = make(map[string]interface{})
				}
				u.Metadata["permissions_checked"] = true
				return u
			}),
			pipz.Transform("audit_log", func(_ context.Context, u User) User {
				// Simulate audit logging
				if u.Metadata == nil {
					u.Metadata = make(map[string]interface{})
				}
				u.Metadata["audit_logged"] = time.Now().Unix()
				return u
			}),
		)

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			results, err := concurrent.Process(ctx, user)
			if err != nil {
				b.Fatal(err)
			}
			_ = results
		}
	})
}

// BenchmarkCloningOverhead measures the overhead of deep copying complex data structures.
func BenchmarkCloningOverhead(b *testing.B) {
	// Create sample data with varying complexity
	simpleUser := User{
		ID:    1,
		Name:  "John",
		Email: "john@example.com",
		Age:   30,
	}

	complexUser := User{
		ID:    1,
		Name:  "John Doe with a much longer name that requires more memory",
		Email: "john.doe.with.a.very.long.email.address@example-company.com",
		Age:   30,
		Metadata: map[string]interface{}{
			"department": "engineering",
			"level":      "senior",
			"projects":   []string{"pipz", "streamz", "project-alpha", "project-beta"},
			"certifications": map[string]string{
				"aws":        "solutions-architect",
				"kubernetes": "cka",
				"go":         "certified-developer",
			},
			"performance_history": []map[string]interface{}{
				{"year": 2023, "rating": 4.5, "goals_met": 8},
				{"year": 2022, "rating": 4.2, "goals_met": 7},
				{"year": 2021, "rating": 4.0, "goals_met": 6},
			},
		},
	}

	largeOrder := Order{
		ID:         "ORD-2024-LARGE-ORDER-WITH-MANY-ITEMS-001",
		CustomerID: 12345,
		Items:      make([]OrderItem, 50), // 50 items
		Total:      9999.99,
		Status:     "pending_approval_by_senior_management",
		Timestamp:  time.Now(),
		Shipping: ShippingInfo{
			Address: "123 Main Street, Apartment 4B, Building Complex Alpha",
			City:    "San Francisco",
			State:   "California",
			ZipCode: "94105",
			Country: "United States of America",
			Method:  "express_international_overnight",
			Cost:    299.99,
		},
	}

	// Fill the large order with items
	for i := range largeOrder.Items {
		largeOrder.Items[i] = OrderItem{
			ProductID: fmt.Sprintf("PROD-%03d-EXTENDED-PRODUCT-NAME", i),
			Quantity:  i%10 + 1,
			Price:     float64(i)*1.5 + 9.99,
		}
	}

	b.Run("Clone_Simple_Integer", func(b *testing.B) {
		data := ClonableInt(42)
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			cloned := data.Clone()
			_ = cloned
		}
	})

	b.Run("Clone_Simple_User", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			cloned := simpleUser.Clone()
			_ = cloned
		}
	})

	b.Run("Clone_Complex_User", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			cloned := complexUser.Clone()
			_ = cloned
		}
	})

	b.Run("Clone_Large_Order", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			cloned := largeOrder.Clone()
			_ = cloned
		}
	})
}

// BenchmarkErrorHandlingOverhead measures the overhead of rich error context with realistic data.
func BenchmarkErrorHandlingOverhead(b *testing.B) {
	ctx := context.Background()

	complexUser := User{
		ID:    1,
		Name:  "Test User",
		Email: "test@example.com",
		Age:   30,
		Metadata: map[string]interface{}{
			"large_data": strings.Repeat("x", 1000), // 1KB of data
		},
	}

	b.Run("Simple_Error_Simple_Data", func(b *testing.B) {
		pipeline := pipz.NewSequence("simple-error",
			pipz.Apply("fail", func(_ context.Context, _ ClonableInt) (ClonableInt, error) {
				return 0, errors.New("simple error")
			}),
		)

		data := ClonableInt(42)
		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			_, err := pipeline.Process(ctx, data)
			if err == nil {
				b.Fatal("expected error")
			}
			_ = err.Error() // Force error string generation
		}
	})

	b.Run("Rich_Error_Complex_Data", func(b *testing.B) {
		pipeline := pipz.NewSequence("rich-error",
			pipz.Apply("fail", func(_ context.Context, u User) (User, error) {
				return u, errors.New("complex error with user data")
			}),
		)

		b.ResetTimer()
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			_, err := pipeline.Process(ctx, complexUser)
			if err == nil {
				b.Fatal("expected error")
			}
			_ = err.Error() // Force error string generation
		}
	})
}
