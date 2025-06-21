package sse

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"strings"

	"gateway/internal/core"
	"gateway/pkg/errors"
)

// writer implements core.SSEWriter
type writer struct {
	w       io.Writer
	flusher http.Flusher
	buf     *bufio.Writer
	closed  bool
}

// newWriter creates a new SSE writer
func newWriter(w io.Writer) *writer {
	flusher, _ := w.(http.Flusher)
	return &writer{
		w:       w,
		flusher: flusher,
		buf:     bufio.NewWriter(w),
	}
}

// WriteEvent writes an SSE event
func (w *writer) WriteEvent(event *core.SSEEvent) error {
	if w.closed {
		return errors.NewError(errors.ErrorTypeInternal, "SSE writer is closed")
	}

	// Write event ID if present
	if event.ID != "" {
		if _, err := fmt.Fprintf(w.buf, "id: %s\n", event.ID); err != nil {
			return errors.NewError(errors.ErrorTypeInternal, "failed to write SSE event ID").WithCause(err)
		}
	}

	// Write event type if present
	if event.Type != "" {
		if _, err := fmt.Fprintf(w.buf, "event: %s\n", event.Type); err != nil {
			return errors.NewError(errors.ErrorTypeInternal, "failed to write SSE event type").WithCause(err)
		}
	}

	// Write retry if present
	if event.Retry > 0 {
		if _, err := fmt.Fprintf(w.buf, "retry: %d\n", event.Retry); err != nil {
			return errors.NewError(errors.ErrorTypeInternal, "failed to write SSE retry").WithCause(err)
		}
	}

	// Write data lines
	if event.Data != "" {
		lines := strings.Split(event.Data, "\n")
		for _, line := range lines {
			if _, err := fmt.Fprintf(w.buf, "data: %s\n", line); err != nil {
				return errors.NewError(errors.ErrorTypeInternal, "failed to write SSE data").WithCause(err)
			}
		}
	}

	// Write comment if present
	if event.Comment != "" {
		if _, err := fmt.Fprintf(w.buf, ": %s\n", event.Comment); err != nil {
			return errors.NewError(errors.ErrorTypeInternal, "failed to write SSE comment").WithCause(err)
		}
	}

	// End event with blank line
	if _, err := w.buf.WriteString("\n"); err != nil {
		return errors.NewError(errors.ErrorTypeInternal, "failed to write SSE event terminator").WithCause(err)
	}

	return w.Flush()
}

// WriteComment writes a comment (useful for keepalive)
func (w *writer) WriteComment(comment string) error {
	if w.closed {
		return errors.NewError(errors.ErrorTypeInternal, "SSE writer is closed")
	}

	if _, err := fmt.Fprintf(w.buf, ": %s\n", comment); err != nil {
		return errors.NewError(errors.ErrorTypeInternal, "failed to write SSE comment").WithCause(err)
	}

	return w.Flush()
}

// Flush flushes any buffered data
func (w *writer) Flush() error {
	if err := w.buf.Flush(); err != nil {
		return errors.NewError(errors.ErrorTypeInternal, "failed to flush SSE buffer").WithCause(err)
	}

	if w.flusher != nil {
		w.flusher.Flush()
	}

	return nil
}

// Close closes the writer
func (w *writer) Close() error {
	if w.closed {
		return nil
	}

	w.closed = true
	return w.Flush()
}