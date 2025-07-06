# SimpleContract Performance Analysis

## Benchmark Results Summary

| Scenario | Global Contract | Simple Contract | Performance Gain |
|----------|----------------|-----------------|------------------|
| Sequential Processing | 15,039 ns/op | 4,168 ns/op | **3.6x faster** |
| Concurrent Processing | 2,284 ns/op | 504 ns/op | **4.5x faster** |
| Memory Usage | 2,342 B/op | 680 B/op | **71% less memory** |
| Allocations | 54 allocs/op | 15 allocs/op | **72% fewer allocs** |

## Key Findings

### 1. Sequential Performance
- **SimpleContract is 3.6x faster** than the global Contract
- Global: ~66,500 ops/sec
- Simple: ~240,000 ops/sec

### 2. Concurrent Performance
- **SimpleContract scales much better** under concurrent load
- Global: ~438,000 ops/sec (limited by mutex contention)
- Simple: ~1,984,000 ops/sec (no global locks)
- **4.5x improvement** in concurrent scenarios

### 3. Memory Efficiency
- SimpleContract uses **71% less memory** (680B vs 2,342B)
- **72% fewer allocations** (15 vs 54)
- No registry lookup overhead

### 4. Lookup Cost Analysis
- Contract lookup adds ~400ns overhead per operation
- Caching the contract reference saves minimal time (~354ns)
- The real cost is in the global registry mutex lock

## Performance Breakdown

### Global Contract Overhead Sources:
1. **Registry Lookup**: ~400-500ns
   - Type signature generation
   - Map lookup with mutex read lock
   
2. **Mutex Contention**: ~1,500-2,000ns under load
   - Read lock on every Process() call
   - Write lock during Register()
   
3. **Extra Allocations**: ~1,662 bytes
   - Registry key generation
   - Interface conversions
   - Mutex operations

### SimpleContract Advantages:
1. **No Registry Lookup**: Direct field access
2. **No Mutex Operations**: Lock-free processing
3. **Fewer Allocations**: Direct processor slice access
4. **Better CPU Cache Locality**: All data is local

## Trade-offs Analysis

### SimpleContract Pros:
✅ **3-4x faster** processing
✅ **70% less memory** usage
✅ **Better concurrent scaling**
✅ **Predictable performance**
✅ **No global state**
✅ **Easier to test**

### SimpleContract Cons:
❌ **No global discovery** - must pass references
❌ **No type-based lookup** - loses key pipz feature
❌ **Manual wiring** required
❌ **Can't share pipelines** across packages without explicit passing

## Use Case Recommendations

### When to Use SimpleContract:

1. **High-Performance Hot Paths**
   ```go
   // In a tight loop or high-frequency operation
   contract := pipz.NewSimpleContract[Data]()
   contract.Register(processors...)
   
   for _, item := range millionsOfItems {
       result, _ := contract.Process(item)
   }
   ```

2. **Concurrent Processing**
   ```go
   // When processing in parallel
   contract := pipz.NewSimpleContract[Request]()
   // Each goroutine can use the same contract safely
   ```

3. **Isolated Components**
   ```go
   // When discovery isn't needed
   type Service struct {
       pipeline *pipz.SimpleContract[Order]
   }
   ```

### When to Keep Global Contract:

1. **Cross-Package Discovery**
   ```go
   // When you need type-based discovery
   contract := pipz.GetContract[User](authKey)
   ```

2. **Dynamic Pipeline Selection**
   ```go
   // When pipeline depends on runtime conditions
   key := getKeyForTenant(tenantID)
   contract := pipz.GetContract[Data](key)
   ```

3. **Plugin Systems**
   ```go
   // When external code needs to find pipelines
   ```

## Hybrid Approach Recommendation

Consider offering both APIs:

```go
// For discovery and convenience
globalContract := pipz.GetContract[User](authKey)

// For performance-critical paths
fastContract := pipz.NewSimpleContract[User]()
fastContract.Register(globalContract.GetProcessors()...) // Hypothetical API

// Or cache the simple contract
var cachedContract = createHighPerfContract()
```

## Conclusion

SimpleContract offers **significant performance benefits** (3-4x faster, 70% less memory) at the cost of losing global discovery. For performance-critical applications, offering SimpleContract as an alternative API would be valuable.

The performance gains are especially pronounced in:
- High-frequency operations
- Concurrent processing
- Memory-constrained environments
- Latency-sensitive applications

The trade-off is clear: **performance vs discoverability**. Many applications could benefit from having both options available.