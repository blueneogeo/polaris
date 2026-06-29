package proxy

import (
	"net/http"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/packages/ssestream"
)

// ── BlockDetector ──────────────────────────────────────────────────

// BlockDetector reads an SSE stream from an upstream response and emits
// blocks (contiguous groups of chunks) at natural boundaries:
// thinking → text, text → tool, tool → text, etc.
type BlockDetector struct {
	stream      *ssestream.Stream[openai.ChatCompletionChunk]
	currentType BlockType
	acc         []openai.ChatCompletionChunk
	nextChunk   *openai.ChatCompletionChunk
	streamDone  bool
	err         error
}

// NewBlockDetector creates a BlockDetector from an upstream HTTP response.
func NewBlockDetector(resp *http.Response) *BlockDetector {
	stream := ssestream.NewStream[openai.ChatCompletionChunk](
		ssestream.NewDecoder(resp),
		nil,
	)
	return &BlockDetector{stream: stream}
}

// Next advances to the next block. Returns false when the stream is exhausted
// or an error occurred. Call Err() to check for errors after Next returns false.
func (d *BlockDetector) Next() bool {
	if d.err != nil || d.streamDone {
		return false
	}

	// Start a new block
	d.acc = d.acc[:0]
	blockStarted := false

	// If we have a chunk from a previous type-change boundary, start with it
	if d.nextChunk != nil {
		d.acc = append(d.acc, *d.nextChunk)
		d.currentType = classifyChunk(*d.nextChunk)
		blockStarted = true
		d.nextChunk = nil
	}

	for d.stream.Next() {
		chunk := d.stream.Current()
		chunkType := classifyChunk(chunk)

		if !blockStarted {
			d.currentType = chunkType
			d.acc = append(d.acc, chunk)
			blockStarted = true
			continue
		}

		// If type changed and the new type is different, finish current block
		if chunkType != "" && chunkType != d.currentType {
			// Save this chunk as the start of the next block
			c := chunk
			d.nextChunk = &c
			return true
		}

		d.acc = append(d.acc, chunk)
	}

	// Stream ended — emit whatever we accumulated
	d.streamDone = true
	return blockStarted
}

// Err returns the first error encountered during iteration.
func (d *BlockDetector) Err() error {
	if d.err != nil {
		return d.err
	}
	return d.stream.Err()
}

// Block returns the current block. Call only after Next() returns true.
func (d *BlockDetector) Block() Block {
	return Block{
		Type:   d.currentType,
		Chunks: d.acc,
	}
}

// ── Chunk classification ───────────────────────────────────────────

func classifyChunk(chunk openai.ChatCompletionChunk) BlockType {
	if len(chunk.Choices) == 0 {
		return BlockText
	}
	delta := chunk.Choices[0].Delta

	// Check ExtraFields for reasoning/thinking content (provider-dependent).
	// Anthropic/openai-compatible servers send thinking in reasoning_content.
	if delta.JSON.ExtraFields != nil {
		if _, ok := delta.JSON.ExtraFields["reasoning_content"]; ok {
			return BlockThinking
		}
		if _, ok := delta.JSON.ExtraFields["thinking"]; ok {
			return BlockThinking
		}
	}

	// Check for tool calls
	if len(delta.ToolCalls) > 0 {
		return BlockToolCall
	}

	// Text content (including empty deltas)
	return BlockText
}
