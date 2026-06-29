package proxy

import (
	"context"

	"github.com/openai/openai-go/v3"
)

// PromptEnhancer modifies a chat completion request before it's forwarded
// to the worker model. This is where Polaris cleans up TTS mistakes,
// injects relevant rules as context, and adds preferences from memory.
type PromptEnhancer interface {
	// Enhance modifies the params in place. Called before the request
	// is forwarded to the upstream worker model.
	Enhance(ctx context.Context, params *openai.ChatCompletionNewParams) error
}

// NoopEnhancer does nothing — the request passes through unchanged.
type NoopEnhancer struct{}

func (n *NoopEnhancer) Enhance(ctx context.Context, params *openai.ChatCompletionNewParams) error {
	return nil
}
