package api

import (
	"net/http"

	"github.com/yizhuying/tg-music/internal/response"

	"github.com/gin-gonic/gin"
)

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func (s *Server) requireTelegramClient() gin.HandlerFunc {
	return func(c *gin.Context) {
		if s.tgClient == nil {
			if err := s.ensureTelegramClient(); err != nil {
				response.BadRequest(c, "客户端未建立连接，请先完成 Telegram 认证")
				c.Abort()
				return
			}
		}
		c.Next()
	}
}

func (s *Server) initTelegramClient() gin.HandlerFunc {
	return func(c *gin.Context) {
		if s.tgClient == nil {
			if err := s.ensureTelegramClient(); err != nil {
				response.ServerError(c, err.Error())
				c.Abort()
				return
			}
		}
		c.Next()
	}
}
