package service

import (
	"context"
	"finance-chat/agent"
	"finance-chat/config"
	"fmt"
	"sync"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

type openaiClient struct {
	client *openai.Client
	model  string
}

var (
	openaiInstance         agent.LLMClient
	openaiOnce             sync.Once
	openaiInternalInstance agent.LLMClient
	openaiInternalOnce     sync.Once
)

// NewOpenAIClient creates a singleton OpenAI LLM client.
func NewOpenAIClient(cfg *config.Config) agent.LLMClient {
	openaiOnce.Do(func() {
		openaiInstance = newOpenAIClient(cfg.OpenAIAPIKey, cfg.OpenAIModel)
	})
	return openaiInstance
}

// NewOpenAIInternalClient creates the client reserved for internal features
// such as dashboard analysis, keeping that usage separate from chat parsing.
func NewOpenAIInternalClient(cfg *config.Config) agent.LLMClient {
	openaiInternalOnce.Do(func() {
		openaiInternalInstance = newOpenAIClient(cfg.OpenAIAPIKeyInternal, cfg.OpenAIModel)
	})
	return openaiInternalInstance
}

func newOpenAIClient(apiKey, model string) agent.LLMClient {
	c := openai.NewClient(option.WithAPIKey(apiKey))
	return &openaiClient{client: &c, model: model}
}

// Complete sends a prompt and waits for the full response.
// maxTokens caps output length; pass 0 to use the model default.
func (o *openaiClient) Complete(prompt string) (string, error) {
	return o.completeWithLimit(prompt, 0)
}

// CompleteWithTokenLimit is like Complete but enforces a hard output token cap.
func (o *openaiClient) CompleteWithTokenLimit(prompt string, maxTokens int) (string, error) {
	return o.completeWithLimit(prompt, maxTokens)
}

func (o *openaiClient) completeWithLimit(prompt string, maxTokens int) (string, error) {
	ctx := context.Background()

	params := openai.ChatCompletionNewParams{
		Model: o.model,
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(prompt),
		},
	}
	if maxTokens > 0 {
		params.MaxTokens = openai.Int(int64(maxTokens))
	}

	resp, err := o.client.Chat.Completions.New(ctx, params)
	if err != nil {
		return "", fmt.Errorf("openai complete: %w", err)
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("openai complete: no choices returned")
	}
	return resp.Choices[0].Message.Content, nil
}

// CompleteStream sends a prompt and streams tokens back over a channel.
// The channel is closed when the model finishes or an error occurs.
func (o *openaiClient) CompleteStream(prompt string) (<-chan string, error) {
	ch := make(chan string, 32)

	go func() {
		defer close(ch)

		ctx := context.Background()
		stream := o.client.Chat.Completions.NewStreaming(ctx, openai.ChatCompletionNewParams{
			Model: o.model,
			Messages: []openai.ChatCompletionMessageParamUnion{
				openai.UserMessage(prompt),
			},
		})

		for stream.Next() {
			chunk := stream.Current()
			if len(chunk.Choices) > 0 {
				if delta := chunk.Choices[0].Delta.Content; delta != "" {
					ch <- delta
				}
			}
		}
		if err := stream.Err(); err != nil {
			ch <- fmt.Sprintf("error: openai stream: %v", err)
		}
	}()

	return ch, nil
}
