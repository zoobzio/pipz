package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

var (
	benchAll  bool
	benchCore bool
	benchTime string

	benchmarkCmd = &cobra.Command{
		Use:     "benchmark [example]",
		Aliases: []string{"bench"},
		Short:   "Run performance benchmarks",
		Long: `Run performance benchmarks for pipz examples.

When run without arguments, displays an interactive menu.
When run with an example name, benchmarks that specific example.

Available examples:
  validation  Benchmark validation pipeline
  security    Benchmark security audit pipeline
  transform   Benchmark ETL pipeline (coming soon)
  middleware  Benchmark middleware pipeline (coming soon)
  payment     Benchmark payment pipeline (coming soon)

Special options:
  --core      Benchmark core pipz features only (no examples)
  --all       Run all benchmarks (core + examples)`,
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

			return runBenchmark(example, benchAll, benchCore, benchTime)
		},
	}
)

func init() {
	benchmarkCmd.Flags().BoolVar(&benchAll, "all", false, "Run all benchmarks")
	benchmarkCmd.Flags().BoolVar(&benchCore, "core", false, "Run core pipz benchmarks only")
	benchmarkCmd.Flags().StringVar(&benchTime, "time", "10s", "Benchmark duration per test")
}

func runBenchmark(example string, all, core bool, duration string) error {
	if all && core {
		return fmt.Errorf("cannot specify both --all and --core")
	}

	if example != "" && (all || core) {
		return fmt.Errorf("cannot specify example with --all or --core")
	}

	// Build the benchmark command
	args := []string{"test", "-bench", ".", "-benchtime", duration, "-run", "^$"}

	if core {
		fmt.Println(colorCyan + "\n═══ CORE PIPZ BENCHMARKS ═══" + colorReset)
		fmt.Println("Running performance tests for core pipz features...")

		// Run benchmarks in the root package
		cmd := exec.Command("go", append(args, "../")...)
		return runBenchmarkCommand(cmd)
	}

	if all {
		fmt.Println(colorCyan + "\n═══ ALL BENCHMARKS ═══" + colorReset)
		fmt.Println("Running all performance tests...")

		// Run benchmarks in all packages
		cmd := exec.Command("go", append(args, "../...", "../examples/...")...)
		return runBenchmarkCommand(cmd)
	}

	if example == "" {
		return runInteractiveBenchmarkMenu(duration)
	}

	// Run specific example benchmark
	ex, ok := getExampleByName(example)
	if !ok {
		return fmt.Errorf("unknown example: %s\n\nRun 'pipz list' to see available examples", example)
	}

	fmt.Printf(colorCyan+"\n═══ %s BENCHMARK ═══"+colorReset+"\n", strings.ToUpper(ex.Name()))
	fmt.Printf("Running performance tests for %s example...\n\n", ex.Name())

	// Run the benchmark in the example's package
	examplePath := fmt.Sprintf("../examples/%s", ex.Name())
	cmd := exec.Command("go", append(args, examplePath)...)
	return runBenchmarkCommand(cmd)
}

func runBenchmarkCommand(cmd *exec.Cmd) error {
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("benchmark failed: %w", err)
	}

	fmt.Println(colorGreen + "\n✅ Benchmark completed!" + colorReset)
	return nil
}

func runInteractiveBenchmarkMenu(duration string) error {
	fmt.Println(colorCyan + `
 ____  ___ ____  ____
|  _ \|_ _|  _ \|_  /
| |_) || || |_) |/ / 
|  __/ | ||  __// /_ 
|_|   |___|_|  /____|
` + colorReset)

	fmt.Println(colorWhite + "Benchmark Menu" + colorReset)
	fmt.Println(colorGray + "Select what to benchmark:\n" + colorReset)

	fmt.Println(colorYellow + "═══ OPTIONS ═══" + colorReset)
	fmt.Println(colorWhite + "1." + colorReset + " Core pipz features")
	fmt.Println(colorWhite + "2." + colorReset + " All examples")

	examples := getAllExamples()
	for i, ex := range examples {
		fmt.Printf("%s%d.%s %s example\n", colorWhite, i+3, colorReset, ex.Name())
	}

	fmt.Println(colorWhite + "q." + colorReset + " Quit")

	reader := bufio.NewReader(os.Stdin)
	fmt.Print("\nSelect an option: ")
	input, _ := reader.ReadString('\n')
	choice := strings.TrimSpace(strings.ToLower(input))

	switch choice {
	case "1":
		return runBenchmark("", false, true, duration)
	case "2":
		return runBenchmark("", true, false, duration)
	case "q", "quit", "exit":
		return nil
	default:
		// Check if it's a number for examples
		for i, ex := range examples {
			if choice == fmt.Sprintf("%d", i+3) {
				return runBenchmark(ex.Name(), false, false, duration)
			}
		}
		return fmt.Errorf("invalid option: %s", choice)
	}
}
