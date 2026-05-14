package api

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"tg-music/internal/telegram"

	"github.com/gin-gonic/gin"
)

// AuthState holds intermediate authentication state between API calls.
type AuthState struct {
	PhoneCodeHash string
	PhoneNumber   string
}

func (s *Server) getAuthStatus(c *gin.Context) {
	cfg := s.config.Get()
	sessionPath := filepath.Join(cfg.SessionDir, cfg.SessionName+".json")
	_, err := os.Stat(sessionPath)
	exists := err == nil

	c.JSON(http.StatusOK, gin.H{
		"authorized":   exists,
		"phone_number": cfg.PhoneNumber,
		"needs_2fa":    false,
	})
}

func (s *Server) sendCode(c *gin.Context) {
	var req struct {
		PhoneNumber string `json:"phone_number" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请输入手机号"})
		return
	}

	cfg := s.config.Get()
	cfg.PhoneNumber = req.PhoneNumber
	_ = s.config.Save(cfg)

	if err := s.ensureTelegramClient(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	result, err := s.tgClient.SendCode(context.Background(), req.PhoneNumber)
	if err != nil {
		// AUTH_RESTART means we need to retry after resetting the client
		if strings.Contains(err.Error(), "AUTH_RESTART") {
			s.tgClient = nil
			s.cancelRunning = nil
			if err := s.ensureTelegramClient(); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}
			result, err = s.tgClient.SendCode(context.Background(), req.PhoneNumber)
		}
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}

	s.authStateMu.Lock()
	s.authState = AuthState{
		PhoneCodeHash: result.PhoneCodeHash,
		PhoneNumber:   req.PhoneNumber,
	}
	s.authStateMu.Unlock()

	c.JSON(http.StatusOK, gin.H{
		"ok":              true,
		"message":         "验证码已发送",
		"phone_code_hash": result.PhoneCodeHash,
	})
}

func (s *Server) signIn(c *gin.Context) {
	var req struct {
		PhoneCode string `json:"phone_code" binding:"required"`
		Password  string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请输入验证码"})
		return
	}

	s.authStateMu.Lock()
	state := s.authState
	s.authStateMu.Unlock()

	if state.PhoneCodeHash == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请先发送验证码"})
		return
	}

	if err := s.ensureTelegramClient(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var result *telegram.SignInResult
	var err error
	if req.Password != "" {
		result, err = s.tgClient.SignInWithPassword(context.Background(), state.PhoneNumber, req.PhoneCode, state.PhoneCodeHash, req.Password)
	} else {
		result, err = s.tgClient.SignIn(context.Background(), state.PhoneNumber, req.PhoneCode, state.PhoneCodeHash)
	}
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if result.Need2FA {
		c.JSON(http.StatusOK, gin.H{"need_2fa": true, "message": "需要输入两步验证密码"})
		return
	}

	if result.Success {
		s.authStateMu.Lock()
		s.authState = AuthState{}
		s.authStateMu.Unlock()
		c.JSON(http.StatusOK, gin.H{"ok": true, "message": "登录成功"})
		return
	}

	c.JSON(http.StatusBadRequest, gin.H{"error": "登录失败"})
}

func (s *Server) logout(c *gin.Context) {
	s.authStateMu.Lock()
	s.authState = AuthState{}
	s.authStateMu.Unlock()

	if s.cancelRunning != nil {
		s.cancelRunning()
		s.cancelRunning = nil
	}
	s.tgClient = nil
	s.downloadManager = nil

	cfg := s.config.Get()
	sessionPath := filepath.Join(cfg.SessionDir, cfg.SessionName+".json")
	_ = os.Remove(sessionPath)

	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "已退出登录"})
}
