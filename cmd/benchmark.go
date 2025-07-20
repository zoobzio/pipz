package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
)

var benchmarkCmd = &cobra.Command{
	Use:   "benchmark [example]",
	Short: "Run performance benchmarks",
	Long: `Run performance benchmarks for pipz examples.

When run without arguments, runs benchmarks for all examples.
When run with an example name, runs benchmarks for that specific example.

Available examples:
  validation  Business rule validation benchmarks
  security    Security pipeline benchmarks
  etl         ETL processing benchmarks
  webhook     Webhook processing benchmarks
  payment     Payment processing benchmarks`,
	RunE: func(cmd *cobra.Command, args []string) error {
		example := ""
		if len(args) > 0 {
			example = args[0]
		}

		return runBenchmarks(example)
	},
}

func runBenchmarks(example string) error {
	if example == "" {
		// Run all benchmarks
		fmt.Println("Running all benchmarks...")
		return runAllBenchmarks()
	}

	// Run specific example benchmark
	examplePath := filepath.Join("..", "examples", example)
	if _, err := os.Stat(examplePath); os.IsNotExist(err) {
		return fmt.Errorf("example '%s' not found", example)
	}

	fmt.Printf("Running %s benchmarks...\n", example)
	cmd := exec.Command("go", "test", "-bench=.", "-benchmem", "-benchtime=10s")
	cmd.Dir = examplePath
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func runAllBenchmarks() error {
	examples := []string{"validation", "security", "etl", "webhook", "payment", "events", "ai", "middleware", "workflow", "moderation"}
	
	for _, ex := range examples {
		examplePath := filepath.Join("..", "examples", ex)
		if _, err := os.Stat(examplePath); os.IsNotExist(err) {
			continue
		}

		fmt.Printf("\n=== Running %s benchmarks ===\n", ex)
		cmd := exec.Command("go", "test", "-bench=.", "-benchmem", "-benchtime=1s")
		cmd.Dir = examplePath
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		
		if err := cmd.Run(); err != nil {
			fmt.Printf("Warning: %s benchmarks failed: %v\n", ex, err)
		}
	}
	
	return nil
}