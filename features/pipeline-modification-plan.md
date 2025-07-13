# Pipeline Modification API - Implementation Plan

## Overview

This feature adds dynamic pipeline modification capabilities to pipz, enabling runtime manipulation of processor sequences in both `Contract[T]` and `Chain[T]` types. Users can now build adaptive pipelines that change based on runtime conditions, implement queue/stack workflows, and precisely control processor ordering.

## Motivation

### Current State
- Pipelines are static once built via `Register()` or `Add()`
- No way to remove, reorder, or conditionally modify processors
- Limited to append-only composition patterns

### Desired State  
- Full CRUD operations on pipeline processors
- Queue (FIFO) and stack (LIFO) workflow support
- Thread-safe runtime modification
- Precise positioning and movement controls

### Use Cases
- **Conditional processing**: Add fraud detection only for high-value transactions
- **Queue workflows**: Build processing queues with `PushTail()` + `PopHead()`
- **Stack workflows**: Build undo/redo systems with `PushTail()` + `PopTail()`
- **Dynamic optimization**: Reorder processors based on runtime performance metrics
- **A/B testing**: Swap processors to test different implementations

## API Design

### Core Operations

#### Queue/Stack Operations
```go
// Add processors
PushHead(processors ...T)     // Add to front (runs first)
PushTail(processors ...T)     // Add to back (runs last) 

// Remove processors
PopHead() (T, error)          // Remove from front
PopTail() (T, error)          // Remove from back
```

#### Movement Operations  
```go
MoveToHead(index int) error   // Move processor to front
MoveToTail(index int) error   // Move processor to back
MoveTo(from, to int) error    // Move processor to specific position
Swap(i, j int) error          // Swap two processors
Reverse()                     // Reverse entire pipeline order
```

#### Precise Operations
```go
InsertAt(index int, processors ...T)  // Insert at specific position
RemoveAt(index int) error             // Remove by index
ReplaceAt(index int, processor T) error // Replace processor at index
```

#### Utility Operations
```go
Clear()           // Remove all processors
Len() int         // Get processor count
IsEmpty() bool    // Check if empty
```

### Type-Specific Signatures

#### Contract[T]
```go
type Contract[T any] struct {
    mu         sync.RWMutex
    processors []Processor[T]
}

// All modification methods take Processor[T] parameters
func (c *Contract[T]) PushHead(processors ...Processor[T])
func (c *Contract[T]) PopHead() (Processor[T], error)
// ... etc
```

#### Chain[T]  
```go
type Chain[T any] struct {
    mu         sync.RWMutex  
    processors []Chainable[T]
}

// All modification methods take Chainable[T] parameters
func (c *Chain[T]) PushHead(processors ...Chainable[T])
func (c *Chain[T]) PopHead() (Chainable[T], error)
// ... etc
```

### Error Handling

Standard error types for consistent handling:
```go
var (
    ErrIndexOutOfBounds = errors.New("index out of bounds")
    ErrEmptyPipeline    = errors.New("pipeline is empty")  
    ErrInvalidRange     = errors.New("invalid range")
)
```

### Thread Safety

- **RWMutex protection**: Read lock for `Process()`, write lock for modifications
- **Atomic operations**: All modification methods are thread-safe
- **Consistent state**: No partial modifications possible

## Implementation Plan

### Phase 1: Core Infrastructure
1. **Add mutex fields** to Contract and Chain structs
2. **Update Process() methods** with read locks
3. **Define error types** for consistent error handling
4. **Add basic utility methods** (Len, IsEmpty, Clear)

### Phase 2: Queue/Stack Operations  
1. **Implement PushHead/PushTail** for both types
2. **Implement PopHead/PopTail** with proper error handling
3. **Add comprehensive tests** for edge cases (empty pipelines, etc.)
4. **Add benchmarks** to measure performance impact

### Phase 3: Movement Operations
1. **Implement MoveToHead/MoveToTail** with bounds checking
2. **Implement MoveTo and Swap** with validation  
3. **Implement Reverse** operation
4. **Test complex movement scenarios**

### Phase 4: Precise Operations
1. **Implement InsertAt** using `slices.Insert()`
2. **Implement RemoveAt** using `slices.Delete()` with bounds checking
3. **Implement ReplaceAt** using `slices.Replace()` with bounds checking
4. **Add benchmarks comparing slices package vs manual operations**

### Phase 5: Integration & Documentation
1. **Update existing Register/Add methods** to use PushTail internally
2. **Add comprehensive example demos**
3. **Update API documentation**
4. **Performance analysis and optimization**

## Breaking Changes

### API Method Renames
To provide consistent, clear naming:

```go
// OLD API (deprecated but maintained for compatibility)
contract.Register(processor1, processor2)
chain.Add(chainable1, chainable2)

// NEW API (recommended)  
contract.PushTail(processor1, processor2)
chain.PushTail(chainable1, chainable2)
```

