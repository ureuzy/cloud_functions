package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"

	"github.com/ureuzy/cloud_functions/ai-sensei/event-handler/config"
)

type Server struct {
	config         *config.Config
	slackClient    *slack.Client
	firestoreClient *FirestoreClient
	geminiClient   *GeminiClient
}

func main() {
	ctx := context.Background()
	conf, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	log.Printf("Starting AI Sensei Event Handler on port %s", conf.Port)

	// Initialize clients
	slackClient := slack.New(conf.SlackBotToken)

	firestoreClient, err := NewFirestoreClient(ctx, conf.ProjectID)
	if err != nil {
		log.Fatalf("Failed to create Firestore client: %v", err)
	}
	defer firestoreClient.Close()

	geminiClient, err := NewGeminiClient(ctx, conf.ProjectID, conf.Location, conf.ModelName)
	if err != nil {
		log.Fatalf("Failed to create Gemini client: %v", err)
	}

	server := &Server{
		config:         conf,
		slackClient:    slackClient,
		firestoreClient: firestoreClient,
		geminiClient:   geminiClient,
	}

	// Setup router
	r := gin.Default()

	r.POST("/slack/events", server.handleSlackEvents)
	r.POST("/slack/interactive", server.handleInteractiveComponents)

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	log.Printf("Server listening on :%s", conf.Port)
	if err := r.Run(":" + conf.Port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

// handleSlackEvents handles Slack Events API
func (s *Server) handleSlackEvents(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		log.Printf("Failed to read request body: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	// Verify Slack signature
	sv, err := slack.NewSecretsVerifier(c.Request.Header, s.config.SlackSigningSecret)
	if err != nil {
		log.Printf("Failed to create secrets verifier: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if _, err := sv.Write(body); err != nil {
		log.Printf("Failed to write to secrets verifier: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	if err := sv.Ensure(); err != nil {
		log.Printf("Failed to verify Slack signature: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	eventsAPIEvent, err := slackevents.ParseEvent(json.RawMessage(body), slackevents.OptionNoVerifyToken())
	if err != nil {
		log.Printf("Failed to parse event: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to parse event"})
		return
	}

	// URL verification challenge
	if eventsAPIEvent.Type == slackevents.URLVerification {
		var r *slackevents.ChallengeResponse
		if err := json.Unmarshal(body, &r); err != nil {
			log.Printf("Failed to unmarshal challenge: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"challenge": r.Challenge})
		return
	}

	// Handle callback events
	if eventsAPIEvent.Type == slackevents.CallbackEvent {
		innerEvent := eventsAPIEvent.InnerEvent
		switch ev := innerEvent.Data.(type) {
		case *slackevents.AppMentionEvent:
			log.Printf("App mentioned in thread: %s", ev.ThreadTimeStamp)
			// Handle thread messages
			go s.handleThreadMessage(ev)
		case *slackevents.MessageEvent:
			// Only process thread replies
			if ev.ThreadTimeStamp != "" && ev.BotID == "" {
				log.Printf("Message in thread: %s", ev.ThreadTimeStamp)
				go s.handleThreadMessage(ev)
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// handleInteractiveComponents handles button clicks
func (s *Server) handleInteractiveComponents(c *gin.Context) {
	var payload slack.InteractionCallback
	err := json.Unmarshal([]byte(c.PostForm("payload")), &payload)
	if err != nil {
		log.Printf("Failed to unmarshal payload: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	// Handle button actions
	if payload.Type == slack.InteractionTypeBlockActions {
		for _, action := range payload.ActionCallback.BlockActions {
			switch action.ActionID {
			case "start_learning":
				topic := action.Value
				messageTs := payload.Message.Timestamp
				log.Printf("Button clicked - Channel: %s, MessageTs: %s, Topic: %s", payload.Channel.ID, messageTs, topic)

				// Remove action block (buttons) from the message and add started message
				var updatedBlocks []slack.Block
				for _, block := range payload.Message.Blocks.BlockSet {
					// Skip the action block
					if block.BlockType() != slack.MBTAction {
						updatedBlocks = append(updatedBlocks, block)
					}
				}

				// Add "学習を開始しました！" message
				startedBlock := slack.NewSectionBlock(
					slack.NewTextBlockObject("mrkdwn", "✅ *学習を開始しました！*", false, false),
					nil,
					nil,
				)
				updatedBlocks = append(updatedBlocks, startedBlock)

				_, _, _, err := s.slackClient.UpdateMessage(
					payload.Channel.ID,
					messageTs,
					slack.MsgOptionBlocks(updatedBlocks...),
				)
				if err != nil {
					log.Printf("Failed to update message: %v", err)
				}

				go s.startLearningSession(payload.Channel.ID, messageTs, topic)
			case "skip_topic":
				topic := action.Value
				messageTs := payload.Message.Timestamp
				log.Printf("Topic skipped: %s", topic)

				// Delete the message
				_, _, err := s.slackClient.DeleteMessage(payload.Channel.ID, messageTs)
				if err != nil {
					log.Printf("Failed to delete message: %v", err)
				}
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// startLearningSession starts a learning session in a thread
func (s *Server) startLearningSession(channel, messageTs, topic string) {
	ctx := context.Background()

	// Create thread in Firestore
	err := s.firestoreClient.CreateThread(ctx, messageTs, topic)
	if err != nil {
		log.Printf("Failed to create thread: %v", err)
		return
	}

	// Generate first lecture
	lecture, err := s.geminiClient.StartLecture(ctx, topic)
	if err != nil {
		log.Printf("Failed to start lecture: %v", err)
		return
	}

	// Save to Firestore
	err = s.firestoreClient.AddMessage(ctx, messageTs, "model", lecture, s.config.MaxRecentMessages, s.geminiClient)
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

	// Get thread info for topic
	thread, err := s.firestoreClient.GetThread(ctx, threadTs)
	if err != nil {
		log.Printf("Failed to get thread: %v", err)
		return
	}

	// Save user message
	err = s.firestoreClient.AddMessage(ctx, threadTs, "user", userMessage, s.config.MaxRecentMessages, s.geminiClient)
	if err != nil {
		log.Printf("Failed to save user message: %v", err)
	}

	// Generate response
	response, err := s.geminiClient.ContinueLecture(ctx, thread.Topic, summary, recentMessages, userMessage)
	if err != nil {
		log.Printf("Failed to continue lecture: %v", err)
		return
	}

	// Save assistant message
	err = s.firestoreClient.AddMessage(ctx, threadTs, "model", response, s.config.MaxRecentMessages, s.geminiClient)
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
