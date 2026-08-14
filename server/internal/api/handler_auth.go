package api

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/yizhuying/tg-music/internal/response"
	"github.com/yizhuying/tg-music/internal/telegram"

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

	response.OK(c, gin.H{
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
		response.BadRequest(c, "请输入手机号")
		return
	}

	cfg := s.config.Get()
	cfg.PhoneNumber = req.PhoneNumber
	_ = s.config.Save(cfg)

	result, err := s.tgClient.SendCode(context.Background(), req.PhoneNumber)
	if err != nil {
		if strings.Contains(err.Error(), "AUTH_RESTART") {
			s.tgClient = nil
			s.cancelRunning = nil
			if err := s.ensureTelegramClient(); err != nil {
				response.ServerError(c, err.Error())
				return
			}
			result, err = s.tgClient.SendCode(context.Background(), req.PhoneNumber)
		}
		if err != nil {
			response.BadRequest(c, err.Error())
			return
		}
	}

	s.authStateMu.Lock()
	s.authState = AuthState{
		PhoneCodeHash: result.PhoneCodeHash,
		PhoneNumber:   req.PhoneNumber,
	}
	s.authStateMu.Unlock()

	response.OKMsgData(c, "验证码已发送", gin.H{"phone_code_hash": result.PhoneCodeHash})
}

// signIn handles the sign-in process using the phone code and optional password for 2FA.
func (s *Server) signIn(c *gin.Context) {
	var req struct {
		PhoneCode string `json:"phone_code" binding:"required"`
		Password  string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "请输入验证码")
		return
	}

	s.authStateMu.Lock()
	state := s.authState
	s.authStateMu.Unlock()

	if state.PhoneCodeHash == "" {
		response.BadRequest(c, "请先发送验证码")
		return
	}

	var result *telegram.SignInResult
	var signinErr error
	if req.Password != "" {
		result, signinErr = s.tgClient.SignInWithPassword(context.Background(), req.Password)
	} else {
		result, signinErr = s.tgClient.SignIn(context.Background(), state.PhoneNumber, req.PhoneCode, state.PhoneCodeHash)
	}
	if signinErr != nil {
		response.BadRequest(c, signinErr.Error())
		return
	}

	if result.Need2FA {
		response.OKMsgData(c, "需要输入两步验证密码", gin.H{"need_2fa": true})
		return
	}

	if result.Success {
		s.authStateMu.Lock()
		s.authState = AuthState{}
		s.authStateMu.Unlock()
		response.OKMsg(c, "登录成功")
		return
	}

	response.BadRequest(c, "登录失败")
}

func (s *Server) logout(c *gin.Context) {
	s.authStateMu.Lock()
	s.authState = AuthState{}
	s.authStateMu.Unlock()

	if s.cancelRunning != nil {
		s.cancelRunning()
		s.cancelRunning = nil
	}
	if s.downloadManager != nil {
		s.downloadManager.Close()
	}
	s.tgClient = nil
	s.downloadManager = nil

	cfg := s.config.Get()
	sessionPath := filepath.Join(cfg.SessionDir, cfg.SessionName+".json")
	_ = os.Remove(sessionPath)

	response.OKMsg(c, "已退出登录")
}
