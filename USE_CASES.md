# pipz Use Cases

This document showcases real-world use cases for pipz pipelines. Each example demonstrates how pipz can simplify common processing patterns.

## Data Validation Pipeline

Validate complex data structures with reusable validators:

```go
type Order struct {
    ID         string
    CustomerID string
    Items      []OrderItem
    Total      float64
}

type OrderItem struct {
    ProductID string
    Quantity  int
    Price     float64
}

// Validators
func validateOrderID(o Order) (Order, error) {
    if o.ID == "" {
        return Order{}, fmt.Errorf("order ID required")
    }
    if !strings.HasPrefix(o.ID, "ORD-") {
        return Order{}, fmt.Errorf("order ID must start with ORD-")
    }
    return o, nil
}

func validateItems(o Order) (Order, error) {
    if len(o.Items) == 0 {
        return Order{}, fmt.Errorf("order must have at least one item")
    }
    for i, item := range o.Items {
        if item.Quantity <= 0 {
            return Order{}, fmt.Errorf("item %d: quantity must be positive", i)
        }
        if item.Price < 0 {
            return Order{}, fmt.Errorf("item %d: price cannot be negative", i)
        }
    }
    return o, nil
}

func validateTotal(o Order) (Order, error) {
    calculated := 0.0
    for _, item := range o.Items {
        calculated += float64(item.Quantity) * item.Price
    }
    if math.Abs(calculated-o.Total) > 0.01 {
        return Order{}, fmt.Errorf("total mismatch: expected %.2f, got %.2f", calculated, o.Total)
    }
    return o, nil
}

// Usage
validationPipeline := pipz.NewContract[Order]()
validationPipeline.Register(
    pipz.Apply(validateOrderID),
    pipz.Apply(validateItems),
    pipz.Apply(validateTotal),
)

// Process an order
order, err := validationPipeline.Process(incomingOrder)
if err != nil {
    return fmt.Errorf("order validation failed: %w", err)
}
```

## Security Audit Pipeline

Track and audit data access with composable audit processors:

```go
type AuditableData struct {
    Data      *User
    UserID    string
    Timestamp time.Time
    Actions   []string
}

type User struct {
    Name      string
    Email     string
    SSN       string
    IsAdmin   bool
}

func checkPermissions(a AuditableData) (AuditableData, error) {
    if a.UserID == "" {
        return AuditableData{}, fmt.Errorf("unauthorized: no user ID")
    }
    a.Actions = append(a.Actions, "permissions verified")
    return a, nil
}

func logAccess(a AuditableData) (AuditableData, error) {
    a.Actions = append(a.Actions, fmt.Sprintf("accessed by %s at %s", 
        a.UserID, a.Timestamp.Format(time.RFC3339)))
    
    // Log to your audit system
    auditLog.Record(a.UserID, "data_access", a.Data)
    return a, nil
}

func redactSensitive(a AuditableData) (AuditableData, error) {
    if a.Data != nil && !a.Data.IsAdmin {
        // Redact SSN for non-admins
        if len(a.Data.SSN) > 4 {
            a.Data.SSN = "***-**-" + a.Data.SSN[len(a.Data.SSN)-4:]
        }
        // Mask email
        if idx := strings.Index(a.Data.Email, "@"); idx > 2 {
            a.Data.Email = a.Data.Email[:2] + "****" + a.Data.Email[idx:]
        }
    }
    a.Actions = append(a.Actions, "sensitive data redacted")
    return a, nil
}

// Create audit pipeline
auditPipeline := pipz.NewContract[AuditableData]()
auditPipeline.Register(
    pipz.Apply(checkPermissions),
    pipz.Apply(logAccess),
    pipz.Apply(redactSensitive),
)

// Use in data access layer
func getUserData(userID string, requestorID string) (*User, error) {
    user := fetchUserFromDB(userID)
    
    auditData := AuditableData{
        Data:      user,
        UserID:    requestorID,
        Timestamp: time.Now(),
    }
    
    result, err := auditPipeline.Process(auditData)
    if err != nil {
        return nil, err
    }
    
    return result.Data, nil
}
```

