# pipz Use Cases

This document showcases real-world problems that pipz solves. Each use case starts with a problem statement and demonstrates why pipz is the right solution.

## Data Validation Pipeline

**Problem**: Complex business objects require multi-step validation with clear, actionable error messages. Traditional validation approaches often result in deeply nested if-statements or validation logic scattered throughout the codebase.

**Why pipz**: Compose reusable validators that each handle one concern, get detailed error locations, and maintain single responsibility principle.

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

// Create validation processors
validateOrderID := pipz.Validate("order_id", func(ctx context.Context, o Order) error {
    if o.ID == "" {
        return fmt.Errorf("order ID required")
    }
    if !strings.HasPrefix(o.ID, "ORD-") {
        return fmt.Errorf("order ID must start with ORD-")
    }
    return nil
})

validateItems := pipz.Validate("items", func(ctx context.Context, o Order) error {
    if len(o.Items) == 0 {
        return fmt.Errorf("order must have at least one item")
    }
    for i, item := range o.Items {
        if item.Quantity <= 0 {
            return fmt.Errorf("item %d: quantity must be positive", i)
        }
        if item.Price < 0 {
            return fmt.Errorf("item %d: price cannot be negative", i)
        }
    }
    return nil
})

validateTotal := pipz.Validate("total", func(ctx context.Context, o Order) error {
    calculated := 0.0
    for _, item := range o.Items {
        calculated += float64(item.Quantity) * item.Price
    }
    if math.Abs(calculated-o.Total) > 0.01 {
        return fmt.Errorf("total mismatch: expected %.2f, got %.2f", calculated, o.Total)
    }
    return nil
})

// Compose into validation pipeline
validationPipeline := pipz.Sequential(
    validateOrderID,
    validateItems,
    validateTotal,
)

// Process an order
ctx := context.Background()
order, err := validationPipeline.Process(ctx, incomingOrder)
if err != nil {
    return fmt.Errorf("order validation failed: %w", err)
}
```

## Security Audit Pipeline

**Problem**: Applications need to track data access, redact sensitive information based on user permissions, and maintain audit logs for compliance. This logic often gets tangled with business logic.

**Why pipz**: Chain security checks that must happen in order, cleanly separate security concerns from business logic, and ensure audit trails are consistent.

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

// Create security processors
checkPermissions := pipz.Apply("check_permissions", func(ctx context.Context, a AuditableData) (AuditableData, error) {
    if a.UserID == "" {
        return AuditableData{}, fmt.Errorf("unauthorized: no user ID")
    }
    a.Actions = append(a.Actions, "permissions verified")
    return a, nil
})

logAccess := pipz.Apply("log_access", func(ctx context.Context, a AuditableData) (AuditableData, error) {
    a.Actions = append(a.Actions, fmt.Sprintf("accessed by %s at %s", 
        a.UserID, a.Timestamp.Format(time.RFC3339)))
    
    // Log to your audit system
    auditLog.Record(a.UserID, "data_access", a.Data)
    return a, nil
})

redactSensitive := pipz.Apply("redact_sensitive", func(ctx context.Context, a AuditableData) (AuditableData, error) {
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
})

// Create audit pipeline
auditPipeline := pipz.Sequential(
    checkPermissions,
    logAccess,
    redactSensitive,
)

// Use in data access layer
func getUserData(userID string, requestorID string) (*User, error) {
    user := fetchUserFromDB(userID)
    
    auditData := AuditableData{
        Data:      user,
        UserID:    requestorID,
        Timestamp: time.Now(),
    }
    
    ctx := context.Background()
    result, err := auditPipeline.Process(ctx, auditData)
    if err != nil {
        return nil, err
    }
    
    return result.Data, nil
}
```

## Data Transformation Pipeline

**Problem**: Converting data between formats (CSV→Database) requires parsing, validation, normalization, and error handling. Each step can fail, and errors need to be handled gracefully.

