# Recipe: Data Validation Pipeline

Build comprehensive data validation with clear error messages and progressive enhancement.

## Problem

You need to validate complex data structures with:
- Multiple validation rules
- Clear error messages
- Optional enrichment
- Business rule enforcement
- Audit trails

## Solution

Create a layered validation pipeline:

```go
// Example: User registration validation
type User struct {
    Email     string
    Password  string
    Age       int
    Country   string
    Referrer  string
    Score     float64
}

func createUserValidationPipeline() pipz.Chainable[User] {
    return pipz.NewSequence[User]("user-validation",
        // 1. Required field validation
        pipz.Apply("required-fields", func(ctx context.Context, u User) (User, error) {
            if u.Email == "" {
                return u, errors.New("email is required")
            }
            if u.Password == "" {
                return u, errors.New("password is required")
            }
            if u.Age == 0 {
                return u, errors.New("age is required")
            }
            return u, nil
        }),
        
        // 2. Format validation
        pipz.Apply("format-validation", func(ctx context.Context, u User) (User, error) {
            if !strings.Contains(u.Email, "@") {
                return u, fmt.Errorf("invalid email format: %s", u.Email)
            }
            if len(u.Password) < 8 {
                return u, errors.New("password must be at least 8 characters")
            }
            if u.Age < 13 || u.Age > 120 {
                return u, fmt.Errorf("age must be between 13 and 120, got %d", u.Age)
            }
            return u, nil
        }),
        
        // 3. Business rules
        pipz.Apply("business-rules", func(ctx context.Context, u User) (User, error) {
            // Age restrictions by country
            minAge := getMinAgeForCountry(u.Country)
            if u.Age < minAge {
                return u, fmt.Errorf("minimum age for %s is %d", u.Country, minAge)
            }
            
            // Referral validation
            if u.Referrer != "" && !isValidReferrer(u.Referrer) {
                return u, fmt.Errorf("invalid referrer code: %s", u.Referrer)
            }
            
            return u, nil
        }),
        
        // 4. Uniqueness check (external service)
        pipz.Apply("uniqueness", func(ctx context.Context, u User) (User, error) {
            exists, err := database.UserExists(ctx, u.Email)
            if err != nil {
                return u, fmt.Errorf("failed to check uniqueness: %w", err)
            }
            if exists {
                return u, fmt.Errorf("email already registered: %s", u.Email)
            }
            return u, nil
        }),
        
        // 5. Optional enrichment (doesn't fail pipeline)
        pipz.Enrich("risk-score", func(ctx context.Context, u User) (User, error) {
            score, err := riskEngine.Calculate(ctx, u)
            if err != nil {
                // Log but don't fail
                log.Printf("Failed to calculate risk score: %v", err)
                u.Score = 0.5 // Default medium risk
                return u, err
            }
            u.Score = score
            return u, nil
        }),
        
        // 6. Data normalization
        pipz.Transform("normalize", func(ctx context.Context, u User) User {
            u.Email = strings.ToLower(strings.TrimSpace(u.Email))
            u.Country = strings.ToUpper(u.Country)
            return u
        }),
        
        // 7. Audit logging (side effect)
        pipz.Effect("audit", func(ctx context.Context, u User) error {
            return auditLog.Record(ctx, "user.validated", map[string]interface{}{
                "email":    u.Email,
                "country":  u.Country,
                "age":      u.Age,
                "score":    u.Score,
            })
        }),
    )
}
```

## Validation Strategies

### Fast-Fail vs Complete Validation

```go
// Fast-fail: Stop at first error (default)
fastFail := pipz.NewSequence[Data]("fast-fail",
    validateStep1, // Stops here if fails
    validateStep2,
    validateStep3,
)

// Complete validation: Collect all errors
type ValidationResult struct {
    Data   Data
    Errors []error
}

completeValidation := pipz.Apply("complete", func(ctx context.Context, d Data) (ValidationResult, error) {
    result := ValidationResult{Data: d}
    
    // Run all validations
    if err := validateField1(d); err != nil {
        result.Errors = append(result.Errors, err)
    }
    if err := validateField2(d); err != nil {
        result.Errors = append(result.Errors, err)
    }
    if err := validateField3(d); err != nil {
        result.Errors = append(result.Errors, err)
    }
    
    if len(result.Errors) > 0 {
        return result, fmt.Errorf("%d validation errors", len(result.Errors))
    }
    return result, nil
})
```

### Conditional Validation

