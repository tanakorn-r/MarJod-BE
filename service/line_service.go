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
	"net/http"
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
