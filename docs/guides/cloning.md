# Clone Implementation Guide

Understanding and implementing the Clone() method correctly is critical for safe concurrent processing in pipz pipelines.

## Why Clone() Matters

When using concurrent connectors (`Concurrent`, `Race`, `Contest`), pipz creates independent copies of your data for each parallel processor. This isolation prevents data races and ensures predictable behavior. Without proper cloning, concurrent processors can corrupt each other's data, leading to subtle bugs that are difficult to debug.

## The Cloner Interface

```go
type Cloner[T any] interface {
    Clone() T
}
```

Your data types must implement this interface to work with concurrent connectors.

## Implementation Patterns

### Pattern 1: Simple Value Types

For types containing only value fields (no pointers, slices, or maps):

```go
type Config struct {
    MaxRetries int
    Timeout    time.Duration
    Enabled    bool
}

// Simple copy is sufficient for value types
func (c Config) Clone() Config {
    return c  // All fields are copied by value
}
```

### Pattern 2: Types with Slices

Slices share underlying arrays, so they must be deep copied:

❌ **WRONG: Shallow copy shares slice memory**
```go
func (o Order) Clone() Order {
    return Order{
        ID:    o.ID,
        Items: o.Items,  // DANGER: Shares underlying array!
    }
}
// Concurrent processors will see each other's modifications!
```

✅ **RIGHT: Deep copy creates independent slice**
```go
func (o Order) Clone() Order {
    // Create new slice with same capacity for efficiency
    items := make([]Item, len(o.Items))
    copy(items, o.Items)
    
    return Order{
        ID:    o.ID,
        Items: items,  // Independent copy
    }
}
```

### Pattern 3: Types with Maps

Maps are reference types and must be copied explicitly:

❌ **WRONG: Shallow copy shares map reference**
```go
func (r Request) Clone() Request {
    return Request{
        ID:      r.ID,
        Headers: r.Headers,  // DANGER: Same map instance!
    }
}
```

✅ **RIGHT: Deep copy creates independent map**
```go
func (r Request) Clone() Request {
    // Create new map with same capacity
    headers := make(map[string]string, len(r.Headers))
    for k, v := range r.Headers {
        headers[k] = v
    }
    
    return Request{
        ID:      r.ID,
        Headers: headers,  // Independent copy
    }
}
```

### Pattern 4: Types with Pointers

Pointers require careful consideration - decide whether to share or copy the pointed-to value:

```go
type Document struct {
    ID       string
    Content  string
    Metadata *Metadata  // Pointer field
}

// Option 1: Share the pointed-to value (if immutable)
func (d Document) Clone() Document {
    return Document{
        ID:       d.ID,
        Content:  d.Content,
        Metadata: d.Metadata,  // Shares same Metadata instance
    }
}

// Option 2: Deep copy the pointed-to value (if mutable)
func (d Document) Clone() Document {
    var metadata *Metadata
    if d.Metadata != nil {
        // Create independent copy
        metaCopy := *d.Metadata
        metadata = &metaCopy
    }
    
    return Document{
        ID:       d.ID,
        Content:  d.Content,
        Metadata: metadata,  // Independent copy
    }
}
```

### Pattern 5: Nested Structures

For complex nested structures, implement Clone() recursively:

```go
type Order struct {
    ID       string
    Customer Customer
    Items    []OrderItem
    Metadata map[string]any
}

type Customer struct {
    ID        string
    Name      string
    Addresses []Address
}

type OrderItem struct {
    ProductID string
    Quantity  int
    Options   map[string]string
}

// Comprehensive deep clone
func (o Order) Clone() Order {
    // Clone nested struct (if it has reference types)
    customer := o.Customer.Clone()
    
    // Clone slice of structs
    items := make([]OrderItem, len(o.Items))
    for i, item := range o.Items {
        items[i] = item.Clone()
    }
    
    // Clone map with interface{} values
    metadata := make(map[string]any, len(o.Metadata))
    for k, v := range o.Metadata {
        // Handle different value types
        switch val := v.(type) {
        case []byte:
            // Deep copy byte slices
            b := make([]byte, len(val))
            copy(b, val)
            metadata[k] = b
        default:
            // Copy other values directly
            metadata[k] = val
        }
    }
    
    return Order{
        ID:       o.ID,
        Customer: customer,
        Items:    items,
        Metadata: metadata,
    }
}

func (c Customer) Clone() Customer {
    addresses := make([]Address, len(c.Addresses))
    copy(addresses, c.Addresses)
    
    return Customer{
        ID:        c.ID,
        Name:      c.Name,
        Addresses: addresses,
    }
}

func (i OrderItem) Clone() OrderItem {
    options := make(map[string]string, len(i.Options))
    for k, v := range i.Options {
        options[k] = v
    }
    
    return OrderItem{
        ProductID: i.ProductID,
        Quantity:  i.Quantity,
        Options:   options,
    }
}
```

