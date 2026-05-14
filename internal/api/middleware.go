package api

import (
	"net/http"
	"os"
	"tg-music/internal/config"

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

func authMiddleware(cfg *config.Manager) gin.HandlerFunc {
	expected := os.Getenv("ADMIN_PASSWORD")
	if expected == "" {
		expected = "admin"
	}

	return func(c *gin.Context) {
		pwd := c.GetHeader("X-Admin-Password")
		if pwd == "" || pwd != expected {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			c.Abort()
			return
		}
		c.Next()
	}
}
