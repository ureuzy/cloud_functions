package main

import (
	"context"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/slack-go/slack"

	"github.com/ureuzy/cloud_functions/ai-sensei/event-handler/config"
)

type Server struct {
	config          *config.Config
	slackClient     *slack.Client
	firestoreClient *FirestoreClient
	aiClient        AIClient
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

	// Initialize AI client based on provider
	var aiClient AIClient
	switch conf.AIProvider {
	case "claude":
		log.Printf("Initializing Claude client with model: %s", conf.ModelName)
		aiClient, err = NewClaudeClient(ctx, conf.ClaudeAPIKey, conf.ModelName)
		if err != nil {
			log.Fatalf("Failed to create Claude client: %v", err)
		}
	case "gemini":
		fallthrough
	default:
		log.Printf("Initializing Gemini client with model: %s", conf.ModelName)
		aiClient, err = NewGeminiClient(ctx, conf.ProjectID, conf.Location, conf.ModelName)
		if err != nil {
			log.Fatalf("Failed to create Gemini client: %v", err)
		}
	}

	server := &Server{
		config:          conf,
		slackClient:     slackClient,
		firestoreClient: firestoreClient,
		aiClient:        aiClient,
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
