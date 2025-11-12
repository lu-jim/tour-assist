package eval

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// LoadDataset loads a test dataset from a JSON file
func LoadDataset(path string) ([]TestCase, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read dataset file: %w", err)
	}

	var testCases []TestCase
	if err := json.Unmarshal(data, &testCases); err != nil {
		return nil, fmt.Errorf("failed to parse dataset JSON: %w", err)
	}

	return testCases, nil
}

// SaveDataset saves a test dataset to a JSON file
func SaveDataset(path string, testCases []TestCase) error {
	data, err := json.MarshalIndent(testCases, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal dataset: %w", err)
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write dataset file: %w", err)
	}

	return nil
}

// SaveReport saves an evaluation report to a JSON file
func SaveReport(path string, report EvalReport) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal report: %w", err)
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write report file: %w", err)
	}

	return nil
}

// LoadReport loads an evaluation report from a JSON file
func LoadReport(path string) (*EvalReport, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read report file: %w", err)
	}

	var report EvalReport
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, fmt.Errorf("failed to parse report JSON: %w", err)
	}

	return &report, nil
}

// GetDefaultDataset returns a comprehensive default dataset for title generation testing
func GetDefaultDataset() []TestCase {
	return []TestCase{
		{
			ID: "title_01",
			Input: Input{
				Message: "What is the weather like in Barcelona?",
			},
			Expected: Expected{
				TitleKeywords: []string{"weather", "Barcelona"},
				TitleMaxLen:   80,
				TitleMinWords: 2,
				TitleMaxWords: 6,
			},
			Metadata: Metadata{
				Category:   "weather",
				Difficulty: "easy",
			},
			Description: "Weather question should generate location-based title",
		},
		{
			ID: "title_02",
			Input: Input{
				Message: "I'm planning a trip to Paris next week. Can you tell me the weather forecast?",
			},
			Expected: Expected{
				TitleKeywords: []string{"Paris", "forecast"},
				TitleMaxLen:   80,
				TitleMinWords: 2,
				TitleMaxWords: 6,
			},
			Metadata: Metadata{
				Category:   "weather",
				Difficulty: "medium",
			},
			Description: "Weather forecast should have clear title",
		},
		{
			ID: "title_03",
			Input: Input{
				Message: "What are the upcoming holidays in Barcelona?",
			},
			Expected: Expected{
				TitleKeywords: []string{"holiday"},
				TitleMaxLen:   80,
				TitleMinWords: 2,
				TitleMaxWords: 6,
			},
			Metadata: Metadata{
				Category:   "calendar",
				Difficulty: "easy",
			},
			Description: "Holiday check in Catalonia should mention holidays",
		},
		{
			ID: "title_04",
			Input: Input{
				Message: "Are there any flights from barcelona to paris for next friday?",
			},
			Expected: Expected{
				TitleKeywords: []string{"flight"},
				TitleMaxLen:   80,
				TitleMinWords: 2,
				TitleMaxWords: 6,
			},
			Metadata: Metadata{
				Category:   "travel",
				Difficulty: "easy",
			},
			Description: "Flight search query should generate travel-focused title",
		},
		{
			ID: "title_05",
			Input: Input{
				Message: "Hi",
			},
			Expected: Expected{
				TitleMaxLen:   80,
				TitleMinWords: 1,
				TitleMaxWords: 6,
			},
			Metadata: Metadata{
				Category:   "edge_case",
				Difficulty: "easy",
				Tags:       []string{"short_input"},
			},
			Description: "Very short greeting should generate appropriate title",
		},
	}
}
