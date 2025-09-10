# Building Your First Pipeline

This guide walks through building a complete data processing pipeline from scratch.

## The Scenario

We'll build a user registration pipeline that:
1. Validates user input
2. Checks for existing accounts
3. Enriches data with defaults
4. Creates the account
5. Sends welcome emails
6. Logs for analytics

## Step 1: Define Your Data Model

```go
package main

import (
    "context"
    "errors"
    "fmt"
    "strings"
    "time"
    
    "github.com/zoobzio/pipz"
)

type User struct {
    ID           string
    Email        string
    Username     string
    Password     string // hashed
    FullName     string
    Country      string
    Verified     bool
    CreatedAt    time.Time
    Preferences  UserPreferences
}

type UserPreferences struct {
    Newsletter  bool
    Language    string
    Theme       string
}
```

## Step 2: Define Your Keys (Constants)

Define all processor and connector names as constants - these are the "keys" to your system:

```go
// All names are constants - this is the key to the system
const (
    // Validation processors
    ProcessorValidate       = "validate"
    ProcessorCheckDuplicate = "check_duplicate"
    
    // Transformation processors  
    ProcessorEnrich = "enrich"
    
    // Persistence processors
    ProcessorSave = "save"
    
    // Post-registration processors
    ProcessorSendWelcome  = "send_welcome"
    ProcessorLogAnalytics = "log_analytics"
    
    // Connector names
    PipelineRegistration      = "registration"
    ConnectorPostRegistration = "post-registration"
    ConnectorEmailHandle      = "email-with-error-handling"
    ConnectorSaveRetry        = "save-with-retry"
)
```

## Step 3: Define Your Business Logic

Write the core business logic as pure functions:

```go
// Validation logic
func validateUser(ctx context.Context, user User) error {
    if user.Email == "" {
        return errors.New("email is required")
    }
    if !strings.Contains(user.Email, "@") {
        return errors.New("invalid email format")
    }
    if len(user.Username) < 3 {
        return errors.New("username must be at least 3 characters")
    }
    if len(user.Password) < 8 {
        return errors.New("password must be at least 8 characters")
    }
    return nil
}

// Check for existing user
func checkDuplicate(ctx context.Context, user User) error {
    // Simulate database check
    existingEmails := map[string]bool{
        "admin@example.com": true,
        "test@example.com":  true,
    }
    
    if existingEmails[user.Email] {
        return fmt.Errorf("email %s already registered", user.Email)
    }
    return nil
}

// Normalize and enrich data
func enrichUser(ctx context.Context, user User) User {
    // Normalize email
    user.Email = strings.ToLower(strings.TrimSpace(user.Email))
    
    // Set defaults
    if user.Country == "" {
        user.Country = detectCountry(ctx)
    }
    
    if user.Preferences.Language == "" {
        user.Preferences.Language = "en"
    }
    
    if user.Preferences.Theme == "" {
        user.Preferences.Theme = "light"
    }
    
    // Set timestamps
    user.CreatedAt = time.Now()
    user.ID = generateID()
    
    return user
}

// Save to database
func saveUser(ctx context.Context, user User) error {
    // Simulate database save
    fmt.Printf("Saving user: %s\n", user.Email)
    
    // In real app:
    // return db.Save(ctx, user)
    
    return nil
}

// Send welcome email
func sendWelcomeEmail(ctx context.Context, user User) error {
    // Simulate email sending
    fmt.Printf("Sending welcome email to: %s\n", user.Email)
    
    // In real app:
    // return emailService.Send(ctx, WelcomeEmail{
    //     To: user.Email,
    //     Name: user.FullName,
    // })
    
    return nil
}

// Log for analytics
func logRegistration(ctx context.Context, user User) error {
    fmt.Printf("[ANALYTICS] New user registered: %s from %s\n", 
        user.Username, user.Country)
    return nil
}

// Error handlers
func logEmailError(ctx context.Context, err *pipz.Error[User]) error {
    fmt.Printf("Failed to send welcome email: %v\n", err.Err)
    return nil
}

// Helper functions
func detectCountry(ctx context.Context) string {
    // In real app: use GeoIP or similar
    return "US"
}

func generateID() string {
    return fmt.Sprintf("user_%d", time.Now().UnixNano())
}
```

