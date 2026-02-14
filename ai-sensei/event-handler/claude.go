package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

type ClaudeClient struct {
	client    anthropic.Client
	modelName string
}

func NewClaudeClient(ctx context.Context, apiKey, modelName string) (*ClaudeClient, error) {
	client := anthropic.NewClient(
		option.WithAPIKey(apiKey),
	)

	return &ClaudeClient{
		client:    client,
		modelName: modelName,
	}, nil
}

// StartLecture generates the first lecture message for a topic
func (c *ClaudeClient) StartLecture(ctx context.Context, topic, description string) (string, error) {
	prompt := buildStartLecturePrompt(topic, description)

	message, err := c.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(c.modelName),
		MaxTokens: 1024,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(prompt)),
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to generate content: %w", err)
	}

	if len(message.Content) == 0 {
		return "", fmt.Errorf("no content generated")
	}

	// Extract text from content blocks
	var result strings.Builder
	for _, block := range message.Content {
		if block.Type == "text" {
			result.WriteString(block.Text)
		}
	}

	return result.String(), nil
}

// SummarizeMessages summarizes old messages for context compression
func (c *ClaudeClient) SummarizeMessages(ctx context.Context, topic string, messages []Message) (string, error) {
	if len(messages) == 0 {
		return "", nil
	}

	prompt := buildSummarizePrompt(topic, messages)

	message, err := c.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(c.modelName),
		MaxTokens: 512,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(prompt)),
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to generate summary: %w", err)
	}

	if len(message.Content) == 0 {
		return "", fmt.Errorf("no summary generated")
	}

	// Extract text from content blocks
	var result strings.Builder
	for _, block := range message.Content {
		if block.Type == "text" {
			result.WriteString(block.Text)
		}
	}

	return result.String(), nil
}

// ContinueLecture continues the conversation based on history
func (c *ClaudeClient) ContinueLecture(ctx context.Context, topic string, agendaItems []AgendaItem, currentStep int, summary string, recentMessages []Message, userMessage string) (string, error) {
	prompt := buildContinueLecturePrompt(topic, agendaItems, currentStep, summary, recentMessages, userMessage)

	message, err := c.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(c.modelName),
		MaxTokens: 1024,
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(prompt)),
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to generate content: %w", err)
	}

	if len(message.Content) == 0 {
		return "", fmt.Errorf("no content generated")
	}

	// Extract text from content blocks
	var result strings.Builder
	for _, block := range message.Content {
		if block.Type == "text" {
			result.WriteString(block.Text)
		}
	}

	return result.String(), nil
}
