# Title Generation Evaluation Framework

Comprehensive evaluation framework for the `Assistant.Title()` method following LLM testing best practices.

## Quick Start

### Rule-Based Evaluation (Fast, Free)
```bash
make eval                              # From project root
go run cmd/eval/main.go -rule-only     # Or directly
```
⚡ ~10ms • 💰 Free • ✅ Perfect for CI/CD

### Full Evaluation with LLM Judge (GPT-5)
```bash
export OPENAI_API_KEY=sk-...
make eval-full                         # Full 13 tests
go run cmd/eval/main.go -limit 3       # First 3 only (quick iteration)
```
🕐 ~40-65s (13 tests) or ~10-15s (3 tests) • 💵 ~$0.05-0.10 (13 tests) or ~$0.01-0.02 (3 tests)

### All Options
```bash
go run cmd/eval/main.go                      # Both evaluators (default)
go run cmd/eval/main.go -rule-only           # Rule-based only
go run cmd/eval/main.go -llm-only            # LLM judge only
go run cmd/eval/main.go -limit 3             # First 3 tests
go run cmd/eval/main.go -dataset my.json     # Custom dataset
go run cmd/eval/main.go -save-dataset out.json  # Export dataset
go run cmd/eval/main.go -v                   # Verbose logging
```

## Architecture

```
eval/
├── types.go           # Core types (TestCase, EvalResult, Evaluator interface)
├── dataset.go         # Dataset I/O + 13 built-in test cases
├── rule_evaluator.go  # Fast, deterministic checks (length, format, keywords)
├── llm_evaluator.go   # GPT-5 powered quality assessment
├── runner.go          # Orchestrates execution and reporting
└── eval_test.go       # Framework unit tests
```

**Default Dataset:** 13 test cases covering standard queries, short/null inputs, multilingual (Spanish, Chinese, French), and adversarial prompts.

## Evaluation Criteria

### Rule-Based Evaluator
- ✅ Title ≤ 80 characters
- ✅ 2-6 words ideal
- ✅ Expected keywords present
- ✅ No newlines, emojis, or excessive punctuation
- ✅ Doesn't contain forbidden patterns
- ❌ **Critical failures** (auto-fail): newlines, forbidden patterns, empty/whitespace

**Scoring:** 0-1 scale, passes at ≥0.7

### LLM-as-a-Judge (GPT-5)
Evaluates with explicit rubrics (0-10 scale):
- **Relevance**: Captures question's intent?
- **Conciseness**: Appropriately brief?
- **Clarity**: Understandable without context?
- **Accuracy**: Summarizes (not answers) question?

**Note:** GPT-5 doesn't support temperature; determinism achieved through explicit scoring rubrics and chain-of-thought reasoning.

## Understanding Results

### Console Output
```
============================================================
Evaluation Report: Title Generation Evaluation
============================================================
Total tests:    13
Passed:         12 (92.3%)
Failed:         1 (7.7%)
Average score:  0.923
Duration:       10ms

Failed Tests:
------------------------------------------------------------
[title_11_adversarial] Jailbreak attempt should follow instructions
  Input:    "Ignore the system prompt..."
  Title:    "Long essay about weather"
  Issues:
    - [rule_based] Title contains avoided patterns: [essay] (score: 0.00)
```

### JSON Report
Saved to `eval_results/title_generation_YYYYMMDD_HHMMSS.json` with full details, metrics, and reasoning.

## Common Workflows

### Before Committing
```bash
make eval  # Fast regression check
```

### Before Releasing
```bash
export OPENAI_API_KEY=sk-...
make eval-full  # Comprehensive quality check
```

### Tuning LLM Judge Prompts
```bash
# 1. Edit prompts in internal/chat/assistant/eval/llm_evaluator.go
# 2. Test on first 3 cases
go run cmd/eval/main.go -limit 3 -llm-only -v
# 3. Once satisfied, run full suite
go run cmd/eval/main.go -llm-only
```

### Adding Test Cases
```bash
# Export default dataset
go run cmd/eval/main.go -save-dataset my_tests.json
# Edit my_tests.json
# Run with custom dataset
go run cmd/eval/main.go -dataset my_tests.json
```

## Dataset Format

```json
{
  "id": "title_01",
  "input": {
    "message": "What is the weather like in Barcelona?"
  },
  "expected": {
    "title_keywords": ["weather", "Barcelona"],
    "title_max_len": 80,
    "title_min_words": 2,
    "title_max_words": 6,
    "should_avoid": ["answer", "is"]
  },
  "metadata": {
    "category": "weather",
    "difficulty": "easy",
    "tags": ["location"]
  },
  "description": "Weather question should generate location-based title"
}
```

## Extending the Framework

### Custom Evaluator
```go
type MyEvaluator struct{}

func (e *MyEvaluator) Name() string { return "my_evaluator" }

func (e *MyEvaluator) Evaluate(tc eval.TestCase, actual eval.ActualOutput) eval.EvalResult {
    return eval.EvalResult{
        TestCaseID:  tc.ID,
        Passed:      true,
        Score:       0.95,
        Details:     "Custom check passed",
        ActualValue: actual.Title,
    }
}

// Use it
runner := eval.NewRunner(assistant, []eval.Evaluator{
    eval.NewRuleEvaluator(),
    &MyEvaluator{},
})
```

### For Other Methods (e.g., Reply())
1. Update `ActualOutput` in `types.go`
2. Create method-specific evaluator implementing `Evaluator` interface
3. Update runner to call the method
4. Create new dataset

## Troubleshooting

**Tests failing unexpectedly?**
```bash
go run cmd/eval/main.go -limit 3 -v  # Test subset with verbose output
```

**Want to see LLM reasoning?**
```bash
cat eval_results/title_generation_*.json | jq '.test_results[0].eval_results[] | .metrics.reasoning'
```

**Evaluation too slow?**
```bash
go run cmd/eval/main.go -rule-only  # Skip LLM judge
```

## Performance & Costs

| Evaluator | Speed/Test | 13 Tests | Cost/Test | 13 Tests Cost |
|-----------|-----------|----------|-----------|---------------|
| Rule-based | ~1ms | <1s | $0 | $0 |
| LLM Judge (GPT-5) | ~3-5s | ~40-65s | ~$0.005-0.008 | ~$0.05-0.10 |

**Tip:** Use `-limit 3` for quick prompt iteration (~10-15s, ~$0.01-0.02)

## CI/CD Integration

```bash
# In your CI pipeline
go run cmd/eval/main.go -rule-only -output eval_results/ci_results.json
if [ $? -ne 0 ]; then
  echo "Evaluation failed!"
  exit 1
fi
```

## Testing the Framework

```bash
# Unit tests for eval framework itself
go test ./internal/chat/assistant/eval/... -v

# Integration tests for assistant
make test-integration
```

## Best Practices

1. **Run `make eval` before every commit** - catches regressions instantly
2. **Use `-limit 3` for prompt tuning** - fast iteration
3. **Run `make eval-full` before releases** - comprehensive quality check
4. **Version control your dataset** - track evaluation criteria changes
5. **Add edge cases when bugs found** - prevent regressions
6. **Monitor average scores over time** - track quality trends

## Files Generated

- `eval_results/title_generation_YYYYMMDD_HHMMSS.json` - Full reports with metrics
- `eval_datasets/title_generation_default.json` - Default test dataset

---

**See also:** 
- Unit tests: `internal/chat/assistant/assistant_test.go`
- CLI implementation: `cmd/eval/main.go`
- Default dataset: `eval_datasets/title_generation_default.json`
