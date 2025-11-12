package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/acai-travel/tech-challenge/internal/chat/assistant"
	"github.com/acai-travel/tech-challenge/internal/chat/assistant/eval"
)

func main() {
	var (
		datasetPath = flag.String("dataset", "", "Path to test dataset JSON file (optional, uses default if not provided)")
		outputPath  = flag.String("output", "", "Path to save evaluation report (optional, auto-generated if not provided)")
		useRuleOnly = flag.Bool("rule-only", false, "Use only rule-based evaluation (skip LLM judge)")
		useLLMOnly  = flag.Bool("llm-only", false, "Use only LLM-as-judge evaluation (skip rule-based)")
		saveDataset = flag.String("save-dataset", "", "Save default dataset to file and exit")
		verbose     = flag.Bool("v", false, "Verbose logging")
		limitTests  = flag.Int("limit", 0, "Limit number of tests to run (0 = run all, useful for quick iteration)")
	)

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Run title generation evaluations for the AI assistant.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  # Run with default dataset and both evaluators:\n")
		fmt.Fprintf(os.Stderr, "  %s\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  # Run with custom dataset:\n")
		fmt.Fprintf(os.Stderr, "  %s -dataset my_tests.json\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  # Run rule-based evaluation only (fast, no API calls):\n")
		fmt.Fprintf(os.Stderr, "  %s -rule-only\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  # Test first 3 cases with LLM judge (quick iteration):\n")
		fmt.Fprintf(os.Stderr, "  %s -limit 3\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  # Save default dataset to file:\n")
		fmt.Fprintf(os.Stderr, "  %s -save-dataset dataset.json\n\n", os.Args[0])
	}

	flag.Parse()

	// Configure logging
	logLevel := slog.LevelInfo
	if *verbose {
		logLevel = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	}))
	slog.SetDefault(logger)

	ctx := context.Background()

	// Handle save-dataset command
	if *saveDataset != "" {
		if err := saveDefaultDataset(*saveDataset); err != nil {
			slog.Error("Failed to save dataset", "error", err)
			os.Exit(1)
		}
		slog.Info("Dataset saved successfully", "path", *saveDataset)
		return
	}

	// Validate flags
	if *useRuleOnly && *useLLMOnly {
		slog.Error("Cannot use both -rule-only and -llm-only flags")
		flag.Usage()
		os.Exit(1)
	}

	// Load test cases
	var testCases []eval.TestCase
	var err error

	if *datasetPath != "" {
		slog.Info("Loading dataset from file", "path", *datasetPath)
		testCases, err = eval.LoadDataset(*datasetPath)
		if err != nil {
			slog.Error("Failed to load dataset", "error", err)
			os.Exit(1)
		}
	} else {
		slog.Info("Using default dataset")
		testCases = eval.GetDefaultDataset()
	}

	slog.Info("Loaded test cases", "count", len(testCases))

	// Limit test cases if requested
	if *limitTests > 0 && *limitTests < len(testCases) {
		testCases = testCases[:*limitTests]
		slog.Info("Limited test cases for quick iteration", "running", len(testCases))
	}

	// Create evaluators
	var evaluators []eval.Evaluator

	if *useLLMOnly {
		slog.Info("Using LLM-as-judge evaluator only")
		evaluators = []eval.Evaluator{eval.NewLLMEvaluator()}
	} else if *useRuleOnly {
		slog.Info("Using rule-based evaluator only")
		evaluators = []eval.Evaluator{eval.NewRuleEvaluator()}
	} else {
		slog.Info("Using both rule-based and LLM-as-judge evaluators")
		evaluators = []eval.Evaluator{
			eval.NewRuleEvaluator(),
			eval.NewLLMEvaluator(),
		}
	}

	// Create assistant
	asst := assistant.New()

	// Create runner
	runner := eval.NewRunner(asst, evaluators)

	// Run evaluation
	slog.Info("Starting evaluation run")
	report, err := runner.Run(ctx, testCases)
	if err != nil {
		slog.Error("Evaluation run failed", "error", err)
		os.Exit(1)
	}

	// Set report name
	report.DatasetName = "Title Generation Evaluation"

	// Determine output path
	outputFile := *outputPath
	if outputFile == "" {
		// Auto-generate filename with timestamp
		timestamp := time.Now().Format("20060102_150405")
		outputFile = filepath.Join("eval_results", fmt.Sprintf("title_generation_%s.json", timestamp))
	}

	// Save report
	slog.Info("Saving evaluation report", "path", outputFile)
	if err := eval.SaveReport(outputFile, *report); err != nil {
		slog.Error("Failed to save report", "error", err)
		os.Exit(1)
	}

	// Print summary to console
	fmt.Println()
	eval.PrintSummary(report)
	fmt.Println()
	fmt.Printf("Full report saved to: %s\n", outputFile)

	// Exit with error code if tests failed
	if report.FailedTests > 0 {
		os.Exit(1)
	}
}

func saveDefaultDataset(path string) error {
	testCases := eval.GetDefaultDataset()
	return eval.SaveDataset(path, testCases)
}
