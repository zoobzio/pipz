package pipz

import (
	"context"

	"github.com/google/uuid"
)

// Pipeline wraps a Chainable to define a semantic execution context.
// It provides correlation between runtime signals and pipeline identity
// by injecting execution and pipeline IDs into the context.
//
// Each Process() call generates a unique execution ID, while the
// pipeline ID remains stable (derived from the Pipeline's identity).
// Nested connectors can extract these IDs to include in signals,
// enabling distributed tracing and observability.
//
// Example:
//
//	// Define a semantic pipeline
//	orderPipeline := pipz.NewPipeline(
//	    pipz.NewIdentity("order-processing", "Main order flow"),
//	    pipz.NewSequence(internalID, validate, enrich, save),
//	)
//
//	// All signals from nested connectors get correlation IDs
//	result, err := orderPipeline.Process(ctx, order)
//
//	// In signal handlers, extract IDs for correlation
//	if execID, ok := pipz.ExecutionIDFromContext(ctx); ok {
//	    log.Printf("Execution %s completed", execID)
//	}
type Pipeline[T any] struct {
	identity Identity
	root     Chainable[T]
}

// NewPipeline creates a Pipeline that wraps a Chainable with execution context.
// The identity defines the semantic pipeline name for correlation purposes,
// separate from the identities of the components within.
func NewPipeline[T any](identity Identity, root Chainable[T]) *Pipeline[T] {
	return &Pipeline[T]{
		identity: identity,
		root:     root,
	}
}

// Process executes the wrapped Chainable with execution context.
// Each call generates a unique execution ID and injects both the
// execution ID and pipeline ID into the context before delegating
// to the root Chainable.
func (p *Pipeline[T]) Process(ctx context.Context, data T) (T, error) {
	ctx = context.WithValue(ctx, executionIDKey{}, uuid.New())
	ctx = context.WithValue(ctx, pipelineIDKey{}, p.identity.ID())
	return p.root.Process(ctx, data)
}

// Identity returns the pipeline's identity.
func (p *Pipeline[T]) Identity() Identity {
	return p.identity
}

// Schema returns a Node representing this pipeline in the schema.
// The pipeline wraps the root's schema as a child.
func (p *Pipeline[T]) Schema() Node {
	return Node{
		Identity: p.identity,
		Type:     "pipeline",
		Flow:     PipelineFlow{Root: p.root.Schema()},
	}
}

// Close gracefully shuts down the wrapped Chainable.
func (p *Pipeline[T]) Close() error {
	return p.root.Close()
}

// Context key types (unexported to prevent collisions).
type executionIDKey struct{}
type pipelineIDKey struct{}

// ExecutionIDFromContext extracts the execution ID from context.
// Returns the execution UUID and true if present, or uuid.Nil and false otherwise.
func ExecutionIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	if ctx == nil {
		return uuid.Nil, false
	}
	id, ok := ctx.Value(executionIDKey{}).(uuid.UUID)
	return id, ok
}

// PipelineIDFromContext extracts the pipeline ID from context.
// Returns the pipeline UUID and true if present, or uuid.Nil and false otherwise.
func PipelineIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	if ctx == nil {
		return uuid.Nil, false
	}
	id, ok := ctx.Value(pipelineIDKey{}).(uuid.UUID)
	return id, ok
}
