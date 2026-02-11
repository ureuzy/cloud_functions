package main

import (
	"context"
	"log"
	"net/http"

	firebase "firebase.google.com/go/v4"
	"github.com/gin-gonic/gin"
	"github.com/ureuzy/cloud_functions/common-api/config"
)

type Response struct {
	Status  string      `json:"status"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

func main() {
	ctx := context.Background()
	conf, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	// Firebase Appの初期化（フロントエンドが属するプロジェクトIDを指定）
	fbApp, err := firebase.NewApp(ctx, &firebase.Config{ProjectID: conf.FirebaseProjectID})
	if err != nil {
		log.Fatalf("error initializing firebase app: %v\n", err)
	}

	gcClient, err := NewGoogleCloudClient(ctx, conf.ProjectID)
	if err != nil {
		log.Fatalf("failed to create google cloud client: %v", err)
	}
	defer gcClient.Close()

	// ハンドラーの初期化
	storageHandler := NewStorageHandler(gcClient, conf.BucketName)
	schedulerHandler := NewSchedulerHandler(gcClient, conf.ProjectID)

	// Ginの設定
	r := gin.Default()

	// ミドルウェアの適用
	r.Use(CORSMiddleware())

	// ヘルスチェック（認証なし）
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, Response{Status: "ok"})
	})

	r.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, Response{
			Status:  "ok",
			Message: "Common API is running",
		})
	})

	// 認証が必要なAPIグループ
	api := r.Group("/api/v1")
	api.Use(AuthMiddleware(fbApp))
	{
		// Schedulerグループ
		schedulerGroup := api.Group("/scheduler")
		{
			schedulerGroup.GET("/jobs", schedulerHandler.ListJobs)
			schedulerGroup.POST("/pause", schedulerHandler.PauseJob)
			schedulerGroup.POST("/resume", schedulerHandler.ResumeJob)
			schedulerGroup.POST("/run", schedulerHandler.RunJob)
		}

		// Storageグループ
		storage := api.Group("/storage")
		{
			storage.GET("/bucket", storageHandler.GetBucketAttrs)
			storage.GET("/count", storageHandler.CountObjects)
		}
	}

	log.Printf("Starting server on port %s", conf.Port)
	if err := r.Run(":" + conf.Port); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}
