package pipz

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestNewPipeline(t *testing.T) {
	pipelineID := NewIdentity("order-processing", "Main order flow")
	processorID := NewIdentity("validate", "Validates input")
	processor := Transform(processorID, func(_ context.Context, s string) string {
		return s
	})

	pipeline := NewPipeline(pipelineID, processor)

	if pipeline == nil {
		t.Fatal("NewPipeline returned nil")
	}

	if pipeline.Identity().Name() != "order-processing" {
		t.Errorf("Identity().Name() = %q, want %q", pipeline.Identity().Name(), "order-processing")
	}
	if pipeline.Identity().Description() != "Main order flow" {
		t.Errorf("Identity().Description() = %q, want %q", pipeline.Identity().Description(), "Main order flow")
	}
}

func TestPipeline_Process_InjectsContext(t *testing.T) {
	pipelineID := NewIdentity("test-pipeline", "")

	var capturedExecID uuid.UUID
	var capturedPipeID uuid.UUID
	var execOK, pipeOK bool

	processorID := NewIdentity("capture", "")
	processor := Transform(processorID, func(ctx context.Context, s string) string {
		capturedExecID, execOK = ExecutionIDFromContext(ctx)
		capturedPipeID, pipeOK = PipelineIDFromContext(ctx)
		return s
	})

	pipeline := NewPipeline(pipelineID, processor)
	_, err := pipeline.Process(context.Background(), "test")

	if err != nil {
		t.Fatalf("Process returned error: %v", err)
	}
	if !execOK {
		t.Error("ExecutionIDFromContext should return ok=true")
	}
	if capturedExecID == uuid.Nil {
		t.Error("Execution ID should not be nil UUID")
	}
	if !pipeOK {
		t.Error("PipelineIDFromContext should return ok=true")
	}
	if capturedPipeID != pipelineID.ID() {
		t.Errorf("Pipeline ID = %v, want %v", capturedPipeID, pipelineID.ID())
	}
}

func TestPipeline_Process_UniqueExecutionIDs(t *testing.T) {
	pipelineID := NewIdentity("test-pipeline", "")

	var capturedIDs []uuid.UUID

	processorID := NewIdentity("capture", "")
	processor := Transform(processorID, func(ctx context.Context, s string) string {
		id, _ := ExecutionIDFromContext(ctx)
		capturedIDs = append(capturedIDs, id)
		return s
	})

	pipeline := NewPipeline(pipelineID, processor)

	for i := 0; i < 3; i++ {
		_, _ = pipeline.Process(context.Background(), "test")
	}

	if len(capturedIDs) != 3 {
		t.Fatalf("Expected 3 captured IDs, got %d", len(capturedIDs))
	}

	seen := make(map[uuid.UUID]bool)
	for i, id := range capturedIDs {
		if seen[id] {
			t.Errorf("Duplicate execution ID at index %d", i)
		}
		seen[id] = true
	}
}

func TestPipeline_Process_StablePipelineID(t *testing.T) {
	pipelineID := NewIdentity("test-pipeline", "")

	var capturedIDs []uuid.UUID

	processorID := NewIdentity("capture", "")
	processor := Transform(processorID, func(ctx context.Context, s string) string {
		id, _ := PipelineIDFromContext(ctx)
		capturedIDs = append(capturedIDs, id)
		return s
	})

	pipeline := NewPipeline(pipelineID, processor)

	for i := 0; i < 3; i++ {
		_, _ = pipeline.Process(context.Background(), "test")
	}

	for i, id := range capturedIDs {
		if id != pipelineID.ID() {
			t.Errorf("Pipeline ID at index %d = %v, want %v", i, id, pipelineID.ID())
		}
	}
}

func TestPipeline_Process_DelegatesCorrectly(t *testing.T) {
	pipelineID := NewIdentity("test-pipeline", "")
	processorID := NewIdentity("double", "")
	processor := Transform(processorID, func(_ context.Context, n int) int {
		return n * 2
	})

	pipeline := NewPipeline(pipelineID, processor)
	result, err := pipeline.Process(context.Background(), 21)

	if err != nil {
		t.Fatalf("Process returned error: %v", err)
	}
	if result != 42 {
		t.Errorf("result = %d, want 42", result)
	}
}

