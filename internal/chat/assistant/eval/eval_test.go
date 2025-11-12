package eval

import (
	"testing"
)

func TestRuleEvaluator_Evaluate(t *testing.T) {
	evaluator := NewRuleEvaluator()

	tests := []struct {
		name         string
		testCase     TestCase
		actual       ActualOutput
		wantPassed   bool
		wantMinScore float64
	}{
		{
			name: "perfect title passes all checks",
			testCase: TestCase{
				ID: "test_01",
				Input: Input{
					Message: "What is the weather?",
				},
				Expected: Expected{
					TitleKeywords: []string{"weather"},
					TitleMaxLen:   80,
					TitleMinWords: 2,
					TitleMaxWords: 6,
				},
			},
			actual: ActualOutput{
				Title: "Weather inquiry",
			},
			wantPassed:   true,
			wantMinScore: 1.0,
		},
		{
			name: "title too long fails",
			testCase: TestCase{
				ID: "test_02",
				Input: Input{
					Message: "What is the weather?",
				},
				Expected: Expected{
					TitleMaxLen: 20,
				},
			},
			actual: ActualOutput{
				Title: "This is a very long title that exceeds the maximum length",
			},
			wantPassed:   false,
			wantMinScore: 0,
		},
		{
			name: "missing keywords reduces score",
			testCase: TestCase{
				ID: "test_03",
				Input: Input{
					Message: "What is the weather in Barcelona?",
				},
				Expected: Expected{
					TitleKeywords: []string{"weather", "Barcelona"},
					TitleMaxLen:   80,
				},
			},
			actual: ActualOutput{
				Title: "Question about climate",
			},
			wantPassed:   false,
			wantMinScore: 0,
		},
		{
			name: "title with newline fails format check",
			testCase: TestCase{
				ID: "test_04",
				Input: Input{
					Message: "What is the weather?",
				},
				Expected: Expected{
					TitleMaxLen: 80,
				},
			},
			actual: ActualOutput{
				Title: "Weather\nInquiry",
			},
			wantPassed:   false,
			wantMinScore: 0,
		},
		{
			name: "title with avoided pattern fails",
			testCase: TestCase{
				ID: "test_05",
				Input: Input{
					Message: "What is the weather?",
				},
				Expected: Expected{
					TitleMaxLen: 80,
					ShouldAvoid: []string{"answer", "response"},
				},
			},
			actual: ActualOutput{
				Title: "Answer to weather question",
			},
			wantPassed:   false,
			wantMinScore: 0,
		},
		{
			name: "empty title fails",
			testCase: TestCase{
				ID: "test_06",
				Input: Input{
					Message: "What is the weather?",
				},
				Expected: Expected{
					TitleMaxLen: 80,
				},
			},
			actual: ActualOutput{
				Title: "   ",
			},
			wantPassed:   false,
			wantMinScore: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := evaluator.Evaluate(tt.testCase, tt.actual)

			if result.Passed != tt.wantPassed {
				t.Errorf("Passed = %v, want %v. Details: %s", result.Passed, tt.wantPassed, result.Details)
			}

			if result.Score < tt.wantMinScore {
				t.Errorf("Score = %.2f, want >= %.2f. Details: %s", result.Score, tt.wantMinScore, result.Details)
			}

			if result.TestCaseID != tt.testCase.ID {
				t.Errorf("TestCaseID = %s, want %s", result.TestCaseID, tt.testCase.ID)
			}

			if result.ActualValue != tt.actual.Title {
				t.Errorf("ActualValue = %s, want %s", result.ActualValue, tt.actual.Title)
			}
		})
	}
}

func TestCompositeEvaluator(t *testing.T) {
	rule1 := NewRuleEvaluator()
	rule2 := NewRuleEvaluator()

	composite := NewCompositeEvaluator(rule1, rule2)

	testCase := TestCase{
		ID: "test_composite",
		Input: Input{
			Message: "What is the weather?",
		},
		Expected: Expected{
			TitleKeywords: []string{"weather"},
			TitleMaxLen:   80,
		},
	}

	actual := ActualOutput{
		Title: "Weather inquiry",
	}

	result := composite.Evaluate(testCase, actual)

	if !result.Passed {
		t.Errorf("Composite evaluation should pass with good title, got: %s", result.Details)
	}

	// Score should be average of both evaluators (both should give 1.0)
	if result.Score < 0.9 {
		t.Errorf("Score = %.2f, expected close to 1.0", result.Score)
	}
}

