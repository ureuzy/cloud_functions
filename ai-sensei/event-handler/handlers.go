package main

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
)

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

				// Delete the lesson from Firestore
				ctx := context.Background()
				err := s.firestoreClient.DeleteRecentLesson(ctx, topic)
				if err != nil {
					log.Printf("Failed to delete lesson from Firestore: %v", err)
				}

				// Delete the message
				_, _, err = s.slackClient.DeleteMessage(payload.Channel.ID, messageTs)
				if err != nil {
					log.Printf("Failed to delete message: %v", err)
				}
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
