package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	version = "0.5.1"
	rootCmd = &cobra.Command{
		Use:   "pipz",
		Short: "Composable data pipeline demos and benchmarks",
		Long: `pipz is a CLI tool for exploring composable data pipelines through
interactive demonstrations and performance benchmarks.

Learn about pipz features, run example implementations, and measure
performance characteristics of both core functionality and real-world use cases.`,
		Version: version,
	}
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	// Disable default completion command
	rootCmd.CompletionOptions.DisableDefaultCmd = true

	// Add commands
	rootCmd.AddCommand(demoCmd)
	rootCmd.AddCommand(benchmarkCmd)
	rootCmd.AddCommand(listCmd)
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all available examples",
	Long:  "Display a list of all available pipeline examples with descriptions.",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Available examples:")
		fmt.Println()
		for _, ex := range getAllExamples() {
			fmt.Printf("  %-12s %s\n", ex.Name(), ex.Description())
		}
	},
}
