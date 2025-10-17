package eval

import "time"

// TestCase represents a single test case for title generation evaluation
type TestCase struct {
	ID          string   `json:"id"`
	Input       Input    `json:"input"`
	Expected    Expected `json:"expected"`
	Metadata    Metadata `json:"metadata"`
	Description string   `json:"description"`
}

// Input contains the input data for the test case
type Input struct {
	Message string `json:"message"`
}

// Expected contains the expected outputs and criteria
type Expected struct {
	TitleKeywords []string `json:"title_keywords,omitempty"`
	TitleMaxLen   int      `json:"title_max_len,omitempty"`
	TitleMinWords int      `json:"title_min_words,omitempty"`
	TitleMaxWords int      `json:"title_max_words,omitempty"`
	ShouldAvoid   []string `json:"should_avoid,omitempty"` // Patterns that shouldn't appear in title
}

// Metadata contains additional context about the test case
type Metadata struct {
	Category   string   `json:"category"`
	Difficulty string   `json:"difficulty"`
	Tags       []string `json:"tags,omitempty"`
}

// ActualOutput represents the actual output from the assistant
type ActualOutput struct {
	Title     string     `json:"Title"`
	Reply     string     `json:"Reply,omitempty"`
	ToolCalls []ToolCall `json:"ToolCalls,omitempty"`
	Error     *string    `json:"Error"`
}

// ToolCall represents a tool that was called
type ToolCall struct {
	ToolName  string                 `json:"ToolName"`
	Arguments map[string]interface{} `json:"Arguments"`
}

// EvalResult represents the result of a single evaluation
type EvalResult struct {
	TestCaseID  string                 `json:"test_case_id"`
	Passed      bool                   `json:"passed"`
	Score       float64                `json:"score"`
	Details     string                 `json:"details"`
	Metrics     map[string]interface{} `json:"metrics,omitempty"`
	ActualValue string                 `json:"actual_value"`
}

// TestResult combines test case with actual output and evaluation results
type TestResult struct {
	TestCase    TestCase     `json:"test_case"`
	Actual      ActualOutput `json:"actual"`
	EvalResults []EvalResult `json:"eval_results"`
	OverallPass bool         `json:"overall_pass"`
	Duration    int64        `json:"duration"` // nanoseconds
}

// EvalReport represents a complete evaluation run report
type EvalReport struct {
	DatasetName  string       `json:"dataset_name"`
	StartTime    time.Time    `json:"start_time"`
	EndTime      time.Time    `json:"end_time"`
	Duration     int64        `json:"duration"` // nanoseconds
	TotalTests   int          `json:"total_tests"`
	PassedTests  int          `json:"passed_tests"`
	FailedTests  int          `json:"failed_tests"`
	AverageScore float64      `json:"average_score"`
	TestResults  []TestResult `json:"test_results"`
}

// Evaluator is an interface for different evaluation strategies
type Evaluator interface {
	// Name returns the evaluator's name
	Name() string

	// Evaluate runs the evaluation for a test case and actual output
	Evaluate(testCase TestCase, actual ActualOutput) EvalResult
}