```go
conditionalValidation := pipz.NewSequence[Order]("order-validation",
    // Always validate
    pipz.Apply("base-validation", validateOrderBase),
    
    // Only validate premium features for premium customers
    pipz.Filter("premium-only",
        func(ctx context.Context, o Order) bool {
            return o.Customer.Tier == "premium"
        },
        pipz.Apply("premium-validation", validatePremiumFeatures),
    ),
    
    // Different validation for different payment methods
    pipz.Switch[Order]("payment-validation",
        func(ctx context.Context, o Order) string {
            return o.PaymentMethod
        },
    ).
    AddRoute("credit_card", validateCreditCard).
    AddRoute("paypal", validatePayPal).
    AddRoute("crypto", validateCrypto),
)
```

## Complex Validation Patterns

### Nested Object Validation

```go
// Validate nested structures
type Order struct {
    ID       string
    Customer Customer
    Items    []Item
    Payment  Payment
}

orderValidation := pipz.NewSequence[Order]("order",
    // Validate order itself
    pipz.Apply("order-base", validateOrderFields),
    
    // Validate nested customer
    pipz.Apply("customer", func(ctx context.Context, o Order) (Order, error) {
        validatedCustomer, err := customerValidator.Process(ctx, o.Customer)
        if err != nil {
            return o, fmt.Errorf("invalid customer: %w", err)
        }
        o.Customer = validatedCustomer
        return o, nil
    }),
    
    // Validate each item
    pipz.Apply("items", func(ctx context.Context, o Order) (Order, error) {
        for i, item := range o.Items {
            validatedItem, err := itemValidator.Process(ctx, item)
            if err != nil {
                return o, fmt.Errorf("invalid item at index %d: %w", i, err)
            }
            o.Items[i] = validatedItem
        }
        return o, nil
    }),
)
```

### Cross-Field Validation

```go
crossFieldValidation := pipz.Apply("cross-field", func(ctx context.Context, form Form) (Form, error) {
    // Date range validation
    if form.StartDate.After(form.EndDate) {
        return form, errors.New("start date must be before end date")
    }
    
    // Conditional requirements
    if form.ShippingMethod == "express" && form.Address.Country != "US" {
        return form, errors.New("express shipping only available in US")
    }
    
    // Sum validation
    var total float64
    for _, item := range form.Items {
        total += item.Price * float64(item.Quantity)
    }
    if math.Abs(total-form.Total) > 0.01 {
        return form, fmt.Errorf("total mismatch: calculated %.2f, provided %.2f", total, form.Total)
    }
    
    return form, nil
})
```

## Testing Validation

```go
func TestUserValidation(t *testing.T) {
    validator := createUserValidationPipeline()
    
    tests := []struct {
        name    string
        user    User
        wantErr string
    }{
        {
            name:    "missing email",
            user:    User{Password: "pass123", Age: 25},
            wantErr: "email is required",
        },
        {
            name:    "invalid email format",
            user:    User{Email: "notanemail", Password: "pass123", Age: 25},
            wantErr: "invalid email format",
        },
        {
            name:    "password too short",
            user:    User{Email: "test@example.com", Password: "short", Age: 25},
            wantErr: "at least 8 characters",
        },
        {
            name: "valid user",
            user: User{
                Email:    "test@example.com",
                Password: "securepass123",
                Age:      25,
                Country:  "US",
            },
            wantErr: "",
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            _, err := validator.Process(context.Background(), tt.user)
            if tt.wantErr == "" {
                assert.NoError(t, err)
            } else {
                assert.Contains(t, err.Error(), tt.wantErr)
            }
        })
    }
}
```

## Error Reporting

```go
// User-friendly error messages
type ValidationError struct {
    Field   string `json:"field"`
    Message string `json:"message"`
    Code    string `json:"code"`
}

func createAPIValidation() pipz.Chainable[Request] {
    return pipz.NewHandle("api-validation",
        validationPipeline,
        pipz.Apply("format-errors", func(ctx context.Context, err *pipz.Error[Request]) (*pipz.Error[Request], error) {
            // Transform technical errors to user-friendly format
            userError := ValidationError{
                Field:   extractField(err.Stage),
                Message: userFriendlyMessage(err.Cause),
                Code:    errorCode(err.Cause),
            }
            
            // Return as JSON response
            response.JSON(400, userError)
            return err, nil
        }),
    )
}
```

## Performance Optimization

```go
// Parallel validation for independent checks
parallelValidation := pipz.NewSequence[User]("optimized",
    // Quick local validations first
    pipz.Apply("quick-checks", quickLocalValidation),
    
    // Expensive external validations in parallel
    pipz.NewConcurrent[User](
        pipz.Enrich("geo-validation", validateGeoLocation),
        pipz.Enrich("risk-assessment", assessRisk),
        pipz.Enrich("credit-check", checkCredit),
    ),
    
    // Final validation based on enriched data
    pipz.Apply("final-check", finalValidation),
)
```

## See Also

- [Apply Reference](../reference/processors/apply.md)
- [Enrich Reference](../reference/processors/enrich.md)
- [Filter Reference](../reference/connectors/filter.md)
- [Testing Guide](../guides/testing.md)