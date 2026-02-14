package main

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/firestore"
)

type Message struct {
	Role      string    `firestore:"role"`      // "user" or "model"
	Content   string    `firestore:"content"`
	Timestamp time.Time `firestore:"timestamp"`
}

type AgendaItem struct {
	StepNumber  int    `firestore:"step_number"`
	Description string `firestore:"description"`
	Completed   bool   `firestore:"completed"`
}

type ConversationThread struct {
	ThreadTs       string        `firestore:"thread_ts"`
	Topic          string        `firestore:"topic"`
	Description    string        `firestore:"description"`     // トピックの概要説明
	AgendaItems    []AgendaItem  `firestore:"agenda_items"`    // 講義のアジェンダ（構造化）
	CurrentStep    int           `firestore:"current_step"`    // 現在のステップ番号（1-indexed）
	Summary        string        `firestore:"summary"`         // 古い会話の要約
	RecentMessages []Message     `firestore:"recent_messages"` // 最新N件の詳細メッセージ
	CreatedAt      time.Time     `firestore:"created_at"`
	UpdatedAt      time.Time     `firestore:"updated_at"`
}

type FirestoreClient struct {
	client *firestore.Client
}

func NewFirestoreClient(ctx context.Context, projectID string) (*FirestoreClient, error) {
	client, err := firestore.NewClient(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to create firestore client: %w", err)
	}
	return &FirestoreClient{client: client}, nil
}

func (f *FirestoreClient) Close() error {
	return f.client.Close()
}

// GetThread retrieves a conversation thread
func (f *FirestoreClient) GetThread(ctx context.Context, threadTs string) (*ConversationThread, error) {
	doc, err := f.client.Collection("ai_sensei_threads").Doc(threadTs).Get(ctx)
	if err != nil {
		return nil, err
	}

	var thread ConversationThread
	if err := doc.DataTo(&thread); err != nil {
		return nil, err
	}

	return &thread, nil
}

// CreateThread creates a new conversation thread
func (f *FirestoreClient) CreateThread(ctx context.Context, threadTs, topic, description string) error {
	thread := ConversationThread{
		ThreadTs:       threadTs,
		Topic:          topic,
		Description:    description,
		Summary:        "",
		RecentMessages: []Message{},
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	_, err := f.client.Collection("ai_sensei_threads").Doc(threadTs).Set(ctx, thread)
	return err
}

// AddMessage adds a message to the thread
func (f *FirestoreClient) AddMessage(ctx context.Context, threadTs string, role, content string, maxRecentMessages int, aiClient AIClient) error {
	docRef := f.client.Collection("ai_sensei_threads").Doc(threadTs)

	// まずトランザクション外でスレッドデータを取得
	doc, err := docRef.Get(ctx)
	if err != nil {
		return err
	}

	var thread ConversationThread
	if err := doc.DataTo(&thread); err != nil {
		return err
	}

	// 新しいメッセージを追加
	newMessage := Message{
		Role:      role,
		Content:   content,
		Timestamp: time.Now(),
	}

	thread.RecentMessages = append(thread.RecentMessages, newMessage)

	// 最新N件のみ保持し、古いメッセージは要約に含める
	if len(thread.RecentMessages) > maxRecentMessages {
		// 削除される古いメッセージを取得
		numToRemove := len(thread.RecentMessages) - maxRecentMessages
		oldMessages := thread.RecentMessages[:numToRemove]

		// AIで要約生成（トランザクション外で実行）
		if aiClient != nil {
			newSummary, err := aiClient.SummarizeMessages(ctx, thread.Topic, oldMessages)
			if err != nil {
				// 要約失敗時はログを出して続行（古いメッセージは削除）
				fmt.Printf("Failed to summarize messages: %v\n", err)
			} else {
				// 既存の要約と新しい要約を結合
				if thread.Summary == "" {
					thread.Summary = newSummary
				} else {
					thread.Summary = thread.Summary + "\n\n" + newSummary
				}
			}
		}

		// 古いメッセージを削除
		thread.RecentMessages = thread.RecentMessages[numToRemove:]
	}

	thread.UpdatedAt = time.Now()

	// トランザクションで更新
	return f.client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		return tx.Set(docRef, thread)
	})
}

// GetConversationHistory returns formatted conversation history for Gemini
func (f *FirestoreClient) GetConversationHistory(ctx context.Context, threadTs string) (string, []Message, error) {
	thread, err := f.GetThread(ctx, threadTs)
	if err != nil {
		return "", nil, err
	}

	return thread.Summary, thread.RecentMessages, nil
}

