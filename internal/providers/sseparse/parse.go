// Package sseparse provides a minimal Server-Sent Events parser for
// streaming HTTP responses returned by AI provider APIs.
package sseparse

import (
	"bufio"
	"io"
	"strings"
)

// ForEachEvent reads SSE data from r and invokes fn for every complete event.
//
// Parsing rules:
//   - Events are separated by blank lines.
//   - "event:" sets the event name for the current block.
//   - "data:" lines are accumulated; multiple data lines within one block are
//     joined with "\n".
//   - "data: [DONE]" terminates the stream — fn is NOT called and nil is returned.
//   - Lines starting with ":" are comments and are ignored.
//
// fn is called with the event name (empty string when no "event:" field was
// present) and the raw data bytes.
//
// Return values from fn:
//   - nil → continue.
//   - io.EOF → stop and return nil from ForEachEvent.
//   - any other error → stop and return that error from ForEachEvent.
func ForEachEvent(r io.Reader, fn func(event string, data []byte) error) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	var (
		eventName string
		dataBuf   strings.Builder
		hasData   bool
	)

	for scanner.Scan() {
		line := scanner.Text()

		switch {
		case line == "":
			// Blank line: dispatch the accumulated event (if any).
			if !hasData {
				continue
			}
			raw := dataBuf.String()
			name := eventName

			// Reset per-event state before calling fn so re-entrant calls work.
			eventName = ""
			dataBuf.Reset()
			hasData = false

			if raw == "[DONE]" {
				return nil
			}
			if err := fn(name, []byte(raw)); err != nil {
				if err == io.EOF {
					return nil
				}
				return err
			}

		case strings.HasPrefix(line, "data:"):
			val := strings.TrimPrefix(line, "data:")
			if len(val) > 0 && val[0] == ' ' {
				val = val[1:]
			}
			if hasData {
				dataBuf.WriteByte('\n')
			}
			dataBuf.WriteString(val)
			hasData = true

		case strings.HasPrefix(line, "event:"):
			val := strings.TrimPrefix(line, "event:")
			if len(val) > 0 && val[0] == ' ' {
				val = val[1:]
			}
			eventName = val

		// id:, retry:, and comments (":") are silently ignored.
		}
	}

	// Dispatch a trailing event that was not followed by a blank line.
	if hasData {
		raw := dataBuf.String()
		if raw != "[DONE]" {
			if err := fn(eventName, []byte(raw)); err != nil && err != io.EOF {
				return err
			}
		}
	}

	return scanner.Err()
}
