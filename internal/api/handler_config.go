package api

import (
	"net/http"
	"telegram-music/internal/config"

	"github.com/gin-gonic/gin"
)

func (s *Server) getConfig(c *gin.Context) {
	cfg := s.config.Get()
	c.JSON(http.StatusOK, cfg)
}

func (s *Server) saveConfig(c *gin.Context) {
	var cfg config.Config
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid data"})
		return
	}
	if err := s.config.Save(cfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "配置已保存"})
}
