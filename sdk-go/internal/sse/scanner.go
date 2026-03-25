package sse

import (
	"bufio"
	"io"
	"strings"
)

// Scanner implements a basic server-sent events scanner
type Scanner struct {
	scanner *bufio.Scanner
}

const MaxScanTokenSize = 5 * 1024 * 1024 // 5MB

// NewScanner creates a new SSE scanner from an io.Reader
func NewScanner(reader io.Reader) *Scanner {
	scanner := bufio.NewScanner(reader)
	buf := make([]byte, MaxScanTokenSize)
	scanner.Buffer(buf, MaxScanTokenSize)
	return &Scanner{
		scanner: scanner,
	}
}

// Scan advances the scanner to the next line
func (s *Scanner) Scan() bool {
	return s.scanner.Scan()
}

// Text returns the current line as a string
func (s *Scanner) Text() string {
	return s.scanner.Text()
}

// Err returns any error encountered during scanning
func (s *Scanner) Err() error {
	return s.scanner.Err()
}

// Event represents a server-sent event
type Event struct {
	Type string
	Data string
	ID   string
}

// ParseEvent parses a single SSE event from lines of text
func ParseEvent(lines []string) *Event {
	event := &Event{}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}

		if strings.HasPrefix(line, "event:") {
			event.Type = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		} else if strings.HasPrefix(line, "data:") {
			data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if event.Data != "" {
				event.Data += "\n"
			}
			event.Data += data
		} else if strings.HasPrefix(line, "id:") {
			event.ID = strings.TrimSpace(strings.TrimPrefix(line, "id:"))
		}
	}

	return event
}

// IsDataLine checks if a line is a data line and returns the data content
func IsDataLine(line string) (string, bool) {
	line = strings.TrimSpace(line)
	if data, ok := strings.CutPrefix(line, "data:"); ok {
		return strings.TrimPrefix(data, " "), true
	}
	return "", false
}
