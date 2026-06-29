package proxy

import (
	"context"
)

// Validator checks blocks against rules and returns a Decision.
// A PassValidator is used for v0 — everything passes through.
// In v0.2+, this becomes the rules engine backed by an LLM judge.
type Validator interface {
	// ValidateText checks a text block against all rules.
	ValidateText(ctx context.Context, block Block) (Decision, error)

	// ValidateToolCall checks a tool call block against all rules.
	ValidateToolCall(ctx context.Context, block Block) (Decision, error)
}

// PassValidator allows everything through without inspection.
type PassValidator struct{}

func (p *PassValidator) ValidateText(ctx context.Context, block Block) (Decision, error) {
	return Decision{Allowed: true}, nil
}

func (p *PassValidator) ValidateToolCall(ctx context.Context, block Block) (Decision, error) {
	return Decision{Allowed: true}, nil
}
