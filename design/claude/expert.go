package claude

import (
	"context"
	"embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"text/template"

	"github.com/lpapez/ribice/design"
	"github.com/lpapez/ribice/kb"
)

// PromptVersion names this set of prompts. Changing any prompt must change
// it: it is part of every expert's id, so old verdicts are not taken for
// answers to the new wording, and design.Combine counts only an expert's
// latest version where it answered a task under several.
//
// v2 (2026-09-27): perceive's ten people know no words of the field, and a
// question with such a word counts them as unable to answer. Under v1,
// Claude rated "Did little turrets rise from its top?" answerable by 99%.
const PromptVersion = "v2"

//go:embed prompts/*.txt
var promptFiles embed.FS

// Variants lists the prompt wordings each kind has. Each variant is its own
// expert, so three variants of one model are at least three differently
// asked samples rather than one sample three times.
var Variants = map[design.Kind][]string{
	design.Assign:   {"a", "b", "c"},
	design.Perceive: {"a", "b", "c"},
	design.Observe:  {"a"},
}

// Expert is Claude answering tasks through the CLI, with one wording of the
// prompts.
type Expert struct {
	Client  *Client
	Model   string // e.g. "claude-opus-5"
	Effort  string // e.g. "low"; "" keeps the CLI's default
	Variant string // "a", "b" or "c"
	Domain  string // what the knowledge base is about, plural: "clouds"
	Jobs    int    // calls at once; 0 means 4
	Log     io.Writer
}

func (e *Expert) ID() string {
	return "claude:" + e.Model + "@" + PromptVersion + e.Variant
}

func (e *Expert) Accepts(t design.Task) bool {
	if !hasVariant(t.Kind, e.Variant) {
		return false
	}
	switch t.Kind {
	case design.Assign:
		return t.Subject.Entity != ""
	case design.Perceive:
		return t.Subject.Text != ""
	case design.Observe:
		return len(t.Subject.Photos) > 0
	}
	return false
}

func hasVariant(k design.Kind, v string) bool {
	for _, x := range Variants[k] {
		if x == v {
			return true
		}
	}
	return false
}

// Answer asks tasks in batches: one call for all of an entity's assign
// tasks, one for all the cases of an attribute's perceive tasks, one for all
// the questions about a photo. A reply is checked answer by answer; the
// tasks it left unanswered or answered wrongly are asked once more, then
// left for the next run. The run stops early, without an error, once the
// client pauses for the five-hour window.
func (e *Expert) Answer(ctx context.Context, tasks []design.Task, emit func(design.Verdict) error) error {
	batches := batch(tasks)
	jobs := e.Jobs
	if jobs <= 0 {
		jobs = 4
	}

	type result struct {
		verdicts []design.Verdict
		err      error
	}
	work := make(chan []design.Task)
	results := make(chan result)
	var wg sync.WaitGroup
	for i := 0; i < jobs; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for b := range work {
				vs, err := e.ask(ctx, b)
				results <- result{vs, err}
			}
		}()
	}
	go func() {
		defer close(work)
		for _, b := range batches {
			if e.Client.Paused() || ctx.Err() != nil {
				return
			}
			select {
			case work <- b:
			case <-ctx.Done():
				return
			}
		}
	}()
	go func() {
		wg.Wait()
		close(results)
	}()

	// Verdicts are emitted from this goroutine only, so emit need not be safe
	// for concurrent use.
	var emitErr error
	done := 0
	for r := range results {
		switch {
		case errors.Is(r.err, ErrPaused):
		case r.err != nil:
			e.logf("\n  %v", r.err)
		}
		for _, v := range r.verdicts {
			if emitErr == nil {
				emitErr = emit(v)
			}
		}
		done += len(r.verdicts)
		st := e.Client.Status()
		e.logf("\r  %s: %d/%d answered, window %.0f%%", e.ID(), done, len(tasks), st.Window*100)
	}
	st := e.Client.Status()
	e.logf("\n  about $%.2f at API prices, from the subscription\n", st.Cost)
	if e.Client.Paused() {
		at := "later"
		if !st.Resets.IsZero() {
			at = st.Resets.Format("15:04")
		}
		e.logf("  paused with %d left: the five-hour window is %.0f%% used; it resets at %s\n",
			len(tasks)-done, st.Window*100, at)
	}
	if emitErr != nil {
		return emitErr
	}
	return ctx.Err()
}

