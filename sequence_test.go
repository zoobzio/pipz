package pipz

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/zoobzio/capitan"
)

// Test identity variables.
var (
	// Sequence identities.
	testSequence = NewIdentity("test", "")
	mainSequence = NewIdentity("main", "")
	subSequence  = NewIdentity("sub", "")
	seq1         = NewIdentity("seq1", "")
	seq2         = NewIdentity("seq2", "")

	// Processor identities.
	upperID      = NewIdentity("upper", "")
	trimID       = NewIdentity("trim", "")
	lowerID      = NewIdentity("lower", "")
	nonEmpty     = NewIdentity("non_empty", "")
	doubleID     = NewIdentity("double", "")
	incrementID  = NewIdentity("increment", "")
	addTen       = NewIdentity("add_ten", "")
	squareID     = NewIdentity("square", "")
	step1        = NewIdentity("step1", "")
	step2        = NewIdentity("step2", "")
	step3        = NewIdentity("step3", "")
	errorProc    = NewIdentity("error-proc", "")
	slowID       = NewIdentity("slow", "")
	slowProc    = NewIdentity("slow-proc", "")
	transformID = NewIdentity("transform", "")
	processorID = NewIdentity("processor", "")

	// Modification test identities.
	headID = NewIdentity("head", "")
	tailID = NewIdentity("tail", "")
	p0     = NewIdentity("p0", "")
	p1ID   = NewIdentity("p1", "")
	p2ID   = NewIdentity("p2", "")
	p3     = NewIdentity("p3", "")
	p4     = NewIdentity("p4", "")

	// Concurrency test identities.
	concurrentID = NewIdentity("concurrent", "")
	modifyID     = NewIdentity("modify", "")

	// Edge case identities.
	nilCtx       = NewIdentity("nil-ctx", "")
	emptyID      = NewIdentity("", "")
	veryLong     = NewIdentity("very-long", "")
	panicProcID  = NewIdentity("panic", "")
	recoverProc  = NewIdentity("recover", "")
	beforePanic  = NewIdentity("before-panic", "")
	customSeqID  = NewIdentity("custom-sequence", "")

	// Connector identities.
	riskyID  = NewIdentity("risky", "")
	backupID = NewIdentity("backup", "")
	safeID   = NewIdentity("safe", "")
)

func TestNewSequence(t *testing.T) {
	Sequence := NewSequence[string](testSequence)

	if Sequence == nil {
		t.Fatal("NewSequence should not return nil")
	}

	if Sequence.Len() != 0 {
		t.Errorf("new Sequence should be empty, got length %d", Sequence.Len())
	}

	if Sequence.Len() != 0 {
		t.Error("new Sequence should be empty")
	}
}

func TestSequenceRegister(t *testing.T) {
	t.Run("Register Single Processor", func(t *testing.T) {
		Sequence := NewSequence[string](testSequence)
		processor := Transform(upperID, func(_ context.Context, s string) string {
			return strings.ToUpper(s)
		})

		Sequence.Register(processor)

		if Sequence.Len() != 1 {
			t.Errorf("expected 1 processor, got %d", Sequence.Len())
		}
	})

	t.Run("Register Multiple Processors", func(t *testing.T) {
		Sequence := NewSequence[string](testSequence)

		Sequence.Register(
			Transform(trimID, func(_ context.Context, s string) string {
				return strings.TrimSpace(s)
			}),
			Transform(lowerID, func(_ context.Context, s string) string {
				return strings.ToLower(s)
			}),
			Effect(nonEmpty, func(_ context.Context, s string) error {
				if s == "" {
					return errors.New("empty string")
				}
				return nil
			}),
		)

		if Sequence.Len() != 3 {
			t.Errorf("expected 3 processors, got %d", Sequence.Len())
		}

		names := Sequence.Names()
		expected := []string{trimID.Name(), lowerID.Name(), nonEmpty.Name()}
		if !reflect.DeepEqual(names, expected) {
			t.Errorf("expected names %v, got %v", expected, names)
		}
	})

	t.Run("Register Connectors Directly", func(t *testing.T) {
		// Create a sub-sequence
		subSeq := NewSequence[int](subSequence)
		subSeq.Register(
			Transform(doubleID, func(_ context.Context, n int) int {
				return n * 2
			}),
		)

		// Create a fallback connector
		fallback := NewFallback(safeID,
			Apply(riskyID, func(_ context.Context, n int) (int, error) {
				if n == 100 {
					return 0, errors.New("error at 100")
				}
				return n + 1, nil
			}),
			Transform(backupID, func(_ context.Context, n int) int {
				return n + 1000
			}),
		)

		// Register both processors and connectors
		mainSeq := NewSequence[int](mainSequence)
		mainSeq.Register(
			Transform(incrementID, func(_ context.Context, n int) int {
				return n + 1
			}),
			subSeq,   // Connector registered directly
			fallback, // Another connector
		)

		if mainSeq.Len() != 3 {
			t.Errorf("expected 3 items, got %d", mainSeq.Len())
		}

		// Test normal case: 5 + 1 = 6, 6 * 2 = 12, 12 + 1 = 13
		result, err := mainSeq.Process(context.Background(), 5)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != 13 {
			t.Errorf("expected 13, got %d", result)
		}

		// Test fallback case: 49 + 1 = 50, 50 * 2 = 100, 100 triggers error, fallback = 100 + 1000 = 1100
		result, err = mainSeq.Process(context.Background(), 49)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != 1100 {
			t.Errorf("expected 1100, got %d", result)
		}
	})

	t.Run("NewSequence With Initial Processors", func(t *testing.T) {
		// Create processors
		upperProc := Transform(upperID, func(_ context.Context, s string) string {
			return strings.ToUpper(s)
		})
		trimProc := Transform(trimID, func(_ context.Context, s string) string {
			return strings.TrimSpace(s)
		})
		validateProc := Effect(NewIdentity("validate", ""), func(_ context.Context, s string) error {
			if s == "" {
				return errors.New("empty string")
			}
			return nil
		})

		// Single line declaration with initial processors
		seq := NewSequence(NewIdentity("pipeline", ""), upperProc, trimProc, validateProc)

		// Should have 3 processors
		if seq.Len() != 3 {
			t.Errorf("expected 3 processors, got %d", seq.Len())
		}

		// Test processing
		result, err := seq.Process(context.Background(), "  hello world  ")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != "HELLO WORLD" {
			t.Errorf("expected 'HELLO WORLD', got '%s'", result)
		}

		// Can still add more processors later (admin API)
		seq.Register(Transform(NewIdentity("exclaim", ""), func(_ context.Context, s string) string {
			return s + "!"
		}))

		if seq.Len() != 4 {
			t.Errorf("expected 4 processors after Register, got %d", seq.Len())
		}

		// Test with additional processor
		result, err = seq.Process(context.Background(), "  hello  ")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != "HELLO!" {
			t.Errorf("expected 'HELLO!', got '%s'", result)
		}
	})

	t.Run("NewSequence With Empty Initial Processors", func(t *testing.T) {
		// Can still create empty sequence
		seq := NewSequence[int](NewIdentity("empty-pipeline", ""))

		if seq.Len() != 0 {
			t.Errorf("expected 0 processors, got %d", seq.Len())
		}

		// Process should pass through unchanged
		result, err := seq.Process(context.Background(), 42)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != 42 {
			t.Errorf("expected 42, got %d", result)
		}
	})
}

