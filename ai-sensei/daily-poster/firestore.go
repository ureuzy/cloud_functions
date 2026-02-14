package main

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/firestore"
	"google.golang.org/api/iterator"
)

type LessonHistory struct {
	Date        time.Time `firestore:"date"`
	Topic       string    `firestore:"topic"`
	Description string    `firestore:"description"`
}

type FirestoreClient struct {
	client *firestore.Client
}

func NewFirestoreClient(ctx context.Context, projectID string) (*FirestoreClient, error) {
	client, err := firestore.NewClient(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to create Firestore client: %w", err)
	}

	return &FirestoreClient{client: client}, nil
}

func (f *FirestoreClient) Close() error {
	return f.client.Close()
}

// GetRecentLessons retrieves lessons from the past N days
func (f *FirestoreClient) GetRecentLessons(ctx context.Context, days int) ([]LessonHistory, error) {
	cutoffDate := time.Now().AddDate(0, 0, -days)

	iter := f.client.Collection("ai_sensei_lessons").
		Where("date", ">=", cutoffDate).
		OrderBy("date", firestore.Desc).
		Documents(ctx)

	var lessons []LessonHistory
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to iterate lessons: %w", err)
		}

		var lesson LessonHistory
		if err := doc.DataTo(&lesson); err != nil {
			return nil, fmt.Errorf("failed to parse lesson: %w", err)
		}

		lessons = append(lessons, lesson)
	}

	return lessons, nil
}

// SaveLesson saves a new lesson to Firestore
func (f *FirestoreClient) SaveLesson(ctx context.Context, topic, description string) error {
	lesson := LessonHistory{
		Date:        time.Now(),
		Topic:       topic,
		Description: description,
	}

	_, _, err := f.client.Collection("ai_sensei_lessons").Add(ctx, lesson)
	if err != nil {
		return fmt.Errorf("failed to save lesson: %w", err)
	}

	return nil
}
