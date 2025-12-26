package pipz

import "encoding/json"

// FlowVariant is a discriminator for the Flow interface implementation type.
// Used for runtime type identification when type assertions are needed.
type FlowVariant string

// Flow variants for all pipeline node types.
const (
	// Connectors (have children).
	FlowVariantSequence       FlowVariant = "sequence"
	FlowVariantFallback       FlowVariant = "fallback"
	FlowVariantRace           FlowVariant = "race"
	FlowVariantContest        FlowVariant = "contest"
	FlowVariantConcurrent     FlowVariant = "concurrent"
	FlowVariantSwitch         FlowVariant = "switch"
	FlowVariantFilter         FlowVariant = "filter"
	FlowVariantHandle         FlowVariant = "handle"
	FlowVariantScaffold       FlowVariant = "scaffold"
	FlowVariantBackoff        FlowVariant = "backoff"
	FlowVariantRetry          FlowVariant = "retry"
	FlowVariantTimeout        FlowVariant = "timeout"
	FlowVariantRateLimiter    FlowVariant = "ratelimiter"
	FlowVariantCircuitBreaker FlowVariant = "circuitbreaker"
	FlowVariantWorkerpool     FlowVariant = "workerpool"
	FlowVariantPipeline       FlowVariant = "pipeline"

	// Processors (leaf nodes).
	FlowVariantApply     FlowVariant = "apply"
	FlowVariantTransform FlowVariant = "transform"
	FlowVariantEffect    FlowVariant = "effect"
	FlowVariantEnrich    FlowVariant = "enrich"
	FlowVariantMutate    FlowVariant = "mutate"
)

// Flow represents how children are organized within a connector node.
// Each connector type has its own Flow implementation that describes
// the semantic relationship between the connector and its children.
//
// Leaf nodes (processors) have nil Flow since they have no children.
type Flow interface {
	// Variant returns the type discriminator for this flow.
	Variant() FlowVariant
}

// FlowKey provides bidirectional type-safe extraction for Flow types.
// This follows the pattern established in capitan for type-safe field extraction.
//
// Example:
//
//	if seq, ok := SequenceKey.From(node); ok {
//	    for _, step := range seq.Steps {
//	        // process each step
//	    }
//	}
type FlowKey[T Flow] struct {
	variant FlowVariant
}

// Variant returns the flow type this key extracts.
func (k FlowKey[T]) Variant() FlowVariant { return k.variant }

// From extracts the typed Flow from a Node.
// Returns the flow and true if the node's flow matches this key's type,
// or zero value and false otherwise.
func (FlowKey[T]) From(node Node) (T, bool) {
	var zero T
	if node.Flow == nil {
		return zero, false
	}
	if flow, ok := node.Flow.(T); ok {
		return flow, true
	}
	return zero, false
}

// Pre-defined FlowKeys for each flow type.
var (
	SequenceKey       = FlowKey[SequenceFlow]{variant: FlowVariantSequence}
	FallbackKey       = FlowKey[FallbackFlow]{variant: FlowVariantFallback}
	RaceKey           = FlowKey[RaceFlow]{variant: FlowVariantRace}
	ContestKey        = FlowKey[ContestFlow]{variant: FlowVariantContest}
	ConcurrentKey     = FlowKey[ConcurrentFlow]{variant: FlowVariantConcurrent}
	SwitchKey         = FlowKey[SwitchFlow]{variant: FlowVariantSwitch}
	FilterKey         = FlowKey[FilterFlow]{variant: FlowVariantFilter}
	HandleKey         = FlowKey[HandleFlow]{variant: FlowVariantHandle}
	ScaffoldKey       = FlowKey[ScaffoldFlow]{variant: FlowVariantScaffold}
	BackoffKey        = FlowKey[BackoffFlow]{variant: FlowVariantBackoff}
	RetryKey          = FlowKey[RetryFlow]{variant: FlowVariantRetry}
	TimeoutKey        = FlowKey[TimeoutFlow]{variant: FlowVariantTimeout}
	RateLimiterKey    = FlowKey[RateLimiterFlow]{variant: FlowVariantRateLimiter}
	CircuitBreakerKey = FlowKey[CircuitBreakerFlow]{variant: FlowVariantCircuitBreaker}
	WorkerpoolKey     = FlowKey[WorkerpoolFlow]{variant: FlowVariantWorkerpool}
	PipelineKey       = FlowKey[PipelineFlow]{variant: FlowVariantPipeline}
)

// -----------------------------------------------------------------------------
// Connector Flow Types
// -----------------------------------------------------------------------------

// SequenceFlow represents an ordered sequence of processing steps.
// Each step is executed in order, with the output of each step
// becoming the input of the next.
type SequenceFlow struct {
	Steps []Node `json:"steps"`
}

// Variant implements Flow.
func (SequenceFlow) Variant() FlowVariant { return FlowVariantSequence }

