// Package transcript reads a Claude Code session's transcript (the JSONL file at a hook's
// transcript_path, docs/hooks.md) into the turns a person reads.
package transcript

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/TorratDev/the-eye/internal/hub"
)

// Turn is one thing said or done in the session.
type Turn struct {
	Kind  string // user, assistant, tool (a call), result (a tool's output)
	Text  string // the words, the tool's output, or for a call the tool's input as JSON
	Tool  string // the tool's name, for a call
	Error bool   // a result that is a tool error
}

type entry struct {
	Type      string `json:"type"`
	Sidechain bool   `json:"isSidechain"`
	Message   struct {
		Content json.RawMessage `json:"content"`
	} `json:"message"`
}

type block struct {
	Type    string          `json:"type"`
	Text    string          `json:"text"`
	Name    string          `json:"name"`
	Input   json.RawMessage `json:"input"`
	Content json.RawMessage `json:"content"`
	IsError bool            `json:"is_error"`
}

// Read parses the transcript at path.
func Read(path string) ([]Turn, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return Parse(f)
}

// Parse reads transcript entries in order, one JSON object per line. Claude Code's bookkeeping
// entries, thinking blocks and subagent (sidechain) entries are skipped. A last line with no newline
// that does not parse is one the session is still writing, so it ends the turns without an error.
func Parse(r io.Reader) ([]Turn, error) {
	var turns []Turn
	br := bufio.NewReader(r)
	for {
		line, readErr := br.ReadBytes('\n')
		if readErr != nil && readErr != io.EOF {
			return turns, readErr
		}
		last := readErr == io.EOF
		if len(bytes.TrimSpace(line)) > 0 {
			var e entry
			if err := json.Unmarshal(line, &e); err != nil {
				if last {
					return turns, nil
				}
				return turns, err
			}
			var err error
			if turns, err = appendEntry(turns, e); err != nil {
				return turns, err
			}
		}
		if last {
			return turns, nil
		}
	}
}

// appendEntry adds one entry's turns. Only the main session's user and assistant entries count.
func appendEntry(turns []Turn, e entry) ([]Turn, error) {
	if (e.Type != "user" && e.Type != "assistant") || e.Sidechain {
		return turns, nil
	}
	var s string
	if json.Unmarshal(e.Message.Content, &s) == nil {
		return append(turns, Turn{Kind: e.Type, Text: s}), nil
	}
	var blocks []block
	if err := json.Unmarshal(e.Message.Content, &blocks); err != nil {
		return turns, err
	}
	for _, b := range blocks {
		switch b.Type {
		case "text":
			turns = append(turns, Turn{Kind: e.Type, Text: b.Text})
		case "tool_use":
			turns = append(turns, Turn{Kind: "tool", Tool: b.Name, Text: string(b.Input)})
		case "tool_result":
			turns = append(turns, Turn{Kind: "result", Text: resultText(b.Content), Error: b.IsError})
		}
	}
	return turns, nil
}

// resultText reads a tool result's content, which is a string or a list of text blocks.
func resultText(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var blocks []block
	json.Unmarshal(raw, &blocks)
	var parts []string
	for _, b := range blocks {
		if b.Type == "text" {
			parts = append(parts, b.Text)
		}
	}
	return strings.Join(parts, "\n")
}

// LastAssistant returns the text of the last assistant turn, or "".
func LastAssistant(turns []Turn) string {
	for i := len(turns) - 1; i >= 0; i-- {
		if turns[i].Kind == "assistant" {
			return turns[i].Text
		}
	}
	return ""
}

// PathFor returns the transcript_path from the newest of a pane's hook events that carries one, or "".
// The path comes from a hook body, so only an absolute .jsonl path is taken.
func PathFor(st *hub.Store, pane string) (string, error) {
	events, err := st.Events(pane)
	if err != nil {
		return "", err
	}
	for i := len(events) - 1; i >= 0; i-- {
		var p struct {
			Path string `json:"transcript_path"`
		}
		if json.Unmarshal([]byte(events[i].Payload), &p) == nil && filepath.IsAbs(p.Path) && filepath.Ext(p.Path) == ".jsonl" {
			return p.Path, nil
		}
	}
	return "", nil
}