func (e *Expert) logf(format string, args ...any) {
	if e.Log != nil {
		fmt.Fprintf(e.Log, format, args...)
	}
}

// batch groups tasks that go into one call, keeping the order in which each
// group first appears.
func batch(tasks []design.Task) [][]design.Task {
	var order []string
	groups := map[string][]design.Task{}
	for _, t := range tasks {
		var key string
		switch t.Kind {
		case design.Assign:
			key = "assign\x00" + t.Subject.Entity
		case design.Perceive:
			// The cases share one question, so they must share its wording too.
			key = "perceive\x00" + t.Attribute + "\x00" + t.Question + "\x00" + optionKey(t.Options)
		case design.Observe:
			key = "observe\x00" + strings.Join(t.Subject.Photos, "\x00")
		}
		if _, ok := groups[key]; !ok {
			order = append(order, key)
		}
		groups[key] = append(groups[key], t)
	}
	out := make([][]design.Task, len(order))
	for i, k := range order {
		out[i] = groups[k]
	}
	return out
}

func optionKey(opts []design.Option) string {
	parts := make([]string, len(opts))
	for i, o := range opts {
		parts[i] = string(o.Value) + "=" + o.Label
	}
	return strings.Join(parts, "\x00")
}

// ask puts one batch, and once more whatever the first reply left out.
func (e *Expert) ask(ctx context.Context, b []design.Task) ([]design.Verdict, error) {
	var out []design.Verdict
	todo := b
	var lastErr error
	for attempt := 0; attempt < 2 && len(todo) > 0; attempt++ {
		req, err := e.request(todo)
		if err != nil {
			return out, err
		}
		reply, err := e.Client.Call(ctx, req)
		if errors.Is(err, ErrPaused) {
			return out, err
		}
		if err != nil {
			lastErr = err
			continue
		}
		verdicts, missing, err := e.parse(todo, reply)
		if err != nil {
			lastErr = err
		}
		out = append(out, verdicts...)
		todo = missing
	}
	if len(todo) > 0 && lastErr == nil {
		lastErr = fmt.Errorf("%d answers missing or invalid after a retry", len(todo))
	}
	if lastErr != nil {
		lastErr = fmt.Errorf("%s, %s: %w", b[0].Kind, describe(b[0]), lastErr)
	}
	return out, lastErr
}

func describe(t design.Task) string {
	switch t.Kind {
	case design.Assign:
		return t.Subject.Entity
	case design.Perceive:
		return t.Attribute
	}
	return strings.Join(t.Subject.Photos, ", ")
}

// --- requests ----------------------------------------------------------------

func (e *Expert) request(b []design.Task) (Request, error) {
	kind := b[0].Kind
	system, err := e.system(kind)
	if err != nil {
		return Request{}, err
	}
	req := Request{System: system, Model: e.Model, Effort: e.Effort}
	var text strings.Builder
	switch kind {
	case design.Assign:
		req.Schema = pickSchema
		s := b[0].Subject
		fmt.Fprintf(&text, "%s\n", s.Entity)
		if s.Text != "" {
			fmt.Fprintf(&text, "%s\n", s.Text)
		}
		writeQuestions(&text, b)
	case design.Perceive:
		req.Schema = splitSchema(len(b[0].Options))
		fmt.Fprintf(&text, "Question: %s\nOptions:\n", b[0].Question)
		writeOptions(&text, b[0].Options, "")
		text.WriteString("\nCases:\n")
		for i, t := range b {
			fmt.Fprintf(&text, "%d. What is really there: %s\n", i+1, t.Subject.Text)
		}
	case design.Observe:
		req.Schema = pickSchema
		for _, path := range b[0].Subject.Photos {
			img, err := imageBlock(path)
			if err != nil {
				return Request{}, err
			}
			req.Content = append(req.Content, img)
		}
		writeQuestions(&text, b)
	}
	req.Content = append(req.Content, TextBlock(text.String()))
	return req, nil
}

