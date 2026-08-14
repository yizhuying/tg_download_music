package telegram

import (
	"context"
	"errors"
	"fmt"

	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/tg"
	"github.com/gotd/td/tgerr"
)

// SendCodeResult contains the phone_code_hash needed for SignIn.
type SendCodeResult struct {
	PhoneCodeHash string
}

// SendCode sends a verification code to the given phone number.
func (c *Client) SendCode(ctx context.Context, phone string) (*SendCodeResult, error) {
	req := &tg.AuthSendCodeRequest{
		PhoneNumber: phone,
		APIID:       c.cfg.APIID,
		APIHash:     c.cfg.APIHash,
		Settings:    tg.CodeSettings{},
	}

	sentCode, err := c.client.API().AuthSendCode(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("send code: %w", err)
	}

	switch s := sentCode.(type) {
	case *tg.AuthSentCode:
		return &SendCodeResult{PhoneCodeHash: s.PhoneCodeHash}, nil
	default:
		// Fallback for other AuthSentCodeClass implementations
		return &SendCodeResult{PhoneCodeHash: ""}, nil
	}
}

// SignInResult holds the outcome of a sign-in attempt.
type SignInResult struct {
	Need2FA bool
	Success bool
	User    string
}

// SignIn attempts to authenticate with the verification code.
// Returns Need2FA=true if a 2FA password is required.
func (c *Client) SignIn(ctx context.Context, phone, code, phoneCodeHash string) (*SignInResult, error) {
	req := &tg.AuthSignInRequest{
		PhoneNumber:   phone,
		PhoneCodeHash: phoneCodeHash,
	}
	req.SetPhoneCode(code)

	auth, err := c.client.API().AuthSignIn(ctx, req)
	if err != nil {
		if isPasswordError(err) {
			return &SignInResult{Need2FA: true}, nil
		}
		return nil, fmt.Errorf("sign in: %w", err)
	}

	userName := extractUserNameFromAuth(auth)
	return &SignInResult{Success: true, User: userName}, nil
}

// SignInWithPassword completes sign-in with the 2FA cloud password via the
// SRP flow. The verification code must already have been accepted by SignIn
// (which reported Need2FA); the code is consumed at that point, so only the
// password is needed here.
func (c *Client) SignInWithPassword(ctx context.Context, password string) (*SignInResult, error) {
	authz, err := c.client.Auth().Password(ctx, password)
	if err != nil {
		if errors.Is(err, auth.ErrPasswordInvalid) {
			return nil, fmt.Errorf("两步验证密码错误")
		}
		return nil, fmt.Errorf("sign in with password: %w", err)
	}
	return &SignInResult{Success: true, User: extractUserNameFromAuth(authz)}, nil
}

func isPasswordError(err error) bool {
	var te *tgerr.Error
	if errors.As(err, &te) {
		return te.Type == "SESSION_PASSWORD_NEEDED"
	}
	return false
}

func extractUserNameFromAuth(auth tg.AuthAuthorizationClass) string {
	a, ok := auth.(*tg.AuthAuthorization)
	if !ok || a == nil {
		return ""
	}
	u, ok := a.User.(*tg.User)
	if !ok || u == nil {
		return ""
	}
	if u.FirstName != "" {
		return u.FirstName
	}
	return u.Username
}
