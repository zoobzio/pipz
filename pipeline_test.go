package pipz

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestNewPipeline(t *testing.T) {
	Pipeline := NewPipeline[string]()

	if Pipeline == nil {
		t.Fatal("NewPipeline should not return nil")
	}

	if Pipeline.Len() != 0 {
		t.Errorf("new Pipeline should be empty, got length %d", Pipeline.Len())
	}

	if !Pipeline.IsEmpty() {
		t.Error("new Pipeline should be empty")
	}
}

func TestPipelineRegister(t *testing.T) {
	t.Run("Register Single Processor", func(t *testing.T) {
		Pipeline := NewPipeline[string]()
		processor := Transform("upper", func(_ context.Context, s string) string {
			return strings.ToUpper(s)
		})

		Pipeline.Register(processor)

		if Pipeline.Len() != 1 {
			t.Errorf("expected 1 processor, got %d", Pipeline.Len())
		}
	})

	t.Run("Register Multiple Processors", func(t *testing.T) {
		Pipeline := NewPipeline[string]()

		Pipeline.Register(
			Transform("trim", func(_ context.Context, s string) string {
				return strings.TrimSpace(s)
			}),
			Transform("lower", func(_ context.Context, s string) string {
				return strings.ToLower(s)
			}),
			Effect("non_empty", func(_ context.Context, s string) error {
				if s == "" {
					return errors.New("empty string")
				}
				return nil
			}),
		)

		if Pipeline.Len() != 3 {
			t.Errorf("expected 3 processors, got %d", Pipeline.Len())
		}

		names := Pipeline.Names()
		expected := []string{"trim", "lower", "non_empty"}
		if !reflect.DeepEqual(names, expected) {
			t.Errorf("expected names %v, got %v", expected, names)
		}
	})
}

