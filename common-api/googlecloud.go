package main

import (
	"context"
	"errors"
	"fmt"

	"cloud.google.com/go/scheduler/apiv1"
	"cloud.google.com/go/scheduler/apiv1/schedulerpb"
	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
)

type GoogleCloudClient struct {
	storage   *storage.Client
	scheduler *scheduler.CloudSchedulerClient
	projectID string
}

func NewGoogleCloudClient(ctx context.Context, projectID string) (*GoogleCloudClient, error) {
	storageClient, err := storage.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create storage client: %w", err)
	}

	schedulerClient, err := scheduler.NewCloudSchedulerClient(ctx)
	if err != nil {
		storageClient.Close()
		return nil, fmt.Errorf("failed to create scheduler client: %w", err)
	}

	return &GoogleCloudClient{
		storage:   storageClient,
		scheduler: schedulerClient,
		projectID: projectID,
	}, nil
}

func (g *GoogleCloudClient) Close() {
	g.storage.Close()
	g.scheduler.Close()
}

// GetBucketAttrs retrieves bucket attributes
func (g *GoogleCloudClient) GetBucketAttrs(ctx context.Context, bucketName string) (*storage.BucketAttrs, error) {
	return g.storage.Bucket(bucketName).Attrs(ctx)
}

// CountObjects counts objects with a given prefix
func (g *GoogleCloudClient) CountObjects(ctx context.Context, bucketName, prefix string) (int, error) {
	it := g.storage.Bucket(bucketName).Objects(ctx, &storage.Query{
		Prefix: prefix,
	})
	count := 0
	for {
		_, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return 0, err
		}
		count++
	}
	return count, nil
}

// ListSchedulerJobs lists jobs in a specific location
func (g *GoogleCloudClient) ListSchedulerJobs(ctx context.Context, location string) ([]*schedulerpb.Job, error) {
	parent := fmt.Sprintf("projects/%s/locations/%s", g.projectID, location)
	req := &schedulerpb.ListJobsRequest{
		Parent: parent,
	}

	var jobs []*schedulerpb.Job
	it := g.scheduler.ListJobs(ctx, req)
	for {
		job, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	return jobs, nil
}

// PauseSchedulerJob pauses a scheduler job
func (g *GoogleCloudClient) PauseSchedulerJob(ctx context.Context, jobName string) (*schedulerpb.Job, error) {
	req := &schedulerpb.PauseJobRequest{
		Name: jobName,
	}
	return g.scheduler.PauseJob(ctx, req)
}

// ResumeSchedulerJob resumes a scheduler job
func (g *GoogleCloudClient) ResumeSchedulerJob(ctx context.Context, jobName string) (*schedulerpb.Job, error) {
	req := &schedulerpb.ResumeJobRequest{
		Name: jobName,
	}
	return g.scheduler.ResumeJob(ctx, req)
}

// RunSchedulerJob manually triggers a scheduler job
func (g *GoogleCloudClient) RunSchedulerJob(ctx context.Context, jobName string) (*schedulerpb.Job, error) {
	req := &schedulerpb.RunJobRequest{
		Name: jobName,
	}
	return g.scheduler.RunJob(ctx, req)
}
