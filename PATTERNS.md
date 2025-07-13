# pipz Pattern Catalog

A comprehensive collection of patterns and recipes for common use cases with pipz.

## Table of Contents

- [Validation Patterns](#validation-patterns)
- [Transformation Patterns](#transformation-patterns)
- [Error Handling Patterns](#error-handling-patterns)
- [Integration Patterns](#integration-patterns)
- [Testing Patterns](#testing-patterns)
- [Performance Patterns](#performance-patterns)

## Validation Patterns

### Multi-Field Validation

Validate multiple fields with clear error messages:

```go
func ValidateUser() *pipz.Chain[User] {
    return pipz.NewChain(
        pipz.Validate(func(u User) error {
            if u.Email == "" {
                return errors.New("email is required")
            }
            if !strings.Contains(u.Email, "@") {
                return errors.New("email must contain @")
            }
            return nil
        }),
        pipz.Validate(func(u User) error {
            if len(u.Password) < 8 {
                return errors.New("password must be at least 8 characters")
            }
            return nil
        }),
        pipz.Validate(func(u User) error {
            if u.Age < 18 {
                return errors.New("must be 18 or older")
            }
            return nil
        }),
    )
}
```

### Conditional Validation

Apply validation only when certain conditions are met:

```go
func ValidatePremiumUser() Chainable[User] {
    return pipz.Mutate(
        func(u User) bool { return u.IsPremium },
        func(u User) User {
            // Premium users need additional validation
            if u.CreditCard == "" {
                panic("premium users must have credit card")
            }
            return u
        },
    )
}
```

### Cross-Field Validation

Validate relationships between fields:

```go
validateDateRange := pipz.Validate(func(booking Booking) error {
    if booking.EndDate.Before(booking.StartDate) {
        return errors.New("end date must be after start date")
    }
    if booking.EndDate.Sub(booking.StartDate) > 30*24*time.Hour {
        return errors.New("booking cannot exceed 30 days")
    }
    return nil
})
```

## Transformation Patterns

### Normalization Pipeline

Standardize data formats:

```go
func NormalizeUser() *pipz.Chain[User] {
    return pipz.NewChain(
        // Normalize email
        pipz.Transform(func(u User) User {
            u.Email = strings.ToLower(strings.TrimSpace(u.Email))
            return u
        }),
        // Normalize phone
        pipz.Transform(func(u User) User {
            u.Phone = strings.ReplaceAll(u.Phone, "-", "")
            u.Phone = strings.ReplaceAll(u.Phone, " ", "")
            u.Phone = strings.ReplaceAll(u.Phone, "(", "")
            u.Phone = strings.ReplaceAll(u.Phone, ")", "")
            return u
        }),
        // Capitalize name
        pipz.Transform(func(u User) User {
            u.FirstName = strings.Title(strings.ToLower(u.FirstName))
            u.LastName = strings.Title(strings.ToLower(u.LastName))
            return u
        }),
    )
}
```

### Data Enrichment

Add derived data:

```go
func EnrichOrder() *pipz.Chain[Order] {
    return pipz.NewChain(
        // Calculate subtotal
        pipz.Transform(func(o Order) Order {
            var subtotal float64
            for _, item := range o.Items {
                subtotal += item.Price * float64(item.Quantity)
            }
            o.Subtotal = subtotal
            return o
        }),
        // Apply discount
        pipz.Mutate(
            func(o Order) bool { return o.Subtotal > 100 },
            func(o Order) Order {
                o.Discount = o.Subtotal * 0.1
                return o
            },
        ),
        // Calculate total
        pipz.Transform(func(o Order) Order {
            o.Total = o.Subtotal - o.Discount + o.Tax
            return o
        }),
    )
}
```

### Nested Object Transformation

Transform nested structures:

```go
func TransformCompany() Chainable[Company] {
    return pipz.Transform(func(c Company) Company {
        // Transform all employees
        for i, emp := range c.Employees {
            processed, _ := employeePipeline.Process(emp)
            c.Employees[i] = processed
        }
        // Transform all departments
        for i, dept := range c.Departments {
            processed, _ := departmentPipeline.Process(dept)
            c.Departments[i] = processed
        }
        return c
    })
}
```

## Error Handling Patterns

### Retry Pattern

Implement retry logic for unstable operations:

```go
func RetryableOperation(maxRetries int) Chainable[Data] {
    return pipz.Apply(func(d Data) (Data, error) {
        var lastErr error
        for i := 0; i < maxRetries; i++ {
            result, err := unstableOperation(d)
            if err == nil {
                return result, nil
            }
            lastErr = err
            if i < maxRetries-1 {
                time.Sleep(time.Duration(i+1) * time.Second)
            }
        }
        return d, fmt.Errorf("failed after %d retries: %w", maxRetries, lastErr)
    })
}
```

### Fallback Pattern

Provide fallback values on error:

```go
func EnrichWithFallback() Chainable[User] {
    return pipz.Apply(func(u User) (User, error) {
        // Try primary service
        location, err := primaryGeoService.Lookup(u.IPAddress)
        if err == nil {
            u.Location = location
            return u, nil
        }
        
        // Try fallback service
        location, err = fallbackGeoService.Lookup(u.IPAddress)
        if err == nil {
            u.Location = location
            return u, nil
        }
        
        // Use default
        u.Location = "Unknown"
        return u, nil
    })
}
```

### Circuit Breaker Pattern

Prevent cascading failures:

```go
type CircuitBreaker struct {
    failures  int
    lastFail  time.Time
    threshold int
    timeout   time.Duration
    mu        sync.Mutex
}

func (cb *CircuitBreaker) Wrap(operation Chainable[Data]) Chainable[Data] {
    return pipz.Apply(func(d Data) (Data, error) {
        cb.mu.Lock()
        defer cb.mu.Unlock()
        
        // Check if circuit is open
        if cb.failures >= cb.threshold {
            if time.Since(cb.lastFail) < cb.timeout {
                return d, errors.New("circuit breaker open")
            }
            // Reset after timeout
            cb.failures = 0
        }
        
        // Try operation
        result, err := operation.Process(d)
        if err != nil {
            cb.failures++
            cb.lastFail = time.Now()
            return d, err
        }
        
        // Success, reset failures
        cb.failures = 0
        return result, nil
    })
}
```

## Integration Patterns

### Database Integration

Wrap database operations:

```go
func SaveToDatabase(db *sql.DB) Chainable[User] {
    return pipz.Apply(func(u User) (User, error) {
        tx, err := db.Begin()
        if err != nil {
            return u, err
        }
        defer tx.Rollback()
        
        result, err := tx.Exec(
            "INSERT INTO users (email, name) VALUES (?, ?)",
            u.Email, u.Name,
        )
        if err != nil {
            return u, err
        }
        
        id, err := result.LastInsertId()
        if err != nil {
            return u, err
        }
        
        u.ID = id
        return u, tx.Commit()
    })
}
```

### HTTP Client Integration

Make HTTP requests in pipelines:

```go
func CallExternalAPI(client *http.Client) Chainable[Request] {
    return pipz.Apply(func(r Request) (Request, error) {
        // Build request
        httpReq, err := http.NewRequest("POST", r.URL, bytes.NewReader(r.Body))
        if err != nil {
            return r, err
        }
        
        // Add headers
        for k, v := range r.Headers {
            httpReq.Header.Set(k, v)
        }
        
        // Make request
        resp, err := client.Do(httpReq)
        if err != nil {
            return r, err
        }
        defer resp.Body.Close()
        
        // Read response
        body, err := io.ReadAll(resp.Body)
        if err != nil {
            return r, err
        }
        
        r.Response = body
        r.StatusCode = resp.StatusCode
        return r, nil
    })
}
```

### Message Queue Integration

Process messages from queues:

```go
func ProcessMessage(queue Queue) Chainable[Message] {
    return pipz.NewChain(
        // Decode message
        pipz.Apply(func(m Message) (Message, error) {
            return queue.Decode(m.Raw)
        }),
        // Validate
        validateMessage,
        // Process
        processMessage,
        // Acknowledge
        pipz.Effect(func(m Message) {
            queue.Ack(m.ID)
        }),
    )
}
```

## Testing Patterns

### Table-Driven Tests

Test pipelines with multiple scenarios:

```go
func TestUserPipeline(t *testing.T) {
    pipeline := BuildUserPipeline()
    
    tests := []struct {
        name    string
        input   User
        want    User
        wantErr bool
    }{
        {
            name:    "valid user",
            input:   User{Email: "TEST@EXAMPLE.COM", Age: 25},
            want:    User{Email: "test@example.com", Age: 25},
            wantErr: false,
        },
        {
            name:    "invalid email",
            input:   User{Email: "invalid", Age: 25},
            want:    User{},
            wantErr: true,
        },
        {
            name:    "underage user",
            input:   User{Email: "test@example.com", Age: 16},
            want:    User{},
            wantErr: true,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := pipeline.Process(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("Process() error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
                t.Errorf("Process() = %v, want %v", got, tt.want)
            }
        })
    }
}
```

### Mock External Dependencies

Create testable pipelines with dependency injection:

```go
type UserService struct {
    DB     Database
    Cache  Cache
    Mailer Mailer
}

func (s *UserService) CreatePipeline() Chainable[User] {
    return pipz.NewChain(
        validateUser,
        normalizeUser,
        pipz.Apply(func(u User) (User, error) {
            return s.DB.Save(u)
        }),
        pipz.Effect(func(u User) {
            s.Cache.Set(u.ID, u)
        }),
        pipz.Effect(func(u User) {
            s.Mailer.SendWelcome(u.Email)
        }),
    )
}

// In tests
func TestUserCreation(t *testing.T) {
    service := UserService{
        DB:     &MockDB{},
        Cache:  &MockCache{},
        Mailer: &MockMailer{},
    }
    
    pipeline := service.CreatePipeline()
    // Test with mocks...
}
```

### Benchmark Patterns

Benchmark pipeline performance:

```go
func BenchmarkPipeline(b *testing.B) {
    pipeline := BuildComplexPipeline()
    data := GenerateTestData()
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, _ = pipeline.Process(data)
    }
}

func BenchmarkPipelineParallel(b *testing.B) {
    pipeline := BuildComplexPipeline()
    data := GenerateTestData()
    
    b.RunParallel(func(pb *testing.PB) {
        for pb.Next() {
            _, _ = pipeline.Process(data)
        }
    })
}
```

## Performance Patterns

### Batch Processing

Process multiple items efficiently:

```go
func BatchProcessor(pipeline Chainable[Item]) Chainable[[]Item] {
    return pipz.Apply(func(items []Item) ([]Item, error) {
        results := make([]Item, len(items))
        
        for i, item := range items {
            processed, err := pipeline.Process(item)
            if err != nil {
                return nil, fmt.Errorf("failed to process item %d: %w", i, err)
            }
            results[i] = processed
        }
        
        return results, nil
    })
}
```

### Parallel Processing

Process items in parallel with worker pool:

```go
func ParallelProcessor(pipeline Chainable[Item], workers int) Chainable[[]Item] {
    return pipz.Apply(func(items []Item) ([]Item, error) {
        var wg sync.WaitGroup
        results := make([]Item, len(items))
        errors := make([]error, len(items))
        
        // Create work channel
        work := make(chan int, len(items))
        for i := range items {
            work <- i
        }
        close(work)
        
        // Start workers
        wg.Add(workers)
        for w := 0; w < workers; w++ {
            go func() {
                defer wg.Done()
                for idx := range work {
                    results[idx], errors[idx] = pipeline.Process(items[idx])
                }
            }()
        }
        
        wg.Wait()
        
        // Check errors
        for i, err := range errors {
            if err != nil {
                return nil, fmt.Errorf("item %d failed: %w", i, err)
            }
        }
        
        return results, nil
    })
}
```

### Caching Pattern

Add caching to expensive operations:

```go
func CachedProcessor(cache Cache, ttl time.Duration) Chainable[Request] {
    return pipz.Apply(func(r Request) (Request, error) {
        // Check cache
        key := r.CacheKey()
        if cached, found := cache.Get(key); found {
            r.Response = cached
            return r, nil
        }
        
        // Process request
        processed, err := expensiveOperation(r)
        if err != nil {
            return r, err
        }
        
        // Cache result
        cache.Set(key, processed.Response, ttl)
        return processed, nil
    })
}
```