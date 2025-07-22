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

## Step 2: Create Basic Processors

Start with individual processors for each step:

```go
// Validation
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
        user.Country = detectCountry(ctx) // Simplified
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

// Helper functions
func detectCountry(ctx context.Context) string {
    // In real app: use GeoIP or similar
    return "US"
}

func generateID() string {
    return fmt.Sprintf("user_%d", time.Now().UnixNano())
}
```

## Step 3: Create Side Effect Processors

```go
// Save to database
func saveUser(ctx context.Context, user User) error {
    // Simulate database save
    fmt.Printf("Saving user: %s\n", user.Email)
    
    // In real app:
    // db.Save(ctx, user)
    
    return nil
}

// Send welcome email
func sendWelcomeEmail(ctx context.Context, user User) error {
    // Simulate email sending
    fmt.Printf("Sending welcome email to: %s\n", user.Email)
    
    // In real app:
    // emailService.Send(ctx, WelcomeEmail{
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
```

## Step 4: Build the Pipeline

Now compose everything into a pipeline:

```go
func createRegistrationPipeline() pipz.Chainable[User] {
    return pipz.Sequential(
        // Validation phase
        pipz.Effect("validate", validateUser),
        pipz.Effect("check_duplicate", checkDuplicate),
        
        // Transformation phase
        pipz.Transform("enrich", enrichUser),
        
        // Persistence phase
        pipz.Effect("save", saveUser),
        
        // Post-registration phase (parallel)
        pipz.Concurrent(
            pipz.Effect("send_welcome", sendWelcomeEmail),
            pipz.Effect("log_analytics", logRegistration),
        ),
    )
}
```

Note: Since `Concurrent` requires `Cloner`, let's implement it:

```go
func (u User) Clone() User {
    // User has no pointer fields, so simple copy works
    return u
}
```

## Step 5: Use the Pipeline

```go
func main() {
    pipeline := createRegistrationPipeline()
    
    // Test with valid user
    newUser := User{
        Email:    "john.doe@example.com",
        Username: "johndoe",
        Password: "securepassword123", // Would be hashed in real app
        FullName: "John Doe",
    }
    
    ctx := context.Background()
    registered, err := pipeline.Process(ctx, newUser)
    if err != nil {
        fmt.Printf("Registration failed: %v\n", err)
        return
    }
    
    fmt.Printf("Successfully registered: %+v\n", registered)
}
```

## Step 6: Add Error Handling

Make the pipeline more robust:

```go
func createRobustRegistrationPipeline() pipz.Chainable[User] {
    return pipz.Sequential(
        // Validation with early exit
        pipz.Effect("validate", validateUser),
        pipz.Effect("check_duplicate", checkDuplicate),
        
        // Enrich data
        pipz.Transform("enrich", enrichUser),
        
        // Save with retry
        pipz.RetryWithBackoff(
            pipz.Effect("save", saveUser),
            3,
            100*time.Millisecond,
        ),
        
        // Non-critical operations shouldn't fail registration
        pipz.Concurrent(
            pipz.WithErrorHandler(
                pipz.Effect("send_welcome", sendWelcomeEmail),
                pipz.Effect("log_email_error", func(ctx context.Context, err error) error {
                    fmt.Printf("Failed to send welcome email: %v\n", err)
                    return nil
                }),
            ),
            pipz.Effect("log_analytics", logRegistration),
        ),
    )
}
```

## Step 7: Add Conditional Logic

Add premium user handling:

```go
type UserType string

const (
    TypeRegular UserType = "regular"
    TypePremium UserType = "premium"
)

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

func createConditionalPipeline() pipz.Chainable[User] {
    return pipz.Sequential(
        // Common steps
        pipz.Effect("validate", validateUser),
        pipz.Effect("check_duplicate", checkDuplicate),
        pipz.Transform("enrich", enrichUser),
        pipz.Effect("save", saveUser),
        
        // Route based on user type
        pipz.Switch(
            detectUserType,
            map[UserType]pipz.Chainable[User]{
                TypeRegular: pipz.Effect("regular_onboarding", func(ctx context.Context, u User) error {
                    fmt.Println("Starting regular onboarding flow")
                    return nil
                }),
                TypePremium: pipz.Sequential(
                    pipz.Effect("premium_onboarding", func(ctx context.Context, u User) error {
                        fmt.Println("Starting premium onboarding flow")
                        return nil
                    }),
                    pipz.Effect("assign_account_manager", func(ctx context.Context, u User) error {
                        fmt.Println("Assigning dedicated account manager")
                        return nil
                    }),
                ),
            },
        ),
    )
}
```

## Step 8: Test Your Pipeline

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
3. **Use the Right Connector**: Sequential for steps, Concurrent for parallel work
4. **Handle Errors Appropriately**: Critical vs non-critical operations
5. **Test Each Component**: Processors are independently testable

## Next Steps

- [Error Recovery Patterns](./error-recovery.md) - Advanced error handling
- [Testing Pipelines](./testing.md) - Comprehensive testing strategies
- [Performance Optimization](./performance.md) - Making pipelines fast