func TestSequenceProcess(t *testing.T) {
	t.Run("Empty Sequence", func(t *testing.T) {
		Sequence := NewSequence[int](testSequence)
		result, err := Sequence.Process(context.Background(), 42)

		if err != nil {
			t.Fatalf("empty sequence should not error: %v", err)
		}
		if result != 42 {
			t.Errorf("empty sequence should return input unchanged, got %d", result)
		}
	})

	t.Run("Single Processor Success", func(t *testing.T) {
		Sequence := NewSequence[int](testSequence)
		Sequence.Register(Transform(doubleID, func(_ context.Context, n int) int {
			return n * 2
		}))

		result, err := Sequence.Process(context.Background(), 5)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != 10 {
			t.Errorf("expected 10, got %d", result)
		}
	})

	t.Run("Multiple Processors Chain", func(t *testing.T) {
		Sequence := NewSequence[int](testSequence)
		Sequence.Register(
			Transform(doubleID, func(_ context.Context, n int) int {
				return n * 2
			}),
			Transform(addTen, func(_ context.Context, n int) int {
				return n + 10
			}),
			Transform(squareID, func(_ context.Context, n int) int {
				return n * n
			}),
		)

		// 5 -> 10 -> 20 -> 400
		result, err := Sequence.Process(context.Background(), 5)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != 400 {
			t.Errorf("expected 400, got %d", result)
		}
	})

	t.Run("Processor Error Stops Sequence", func(t *testing.T) {
		Sequence := NewSequence[string](testSequence)
		Sequence.Register(
			Transform(step1, func(_ context.Context, s string) string {
				return s + "_1"
			}),
			Apply(step2, func(_ context.Context, _ string) (string, error) {
				return "", errors.New("step2 failed")
			}),
			Transform(step3, func(_ context.Context, s string) string {
				t.Error("step3 should not be called")
				return s + "_3"
			}),
		)

		result, err := Sequence.Process(context.Background(), "test")

		if err == nil {
			t.Fatal("expected error from step2")
		}
		if result != "" {
			t.Errorf("expected empty string on error, got %q", result)
		}

		var pipeErr *Error[string]
		if errors.As(err, &pipeErr) {
			if pipeErr.InputData != "test_1" {
				t.Errorf("expected InputData to be \"test_1\", got %q", pipeErr.InputData)
			}
		} else {
			t.Error("expected error to be of type *pipz.Error[string]")
		}
	})

	t.Run("Context Cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		Sequence := NewSequence[int](testSequence)
		Sequence.Register(Transform(doubleID, func(_ context.Context, n int) int {
			return n * 2
		}))

		_, err := Sequence.Process(ctx, 5)

		if err == nil {
			t.Fatal("expected context cancellation error")
		}
		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled, got %v", err)
		}
	})

	t.Run("Context Timeout", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
		defer cancel()

		Sequence := NewSequence[int](testSequence)
		Sequence.Register(Apply(slowID, func(ctx context.Context, n int) (int, error) {
			select {
			case <-time.After(100 * time.Millisecond):
				return n * 2, nil
			case <-ctx.Done():
				return 0, ctx.Err()
			}
		}))

		_, err := Sequence.Process(ctx, 5)

		if err == nil {
			t.Fatal("expected timeout error")
		}
		var pipeErr *Error[int]
		if errors.As(err, &pipeErr) {
			if !pipeErr.Timeout {
				t.Error("expected Timeout flag to be true")
			}
		} else {
			t.Error("expected error to be of type *pipz.Error[int]")
		}
	})

	t.Run("Error Contains Input Data", func(t *testing.T) {
		Sequence := NewSequence[int](testSequence)
		Sequence.Register(
			Transform(doubleID, func(_ context.Context, n int) int {
				return n * 2
			}),
			Apply(errorProc, func(_ context.Context, _ int) (int, error) {
				return 0, errors.New("processing failed")
			}),
		)

		_, err := Sequence.Process(context.Background(), 5)

		var pipeErr *Error[int]
		if errors.As(err, &pipeErr) {
			if pipeErr.InputData != 10 {
				t.Errorf("expected InputData to be 10, got %d", pipeErr.InputData)
			}
		} else {
			t.Error("expected error to be of type *pipz.Error[int]")
		}
	})
}