## Step 4: Define Your Processors

Wrap your business logic with pipz processors:

```go
// Processors as reusable variables
var (
    // Validation processors
    ValidateUser    = pipz.Effect(ProcessorValidate, validateUser)
    CheckDuplicate  = pipz.Effect(ProcessorCheckDuplicate, checkDuplicate)
    
    // Transformation processors
    EnrichUser = pipz.Transform(ProcessorEnrich, enrichUser)
    
    // Persistence processors
    SaveUser = pipz.Effect(ProcessorSave, saveUser)
    
    // Post-registration processors
    SendWelcome  = pipz.Effect(ProcessorSendWelcome, sendWelcomeEmail)
    LogAnalytics = pipz.Effect(ProcessorLogAnalytics, logRegistration)
    
    // Error handling
    LogEmailError = pipz.Effect("log_email_error", logEmailError)
)

// Note: Since Concurrent requires Cloner, implement it:
func (u User) Clone() User {
    // User has no pointer fields, so simple copy works
    return u
}
```

## Step 5: Define Your Connectors

Compose processors into sequences and connectors:

```go
// Composed connectors
var (
    // Basic registration pipeline
    RegistrationPipeline = pipz.NewSequence[User](PipelineRegistration,
        // Validation phase
        ValidateUser,
        CheckDuplicate,
        
        // Transformation phase
        EnrichUser,
        
        // Persistence phase
        SaveUser,
        
        // Post-registration phase (parallel)
        pipz.NewConcurrent(ConnectorPostRegistration,
            SendWelcome,
            LogAnalytics,
        ),
    )
    
    // Robust email sending with error handling
    EmailWithErrorHandling = pipz.NewHandle(ConnectorEmailHandle,
        SendWelcome,
        LogEmailError,
    )
    
    // Save with retry logic
    SaveWithRetry = pipz.NewBackoff(ConnectorSaveRetry,
        SaveUser,
        3,
        100*time.Millisecond,
    )
    
    // Robust registration pipeline
    RobustRegistrationPipeline = pipz.NewSequence[User]("robust-registration",
        // Validation with early exit
        ValidateUser,
        CheckDuplicate,
        
        // Enrich data
        EnrichUser,
        
        // Save with retry
        SaveWithRetry,
        
        // Non-critical operations shouldn't fail registration
        pipz.NewConcurrent(ConnectorPostRegistration,
            EmailWithErrorHandling,
            LogAnalytics,
        ),
    )
)
```

## Step 6: Create Functions to Execute Pipelines

```go
// Simple registration
func RegisterUser(ctx context.Context, user User) (User, error) {
    return RegistrationPipeline.Process(ctx, user)
}

// Robust registration with error handling
func RegisterUserRobust(ctx context.Context, user User) (User, error) {
    return RobustRegistrationPipeline.Process(ctx, user)
}

func main() {
    // Test with valid user
    newUser := User{
        Email:    "john.doe@example.com",
        Username: "johndoe",
        Password: "securepassword123", // Would be hashed in real app
        FullName: "John Doe",
    }
    
    ctx := context.Background()
    registered, err := RegisterUser(ctx, newUser)
    if err != nil {
        var pipeErr *pipz.Error[User]
        if errors.As(err, &pipeErr) {
            fmt.Printf("Registration failed at %v: %v\n", pipeErr.Path, pipeErr.Err)
        } else {
            fmt.Printf("Registration failed: %v\n", err)
        }
        return
    }
    
    fmt.Printf("Successfully registered: %+v\n", registered)
}
```

## Step 7: Dynamic Pipeline Modification

Pipelines can be modified at runtime:

```go
// Add fraud detection for high-risk domains
func AddFraudDetection() {
    fraudCheck := pipz.Effect("fraud_check", func(ctx context.Context, user User) error {
        // Check against fraud database
        fmt.Printf("Checking user %s for fraud indicators\n", user.Email)
        return nil
    })
    
    // Insert after validation but before enrichment
    RegistrationPipeline.After(ProcessorCheckDuplicate, fraudCheck)
}

// Replace email sender for A/B testing
func UseNewEmailProvider() {
    newEmailSender := pipz.Effect(ProcessorSendWelcome, func(ctx context.Context, user User) error {
        fmt.Printf("[NEW PROVIDER] Sending welcome email to: %s\n", user.Email)
        return nil
    })
    
    RegistrationPipeline.Replace(ProcessorSendWelcome, newEmailSender)
}
```

