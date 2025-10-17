package eval

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/openai/openai-go/v2"
)

// LLMEvaluator implements LLM-as-a-judge evaluation using GPT-4
type LLMEvaluator struct {
	client openai.Client
}

// NewLLMEvaluator creates a new LLM-based evaluator
func NewLLMEvaluator() *LLMEvaluator {
	return &LLMEvaluator{
		client: openai.NewClient(),
	}
}

// Name returns the evaluator's name
func (e *LLMEvaluator) Name() string {
	return "llm_judge"
}

// Evaluate uses GPT-5 to assess title quality
func (e *LLMEvaluator) Evaluate(testCase TestCase, actual ActualOutput) EvalResult {
	ctx := context.Background()

	// Construct evaluation prompt with chain-of-thought reasoning
	// Note: Enhanced for determinism since GPT-5 doesn't support temperature
	systemPrompt := `You are an expert evaluator assessing the quality of AI-generated conversation titles. You must be consistent and objective in your evaluations.

Your task is to evaluate whether a title appropriately summarizes a user's question or message.

Evaluation criteria (score each 0-10):
1. **Relevance**: Does the title capture the core intent/topic of the user's message?
   - 10: Perfect match with main topic
   - 7-9: Good match, minor details missed
   - 4-6: Partially relevant
   - 0-3: Wrong topic or unrelated
   
2. **Conciseness**: Is the title brief (ideally 2-6 words, max 80 characters)?
   - 10: 2-6 words, under 50 chars
   - 7-9: 2-8 words, under 80 chars
   - 4-6: 9-12 words or 81-100 chars
   - 0-3: Too verbose
   
3. **Clarity**: Is the title clear and understandable without additional context?
   - 10: Immediately clear to anyone
   - 7-9: Clear with minimal context
   - 4-6: Somewhat ambiguous
   - 0-3: Confusing or unclear
   
4. **Accuracy**: Does the title focus on summarizing what was ASKED, not answering it?
   - 10: Summarizes question perfectly
   - 7-9: Summarizes with minor answer elements
   - 4-6: Mixed question/answer
   - 0-3: Answers instead of summarizes

IMPORTANT: You must respond with ONLY a valid JSON object in this EXACT format (no extra text):
{
  "reasoning": "Brief step-by-step analysis covering all 4 criteria",
  "relevance_score": <number 0-10>,
  "conciseness_score": <number 0-10>,
  "clarity_score": <number 0-10>,
  "accuracy_score": <number 0-10>,
  "overall_score": <number 0-10>,
  "passed": <true or false>,
  "issues": ["array", "of", "specific", "issues"]
}

Overall score should be the average of the 4 criteria scores. Pass if overall_score >= 7.`

	userPrompt := fmt.Sprintf(`Evaluate this title:

User's message: "%s"
Generated title: "%s"

Expected criteria:
- Keywords to include: %v
- Max length: %d characters
- Ideal word count: 2-6 words

Provide your evaluation in JSON format.`,
		testCase.Input.Message,
		actual.Title,
		testCase.Expected.TitleKeywords,
		testCase.Expected.TitleMaxLen,
	)

	msgs := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(systemPrompt),
		openai.UserMessage(userPrompt),
	}

	resp, err := e.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model:    openai.ChatModelGPT5,
		Messages: msgs,
		// Note: GPT-5 doesn't support temperature parameter
		// Determinism is enforced through explicit prompt instructions
	})

	if err != nil {
		return EvalResult{
			TestCaseID:  testCase.ID,
			Passed:      false,
			Score:       0,
			Details:     fmt.Sprintf("LLM evaluation failed: %v", err),
			ActualValue: actual.Title,
		}
	}

	if len(resp.Choices) == 0 {
		return EvalResult{
			TestCaseID:  testCase.ID,
			Passed:      false,
			Score:       0,
			Details:     "No response from LLM judge",
			ActualValue: actual.Title,
		}
	}

	// Parse the JSON response
	content := resp.Choices[0].Message.Content

	// Extract JSON from response (in case there's extra text)
	jsonStart := strings.Index(content, "{")
	jsonEnd := strings.LastIndex(content, "}")
	if jsonStart == -1 || jsonEnd == -1 {
		return EvalResult{
			TestCaseID:  testCase.ID,
			Passed:      false,
			Score:       0,
			Details:     fmt.Sprintf("Invalid JSON response from LLM judge: %s", content),
			ActualValue: actual.Title,
		}
	}

	jsonContent := content[jsonStart : jsonEnd+1]

	var judgeResult struct {
		Reasoning        string   `json:"reasoning"`
		RelevanceScore   float64  `json:"relevance_score"`
		ConcisenessScore float64  `json:"conciseness_score"`
		ClarityScore     float64  `json:"clarity_score"`
		AccuracyScore    float64  `json:"accuracy_score"`
		OverallScore     float64  `json:"overall_score"`
		Passed           bool     `json:"passed"`
		Issues           []string `json:"issues"`
	}

	if err := json.Unmarshal([]byte(jsonContent), &judgeResult); err != nil {
		return EvalResult{
			TestCaseID:  testCase.ID,
			Passed:      false,
			Score:       0,
			Details:     fmt.Sprintf("Failed to parse LLM judge response: %v. Content: %s", err, jsonContent),
			ActualValue: actual.Title,
		}
	}

	// Normalize score to 0-1 range
	normalizedScore := judgeResult.OverallScore / 10.0

	details := judgeResult.Reasoning
	if len(judgeResult.Issues) > 0 {
		details += " Issues: " + strings.Join(judgeResult.Issues, ", ")
	}

	metrics := map[string]interface{}{
		"relevance_score":   judgeResult.RelevanceScore,
		"conciseness_score": judgeResult.ConcisenessScore,
		"clarity_score":     judgeResult.ClarityScore,
		"accuracy_score":    judgeResult.AccuracyScore,
		"overall_score":     judgeResult.OverallScore,
		"reasoning":         judgeResult.Reasoning,
	}

	return EvalResult{
		TestCaseID:  testCase.ID,
		Passed:      judgeResult.Passed,
		Score:       normalizedScore,
		Details:     details,
		Metrics:     metrics,
		ActualValue: actual.Title,
	}
}