func TestPipeline_Process_PropagatesErrors(t *testing.T) {
	pipelineID := NewIdentity("test-pipeline", "")
	processorID := NewIdentity("fail", "")
	expectedErr := errors.New("processing failed")
	processor := Apply(processorID, func(_ context.Context, _ string) (string, error) {
		return "", expectedErr
	})

	pipeline := NewPipeline(pipelineID, processor)
	_, err := pipeline.Process(context.Background(), "test")

	if err == nil {
		t.Fatal("Expected error, got nil")
	}
}

func TestPipeline_Identity(t *testing.T) {
	pipelineID := NewIdentity("my-pipeline", "My description")
	processorID := NewIdentity("proc", "")
	processor := Transform(processorID, func(_ context.Context, s string) string { return s })

	pipeline := NewPipeline(pipelineID, processor)

	identity := pipeline.Identity()
	if identity.Name() != "my-pipeline" {
		t.Errorf("Identity().Name() = %q, want %q", identity.Name(), "my-pipeline")
	}
	if identity.Description() != "My description" {
		t.Errorf("Identity().Description() = %q, want %q", identity.Description(), "My description")
	}
	if identity.ID() != pipelineID.ID() {
		t.Error("Identity ID should match the original")
	}
}

func TestPipeline_Schema(t *testing.T) {
	pipelineID := NewIdentity("my-pipeline", "")
	processorID := NewIdentity("proc", "")
	processor := Transform(processorID, func(_ context.Context, s string) string { return s })

	pipeline := NewPipeline(pipelineID, processor)

	schema := pipeline.Schema()

	if schema.Type != "pipeline" {
		t.Errorf("Schema.Type = %q, want %q", schema.Type, "pipeline")
	}
	if schema.Identity.Name() != "my-pipeline" {
		t.Errorf("Schema.Identity.Name() = %q, want %q", schema.Identity.Name(), "my-pipeline")
	}

	flow, ok := PipelineKey.From(schema)
	if !ok {
		t.Fatal("Schema.Flow should be PipelineFlow")
	}
	if flow.Root.Identity.Name() != "proc" {
		t.Errorf("Flow.Root identity = %q, want %q", flow.Root.Identity.Name(), "proc")
	}
}

func TestPipeline_Close(t *testing.T) {
	pipelineID := NewIdentity("test-pipeline", "")
	seqID := NewIdentity("seq", "")

	// Use a Sequence which has a Close method that does something
	seq := NewSequence[string](seqID)
	pipeline := NewPipeline(pipelineID, seq)

	err := pipeline.Close()
	if err != nil {
		t.Errorf("Close returned error: %v", err)
	}
}

func TestExecutionIDFromContext(t *testing.T) {
	tests := []struct {
		name    string
		ctx     context.Context
		wantOK  bool
		wantNil bool
	}{
		{
			name:    "nil context",
			ctx:     nil,
			wantOK:  false,
			wantNil: true,
		},
		{
			name:    "empty context",
			ctx:     context.Background(),
			wantOK:  false,
			wantNil: true,
		},
		{
			name: "context with execution ID",
			ctx: func() context.Context {
				pipelineID := NewIdentity("test", "")
				var captured context.Context
				captureProc := Transform(NewIdentity("cap", ""), func(ctx context.Context, s string) string {
					captured = ctx
					return s
				})
				pipeline := NewPipeline(pipelineID, captureProc)
				_, _ = pipeline.Process(context.Background(), "test")
				return captured
			}(),
			wantOK:  true,
			wantNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, ok := ExecutionIDFromContext(tt.ctx)

			if ok != tt.wantOK {
				t.Errorf("ok = %v, want %v", ok, tt.wantOK)
			}
			if tt.wantNil && id != uuid.Nil {
				t.Errorf("id = %v, want uuid.Nil", id)
			}
			if !tt.wantNil && id == uuid.Nil {
				t.Error("id should not be uuid.Nil")
			}
		})
	}
}