func TestSequenceLink(t *testing.T) {

	seq1 := NewSequence[string](seq1)
	seq1.Register(
		Transform(p1ID, func(_ context.Context, s string) string {
			return s + "_p1"
		}),
		Transform(p2ID, func(_ context.Context, s string) string {
			return s + "_p2"
		}),
	)

	seq2 := NewSequence[string](seq2)
	seq2.Register(
		Transform(p3, func(_ context.Context, s string) string {
			return s + "_p3"
		}),
		Transform(p4, func(_ context.Context, s string) string {
			return s + "_p4"
		}),
	)

	// Add seq1 as a processor in seq2
	seq2.Unshift(seq1)

	result, err := seq2.Process(context.Background(), "start")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "start_p1_p2_p3_p4"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestSequenceIntrospection(t *testing.T) {

	t.Run("Names", func(t *testing.T) {
		Sequence := NewSequence[int](testSequence)
		Sequence.Register(
			Transform(transformID, func(_ context.Context, n int) int { return n }),
			Effect(processorID, func(_ context.Context, _ int) error { return nil }),
		)

		names := Sequence.Names()
		expected := []string{transformID.Name(), processorID.Name()}
		if !reflect.DeepEqual(names, expected) {
			t.Errorf("expected names %v, got %v", expected, names)
		}
	})

	t.Run("Len", func(t *testing.T) {
		Sequence := NewSequence[int](testSequence)

		if Sequence.Len() != 0 {
			t.Errorf("empty sequence should have length 0, got %d", Sequence.Len())
		}

		Sequence.Register(Transform(transformID, func(_ context.Context, n int) int { return n }))

		if Sequence.Len() != 1 {
			t.Errorf("sequence should have length 1, got %d", Sequence.Len())
		}
	})
}

func TestSequenceModification(t *testing.T) {

	makeTransform := func(identity Identity, suffix string) Chainable[string] {
		return Transform(identity, func(_ context.Context, s string) string {
			return s + suffix
		})
	}

	t.Run("Clear", func(t *testing.T) {
		Sequence := NewSequence[string](testSequence)
		Sequence.Register(
			makeTransform(p1ID, "_1"),
			makeTransform(p2ID, "_2"),
		)

		Sequence.Clear()

		if Sequence.Len() != 0 {
			t.Errorf("cleared sequence should have length 0, got %d", Sequence.Len())
		}
	})

	t.Run("Unshift", func(t *testing.T) {
		Sequence := NewSequence[string](testSequence)
		Sequence.Register(makeTransform(p1ID, "_1"))
		Sequence.Unshift(makeTransform(headID, "_head"))

		names := Sequence.Names()
		expected := []string{headID.Name(), p1ID.Name()}
		if !reflect.DeepEqual(names, expected) {
			t.Errorf("expected names %v, got %v", expected, names)
		}
	})

	t.Run("Push", func(t *testing.T) {
		Sequence := NewSequence[string](testSequence)
		Sequence.Register(makeTransform(p1ID, "_1"))
		Sequence.Push(makeTransform(tailID, "_tail"))

		names := Sequence.Names()
		expected := []string{p1ID.Name(), tailID.Name()}
		if !reflect.DeepEqual(names, expected) {
			t.Errorf("expected names %v, got %v", expected, names)
		}
	})

	t.Run("Shift", func(t *testing.T) {
		Sequence := NewSequence[string](testSequence)
		headProc := makeTransform(headID, "_head")
		Sequence.Register(headProc, makeTransform(p1ID, "_1"))

		popped, err := Sequence.Shift()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if popped.Identity().Name() != headID.Name() {
			t.Errorf("expected popped processor name %q, got %q", headID.Name(), popped.Identity().Name())
		}

		if Sequence.Len() != 1 {
			t.Errorf("expected 1 processor after pop, got %d", Sequence.Len())
		}
	})

	t.Run("Shift Empty", func(t *testing.T) {
		Sequence := NewSequence[string](testSequence)
		_, err := Sequence.Shift()
		if err == nil {
			t.Error("expected error when popping from empty sequence")
		}
	})

	t.Run("Pop", func(t *testing.T) {
		Sequence := NewSequence[string](testSequence)
		tailProc := makeTransform(tailID, "_tail")
		Sequence.Register(makeTransform(p1ID, "_1"), tailProc)

		popped, err := Sequence.Pop()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if popped.Identity().Name() != tailID.Name() {
			t.Errorf("expected popped processor name %q, got %q", tailID.Name(), popped.Identity().Name())
		}

		if Sequence.Len() != 1 {
			t.Errorf("expected 1 processor after pop, got %d", Sequence.Len())
		}
	})

	t.Run("Pop Empty", func(t *testing.T) {
		Sequence := NewSequence[string](testSequence)
		_, err := Sequence.Pop()
		if err == nil {
			t.Error("expected error when popping from empty sequence")
		}
	})
	t.Run("Bounds Checking", func(t *testing.T) {
		Sequence := NewSequence[string](testSequence)
		Sequence.Register(makeTransform(p0, "_0"))

		tests := []struct {
			fn      func() error
			name    string
			wantErr bool
		}{}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := tt.fn()
				if (err != nil) != tt.wantErr {
					t.Errorf("got error = %v, wantErr = %v", err, tt.wantErr)
				}
			})
		}
	})
}

