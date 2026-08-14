package api

import (
	"runtime"

	"github.com/yizhuying/tg-music/internal/response"

	"github.com/gin-gonic/gin"
)

func (s *Server) getSystemInfo(c *gin.Context) {
	response.OK(c, gin.H{
		"version":    s.version,
		"build_time": s.buildTime,
		"go_version": runtime.Version(),
		"os":         runtime.GOOS,
		"arch":       runtime.GOARCH,
	})
}
