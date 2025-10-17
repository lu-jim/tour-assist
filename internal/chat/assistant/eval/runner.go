package eval

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/acai-travel/tech-challenge/internal/chat/assistant"
	"github.com/acai-travel/tech-challenge/internal/chat/model"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Runner orchestrates the evaluation process
type Runner struct {
	assistant  *assistant.Assistant
	evaluators []Evaluator
}

// NewRunner creates a new evaluation runner
func NewRunner(asst *assistant.Assistant, evaluators []Evaluator) *Runner {
	return &Runner{
		assistant:  asst,
		evaluators: evaluators,
	}
}

// Run executes the evaluation for all test cases
func (r *Runner) Run(ctx context.Context, testCases []TestCase) (*EvalReport, error) {
	report := &EvalReport{
		StartTime:   time.Now(),
		TotalTests:  len(testCases),
		TestResults: make([]TestResult, 0, len(testCases)),
	}

	slog.InfoContext(ctx, "Starting evaluation run", "total_tests", len(testCases))

	for i, testCase := range testCases {
		slog.InfoContext(ctx, "Running test case",
			"id", testCase.ID,
			"progress", fmt.Sprintf("%d/%d", i+1, len(testCases)))

		result, err := r.runTestCase(ctx, testCase)
		if err != nil {
			slog.ErrorContext(ctx, "Test case failed", "id", testCase.ID, "error", err)
			// Continue with other tests even if one fails
			result = TestResult{
				TestCase: testCase,
				Actual: ActualOutput{
					Title: "",
					Error: stringPtr(err.Error()),
				},
				EvalResults: []EvalResult{{
					TestCaseID:  testCase.ID,
					Passed:      false,
					Score:       0,
					Details:     fmt.Sprintf("Execution failed: %v", err),
					ActualValue: "",
				}},
				OverallPass: false,
			}
		}

		report.TestResults = append(report.TestResults, result)

		if result.OverallPass {
			report.PassedTests++
		} else {
			report.FailedTests++
		}
	}

	report.EndTime = time.Now()
	report.Duration = report.EndTime.Sub(report.StartTime).Nanoseconds()

	// Calculate average score
	totalScore := 0.0
	for _, result := range report.TestResults {
		// Average score across all evaluators for this test
		testScore := 0.0
		for _, evalResult := range result.EvalResults {
			testScore += evalResult.Score
		}
		if len(result.EvalResults) > 0 {
			testScore /= float64(len(result.EvalResults))
		}
		totalScore += testScore
	}
	if len(report.TestResults) > 0 {
		report.AverageScore = totalScore / float64(len(report.TestResults))
	}

	slog.InfoContext(ctx, "Evaluation run completed",
		"total", report.TotalTests,
		"passed", report.PassedTests,
		"failed", report.FailedTests,
		"avg_score", report.AverageScore,
		"duration", report.Duration)

	return report, nil
}

// runTestCase executes a single test case
func (r *Runner) runTestCase(ctx context.Context, testCase TestCase) (TestResult, error) {
	startTime := time.Now()

	// Create a conversation from the test case
	conv := &model.Conversation{
		ID: primitive.NewObjectID(),
		Messages: []*model.Message{
			{
				ID:        primitive.NewObjectID(),
				Role:      model.RoleUser,
				Content:   testCase.Input.Message,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Generate title using the assistant
	title, err := r.assistant.Title(ctx, conv)

	duration := time.Since(startTime).Nanoseconds()

	if err != nil {
		return TestResult{}, fmt.Errorf("title generation failed: %w", err)
	}

	// Prepare actual output
	actual := ActualOutput{
		Title: title,
		Error: nil,
	}

	// Run all evaluators
	evalResults := make([]EvalResult, 0, len(r.evaluators))
	for _, evaluator := range r.evaluators {
		result := evaluator.Evaluate(testCase, actual)
		evalResults = append(evalResults, result)
	}

	// Determine overall pass/fail
	overallPass := true
	for _, result := range evalResults {
		if !result.Passed {
			overallPass = false
			break
		}
	}

	return TestResult{
		TestCase:    testCase,
		Actual:      actual,
		EvalResults: evalResults,
		OverallPass: overallPass,
		Duration:    duration,
	}, nil
}

// RunSingleTest runs evaluation for a single test case (useful for debugging)
func (r *Runner) RunSingleTest(ctx context.Context, testCase TestCase) (TestResult, error) {
	return r.runTestCase(ctx, testCase)
}

// stringPtr returns a pointer to a string
func stringPtr(s string) *string {
	return &s
}

// PrintSummary prints a human-readable summary of the evaluation report
func PrintSummary(report *EvalReport) {
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("Evaluation Report: %s\n", report.DatasetName)
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("Total tests:    %d\n", report.TotalTests)
	fmt.Printf("Passed:         %d (%.1f%%)\n", report.PassedTests,
		float64(report.PassedTests)/float64(report.TotalTests)*100)
	fmt.Printf("Failed:         %d (%.1f%%)\n", report.FailedTests,
		float64(report.FailedTests)/float64(report.TotalTests)*100)
	fmt.Printf("Average score:  %.3f\n", report.AverageScore)
	fmt.Printf("Duration:       %v\n", time.Duration(report.Duration))
	fmt.Println()

	// Print failed tests
	if report.FailedTests > 0 {
		fmt.Println("Failed Tests:")
		fmt.Println(strings.Repeat("-", 60))
		for _, result := range report.TestResults {
			if !result.OverallPass {
				fmt.Printf("\n[%s] %s\n", result.TestCase.ID, result.TestCase.Description)
				fmt.Printf("  Input:    %q\n", result.TestCase.Input.Message)
				fmt.Printf("  Title:    %q\n", result.Actual.Title)
				fmt.Println("  Issues:")
				for _, evalResult := range result.EvalResults {
					if !evalResult.Passed {
						fmt.Printf("    - [%s] %s (score: %.2f)\n",
							evalResult.TestCaseID, evalResult.Details, evalResult.Score)
					}
				}
			}
		}
		fmt.Println()
	}

	// Print successful tests summary
	if report.PassedTests > 0 {
		fmt.Println("Passed Tests:")
		fmt.Println(strings.Repeat("-", 60))
		for _, result := range report.TestResults {
			if result.OverallPass {
				avgScore := 0.0
				for _, er := range result.EvalResults {
					avgScore += er.Score
				}
				avgScore /= float64(len(result.EvalResults))

				fmt.Printf("✓ [%s] %s (score: %.2f)\n",
					result.TestCase.ID, result.TestCase.Description, avgScore)
			}
		}
	}
	fmt.Println(strings.Repeat("=", 60))
}
