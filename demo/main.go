// Package main provides interactive demonstrations of pipz capabilities.
// The demos showcase real-world use cases and best practices for building
// type-safe, discoverable processing pipelines.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	// interactive enables typewriter effects and pauses between sections
	interactive bool
	
	// rootCmd is the base command for all demos
	rootCmd = &cobra.Command{
		Use:   "pipz-demo",
		Short: "Interactive demonstrations of pipz capabilities",
		Long: `
╔═══════════════════════════════════════════════════════════════════════════════╗
║                          🚀 PIPZ CAPABILITY DEMOS                             ║
║                                                                               ║
║  Build type-safe processing pipelines in Go that you can retrieve from       ║
║  anywhere in your codebase using just the types.                             ║
╚═══════════════════════════════════════════════════════════════════════════════╝

These interactive demonstrations showcase real-world use cases and the power
of type-based pipeline discovery.`,
	}
)

// main is the entry point for the demo application.
// It executes the root command and handles any errors.
func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// init sets up the root command and global flags.
func init() {
	// Disable default completion command
	rootCmd.CompletionOptions.DisableDefaultCmd = true
	
	// Add interactive flag
	rootCmd.PersistentFlags().BoolVarP(&interactive, "interactive", "i", false, "Enable interactive mode with typewriter effect and pauses")
}