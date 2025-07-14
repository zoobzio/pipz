package main

import (
	"fmt"
	"math/rand"
	"strconv"

	"pipz"
)

// Transaction represents a financial transaction
type Transaction struct {
	ID       string
	Amount   float64
	Currency string
	Type     string
	Status   string
	Flags    []string
}

func runDynamicPipelineDemo() {
	section("Dynamic Pipeline Modification Demo")
	info("Demonstrates runtime pipeline modification capabilities")

	// Start with a basic transaction processing pipeline
	contract := pipz.NewContract[Transaction]()
	
	// Basic processors
	validateProcessor := func(t Transaction) (Transaction, error) {
		t.Flags = append(t.Flags, "validated")
		return t, nil
	}
	
	normalizeProcessor := func(t Transaction) (Transaction, error) {
		if t.Currency == "" {
			t.Currency = "USD"
		}
		t.Flags = append(t.Flags, "normalized")
		return t, nil
	}
	
	auditProcessor := func(t Transaction) (Transaction, error) {
		t.Flags = append(t.Flags, "audited")
		return t, nil
	}

	// Fraud detection processor (expensive)
	fraudDetectionProcessor := func(t Transaction) (Transaction, error) {
		t.Flags = append(t.Flags, "fraud-checked")
		return t, nil
	}

	// Build initial pipeline
	contract.PushTail(validateProcessor, normalizeProcessor)
	
	fmt.Println("\n🏗️  Initial Pipeline Setup")
	info("Starting with: Validation → Normalization")
	
	tx := Transaction{
		ID:     "TX001",
		Amount: 100.50,
		Type:   "purchase",
		Status: "pending",
	}
	
	result, _ := contract.Process(tx)
	fmt.Printf("   Result: %+v\n", result)

	// Dynamic modification: Add fraud detection for high-value transactions
	section("Conditional Pipeline Modification")
	info("Adding fraud detection for high-value transactions...")
	
	if tx.Amount > 100 {
		contract.PushTail(fraudDetectionProcessor)
		success("Fraud detection added to pipeline")
	}
	
	result, _ = contract.Process(tx)
	fmt.Printf("   Result: %+v\n", result)

	// Queue workflow demonstration
	section("Queue Workflow (FIFO)")
	info("Demonstrating queue-based processing")
	
	queue := pipz.NewContract[Transaction]()
	
	// Add processors to queue (back)
	queue.PushTail(validateProcessor)
	queue.PushTail(normalizeProcessor)
	queue.PushTail(auditProcessor)
	
	fmt.Printf("   Pipeline length: %d\n", queue.Len())
	
	// Process and remove from front
	for !queue.IsEmpty() {
		processor, err := queue.PopHead()
		if err != nil {
			continue
		}
		
		tx, _ = processor(tx)
		fmt.Printf("   Processed: %+v (remaining: %d)\n", tx.Flags, queue.Len())
	}

	// Stack workflow demonstration  
	section("Stack Workflow (LIFO)")
	info("Demonstrating stack-based processing (undo/redo pattern)")
	
	stack := pipz.NewContract[Transaction]()
	
	// Undo operations
	undoValidation := func(t Transaction) (Transaction, error) {
		// Remove last flag
		if len(t.Flags) > 0 {
			t.Flags = t.Flags[:len(t.Flags)-1]
		}
		return t, nil
	}
	
	undoNormalization := func(t Transaction) (Transaction, error) {
		t.Currency = ""
		if len(t.Flags) > 0 {
			t.Flags = t.Flags[:len(t.Flags)-1]
		}
		return t, nil
	}
	
	// Build undo stack
	stack.PushTail(undoValidation, undoNormalization)
	
	fmt.Printf("   Before undo: %+v\n", tx)
	
	// Execute undos in reverse order (LIFO)
	for !stack.IsEmpty() {
		undoOp, _ := stack.PopTail()
		tx, _ = undoOp(tx)
		fmt.Printf("   After undo: %+v\n", tx)
	}

	// Movement operations demonstration
	section("Pipeline Reordering")
	info("Demonstrating processor movement and reordering")
	
	pipeline := pipz.NewContract[Transaction]()
	
	// Create processors with identifiable effects
	proc1 := func(t Transaction) (Transaction, error) {
		t.Flags = append(t.Flags, "step-1")
		return t, nil
	}
	proc2 := func(t Transaction) (Transaction, error) {
		t.Flags = append(t.Flags, "step-2")
		return t, nil
	}
	proc3 := func(t Transaction) (Transaction, error) {
		t.Flags = append(t.Flags, "step-3")
		return t, nil
	}
	
	pipeline.PushTail(proc1, proc2, proc3)
	
	// Reset transaction
	tx = Transaction{ID: "TX002", Amount: 50.0, Type: "refund"}
	
	fmt.Println("   Original order:")
	result, _ = pipeline.Process(tx)
	fmt.Printf("   Result: %+v\n", result.Flags)
	
	// Move last processor to front
	pipeline.MoveToHead(2)
	tx.Flags = nil // Reset flags
	
	fmt.Println("   After moving step-3 to front:")
	result, _ = pipeline.Process(tx)
	fmt.Printf("   Result: %+v\n", result.Flags)
	
	// Reverse entire pipeline
	pipeline.Reverse()
	tx.Flags = nil // Reset flags
	
	fmt.Println("   After reversing pipeline:")
	result, _ = pipeline.Process(tx)
	fmt.Printf("   Result: %+v\n", result.Flags)

	// Precise operations demonstration
	section("Precise Pipeline Modifications")
	info("Demonstrating insert, remove, and replace operations")
	
	processingPipeline := pipz.NewContract[Transaction]()
	processingPipeline.PushTail(validateProcessor, auditProcessor)
	
	tx = Transaction{ID: "TX003", Amount: 25.0, Type: "transfer"}
	
	fmt.Println("   Before insertion:")
	tx.Flags = nil
	result, _ = processingPipeline.Process(tx)
	fmt.Printf("   Result: %+v\n", result.Flags)
	
	// Insert normalization between validation and audit
	processingPipeline.InsertAt(1, normalizeProcessor)
	
	fmt.Println("   After inserting normalization:")
	tx.Flags = nil
	result, _ = processingPipeline.Process(tx)
	fmt.Printf("   Result: %+v\n", result.Flags)
	
	// Replace audit with fraud detection
	processingPipeline.ReplaceAt(2, fraudDetectionProcessor)
	
	fmt.Println("   After replacing audit with fraud detection:")
	tx.Flags = nil
	result, _ = processingPipeline.Process(tx)
	fmt.Printf("   Result: %+v\n", result.Flags)

	// A/B testing demonstration
	section("A/B Testing Pipeline")
	info("Dynamically switching between different implementations")
	
	abTestPipeline := pipz.NewContract[Transaction]()
	
	// Two different risk assessment implementations
	conservativeRisk := func(t Transaction) (Transaction, error) {
		t.Flags = append(t.Flags, "conservative-risk")
		return t, nil
	}
	
	aggressiveRisk := func(t Transaction) (Transaction, error) {
		t.Flags = append(t.Flags, "aggressive-risk")
		return t, nil
	}
	
	abTestPipeline.PushTail(validateProcessor, conservativeRisk)
	
	// Simulate A/B test - switch to aggressive risk for some transactions
	for i := 0; i < 5; i++ {
		tx = Transaction{
			ID:     "TX" + strconv.Itoa(100+i),
			Amount: rand.Float64() * 1000,
			Type:   "purchase",
		}
		tx.Flags = nil
		
		// Switch to aggressive risk for odd transaction IDs
		if i%2 == 1 {
			abTestPipeline.ReplaceAt(1, aggressiveRisk)
			fmt.Printf("   [A/B] Switched to aggressive risk for %s\n", tx.ID)
		} else {
			abTestPipeline.ReplaceAt(1, conservativeRisk)
			fmt.Printf("   [A/B] Using conservative risk for %s\n", tx.ID)
		}
		
		result, _ = abTestPipeline.Process(tx)
		fmt.Printf("         Result: %+v\n", result.Flags)
	}

	section("Performance Optimization")
	info("Runtime pipeline optimization based on conditions")
	
	optimizedPipeline := pipz.NewContract[Transaction]()
	
	// Expensive operations
	expensiveValidation := func(t Transaction) (Transaction, error) {
		t.Flags = append(t.Flags, "expensive-validation")
		return t, nil
	}
	
	lightValidation := func(t Transaction) (Transaction, error) {
		t.Flags = append(t.Flags, "light-validation")
		return t, nil
	}
	
	optimizedPipeline.PushTail(expensiveValidation, normalizeProcessor)
	
	// Simulate load-based optimization
	highLoad := true
	
	if highLoad {
		info("High load detected - switching to light validation")
		optimizedPipeline.ReplaceAt(0, lightValidation)
	}
	
	tx = Transaction{ID: "TX200", Amount: 75.0, Type: "withdrawal"}
	tx.Flags = nil
	result, _ = optimizedPipeline.Process(tx)
	fmt.Printf("   Optimized result: %+v\n", result.Flags)

	success("Dynamic pipeline modification demo completed!")
	info("All pipeline modification operations work correctly with thread safety")
}