## Step 8: Add Conditional Logic

Add premium user handling with the Switch connector:

```go
// Additional constants
const (
    ProcessorRegularOnboarding  = "regular_onboarding"
    ProcessorPremiumOnboarding  = "premium_onboarding"
    ProcessorAssignManager      = "assign_account_manager"
    RouterUserType              = "user-type-router"
    PipelinePremiumFlow         = "premium-flow"
    PipelineConditionalReg      = "conditional-registration"
)

// Route keys
type UserType string

const (
    TypeRegular UserType = "regular"
    TypePremium UserType = "premium"
)

// Business logic
func detectUserType(ctx context.Context, user User) UserType {
    // Premium domains get premium accounts
    premiumDomains := []string{"company.com", "enterprise.org"}
    
    emailDomain := strings.Split(user.Email, "@")[1]
    for _, domain := range premiumDomains {
        if emailDomain == domain {
            return TypePremium
        }
    }
    return TypeRegular
}

func regularOnboarding(ctx context.Context, u User) error {
    fmt.Println("Starting regular onboarding flow")
    return nil
}

func premiumOnboarding(ctx context.Context, u User) error {
    fmt.Println("Starting premium onboarding flow")
    return nil
}

func assignAccountManager(ctx context.Context, u User) error {
    fmt.Println("Assigning dedicated account manager")
    return nil
}

// Processors
var (
    RegularOnboarding  = pipz.Effect(ProcessorRegularOnboarding, regularOnboarding)
    PremiumOnboarding  = pipz.Effect(ProcessorPremiumOnboarding, premiumOnboarding)
    AssignManager      = pipz.Effect(ProcessorAssignManager, assignAccountManager)
)

// Connectors
var (
    // Premium user flow
    PremiumFlow = pipz.NewSequence[User](PipelinePremiumFlow,
        PremiumOnboarding,
        AssignManager,
    )
    
    // Router for user types
    UserTypeRouter = pipz.NewSwitch(RouterUserType, detectUserType).
        AddRoute(TypeRegular, RegularOnboarding).
        AddRoute(TypePremium, PremiumFlow)
    
    // Conditional registration pipeline
    ConditionalRegistrationPipeline = pipz.NewSequence[User](PipelineConditionalReg,
        // Common steps
        ValidateUser,
        CheckDuplicate,
        EnrichUser,
        SaveUser,
        
        // Route based on user type
        UserTypeRouter,
    )
)

// Function to execute
func RegisterUserConditional(ctx context.Context, user User) (User, error) {
    return ConditionalRegistrationPipeline.Process(ctx, user)
}
```

## Step 9: Test Your Pipeline

```go
func TestRegistrationPipeline(t *testing.T) {
    pipeline := createRegistrationPipeline()
    
    tests := []struct {
        name    string
        user    User
        wantErr bool
    }{
        {
            name: "valid user",
            user: User{
                Email:    "valid@example.com",
                Username: "validuser",
                Password: "password123",
            },
            wantErr: false,
        },
        {
            name: "invalid email",
            user: User{
                Email:    "invalid",
                Username: "validuser",
                Password: "password123",
            },
            wantErr: true,
        },
        {
            name: "duplicate email",
            user: User{
                Email:    "admin@example.com",
                Username: "newadmin",
                Password: "password123",
            },
            wantErr: true,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            _, err := pipeline.Process(context.Background(), tt.user)
            if (err != nil) != tt.wantErr {
                t.Errorf("Process() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

## Key Takeaways

1. **Start Simple**: Begin with basic processors and compose them
2. **Add Robustness Gradually**: Layer in retry, timeout, and error handling
3. **Use the Right Connector**: Sequence for steps, Concurrent for parallel work
4. **Handle Errors Appropriately**: Critical vs non-critical operations
5. **Test Each Component**: Processors are independently testable

## Next Steps

- [Error Recovery Patterns](../guides/safety-reliability.md) - Advanced error handling
- [Testing Pipelines](../guides/testing.md) - Comprehensive testing strategies
- [Performance Optimization](../guides/performance.md) - Making pipelines fast