## Data Transformation Pipeline

Transform data between formats with validation and enrichment:

```go
type CSVRecord struct {
    Fields []string
}

type DatabaseRecord struct {
    ID        int
    Name      string
    Email     string
    Phone     string
    CreatedAt time.Time
}

type TransformContext struct {
    CSV    *CSVRecord
    DB     *DatabaseRecord
    Errors []string
}

func parseCSV(ctx TransformContext) (TransformContext, error) {
    if ctx.CSV == nil || len(ctx.CSV.Fields) < 4 {
        return TransformContext{}, fmt.Errorf("insufficient CSV fields")
    }
    
    id, _ := strconv.Atoi(ctx.CSV.Fields[0])
    
    ctx.DB = &DatabaseRecord{
        ID:        id,
        Name:      ctx.CSV.Fields[1],
        Email:     ctx.CSV.Fields[2],
        Phone:     ctx.CSV.Fields[3],
        CreatedAt: time.Now(),
    }
    return ctx, nil
}

func validateEmail(ctx TransformContext) (TransformContext, error) {
    if ctx.DB == nil {
        return TransformContext{}, fmt.Errorf("no database record to validate")
    }
    
    if !strings.Contains(ctx.DB.Email, "@") {
        ctx.Errors = append(ctx.Errors, "invalid email format")
        ctx.DB.Email = ""
    } else {
        ctx.DB.Email = strings.ToLower(ctx.DB.Email)
    }
    return ctx, nil
}

func normalizePhone(ctx TransformContext) (TransformContext, error) {
    if ctx.DB == nil || ctx.DB.Phone == "" {
        return ctx, nil
    }
    
    // Remove all non-digits
    cleaned := strings.Map(func(r rune) rune {
        if r >= '0' && r <= '9' {
            return r
        }
        return -1
    }, ctx.DB.Phone)
    
    if len(cleaned) == 10 {
        ctx.DB.Phone = fmt.Sprintf("+1-%s-%s-%s", 
            cleaned[:3], cleaned[3:6], cleaned[6:])
    } else {
        ctx.Errors = append(ctx.Errors, "invalid phone number")
    }
    
    return ctx, nil
}

// Create transformation pipeline
transformPipeline := pipz.NewContract[TransformContext]()
transformPipeline.Register(
    pipz.Apply(parseCSV),
    pipz.Apply(validateEmail),
    pipz.Apply(normalizePhone),
)

// Process CSV data
ctx := TransformContext{
    CSV: &CSVRecord{
        Fields: []string{"123", "John Doe", "john@example.com", "555-1234"},
    },
}

result, err := transformPipeline.Process(ctx)
if err != nil {
    return fmt.Errorf("transformation failed: %w", err)
}

if len(result.Errors) > 0 {
    log.Printf("Transformation warnings: %v", result.Errors)
}
```

## Multi-Stage Processing Workflow

Combine multiple pipelines for complex workflows:

```go
type WorkflowData struct {
    User      User
    Metadata  map[string]interface{}
    Processed []string
}

// Create separate pipelines for each stage
func createValidationPipeline() *pipz.Contract[WorkflowData] {
    pipeline := pipz.NewContract[WorkflowData]()
    pipeline.Register(
        pipz.Validate(func(w WorkflowData) error {
            if w.User.Email == "" {
                return fmt.Errorf("email required")
            }
            return nil
        }),
        pipz.Effect(func(w WorkflowData) error {
            log.Printf("Validation passed for user %s", w.User.Email)
            return nil
        }),
    )
    return pipeline
}

func createEnrichmentPipeline() *pipz.Contract[WorkflowData] {
    pipeline := pipz.NewContract[WorkflowData]()
    pipeline.Register(
        pipz.Enrich(func(w WorkflowData) (WorkflowData, error) {
            // Fetch additional data
            profile, err := fetchUserProfile(w.User.ID)
            if err != nil {
                return w, err
            }
            w.Metadata["profile"] = profile
            w.Processed = append(w.Processed, "enriched")
            return w, nil
        }),
    )
    return pipeline
}

func createPersistencePipeline() *pipz.Contract[WorkflowData] {
    pipeline := pipz.NewContract[WorkflowData]()
    pipeline.Register(
        pipz.Apply(func(w WorkflowData) (WorkflowData, error) {
            // Save to database
            if err := saveUser(w.User); err != nil {
                return w, fmt.Errorf("failed to save user: %w", err)
            }
            w.Processed = append(w.Processed, "persisted")
            return w, nil
        }),
        pipz.Effect(func(w WorkflowData) error {
            // Send notification
            notifyUserCreated(w.User)
            return nil
        }),
    )
    return pipeline
}

// Compose into a workflow
workflow := pipz.NewChain[WorkflowData]()
workflow.Add(
    createValidationPipeline().Link(),
    createEnrichmentPipeline().Link(),
    createPersistencePipeline().Link(),
)

// Process through the complete workflow
result, err := workflow.Process(WorkflowData{
    User:     newUser,
    Metadata: make(map[string]interface{}),
})
```

## Request Middleware Pipeline

Clean request processing with reusable middleware:

```go
type Request struct {
    Headers    map[string]string
    Body       []byte
    UserID     string
    Authorized bool
    RateLimit  *RateLimit
}

func authenticate(r Request) (Request, error) {
    token := r.Headers["Authorization"]
    if token == "" {
        return Request{}, fmt.Errorf("missing auth token")
    }
    
    userID, err := validateToken(token)
    if err != nil {
        return Request{}, fmt.Errorf("invalid token: %w", err)
    }
    
    r.UserID = userID
    r.Authorized = true
    return r, nil
}

func checkRateLimit(r Request) (Request, error) {
    if !r.Authorized {
        return Request{}, fmt.Errorf("must be authorized")
    }
    
    limit := getRateLimit(r.UserID)
    if limit.Exceeded() {
        return Request{}, fmt.Errorf("rate limit exceeded")
    }
    
    limit.Increment()
    r.RateLimit = limit
    return r, nil
}

func logRequest(r Request) error {
    log.Printf("[%s] Request from user %s", 
        time.Now().Format(time.RFC3339), r.UserID)
    return nil
}

// Create middleware pipeline
middlewarePipeline := pipz.NewContract[Request]()
middlewarePipeline.Register(
    pipz.Apply(authenticate),
    pipz.Apply(checkRateLimit),
    pipz.Effect(logRequest),
)

// Use in HTTP handler
func handleRequest(w http.ResponseWriter, r *http.Request) {
    req := Request{
        Headers: r.Header,
        Body:    readBody(r),
    }
    
    processed, err := middlewarePipeline.Process(req)
    if err != nil {
        http.Error(w, err.Error(), getErrorCode(err))
        return
    }
    
    // Handle the authenticated, rate-limited request
    handleBusinessLogic(w, processed)
}
```

## Error Handling Pipeline

Centralized error handling with recovery strategies:

