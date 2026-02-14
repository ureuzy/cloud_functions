package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
)

// startLearningSession starts a learning session in a thread
func (s *Server) startLearningSession(channel, messageTs, topic string) {
	ctx := context.Background()

	// Get lesson description from Firestore
	description, err := s.firestoreClient.GetLesson(ctx, topic)
	if err != nil {
		log.Printf("Failed to get lesson description: %v", err)
		description = "" // Continue with empty description if not found
	}

	// Create thread in Firestore
	err = s.firestoreClient.CreateThread(ctx, messageTs, topic, description)
	if err != nil {
		log.Printf("Failed to create thread: %v", err)
		return
	}

	// Generate first lecture
	lecture, err := s.aiClient.StartLecture(ctx, topic, description)
	if err != nil {
		log.Printf("Failed to start lecture: %v", err)
		return
	}

	// Extract and save agenda
	agendaItems := extractAgenda(lecture)
	if len(agendaItems) > 0 {
		err = s.firestoreClient.UpdateAgendaItems(ctx, messageTs, agendaItems)
		if err != nil {
			log.Printf("Failed to save agenda: %v", err)
		}
	}

	// Save to Firestore
	err = s.firestoreClient.AddMessage(ctx, messageTs, "model", lecture, s.config.MaxRecentMessages, s.aiClient)
	if err != nil {
		log.Printf("Failed to save message: %v", err)
	}

	// Post to Slack thread
	_, _, err = s.slackClient.PostMessage(
		channel,
		slack.MsgOptionTS(messageTs),
		slack.MsgOptionText(lecture, false),
	)
	if err != nil {
		log.Printf("Failed to post message: %v", err)
	}
}

// handleThreadMessage handles messages in a learning thread
func (s *Server) handleThreadMessage(ev interface{}) {
	ctx := context.Background()

	var channel, threadTs, userMessage string

	switch e := ev.(type) {
	case *slackevents.AppMentionEvent:
		channel = e.Channel
		threadTs = e.ThreadTimeStamp
		userMessage = strings.TrimSpace(strings.Replace(e.Text, fmt.Sprintf("<@%s>", e.User), "", 1))
	case *slackevents.MessageEvent:
		channel = e.Channel
		threadTs = e.ThreadTimeStamp
		userMessage = e.Text
	default:
		log.Printf("Unexpected event type")
		return
	}

	if threadTs == "" {
		return
	}

	// Get conversation history
	summary, recentMessages, err := s.firestoreClient.GetConversationHistory(ctx, threadTs)
	if err != nil {
		log.Printf("Failed to get conversation history: %v", err)
		return
	}

	// Get thread info for topic and agenda
	thread, err := s.firestoreClient.GetThread(ctx, threadTs)
	if err != nil {
		log.Printf("Failed to get thread: %v", err)
		return
	}

	// Save user message
	err = s.firestoreClient.AddMessage(ctx, threadTs, "user", userMessage, s.config.MaxRecentMessages, s.aiClient)
	if err != nil {
		log.Printf("Failed to save user message: %v", err)
	}

	// Generate response with structured agenda and current step
	response, err := s.aiClient.ContinueLecture(ctx, thread.Topic, thread.AgendaItems, thread.CurrentStep, summary, recentMessages, userMessage)
	if err != nil {
		log.Printf("Failed to continue lecture: %v", err)
		return
	}

	// Check if AI signaled step completion
	if detectStepCompletion(response, thread.CurrentStep) {
		log.Printf("Detected step %d completion", thread.CurrentStep)
		err = s.firestoreClient.MarkStepCompleted(ctx, threadTs, thread.CurrentStep)
		if err != nil {
			log.Printf("Failed to mark step as completed: %v", err)
		}
	}

	// Save assistant message
	err = s.firestoreClient.AddMessage(ctx, threadTs, "model", response, s.config.MaxRecentMessages, s.aiClient)
	if err != nil {
		log.Printf("Failed to save response: %v", err)
	}

	// Post to Slack
	_, _, err = s.slackClient.PostMessage(
		channel,
		slack.MsgOptionTS(threadTs),
		slack.MsgOptionText(response, false),
	)
	if err != nil {
		log.Printf("Failed to post response: %v", err)
	}

	// Add delay to respect rate limits
	time.Sleep(time.Second * 1)
}
