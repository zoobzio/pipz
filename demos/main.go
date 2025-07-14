package main

import (
	"fmt"
	"os"
	"strings"
)

// Demo represents a runnable demonstration
type Demo struct {
	Name        string
	Description string
	Run         func()
}

var demos []Demo

func init() {
	demos = []Demo{
		{
			Name:        "validation",
			Description: "Data validation pipeline for e-commerce orders",
			Run:         runValidationDemo,
		},
		{
			Name:        "payment",
			Description: "Payment processing with fallback providers",
			Run:         runPaymentDemo,
		},
		{
			Name:        "security",
			Description: "Security audit pipeline with data redaction",
			Run:         runSecurityDemo,
		},
		{
			Name:        "transform",
			Description: "ETL pipeline for CSV to database transformation",
			Run:         runTransformDemo,
		},
		{
			Name:        "composability",
			Description: "Composing multiple pipelines together",
			Run:         runComposabilityDemo,
		},
		{
			Name:        "errors",
			Description: "Error handling and recovery strategies",
			Run:         runErrorDemo,
		},
		{
			Name:        "dynamic",
			Description: "Dynamic pipeline modification and runtime optimization",
			Run:         runDynamicPipelineDemo,
		},
		{
			Name:        "all",
			Description: "Run all demonstrations",
			Run:         runAllDemos,
		},
	}
}

func main() {
	printHeader()

	if len(os.Args) < 2 {
		showUsage()
		os.Exit(0)
	}

	demoName := os.Args[1]
	
	for _, demo := range demos {
		if demo.Name == demoName {
			fmt.Printf("\n🚀 Running %s demo...\n", demo.Name)
			fmt.Println(strings.Repeat("=", 60))
			demo.Run()
			return
		}
	}

	fmt.Printf("❌ Unknown demo: %s\n", demoName)
	showUsage()
	os.Exit(1)
}

func printHeader() {
	fmt.Println(`
╔═══════════════════════════════════════════════════════════════════════════════╗
║                          🚀 PIPZ CAPABILITY DEMOS                             ║
║                                                                               ║
║  Build type-safe processing pipelines in Go with zero serialization overhead. ║
╚═══════════════════════════════════════════════════════════════════════════════╝`)
}

func showUsage() {
	fmt.Println("\nUsage: go run main.go <demo-name>")
	fmt.Println("\nAvailable demos:")
	for _, demo := range demos {
		fmt.Printf("  %-15s - %s\n", demo.Name, demo.Description)
	}
	fmt.Println("\nExample: go run main.go validation")
}

func runAllDemos() {
	for i, demo := range demos {
		if demo.Name == "all" {
			continue
		}
		if i > 0 {
			fmt.Println("\n" + strings.Repeat("-", 60) + "\n")
		}
		fmt.Printf("📌 Demo %d/%d: %s\n", i+1, len(demos)-1, demo.Description)
		demo.Run()
	}
}

// Helper functions for demo output
func section(title string) {
	fmt.Printf("\n✨ %s\n", title)
	fmt.Println(strings.Repeat("-", 40))
}

func info(msg string) {
	fmt.Printf("ℹ️  %s\n", msg)
}

func success(msg string) {
	fmt.Printf("✅ %s\n", msg)
}

func showError(msg string) {
	fmt.Printf("❌ %s\n", msg)
}

func code(lang, content string) {
	fmt.Printf("\n```%s\n%s\n```\n", lang, content)
}