// FallbackFlow represents primary/backup processing.
// The primary is tried first; if it fails, backups are tried in order.
type FallbackFlow struct {
	Primary Node   `json:"primary"`
	Backups []Node `json:"backups"`
}

// Variant implements Flow.
func (FallbackFlow) Variant() FlowVariant { return FlowVariantFallback }

// RaceFlow represents parallel execution where first success wins.
// All competitors start simultaneously; first to succeed cancels others.
type RaceFlow struct {
	Competitors []Node `json:"competitors"`
}

// Variant implements Flow.
func (RaceFlow) Variant() FlowVariant { return FlowVariantRace }

// ContestFlow represents parallel execution with result selection.
// All competitors complete; a selector chooses the best result.
type ContestFlow struct {
	Competitors []Node `json:"competitors"`
}

// Variant implements Flow.
func (ContestFlow) Variant() FlowVariant { return FlowVariantContest }

// ConcurrentFlow represents parallel execution of independent tasks.
// All tasks run simultaneously; combined into single output.
type ConcurrentFlow struct {
	Tasks []Node `json:"tasks"`
}

// Variant implements Flow.
func (ConcurrentFlow) Variant() FlowVariant { return FlowVariantConcurrent }

// SwitchFlow represents conditional routing to different processors.
// The condition determines which route key to use.
type SwitchFlow struct {
	Routes map[string]Node `json:"routes"`
}

// Variant implements Flow.
func (SwitchFlow) Variant() FlowVariant { return FlowVariantSwitch }

// FilterFlow represents conditional processing.
// Items matching the predicate are processed; others pass through.
type FilterFlow struct {
	Processor Node `json:"processor"`
}

// Variant implements Flow.
func (FilterFlow) Variant() FlowVariant { return FlowVariantFilter }

// HandleFlow represents error observation and handling.
// The processor is wrapped; errors flow to the error handler.
type HandleFlow struct {
	Processor    Node `json:"processor"`
	ErrorHandler Node `json:"error_handler"`
}

// Variant implements Flow.
func (HandleFlow) Variant() FlowVariant { return FlowVariantHandle }

// ScaffoldFlow represents fire-and-forget parallel execution.
// Processors run asynchronously without blocking the main flow.
type ScaffoldFlow struct {
	Processors []Node `json:"processors"`
}

// Variant implements Flow.
func (ScaffoldFlow) Variant() FlowVariant { return FlowVariantScaffold }

// BackoffFlow represents processing with exponential backoff retry.
type BackoffFlow struct {
	Processor Node `json:"processor"`
}

// Variant implements Flow.
func (BackoffFlow) Variant() FlowVariant { return FlowVariantBackoff }

// RetryFlow represents processing with simple retry logic.
type RetryFlow struct {
	Processor Node `json:"processor"`
}

// Variant implements Flow.
func (RetryFlow) Variant() FlowVariant { return FlowVariantRetry }

// TimeoutFlow represents processing with a timeout constraint.
type TimeoutFlow struct {
	Processor Node `json:"processor"`
}

// Variant implements Flow.
func (TimeoutFlow) Variant() FlowVariant { return FlowVariantTimeout }

// RateLimiterFlow represents processing with rate limiting.
type RateLimiterFlow struct {
	Processor Node `json:"processor"`
}

// Variant implements Flow.
func (RateLimiterFlow) Variant() FlowVariant { return FlowVariantRateLimiter }

// CircuitBreakerFlow represents processing with circuit breaker protection.
type CircuitBreakerFlow struct {
	Processor Node `json:"processor"`
}

// Variant implements Flow.
func (CircuitBreakerFlow) Variant() FlowVariant { return FlowVariantCircuitBreaker }

// WorkerpoolFlow represents processing distributed across a worker pool.
type WorkerpoolFlow struct {
	Processors []Node `json:"processors"`
}

// Variant implements Flow.
func (WorkerpoolFlow) Variant() FlowVariant { return FlowVariantWorkerpool }

// PipelineFlow represents a semantic execution context wrapper.
// It wraps a root chainable with execution tracking metadata.
type PipelineFlow struct {
	Root Node `json:"root"`
}

// Variant implements Flow.
func (PipelineFlow) Variant() FlowVariant { return FlowVariantPipeline }

// -----------------------------------------------------------------------------
// Node
// -----------------------------------------------------------------------------

