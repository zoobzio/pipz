package pipz

import (
	"context"
	"encoding/json"
	"testing"
)

// -----------------------------------------------------------------------------
// Flow Variant Tests
// -----------------------------------------------------------------------------

func TestFlowVariants(t *testing.T) {
	tests := []struct {
		name    string
		flow    Flow
		variant FlowVariant
	}{
		{"SequenceFlow", SequenceFlow{}, FlowVariantSequence},
		{"FallbackFlow", FallbackFlow{}, FlowVariantFallback},
		{"RaceFlow", RaceFlow{}, FlowVariantRace},
		{"ContestFlow", ContestFlow{}, FlowVariantContest},
		{"ConcurrentFlow", ConcurrentFlow{}, FlowVariantConcurrent},
		{"SwitchFlow", SwitchFlow{}, FlowVariantSwitch},
		{"FilterFlow", FilterFlow{}, FlowVariantFilter},
		{"HandleFlow", HandleFlow{}, FlowVariantHandle},
		{"ScaffoldFlow", ScaffoldFlow{}, FlowVariantScaffold},
		{"BackoffFlow", BackoffFlow{}, FlowVariantBackoff},
		{"RetryFlow", RetryFlow{}, FlowVariantRetry},
		{"TimeoutFlow", TimeoutFlow{}, FlowVariantTimeout},
		{"RateLimiterFlow", RateLimiterFlow{}, FlowVariantRateLimiter},
		{"CircuitBreakerFlow", CircuitBreakerFlow{}, FlowVariantCircuitBreaker},
		{"WorkerpoolFlow", WorkerpoolFlow{}, FlowVariantWorkerpool},
		{"PipelineFlow", PipelineFlow{}, FlowVariantPipeline},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.flow.Variant(); got != tt.variant {
				t.Errorf("Variant() = %v, want %v", got, tt.variant)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// FlowKey Tests
// -----------------------------------------------------------------------------

func TestFlowKey_Variant(t *testing.T) {
	tests := []struct {
		name    string
		key     FlowVariant
		variant FlowVariant
	}{
		{"SequenceKey", SequenceKey.Variant(), FlowVariantSequence},
		{"FallbackKey", FallbackKey.Variant(), FlowVariantFallback},
		{"RaceKey", RaceKey.Variant(), FlowVariantRace},
		{"ContestKey", ContestKey.Variant(), FlowVariantContest},
		{"ConcurrentKey", ConcurrentKey.Variant(), FlowVariantConcurrent},
		{"SwitchKey", SwitchKey.Variant(), FlowVariantSwitch},
		{"FilterKey", FilterKey.Variant(), FlowVariantFilter},
		{"HandleKey", HandleKey.Variant(), FlowVariantHandle},
		{"ScaffoldKey", ScaffoldKey.Variant(), FlowVariantScaffold},
		{"BackoffKey", BackoffKey.Variant(), FlowVariantBackoff},
		{"RetryKey", RetryKey.Variant(), FlowVariantRetry},
		{"TimeoutKey", TimeoutKey.Variant(), FlowVariantTimeout},
		{"RateLimiterKey", RateLimiterKey.Variant(), FlowVariantRateLimiter},
		{"CircuitBreakerKey", CircuitBreakerKey.Variant(), FlowVariantCircuitBreaker},
		{"WorkerpoolKey", WorkerpoolKey.Variant(), FlowVariantWorkerpool},
		{"PipelineKey", PipelineKey.Variant(), FlowVariantPipeline},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.key != tt.variant {
				t.Errorf("Variant() = %v, want %v", tt.key, tt.variant)
			}
		})
	}
}

func TestFlowKey_From(t *testing.T) {
	seqFlow := SequenceFlow{Steps: []Node{{Type: "processor"}}}
	node := Node{
		Identity: NewIdentity("test", ""),
		Type:     "sequence",
		Flow:     seqFlow,
	}

	t.Run("matching type", func(t *testing.T) {
		flow, ok := SequenceKey.From(node)
		if !ok {
			t.Error("Expected ok=true for matching flow type")
		}
		if len(flow.Steps) != 1 {
			t.Errorf("Steps length = %d, want 1", len(flow.Steps))
		}
	})

	t.Run("non-matching type", func(t *testing.T) {
		_, ok := FallbackKey.From(node)
		if ok {
			t.Error("Expected ok=false for non-matching flow type")
		}
	})

	t.Run("nil flow", func(t *testing.T) {
		nodeNoFlow := Node{Type: "processor"}
		_, ok := SequenceKey.From(nodeNoFlow)
		if ok {
			t.Error("Expected ok=false for nil flow")
		}
	})
}

func TestFlowKey_From_AllTypes(t *testing.T) {
	// Test that each FlowKey can extract its corresponding flow type
	tests := []struct {
		name string
		node Node
		test func(Node) bool
	}{
		{
			name: "SequenceKey",
			node: Node{Flow: SequenceFlow{Steps: []Node{}}},
			test: func(n Node) bool { _, ok := SequenceKey.From(n); return ok },
		},
		{
			name: "FallbackKey",
			node: Node{Flow: FallbackFlow{}},
			test: func(n Node) bool { _, ok := FallbackKey.From(n); return ok },
		},
		{
			name: "RaceKey",
			node: Node{Flow: RaceFlow{}},
			test: func(n Node) bool { _, ok := RaceKey.From(n); return ok },
		},
		{
			name: "ContestKey",
			node: Node{Flow: ContestFlow{}},
			test: func(n Node) bool { _, ok := ContestKey.From(n); return ok },
		},
		{
			name: "ConcurrentKey",
			node: Node{Flow: ConcurrentFlow{}},
			test: func(n Node) bool { _, ok := ConcurrentKey.From(n); return ok },
		},
		{
			name: "SwitchKey",
			node: Node{Flow: SwitchFlow{}},
			test: func(n Node) bool { _, ok := SwitchKey.From(n); return ok },
		},
		{
			name: "FilterKey",
			node: Node{Flow: FilterFlow{}},
			test: func(n Node) bool { _, ok := FilterKey.From(n); return ok },
		},
		{
			name: "HandleKey",
			node: Node{Flow: HandleFlow{}},
			test: func(n Node) bool { _, ok := HandleKey.From(n); return ok },
		},
		{
			name: "ScaffoldKey",
			node: Node{Flow: ScaffoldFlow{}},
			test: func(n Node) bool { _, ok := ScaffoldKey.From(n); return ok },
		},
		{
			name: "BackoffKey",
			node: Node{Flow: BackoffFlow{}},
			test: func(n Node) bool { _, ok := BackoffKey.From(n); return ok },
		},
		{
			name: "RetryKey",
			node: Node{Flow: RetryFlow{}},
			test: func(n Node) bool { _, ok := RetryKey.From(n); return ok },
		},
		{
			name: "TimeoutKey",
			node: Node{Flow: TimeoutFlow{}},
			test: func(n Node) bool { _, ok := TimeoutKey.From(n); return ok },
		},
		{
			name: "RateLimiterKey",
			node: Node{Flow: RateLimiterFlow{}},
			test: func(n Node) bool { _, ok := RateLimiterKey.From(n); return ok },
		},
		{
			name: "CircuitBreakerKey",
			node: Node{Flow: CircuitBreakerFlow{}},
			test: func(n Node) bool { _, ok := CircuitBreakerKey.From(n); return ok },
		},
		{
			name: "WorkerpoolKey",
			node: Node{Flow: WorkerpoolFlow{}},
			test: func(n Node) bool { _, ok := WorkerpoolKey.From(n); return ok },
		},
		{
			name: "PipelineKey",
			node: Node{Flow: PipelineFlow{Root: Node{Identity: NewIdentity("root", "")}}},
			test: func(n Node) bool { _, ok := PipelineKey.From(n); return ok },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !tt.test(tt.node) {
				t.Errorf("%s.From() should return ok=true for matching flow", tt.name)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// Node JSON Tests
// -----------------------------------------------------------------------------

func TestNode_MarshalJSON(t *testing.T) {
	identity := NewIdentity("test-node", "A test node")
	node := Node{
		Identity: identity,
		Type:     "processor",
		Metadata: map[string]any{"key": "value"},
	}

	data, err := json.Marshal(node)
	if err != nil {
		t.Fatalf("MarshalJSON failed: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	// Check fields
	if result["name"] != "test-node" {
		t.Errorf("name = %v, want %v", result["name"], "test-node")
	}
	if result["description"] != "A test node" {
		t.Errorf("description = %v, want %v", result["description"], "A test node")
	}
	if result["type"] != "processor" {
		t.Errorf("type = %v, want %v", result["type"], "processor")
	}
	if result["id"] == nil || result["id"] == "" {
		t.Error("id should be present and non-empty")
	}
	if result["metadata"] == nil {
		t.Error("metadata should be present")
	}
}

func TestNode_MarshalJSON_WithFlow(t *testing.T) {
	node := Node{
		Identity: NewIdentity("sequence", ""),
		Type:     "sequence",
		Flow: SequenceFlow{
			Steps: []Node{
				{Identity: NewIdentity("step1", ""), Type: "processor"},
				{Identity: NewIdentity("step2", ""), Type: "processor"},
			},
		},
	}

	data, err := json.Marshal(node)
	if err != nil {
		t.Fatalf("MarshalJSON failed: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	if result["flow"] == nil {
		t.Error("flow should be present")
	}

	flow, ok := result["flow"].(map[string]any)
	if !ok {
		t.Fatal("flow should be a map")
	}

	steps, ok := flow["steps"].([]any)
	if !ok {
		t.Fatal("steps should be an array")
	}
	if len(steps) != 2 {
		t.Errorf("steps length = %d, want 2", len(steps))
	}
}

func TestNode_MarshalJSON_EmptyDescription(t *testing.T) {
	node := Node{
		Identity: NewIdentity("test", ""), // Empty description
		Type:     "processor",
	}

	data, err := json.Marshal(node)
	if err != nil {
		t.Fatalf("MarshalJSON failed: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	// Empty description is omitted due to omitempty tag
	if _, exists := result["description"]; exists {
		t.Error("description field should be omitted when empty")
	}

	// Other required fields should still be present
	if result["name"] != "test" {
		t.Errorf("name = %v, want %v", result["name"], "test")
	}
	if result["type"] != "processor" {
		t.Errorf("type = %v, want %v", result["type"], "processor")
	}
}

func TestNode_UnmarshalJSON(t *testing.T) {
	jsonData := `{
		"id": "some-uuid",
		"name": "test-node",
		"description": "A test description",
		"type": "processor",
		"metadata": {"key": "value"}
	}`

	var node Node
	if err := json.Unmarshal([]byte(jsonData), &node); err != nil {
		t.Fatalf("UnmarshalJSON failed: %v", err)
	}

	if node.Identity.Name() != "test-node" {
		t.Errorf("Identity.Name() = %v, want %v", node.Identity.Name(), "test-node")
	}
	if node.Identity.Description() != "A test description" {
		t.Errorf("Identity.Description() = %v, want %v", node.Identity.Description(), "A test description")
	}
	if node.Type != "processor" {
		t.Errorf("Type = %v, want %v", node.Type, "processor")
	}
	if node.Metadata["key"] != "value" {
		t.Errorf("Metadata[key] = %v, want %v", node.Metadata["key"], "value")
	}
	// Note: UUID is regenerated on unmarshal, so we just check it's not nil
	if node.Identity.ID().String() == "" {
		t.Error("Identity should have a valid UUID after unmarshal")
	}
}

func TestNode_UnmarshalJSON_Invalid(t *testing.T) {
	var node Node
	err := json.Unmarshal([]byte(`{invalid json`), &node)
	if err == nil {
		t.Error("Expected error for invalid JSON")
	}
}

func TestNode_UnmarshalJSON_MinimalFields(t *testing.T) {
	jsonData := `{"name": "minimal", "type": "processor"}`

	var node Node
	if err := json.Unmarshal([]byte(jsonData), &node); err != nil {
		t.Fatalf("UnmarshalJSON failed: %v", err)
	}

	if node.Identity.Name() != "minimal" {
		t.Errorf("Identity.Name() = %v, want %v", node.Identity.Name(), "minimal")
	}
	if node.Type != "processor" {
		t.Errorf("Type = %v, want %v", node.Type, "processor")
	}
}

// -----------------------------------------------------------------------------
// Schema Tests
// -----------------------------------------------------------------------------

func TestNewSchema(t *testing.T) {
	root := Node{
		Identity: NewIdentity("root", ""),
		Type:     "sequence",
	}

	schema := NewSchema(root)

	if schema.Root.Identity.Name() != "root" {
		t.Errorf("Root.Identity.Name() = %v, want %v", schema.Root.Identity.Name(), "root")
	}
}

func TestSchema_Walk(t *testing.T) {
	// Build a complex schema tree
	schema := NewSchema(Node{
		Identity: NewIdentity("root", ""),
		Type:     "sequence",
		Flow: SequenceFlow{
			Steps: []Node{
				{Identity: NewIdentity("step1", ""), Type: "processor"},
				{
					Identity: NewIdentity("fallback", ""),
					Type:     "fallback",
					Flow: FallbackFlow{
						Primary: Node{Identity: NewIdentity("primary", ""), Type: "processor"},
						Backups: []Node{
							{Identity: NewIdentity("backup1", ""), Type: "processor"},
						},
					},
				},
			},
		},
	})

	var visited []string
	schema.Walk(func(node Node) {
		visited = append(visited, node.Identity.Name())
	})

	expected := []string{"root", "step1", "fallback", "primary", "backup1"}
	if len(visited) != len(expected) {
		t.Errorf("Visited %d nodes, want %d", len(visited), len(expected))
	}
	for i, name := range expected {
		if i >= len(visited) {
			break
		}
		if visited[i] != name {
			t.Errorf("visited[%d] = %v, want %v", i, visited[i], name)
		}
	}
}

func TestSchema_Walk_AllFlowTypes(t *testing.T) {
	// Test that Walk handles all flow types correctly
	tests := []struct {
		name          string
		node          Node
		expectedCount int
	}{
		{
			name: "SequenceFlow",
			node: Node{
				Identity: NewIdentity("seq", ""),
				Flow: SequenceFlow{
					Steps: []Node{
						{Identity: NewIdentity("s1", "")},
						{Identity: NewIdentity("s2", "")},
					},
				},
			},
			expectedCount: 3,
		},
		{
			name: "FallbackFlow",
			node: Node{
				Identity: NewIdentity("fb", ""),
				Flow: FallbackFlow{
					Primary: Node{Identity: NewIdentity("p", "")},
					Backups: []Node{
						{Identity: NewIdentity("b1", "")},
						{Identity: NewIdentity("b2", "")},
					},
				},
			},
			expectedCount: 4,
		},
		{
			name: "RaceFlow",
			node: Node{
				Identity: NewIdentity("race", ""),
				Flow: RaceFlow{
					Competitors: []Node{
						{Identity: NewIdentity("c1", "")},
						{Identity: NewIdentity("c2", "")},
					},
				},
			},
			expectedCount: 3,
		},
		{
			name: "ContestFlow",
			node: Node{
				Identity: NewIdentity("contest", ""),
				Flow: ContestFlow{
					Competitors: []Node{
						{Identity: NewIdentity("c1", "")},
					},
				},
			},
			expectedCount: 2,
		},
		{
			name: "ConcurrentFlow",
			node: Node{
				Identity: NewIdentity("concurrent", ""),
				Flow: ConcurrentFlow{
					Tasks: []Node{
						{Identity: NewIdentity("t1", "")},
						{Identity: NewIdentity("t2", "")},
					},
				},
			},
			expectedCount: 3,
		},
		{
			name: "SwitchFlow",
			node: Node{
				Identity: NewIdentity("switch", ""),
				Flow: SwitchFlow{
					Routes: map[string]Node{
						"a": {Identity: NewIdentity("ra", "")},
						"b": {Identity: NewIdentity("rb", "")},
					},
				},
			},
			expectedCount: 3,
		},
		{
			name: "FilterFlow",
			node: Node{
				Identity: NewIdentity("filter", ""),
				Flow: FilterFlow{
					Processor: Node{Identity: NewIdentity("proc", "")},
				},
			},
			expectedCount: 2,
		},
		{
			name: "HandleFlow",
			node: Node{
				Identity: NewIdentity("handle", ""),
				Flow: HandleFlow{
					Processor:    Node{Identity: NewIdentity("proc", "")},
					ErrorHandler: Node{Identity: NewIdentity("err", "")},
				},
			},
			expectedCount: 3,
		},
		{
			name: "ScaffoldFlow",
			node: Node{
				Identity: NewIdentity("scaffold", ""),
				Flow: ScaffoldFlow{
					Processors: []Node{
						{Identity: NewIdentity("p1", "")},
					},
				},
			},
			expectedCount: 2,
		},
		{
			name: "BackoffFlow",
			node: Node{
				Identity: NewIdentity("backoff", ""),
				Flow: BackoffFlow{
					Processor: Node{Identity: NewIdentity("proc", "")},
				},
			},
			expectedCount: 2,
		},
		{
			name: "RetryFlow",
			node: Node{
				Identity: NewIdentity("retry", ""),
				Flow: RetryFlow{
					Processor: Node{Identity: NewIdentity("proc", "")},
				},
			},
			expectedCount: 2,
		},
		{
			name: "TimeoutFlow",
			node: Node{
				Identity: NewIdentity("timeout", ""),
				Flow: TimeoutFlow{
					Processor: Node{Identity: NewIdentity("proc", "")},
				},
			},
			expectedCount: 2,
		},
		{
			name: "RateLimiterFlow",
			node: Node{
				Identity: NewIdentity("rl", ""),
				Flow: RateLimiterFlow{
					Processor: Node{Identity: NewIdentity("proc", "")},
				},
			},
			expectedCount: 2,
		},
		{
			name: "CircuitBreakerFlow",
			node: Node{
				Identity: NewIdentity("cb", ""),
				Flow: CircuitBreakerFlow{
					Processor: Node{Identity: NewIdentity("proc", "")},
				},
			},
			expectedCount: 2,
		},
		{
			name: "WorkerpoolFlow",
			node: Node{
				Identity: NewIdentity("wp", ""),
				Flow: WorkerpoolFlow{
					Processors: []Node{
						{Identity: NewIdentity("p1", "")},
						{Identity: NewIdentity("p2", "")},
					},
				},
			},
			expectedCount: 3,
		},
		{
			name: "PipelineFlow",
			node: Node{
				Identity: NewIdentity("pipeline", ""),
				Flow: PipelineFlow{
					Root: Node{
						Identity: NewIdentity("seq", ""),
						Flow: SequenceFlow{
							Steps: []Node{
								{Identity: NewIdentity("s1", "")},
								{Identity: NewIdentity("s2", "")},
							},
						},
					},
				},
			},
			expectedCount: 4, // pipeline + seq + s1 + s2
		},
		{
			name: "NoFlow",
			node: Node{
				Identity: NewIdentity("leaf", ""),
				Flow:     nil,
			},
			expectedCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema := NewSchema(tt.node)
			count := 0
			schema.Walk(func(_ Node) {
				count++
			})
			if count != tt.expectedCount {
				t.Errorf("Walk visited %d nodes, want %d", count, tt.expectedCount)
			}
		})
	}
}

func TestSchema_Find(t *testing.T) {
	schema := NewSchema(Node{
		Identity: NewIdentity("root", ""),
		Type:     "sequence",
		Flow: SequenceFlow{
			Steps: []Node{
				{Identity: NewIdentity("target", ""), Type: "processor"},
				{Identity: NewIdentity("other", ""), Type: "effect"},
			},
		},
	})

	t.Run("found", func(t *testing.T) {
		result := schema.Find(func(n Node) bool {
			return n.Identity.Name() == "target"
		})
		if result == nil {
			t.Fatal("Expected to find node")
		}
		if result.Identity.Name() != "target" {
			t.Errorf("Found node name = %v, want %v", result.Identity.Name(), "target")
		}
	})

	t.Run("not found", func(t *testing.T) {
		result := schema.Find(func(n Node) bool {
			return n.Identity.Name() == "nonexistent"
		})
		if result != nil {
			t.Error("Expected nil for non-existent node")
		}
	})

	t.Run("first match returned", func(t *testing.T) {
		// Find first processor type
		result := schema.Find(func(n Node) bool {
			return n.Type == "processor" || n.Type == "effect"
		})
		if result == nil {
			t.Fatal("Expected to find node")
		}
		// Should find "target" first (depth-first, pre-order)
		if result.Identity.Name() != "target" {
			t.Errorf("Expected first match 'target', got %v", result.Identity.Name())
		}
	})
}

func TestSchema_FindByName(t *testing.T) {
	schema := NewSchema(Node{
		Identity: NewIdentity("root", ""),
		Flow: SequenceFlow{
			Steps: []Node{
				{Identity: NewIdentity("step1", "")},
				{Identity: NewIdentity("step2", "")},
			},
		},
	})

	t.Run("found", func(t *testing.T) {
		result := schema.FindByName("step2")
		if result == nil {
			t.Fatal("Expected to find node")
		}
		if result.Identity.Name() != "step2" {
			t.Errorf("Found node name = %v, want %v", result.Identity.Name(), "step2")
		}
	})

	t.Run("not found", func(t *testing.T) {
		result := schema.FindByName("step3")
		if result != nil {
			t.Error("Expected nil for non-existent name")
		}
	})
}

func TestSchema_FindByType(t *testing.T) {
	schema := NewSchema(Node{
		Identity: NewIdentity("root", ""),
		Type:     "sequence",
		Flow: SequenceFlow{
			Steps: []Node{
				{Identity: NewIdentity("p1", ""), Type: "processor"},
				{Identity: NewIdentity("e1", ""), Type: "effect"},
				{Identity: NewIdentity("p2", ""), Type: "processor"},
			},
		},
	})

	t.Run("multiple matches", func(t *testing.T) {
		results := schema.FindByType("processor")
		if len(results) != 2 {
			t.Errorf("Found %d processors, want 2", len(results))
		}
	})

	t.Run("single match", func(t *testing.T) {
		results := schema.FindByType("effect")
		if len(results) != 1 {
			t.Errorf("Found %d effects, want 1", len(results))
		}
	})

	t.Run("no matches", func(t *testing.T) {
		results := schema.FindByType("nonexistent")
		if len(results) != 0 {
			t.Errorf("Found %d nodes, want 0", len(results))
		}
	})

	t.Run("root type", func(t *testing.T) {
		results := schema.FindByType("sequence")
		if len(results) != 1 {
			t.Errorf("Found %d sequences, want 1", len(results))
		}
	})
}

func TestSchema_Count(t *testing.T) {
	tests := []struct {
		name     string
		schema   Schema
		expected int
	}{
		{
			name:     "single node",
			schema:   NewSchema(Node{Identity: NewIdentity("single", "")}),
			expected: 1,
		},
		{
			name: "sequence with steps",
			schema: NewSchema(Node{
				Identity: NewIdentity("seq", ""),
				Flow: SequenceFlow{
					Steps: []Node{
						{Identity: NewIdentity("s1", "")},
						{Identity: NewIdentity("s2", "")},
						{Identity: NewIdentity("s3", "")},
					},
				},
			}),
			expected: 4,
		},
		{
			name: "nested structure",
			schema: NewSchema(Node{
				Identity: NewIdentity("root", ""),
				Flow: SequenceFlow{
					Steps: []Node{
						{
							Identity: NewIdentity("nested", ""),
							Flow: SequenceFlow{
								Steps: []Node{
									{Identity: NewIdentity("deep", "")},
								},
							},
						},
					},
				},
			}),
			expected: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.schema.Count(); got != tt.expected {
				t.Errorf("Count() = %d, want %d", got, tt.expected)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// Integration Tests
// -----------------------------------------------------------------------------

func TestSchema_FromRealPipeline(t *testing.T) {
	// Create a realistic pipeline and extract its schema
	validateID := NewIdentity("validate", "Validates input")
	transformID := NewIdentity("transform", "Transforms data")
	pipelineID := NewIdentity("pipeline", "Main pipeline")

	validate := Apply(validateID, func(_ context.Context, s string) (string, error) {
		return s, nil
	})
	transform := Transform(transformID, func(_ context.Context, s string) string {
		return s
	})

	pipeline := NewSequence(pipelineID, validate, transform)
	schema := NewSchema(pipeline.Schema())

	// Verify schema structure
	if schema.Root.Identity.Name() != "pipeline" {
		t.Errorf("Root name = %v, want %v", schema.Root.Identity.Name(), "pipeline")
	}

	if schema.Count() != 3 {
		t.Errorf("Count = %d, want 3", schema.Count())
	}

	// Find specific nodes
	validateNode := schema.FindByName("validate")
	if validateNode == nil {
		t.Error("Should find validate node")
	}

	transformNode := schema.FindByName("transform")
	if transformNode == nil {
		t.Error("Should find transform node")
	}
}

func TestSchema_JSONRoundtrip(t *testing.T) {
	original := Node{
		Identity: NewIdentity("test", "A test node"),
		Type:     "processor",
		Metadata: map[string]any{
			"custom": "value",
			"count":  float64(42), // JSON numbers are float64
		},
	}

	// Marshal
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	// Unmarshal
	var restored Node
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	// Verify (note: UUID is regenerated, so we don't compare it)
	if restored.Identity.Name() != original.Identity.Name() {
		t.Errorf("Name mismatch: got %v, want %v", restored.Identity.Name(), original.Identity.Name())
	}
	if restored.Identity.Description() != original.Identity.Description() {
		t.Errorf("Description mismatch: got %v, want %v", restored.Identity.Description(), original.Identity.Description())
	}
	if restored.Type != original.Type {
		t.Errorf("Type mismatch: got %v, want %v", restored.Type, original.Type)
	}
	if restored.Metadata["custom"] != original.Metadata["custom"] {
		t.Errorf("Metadata[custom] mismatch: got %v, want %v", restored.Metadata["custom"], original.Metadata["custom"])
	}
}
