package main

import (
	"context"
	"log"
	"net/http"
	"strings"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
	"github.com/gin-gonic/gin"
)

func AuthMiddleware(app *firebase.App) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, Response{Status: "error", Message: "Authorization header is required"})
			c.Abort()
			return
		}

		idToken := strings.TrimSpace(strings.Replace(authHeader, "Bearer", "", 1))
		client, err := app.Auth(context.Background())
		if err != nil {
			log.Printf("error getting Auth client: %v\n", err)
			c.JSON(http.StatusInternalServerError, Response{Status: "error", Message: "Internal server error"})
			c.Abort()
			return
		}

		token, err := client.VerifyIDToken(context.Background(), idToken)
		if err != nil {
			log.Printf("error verifying ID token: %v. Token prefix: %s...", err, idToken[:10])
			c.JSON(http.StatusUnauthorized, Response{Status: "error", Message: "Invalid token: " + err.Error()})
			c.Abort()
			return
		}

		// ユーザー情報をコンテキストに保存
		c.Set("user", token)
		c.Next()
	}
}

func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

func GetUser(c *gin.Context) *auth.Token {
	if user, exists := c.Get("user"); exists {
		if token, ok := user.(*auth.Token); ok {
			return token
		}
	}
	return nil
}
