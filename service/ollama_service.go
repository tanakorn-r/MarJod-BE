package service

import (
	"bufio"
	"bytes"
	"encoding/json"
	"finance-chat/config"
	"fmt"
	"io"
	"net/http"
	"sync"
)

// LLMClient is a generic interface for any LLM backend.
// It knows nothing about finance — just sends a prompt and returns text.
type LLMClient interface {
	// Complete sends a prompt and waits for the full response.
	Complete(prompt string) (string, error)

	// CompleteStream sends a prompt and returns a channel of token strings.
	// The caller must drain the channel. A non-nil error is sent as the last
	// value prefixed with "error:" if something goes wrong mid-stream.
	CompleteStream(prompt string) (<-chan string, error)
}

type ollamaClient struct {
	cfg    *config.Config
	client *http.Client
}

var (
	ollamaInstance LLMClient
	ollamaOnce     sync.Once
)

func NewOllamaClient(cfg *config.Config) LLMClient {
	ollamaOnce.Do(func() {
		ollamaInstance = &ollamaClient{
			cfg:    cfg,
			client: &http.Client{},
		}
	})
	return ollamaInstance
}

type ollamaRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

type ollamaChunk struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
}

// Complete — blocking, stream:false
func (o *ollamaClient) Complete(prompt string) (string, error) {
	body, _ := json.Marshal(ollamaRequest{Model: o.cfg.OllamaModel, Prompt: prompt, Stream: false})

	resp, err := o.client.Post(o.cfg.OllamaURL+"/api/generate", "application/json", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("ollama request failed: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading ollama response: %w", err)
	}

	var chunk ollamaChunk
	if err := json.Unmarshal(raw, &chunk); err != nil {
		return "", fmt.Errorf("parsing ollama response: %w", err)
	}

	return chunk.Response, nil
}

// CompleteStream — non-blocking, stream:true. Returns a channel of tokens.
// Channel is closed when the model signals done or an error occurs.
func (o *ollamaClient) CompleteStream(prompt string) (<-chan string, error) {
	body, _ := json.Marshal(ollamaRequest{Model: o.cfg.OllamaModel, Prompt: prompt, Stream: true})

	resp, err := o.client.Post(o.cfg.OllamaURL+"/api/generate", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("ollama stream request failed: %w", err)
	}

	ch := make(chan string, 32)

	go func() {
		defer close(ch)
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}

			var chunk ollamaChunk
			if err := json.Unmarshal(line, &chunk); err != nil {
				ch <- fmt.Sprintf("error: failed to parse chunk: %v", err)
				return
			}

			if chunk.Response != "" {
				ch <- chunk.Response
			}

			if chunk.Done {
				return
			}
		}

		if err := scanner.Err(); err != nil {
			ch <- fmt.Sprintf("error: stream read error: %v", err)
		}
	}()

	return ch, nil
}