```go
type PaymentError struct {
    Payment       Payment
    OriginalError error
    ErrorType     string
    Recoverable   bool
    RecoveryAction string
}

func categorizeError(pe PaymentError) (PaymentError, error) {
    errMsg := pe.OriginalError.Error()
    
    switch {
    case strings.Contains(errMsg, "insufficient funds"):
        pe.ErrorType = "insufficient_funds"
        pe.Recoverable = false
        
    case strings.Contains(errMsg, "network"):
        pe.ErrorType = "network_error"
        pe.Recoverable = true
        pe.RecoveryAction = "retry_with_backoff"
        
    case strings.Contains(errMsg, "invalid card"):
        pe.ErrorType = "invalid_payment_method"
        pe.Recoverable = false
        
    default:
        pe.ErrorType = "unknown"
        pe.Recoverable = true
        pe.RecoveryAction = "manual_review"
    }
    
    return pe, nil
}

func notifyCustomer(pe PaymentError) (PaymentError, error) {
    switch pe.ErrorType {
    case "insufficient_funds":
        sendEmail(pe.Payment.CustomerEmail, 
            "Payment Declined - Insufficient Funds",
            insufficientFundsTemplate(pe.Payment))
            
    case "invalid_payment_method":
        sendEmail(pe.Payment.CustomerEmail,
            "Payment Method Invalid",
            updatePaymentMethodTemplate(pe.Payment))
    }
    
    return pe, nil
}

func attemptRecovery(pe PaymentError) (PaymentError, error) {
    if !pe.Recoverable {
        return pe, nil
    }
    
    switch pe.RecoveryAction {
    case "retry_with_backoff":
        // Try alternative payment provider
        if err := retryWithBackupProvider(pe.Payment); err == nil {
            pe.OriginalError = nil // Recovered!
            return pe, nil
        }
        
    case "manual_review":
        // Queue for manual processing
        queueForReview(pe.Payment)
    }
    
    return pe, nil
}

// Create error handling pipeline
errorPipeline := pipz.NewContract[PaymentError]()
errorPipeline.Register(
    pipz.Apply(categorizeError),
    pipz.Apply(notifyCustomer),
    pipz.Apply(attemptRecovery),
    pipz.Effect(func(pe PaymentError) error {
        // Always audit payment errors
        auditPaymentError(pe)
        return nil
    }),
)

// Use in payment processing
func processPayment(p Payment) error {
    if err := chargeCard(p); err != nil {
        result, _ := errorPipeline.Process(PaymentError{
            Payment:       p,
            OriginalError: err,
        })
        
        if result.OriginalError == nil {
            // Error was recovered!
            return nil
        }
        
        return result.OriginalError
    }
    return nil
}
```

## Testing with Pipelines

Use pipelines to create testable, mockable processing:

```go
// Production pipeline
func createProductionPipeline() *pipz.Contract[Order] {
    pipeline := pipz.NewContract[Order]()
    pipeline.Register(
        pipz.Apply(validateOrder),
        pipz.Apply(calculateTax),
        pipz.Apply(chargePayment),
        pipz.Effect(sendConfirmationEmail),
    )
    return pipeline
}

// Test pipeline with mocked external calls
func createTestPipeline() *pipz.Contract[Order] {
    pipeline := pipz.NewContract[Order]()
    pipeline.Register(
        pipz.Apply(validateOrder),      // Real validation
        pipz.Apply(calculateTax),       // Real tax calculation
        pipz.Apply(mockChargePayment),  // Mock payment
        pipz.Effect(func(o Order) error {
            // Log instead of sending email
            log.Printf("Would send email to %s", o.CustomerEmail)
            return nil
        }),
    )
    return pipeline
}

// In tests
func TestOrderProcessing(t *testing.T) {
    pipeline := createTestPipeline()
    
    order := Order{
        ID:     "TEST-001",
        Total:  100.00,
        Items:  []OrderItem{{ProductID: "PROD-1", Quantity: 2, Price: 50.00}},
    }
    
    result, err := pipeline.Process(order)
    if err != nil {
        t.Fatalf("Order processing failed: %v", err)
    }
    
    // Assertions...
}
```

## Additional Use Case Concepts

*Note: These are theoretical examples showing how pipz could be applied to various domains. They are not included in the demo system.*

### Event Processing Pipeline
```go
type Event struct {
    ID        string
    Type      string
    UserID    string
    Payload   map[string]interface{}
    Timestamp time.Time
}

// Process analytics events through multiple stages
eventPipeline := pipz.NewContract[Event]()
eventPipeline.Register(
    pipz.Validate(validateEventSchema),
    pipz.Transform(enrichWithUserData),
    pipz.Mutate(addGeolocation, isLocationEvent),
    pipz.Effect(sendToAnalytics),
    pipz.Apply(triggerNotifications),
)
```