func TestPipelineProcess(t *testing.T) {
	t.Run("Empty Pipeline", func(t *testing.T) {
		Pipeline := NewPipeline[int]()
		result, err := Pipeline.Process(context.Background(), 42)

		if err != nil {
			t.Fatalf("empty pipeline should not error: %v", err)
		}
		if result != 42 {
			t.Errorf("empty pipeline should return input unchanged, got %d", result)
		}
	})

	t.Run("Single Processor Success", func(t *testing.T) {
		Pipeline := NewPipeline[int]()
		Pipeline.Register(Transform("double", func(_ context.Context, n int) int {
			return n * 2
		}))

		result, err := Pipeline.Process(context.Background(), 5)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != 10 {
			t.Errorf("expected 10, got %d", result)
		}
	})

	t.Run("Multiple Processors Chain", func(t *testing.T) {
		Pipeline := NewPipeline[int]()
		Pipeline.Register(
			Transform("double", func(_ context.Context, n int) int {
				return n * 2
			}),
			Transform("add_ten", func(_ context.Context, n int) int {
				return n + 10
			}),
			Transform("square", func(_ context.Context, n int) int {
				return n * n
			}),
		)

		// 5 -> 10 -> 20 -> 400
		result, err := Pipeline.Process(context.Background(), 5)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != 400 {
			t.Errorf("expected 400, got %d", result)
		}
	})

	t.Run("Processor Error Stops Pipeline", func(t *testing.T) {
		Pipeline := NewPipeline[string]()
		Pipeline.Register(
			Transform("step1", func(_ context.Context, s string) string {
				return s + "_1"
			}),
			Apply("step2", func(_ context.Context, _ string) (string, error) {
				return "", errors.New("step2 failed")
			}),
			Transform("step3", func(_ context.Context, s string) string {
				t.Error("step3 should not be called")
				return s + "_3"
			}),
		)

		result, err := Pipeline.Process(context.Background(), "test")

		if err == nil {
			t.Fatal("expected error from step2")
		}

		var pipelineErr *PipelineError[string]
		if !errors.As(err, &pipelineErr) {
			t.Fatal("error should be PipelineError")
		}

		if pipelineErr.ProcessorName != "step2" {
			t.Errorf("expected processor name 'step2', got %q", pipelineErr.ProcessorName)
		}
		if pipelineErr.StageIndex != 1 {
			t.Errorf("expected stage index 1, got %d", pipelineErr.StageIndex)
		}
		if !strings.Contains(pipelineErr.Error(), "step2 failed") {
			t.Errorf("error should contain original message: %v", pipelineErr.Error())
		}
		if result != "" {
			t.Errorf("failed pipeline should return zero value, got %q", result)
		}
	})

	t.Run("Context Cancellation", func(t *testing.T) {
		Pipeline := NewPipeline[string]()
		Pipeline.Register(
			Transform("step1", func(_ context.Context, s string) string {
				return s + "_1"
			}),
			Transform("step2", func(_ context.Context, s string) string {
				// This processor won't be reached
				t.Error("step2 should not be called after cancellation")
				return s + "_2"
			}),
		)

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		result, err := Pipeline.Process(ctx, "test")

		if err == nil {
			t.Fatal("expected cancellation error")
		}

		var pipelineErr *PipelineError[string]
		if !errors.As(err, &pipelineErr) {
			t.Fatal("error should be PipelineError")
		}

		if !pipelineErr.IsCanceled() {
			t.Error("error should indicate cancellation")
		}
		if pipelineErr.ProcessorName != "step1" {
			t.Errorf("cancellation should happen at step1")
		}
		if result != "" {
			t.Errorf("canceled pipeline should return zero value")
		}
	})

	t.Run("Context Timeout", func(t *testing.T) {
		Pipeline := NewPipeline[string]()
		Pipeline.Register(
			Transform("step1", func(_ context.Context, s string) string {
				return s + "_1"
			}),
			Transform("slow", func(_ context.Context, s string) string {
				// Simulate slow processor
				time.Sleep(100 * time.Millisecond)
				return s + "_processed"
			}),
		)

		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()

		// Let first processor complete, timeout before second
		time.Sleep(25 * time.Millisecond)

		start := time.Now()
		result, err := Pipeline.Process(ctx, "test")
		duration := time.Since(start)

		// The timeout will be detected before the slow processor starts
		// because we check context before each processor
		if err == nil {
			t.Fatal("expected timeout error")
		}

		var pipelineErr *PipelineError[string]
		if !errors.As(err, &pipelineErr) {
			t.Fatal("error should be PipelineError")
		}

		if !pipelineErr.IsTimeout() {
			t.Error("error should indicate timeout")
		}
		// Timeout can occur at either processor depending on timing
		if pipelineErr.ProcessorName != "step1" && pipelineErr.ProcessorName != "slow" {
			t.Errorf("timeout should occur at step1 or slow processor, got %s", pipelineErr.ProcessorName)
		}
		if result != "" {
			t.Errorf("timed out pipeline should return zero value")
		}
		// Should timeout after first processor but before slow one
		if duration > 50*time.Millisecond {
			t.Errorf("timeout detection took too long: %v", duration)
		}
	})

	t.Run("PipelineError Contains Input Data", func(t *testing.T) {
		type User struct {
			Name  string
			Email string
		}

		Pipeline := NewPipeline[User]()
		Pipeline.Register(
			Effect("check_email", func(_ context.Context, u User) error {
				if !strings.Contains(u.Email, "@") {
					return errors.New("invalid email")
				}
				return nil
			}),
		)

		user := User{Name: "Alice", Email: "invalid"}
		_, err := Pipeline.Process(context.Background(), user)

		var pipelineErr *PipelineError[User]
		if !errors.As(err, &pipelineErr) {
			t.Fatal("error should be PipelineError")
		}

		// InputData contains what the failed processor was trying to output
		// Since Effect returns zero value on error, InputData will be empty
		if pipelineErr.InputData.Name != "" || pipelineErr.InputData.Email != "" {
			t.Errorf("Effect returns zero value on error, got %+v", pipelineErr.InputData)
		}

		// The actual input user data is lost because Effect doesn't preserve it
		// This is a limitation of the current design
	})
}

