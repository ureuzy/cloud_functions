package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

type SchedulerHandler struct {
	gcClient  *GoogleCloudClient
	projectID string
}

type JobRequest struct {
	Name string `json:"name" form:"name" binding:"required"`
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

func (h *SchedulerHandler) PauseJob(c *gin.Context) {
	var req JobRequest
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Status:  "error",
			Message: "job name is required",
		})
		return
	}

	job, err := h.gcClient.PauseSchedulerJob(c.Request.Context(), req.Name)
	if err != nil {
		log.Printf("failed to pause scheduler job: %v", err)
		c.JSON(http.StatusInternalServerError, Response{
			Status:  "error",
			Message: "failed to pause scheduler job",
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Status:  "ok",
		Message: fmt.Sprintf("job %s paused", job.GetName()),
		Data:    job,
	})
}

func (h *SchedulerHandler) ResumeJob(c *gin.Context) {
	var req JobRequest
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Status:  "error",
			Message: "job name is required",
		})
		return
	}

	job, err := h.gcClient.ResumeSchedulerJob(c.Request.Context(), req.Name)
	if err != nil {
		log.Printf("failed to resume scheduler job: %v", err)
		c.JSON(http.StatusInternalServerError, Response{
			Status:  "error",
			Message: "failed to resume scheduler job",
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Status:  "ok",
		Message: fmt.Sprintf("job %s resumed", job.GetName()),
		Data:    job,
	})
}

func (h *SchedulerHandler) RunJob(c *gin.Context) {
	var req JobRequest
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Status:  "error",
			Message: "job name is required",
		})
		return
	}

	job, err := h.gcClient.RunSchedulerJob(c.Request.Context(), req.Name)
	if err != nil {
		log.Printf("failed to run scheduler job: %v", err)
		c.JSON(http.StatusInternalServerError, Response{
			Status:  "error",
			Message: "failed to run scheduler job",
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Status:  "ok",
		Message: fmt.Sprintf("job %s triggered", job.GetName()),
		Data:    job,
	})
}
