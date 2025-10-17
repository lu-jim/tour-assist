package eval

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

// RuleEvaluator implements rule-based evaluation for title quality
type RuleEvaluator struct{}

// NewRuleEvaluator creates a new rule-based evaluator
func NewRuleEvaluator() *RuleEvaluator {
	return &RuleEvaluator{}
}

// Name returns the evaluator's name
func (e *RuleEvaluator) Name() string {
	return "rule_based"
}

// Evaluate runs rule-based evaluation checks
func (e *RuleEvaluator) Evaluate(testCase TestCase, actual ActualOutput) EvalResult {
	title := actual.Title
	expected := testCase.Expected

	metrics := make(map[string]interface{})
	issues := []string{}
	score := 1.0 // Start with perfect score

	// Check 1: Title length
	titleLen := utf8.RuneCountInString(title)
	metrics["title_length"] = titleLen

	if expected.TitleMaxLen > 0 && titleLen > expected.TitleMaxLen {
		score -= 0.4
		issues = append(issues, fmt.Sprintf("Title exceeds max length: %d > %d", titleLen, expected.TitleMaxLen))
	}

	// Check 2: Word count
	words := strings.Fields(title)
	wordCount := len(words)
	metrics["word_count"] = wordCount

	if expected.TitleMinWords > 0 && wordCount < expected.TitleMinWords {
		score -= 0.3
		issues = append(issues, fmt.Sprintf("Title has too few words: %d < %d", wordCount, expected.TitleMinWords))
	}

	if expected.TitleMaxWords > 0 && wordCount > expected.TitleMaxWords {
		score -= 0.3
		issues = append(issues, fmt.Sprintf("Title has too many words: %d > %d", wordCount, expected.TitleMaxWords))
	}

	// Check 3: Keyword matching
	if len(expected.TitleKeywords) > 0 {
		titleLower := strings.ToLower(title)
		foundKeywords := 0
		for _, keyword := range expected.TitleKeywords {
			if strings.Contains(titleLower, strings.ToLower(keyword)) {
				foundKeywords++
			}
		}
		keywordMatchRate := float64(foundKeywords) / float64(len(expected.TitleKeywords))
		metrics["keyword_match_rate"] = keywordMatchRate
		metrics["found_keywords"] = foundKeywords
		metrics["total_keywords"] = len(expected.TitleKeywords)

		if keywordMatchRate == 0 {
			score -= 0.5
			issues = append(issues, fmt.Sprintf("No keywords matched (0/%d)", len(expected.TitleKeywords)))
		} else if keywordMatchRate < 0.5 {
			score -= 0.3
			issues = append(issues, fmt.Sprintf("Low keyword match rate: %.2f", keywordMatchRate))
		} else if keywordMatchRate < 1.0 {
			score -= 0.1
		}
	}

	// Check 4: Format validation (no newlines) - critical failure
	if strings.Contains(title, "\n") {
		score = 0
		issues = append(issues, "Title contains newlines (critical)")
	}

	// Check 5: Check for avoided patterns - critical failure
	if len(expected.ShouldAvoid) > 0 {
		titleLower := strings.ToLower(title)
		foundAvoid := []string{}
		for _, avoid := range expected.ShouldAvoid {
			if strings.Contains(titleLower, strings.ToLower(avoid)) {
				foundAvoid = append(foundAvoid, avoid)
			}
		}
		if len(foundAvoid) > 0 {
			score = 0
			issues = append(issues, fmt.Sprintf("Title contains avoided patterns (critical): %v", foundAvoid))
		}
	}

	// Check 6: Excessive punctuation or emojis
	emojiPattern := regexp.MustCompile(`[\x{1F600}-\x{1F64F}\x{1F300}-\x{1F5FF}\x{1F680}-\x{1F6FF}\x{2600}-\x{26FF}\x{2700}-\x{27BF}]`)
	if emojiPattern.MatchString(title) {
		score -= 0.1
		issues = append(issues, "Title contains emojis")
	}

	punctCount := strings.Count(title, "!") + strings.Count(title, "?") + strings.Count(title, "...")
	if punctCount > 1 {
		score -= 0.1
		issues = append(issues, "Title has excessive punctuation")
	}

	// Check 7: Empty or whitespace-only title
	if strings.TrimSpace(title) == "" {
		score = 0
		issues = append(issues, "Title is empty or whitespace-only")
	}

	// Ensure score is between 0 and 1
	if score < 0 {
		score = 0
	}

	// Determine pass/fail
	passed := score >= 0.7 // 70% threshold for passing

	details := "Title meets all criteria"
	if len(issues) > 0 {
		details = strings.Join(issues, "; ")
	}

	return EvalResult{
		TestCaseID:  testCase.ID,
		Passed:      passed,
		Score:       score,
		Details:     details,
		Metrics:     metrics,
		ActualValue: title,
	}
}

// CompositeEvaluator runs multiple evaluators and combines their results
type CompositeEvaluator struct {
	evaluators []Evaluator
}

// NewCompositeEvaluator creates a new composite evaluator
func NewCompositeEvaluator(evaluators ...Evaluator) *CompositeEvaluator {
	return &CompositeEvaluator{
		evaluators: evaluators,
	}
}

// Name returns the evaluator's name
func (e *CompositeEvaluator) Name() string {
	return "composite"
}

// Evaluate runs all sub-evaluators and combines results
func (e *CompositeEvaluator) Evaluate(testCase TestCase, actual ActualOutput) EvalResult {
	if len(e.evaluators) == 0 {
		return EvalResult{
			TestCaseID:  testCase.ID,
			Passed:      false,
			Score:       0,
			Details:     "No evaluators configured",
			ActualValue: actual.Title,
		}
	}

	// Run all evaluators
	results := make([]EvalResult, 0, len(e.evaluators))
	for _, evaluator := range e.evaluators {
		result := evaluator.Evaluate(testCase, actual)
		results = append(results, result)
	}

	// Combine results
	totalScore := 0.0
	allPassed := true
	details := []string{}

	for i, result := range results {
		totalScore += result.Score
		if !result.Passed {
			allPassed = false
		}
		details = append(details, fmt.Sprintf("%s: %s", e.evaluators[i].Name(), result.Details))
	}

	avgScore := totalScore / float64(len(e.evaluators))

	return EvalResult{
		TestCaseID:  testCase.ID,
		Passed:      allPassed && avgScore >= 0.7,
		Score:       avgScore,
		Details:     strings.Join(details, " | "),
		ActualValue: actual.Title,
		Metrics: map[string]interface{}{
			"sub_results": results,
		},
	}
}