func TestPipelineLink(t *testing.T) {
	Pipeline := NewPipeline[string]()
	chainable := Pipeline.Link()

	if chainable == nil {
		t.Fatal("Link should return non-nil Chainable")
	}

	// Verify it's not nil and is actually usable
	result, err := chainable.Process(context.Background(), "test")
	if err != nil {
		t.Errorf("Link should return functioning Chainable: %v", err)
	}
	if result != "test" {
		t.Errorf("empty pipeline should return input unchanged")
	}
}

func TestPipelineIntrospection(t *testing.T) {
	t.Run("Names", func(t *testing.T) {
		Pipeline := NewPipeline[string]()
		Pipeline.Register(
			Transform("step1", func(_ context.Context, s string) string { return s }),
			Apply("step2", func(_ context.Context, s string) (string, error) { return s, nil }),
			Effect("step3", func(_ context.Context, _ string) error { return nil }),
		)

		names := Pipeline.Names()
		expected := []string{"step1", "step2", "step3"}

		if !reflect.DeepEqual(names, expected) {
			t.Errorf("expected names %v, got %v", expected, names)
		}
	})

	t.Run("Find Existing Processor", func(t *testing.T) {
		Pipeline := NewPipeline[int]()
		doubleProc := Transform("double", func(_ context.Context, n int) int {
			return n * 2
		})
		tripleProc := Transform("triple", func(_ context.Context, n int) int {
			return n * 3
		})

		Pipeline.Register(doubleProc, tripleProc)

		found, err := Pipeline.Find("triple")
		if err != nil {
			t.Fatalf("expected to find processor: %v", err)
		}

		if found.Name != "triple" {
			t.Errorf("found wrong processor: %s", found.Name)
		}

		// Verify it's the actual processor
		result, err := found.Fn(context.Background(), 5)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result != 15 {
			t.Errorf("found processor doesn't work correctly")
		}
	})

	t.Run("Find Non-Existent Processor", func(t *testing.T) {
		Pipeline := NewPipeline[string]()
		Pipeline.Register(Transform("exists", func(_ context.Context, s string) string { return s }))

		_, err := Pipeline.Find("does_not_exist")
		if err == nil {
			t.Fatal("expected error when processor not found")
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("error should indicate processor not found: %v", err)
		}
	})

	t.Run("Len and IsEmpty", func(t *testing.T) {
		Pipeline := NewPipeline[string]()

		if !Pipeline.IsEmpty() {
			t.Error("new Pipeline should be empty")
		}
		if Pipeline.Len() != 0 {
			t.Error("new Pipeline should have length 0")
		}

		Pipeline.Register(Transform("test", func(_ context.Context, s string) string { return s }))

		if Pipeline.IsEmpty() {
			t.Error("Pipeline with processor should not be empty")
		}
		if Pipeline.Len() != 1 {
			t.Errorf("Pipeline should have length 1, got %d", Pipeline.Len())
		}
	})
}

