package ai

import (
	"context"
	"time"
)

const (
	DefaultTimeout        = 30 * time.Second
	DeepSeek       string = "deepseek"
	OpenAI         string = "openai"
)

type (
	// AiClient is the interface for AI chatbot clients.
	AiClient interface {
		// ChatCompletion returns the completion of the given input text.
		ChatCompletion(context.Context, string) (string, error)
		// StreamCompletion returns a channel that streams the completion of the given input text.
		StreamCompletion(context.Context, string) (<-chan string, error)
		// Check checks the health of the AI chatbot client.
		Check(context.Context) error
	}

	// Message is a message
	Message struct {
		Role    string `json:"role"` // system/user/assistant
		Content string `json:"content"`
	}
)