func TestLoadAndSaveDataset(t *testing.T) {
	// Create test cases
	testCases := []TestCase{
		{
			ID: "test_01",
			Input: Input{
				Message: "Test message",
			},
			Expected: Expected{
				TitleKeywords: []string{"test"},
				TitleMaxLen:   80,
			},
			Metadata: Metadata{
				Category:   "test",
				Difficulty: "easy",
			},
			Description: "Test case",
		},
	}

	// Save to temp file
	tmpFile := t.TempDir() + "/test_dataset.json"
	err := SaveDataset(tmpFile, testCases)
	if err != nil {
		t.Fatalf("SaveDataset failed: %v", err)
	}

	// Load back
	loaded, err := LoadDataset(tmpFile)
	if err != nil {
		t.Fatalf("LoadDataset failed: %v", err)
	}

	if len(loaded) != len(testCases) {
		t.Errorf("Loaded %d test cases, want %d", len(loaded), len(testCases))
	}

	if loaded[0].ID != testCases[0].ID {
		t.Errorf("Loaded test case ID = %s, want %s", loaded[0].ID, testCases[0].ID)
	}

	if loaded[0].Input.Message != testCases[0].Input.Message {
		t.Errorf("Loaded message = %s, want %s", loaded[0].Input.Message, testCases[0].Input.Message)
	}
}

func TestSaveAndLoadReport(t *testing.T) {
	report := EvalReport{
		DatasetName:  "Test Dataset",
		TotalTests:   5,
		PassedTests:  4,
		FailedTests:  1,
		AverageScore: 0.85,
		TestResults: []TestResult{
			{
				TestCase: TestCase{
					ID: "test_01",
					Input: Input{
						Message: "Test",
					},
				},
				Actual: ActualOutput{
					Title: "Test Title",
				},
				EvalResults: []EvalResult{
					{
						TestCaseID:  "test_01",
						Passed:      true,
						Score:       0.9,
						Details:     "Good",
						ActualValue: "Test Title",
					},
				},
				OverallPass: true,
			},
		},
	}

	// Save to temp file
	tmpFile := t.TempDir() + "/test_report.json"
	err := SaveReport(tmpFile, report)
	if err != nil {
		t.Fatalf("SaveReport failed: %v", err)
	}

	// Load back
	loaded, err := LoadReport(tmpFile)
	if err != nil {
		t.Fatalf("LoadReport failed: %v", err)
	}

	if loaded.DatasetName != report.DatasetName {
		t.Errorf("Loaded dataset name = %s, want %s", loaded.DatasetName, report.DatasetName)
	}

	if loaded.TotalTests != report.TotalTests {
		t.Errorf("Loaded total tests = %d, want %d", loaded.TotalTests, report.TotalTests)
	}

	if len(loaded.TestResults) != len(report.TestResults) {
		t.Errorf("Loaded %d test results, want %d", len(loaded.TestResults), len(report.TestResults))
	}
}

func TestGetDefaultDataset(t *testing.T) {
	dataset := GetDefaultDataset()

	if len(dataset) == 0 {
		t.Fatal("Default dataset should not be empty")
	}

	// Check that all test cases have required fields
	for i, tc := range dataset {
		if tc.ID == "" {
			t.Errorf("Test case %d missing ID", i)
		}
		if tc.Input.Message == "" {
			t.Errorf("Test case %s missing input message", tc.ID)
		}
		if tc.Description == "" {
			t.Errorf("Test case %s missing description", tc.ID)
		}
		if tc.Metadata.Category == "" {
			t.Errorf("Test case %s missing category", tc.ID)
		}
	}

	// Verify we have edge cases
	hasShortInput := false

	for _, tc := range dataset {
		for _, tag := range tc.Metadata.Tags {
			if tag == "short_input" {
				hasShortInput = true
			}
		}
	}

	if !hasShortInput {
		t.Error("Default dataset should include short input edge cases")
	}
}