func TestSequenceConcurrency(t *testing.T) {

	t.Run("Concurrent Process", func(t *testing.T) {
		Sequence := NewSequence[int](concurrentID)
		Sequence.Register(
			Transform(doubleID, func(_ context.Context, n int) int {
				return n * 2
			}),
			Apply(slowProc, func(_ context.Context, n int) (int, error) {
				time.Sleep(10 * time.Millisecond)
				return n + 1, nil
			}),
		)

		var wg sync.WaitGroup
		results := make([]int, 10)
		errs := make([]*Error[int], 10)

		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				var err error
				results[idx], err = Sequence.Process(context.Background(), idx)
				if err != nil {
					var pipeErr *Error[int]
					if errors.As(err, &pipeErr) {
						errs[idx] = pipeErr
					}
				}
			}(i)
		}

		wg.Wait()

		for i := 0; i < 10; i++ {
			if errs[i] != nil {
				t.Errorf("unexpected error for input %d: %v", i, errs[i])
			}
			expected := i*2 + 1
			if results[i] != expected {
				t.Errorf("for input %d, expected %d, got %d", i, expected, results[i])
			}
		}
	})

	t.Run("Concurrent Modification", func(_ *testing.T) {
		Sequence := NewSequence[string](modifyID)

		// Pre-defined processors for concurrent testing
		proc1 := Transform(p1ID, func(_ context.Context, s string) string { return s + "_1" })
		proc2 := Transform(p2ID, func(_ context.Context, s string) string { return s + "_2" })
		proc3 := Transform(p3, func(_ context.Context, s string) string { return s + "_3" })
		proc4 := Transform(p4, func(_ context.Context, s string) string { return s + "_4" })

		Sequence.Register(Transform(p0, func(_ context.Context, s string) string {
			return s + "_0"
		}))

		var wg sync.WaitGroup
		wg.Add(3)

		// Reader
		go func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				_ = Sequence.Len()
				_ = Sequence.Names()
				time.Sleep(time.Microsecond)
			}
		}()

		// Modifier 1 - repeatedly add/remove known processors
		go func() {
			defer wg.Done()
			processors := []Chainable[string]{proc1, proc2, proc3, proc4}
			for i := 0; i < 50; i++ {
				proc := processors[i%len(processors)]
				Sequence.Push(proc)
				time.Sleep(time.Microsecond)
			}
		}()

		// Modifier 2
		go func() {
			defer wg.Done()
			for i := 0; i < 25; i++ {
				if Sequence.Len() > 1 {
					_, err := Sequence.Shift()
					_ = err // Intentionally ignoring error in concurrent test
				}
				time.Sleep(2 * time.Microsecond)
			}
		}()

		wg.Wait()

		// Just verify we didn't crash
		_ = Sequence.Len()
	})
}