### Webhook Processing
```go
type Webhook struct {
    Provider  string  // stripe, github, slack
    Signature string
    Payload   []byte
    Verified  bool
}

// Secure webhook processing with verification
webhookPipeline := pipz.NewContract[Webhook]()
webhookPipeline.Register(
    pipz.Apply(verifySignature),
    pipz.Apply(parsePayload),
    pipz.Apply(validateTimestamp), // Prevent replay attacks
    pipz.Effect(logWebhook),
    pipz.Apply(routeToHandler),
)
```

### Image Processing
```go
type ImageJob struct {
    Original  []byte
    Format    string
    Metadata  map[string]interface{}
    Processed map[string][]byte
}

// Multi-stage image transformation
imagePipeline := pipz.NewContract[ImageJob]()
imagePipeline.Register(
    pipz.Validate(validateImageFormat),
    pipz.Apply(extractMetadata),
    pipz.Transform(stripEXIF),        // Privacy
    pipz.Apply(generateThumbnails),
    pipz.Mutate(addWatermark, isPremiumUser),
    pipz.Apply(optimizeSize),
)
```

### Message Queue Worker
```go
type QueueMessage struct {
    ID       string
    Body     interface{}
    Attempts int
    Status   string
}

// Reliable message processing
mqPipeline := pipz.NewContract[QueueMessage]()
mqPipeline.Register(
    pipz.Apply(deserializeMessage),
    pipz.Validate(checkRetryLimit),
    pipz.Apply(processMessage),
    pipz.Effect(acknowledgeMessage),
    pipz.Apply(scheduleRetryIfNeeded),
)
```

### Feature Flag Evaluation
```go
type FeatureContext struct {
    UserID  string
    Feature string
    Enabled bool
    Variant string
}

// Complex feature flag logic
featurePipeline := pipz.NewContract[FeatureContext]()
featurePipeline.Register(
    pipz.Apply(checkUserSegment),
    pipz.Apply(evaluateRules),
    pipz.Mutate(applyOverrides, isInternalUser),
    pipz.Effect(trackFeatureUsage),
    pipz.Transform(selectVariant),
)
```

### Content Moderation
```go
type Content struct {
    Text   string
    Author string
    Flags  []string
    Score  float64
}

// Multi-stage content analysis
moderationPipeline := pipz.NewContract[Content]()
moderationPipeline.Register(
    pipz.Apply(detectSpam),
    pipz.Apply(checkProfanity),
    pipz.Apply(scanForPII),
    pipz.Mutate(autoQuarantine, hasHighRiskFlags),
    pipz.Effect(notifyModerators),
)
```

### API Response Transformation
```go
type APIResponse struct {
    StatusCode int
    Body       interface{}
    Version    string
}

// Version compatibility and transformation
responsePipeline := pipz.NewContract[APIResponse]()
responsePipeline.Register(
    pipz.Transform(versionTransform),    // v1 -> v2
    pipz.Apply(filterSensitiveData),
    pipz.Mutate(addDebugInfo, isDevelopment),
    pipz.Transform(compressResponse),
)
```

These examples demonstrate pipz's versatility across different domains while maintaining the same simple, composable pattern.

## Key Patterns

### 1. Separation of Concerns
Each processor handles one specific aspect of processing, making code easier to understand, test, and maintain.

### 2. Composability
Small, focused pipelines can be combined into larger workflows using chains.

### 3. Error Handling
Errors stop processing immediately, allowing for clean error handling at the pipeline level.

### 4. Testability
Pipelines make it easy to mock external dependencies while keeping business logic intact.

### 5. Reusability
Once created, pipelines can be used anywhere in your codebase without duplication.