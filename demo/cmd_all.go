package main

import (
	"fmt"
	"github.com/spf13/cobra"
	"pipz/demo/constants"
	"pipz/demo/testutil"
	"time"
)

// allCmd runs all demonstrations in sequence.
var allCmd = &cobra.Command{
	Use:   "all",
	Short: "Run all demonstrations in sequence",
	Long:  `Runs through all pipz capability demonstrations with brief pauses between each.`,
	Run:   runAllDemos,
}

func init() {
	rootCmd.AddCommand(allCmd)
}

// demoInfo holds metadata about each demo.
type demoInfo struct {
	name string
	desc string
	cmd  *cobra.Command
}

// runAllDemos executes all registered demonstrations in sequence.
// It provides an overview of pipz concepts before diving into specific use cases.
func runAllDemos(cmd *cobra.Command, args []string) {
	pp := testutil.NewPrettyPrinter()
	
	// Get interactive flag from parent command
	interactive, _ := cmd.Flags().GetBool("interactive")
	pp.SetInteractive(interactive)
	
	// Define all demos with their metadata
	demos := []demoInfo{
		{
			"Security Audit Pipeline", 
			"Healthcare data access with automatic redaction",
			securityCmd,
		},
		{
			"Data Transformation Pipeline",
			"CSV to database conversion with validation",
			transformCmd,
		},
		{
			"Multi-tenant Type Universes", 
			"Complete isolation using type-based namespaces",
			universesCmd,
		},
		{
			"Data Validation Pipeline",
			"Complex order validation with composable rules",
			validationCmd,
		},
		{
			"Multi-Stage Workflow",
			"User registration with validation, enrichment, and persistence",
			workflowCmd,
		},
		{
			"Request Middleware",
			"API gateway with auth, rate limiting, and CORS",
			middlewareCmd,
		},
		{
			"Error Handling Pipelines",
			"Sophisticated error recovery using pipelines",
			errorsCmd,
		},
		{
			"Pipeline Versioning",
			"A/B/C testing with type universe isolation",
			versioningCmd,
		},
		{
			"Testing Without Mocks",
			"Elegant testing using type universes",
			testingCmd,
		},
	}
	
	// Introduction
	pp.Section("🚀 PIPZ INTERACTIVE DEMONSTRATIONS")
	
	pp.SubSection("What is pipz?")
	pp.Info("pipz lets you build processing pipelines that are:")
	pp.Success("  ✓ Discoverable from anywhere using just types")
	pp.Success("  ✓ Type-safe with compile-time guarantees")
	pp.Success("  ✓ Zero dependencies - no DI needed")
	pp.Success("  ✓ Globally accessible without singletons")
	
	pp.SubSection("How It Works")
	pp.Code("go", `// 1. Register a pipeline ONCE
const myKey KeyType = "v1"
contract := pipz.GetContract[DataType](myKey)
contract.Register(step1, step2, step3)

// 2. Access from ANYWHERE using just types
contract := pipz.GetContract[DataType](myKey)
result, err := contract.Process(data)`)
	
	pp.Info("")
	pp.Info("Let's see it in action with real use cases...")
	time.Sleep(constants.DemoSleepMedium)
	
	// Run each demo
	for i, demo := range demos {
		if i > 0 {
			fmt.Println()
			pp.WaitForEnter("Press Enter to continue to the next demo...")
			fmt.Print(constants.ClearScreen) // Clear screen between demos
		}
		
		pp.Info("")
		pp.Info(fmt.Sprintf("📍 Demo %d of %d: %s", i+1, len(demos), demo.name))
		pp.Info(fmt.Sprintf("   %s", demo.desc))
		time.Sleep(constants.DemoSleepShort)
		
		// Run the demo command
		demo.cmd.Run(cmd, args)
	}
	
	// Conclusion
	fmt.Println()
	pp.Section("✨ DEMONSTRATIONS COMPLETE")
	
	pp.SubSection("What You've Learned")
	pp.Feature("🔐", "Security Pipelines", "Automatic data redaction and audit trails")
	pp.Feature("🌌", "Type Universes", "Multi-tenant isolation without configuration")
	pp.Feature("🔍", "Type Discovery", "Find pipelines using only type information")
	pp.Feature("📦", "Zero Dependencies", "No DI containers or global state")
	
	pp.SubSection("Next Steps")
	pp.Info("1. Explore individual demos in detail:")
	pp.Code("shell", `pipz-demo security    # Deep dive into security patterns
pipz-demo universes   # Explore multi-tenant architectures`)
	
	pp.Info("")
	pp.Info("2. Try pipz in your own project:")
	pp.Code("shell", `go get github.com/your-org/pipz`)
	
	pp.Info("")
	pp.Success("Happy pipeline building! 🚀")
}