func TestSequenceEdgeCases(t *testing.T) {

	t.Run("Process With Nil Context", func(t *testing.T) {
		Sequence := NewSequence[int](nilCtx)
		Sequence.Register(Transform(doubleID, func(_ context.Context, n int) int {
			return n * 2
		}))

		// Should handle nil context gracefully
		//nolint:staticcheck // SA1012: intentionally testing nil context handling
		result, err := Sequence.Process(nil, 5)
		if err != nil {
			t.Fatalf("unexpected error with nil context: %v", err)
		}
		if result != 10 {
			t.Errorf("expected 10, got %d", result)
		}
	})

	t.Run("Empty Processor Name", func(t *testing.T) {
		Sequence := NewSequence[int](testSequence)
		Sequence.Register(Transform(emptyID, func(_ context.Context, n int) int {
			return n * 2
		}))

		names := Sequence.Names()
		if len(names) != 1 || names[0] != "" {
			t.Errorf("expected empty name in list, got %v", names)
		}
	})

	t.Run("Very Long Sequence", func(t *testing.T) {
		Sequence := NewSequence[int](veryLong)

		// Use a smaller set of pre-defined processors that we reuse
		// This tests the same functionality while following const-driven patterns
		incrementProc := Transform(incrementID, func(_ context.Context, n int) int {
			return n + 1
		})

		// Add the same processor 1000 times to test long sequences
		// In real usage, processors would have unique names, but for testing
		// sequence handling, reusing the same processor is acceptable
		for i := 0; i < 1000; i++ {
			Sequence.Register(incrementProc)
		}

		result, err := Sequence.Process(context.Background(), 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != 1000 {
			t.Errorf("expected 1000, got %d", result)
		}
	})

	t.Run("Processor Panic Recovered", func(t *testing.T) {
		Sequence := NewSequence[string](testSequence)
		Sequence.Register(
			Transform(beforePanic, func(_ context.Context, s string) string {
				return s + "_before"
			}),
			Apply(panicProcID, func(_ context.Context, _ string) (string, error) {
				panic("processor panic!")
			}),
			Transform(recoverProc, func(_ context.Context, s string) string {
				t.Error("should not reach this processor after panic")
				return s + "_after"
			}),
		)

		// The panic should now be caught and converted to an error
		result, err := Sequence.Process(context.Background(), "test")

		// Should get empty result from panic recovery
		if result != "" {
			t.Errorf("expected empty string from panic recovery, got %q", result)
		}

		// Should get pipz.Error with panic information
		if err == nil {
			t.Fatal("expected error from panic recovery")
		}

		var pipzErr *Error[string]
		if !errors.As(err, &pipzErr) {
			t.Fatal("expected pipz.Error from panic recovery")
		}

		// Check path includes both sequence and processor
		if len(pipzErr.Path) != 2 || pipzErr.Path[0].Name() != testSequence.Name() || pipzErr.Path[1].Name() != panicProcID.Name() {
			t.Errorf("expected path [%s, %s], got %v", testSequence.Name(), panicProcID.Name(), pipzErr.Path)
		}
	})
}

func TestSequenceName(t *testing.T) {
	seq := NewSequence[int](customSeqID)
	if seq.Identity().Name() != customSeqID.Name() {
		t.Errorf("expected sequence name %q, got %q", customSeqID.Name(), seq.Identity().Name())
	}
}

func TestSequenceRemove(t *testing.T) {
	t.Run("remove existing processor", func(t *testing.T) {
		seq := NewSequence[string](testSequence)
		seq.Register(
			Transform(upperID, func(_ context.Context, s string) string { return strings.ToUpper(s) }),
			Transform(trimID, func(_ context.Context, s string) string { return strings.TrimSpace(s) }),
			Transform(lowerID, func(_ context.Context, s string) string { return strings.ToLower(s) }),
		)

		// Verify initial state
		names := seq.Names()
		expected := []string{upperID.Name(), trimID.Name(), lowerID.Name()}
		if !reflect.DeepEqual(names, expected) {
			t.Errorf("expected names %v, got %v", expected, names)
		}

		// Remove middle processor
		err := seq.Remove(trimID)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		// Verify removal
		names = seq.Names()
		expected = []string{upperID.Name(), lowerID.Name()}
		if !reflect.DeepEqual(names, expected) {
			t.Errorf("expected names %v after removal, got %v", expected, names)
		}

		// Verify functionality still works
		result, pErr := seq.Process(context.Background(), "  Hello  ")
		if pErr != nil {
			t.Errorf("unexpected error: %v", pErr)
		}
		if result != "  hello  " {
			t.Errorf("expected '  hello  ', got %q", result)
		}
	})

	t.Run("remove non-existent processor", func(t *testing.T) {
		seq := NewSequence[string](testSequence)
		seq.Register(Transform(upperID, func(_ context.Context, s string) string { return strings.ToUpper(s) }))

		nonExistentID := NewIdentity("nonexistent", "")
		err := seq.Remove(nonExistentID)
		if err == nil {
			t.Error("expected error for non-existent processor")
		}
		if !strings.Contains(err.Error(), "nonexistent") {
			t.Errorf("expected error to mention 'nonexistent', got %q", err.Error())
		}
	})

	t.Run("remove from empty sequence", func(t *testing.T) {
		seq := NewSequence[string](testSequence)

		err := seq.Remove(upperID)
		if err == nil {
			t.Error("expected error when removing from empty sequence")
		}
	})
}

func TestSequenceReplace(t *testing.T) {
	t.Run("replace existing processor", func(t *testing.T) {
		seq := NewSequence[string](testSequence)
		seq.Register(
			Transform(upperID, func(_ context.Context, s string) string { return strings.ToUpper(s) }),
			Transform(trimID, func(_ context.Context, s string) string { return strings.TrimSpace(s) }),
			Transform(lowerID, func(_ context.Context, s string) string { return strings.ToLower(s) }),
		)

		// Replace middle processor
		newProcessor := Transform(trimID, func(_ context.Context, s string) string {
			return s + "_replaced"
		})

		err := seq.Replace(trimID, newProcessor)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		// Verify names unchanged
		names := seq.Names()
		expected := []string{upperID.Name(), trimID.Name(), lowerID.Name()}
		if !reflect.DeepEqual(names, expected) {
			t.Errorf("expected names %v, got %v", expected, names)
		}

		// Verify new functionality
		result, pErr := seq.Process(context.Background(), "  Hello  ")
		if pErr != nil {
			t.Errorf("unexpected error: %v", pErr)
		}
		if result != "  hello  _replaced" {
			t.Errorf("expected '  hello  _replaced', got %q", result)
		}
	})

	t.Run("replace non-existent processor", func(t *testing.T) {
		seq := NewSequence[string](testSequence)
		seq.Register(Transform(upperID, func(_ context.Context, s string) string { return strings.ToUpper(s) }))

		nonExistentID := NewIdentity("nonexistent", "")
		newProcessor := Transform(NewIdentity("replacement", ""), func(_ context.Context, s string) string { return strings.ToLower(s) })
		err := seq.Replace(nonExistentID, newProcessor)
		if err == nil {
			t.Error("expected error for non-existent processor")
		}
		if !strings.Contains(err.Error(), "nonexistent") {
			t.Errorf("expected error to mention 'nonexistent', got %q", err.Error())
		}
	})
}

func TestSequenceAfter(t *testing.T) {
	t.Run("insert after existing processor", func(t *testing.T) {
		seq := NewSequence[string](testSequence)
		seq.Register(
			Transform(upperID, func(_ context.Context, s string) string { return strings.ToUpper(s) }),
			Transform(lowerID, func(_ context.Context, s string) string { return strings.ToLower(s) }),
		)

		// Insert after first processor
		trimProcessor := Transform(trimID, func(_ context.Context, s string) string { return strings.TrimSpace(s) })
		err := seq.After(upperID, trimProcessor)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		// Verify order
		names := seq.Names()
		expected := []string{upperID.Name(), trimID.Name(), lowerID.Name()}
		if !reflect.DeepEqual(names, expected) {
			t.Errorf("expected names %v, got %v", expected, names)
		}

		// Verify functionality
		result, procErr := seq.Process(context.Background(), "  hello  ")
		if procErr != nil {
			t.Errorf("unexpected error: %v", procErr)
		}
		if result != "hello" {
			t.Errorf("expected 'hello', got %q", result)
		}
	})

	t.Run("insert after last processor", func(t *testing.T) {
		seq := NewSequence[string](testSequence)
		seq.Register(Transform(upperID, func(_ context.Context, s string) string { return strings.ToUpper(s) }))

		lowerProcessor := Transform(lowerID, func(_ context.Context, s string) string { return strings.ToLower(s) })
		err := seq.After(upperID, lowerProcessor)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		names := seq.Names()
		expected := []string{upperID.Name(), lowerID.Name()}
		if !reflect.DeepEqual(names, expected) {
			t.Errorf("expected names %v, got %v", expected, names)
		}
	})

	t.Run("insert multiple processors", func(t *testing.T) {
		seq := NewSequence[string](testSequence)
		seq.Register(Transform(upperID, func(_ context.Context, s string) string { return strings.ToUpper(s) }))

		proc1 := Transform(NewIdentity("proc1", ""), func(_ context.Context, s string) string { return s + "_1" })
		proc2 := Transform(NewIdentity("proc2", ""), func(_ context.Context, s string) string { return s + "_2" })

		err := seq.After(upperID, proc1, proc2)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		names := seq.Names()
		expected := []string{upperID.Name(), "proc1", "proc2"}
		if !reflect.DeepEqual(names, expected) {
			t.Errorf("expected names %v, got %v", expected, names)
		}
	})

	t.Run("insert after non-existent processor", func(t *testing.T) {
		seq := NewSequence[string](testSequence)
		seq.Register(Transform(upperID, func(_ context.Context, s string) string { return strings.ToUpper(s) }))

		nonExistentID := NewIdentity("nonexistent", "")
		err := seq.After(nonExistentID, Transform(lowerID, func(_ context.Context, s string) string { return strings.ToLower(s) }))
		if err == nil {
			t.Error("expected error for non-existent processor")
		}
		if !strings.Contains(err.Error(), "nonexistent") {
			t.Errorf("expected error to mention 'nonexistent', got %q", err.Error())
		}
	})
}

func TestSequenceBefore(t *testing.T) {
	t.Run("insert before existing processor", func(t *testing.T) {
		seq := NewSequence[string](testSequence)
		seq.Register(
			Transform(upperID, func(_ context.Context, s string) string { return strings.ToUpper(s) }),
			Transform(lowerID, func(_ context.Context, s string) string { return strings.ToLower(s) }),
		)

		// Insert before second processor
		trimProcessor := Transform(trimID, func(_ context.Context, s string) string { return strings.TrimSpace(s) })
		err := seq.Before(lowerID, trimProcessor)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		// Verify order
		names := seq.Names()
		expected := []string{upperID.Name(), trimID.Name(), lowerID.Name()}
		if !reflect.DeepEqual(names, expected) {
			t.Errorf("expected names %v, got %v", expected, names)
		}

		// Verify functionality
		result, procErr := seq.Process(context.Background(), "  hello  ")
		if procErr != nil {
			t.Errorf("unexpected error: %v", procErr)
		}
		if result != "hello" {
			t.Errorf("expected 'hello', got %q", result)
		}
	})

	t.Run("insert before first processor", func(t *testing.T) {
		seq := NewSequence[string](testSequence)
		seq.Register(Transform(lowerID, func(_ context.Context, s string) string { return strings.ToLower(s) }))

		upperProcessor := Transform(upperID, func(_ context.Context, s string) string { return strings.ToUpper(s) })
		err := seq.Before(lowerID, upperProcessor)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		names := seq.Names()
		expected := []string{upperID.Name(), lowerID.Name()}
		if !reflect.DeepEqual(names, expected) {
			t.Errorf("expected names %v, got %v", expected, names)
		}
	})

	t.Run("insert multiple processors", func(t *testing.T) {
		seq := NewSequence[string](testSequence)
		seq.Register(Transform(lowerID, func(_ context.Context, s string) string { return strings.ToLower(s) }))

		proc1 := Transform(NewIdentity("proc1", ""), func(_ context.Context, s string) string { return s + "_1" })
		proc2 := Transform(NewIdentity("proc2", ""), func(_ context.Context, s string) string { return s + "_2" })

		err := seq.Before(lowerID, proc1, proc2)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		names := seq.Names()
		expected := []string{"proc1", "proc2", lowerID.Name()}
		if !reflect.DeepEqual(names, expected) {
			t.Errorf("expected names %v, got %v", expected, names)
		}
	})

	t.Run("insert before non-existent processor", func(t *testing.T) {
		seq := NewSequence[string](testSequence)
		seq.Register(Transform(upperID, func(_ context.Context, s string) string { return strings.ToUpper(s) }))

		nonExistentID := NewIdentity("nonexistent", "")
		err := seq.Before(nonExistentID, Transform(lowerID, func(_ context.Context, s string) string { return strings.ToLower(s) }))
		if err == nil {
			t.Error("expected error for non-existent processor")
		}
		if !strings.Contains(err.Error(), "nonexistent") {
			t.Errorf("expected error to mention 'nonexistent', got %q", err.Error())
		}
	})
}

func TestSequenceNameBasedOperationsConcurrency(t *testing.T) {
	t.Run("concurrent modifications", func(t *testing.T) {
		seq := NewSequence[int](testSequence)
		seq.Register(
			Transform(incrementID, func(_ context.Context, n int) int { return n + 1 }),
			Transform(doubleID, func(_ context.Context, n int) int { return n * 2 }),
			Transform(addTen, func(_ context.Context, n int) int { return n + 10 }),
		)

		var wg sync.WaitGroup

		// Concurrent removals
		wg.Add(2)
		go func() {
			defer wg.Done()
			_ = seq.Remove(doubleID) //nolint:errcheck // Testing race conditions
		}()
		go func() {
			defer wg.Done()
			_ = seq.Remove(addTen) //nolint:errcheck // Testing race conditions
		}()

		// Concurrent insertions
		wg.Add(2)
		go func() {
			defer wg.Done()
			newProc := Transform(squareID, func(_ context.Context, n int) int { return n * n })
			_ = seq.After(incrementID, newProc) //nolint:errcheck // Testing race conditions
		}()
		go func() {
			defer wg.Done()
			newProc := Transform(NewIdentity("multiply_5", ""), func(_ context.Context, n int) int { return n * 5 })
			_ = seq.Before(incrementID, newProc) //nolint:errcheck // Testing race conditions
		}()

		// Concurrent replacements
		wg.Add(1)
		go func() {
			defer wg.Done()
			newProc := Transform(incrementID, func(_ context.Context, n int) int { return n + 100 })
			_ = seq.Replace(incrementID, newProc) //nolint:errcheck // Testing race conditions
		}()

		wg.Wait()

		// Verify sequence is still functional (any result is valid due to race conditions)
		_, err := seq.Process(context.Background(), 1)
		if err != nil {
			t.Errorf("sequence should remain functional after concurrent modifications: %v", err)
		}
	})

	t.Run("Sequence panic recovery", func(t *testing.T) {
		// Create a sequence where one processor panics
		seq := NewSequence(NewIdentity("panic_sequence", ""),
			Transform(NewIdentity("step1", ""), func(_ context.Context, s string) string { return s + "_step1" }),
			Transform(NewIdentity("panic_step", ""), func(_ context.Context, _ string) string { panic("sequence panic") }),
			Transform(NewIdentity("step3", ""), func(_ context.Context, s string) string { return s + "_step3" }), // Never reached
		)

		result, err := seq.Process(context.Background(), "start")

		if result != "" {
			t.Errorf("expected empty string, got %q", result)
		}

		var pipzErr *Error[string]
		if !errors.As(err, &pipzErr) {
			t.Fatal("expected pipz.Error")
		}

		// Path should include both sequence name and processor name
		expectedPath := []string{"panic_sequence", "panic_step"}
		if len(pipzErr.Path) != 2 {
			t.Errorf("expected path length 2, got %d: %v", len(pipzErr.Path), pipzErr.Path)
		}
		if pipzErr.Path[0].Name() != expectedPath[0] || pipzErr.Path[1].Name() != expectedPath[1] {
			t.Errorf("expected path %v, got %v", expectedPath, pipzErr.Path)
		}

		// The input data will be the transformed value from step1
		if pipzErr.InputData != "start_step1" {
			t.Errorf("expected input data 'start_step1', got %q", pipzErr.InputData)
		}
	})

}

func TestSequenceSignals(t *testing.T) {
	t.Run("Emits Completed Signal On Success", func(t *testing.T) {
		var signalName string
		var signalProcessorCount int
		var signalDuration float64

		listener := capitan.Hook(SignalSequenceCompleted, func(_ context.Context, e *capitan.Event) {
			signalName, _ = FieldName.From(e)
			signalProcessorCount, _ = FieldProcessorCount.From(e)
			signalDuration, _ = FieldDuration.From(e)
		})
		defer listener.Close()

		seq := NewSequence[int](NewIdentity("signal-test-seq", ""),
			Transform(NewIdentity("double", ""), func(_ context.Context, n int) int { return n * 2 }),
			Transform(NewIdentity("add-one", ""), func(_ context.Context, n int) int { return n + 1 }),
		)

		result, err := seq.Process(context.Background(), 5)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != 11 {
			t.Errorf("expected 11, got %d", result)
		}

		if err := listener.Drain(context.Background()); err != nil {
			t.Fatalf("drain failed: %v", err)
		}

		if signalName != "signal-test-seq" {
			t.Errorf("expected name 'signal-test-seq', got %q", signalName)
		}
		if signalProcessorCount != 2 {
			t.Errorf("expected processor_count 2, got %d", signalProcessorCount)
		}
		if signalDuration <= 0 {
			t.Error("expected positive duration")
		}
	})

	t.Run("Does Not Emit Signal On Failure", func(t *testing.T) {
		var signalReceived bool

		listener := capitan.Hook(SignalSequenceCompleted, func(_ context.Context, _ *capitan.Event) {
			signalReceived = true
		})
		defer listener.Close()

		seq := NewSequence[int](NewIdentity("signal-fail-seq", ""),
			Apply(NewIdentity("fail", ""), func(_ context.Context, _ int) (int, error) {
				return 0, errors.New("intentional failure")
			}),
		)

		_, err := seq.Process(context.Background(), 5)

		if err == nil {
			t.Fatal("expected error")
		}

		if err := listener.Drain(context.Background()); err != nil {
			t.Fatalf("drain failed: %v", err)
		}

		if signalReceived {
			t.Error("signal should not be emitted on failure")
		}
	})
}

// trackingProcessor tracks Close() calls for testing.
type trackingProcessor[T any] struct {
	identity   Identity
	closeCalls int32
	closeErr   error
}

func newTrackingProcessor[T any](identity Identity) *trackingProcessor[T] {
	return &trackingProcessor[T]{identity: identity}
}

func (*trackingProcessor[T]) Process(_ context.Context, data T) (T, error) {
	return data, nil
}

func (t *trackingProcessor[T]) Identity() Identity {
	return t.identity
}

func (t *trackingProcessor[T]) Schema() Node {
	return Node{
		Identity: t.identity,
		Type:     "tracking_processor",
	}
}

func (t *trackingProcessor[T]) Close() error {
	t.closeCalls++
	return t.closeErr
}

func (t *trackingProcessor[T]) CloseCalls() int {
	return int(t.closeCalls)
}

func (t *trackingProcessor[T]) WithCloseError(err error) *trackingProcessor[T] {
	t.closeErr = err
	return t
}

func TestSequenceClose(t *testing.T) {
	t.Run("Closes All Children", func(t *testing.T) {
		p1 := newTrackingProcessor[int](NewIdentity("p1", ""))
		p2 := newTrackingProcessor[int](NewIdentity("p2", ""))
		p3 := newTrackingProcessor[int](NewIdentity("p3", ""))

		seq := NewSequence(NewIdentity("test", ""), p1, p2, p3)
		err := seq.Close()

		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if p1.CloseCalls() != 1 {
			t.Errorf("p1: expected 1 close call, got %d", p1.CloseCalls())
		}
		if p2.CloseCalls() != 1 {
			t.Errorf("p2: expected 1 close call, got %d", p2.CloseCalls())
		}
		if p3.CloseCalls() != 1 {
			t.Errorf("p3: expected 1 close call, got %d", p3.CloseCalls())
		}
	})

	t.Run("Aggregates Errors", func(t *testing.T) {
		p1 := newTrackingProcessor[int](NewIdentity("p1", "")).WithCloseError(errors.New("p1 error"))
		p2 := newTrackingProcessor[int](NewIdentity("p2", ""))
		p3 := newTrackingProcessor[int](NewIdentity("p3", "")).WithCloseError(errors.New("p3 error"))

		seq := NewSequence(NewIdentity("test", ""), p1, p2, p3)
		err := seq.Close()

		if err == nil {
			t.Error("expected error, got nil")
		}
		// All should still be closed even with errors
		if p1.CloseCalls() != 1 || p2.CloseCalls() != 1 || p3.CloseCalls() != 1 {
			t.Error("expected all processors to be closed")
		}
	})

	t.Run("Empty Sequence", func(t *testing.T) {
		seq := NewSequence[int](NewIdentity("empty", ""))
		err := seq.Close()
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})

	t.Run("Idempotency", func(t *testing.T) {
		p := newTrackingProcessor[int](NewIdentity("p", ""))
		seq := NewSequence(NewIdentity("test", ""), p)

		// Call Close multiple times
		err1 := seq.Close()
		err2 := seq.Close()
		err3 := seq.Close()

		if err1 != nil || err2 != nil || err3 != nil {
			t.Error("expected no errors")
		}
		if p.CloseCalls() != 1 {
			t.Errorf("expected 1 close call, got %d", p.CloseCalls())
		}
	})

	t.Run("Idempotency With Error", func(t *testing.T) {
		p := newTrackingProcessor[int](NewIdentity("p", "")).WithCloseError(errors.New("close error"))
		seq := NewSequence(NewIdentity("test", ""), p)

		err1 := seq.Close()
		err2 := seq.Close()

		// Both calls should return the same error
		if err1 == nil || err2 == nil {
			t.Error("expected error on both calls")
		}
		if err1.Error() != err2.Error() {
			t.Error("expected same error on both calls")
		}
		if p.CloseCalls() != 1 {
			t.Errorf("expected 1 close call, got %d", p.CloseCalls())
		}
	})

	t.Run("Nested Close", func(t *testing.T) {
		// Test that Close propagates through nested structures
		inner1 := newTrackingProcessor[int](NewIdentity("inner1", ""))
		inner2 := newTrackingProcessor[int](NewIdentity("inner2", ""))
		innerSeq := NewSequence(NewIdentity("inner", ""), inner1, inner2)

		outer1 := newTrackingProcessor[int](NewIdentity("outer1", ""))
		outerSeq := NewSequence[int](NewIdentity("outer", ""), outer1, innerSeq)

		err := outerSeq.Close()

		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if outer1.CloseCalls() != 1 {
			t.Errorf("outer1: expected 1 close call, got %d", outer1.CloseCalls())
		}
		if inner1.CloseCalls() != 1 {
			t.Errorf("inner1: expected 1 close call, got %d", inner1.CloseCalls())
		}
		if inner2.CloseCalls() != 1 {
			t.Errorf("inner2: expected 1 close call, got %d", inner2.CloseCalls())
		}
	})
}
