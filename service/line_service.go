package service

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"finance-chat/agent"
	"finance-chat/config"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

type lineService struct {
	cfg    *config.Config
	client *http.Client
}

var (
	lineInstance agent.LineService
	lineOnce     sync.Once
)

func NewLineService(cfg *config.Config) agent.LineService {
	lineOnce.Do(func() {
		lineInstance = &lineService{cfg: cfg, client: &http.Client{}}
	})
	return lineInstance
}

// VerifySignature validates the X-Line-Signature header against the channel secret.
func (s *lineService) VerifySignature(body []byte, signature string) bool {
	mac := hmac.New(sha256.New, []byte(s.cfg.LineChannelSecret))
	mac.Write(body)
	expected := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signature))
}

// ReplyMessage sends a text reply back to the user via the Line Messaging API.
func (s *lineService) ReplyMessage(replyToken, text string) error {
	payload := map[string]any{
		"replyToken": replyToken,
		"messages": []map[string]string{
			{"type": "text", "text": text},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal reply payload: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost,
		"https://api.line.me/v2/bot/message/reply",
		bytes.NewReader(body),
	)
	if err != nil {
		return fmt.Errorf("create reply request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.cfg.LineChannelToken)

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("send reply: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("line reply API returned %d", resp.StatusCode)
	}

	return nil
}

// verifyIDTokenResponse mirrors the payload returned by LINE's ID token
// verification endpoint. See: https://developers.line.biz/en/reference/liff/#verify-id-token
type verifyIDTokenResponse struct {
	Sub   string `json:"sub"`
	Aud   string `json:"aud"`
	Error string `json:"error"`
}

// VerifyIDToken validates a LIFF ID token against LINE's verify endpoint and
// returns the verified LINE userId (the token's "sub" claim). The token's
// audience must match the configured LIFF channel ID.
func (s *lineService) VerifyIDToken(idToken string) (string, error) {
	if strings.TrimSpace(idToken) == "" {
		return "", fmt.Errorf("empty id token")
	}
	if s.cfg.LineLiffChannelID == "" {
		return "", fmt.Errorf("LINE_LIFF_CHANNEL_ID is not configured")
	}

	form := url.Values{
		"id_token":  {idToken},
		"client_id": {s.cfg.LineLiffChannelID},
	}

	resp, err := s.client.PostForm("https://api.line.me/oauth2/v2.1/verify", form)
	if err != nil {
		return "", fmt.Errorf("verify id token: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read verify response: %w", err)
	}

	var result verifyIDTokenResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("parse verify response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		if result.Error != "" {
			return "", fmt.Errorf("id token rejected: %s", result.Error)
		}
		return "", fmt.Errorf("id token verification returned %d", resp.StatusCode)
	}

	if result.Aud != s.cfg.LineLiffChannelID {
		return "", fmt.Errorf("id token audience mismatch")
	}
	if result.Sub == "" {
		return "", fmt.Errorf("id token has no sub claim")
	}

	return result.Sub, nil
}