func TestPipelineModification(t *testing.T) {
	t.Run("Clear", func(t *testing.T) {
		Pipeline := NewPipeline[string]()
		Pipeline.Register(
			Transform("p1", func(_ context.Context, s string) string { return s }),
			Transform("p2", func(_ context.Context, s string) string { return s }),
		)

		if Pipeline.Len() != 2 {
			t.Errorf("expected 2 processors before clear")
		}

		Pipeline.Clear()

		if Pipeline.Len() != 0 {
			t.Errorf("expected 0 processors after clear, got %d", Pipeline.Len())
		}
		if !Pipeline.IsEmpty() {
			t.Error("Pipeline should be empty after clear")
		}
	})

	t.Run("PushHead", func(t *testing.T) {
		Pipeline := NewPipeline[string]()
		Pipeline.Register(Transform("middle", func(_ context.Context, s string) string {
			return s + "_middle"
		}))

		Pipeline.PushHead(Transform("first", func(_ context.Context, s string) string {
			return s + "_first"
		}))

		names := Pipeline.Names()
		if names[0] != "first" || names[1] != "middle" {
			t.Errorf("PushHead should add to beginning: %v", names)
		}

		// Verify execution order
		result, err := Pipeline.Process(context.Background(), "test")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result != "test_first_middle" {
			t.Errorf("incorrect execution order: %s", result)
		}
	})

	t.Run("PushTail", func(t *testing.T) {
		Pipeline := NewPipeline[string]()
		Pipeline.Register(Transform("first", func(_ context.Context, s string) string {
			return s + "_first"
		}))

		Pipeline.PushTail(Transform("last", func(_ context.Context, s string) string {
			return s + "_last"
		}))

		names := Pipeline.Names()
		if names[0] != "first" || names[1] != "last" {
			t.Errorf("PushTail should add to end: %v", names)
		}
	})

	t.Run("PopHead", func(t *testing.T) {
		Pipeline := NewPipeline[string]()
		Pipeline.Register(
			Transform("first", func(_ context.Context, s string) string { return s }),
			Transform("second", func(_ context.Context, s string) string { return s }),
		)

		popped, err := Pipeline.PopHead()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if popped.Name != "first" {
			t.Errorf("PopHead should return first processor, got %s", popped.Name)
		}

		if Pipeline.Len() != 1 {
			t.Errorf("expected 1 processor after PopHead, got %d", Pipeline.Len())
		}

		names := Pipeline.Names()
		if names[0] != "second" {
			t.Errorf("wrong processor remains: %v", names)
		}
	})

	t.Run("PopHead Empty", func(t *testing.T) {
		Pipeline := NewPipeline[string]()

		_, err := Pipeline.PopHead()
		if !errors.Is(err, ErrEmptyPipeline) {
			t.Errorf("PopHead on empty pipeline should return ErrEmptyPipeline, got %v", err)
		}
	})

	t.Run("PopTail", func(t *testing.T) {
		Pipeline := NewPipeline[string]()
		Pipeline.Register(
			Transform("first", func(_ context.Context, s string) string { return s }),
			Transform("second", func(_ context.Context, s string) string { return s }),
		)

		popped, err := Pipeline.PopTail()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if popped.Name != "second" {
			t.Errorf("PopTail should return last processor, got %s", popped.Name)
		}

		if Pipeline.Len() != 1 {
			t.Errorf("expected 1 processor after PopTail, got %d", Pipeline.Len())
		}
	})

	t.Run("PopTail Empty", func(t *testing.T) {
		Pipeline := NewPipeline[string]()

		_, err := Pipeline.PopTail()
		if !errors.Is(err, ErrEmptyPipeline) {
			t.Errorf("PopTail on empty pipeline should return ErrEmptyPipeline, got %v", err)
		}
	})

	t.Run("InsertAt", func(t *testing.T) {
		Pipeline := NewPipeline[string]()
		Pipeline.Register(
			Transform("first", func(_ context.Context, s string) string { return s }),
			Transform("third", func(_ context.Context, s string) string { return s }),
		)

		err := Pipeline.InsertAt(1, Transform("second", func(_ context.Context, s string) string { return s }))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		names := Pipeline.Names()
		expected := []string{"first", "second", "third"}
		if !reflect.DeepEqual(names, expected) {
			t.Errorf("InsertAt failed: expected %v, got %v", expected, names)
		}
	})

	t.Run("RemoveAt", func(t *testing.T) {
		Pipeline := NewPipeline[string]()
		Pipeline.Register(
			Transform("first", func(_ context.Context, s string) string { return s }),
			Transform("remove_me", func(_ context.Context, s string) string { return s }),
			Transform("third", func(_ context.Context, s string) string { return s }),
		)

		err := Pipeline.RemoveAt(1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		names := Pipeline.Names()
		expected := []string{"first", "third"}
		if !reflect.DeepEqual(names, expected) {
			t.Errorf("RemoveAt failed: expected %v, got %v", expected, names)
		}
	})

	t.Run("ReplaceAt", func(t *testing.T) {
		Pipeline := NewPipeline[string]()
		Pipeline.Register(
			Transform("first", func(_ context.Context, s string) string { return s }),
			Transform("old", func(_ context.Context, s string) string { return s }),
			Transform("third", func(_ context.Context, s string) string { return s }),
		)

		err := Pipeline.ReplaceAt(1, Transform("new", func(_ context.Context, s string) string { return s }))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		names := Pipeline.Names()
		if names[1] != "new" {
			t.Errorf("ReplaceAt failed: expected 'new' at index 1, got %s", names[1])
		}
	})

	t.Run("MoveToHead", func(t *testing.T) {
		Pipeline := NewPipeline[string]()
		Pipeline.Register(
			Transform("first", func(_ context.Context, s string) string { return s }),
			Transform("second", func(_ context.Context, s string) string { return s }),
			Transform("third", func(_ context.Context, s string) string { return s }),
		)

		err := Pipeline.MoveToHead(2)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		names := Pipeline.Names()
		expected := []string{"third", "first", "second"}
		if !reflect.DeepEqual(names, expected) {
			t.Errorf("MoveToHead failed: expected %v, got %v", expected, names)
		}
	})

	t.Run("MoveToHead Already at Head", func(t *testing.T) {
		Pipeline := NewPipeline[string]()
		Pipeline.Register(
			Transform("first", func(_ context.Context, s string) string { return s + "_1" }),
			Transform("second", func(_ context.Context, s string) string { return s + "_2" }),
		)

		// Try to move index 0 to head (no-op)
		err := Pipeline.MoveToHead(0)
		if err != nil {
			t.Errorf("MoveToHead(0) should succeed as no-op, got %v", err)
		}

		// Verify order unchanged
		result, err := Pipeline.Process(context.Background(), "test")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result != "test_1_2" {
			t.Errorf("order should be unchanged: %s", result)
		}
	})

	t.Run("MoveToTail", func(t *testing.T) {
		Pipeline := NewPipeline[string]()
		Pipeline.Register(
			Transform("first", func(_ context.Context, s string) string { return s }),
			Transform("second", func(_ context.Context, s string) string { return s }),
			Transform("third", func(_ context.Context, s string) string { return s }),
		)

		err := Pipeline.MoveToTail(0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		names := Pipeline.Names()
		expected := []string{"second", "third", "first"}
		if !reflect.DeepEqual(names, expected) {
			t.Errorf("MoveToTail failed: expected %v, got %v", expected, names)
		}
	})

	t.Run("MoveToTail Invalid Index", func(t *testing.T) {
		Pipeline := NewPipeline[string]()
		Pipeline.Register(Transform("single", func(_ context.Context, s string) string { return s }))

		err := Pipeline.MoveToTail(-1)
		if !errors.Is(err, ErrIndexOutOfBounds) {
			t.Errorf("MoveToTail(-1) should return ErrIndexOutOfBounds, got %v", err)
		}
	})

	t.Run("MoveToTail Already at Tail", func(t *testing.T) {
		Pipeline := NewPipeline[string]()
		Pipeline.Register(
			Transform("first", func(_ context.Context, s string) string { return s + "_1" }),
			Transform("last", func(_ context.Context, s string) string { return s + "_2" }),
		)

		// Try to move last index to tail (no-op)
		err := Pipeline.MoveToTail(1)
		if err != nil {
			t.Errorf("MoveToTail on last element should succeed as no-op, got %v", err)
		}

		// Verify order unchanged
		result, err := Pipeline.Process(context.Background(), "test")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result != "test_1_2" {
			t.Errorf("order should be unchanged: %s", result)
		}
	})

	t.Run("MoveTo", func(t *testing.T) {
		Pipeline := NewPipeline[string]()
		Pipeline.Register(
			Transform("A", func(_ context.Context, s string) string { return s }),
			Transform("B", func(_ context.Context, s string) string { return s }),
			Transform("C", func(_ context.Context, s string) string { return s }),
			Transform("D", func(_ context.Context, s string) string { return s }),
		)

		// Move B (index 1) to index 3
		// After removing B: [A, C, D]
		// Insert at adjusted index (3-1=2): [A, C, B, D]
		err := Pipeline.MoveTo(1, 3)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		names := Pipeline.Names()
		expected := []string{"A", "C", "B", "D"}
		if !reflect.DeepEqual(names, expected) {
			t.Errorf("MoveTo failed: expected %v, got %v", expected, names)
		}
	})

	t.Run("MoveTo Same Index", func(t *testing.T) {
		Pipeline := NewPipeline[string]()
		Pipeline.Register(
			Transform("A", func(_ context.Context, s string) string { return s + "_A" }),
			Transform("B", func(_ context.Context, s string) string { return s + "_B" }),
		)

		// Move to same position (no-op)
		err := Pipeline.MoveTo(1, 1)
		if err != nil {
			t.Errorf("MoveTo same index should succeed as no-op, got %v", err)
		}

		// Verify order unchanged
		result, err := Pipeline.Process(context.Background(), "test")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result != "test_A_B" {
			t.Errorf("order should be unchanged: %s", result)
		}
	})

	t.Run("Swap", func(t *testing.T) {
		Pipeline := NewPipeline[string]()
		Pipeline.Register(
			Transform("first", func(_ context.Context, s string) string { return s }),
			Transform("second", func(_ context.Context, s string) string { return s }),
			Transform("third", func(_ context.Context, s string) string { return s }),
		)

		err := Pipeline.Swap(0, 2)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		names := Pipeline.Names()
		expected := []string{"third", "second", "first"}
		if !reflect.DeepEqual(names, expected) {
			t.Errorf("Swap failed: expected %v, got %v", expected, names)
		}
	})

	t.Run("Reverse", func(t *testing.T) {
		Pipeline := NewPipeline[string]()
		Pipeline.Register(
			Transform("A", func(_ context.Context, s string) string { return s }),
			Transform("B", func(_ context.Context, s string) string { return s }),
			Transform("C", func(_ context.Context, s string) string { return s }),
			Transform("D", func(_ context.Context, s string) string { return s }),
		)

		Pipeline.Reverse()

		names := Pipeline.Names()
		expected := []string{"D", "C", "B", "A"}
		if !reflect.DeepEqual(names, expected) {
			t.Errorf("Reverse failed: expected %v, got %v", expected, names)
		}
	})

	t.Run("Bounds Checking", func(t *testing.T) {
		Pipeline := NewPipeline[string]()
		Pipeline.Register(Transform("only", func(_ context.Context, s string) string { return s }))

		// Test various operations with invalid indices
		if err := Pipeline.InsertAt(-1); !errors.Is(err, ErrIndexOutOfBounds) {
			t.Errorf("InsertAt(-1) should return ErrIndexOutOfBounds")
		}
		if err := Pipeline.InsertAt(5); !errors.Is(err, ErrIndexOutOfBounds) {
			t.Errorf("InsertAt(5) should return ErrIndexOutOfBounds")
		}
		if err := Pipeline.RemoveAt(-1); !errors.Is(err, ErrIndexOutOfBounds) {
			t.Errorf("RemoveAt(-1) should return ErrIndexOutOfBounds")
		}
		if err := Pipeline.RemoveAt(1); !errors.Is(err, ErrIndexOutOfBounds) {
			t.Errorf("RemoveAt(1) should return ErrIndexOutOfBounds")
		}
		if err := Pipeline.MoveToHead(1); !errors.Is(err, ErrIndexOutOfBounds) {
			t.Errorf("MoveToHead(1) should return ErrIndexOutOfBounds")
		}
		if err := Pipeline.MoveTo(0, 5); !errors.Is(err, ErrIndexOutOfBounds) {
			t.Errorf("MoveTo(0, 5) should return ErrIndexOutOfBounds")
		}
		if err := Pipeline.Swap(0, 1); !errors.Is(err, ErrIndexOutOfBounds) {
			t.Errorf("Swap(0, 1) should return ErrIndexOutOfBounds")
		}

		// Test ReplaceAt with invalid index
		processor := Transform("new", func(_ context.Context, s string) string { return s })
		if err := Pipeline.ReplaceAt(5, processor); !errors.Is(err, ErrIndexOutOfBounds) {
			t.Errorf("ReplaceAt(5) should return ErrIndexOutOfBounds")
		}
	})
}

func TestPipelineConcurrency(t *testing.T) {
	t.Run("Concurrent Process", func(t *testing.T) {
		Pipeline := NewPipeline[int]()
		Pipeline.Register(
			Transform("add_one", func(_ context.Context, n int) int {
				time.Sleep(time.Millisecond) // Simulate work
				return n + 1
			}),
			Transform("double", func(_ context.Context, n int) int {
				time.Sleep(time.Millisecond) // Simulate work
				return n * 2
			}),
		)

		var wg sync.WaitGroup
		errors := make([]error, 100)
		results := make([]int, 100)

		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				result, err := Pipeline.Process(context.Background(), idx)
				results[idx] = result
				errors[idx] = err
			}(i)
		}

		wg.Wait()

		// Verify all operations succeeded
		for i, err := range errors {
			if err != nil {
				t.Errorf("concurrent process %d failed: %v", i, err)
			}
			expected := (i + 1) * 2
			if results[i] != expected {
				t.Errorf("process %d: expected %d, got %d", i, expected, results[i])
			}
		}
	})

	t.Run("Concurrent Modification", func(t *testing.T) {
		Pipeline := NewPipeline[string]()
		Pipeline.Register(Transform("initial", func(_ context.Context, s string) string { return s }))

		var wg sync.WaitGroup

		// Concurrent modifications
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				name := fmt.Sprintf("proc_%d", idx)
				Pipeline.PushTail(Transform(name, func(_ context.Context, s string) string { return s }))
			}(i)
		}

		// Concurrent reads
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_ = Pipeline.Names()
				_ = Pipeline.Len()
			}()
		}

		wg.Wait()

		// Should have initial + 10 added processors
		if Pipeline.Len() != 11 {
			t.Errorf("expected 11 processors after concurrent adds, got %d", Pipeline.Len())
		}
	})
}

