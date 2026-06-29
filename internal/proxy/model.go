package proxy

import (
	"context"

	"github.com/openai/openai-go/v3"
)

// Model is the interface for calling an AI model. Polaris uses this
// for its own internal reasoning (judging, prompt enhancement, etc.).
// The worker model is called through the upstream proxy, not through this interface.
type Model interface {
	// Complete sends a chat completion request and returns the full response.
	Complete(ctx context.Context, params openai.ChatCompletionNewParams) (*openai.ChatCompletion, error)
}

// EchoModel is a fake model that echoes back the user's message for development.
type EchoModel struct{}

func (e *EchoModel) Complete(ctx context.Context, params openai.ChatCompletionNewParams) (*openai.ChatCompletion, error) {
	content := "echo"
	if len(params.Messages) > 0 {
		// Extract the last user message content for the echo
		for i := len(params.Messages) - 1; i >= 0; i-- {
			msg := params.Messages[i]
			if contentStr := getMessageContent(msg); contentStr != "" {
				content = contentStr
				break
			}
		}
	}
	id := "echo-1"
	return &openai.ChatCompletion{
		ID:    id,
		Model: params.Model,
		Choices: []openai.ChatCompletionChoice{
			{
				Index: 0,
				Message: openai.ChatCompletionMessage{
					Content: content,
				},
			},
		},
	}, nil
}

func getMessageContent(msg openai.ChatCompletionMessageParamUnion) string {
	// Try to extract content from common message types
	if msg.OfUser != nil {
		if msg.OfUser.Content.OfString.Valid() {
			return msg.OfUser.Content.OfString.Value
		}
	}
	if msg.OfAssistant != nil {
		if msg.OfAssistant.Content.OfString.Valid() {
			return msg.OfAssistant.Content.OfString.Value
		}
	}
	if msg.OfDeveloper != nil {
		if msg.OfDeveloper.Content.OfString.Valid() {
			return msg.OfDeveloper.Content.OfString.Value
		}
	}
	if msg.OfSystem != nil {
		if msg.OfSystem.Content.OfString.Valid() {
			return msg.OfSystem.Content.OfString.Value
		}
	}
	return ""
}