**Migration strategy**: Keep old methods as aliases initially, add deprecation warnings, eventual removal in future major version.

## Performance Considerations

### Memory Allocation
- **All operations use `slices` package**: Optimized by Go team for performance
- **PushHead operations**: O(n) but optimized with `slices.Insert(slice, 0, items...)`
- **InsertAt operations**: O(n) but optimized with `slices.Insert(slice, index, items...)`
- **PushTail operations**: O(1) amortized (same as current append)
- **Remove operations**: O(n) but optimized with `slices.Delete()`
- **Movement operations**: Combination of slices package optimizations

### Implementation Benefits
- **Go standard library optimized**: `slices` package provides best-in-class performance
- **Cleaner code**: No manual slice manipulation, idiomatic Go
- **Consistent behavior**: All operations use same underlying optimizations
- **Future-proof**: Benefits from Go team's ongoing optimizations

### Benchmarking Plan
- Compare modification operations vs current append-only
- Memory allocation profiling
- Concurrent access performance with mutex overhead
- Real-world usage pattern simulation

## Testing Strategy

### Unit Tests
- **Edge cases**: Empty pipelines, out-of-bounds operations
- **State consistency**: Verify pipeline state after modifications  
- **Error conditions**: Proper error types and messages
- **Thread safety**: Concurrent modification and execution

### Integration Tests  
- **Workflow patterns**: Complete FIFO/LIFO examples
- **Mixed operations**: Combining different modification types
- **Performance regression**: Ensure existing functionality unaffected

### Example Tests
- **Dynamic pipeline demo**: Runtime modification based on conditions
- **Queue workflow demo**: Producer/consumer pattern
- **Stack workflow demo**: Undo/redo pattern  
- **Performance comparison**: Before/after modification benchmarks

## Documentation Updates

### README.md
- Add "Dynamic Pipeline Modification" section to main content
- Update quick start examples to show modification capabilities
- Enhance value proposition: static → dynamic pipelines

### API_REFERENCE.md
- Complete API documentation for all new methods
- Error handling examples
- Performance characteristics for each operation

### New Documentation
- **PIPELINE_PATTERNS.md**: Patterns for dynamic pipelines
- **demos/dynamic_pipeline_demo.go**: Interactive modification demo
- **examples/queue_workflow/**: FIFO processing example
- **examples/stack_workflow/**: LIFO processing example

### Enhanced Examples
- **payment example**: Add conditional fraud detection
- **security example**: Add dynamic audit rule injection  
- **validation example**: Add progressive validation complexity

## Success Metrics

### Functionality
- [ ] All modification operations work correctly
- [ ] Thread-safe under concurrent access
- [ ] Proper error handling for all edge cases
- [ ] Performance acceptable for intended use cases

### Developer Experience
- [ ] Clear, intuitive API design
- [ ] Comprehensive documentation and examples
- [ ] Easy migration path from existing API
- [ ] Performance characteristics well understood

### Community Adoption
- [ ] Positive feedback on API design
- [ ] Real-world usage examples from community
- [ ] No significant performance regressions reported
- [ ] Clear use case differentiation (when to use vs not use)

---

## Review from New User Perspective

### Clarity and Accessibility
**Strengths:**
- Clear motivation with concrete use cases
- Intuitive API naming (PushHead/PushTail/PopHead/PopTail)
- Comprehensive examples showing different patterns
- Well-defined error handling approach

**Potential Concerns:**
- **Complexity increase**: API surface area grows significantly
- **Performance implications**: New users might not understand O(n) costs
- **When to use**: Unclear when dynamic modification is appropriate vs static pipelines
- **Learning curve**: Many new concepts (FIFO/LIFO, movement operations, thread safety)

### Recommendations for New User Experience:

1. **Start Simple**: Lead with basic PushTail/PopHead queue pattern
2. **Progressive Disclosure**: Introduction → Basic Operations → Advanced Operations → Performance Tuning
3. **Clear Guidance**: Document when to use dynamic vs static pipelines
4. **Performance Warnings**: Clearly mark expensive operations (PushHead, InsertAt, etc.)
5. **Common Patterns First**: Focus documentation on 80% use cases (queue/stack), then advanced features

### Missing Elements for New Users:
- **Decision tree**: When to use modification vs composition  
- **Migration examples**: Converting existing static pipelines to dynamic
- **Anti-patterns**: What NOT to do with dynamic modification
- **Go version requirements**: Requires Go 1.21+ for `slices` package

### Performance Impact Reassessment:
With the `slices` package, performance concerns are significantly reduced:
- **Standard library optimized**: All operations use Go team's best practices
- **Reasonable performance**: Even O(n) operations are well-optimized for typical pipeline sizes
- **Less documentation overhead**: Don't need extensive performance warnings
- **Focus on functionality**: Emphasize capabilities over performance trade-offs

This plan provides a solid foundation with modern Go idioms and performance characteristics.