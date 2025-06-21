package sse

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"gateway/internal/core"
	"gateway/pkg/errors"
)

// writer implements core.SSEWriter
type writer struct {
	w            io.Writer
	flusher      http.Flusher
	buf          *bufio.Writer
	closed       bool
	disconnected bool
	mu           sync.RWMutex
	ctx          context.Context
}

// newWriter creates a new SSE writer with disconnect detection
func newWriter(w io.Writer, ctx context.Context) *writer {
	flusher, _ := w.(http.Flusher)
	return &writer{
		w:       w,
		flusher: flusher,
		buf:     bufio.NewWriter(w),
		ctx:     ctx,
	}
}

// WriteEvent writes an SSE event
func (w *writer) WriteEvent(event *core.SSEEvent) error {
	w.mu.RLock()
	defer w.mu.RUnlock()

	// Check if context is done (client disconnected)
	select {
	case <-w.ctx.Done():
		w.markDisconnected()
		return errors.NewError(errors.ErrorTypeInternal, "client disconnected")
	default:
	}

	if w.closed || w.disconnected {
		return errors.NewError(errors.ErrorTypeInternal, "SSE writer is closed or disconnected")
	}

	// Write event ID if present
	if event.ID != "" {
		if _, err := fmt.Fprintf(w.buf, "id: %s\n", event.ID); err != nil {
			w.handleWriteError(err)
			return errors.NewError(errors.ErrorTypeInternal, "failed to write SSE event ID").WithCause(err)
		}
	}

	// Write event type if present
	if event.Type != "" {
		if _, err := fmt.Fprintf(w.buf, "event: %s\n", event.Type); err != nil {
			w.handleWriteError(err)
			return errors.NewError(errors.ErrorTypeInternal, "failed to write SSE event type").WithCause(err)
		}
	}

	// Write retry if present
	if event.Retry > 0 {
		if _, err := fmt.Fprintf(w.buf, "retry: %d\n", event.Retry); err != nil {
			w.handleWriteError(err)
			return errors.NewError(errors.ErrorTypeInternal, "failed to write SSE retry").WithCause(err)
		}
	}

	// Write data lines
	if event.Data != "" {
		lines := strings.Split(event.Data, "\n")
		for _, line := range lines {
			if _, err := fmt.Fprintf(w.buf, "data: %s\n", line); err != nil {
				w.handleWriteError(err)
				return errors.NewError(errors.ErrorTypeInternal, "failed to write SSE data").WithCause(err)
			}
		}
	}

	// Write comment if present
	if event.Comment != "" {
		if _, err := fmt.Fprintf(w.buf, ": %s\n", event.Comment); err != nil {
			w.handleWriteError(err)
			return errors.NewError(errors.ErrorTypeInternal, "failed to write SSE comment").WithCause(err)
		}
	}

	// End event with blank line
	if _, err := w.buf.WriteString("\n"); err != nil {
		w.handleWriteError(err)
		return errors.NewError(errors.ErrorTypeInternal, "failed to write SSE event terminator").WithCause(err)
	}

	return w.Flush()
}

// WriteComment writes a comment (useful for keepalive)
func (w *writer) WriteComment(comment string) error {
	w.mu.RLock()
	defer w.mu.RUnlock()

	// Check if context is done (client disconnected)
	select {
	case <-w.ctx.Done():
		w.markDisconnected()
		return errors.NewError(errors.ErrorTypeInternal, "client disconnected")
	default:
	}

	if w.closed || w.disconnected {
		return errors.NewError(errors.ErrorTypeInternal, "SSE writer is closed or disconnected")
	}

	if _, err := fmt.Fprintf(w.buf, ": %s\n", comment); err != nil {
		w.handleWriteError(err)
		return errors.NewError(errors.ErrorTypeInternal, "failed to write SSE comment").WithCause(err)
	}

	return w.Flush()
}

// Flush flushes any buffered data
func (w *writer) Flush() error {
	if err := w.buf.Flush(); err != nil {
		w.handleWriteError(err)
		return errors.NewError(errors.ErrorTypeInternal, "failed to flush SSE buffer").WithCause(err)
	}

	if w.flusher != nil {
		w.flusher.Flush()
	}

	return nil
}

// Close closes the writer
func (w *writer) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return nil
	}

	w.closed = true
	return w.Flush()
}

// IsDisconnected returns true if the client has disconnected
func (w *writer) IsDisconnected() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.disconnected
}

// handleWriteError handles write errors by detecting disconnection
func (w *writer) handleWriteError(err error) {
	if err != nil {
		// Common error messages that indicate client disconnection
		errStr := err.Error()
		if strings.Contains(errStr, "broken pipe") ||
			strings.Contains(errStr, "connection reset") ||
			strings.Contains(errStr, "write: connection refused") {
			w.markDisconnected()
		}
	}
}

// markDisconnected marks the writer as disconnected
func (w *writer) markDisconnected() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.disconnected = true
}