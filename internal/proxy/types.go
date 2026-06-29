package proxy

import (
	"github.com/openai/openai-go/v3"
)

// BlockType classifies a contiguous group of SSE chunks.
type BlockType string

const (
	BlockText     BlockType = "text"
	BlockThinking BlockType = "thinking"
	BlockToolCall BlockType = "tool_call"
)

// Block is a contiguous group of SSE chunks that form a logical unit.
// A block is emitted when the delta type changes (text → tool, tool → text, thinking → anything).
type Block struct {
	Type   BlockType
	Chunks []openai.ChatCompletionChunk
}

// Decision is the result of validating a block against rules.
type Decision struct {
	Allowed  bool
	Reason   string
	Retry    bool   // if true, inject feedback and retry the worker model
	Feedback string // corrective feedback for the worker model, if Retry is true
}
