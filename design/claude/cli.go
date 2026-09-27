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

// Runner runs the CLI with args and stdin in dir, handing each line of its
// standard output to line as soon as it is written. Tests swap in a fake.
type Runner func(ctx context.Context, dir string, args []string, stdin []byte, line func([]byte)) error

// ExecRunner runs the real `claude` binary.
func ExecRunner(ctx context.Context, dir string, args []string, stdin []byte, line func([]byte)) error {
	cmd := exec.CommandContext(ctx, "claude", args...)
	cmd.Dir = dir
	cmd.Stdin = bytes.NewReader(stdin)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	sc := bufio.NewScanner(out)
	sc.Buffer(make([]byte, 0, 64*1024), 64<<20) // the result line carries the whole answer
	lines := 0
	for sc.Scan() {
		lines++
		line(sc.Bytes())
	}
	scanErr := sc.Err()
	if err := cmd.Wait(); err != nil && lines == 0 {
		msg := strings.Split(strings.TrimSpace(stderr.String()), "\n")
		return fmt.Errorf("claude: %w: %s", err, msg[len(msg)-1])
	}
	return scanErr // a failed call still prints its result event, which says why
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

	// Progress, when set, is told how each call is getting on as its reply
	// streams in, at most about once a second per call. Calls then ask the
	// CLI for partial messages, which it otherwise leaves out.
	Progress func(Progress)

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
	ev := &events{structured: req.Schema != nil, start: time.Now()}
	if c.Progress != nil {
		args = append(args, "--include-partial-messages")
		ev.progress = c.Progress
	}
	runErr := run(ctx, empty, args, append(msg, '\n'), ev.feed)
	res, err := ev.finish()
	if runErr != nil && res.answer == nil {
		return nil, runErr
	}

	c.mu.Lock()
	c.cost += res.cost
	if res.windowKnown {
		c.window, c.known, c.resets = res.window, true, res.resets
	}
	c.mu.Unlock()
	return res.answer, err
}

// Progress is how far one call has got.
type Progress struct {
	Thinking int           // characters of thinking received so far
	Writing  int           // characters of the answer received so far
	Elapsed  time.Duration // since the call started
}

type callResult struct {
	answer      json.RawMessage
	cost        float64
	window      float64
	windowKnown bool
	resets      time.Time
}

// events reads the CLI's stream-json output line by line: rate-limit events
// for the window, partial messages for progress, and the result event for the
// answer and cost.
type events struct {
	structured bool
	start      time.Time
	progress   func(Progress)

	res       callResult
	failure   string
	gotResult bool
	now       Progress
	reported  time.Time
}

// event is the part of any stream-json line that events reads.
type event struct {
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
	Event            struct {
		Type  string `json:"type"`
		Delta struct {
			Type        string `json:"type"`
			Thinking    string `json:"thinking"`
			Text        string `json:"text"`
			PartialJSON string `json:"partial_json"`
		} `json:"delta"`
	} `json:"event"`
}

func (e *events) feed(line []byte) {
	var ev event
	if json.Unmarshal(line, &ev) != nil {
		return // not every line is an event we know
	}
	switch ev.Type {
	case "rate_limit_event":
		if w := ev.RateLimitInfo.UnifiedWindows.FiveHour; w.Utilization != nil {
			e.res.window, e.res.windowKnown = *w.Utilization, true
			if w.ResetsAt > 0 {
				e.res.resets = time.Unix(w.ResetsAt, 0)
			}
		}
	case "stream_event":
		e.stream(ev)
	case "result":
		e.gotResult = true
		e.res.cost = ev.TotalCostUSD
		switch {
		case ev.IsError || ev.Subtype != "success":
			e.failure = ev.Subtype
			if e.failure == "" || e.failure == "success" {
				e.failure = "error: " + ev.Result
			}
		case e.structured:
			if len(ev.StructuredOutput) == 0 || string(ev.StructuredOutput) == "null" {
				e.failure = "no structured output"
			} else {
				e.res.answer = append(json.RawMessage(nil), ev.StructuredOutput...)
			}
		default:
			e.res.answer, _ = json.Marshal(ev.Result)
		}
	}
}

// stream counts what a partial message adds, and reports it now and then.
func (e *events) stream(ev event) {
	if ev.Event.Type != "content_block_delta" {
		return
	}
	switch d := ev.Event.Delta; d.Type {
	case "thinking_delta":
		e.now.Thinking += len(d.Thinking)
	case "text_delta":
		e.now.Writing += len(d.Text)
	case "input_json_delta": // the structured answer arrives as a tool call
		e.now.Writing += len(d.PartialJSON)
	}
	if e.progress != nil && time.Since(e.reported) >= time.Second {
		e.reported = time.Now()
		e.now.Elapsed = time.Since(e.start)
		e.progress(e.now)
	}
}

func (e *events) finish() (callResult, error) {
	switch {
	case e.failure != "":
		return e.res, fmt.Errorf("claude: %s", e.failure)
	case !e.gotResult:
		return e.res, errors.New("claude: no result in the output")
	}
	return e.res, nil
}

// parseEvents reads a whole stream-json output at once.
func parseEvents(out []byte, structured bool) (callResult, error) {
	e := &events{structured: structured}
	for _, line := range bytes.Split(out, []byte("\n")) {
		e.feed(line)
	}
	return e.finish()
}
