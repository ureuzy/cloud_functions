package main

import (
	"context"
)

// AIClient is the interface for AI service providers
type AIClient interface {
	// StartLecture generates the first lecture message for a topic with its description
	StartLecture(ctx context.Context, topic, description string) (string, error)

	// ContinueLecture continues the conversation based on history with the agenda
	ContinueLecture(ctx context.Context, topic string, agendaItems []AgendaItem, currentStep int, summary string, recentMessages []Message, userMessage string) (string, error)

	// SummarizeMessages summarizes old messages for context compression
	// existingSummary: if not empty, consolidate with new messages into a single summary
	SummarizeMessages(ctx context.Context, topic string, messages []Message, existingSummary string) (string, error)
}
