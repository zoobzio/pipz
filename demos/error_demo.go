package main

import (
	"fmt"
	"time"

	"pipz"
)

// Types for error demo
type Task struct {
	ID          string
	Name        string
	Priority    int
	Status      string
	RetryCount  int
	LastError   string
	ProcessedAt time.Time
}

// Processors that can fail
func validateTask(t Task) (Task, error) {
	if t.ID == "" {
		return Task{}, fmt.Errorf("task ID required")
	}
	if t.Priority < 1 || t.Priority > 5 {
		return Task{}, fmt.Errorf("priority must be between 1-5, got %d", t.Priority)
	}
	return t, nil
}

func checkResources(t Task) (Task, error) {
	// Simulate resource check
	if t.Priority == 5 {
		// High priority tasks need special resources
		if time.Now().Second()%2 == 0 {
			return Task{}, fmt.Errorf("insufficient resources for high-priority task")
		}
	}
	return t, nil
}

func processTask(t Task) (Task, error) {
	// Simulate processing that might fail
	if t.Name == "unstable-task" {
		return Task{}, fmt.Errorf("task processing failed: unstable operation")
	}
	
	t.Status = "completed"
	t.ProcessedAt = time.Now()
	return t, nil
}

// Recovery strategies
func retryWithBackoff(t Task) Task {
	t.RetryCount++
	t.Status = "retrying"
	// In real system, would implement exponential backoff
	return t
}

func fallbackProcessor(t Task) Task {
	t.Status = "processed-fallback"
	t.ProcessedAt = time.Now()
	return t
}

func runErrorDemo() {
	section("ERROR HANDLING STRATEGIES")
	
	info("Use Case: Task Processing with Recovery")
	info("• Handle validation errors")
	info("• Recover from transient failures")
	info("• Implement retry strategies")
	info("• Provide fallback mechanisms")

	// Standard pipeline
	standardPipeline := pipz.NewContract[Task]()
	standardPipeline.Register(
		pipz.Apply(validateTask),
		pipz.Apply(checkResources),
		pipz.Apply(processTask),
	)

	// Pipeline with error recovery
	recoveryPipeline := pipz.NewContract[Task]()
	recoveryPipeline.Register(
		pipz.Apply(validateTask),
		// Conditional retry for resource errors
		pipz.Apply(func(t Task) (Task, error) {
			result, err := checkResources(t)
			if err != nil && t.RetryCount < 3 {
				t = retryWithBackoff(t)
				// Retry resource check
				return checkResources(t)
			}
			return result, err
		}),
		pipz.Apply(processTask),
	)

	code("go", `// Standard pipeline - fails fast
standard := pipz.NewContract[Task]()
standard.Register(
    pipz.Apply(validateTask),
    pipz.Apply(checkResources),
    pipz.Apply(processTask),
)

// Recovery pipeline - with retry logic
recovery := pipz.NewContract[Task]()
recovery.Register(
    pipz.Apply(validateTask),
    pipz.Apply(retryableResourceCheck),
    pipz.Apply(processTask),
)`)

	// Test Case 1: Validation error (unrecoverable)
	fmt.Println("\n🔧 Test Case 1: Validation Error")
	task1 := Task{
		ID:       "",  // Missing ID
		Name:     "test-task",
		Priority: 3,
	}

	_, err := standardPipeline.Process(task1)
	if err != nil {
		showError(fmt.Sprintf("Validation failed as expected: %v", err))
		info("→ Zero value returned (Go convention)")
	}

	// Test Case 2: Transient error (high priority)
	fmt.Println("\n🔧 Test Case 2: Transient Resource Error")
	task2 := Task{
		ID:       "TASK-002",
		Name:     "important-task",
		Priority: 5,  // High priority
	}

	result, err := standardPipeline.Process(task2)
	if err != nil {
		fmt.Printf("⚠️  First attempt failed: %v\n", err)
		
		// Manual retry with backoff
		task2 = retryWithBackoff(task2)
		time.Sleep(100 * time.Millisecond)
		
		result, err = standardPipeline.Process(task2)
		if err != nil {
			showError(fmt.Sprintf("Retry also failed: %v", err))
		} else {
			success(fmt.Sprintf("Succeeded on retry %d", task2.RetryCount))
		}
	} else {
		success("Processed on first attempt")
	}

	// Test Case 3: Processing error with fallback
	fmt.Println("\n🔧 Test Case 3: Processing Error with Fallback")
	task3 := Task{
		ID:       "TASK-003",
		Name:     "unstable-task",
		Priority: 2,
	}

	result, err = standardPipeline.Process(task3)
	if err != nil {
		fmt.Printf("⚠️  Standard processing failed: %v\n", err)
		
		// Use fallback pipeline
		fallbackPipeline := pipz.NewContract[Task]()
		fallbackPipeline.Register(
			pipz.Transform(fallbackProcessor),
		)
		
		result, err = fallbackPipeline.Process(task3)
		if err != nil {
			showError("Fallback also failed")
		} else {
			success(fmt.Sprintf("Processed via fallback: %s", result.Status))
		}
	}

	// Test Case 4: Error collection
	fmt.Println("\n🔧 Test Case 4: Error Collection Pattern")
	
	type TaskBatch struct {
		Tasks   []Task
		Results []Task
		Errors  []string
	}
	
	batch := TaskBatch{
		Tasks: []Task{
			{ID: "T1", Name: "good-task", Priority: 2},
			{ID: "", Name: "bad-task", Priority: 3},  // Will fail
			{ID: "T3", Name: "unstable-task", Priority: 1},  // Will fail
			{ID: "T4", Name: "another-good", Priority: 4},
		},
	}
	
	fmt.Println("   Processing batch of 4 tasks...")
	for i, task := range batch.Tasks {
		result, err := standardPipeline.Process(task)
		if err != nil {
			batch.Errors = append(batch.Errors, 
				fmt.Sprintf("Task %d (%s): %v", i+1, task.Name, err))
			fmt.Printf("   ❌ Task %d failed\n", i+1)
		} else {
			batch.Results = append(batch.Results, result)
			fmt.Printf("   ✅ Task %d succeeded\n", i+1)
		}
	}
	
	fmt.Printf("   Batch complete: %d succeeded, %d failed\n", 
		len(batch.Results), len(batch.Errors))

	fmt.Println("\n🔍 Error Handling Patterns:")
	info("• Fail-fast for unrecoverable errors")
	info("• Retry with backoff for transient failures")
	info("• Fallback pipelines for critical operations")
	info("• Batch processing with error collection")
	info("• Zero values on error (Go best practice)")
}