## Testing Clone Implementations

### Test 1: Independence Test

Verify that modifications to the clone don't affect the original:

```go
func TestCloneIndependence(t *testing.T) {
    original := Order{
        ID:    "order-1",
        Items: []Item{{ProductID: "prod-1", Quantity: 1}},
        Metadata: map[string]any{
            "priority": "high",
        },
    }
    
    // Create clone
    cloned := original.Clone()
    
    // Modify clone
    cloned.Items[0].Quantity = 5
    cloned.Metadata["priority"] = "low"
    
    // Verify original is unchanged
    if original.Items[0].Quantity != 1 {
        t.Error("Clone modification affected original slice")
    }
    if original.Metadata["priority"] != "high" {
        t.Error("Clone modification affected original map")
    }
}
```

### Test 2: Race Condition Detection

Use Go's race detector to catch sharing issues:

```go
func TestCloneConcurrency(t *testing.T) {
    // Run with: go test -race
    
    original := Order{
        ID:    "order-1",
        Items: []Item{{ProductID: "prod-1", Quantity: 1}},
    }
    
    // Simulate concurrent processing
    var wg sync.WaitGroup
    for i := 0; i < 10; i++ {
        wg.Add(1)
        go func(n int) {
            defer wg.Done()
            
            // Each goroutine gets its own clone
            clone := original.Clone()
            
            // Modify clone independently
            clone.Items[0].Quantity = n
            
            // Process...
            time.Sleep(10 * time.Millisecond)
            
            // Verify our modifications
            if clone.Items[0].Quantity != n {
                t.Errorf("Unexpected quantity: got %d, want %d", 
                    clone.Items[0].Quantity, n)
            }
        }(i)
    }
    wg.Wait()
}
```

### Test 3: Benchmark Clone Performance

Measure the overhead of cloning:

```go
func BenchmarkClone(b *testing.B) {
    order := Order{
        ID:       "order-1",
        Items:    make([]Item, 100),
        Metadata: make(map[string]any, 50),
    }
    
    // Initialize test data
    for i := range order.Items {
        order.Items[i] = Item{
            ProductID: fmt.Sprintf("prod-%d", i),
            Quantity:  i,
        }
    }
    for i := 0; i < 50; i++ {
        order.Metadata[fmt.Sprintf("key-%d", i)] = i
    }
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _ = order.Clone()
    }
}
```

## Common Pitfalls

### Pitfall 1: Forgetting Nested Slices

```go
type Report struct {
    Sections []Section
}

type Section struct {
    Title string
    Data  []byte  // Easy to miss!
}

❌ **WRONG: Nested slices not cloned**
func (r Report) Clone() Report {
    sections := make([]Section, len(r.Sections))
    copy(sections, r.Sections)  // Shallow copy of structs!
    return Report{Sections: sections}
}

✅ **RIGHT: Clone nested slices too**
func (r Report) Clone() Report {
    sections := make([]Section, len(r.Sections))
    for i, s := range r.Sections {
        data := make([]byte, len(s.Data))
        copy(data, s.Data)
        sections[i] = Section{
            Title: s.Title,
            Data:  data,
        }
    }
    return Report{Sections: sections}
}
```

### Pitfall 2: Shared Channel References

Channels should typically not be cloned:

```go
type Worker struct {
    ID      string
    Results chan Result  // Channels are for communication
}

func (w Worker) Clone() Worker {
    return Worker{
        ID:      w.ID,
        Results: w.Results,  // Share the channel - usually correct
    }
}
```

### Pitfall 3: Time and Sync Types

Some standard library types have special considerations:

```go
type Task struct {
    ID        string
    StartTime time.Time    // Value type, safe to copy
    mu        sync.Mutex   // NEVER copy a mutex!
    data      []byte
}

func (t *Task) Clone() Task {
    // Note: Returns value, not pointer
    t.mu.Lock()
    defer t.mu.Unlock()
    
    data := make([]byte, len(t.data))
    copy(data, t.data)
    
    return Task{
        ID:        t.ID,
        StartTime: t.StartTime,
        // mu: zero value (new mutex)
        data: data,
    }
}
```

## Performance Considerations

### Memory Allocation

Deep cloning allocates new memory. Consider the trade-offs:

```go
// Lightweight clone for mostly-immutable data
func (d Document) CloneLightweight() Document {
    // Only clone what might be modified
    return Document{
        ID:       d.ID,
        Metadata: d.Metadata,  // Share if read-only
        Content:  d.Content,   // Share large immutable data
        Tags:     cloneSlice(d.Tags),  // Clone mutable slice
    }
}

// Full deep clone for complete isolation
func (d Document) CloneDeep() Document {
    // Clone everything for total independence
    content := make([]byte, len(d.Content))
    copy(content, d.Content)
    
    return Document{
        ID:       d.ID,
        Metadata: cloneMap(d.Metadata),
        Content:  content,
        Tags:     cloneSlice(d.Tags),
    }
}
```

### Clone Pools

For high-frequency cloning, consider object pools:

```go
var orderPool = sync.Pool{
    New: func() any {
        return &Order{
            Items:    make([]Item, 0, 10),      // Pre-allocate capacity
            Metadata: make(map[string]any, 5),
        }
    },
}

func (o Order) CloneWithPool() Order {
    // Get pooled object
    clone := orderPool.Get().(*Order)
    
    // Reset and populate
    clone.ID = o.ID
    clone.Items = clone.Items[:0]  // Reuse slice backing
    clone.Items = append(clone.Items, o.Items...)
    
    // Clear and repopulate map
    for k := range clone.Metadata {
        delete(clone.Metadata, k)
    }
    for k, v := range o.Metadata {
        clone.Metadata[k] = v
    }
    
    return *clone
}
```

## When Clone() Errors Occur

If you see panics or race conditions with concurrent connectors, check:

1. **Missing Clone() implementation**: Type doesn't implement Cloner interface
2. **Shallow copies**: Slices/maps are being shared between goroutines
3. **Pointer fields**: Pointed-to values are being modified concurrently
4. **Interface fields**: Concrete types in interface{} fields need deep copying

## Debugging Clone Issues

Enable race detection during development:

```bash
# Run tests with race detector
go test -race ./...

# Run your application with race detector
go run -race main.go
```

Common race detector output indicating clone issues:
```
WARNING: DATA RACE
Write at 0x00c000180010 by goroutine 7:
  main.processOrder()
      /path/to/file.go:45 +0x64

Previous write at 0x00c000180010 by goroutine 6:
  main.processOrder()
      /path/to/file.go:45 +0x64
```

This indicates shared memory between goroutines - your Clone() is likely shallow copying.

## Best Practices Summary

1. **Always deep copy reference types** (slices, maps, pointers)
2. **Test with -race flag** during development
3. **Benchmark Clone() performance** for hot paths
4. **Document sharing decisions** when intentionally sharing data
5. **Consider immutability** to avoid cloning altogether
6. **Use code generation** for complex types (see tools like `deepcopy-gen`)

## Example: Production-Ready Clone

Here's a complete example following all best practices:

```go
package main

import (
    "sync"
    "time"
)

type Order struct {
    // Immutable fields (safe to share)
    ID        string
    CreatedAt time.Time
    
    // Mutable value fields (copied by value)
    Status string
    Total  float64
    
    // Reference types (need deep copy)
    Items      []OrderItem
    Tags       []string
    Attributes map[string]string
    
    // Pointer fields (decision needed)
    Customer *Customer
    
    // Never copy
    mu sync.RWMutex
}

func (o Order) Clone() Order {
    // Deep copy slices
    items := make([]OrderItem, len(o.Items))
    for i, item := range o.Items {
        items[i] = item.Clone()
    }
    
    tags := make([]string, len(o.Tags))
    copy(tags, o.Tags)
    
    // Deep copy map
    attributes := make(map[string]string, len(o.Attributes))
    for k, v := range o.Attributes {
        attributes[k] = v
    }
    
    // Deep copy pointer if needed
    var customer *Customer
    if o.Customer != nil {
        custCopy := o.Customer.Clone()
        customer = &custCopy
    }
    
    return Order{
        // Immutable fields
        ID:        o.ID,
        CreatedAt: o.CreatedAt,
        
        // Value fields
        Status: o.Status,
        Total:  o.Total,
        
        // Deep copied references
        Items:      items,
        Tags:       tags,
        Attributes: attributes,
        Customer:   customer,
        
        // mu gets zero value (new mutex)
    }
}

// Helper for OrderItem
func (i OrderItem) Clone() OrderItem {
    // Implement based on OrderItem structure
    return i
}

// Helper for Customer  
func (c Customer) Clone() Customer {
    // Implement based on Customer structure
    return c
}
```

This implementation ensures complete isolation between concurrent processors while maintaining good performance characteristics.