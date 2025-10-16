package chat

import (
	"context"
	"errors"
	"testing"

	"github.com/acai-travel/tech-challenge/internal/chat/model"
	. "github.com/acai-travel/tech-challenge/internal/chat/testing"
	"github.com/acai-travel/tech-challenge/internal/pb"
	"github.com/google/go-cmp/cmp"
	"github.com/twitchtv/twirp"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"google.golang.org/protobuf/testing/protocmp"
)

func TestServer_DescribeConversation(t *testing.T) {
	ctx := context.Background()
	srv := NewServer(model.New(ConnectMongo()), nil)

	t.Run("describe existing conversation", WithFixture(func(t *testing.T, f *Fixture) {
		c := f.CreateConversation()

		out, err := srv.DescribeConversation(ctx, &pb.DescribeConversationRequest{ConversationId: c.ID.Hex()})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		got, want := out.GetConversation(), c.Proto()
		if !cmp.Equal(got, want, protocmp.Transform()) {
			t.Errorf("DescribeConversation() mismatch (-got +want):\n%s", cmp.Diff(got, want, protocmp.Transform()))
		}
	}))

	t.Run("describe non existing conversation should return 404", WithFixture(func(t *testing.T, f *Fixture) {
		_, err := srv.DescribeConversation(ctx, &pb.DescribeConversationRequest{ConversationId: "08a59244257c872c5943e2a2"})
		if err == nil {
			t.Fatal("expected error for non-existing conversation, got nil")
		}

		if te, ok := err.(twirp.Error); !ok || te.Code() != twirp.NotFound {
			t.Fatalf("expected twirp.NotFound error, got %v", err)
		}
	}))
}

// testAssistant is a simple test implementation of the Assistant interface. It returns configurable values and errors.
type testAssistant struct {
	title    string
	titleErr error
	reply    string
	replyErr error
}

func (m *testAssistant) Title(ctx context.Context, conv *model.Conversation) (string, error) {
	return m.title, m.titleErr
}

func (m *testAssistant) Reply(ctx context.Context, conv *model.Conversation) (string, error) {
	return m.reply, m.replyErr
}

func TestServer_StartConversation(t *testing.T) {
	ctx := context.Background()

	// Define test table with various scenarios
	var tests = []struct {
		name         string
		message      string
		testTitle    string
		testTitleErr error
		testReply    string
		testReplyErr error
		wantErr      bool
		wantErrCode  twirp.ErrorCode
	}{
		{
			name:         "valid message creates conversation with title and reply",
			message:      "What is the weather like in Barcelona?",
			testTitle:    "Weather in Barcelona",
			testTitleErr: nil,
			testReply:    "The weather is sunny with a temperature of 25°C.",
			testReplyErr: nil,
			wantErr:      false,
		},
		{
			name:         "valid message with short content",
			message:      "Hi",
			testTitle:    "Greeting",
			testTitleErr: nil,
			testReply:    "Hello! How can I help you today?",
			testReplyErr: nil,
			wantErr:      false,
		},
		{
			name:         "empty message returns required argument error",
			message:      "",
			testTitle:    "",
			testTitleErr: nil,
			testReply:    "",
			testReplyErr: nil,
			wantErr:      true,
			wantErrCode:  twirp.InvalidArgument,
		},
		{
			name:         "whitespace-only message returns required argument error",
			message:      "   \t\n  ",
			testTitle:    "",
			testTitleErr: nil,
			testReply:    "",
			testReplyErr: nil,
			wantErr:      true,
			wantErrCode:  twirp.InvalidArgument,
		},
		{
			name:         "reply error propagates and conversation not created",
			message:      "Tell me a joke",
			testTitle:    "Joke Request",
			testTitleErr: nil,
			testReply:    "",
			testReplyErr: errors.New("OpenAI API error"),
			wantErr:      true,
		},
		{
			name:         "title error cancels reply and returns error",
			message:      "What's the time?",
			testTitle:    "",
			testTitleErr: errors.New("title generation failed"),
			testReply:    "It's 3:45 PM.",
			testReplyErr: nil,
			wantErr:      true,
		},
	}

	// Execute tests
	for _, tt := range tests {
		t.Run(tt.name, WithFixture(func(t *testing.T, f *Fixture) {
			// Setup test assistant
			test := &testAssistant{
				title:    tt.testTitle,
				titleErr: tt.testTitleErr,
				reply:    tt.testReply,
				replyErr: tt.testReplyErr,
			}

			srv := NewServer(f.Repository, test)

			// Execute StartConversation
			resp, err := srv.StartConversation(ctx, &pb.StartConversationRequest{
				Message: tt.message,
			})

			// Check error expectations
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}

				// Check error code if specified
				if tt.wantErrCode != "" {
					if te, ok := err.(twirp.Error); ok {
						if te.Code() != tt.wantErrCode {
							t.Errorf("expected error code %v, got %v", tt.wantErrCode, te.Code())
						}
					} else {
						t.Errorf("expected twirp.Error, got %T", err)
					}
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Validate response
			if resp == nil {
				t.Fatal("expected response, got nil")
			}

			// Check conversation ID is valid
			if resp.ConversationId == "" {
				t.Error("expected non-empty conversation ID")
			}

			if _, err := primitive.ObjectIDFromHex(resp.ConversationId); err != nil {
				t.Errorf("conversation ID is not a valid hex: %v", err)
			}

			// Check title
			expectedTitle := tt.testTitle
			if tt.testTitleErr != nil {
				expectedTitle = "Untitled conversation"
			}
			if resp.Title != expectedTitle {
				t.Errorf("expected title %q, got %q", expectedTitle, resp.Title)
			}

			// Check reply
			if resp.Reply != tt.testReply {
				t.Errorf("expected reply %q, got %q", tt.testReply, resp.Reply)
			}

			// Verify conversation was persisted in database
			conv, err := f.Repository.DescribeConversation(ctx, resp.ConversationId)
			if err != nil {
				t.Fatalf("failed to retrieve created conversation: %v", err)
			}

			// Validate conversation structure
			if conv.Title != expectedTitle {
				t.Errorf("persisted conversation has title %q, expected %q", conv.Title, expectedTitle)
			}

			// Check messages: should have user message and assistant reply
			if len(conv.Messages) != 2 {
				t.Fatalf("expected 2 messages, got %d", len(conv.Messages))
			}

			// Validate user message
			userMsg := conv.Messages[0]
			if userMsg.Role != model.RoleUser {
				t.Errorf("first message should be user role, got %v", userMsg.Role)
			}
			if userMsg.Content != tt.message {
				t.Errorf("user message content = %q, want %q", userMsg.Content, tt.message)
			}

			// Validate assistant message
			assistantMsg := conv.Messages[1]
			if assistantMsg.Role != model.RoleAssistant {
				t.Errorf("second message should be assistant role, got %v", assistantMsg.Role)
			}
			if assistantMsg.Content != tt.testReply {
				t.Errorf("assistant message content = %q, want %q", assistantMsg.Content, tt.testReply)
			}
		}))
	}
}