func TestPipelineIDFromContext(t *testing.T) {
	tests := []struct {
		name    string
		ctx     context.Context
		wantOK  bool
		wantNil bool
	}{
		{
			name:    "nil context",
			ctx:     nil,
			wantOK:  false,
			wantNil: true,
		},
		{
			name:    "empty context",
			ctx:     context.Background(),
			wantOK:  false,
			wantNil: true,
		},
		{
			name: "context with pipeline ID",
			ctx: func() context.Context {
				pipelineID := NewIdentity("test", "")
				var captured context.Context
				captureProc := Transform(NewIdentity("cap", ""), func(ctx context.Context, s string) string {
					captured = ctx
					return s
				})
				p := NewPipeline(pipelineID, captureProc)
				_, _ = p.Process(context.Background(), "test")
				return captured
			}(),
			wantOK:  true,
			wantNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id, ok := PipelineIDFromContext(tt.ctx)

			if ok != tt.wantOK {
				t.Errorf("ok = %v, want %v", ok, tt.wantOK)
			}
			if tt.wantNil && id != uuid.Nil {
				t.Errorf("id = %v, want uuid.Nil", id)
			}
			if !tt.wantNil && id == uuid.Nil {
				t.Error("id should not be uuid.Nil")
			}
		})
	}
}

func TestExecutionIDFromContext_WrongType(t *testing.T) {
	ctx := context.WithValue(context.Background(), executionIDKey{}, "not-a-uuid")

	id, ok := ExecutionIDFromContext(ctx)

	if ok {
		t.Error("Should return ok=false for wrong type")
	}
	if id != uuid.Nil {
		t.Errorf("Should return uuid.Nil for wrong type, got %v", id)
	}
}

func TestPipelineIDFromContext_WrongType(t *testing.T) {
	ctx := context.WithValue(context.Background(), pipelineIDKey{}, "not-a-uuid")

	id, ok := PipelineIDFromContext(ctx)

	if ok {
		t.Error("Should return ok=false for wrong type")
	}
	if id != uuid.Nil {
		t.Errorf("Should return uuid.Nil for wrong type, got %v", id)
	}
}

func TestPipeline_NestedChainables(t *testing.T) {
	// Test that nested chainables see the same execution context
	pipelineID := NewIdentity("outer-pipeline", "")

	var capturedExecIDs []uuid.UUID
	var capturedPipeIDs []uuid.UUID

	proc1ID := NewIdentity("proc1", "")
	proc1 := Transform(proc1ID, func(ctx context.Context, s string) string {
		execID, _ := ExecutionIDFromContext(ctx)
		pipeID, _ := PipelineIDFromContext(ctx)
		capturedExecIDs = append(capturedExecIDs, execID)
		capturedPipeIDs = append(capturedPipeIDs, pipeID)
		return s + "-1"
	})

	proc2ID := NewIdentity("proc2", "")
	proc2 := Transform(proc2ID, func(ctx context.Context, s string) string {
		execID, _ := ExecutionIDFromContext(ctx)
		pipeID, _ := PipelineIDFromContext(ctx)
		capturedExecIDs = append(capturedExecIDs, execID)
		capturedPipeIDs = append(capturedPipeIDs, pipeID)
		return s + "-2"
	})

	seqID := NewIdentity("seq", "")
	seq := NewSequence(seqID, proc1, proc2)
	pipeline := NewPipeline(pipelineID, seq)

	result, err := pipeline.Process(context.Background(), "test")

	if err != nil {
		t.Fatalf("Process returned error: %v", err)
	}
	if result != "test-1-2" {
		t.Errorf("result = %q, want %q", result, "test-1-2")
	}

	// Both processors should see the same execution ID
	if len(capturedExecIDs) != 2 {
		t.Fatalf("Expected 2 captured exec IDs, got %d", len(capturedExecIDs))
	}
	if capturedExecIDs[0] != capturedExecIDs[1] {
		t.Error("Both processors should see the same execution ID")
	}

	// Both processors should see the pipeline ID (outer pipeline, not sequence)
	for i, id := range capturedPipeIDs {
		if id != pipelineID.ID() {
			t.Errorf("Processor %d saw pipeline ID %v, want %v", i, id, pipelineID.ID())
		}
	}
}

func TestPipeline_PreservesExistingContextValues(t *testing.T) {
	type customKey struct{}
	pipelineID := NewIdentity("test-pipeline", "")

	var capturedValue any

	processorID := NewIdentity("capture", "")
	processor := Transform(processorID, func(ctx context.Context, s string) string {
		capturedValue = ctx.Value(customKey{})
		return s
	})

	pipeline := NewPipeline(pipelineID, processor)
	ctx := context.WithValue(context.Background(), customKey{}, "custom-value")
	_, _ = pipeline.Process(ctx, "test")

	if capturedValue != "custom-value" {
		t.Errorf("Original context value lost, got %v", capturedValue)
	}
}