func writeQuestions(w *strings.Builder, b []design.Task) {
	for i, t := range b {
		fmt.Fprintf(w, "\nQuestion %d: %s\n", i+1, t.Question)
		writeOptions(w, t.Options, "   ")
	}
}

func writeOptions(w *strings.Builder, opts []design.Option, indent string) {
	for j, o := range opts {
		fmt.Fprintf(w, "%s%d) %s\n", indent, j+1, o.Label)
	}
}

func (e *Expert) system(kind design.Kind) (string, error) {
	name := fmt.Sprintf("prompts/%s-%s.txt", kind, e.Variant)
	t, err := template.ParseFS(promptFiles, name)
	if err != nil {
		return "", err
	}
	if rules := fmt.Sprintf("prompts/%s-rules.txt", kind); fileExists(rules) {
		raw, _ := promptFiles.ReadFile(rules)
		if _, err := t.New("rules").Parse(string(raw)); err != nil {
			return "", err
		}
	}
	domain := e.Domain
	if domain == "" {
		domain = "the things in a field guide"
	}
	var s strings.Builder
	if err := t.ExecuteTemplate(&s, filepath.Base(name), map[string]string{"Domain": domain}); err != nil {
		return "", err
	}
	return strings.TrimSpace(s.String()), nil
}

func fileExists(name string) bool {
	_, err := promptFiles.Open(name)
	return err == nil
}

func imageBlock(path string) (Block, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Block{}, err
	}
	mt := mime.TypeByExtension(strings.ToLower(filepath.Ext(path)))
	switch mt {
	case "image/jpeg", "image/png", "image/webp", "image/gif":
	default:
		return Block{}, fmt.Errorf("%s: not an image Claude reads (%q)", path, mt)
	}
	return Block{Type: "image", Source: &ImageSource{Type: "base64", MediaType: mt,
		Data: base64.StdEncoding.EncodeToString(data)}}, nil
}

// --- replies -----------------------------------------------------------------

// The reply schemas have no free-text field: an expert chooses, or abstains
// for one of a fixed set of reasons, and nothing else.
var reasonEnum = `{"type": ["string", "null"], "enum": ["not_observable", "ambiguous", "unclear_question", "unknown", null]}`

var pickSchema = json.RawMessage(`{
  "type": "object",
  "properties": {"answers": {"type": "array", "items": {
    "type": "object",
    "properties": {
      "question": {"type": "integer", "description": "the question's number"},
      "choice": {"type": "array", "items": {"type": "integer"}, "maxItems": 2,
                 "description": "the option numbers chosen: one, or two; empty when you cannot choose"},
      "confidence": {"type": "string", "enum": ["low", "medium", "high"]},
      "cant_tell": ` + reasonEnum + `
    },
    "required": ["question", "choice", "confidence", "cant_tell"],
    "additionalProperties": false}}},
  "required": ["answers"],
  "additionalProperties": false}`)

// splitSchema is the perceive reply's schema for a question with n options.
// It pins the counts to exactly n: left free, the model sometimes added the
// people who cannot answer as one more count.
func splitSchema(n int) json.RawMessage {
	return json.RawMessage(fmt.Sprintf(splitSchemaFormat, n, n, n))
}

var splitSchemaFormat = `{
  "type": "object",
  "properties": {"answers": {"type": "array", "items": {
    "type": "object",
    "properties": {
      "case": {"type": "integer", "description": "the case's number"},
      "counts": {"type": "array", "items": {"type": "integer", "minimum": 0}, "minItems": %d, "maxItems": %d,
                 "description": "how many of the ten pick each option: exactly %d numbers, one per option, in the options' order; those who cannot answer go in cannot_answer, not here"},
      "cannot_answer": {"type": "integer", "minimum": 0},
      "reason": ` + reasonEnum + `
    },
    "required": ["case", "counts", "cannot_answer", "reason"],
    "additionalProperties": false}}},
  "required": ["answers"],
  "additionalProperties": false}`

