package sse

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"

	"gateway/internal/core"
)

// reader implements core.SSEReader
type reader struct {
	r      *bufio.Reader
	closed bool
}

// newReader creates a new SSE reader
func newReader(r io.Reader) *reader {
	return &reader{
		r: bufio.NewReader(r),
	}
}

// ReadEvent reads the next SSE event
func (r *reader) ReadEvent() (*core.SSEEvent, error) {
	if r.closed {
		return nil, fmt.Errorf("reader is closed")
	}

	event := &core.SSEEvent{}
	var dataLines []string

	for {
		line, err := r.r.ReadString('\n')
		if err != nil {
			if err == io.EOF && len(dataLines) > 0 {
				// Process any remaining data
				event.Data = strings.Join(dataLines, "\n")
				return event, nil
			}
			return nil, err
		}

		// Remove trailing newline
		line = strings.TrimSuffix(line, "\n")
		line = strings.TrimSuffix(line, "\r")

		// Empty line signals end of event
		if line == "" {
			if len(dataLines) > 0 || event.ID != "" || event.Type != "" {
				event.Data = strings.Join(dataLines, "\n")
				return event, nil
			}
			// Skip empty lines between events
			continue
		}

		// Parse field
		colonIndex := strings.Index(line, ":")
		if colonIndex == -1 {
			// Line without colon is ignored
			continue
		}

		field := line[:colonIndex]
		value := line[colonIndex+1:]

		// Remove optional space after colon
		if len(value) > 0 && value[0] == ' ' {
			value = value[1:]
		}

		// Process field
		switch field {
		case "id":
			event.ID = value
		case "event":
			event.Type = value
		case "data":
			dataLines = append(dataLines, value)
		case "retry":
			if retry, err := strconv.Atoi(value); err == nil {
				event.Retry = retry
			}
		case "":
			// Comment
			event.Comment = value
		default:
			// Unknown field, ignore
		}
	}
}

// Close closes the reader
func (r *reader) Close() error {
	r.closed = true
	return nil
}