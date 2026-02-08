package main

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

type StorageHandler struct {
	gcClient   *GoogleCloudClient
	bucketName string
}

func NewStorageHandler(gcClient *GoogleCloudClient, bucketName string) *StorageHandler {
	return &StorageHandler{
		gcClient:   gcClient,
		bucketName: bucketName,
	}
}

func (h *StorageHandler) GetBucketAttrs(c *gin.Context) {
	attrs, err := h.gcClient.GetBucketAttrs(c.Request.Context(), h.bucketName)
	if err != nil {
		log.Printf("failed to get bucket attributes: %v", err)
		c.JSON(http.StatusInternalServerError, Response{
			Status:  "error",
			Message: "failed to get bucket attributes",
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Status: "ok",
		Data: gin.H{
			"name":                        attrs.Name,
			"location":                    attrs.Location,
			"location_type":               attrs.LocationType,
			"storage_class":               attrs.StorageClass,
			"created":                     attrs.Created,
			"meta_generation":             attrs.MetaGeneration,
			"default_event_based_hold":    attrs.DefaultEventBasedHold,
			"retention_policy":            attrs.RetentionPolicy,
			"uniform_bucket_level_access": attrs.UniformBucketLevelAccess.Enabled,
		},
	})
}

func (h *StorageHandler) CountObjects(c *gin.Context) {
	prefix := c.Query("prefix")
	count, err := h.gcClient.CountObjects(c.Request.Context(), h.bucketName, prefix)
	if err != nil {
		log.Printf("failed to iterate objects with prefix %s: %v", prefix, err)
		c.JSON(http.StatusInternalServerError, Response{
			Status:  "error",
			Message: "failed to iterate objects",
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Status: "ok",
		Data: gin.H{
			"bucket_name":  h.bucketName,
			"prefix":       prefix,
			"object_count": count,
		},
	})
}
