package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var (
	demoAll bool

	demoCmd = &cobra.Command{
		Use:   "demo [example]",
		Short: "Run interactive demonstrations",
		Long: `Run interactive demonstrations of pipz examples.

When run without arguments, displays an interactive menu.
When run with an example name, runs that specific demo.

Available examples:
  validation  Data validation with composable validators
  security    Security audit with permission-based redaction
  transform   ETL transformation pipeline (coming soon)
  middleware  Request middleware composition (coming soon)
  payment     Payment processing with error recovery (coming soon)`,
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) != 0 {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}

			var completions []string
			for _, ex := range getAllExamples() {
				if strings.HasPrefix(ex.Name(), toComplete) {
					completions = append(completions, ex.Name())
				}
			}
			return completions, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			example := ""
			if len(args) > 0 {
				example = args[0]
			}

			return runDemo(example, demoAll)
		},
	}
)

func init() {
	demoCmd.Flags().BoolVar(&demoAll, "all", false, "Run all demos sequentially")
}

// ANSI color codes
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorPurple = "\033[35m"
	colorCyan   = "\033[36m"
	colorGray   = "\033[37m"
	colorWhite  = "\033[97m"
)

// Demo manages the interactive demo experience
type Demo struct {
	reader *bufio.Reader
	ctx    context.Context
}

// runDemo runs a demo based on the example name
func runDemo(example string, all bool) error {
	demo := &Demo{
		reader: bufio.NewReader(os.Stdin),
		ctx:    context.Background(),
	}

	if all {
		return demo.runAllDemos()
	}

	if example == "" {
		return demo.runInteractiveMenu()
	}

	ex, ok := getExampleByName(example)
	if !ok {
		return fmt.Errorf("unknown example: %s\n\nRun 'pipz list' to see available examples", example)
	}

	return ex.Demo(demo.ctx)
}

func (d *Demo) runInteractiveMenu() error {
	d.printWelcome()

	for {
		d.printMainMenu()

		choice := d.readInput("\nSelect an option: ")

		switch choice {
		case "1":
			d.showConcepts()
		case "2":
			d.runExamplesMenu()
		case "3":
			d.showFeatures()
		case "4":
			d.showQuickStart()
		case "q", "quit", "exit":
			fmt.Println(colorGreen + "\nThanks for exploring pipz! 🚀" + colorReset)
			return nil
		default:
			fmt.Println(colorRed + "Invalid option. Please try again." + colorReset)
		}
	}
}

func (d *Demo) printWelcome() {
	fmt.Println(colorCyan + `
 ____  ___ ____  ____
|  _ \|_ _|  _ \|_  /
| |_) || || |_) |/ / 
|  __/ | ||  __// /_ 
|_|   |___|_|  /____|
` + colorReset)

	fmt.Println(colorWhite + "Welcome to the pipz Interactive Demo!" + colorReset)
	fmt.Println(colorGray + "Explore composable data pipelines through hands-on examples.\n" + colorReset)
}

func (d *Demo) printMainMenu() {
	fmt.Println("\n" + colorYellow + "═══ MAIN MENU ═══" + colorReset)
	fmt.Println(colorWhite + "1." + colorReset + " Learn Core Concepts")
	fmt.Println(colorWhite + "2." + colorReset + " Run Interactive Examples")
	fmt.Println(colorWhite + "3." + colorReset + " Explore Features")
	fmt.Println(colorWhite + "4." + colorReset + " Quick Start Guide")
	fmt.Println(colorWhite + "q." + colorReset + " Quit")
}

func (d *Demo) showConcepts() {
	fmt.Println("\n" + colorCyan + "═══ CORE CONCEPTS ═══" + colorReset)

	concepts := []struct {
		name string
		desc string
	}{
		{
			"Processor",
			"The atomic unit - a named function that transforms data",
		},
		{
			"Pipeline",
			"An ordered sequence of processors that execute sequentially",
		},
		{
			"Chain",
			"Composes multiple pipelines into larger workflows",
		},
		{
			"Context Support",
			"Built-in cancellation and timeout handling",
		},
		{
			"Error Context",
			"Rich error information including processor name and stage",
		},
	}

	for _, c := range concepts {
		fmt.Printf("\n%s%s%s\n", colorWhite, c.name, colorReset)
		fmt.Printf("  %s\n", c.desc)
		time.Sleep(500 * time.Millisecond) // Dramatic effect
	}

	d.waitForEnter()
}

func (d *Demo) runExamplesMenu() {
	for {
		d.printExamplesMenu()

		choice := d.readInput("\nSelect an example: ")

		examples := getAllExamples()

		// Check numeric choices
		for i, ex := range examples {
			if choice == fmt.Sprintf("%d", i+1) {
				if err := ex.Demo(d.ctx); err != nil {
					fmt.Printf("%sError: %v%s\n", colorRed, err, colorReset)
				}
				break
			}
		}

		// Check other choices
		switch choice {
		case "b", "back":
			return
		default:
			if choice != "1" && choice != "2" { // Already handled above
				fmt.Println(colorRed + "Invalid option. Please try again." + colorReset)
			}
		}
	}
}