// Node represents a node in the pipeline schema tree.
// It provides a serializable representation of the pipeline structure
// for visualization, debugging, and tooling.
//
// The schema can be generated at build time via the Schema() method
// on any Chainable, providing a complete picture of the pipeline
// structure without executing it.
//
// For connector nodes (Sequence, Fallback, etc.), the Flow field
// contains semantic child relationships. For processor nodes (Apply,
// Transform, etc.), Flow is nil since they are leaf nodes.
//
// Example:
//
//	pipeline := pipz.NewSequence(PipelineID,
//	    pipz.Apply(ValidateID, validate),
//	    pipz.NewFallback(FallbackID,
//	        pipz.Apply(PrimaryID, primary),
//	        pipz.Apply(BackupID, backup),
//	    ),
//	)
//
//	schema := pipeline.Schema()
//	jsonBytes, _ := json.MarshalIndent(schema, "", "  ")
//	fmt.Println(string(jsonBytes))
type Node struct {
	Identity Identity       `json:"-"`
	Type     string         `json:"type"`
	Flow     Flow           `json:"flow,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// nodeJSON is the JSON representation of a Node.
// It flattens Identity into separate fields for cleaner serialization.
type nodeJSON struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Type        string         `json:"type"`
	Flow        Flow           `json:"flow,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// MarshalJSON implements json.Marshaler.
// It flattens the Identity into separate id, name, and description fields.
func (n Node) MarshalJSON() ([]byte, error) {
	return json.Marshal(nodeJSON{
		ID:          n.Identity.ID().String(),
		Name:        n.Identity.Name(),
		Description: n.Identity.Description(),
		Type:        n.Type,
		Flow:        n.Flow,
		Metadata:    n.Metadata,
	})
}

// UnmarshalJSON implements json.Unmarshaler.
// Note: The Identity UUID will be regenerated on unmarshal since
// the original UUID cannot be reconstructed from the JSON.
// Note: Flow unmarshaling is not supported - schemas are built from pipelines, not JSON.
func (n *Node) UnmarshalJSON(data []byte) error {
	var j nodeJSON
	if err := json.Unmarshal(data, &j); err != nil {
		return err
	}

	n.Identity = NewIdentity(j.Name, j.Description)
	n.Type = j.Type
	n.Metadata = j.Metadata
	return nil
}

// -----------------------------------------------------------------------------
// Schema
// -----------------------------------------------------------------------------

// Schema represents a complete pipeline schema.
// It wraps the root Node and provides utilities for traversal and serialization.
type Schema struct {
	Root Node `json:"root"`
}

// NewSchema creates a Schema from a pipeline's root node.
func NewSchema(root Node) Schema {
	return Schema{Root: root}
}

// Walk traverses the schema tree, calling fn for each node.
// Traversal is depth-first, pre-order.
func (s Schema) Walk(fn func(Node)) {
	walkNode(s.Root, fn)
}

func walkNode(node Node, fn func(Node)) {
	fn(node)

	// Walk based on Flow type for semantic traversal
	if node.Flow != nil {
		switch f := node.Flow.(type) {
		case SequenceFlow:
			for _, step := range f.Steps {
				walkNode(step, fn)
			}
		case FallbackFlow:
			walkNode(f.Primary, fn)
			for _, backup := range f.Backups {
				walkNode(backup, fn)
			}
		case RaceFlow:
			for _, comp := range f.Competitors {
				walkNode(comp, fn)
			}
		case ContestFlow:
			for _, comp := range f.Competitors {
				walkNode(comp, fn)
			}
		case ConcurrentFlow:
			for _, task := range f.Tasks {
				walkNode(task, fn)
			}
		case SwitchFlow:
			for _, route := range f.Routes {
				walkNode(route, fn)
			}
		case FilterFlow:
			walkNode(f.Processor, fn)
		case HandleFlow:
			walkNode(f.Processor, fn)
			walkNode(f.ErrorHandler, fn)
		case ScaffoldFlow:
			for _, proc := range f.Processors {
				walkNode(proc, fn)
			}
		case BackoffFlow:
			walkNode(f.Processor, fn)
		case RetryFlow:
			walkNode(f.Processor, fn)
		case TimeoutFlow:
			walkNode(f.Processor, fn)
		case RateLimiterFlow:
			walkNode(f.Processor, fn)
		case CircuitBreakerFlow:
			walkNode(f.Processor, fn)
		case WorkerpoolFlow:
			for _, proc := range f.Processors {
				walkNode(proc, fn)
			}
		case PipelineFlow:
			walkNode(f.Root, fn)
		}
	}
}

// Find returns the first node matching the predicate, or nil if not found.
func (s Schema) Find(predicate func(Node) bool) *Node {
	var result *Node
	s.Walk(func(node Node) {
		if result == nil && predicate(node) {
			result = &node
		}
	})
	return result
}

// FindByName returns the first node with the given name, or nil if not found.
func (s Schema) FindByName(name string) *Node {
	return s.Find(func(n Node) bool {
		return n.Identity.Name() == name
	})
}

// FindByType returns all nodes of the given type.
func (s Schema) FindByType(nodeType string) []Node {
	var results []Node
	s.Walk(func(node Node) {
		if node.Type == nodeType {
			results = append(results, node)
		}
	})
	return results
}

// Count returns the total number of nodes in the schema.
func (s Schema) Count() int {
	count := 0
	s.Walk(func(_ Node) {
		count++
	})
	return count
}
