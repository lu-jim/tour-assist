gen:
	protoc --proto_path=. --twirp_out=. --go_out=. rpc/*.proto

run:
	go run ./cmd/server

test:
	go test ./...

# Run title generation evaluations (rule-based only, fast)
eval:
	go run ./cmd/eval -rule-only

# Run full evaluations with LLM judge (requires OPENAI_API_KEY, slower)
eval-full:
	go run ./cmd/eval

# Run integration tests for assistant (requires OPENAI_API_KEY)
test-integration:
	go test -v ./internal/chat/assistant/... -run Integration

up:
	docker compose up -d

down:
	docker compose down
