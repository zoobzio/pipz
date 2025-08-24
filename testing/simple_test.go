package testing

import (
	"context"
	"testing"

	"github.com/zoobzio/pipz"
)

// Simple test to verify the testing infrastructure works.
func TestSimpleInfrastructure(t *testing.T) {
	ctx := context.Background()

	// Test basic processor
	processor := pipz.Transform("double", func(_ context.Context, n int) int {
		return n * 2
	})

	result, err := processor.Process(ctx, 21)
	if err != nil {
		t.Fatalf("processor failed: %v", err)
	}

	if result != 42 {
		t.Errorf("expected 42, got %d", result)
	}

	// Test simple sequence
	seq := pipz.NewSequence("simple",
		pipz.Transform("add10", func(_ context.Context, n int) int { return n + 10 }),
		pipz.Transform("double", func(_ context.Context, n int) int { return n * 2 }),
	)

	result, err = seq.Process(ctx, 1)
	if err != nil {
		t.Fatalf("sequence failed: %v", err)
	}

	expected := 22 // (1 + 10) * 2
	if result != expected {
		t.Errorf("expected %d, got %d", expected, result)
	}
}

func TestHelpers(t *testing.T) {
	ctx := context.Background()

	// Test mock processor
	mock := NewMockProcessor[string](t, "mock-test")
	mock.WithReturn("mocked", nil)

	result, err := mock.Process(ctx, "input")
	if err != nil {
		t.Fatalf("mock failed: %v", err)
	}

	if result != "mocked" {
		t.Errorf("expected 'mocked', got %q", result)
	}

	// Test assertions
	AssertProcessed(t, mock, 1)
	AssertProcessedWith(t, mock, "input")

	// Test chaos processor with no chaos
	baseProcessor := pipz.Transform("base", func(_ context.Context, s string) string {
		return s + "_processed"
	})

	chaos := NewChaosProcessor("chaos", baseProcessor, ChaosConfig{
		FailureRate: 0.0, // No failures
		Seed:        12345,
	})

	result, err = chaos.Process(ctx, "test")
	if err != nil {
		t.Fatalf("chaos processor failed: %v", err)
	}

	if result != "test_processed" {
		t.Errorf("expected 'test_processed', got %q", result)
	}

	stats := chaos.Stats()
	if stats.TotalCalls != 1 {
		t.Errorf("expected 1 call, got %d", stats.TotalCalls)
	}
}