// UpdateSummary updates the summary of older messages
func (f *FirestoreClient) UpdateSummary(ctx context.Context, threadTs, summary string) error {
	_, err := f.client.Collection("ai_sensei_threads").Doc(threadTs).Update(ctx, []firestore.Update{
		{Path: "summary", Value: summary},
		{Path: "updated_at", Value: time.Now()},
	})
	return err
}

// UpdateAgendaItems updates the agenda items and sets current step to 1
func (f *FirestoreClient) UpdateAgendaItems(ctx context.Context, threadTs string, agendaItems []AgendaItem) error {
	_, err := f.client.Collection("ai_sensei_threads").Doc(threadTs).Update(ctx, []firestore.Update{
		{Path: "agenda_items", Value: agendaItems},
		{Path: "current_step", Value: 1}, // 最初のステップから開始
		{Path: "updated_at", Value: time.Now()},
	})
	return err
}

// UpdateCurrentStep updates the current step number
func (f *FirestoreClient) UpdateCurrentStep(ctx context.Context, threadTs string, step int) error {
	_, err := f.client.Collection("ai_sensei_threads").Doc(threadTs).Update(ctx, []firestore.Update{
		{Path: "current_step", Value: step},
		{Path: "updated_at", Value: time.Now()},
	})
	return err
}

// MarkStepCompleted marks a step as completed and advances to next step if available
func (f *FirestoreClient) MarkStepCompleted(ctx context.Context, threadTs string, stepNumber int) error {
	docRef := f.client.Collection("ai_sensei_threads").Doc(threadTs)

	return f.client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		doc, err := tx.Get(docRef)
		if err != nil {
			return err
		}

		var thread ConversationThread
		if err := doc.DataTo(&thread); err != nil {
			return err
		}

		// Mark the step as completed
		for i := range thread.AgendaItems {
			if thread.AgendaItems[i].StepNumber == stepNumber {
				thread.AgendaItems[i].Completed = true
				break
			}
		}

		// Advance to next step if exists
		if stepNumber < len(thread.AgendaItems) {
			thread.CurrentStep = stepNumber + 1
		}

		thread.UpdatedAt = time.Now()

		return tx.Set(docRef, thread)
	})
}

// GetLesson retrieves the most recent lesson with the given topic
func (f *FirestoreClient) GetLesson(ctx context.Context, topic string) (string, error) {
	// Query for all lessons with this topic
	iter := f.client.Collection("ai_sensei_lessons").
		Where("topic", "==", topic).
		Documents(ctx)

	// Find the most recent one
	var mostRecentDoc *firestore.DocumentSnapshot
	var mostRecentTime time.Time

	for {
		doc, err := iter.Next()
		if err != nil {
			break
		}

		var lesson struct {
			Date        time.Time `firestore:"date"`
			Topic       string    `firestore:"topic"`
			Description string    `firestore:"description"`
		}
		if err := doc.DataTo(&lesson); err != nil {
			continue
		}

		if mostRecentDoc == nil || lesson.Date.After(mostRecentTime) {
			mostRecentDoc = doc
			mostRecentTime = lesson.Date
		}
	}

	if mostRecentDoc == nil {
		return "", fmt.Errorf("no lesson found with topic: %s", topic)
	}

	var lesson struct {
		Date        time.Time `firestore:"date"`
		Topic       string    `firestore:"topic"`
		Description string    `firestore:"description"`
	}
	if err := mostRecentDoc.DataTo(&lesson); err != nil {
		return "", fmt.Errorf("failed to parse lesson: %w", err)
	}

	return lesson.Description, nil
}

// DeleteRecentLesson deletes the most recent lesson with the given topic from ai_sensei_lessons
func (f *FirestoreClient) DeleteRecentLesson(ctx context.Context, topic string) error {
	// Query for all lessons with this topic (without OrderBy to avoid composite index)
	iter := f.client.Collection("ai_sensei_lessons").
		Where("topic", "==", topic).
		Documents(ctx)

	// Find the most recent one
	var mostRecentDoc *firestore.DocumentSnapshot
	var mostRecentTime time.Time

	for {
		doc, err := iter.Next()
		if err != nil {
			break
		}

		var lesson struct {
			Date  time.Time `firestore:"date"`
			Topic string    `firestore:"topic"`
		}
		if err := doc.DataTo(&lesson); err != nil {
			continue
		}

		if mostRecentDoc == nil || lesson.Date.After(mostRecentTime) {
			mostRecentDoc = doc
			mostRecentTime = lesson.Date
		}
	}

	if mostRecentDoc == nil {
		return fmt.Errorf("no lesson found with topic: %s", topic)
	}

	// Delete the most recent document
	_, err := mostRecentDoc.Ref.Delete(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete lesson: %w", err)
	}

	return nil
}