**Why pipz**: Each transformation step is isolated and testable, errors stop processing immediately with context about where they occurred, and the pipeline can be reused for batch processing.

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
transformPipeline := pipz.Sequential(
    pipz.Apply("parse_csv", parseCSV),
    pipz.Apply("validate_email", validateEmail),
    pipz.Apply("normalize_phone", normalizePhone),
)

// Process CSV data
data := TransformContext{
    CSV: &CSVRecord{
        Fields: []string{"123", "John Doe", "john@example.com", "555-1234"},
    },
}

ctx := context.Background()
result, err := transformPipeline.Process(ctx, data)
if err != nil {
    return fmt.Errorf("transformation failed: %w", err)
}

if len(result.Errors) > 0 {
    log.Printf("Transformation warnings: %v", result.Errors)
}
```

## Multi-Stage Processing Workflow

**Problem**: Complex workflows like user onboarding have multiple dependent stages (validation → enrichment → persistence → notification). Each stage has its own requirements and error handling needs.

**Why pipz**: Compose processors into stages using Sequential, each stage remains focused, and stages can be conditionally executed or reordered based on business rules.

```go
type WorkflowData struct {
    User      User
    Metadata  map[string]interface{}
    Processed []string
}

// Create stage processors
validateUser := pipz.Validate("validate_user", func(ctx context.Context, w WorkflowData) error {
    if w.User.Email == "" {
        return fmt.Errorf("email required")
    }
    return nil
})

logValidation := pipz.Effect("log_validation", func(ctx context.Context, w WorkflowData) error {
    log.Printf("Validation passed for user %s", w.User.Email)
    return nil
})

enrichUser := pipz.Apply("enrich_user", func(ctx context.Context, w WorkflowData) (WorkflowData, error) {
    // Fetch additional data
    profile, err := fetchUserProfile(w.User.ID)
    if err != nil {
        return w, err
    }
    w.Metadata["profile"] = profile
    w.Processed = append(w.Processed, "enriched")
    return w, nil
})

persistUser := pipz.Apply("persist_user", func(ctx context.Context, w WorkflowData) (WorkflowData, error) {
    // Save to database
    if err := saveUser(w.User); err != nil {
        return w, fmt.Errorf("failed to save user: %w", err)
    }
    w.Processed = append(w.Processed, "persisted")
    return w, nil
})

notifyCreation := pipz.Effect("notify_creation", func(ctx context.Context, w WorkflowData) error {
    // Send notification
    notifyUserCreated(w.User)
    return nil
})

// Compose into a workflow
workflow := pipz.Sequential(
    // Validation stage
    validateUser,
    logValidation,
    
    // Enrichment stage
    enrichUser,
    
    // Persistence stage
    persistUser,
    notifyCreation,
)

// Process through the complete workflow
ctx := context.Background()
result, err := workflow.Process(ctx, WorkflowData{
    User:     newUser,
    Metadata: make(map[string]interface{}),
})
```

## Request Middleware Pipeline

**Problem**: HTTP requests need authentication, rate limiting, and logging applied in a specific order. Different endpoints may need different middleware combinations.

**Why pipz**: Middleware order is explicit and enforced, easy to add/remove/reorder middleware based on routes, and each middleware is independently testable.

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
middlewarePipeline := pipz.Sequential(
    pipz.Apply("authenticate", authenticate),
    pipz.Apply("rate_limit", checkRateLimit),
    pipz.Effect("log_request", logRequest),
)

// Use in HTTP handler
func handleRequest(w http.ResponseWriter, r *http.Request) {
    req := Request{
        Headers: r.Header,
        Body:    readBody(r),
    }
    
    ctx := r.Context()
    processed, err := middlewarePipeline.Process(ctx, req)
    if err != nil {
        http.Error(w, err.Error(), getErrorCode(err))
        return
    }
    
    // Handle the authenticated, rate-limited request
    handleBusinessLogic(w, processed)
}
```

## Payment Error Recovery Pipeline

**Problem**: Payment failures need to be categorized (network errors vs insufficient funds), customers need appropriate notifications, and some errors should trigger retry logic with alternative providers.

