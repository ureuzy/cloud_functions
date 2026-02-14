package main

import (
	"context"
	"fmt"
	"log"
	"math"
	"math/rand"
	"time"

	"google.golang.org/api/googleapi"
	"google.golang.org/genai"
)

type GeminiClient struct {
	client    *genai.Client
	modelName string
}

func NewGeminiClient(ctx context.Context, projectID, location, modelName string) (*GeminiClient, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		Project:  projectID,
		Location: location,
		Backend:  genai.BackendVertexAI,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create GenAI client: %w", err)
	}

	return &GeminiClient{
		client:    client,
		modelName: modelName,
	}, nil
}

// retryWithExponentialBackoff retries a function with exponential backoff on 429 errors
func retryWithExponentialBackoff(ctx context.Context, maxRetries int, fn func() error) error {
	for attempt := 0; attempt < maxRetries; attempt++ {
		err := fn()
		if err == nil {
			return nil
		}

		// Check if it's a 429 error
		if apiErr, ok := err.(*googleapi.Error); ok && apiErr.Code == 429 {
			if attempt < maxRetries-1 {
				backoff := time.Duration(math.Pow(2, float64(attempt))) * time.Second
				jitter := time.Duration(rand.Int63n(int64(time.Second)))
				waitDuration := backoff + jitter
				log.Printf("Received 429 error, retrying after %v (attempt %d/%d)", waitDuration, attempt+1, maxRetries)
				time.Sleep(waitDuration)
				continue
			}
		}

		return err
	}

	return fmt.Errorf("max retries exceeded")
}

// StartLecture generates the first lecture message for a topic
func (g *GeminiClient) StartLecture(ctx context.Context, topic, description string) (string, error) {
	prompt := buildStartLecturePrompt(topic, description)

	var result *genai.GenerateContentResponse
	err := retryWithExponentialBackoff(ctx, 5, func() error {
		var genErr error
		result, genErr = g.client.Models.GenerateContent(ctx, g.modelName, genai.Text(prompt), nil)
		return genErr
	})
	if err != nil {
		return "", fmt.Errorf("failed to generate content: %v", err)
	}

	if len(result.Candidates) == 0 || len(result.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("no content generated")
	}

	return result.Candidates[0].Content.Parts[0].Text, nil
}

// SummarizeMessages summarizes old messages for context compression
func (g *GeminiClient) SummarizeMessages(ctx context.Context, topic string, messages []Message, existingSummary string) (string, error) {
	if len(messages) == 0 {
		return "", nil
	}

	prompt := buildSummarizePrompt(topic, messages, existingSummary)

	var result *genai.GenerateContentResponse
	err := retryWithExponentialBackoff(ctx, 5, func() error {
		var genErr error
		result, genErr = g.client.Models.GenerateContent(ctx, g.modelName, genai.Text(prompt), nil)
		return genErr
	})
	if err != nil {
		return "", fmt.Errorf("failed to generate summary: %v", err)
	}

	if len(result.Candidates) == 0 || len(result.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("no summary generated")
	}

	return result.Candidates[0].Content.Parts[0].Text, nil
}

// ContinueLecture continues the conversation based on history
func (g *GeminiClient) ContinueLecture(ctx context.Context, topic string, agendaItems []AgendaItem, currentStep int, summary string, recentMessages []Message, userMessage string) (string, error) {
	prompt := buildContinueLecturePrompt(topic, agendaItems, currentStep, summary, recentMessages, userMessage)

	var result *genai.GenerateContentResponse
	err := retryWithExponentialBackoff(ctx, 5, func() error {
		var genErr error
		result, genErr = g.client.Models.GenerateContent(ctx, g.modelName, genai.Text(prompt), nil)
		return genErr
	})
	if err != nil {
		return "", fmt.Errorf("failed to generate content: %v", err)
	}

	if len(result.Candidates) == 0 || len(result.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("no content generated")
	}

	return result.Candidates[0].Content.Parts[0].Text, nil
}