func (d *Demo) printExamplesMenu() {
	fmt.Println("\n" + colorYellow + "═══ INTERACTIVE EXAMPLES ═══" + colorReset)

	examples := getAllExamples()
	for i, ex := range examples {
		fmt.Printf("%s%d.%s %s - %s\n", colorWhite, i+1, colorReset, ex.Name(), ex.Description())
	}

	// Show placeholders for upcoming examples
	upcomingExamples := []struct {
		name string
		desc string
	}{
		{"transform", "ETL transformation with validation"},
		{"middleware", "Request middleware composition"},
		{"payment", "Payment processing with error recovery"},
	}

	startIdx := len(examples) + 1
	for i, ex := range upcomingExamples {
		fmt.Printf("%s%d.%s %s - %s %s(coming soon)%s\n",
			colorGray, startIdx+i, colorReset, ex.name, ex.desc,
			colorYellow, colorReset)
	}

	fmt.Printf("%sb.%s Back to main menu\n", colorWhite, colorReset)
}

func (d *Demo) showFeatures() {
	fmt.Println("\n" + colorCyan + "═══ KEY FEATURES ═══" + colorReset)

	// First show processor types
	fmt.Println("\n" + colorWhite + "Processor Types:" + colorReset)
	processors := []struct {
		name      string
		desc      string
		signature string
	}{
		{
			"Transform",
			"Modifies data and returns the result",
			"func(ctx, T) (T, error)",
		},
		{
			"Validate",
			"Checks data validity without modification",
			"func(ctx, T) error",
		},
		{
			"Effect",
			"Performs side effects (logging, metrics)",
			"func(ctx, T) error",
		},
		{
			"Enrich",
			"Adds data from external sources",
			"func(ctx, T) (T, error)",
		},
		{
			"Mutate",
			"Conditionally modifies data",
			"func(ctx, T) (T, error) + predicate",
		},
	}

	for _, p := range processors {
		fmt.Printf("\n  %s%s%s - %s\n", colorYellow, p.name, colorReset, p.desc)
		fmt.Printf("    %s%s%s\n", colorGray, p.signature, colorReset)
	}

	d.waitForEnter()

	// Then show other features
	fmt.Println("\n" + colorWhite + "Pipeline Features:" + colorReset)
	features := []struct {
		name string
		desc string
		code string
	}{
		{
			"Type Safety",
			"Compile-time type checking with generics",
			"Pipeline[T] only processes values of type T",
		},
		{
			"Context Support",
			"Built-in cancellation and timeout handling",
			"pipeline.Process(ctx, data)",
		},
		{
			"Dynamic Modification",
			"Modify pipelines at runtime",
			"pipeline.PushHead(debugProcessor)",
		},
		{
			"Error Context",
			"Rich error information for debugging",
			"PipelineError includes processor name & stage",
		},
		{
			"Zero Allocations",
			"Optimized for performance",
			"~20ns overhead per processor",
		},
		{
			"Composability",
			"Chain pipelines into workflows",
			"chain.Add(pipeline1.Link(), pipeline2.Link())",
		},
	}

	for _, f := range features {
		fmt.Printf("\n  %s%s%s\n", colorYellow, f.name, colorReset)
		fmt.Printf("    %s\n", f.desc)
		fmt.Printf("    %s%s%s\n", colorGray, f.code, colorReset)
		time.Sleep(300 * time.Millisecond)
	}

	d.waitForEnter()
}

func (d *Demo) showQuickStart() {
	fmt.Println("\n" + colorCyan + "═══ QUICK START ═══" + colorReset)
	fmt.Println("\n1. Install pipz:")
	fmt.Println(colorGray + "   go get github.com/zoobzio/pipz" + colorReset)

	fmt.Println("\n2. Create your first pipeline:")
	fmt.Println(colorGray + `   pipeline := pipz.NewPipeline[string]()
   pipeline.Register(
       pipz.Transform("upper", strings.ToUpper),
       pipz.Validate("not_empty", checkNotEmpty),
   )` + colorReset)

	fmt.Println("\n3. Process data:")
	fmt.Println(colorGray + `   result, err := pipeline.Process(ctx, "hello")` + colorReset)

	fmt.Println("\n4. Learn more:")
	fmt.Println("   • GitHub: " + colorBlue + "https://github.com/zoobzio/pipz" + colorReset)
	fmt.Println("   • Docs: " + colorBlue + "https://pkg.go.dev/github.com/zoobzio/pipz" + colorReset)

	d.waitForEnter()
}

func (d *Demo) runAllDemos() error {
	fmt.Println(colorCyan + "\n═══ RUNNING ALL DEMOS ═══" + colorReset)

	examples := getAllExamples()
	for i, ex := range examples {
		fmt.Printf("\n%s[%d/%d] Running %s demo...%s\n",
			colorYellow, i+1, len(examples), ex.Name(), colorReset)

		if err := ex.Demo(d.ctx); err != nil {
			fmt.Printf("%sError in %s demo: %v%s\n", colorRed, ex.Name(), err, colorReset)
			// Continue with other demos
		}

		if i < len(examples)-1 {
			d.waitForEnter()
		}
	}

	fmt.Println(colorGreen + "\n✅ All demos completed!" + colorReset)
	return nil
}

func (d *Demo) readInput(prompt string) string {
	fmt.Print(prompt)
	input, _ := d.reader.ReadString('\n')
	return strings.TrimSpace(strings.ToLower(input))
}

func (d *Demo) waitForEnter() {
	fmt.Print(colorGray + "\nPress Enter to continue..." + colorReset)
	d.reader.ReadString('\n')
}
