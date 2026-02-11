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

type ConversationThread struct {
	ThreadTs       string    `firestore:"thread_ts"`
	Topic          string    `firestore:"topic"`
	Summary        string    `firestore:"summary"`         // 古い会話の要約
	RecentMessages []Message `firestore:"recent_messages"` // 最新N件の詳細メッセージ
	CreatedAt      time.Time `firestore:"created_at"`
	UpdatedAt      time.Time `firestore:"updated_at"`
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
func (f *FirestoreClient) CreateThread(ctx context.Context, threadTs, topic string) error {
	thread := ConversationThread{
		ThreadTs:       threadTs,
		Topic:          topic,
		Summary:        "",
		RecentMessages: []Message{},
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	_, err := f.client.Collection("ai_sensei_threads").Doc(threadTs).Set(ctx, thread)
	return err
}

// AddMessage adds a message to the thread
func (f *FirestoreClient) AddMessage(ctx context.Context, threadTs string, role, content string, maxRecentMessages int) error {
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

		// 新しいメッセージを追加
		newMessage := Message{
			Role:      role,
			Content:   content,
			Timestamp: time.Now(),
		}

		thread.RecentMessages = append(thread.RecentMessages, newMessage)

		// 最新N件のみ保持し、古いメッセージは要約に含める
		if len(thread.RecentMessages) > maxRecentMessages {
			// 要約が必要（TODO: Geminiで要約生成）
			// 今はシンプルに古いメッセージを削除
			thread.RecentMessages = thread.RecentMessages[len(thread.RecentMessages)-maxRecentMessages:]
		}

		thread.UpdatedAt = time.Now()

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
