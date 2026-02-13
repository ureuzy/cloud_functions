package main

import (
	"context"
)

// AIClient is the interface for AI service providers
type AIClient interface {
	// StartLecture generates the first lecture message for a topic
	StartLecture(ctx context.Context, topic string) (string, error)

	// ContinueLecture continues the conversation based on history
	ContinueLecture(ctx context.Context, topic, summary string, recentMessages []Message, userMessage string) (string, error)

	// SummarizeMessages summarizes old messages for context compression
	SummarizeMessages(ctx context.Context, topic string, messages []Message) (string, error)
}