type pickReply struct {
	Answers []struct {
		Question   int               `json:"question"`
		Choice     []int             `json:"choice"`
		Confidence design.Confidence `json:"confidence"`
		CantTell   *design.Reason    `json:"cant_tell"`
	} `json:"answers"`
}

type splitReply struct {
	Answers []struct {
		Case         int            `json:"case"`
		Counts       []int          `json:"counts"`
		CannotAnswer int            `json:"cannot_answer"`
		Reason       *design.Reason `json:"reason"`
	} `json:"answers"`
}

// people is how many untrained people a perceive answer counts.
const people = 10

// parse turns a reply into verdicts for the tasks it answered well, and
// returns the tasks it did not.
func (e *Expert) parse(b []design.Task, reply json.RawMessage) ([]design.Verdict, []design.Task, error) {
	got := map[int]design.Verdict{}
	var problems []string
	bad := func(n int, format string, args ...any) {
		problems = append(problems, fmt.Sprintf("%d: ", n)+fmt.Sprintf(format, args...))
	}

	switch b[0].Kind {
	case design.Perceive:
		var r splitReply
		if err := json.Unmarshal(reply, &r); err != nil {
			return nil, b, err
		}
		for _, a := range r.Answers {
			if a.Case < 1 || a.Case > len(b) {
				bad(a.Case, "no such case")
				continue
			}
			t := b[a.Case-1]
			v, err := e.split(t, a.Counts, a.CannotAnswer, a.Reason)
			if err != nil {
				bad(a.Case, "%v", err)
				continue
			}
			got[a.Case] = v
		}
	default:
		var r pickReply
		if err := json.Unmarshal(reply, &r); err != nil {
			return nil, b, err
		}
		for _, a := range r.Answers {
			if a.Question < 1 || a.Question > len(b) {
				bad(a.Question, "no such question")
				continue
			}
			t := b[a.Question-1]
			v, err := e.pick(t, a.Choice, a.Confidence, a.CantTell)
			if err != nil {
				bad(a.Question, "%v", err)
				continue
			}
			got[a.Question] = v
		}
	}

	var verdicts []design.Verdict
	var missing []design.Task
	for i, t := range b {
		if v, ok := got[i+1]; ok {
			verdicts = append(verdicts, v)
		} else {
			missing = append(missing, t)
		}
	}
	if len(problems) > 0 {
		return verdicts, missing, fmt.Errorf("invalid answers %s", strings.Join(problems, "; "))
	}
	return verdicts, missing, nil
}

func (e *Expert) pick(t design.Task, choice []int, c design.Confidence, cant *design.Reason) (design.Verdict, error) {
	if cant != nil {
		v := design.Abstention(t.ID, e.ID(), *cant)
		return v, v.Check(t)
	}
	if len(choice) == 0 || len(choice) > max(t.MaxChoices, 1) {
		return design.Verdict{}, fmt.Errorf("%d options chosen, want 1 to %d", len(choice), max(t.MaxChoices, 1))
	}
	idx := make([]int, len(choice))
	for i, n := range choice {
		idx[i] = n - 1
	}
	p, err := design.Picks(t.Options, idx, c)
	if err != nil {
		return design.Verdict{}, err
	}
	v := design.Verdict{Task: t.ID, Expert: e.ID(), P: p}
	return v, v.Check(t)
}

func (e *Expert) split(t design.Task, counts []int, cannot int, reason *design.Reason) (design.Verdict, error) {
	if len(counts) != len(t.Options) {
		return design.Verdict{}, fmt.Errorf("%d counts for %d options", len(counts), len(t.Options))
	}
	total := cannot
	for _, n := range counts {
		total += n
	}
	if total != people {
		return design.Verdict{}, fmt.Errorf("counts add up to %d, not %d", total, people)
	}
	v := design.Verdict{Task: t.ID, Expert: e.ID(), P: map[kb.Value]float64{},
		Abstain: float64(cannot) / people}
	for i, n := range counts {
		v.P[t.Options[i].Value] = float64(n) / people
	}
	if cannot > 0 {
		if reason == nil {
			return design.Verdict{}, fmt.Errorf("%d cannot answer, with no reason", cannot)
		}
		v.Reason = *reason
	}
	return v, v.Check(t)
}
