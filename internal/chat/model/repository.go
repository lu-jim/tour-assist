package model

import (
	"context"
	"errors"

	"github.com/twitchtv/twirp"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

const (
	conversationCollection = "conversations"
)

type Repository struct {
	conn *mongo.Database
}

func New(conn *mongo.Database) *Repository {
	return &Repository{
		conn: conn,
	}
}

func (r *Repository) CreateConversation(ctx context.Context, c *Conversation) error {
	tracer := otel.Tracer("github.com/acai-travel/tech-challenge/internal/chat/model")
	ctx, span := tracer.Start(ctx, "Repository.CreateConversation")
	span.SetAttributes(
		attribute.String("conversation.id", c.ID.Hex()),
		attribute.Int("conversation.message_count", len(c.Messages)),
	)
	defer span.End()

	_, err := r.conn.Collection(conversationCollection).InsertOne(ctx, c)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to create conversation")
		return err
	}

	span.SetStatus(codes.Ok, "conversation created")
	return nil
}

func (r *Repository) DescribeConversation(ctx context.Context, id string) (*Conversation, error) {
	tracer := otel.Tracer("github.com/acai-travel/tech-challenge/internal/chat/model")
	ctx, span := tracer.Start(ctx, "Repository.DescribeConversation")
	span.SetAttributes(attribute.String("conversation.id", id))
	defer span.End()

	var c Conversation

	oid, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "invalid conversation ID")
		return nil, twirp.NotFoundError("invalid conversation ID")
	}

	err = r.conn.Collection(conversationCollection).FindOne(ctx, map[string]any{"_id": oid}).Decode(&c)
	if errors.Is(err, mongo.ErrNoDocuments) {
		span.SetStatus(codes.Error, "conversation not found")
		return nil, twirp.NotFoundError("conversation not found")
	}

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "database error")
		return nil, err
	}

	span.SetAttributes(attribute.Int("conversation.message_count", len(c.Messages)))
	span.SetStatus(codes.Ok, "conversation found")
	return &c, nil
}

func (r *Repository) ListConversations(ctx context.Context) ([]*Conversation, error) {
	tracer := otel.Tracer("github.com/acai-travel/tech-challenge/internal/chat/model")
	ctx, span := tracer.Start(ctx, "Repository.ListConversations")
	defer span.End()

	opts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}})

	cursor, err := r.conn.Collection(conversationCollection).
		Find(ctx, map[string]any{}, opts)

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to query conversations")
		return nil, err
	}

	defer func() {
		_ = cursor.Close(ctx)
	}()

	var items []*Conversation

	for cursor.Next(ctx) {
		var c Conversation

		if err := cursor.Decode(&c); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "failed to decode conversation")
			return nil, err
		}

		items = append(items, &c)
	}

	if err := cursor.Err(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "cursor error")
		return nil, err
	}

	span.SetAttributes(attribute.Int("conversations.count", len(items)))
	span.SetStatus(codes.Ok, "conversations listed")
	return items, nil
}

func (r *Repository) UpdateConversation(ctx context.Context, c *Conversation) error {
	tracer := otel.Tracer("github.com/acai-travel/tech-challenge/internal/chat/model")
	ctx, span := tracer.Start(ctx, "Repository.UpdateConversation")
	span.SetAttributes(
		attribute.String("conversation.id", c.ID.Hex()),
		attribute.Int("conversation.message_count", len(c.Messages)),
	)
	defer span.End()

	_, err := r.conn.Collection(conversationCollection).UpdateOne(ctx,
		map[string]any{"_id": c.ID},
		map[string]any{"$set": c})

	if errors.Is(err, mongo.ErrNoDocuments) {
		span.SetStatus(codes.Error, "conversation not found")
		return twirp.NotFoundError("conversation not found")
	}

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to update conversation")
		return err
	}

	span.SetStatus(codes.Ok, "conversation updated")
	return nil
}

func (r *Repository) DeleteConversation(ctx context.Context, id string) error {
	_, err := r.conn.Collection(conversationCollection).DeleteOne(ctx, map[string]any{"_id": id})
	if errors.Is(err, mongo.ErrNoDocuments) {
		return twirp.NotFoundError("conversation not found")
	}

	return err
}
