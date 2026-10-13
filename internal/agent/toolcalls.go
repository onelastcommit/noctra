package agent

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"sync"
)

const ToolLinePrefix = "[tool] "

const toolDetailMax = 200

var toolDetailKeys = []string{
	"command", "commandline", "cmd",
	"file_path", "absolutepath", "targetfile", "path",
	"pattern", "query", "url", "description",
}

func toolCallLine(name string, input json.RawMessage) string {
	line := ToolLinePrefix + name
	if detail := toolDetail(input); detail != "" {
		line += ": " + detail
	}
	return line
}

func toolDetail(input json.RawMessage) string {
	var fields map[string]any
	if json.Unmarshal(input, &fields) != nil {
		var s string
		if json.Unmarshal(input, &s) != nil {
			return ""
		}
		return oneLine(s)
	}
	byKey := make(map[string]string, len(fields))
	for k, v := range fields {
		if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
			byKey[strings.ToLower(k)] = s
		}
	}
	for _, k := range toolDetailKeys {
		if s, ok := byKey[k]; ok {
			return oneLine(s)
		}
	}
	return ""
}

func oneLine(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > toolDetailMax {
		s = string(r[:toolDetailMax]) + "…"
	}
	return s
}

func withoutToolLines(output string) string {
	if !strings.Contains(output, ToolLinePrefix) {
		return output
	}
	lines := strings.Split(output, "\n")
	kept := lines[:0]
	for _, l := range lines {
		if !strings.HasPrefix(l, ToolLinePrefix) {
			kept = append(kept, l)
		}
	}
	return strings.Join(kept, "\n")
}

type streamParser interface {
	feed(line []byte) (toolLine string)
	response() string
	diagnostics() string
}

type syncWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (s *syncWriter) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.w.Write(p)
}

type parsingWriter struct {
	parser  streamParser
	log     io.Writer
	pending []byte
}

func (w *parsingWriter) Write(p []byte) (int, error) {
	w.pending = append(w.pending, p...)
	for {
		i := bytes.IndexByte(w.pending, '\n')
		if i < 0 {
			break
		}
		w.handle(w.pending[:i])
		w.pending = w.pending[i+1:]
	}
	return len(p), nil
}

func (w *parsingWriter) flush() {
	if len(w.pending) > 0 {
		w.handle(w.pending)
		w.pending = nil
	}
}

func (w *parsingWriter) handle(line []byte) {
	line = bytes.TrimSpace(line)
	if len(line) == 0 {
		return
	}
	if tl := w.parser.feed(line); tl != "" {
		io.WriteString(w.log, tl+"\n")
	}
}