**Why pipz**: Error handling pipeline can attempt recovery strategies based on error type, customer communication is consistent, and retry logic is centralized.

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
errorPipeline := pipz.Sequential(
    pipz.Apply("categorize_error", categorizeError),
    pipz.Apply("notify_customer", notifyCustomer),
    pipz.Apply("attempt_recovery", attemptRecovery),
    pipz.Effect("audit_error", func(ctx context.Context, pe PaymentError) error {
        // Always audit payment errors
        auditPaymentError(pe)
        return nil
    }),
)

// Use in payment processing
func processPayment(p Payment) error {
    if err := chargeCard(p); err != nil {
        ctx := context.Background()
        result, _ := errorPipeline.Process(ctx, PaymentError{
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

**Problem**: Testing complex business processes often requires mocking external services while keeping business logic intact. Traditional approaches lead to complex test setups.

**Why pipz**: Swap out individual processors for test doubles, keep validation and business logic while mocking external calls, and use the same pipeline structure in tests and production.

```go
// Production processors
validateOrder := pipz.Validate("validate", validateOrderFunc)
calculateTax := pipz.Apply("tax", calculateTaxFunc)
chargePayment := pipz.Apply("charge", chargePaymentFunc)
sendEmail := pipz.Effect("email", sendConfirmationEmail)

// Test processors
mockChargePayment := pipz.Apply("mock_charge", func(ctx context.Context, o Order) (Order, error) {
    // Mock successful payment
    o.PaymentID = "MOCK-" + o.ID
    return o, nil
})

mockSendEmail := pipz.Effect("mock_email", func(ctx context.Context, o Order) error {
    // Log instead of sending email
    log.Printf("Would send email to %s", o.CustomerEmail)
    return nil
})

// Production pipeline
productionPipeline := pipz.Sequential(
    validateOrder,
    calculateTax,
    chargePayment,
    sendEmail,
)

// Test pipeline
testPipeline := pipz.Sequential(
    validateOrder,      // Real validation
    calculateTax,       // Real tax calculation
    mockChargePayment,  // Mock payment
    mockSendEmail,      // Mock email
)

// In tests
func TestOrderProcessing(t *testing.T) {
    order := Order{
        ID:     "TEST-001",
        Total:  100.00,
        Items:  []OrderItem{{ProductID: "PROD-1", Quantity: 2, Price: 50.00}},
    }
    
    ctx := context.Background()
    result, err := testPipeline.Process(ctx, order)
    if err != nil {
        t.Fatalf("Order processing failed: %v", err)
    }
    
    // Assertions...
}
```

## Additional Real-World Use Cases

### Stream Processing Pipeline

**Problem**: Large files or continuous data streams need processing but don't fit in memory. Progress tracking and partial failure handling are required.

**Why pipz**: Process chunks through the same validation/transformation pipeline, maintain consistent error handling across chunks, and easily add progress reporting.

```go
type StreamContext struct {
    Reader    io.Reader
    ChunkSize int
    Processed int64
    Errors    []error
}

streamPipeline := pipz.Sequential(
    pipz.Apply("read_chunk", readNextChunk),
    pipz.Validate("validate_format", validateChunkFormat),
    pipz.Apply("process_chunk", processChunkData),
    pipz.Effect("update_progress", updateProgressBar),
    pipz.Apply("write_output", writeProcessedChunk),
)
```

### Webhook Processing

**Problem**: Webhooks from different providers (Stripe, GitHub, Slack) need signature verification, replay attack prevention, and routing to appropriate handlers.

**Why pipz**: Provider-specific processors can be composed, signature verification must happen first (fail fast), and routing logic is separate from processing logic.

```go
type Webhook struct {
    Provider  string  // stripe, github, slack
    Signature string
    Payload   []byte
    Verified  bool
}

webhookPipeline := pipz.Sequential(
    pipz.Apply("verify_signature", verifySignature),
    pipz.Apply("parse_payload", parsePayload),
    pipz.Validate("check_timestamp", preventReplayAttack), 
    pipz.Effect("audit_log", logWebhook),
    pipz.Apply("route", routeToHandler),
)
```

### Content Moderation Pipeline

**Problem**: User-generated content needs multiple checks (spam, profanity, PII) with different actions based on severity. Manual review may be needed for edge cases.

**Why pipz**: Chain multiple detection algorithms, use Mutate to conditionally quarantine based on risk score, and maintain audit trail of all checks performed.

```go
type Content struct {
    Text      string
    UserID    string
    RiskScore float64
    Flags     []string
    Status    string // pending, approved, quarantined
}

moderationPipeline := pipz.Sequential(
    pipz.Apply("spam_check", detectSpam),
    pipz.Apply("profanity_check", checkProfanity),
    pipz.Apply("pii_scan", scanForPII),
    pipz.Apply("calculate_risk", calculateRiskScore),
    pipz.Apply("auto_action", autoQuarantine),
    pipz.Effect("notify", notifyModerators),
)
```

### API Version Transformation

**Problem**: Supporting multiple API versions requires transforming responses based on client version. New fields need to be hidden from old clients, deprecated fields need migration.

**Why pipz**: Version-specific transformations can be dynamically composed, transformation order matters (deprecation → hiding → compression), and easy to test version compatibility.

```go
type APIResponse struct {
    Version    string
    Data       interface{}
    Deprecated []string
}

// Version-specific processors
migrateV1 := pipz.Apply("migrate_v1", migrateV1Fields)
hideNewFields := pipz.Apply("hide_v2_fields", hideNewFieldsFunc)
markDeprecated := pipz.Apply("mark_deprecated", markDeprecatedFields)
formatResponse := pipz.Apply("format", formatResponseFunc)

// Build pipeline based on client version
var processors []pipz.Chainable[APIResponse]

if clientVersion < 2 {
    processors = append(processors, migrateV1, hideNewFields)
}

processors = append(processors, markDeprecated, formatResponse)

responsePipeline := pipz.Sequential(processors...)
```

### Dynamic Pipeline Configuration

**Problem**: Different environments (dev/staging/prod) need different processing steps. Debug logging should only happen in development, extra validation in staging.

**Why pipz**: Compose processors dynamically based on environment configuration, keeping core business logic unchanged.

```go
// Core business processors
validateOrder := pipz.Validate("validate_order", validateOrderFunc)
calculateTax := pipz.Apply("calculate_tax", calculateTaxFunc)
processPayment := pipz.Apply("process_payment", processPaymentFunc)

// Optional processors
debugInput := pipz.Effect("debug_input", logInput)
debugOutput := pipz.Effect("debug_output", logOutput)
fraudCheck := pipz.Validate("fraud_check", performFraudCheck)

// Build pipeline based on environment
var processors []pipz.Chainable[Order]

// Add debug logging in development
if config.Environment == "development" {
    processors = append(processors, debugInput)
}

// Core business logic
processors = append(processors, validateOrder, calculateTax)

// Add fraud check in staging
if config.Environment == "staging" {
    processors = append(processors, fraudCheck)
}

processors = append(processors, processPayment)

// Add output debugging in development
if config.Environment == "development" {
    processors = append(processors, debugOutput)
}

// Create the pipeline
pipeline := pipz.Sequential(processors...)
```

## Perfect For

pipz excels at:

1. **Multi-step data processing** where order matters
2. **Middleware chains** that need to be composable
3. **ETL pipelines** with validation and transformation
4. **Request/response processing** with multiple concerns
5. **Error handling flows** with recovery strategies
6. **Workflows** that need conditional steps
7. **Stream processing** with consistent handling
8. **Any process** that benefits from clear stages and error context

## Key Benefits

### 1. **Separation of Concerns**
Each processor handles one specific aspect, making code easier to understand, test, and maintain.

### 2. **Explicit Order**
Processing order is clear and enforced, no hidden dependencies.

### 3. **Error Context**
Errors include which processor failed and at what stage, making debugging easier.

### 4. **Testability**
Each processor can be tested in isolation, and processors can be swapped for tests.

### 5. **Reusability**
Processors and pipelines can be shared across different parts of your application.

### 6. **Dynamic Configuration**
Pipelines can be modified at runtime based on configuration or feature flags.