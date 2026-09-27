// Package claude reaches Claude through the Claude Code CLI, on a Claude
// subscription, and plays the proposer and the experts of package design.
//
// Each call is a locked-down headless `claude -p`, as tools/calib/claude_cli.py
// makes it:
//
//   - our system prompt replaces Claude Code's; a short preamble remains (the
//     date, the working directory, the account e-mail), which says nothing
//     about any task;
//   - no tools but the one returning the --json-schema answer, no settings,
//     hooks or MCP servers, no saved session, run in an empty directory so no
//     CLAUDE.md or project memory is read;
//   - the user turn goes in as stream-json, images as content blocks, never
//     as file paths.
//
// Every call reports how full the subscription's five-hour window is, and a
// Client stops starting calls past a limit, so a long run leaves its owner
// room to work.
package claude

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// Block is one piece of a user turn: text, or an image.
type Block struct {
	Type   string       `json:"type"` // "text" or "image"
	Text   string       `json:"text,omitempty"`
	Source *ImageSource `json:"source,omitempty"`
}

// ImageSource is an image sent inline, base64-encoded.
type ImageSource struct {
	Type      string `json:"type"` // "base64"
	MediaType string `json:"media_type"`
	Data      string `json:"data"`
}

// TextBlock is a block of plain text.
func TextBlock(s string) Block { return Block{Type: "text", Text: s} }

// Request is one call.
type Request struct {
	System  string
	Model   string
	Effort  string          // "" keeps the CLI's default
	Schema  json.RawMessage // the reply's JSON schema; nil for a plain-text reply
	Content []Block
}

// Runner runs the CLI with args and stdin in dir, returning its stdout.
// Tests swap in a fake.
type Runner func(ctx context.Context, dir string, args []string, stdin []byte) (stdout []byte, err error)

// ExecRunner runs the real `claude` binary.
func ExecRunner(ctx context.Context, dir string, args []string, stdin []byte) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "claude", args...)
	cmd.Dir = dir
	cmd.Stdin = bytes.NewReader(stdin)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil && len(out) == 0 {
		lines := strings.Split(strings.TrimSpace(stderr.String()), "\n")
		return nil, fmt.Errorf("claude: %w: %s", err, lines[len(lines)-1])
	}
	return out, nil // a failed call still prints its result event, which says why
}

// ErrPaused is returned instead of starting a call once the five-hour window
// is past the client's limit.
var ErrPaused = errors.New("the five-hour window is past the limit")

// Client makes calls and keeps track of what they cost and how full the
// subscription's five-hour window is. It is safe for concurrent use.
type Client struct {
	Run       Runner        // nil means ExecRunner
	MaxWindow float64       // stop starting calls once the window is this full; 0 means 0.8
	Timeout   time.Duration // per call; 0 means ten minutes

	mu     sync.Mutex
	cost   float64
	window float64
	known  bool
	resets time.Time
}

// Status is what the calls so far have reported.
type Status struct {
	Cost   float64   // in dollars at API prices, though the subscription pays
	Window float64   // share of the five-hour window used, when Known
	Known  bool      // whether any call has reported the window yet
	Resets time.Time // when the window resets, if reported
}

func (c *Client) Status() Status {
	c.mu.Lock()
	defer c.mu.Unlock()
	return Status{Cost: c.cost, Window: c.window, Known: c.known, Resets: c.resets}
}

// Paused reports whether the client has stopped starting calls.
func (c *Client) Paused() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.known && c.window >= c.maxWindow()
}

func (c *Client) maxWindow() float64 {
	if c.MaxWindow > 0 {
		return c.MaxWindow
	}
	return 0.8
}

// Call makes one request and returns the reply: the structured output when
// the request has a schema, or the reply's text as a JSON string otherwise.
func (c *Client) Call(ctx context.Context, req Request) (json.RawMessage, error) {
	if c.Paused() {
		return nil, ErrPaused
	}
	args := []string{"-p", "--input-format", "stream-json", "--output-format", "stream-json",
		"--verbose", "--system-prompt", req.System, "--tools", "",
		"--setting-sources", "", "--strict-mcp-config", "--no-session-persistence",
		"--model", req.Model}
	if req.Effort != "" {
		args = append(args, "--effort", req.Effort)
	}
	if req.Schema != nil {
		args = append(args, "--json-schema", string(req.Schema))
	}
	msg, err := json.Marshal(map[string]any{
		"type":    "user",
		"message": map[string]any{"role": "user", "content": req.Content},
	})
	if err != nil {
		return nil, err
	}

	empty, err := os.MkdirTemp("", "ribice-design-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(empty)
	timeout := c.Timeout
	if timeout == 0 {
		timeout = 10 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	run := c.Run
	if run == nil {
		run = ExecRunner
	}
	out, err := run(ctx, empty, args, append(msg, '\n'))
	if err != nil {
		return nil, err
	}

	res, err := parseEvents(out, req.Schema != nil)
	c.mu.Lock()
	c.cost += res.cost
	if res.windowKnown {
		c.window, c.known, c.resets = res.window, true, res.resets
	}
	c.mu.Unlock()
	return res.answer, err
}

type callResult struct {
	answer      json.RawMessage
	cost        float64
	window      float64
	windowKnown bool
	resets      time.Time
}

// parseEvents reads the CLI's stream-json output: the rate-limit events for
// the window, and the result event for the answer and cost.
func parseEvents(out []byte, structured bool) (callResult, error) {
	var res callResult
	var failure string
	gotResult := false
	sc := bufio.NewScanner(bytes.NewReader(out))
	sc.Buffer(make([]byte, 0, 64*1024), 16<<20)
	for sc.Scan() {
		var ev struct {
			Type          string `json:"type"`
			RateLimitInfo struct {
				UnifiedWindows struct {
					FiveHour struct {
						Utilization *float64 `json:"utilization"`
						ResetsAt    int64    `json:"resetsAt"`
					} `json:"five_hour"`
				} `json:"unifiedWindows"`
			} `json:"rate_limit_info"`
			Subtype          string          `json:"subtype"`
			IsError          bool            `json:"is_error"`
			TotalCostUSD     float64         `json:"total_cost_usd"`
			Result           string          `json:"result"`
			StructuredOutput json.RawMessage `json:"structured_output"`
		}
		if json.Unmarshal(sc.Bytes(), &ev) != nil {
			continue // not every line is an event we know
		}
		switch ev.Type {
		case "rate_limit_event":
			if w := ev.RateLimitInfo.UnifiedWindows.FiveHour; w.Utilization != nil {
				res.window, res.windowKnown = *w.Utilization, true
				if w.ResetsAt > 0 {
					res.resets = time.Unix(w.ResetsAt, 0)
				}
			}
		case "result":
			gotResult = true
			res.cost = ev.TotalCostUSD
			switch {
			case ev.IsError || ev.Subtype != "success":
				failure = ev.Subtype
				if failure == "" || failure == "success" {
					failure = "error: " + ev.Result
				}
			case structured:
				if len(ev.StructuredOutput) == 0 || string(ev.StructuredOutput) == "null" {
					failure = "no structured output"
				} else {
					res.answer = ev.StructuredOutput
				}
			default:
				res.answer, _ = json.Marshal(ev.Result)
			}
		}
	}
	switch {
	case failure != "":
		return res, fmt.Errorf("claude: %s", failure)
	case !gotResult:
		return res, errors.New("claude: no result in the output")
	}
	return res, nil
}
