package api

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, X-Admin-Password")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func authMiddleware() gin.HandlerFunc {
	expected := os.Getenv("ADMIN_PASSWORD")

	return func(c *gin.Context) {
		if expected == "" {
			c.Next()
			return
		}
		pwd := c.GetHeader("X-Admin-Password")
		if pwd == "" || pwd != expected {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			c.Abort()
			return
		}
		c.Next()
	}
}