func TestPipelineEdgeCases(t *testing.T) {
	t.Run("Process With Nil Context", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				// Expected to panic with nil context
				return
			}
			t.Error("expected panic with nil context")
		}()

		Pipeline := NewPipeline[string]()
		Pipeline.Register(Transform("test", func(_ context.Context, s string) string { return s }))
		_, _ = Pipeline.Process(nil, "test") //nolint:errcheck,staticcheck // Testing nil context panic
	})

	t.Run("Empty Processor Name", func(t *testing.T) {
		Pipeline := NewPipeline[string]()
		Pipeline.Register(Transform("", func(_ context.Context, s string) string { return s + "_processed" }))

		result, err := Pipeline.Process(context.Background(), "test")
		if err != nil {
			t.Fatalf("empty processor name should not cause error: %v", err)
		}
		if result != "test_processed" {
			t.Errorf("processor should still work with empty name")
		}

		names := Pipeline.Names()
		if len(names) != 1 || names[0] != "" {
			t.Errorf("should preserve empty name")
		}
	})

	t.Run("Very Long Pipeline", func(t *testing.T) {
		Pipeline := NewPipeline[int]()

		// Create a pipeline with 1000 processors
		for i := 0; i < 1000; i++ {
			name := fmt.Sprintf("step_%d", i)
			Pipeline.Register(Transform(name, func(_ context.Context, n int) int {
				return n + 1
			}))
		}

		result, err := Pipeline.Process(context.Background(), 0)
		if err != nil {
			t.Fatalf("long pipeline failed: %v", err)
		}
		if result != 1000 {
			t.Errorf("expected 1000, got %d", result)
		}
	})

	t.Run("Processor Panic Recovery", func(t *testing.T) {
		// Note: Currently processors that panic will crash the pipeline
		// This test documents current behavior
		Pipeline := NewPipeline[string]()
		Pipeline.Register(Transform("panic", func(_ context.Context, _ string) string {
			panic("processor panic")
		}))

		defer func() {
			if r := recover(); r != nil {
				// Expected - processor panics propagate
				return
			}
			t.Error("expected panic to propagate")
		}()

		_, _ = Pipeline.Process(context.Background(), "test") //nolint:errcheck // Testing panic propagation
	})
}
