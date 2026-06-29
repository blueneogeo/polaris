package proxy

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
)

// BlockWriter serializes blocks back to SSE format and writes them
// to the HTTP response writer with proper flushing.
type BlockWriter struct {
	w       http.ResponseWriter
	flusher http.Flusher
	logger  *slog.Logger
}

// NewBlockWriter creates a BlockWriter for the given HTTP response writer.
func NewBlockWriter(w http.ResponseWriter, flusher http.Flusher, logger *slog.Logger) *BlockWriter {
	return &BlockWriter{w: w, flusher: flusher, logger: logger}
}

// WriteBlock serializes all chunks in a block to SSE format and writes
// them to the response, flushing after each chunk.
func (bw *BlockWriter) WriteBlock(block Block) error {
	for _, chunk := range block.Chunks {
		data, err := json.Marshal(chunk)
		if err != nil {
			bw.logger.Error("failed to marshal chunk for SSE output", "error", err)
			return fmt.Errorf("marshal SSE chunk: %w", err)
		}
		if _, err := fmt.Fprintf(bw.w, "data: %s\n\n", data); err != nil {
			bw.logger.Error("failed to write SSE data line", "error", err)
			return fmt.Errorf("write SSE data: %w", err)
		}
		bw.flusher.Flush()
	}
	return nil
}

// WriteDone writes the SSE [DONE] marker and flushes.
func (bw *BlockWriter) WriteDone() error {
	_, err := io.WriteString(bw.w, "data: [DONE]\n\n")
	if err != nil {
		return err
	}
	bw.flusher.Flush()
	return nil
}

// WritePolarisThinking writes a Polaris thinking indicator as an SSE data line.
// This is emitted while Polaris is waiting for a validator decision.
func (bw *BlockWriter) WritePolarisThinking(message string) error {
	data := fmt.Sprintf(`[Polaris thinking: %s]`, message)
	_, err := fmt.Fprintf(bw.w, "data: %s\n\n", data)
	if err != nil {
		return err
	}
	bw.flusher.Flush()
	return nil
}

// WriteJSONResponse writes a non-streaming JSON response to the client.
func (bw *BlockWriter) WriteJSONResponse(completion *json.RawMessage) error {
	data, err := json.Marshal(completion)
	if err != nil {
		return fmt.Errorf("marshal completion response: %w", err)
	}
	_, err = bw.w.Write(data)
	return err
}

// Ensure flusher interface check
var _ http.Flusher = (http.Flusher)(nil)