// PairwiseEvaluator compares two titles and determines which is better
type PairwiseEvaluator struct {
	client openai.Client
}

// NewPairwiseEvaluator creates a new pairwise comparison evaluator
func NewPairwiseEvaluator() *PairwiseEvaluator {
	return &PairwiseEvaluator{
		client: openai.NewClient(),
	}
}

// Compare compares two titles and returns which one is better
func (e *PairwiseEvaluator) Compare(ctx context.Context, userMessage, titleA, titleB string) (string, string, error) {
	systemPrompt := `You are an expert evaluator comparing conversation titles. Be consistent and objective.

Given a user message and two candidate titles, determine which title better summarizes the message.

Evaluation criteria in order of importance:
1. Relevance: Which captures the core topic better?
2. Conciseness: Which is more brief (prefer 2-6 words)?
3. Clarity: Which is easier to understand?
4. Accuracy: Which better summarizes (not answers) the question?

If both are equal, choose "A".

Respond with ONLY a JSON object in this EXACT format (no extra text):
{
  "winner": "A",
  "reasoning": "Concise explanation referencing specific criteria"
}

OR

{
  "winner": "B",
  "reasoning": "Concise explanation referencing specific criteria"
}`

	userPrompt := fmt.Sprintf(`User message: "%s"

Title A: "%s"
Title B: "%s"

Which title is better?`, userMessage, titleA, titleB)

	msgs := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(systemPrompt),
		openai.UserMessage(userPrompt),
	}

	resp, err := e.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model:    openai.ChatModelGPT5,
		Messages: msgs,
		// Note: GPT-5 doesn't support temperature parameter
	})

	if err != nil {
		return "", "", fmt.Errorf("pairwise comparison failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", "", fmt.Errorf("no response from LLM judge")
	}

	content := resp.Choices[0].Message.Content

	// Extract JSON
	jsonStart := strings.Index(content, "{")
	jsonEnd := strings.LastIndex(content, "}")
	if jsonStart == -1 || jsonEnd == -1 {
		return "", "", fmt.Errorf("invalid JSON response: %s", content)
	}

	jsonContent := content[jsonStart : jsonEnd+1]

	var result struct {
		Winner    string `json:"winner"`
		Reasoning string `json:"reasoning"`
	}

	if err := json.Unmarshal([]byte(jsonContent), &result); err != nil {
		return "", "", fmt.Errorf("failed to parse response: %w", err)
	}

	return result.Winner, result.Reasoning, nil
}
