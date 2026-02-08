package main

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

type SchedulerHandler struct {
	gcClient  *GoogleCloudClient
	projectID string
}

func NewSchedulerHandler(gcClient *GoogleCloudClient, projectID string) *SchedulerHandler {
	return &SchedulerHandler{
		gcClient:  gcClient,
		projectID: projectID,
	}
}

func (h *SchedulerHandler) ListJobs(c *gin.Context) {
	location := c.DefaultQuery("location", "asia-northeast1")
	jobs, err := h.gcClient.ListSchedulerJobs(c.Request.Context(), location)
	if err != nil {
		log.Printf("failed to list scheduler jobs: %v", err)
		c.JSON(http.StatusInternalServerError, Response{
			Status:  "error",
			Message: "failed to list scheduler jobs",
		})
		return
	}

	var jobData []interface{}
	for _, job := range jobs {
		jobData = append(jobData, gin.H{
			"name":        job.GetName(),
			"schedule":    job.GetSchedule(),
			"timezone":    job.GetTimeZone(),
			"state":       job.GetState().String(),
			"last_run":    job.GetLastAttemptTime(),
			"description": job.GetDescription(),
		})
	}

	c.JSON(http.StatusOK, Response{
		Status: "ok",
		Data: gin.H{
			"project_id": h.projectID,
			"location":   location,
			"jobs":       jobData,
		},
	})
}
