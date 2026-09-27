package claude

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/lpapez/ribice/design"
	"github.com/lpapez/ribice/kb"
)

// fake stands in for the CLI: it records every call and answers with reply,
// which sees the call's user turn.
type fake struct {
	mu     sync.Mutex
	calls  []call
	window float64
	reply  func(n int, turn string) string // the structured output, as JSON
}

type call struct {
	dir  string
	args []string
	turn string
}

func (f *fake) run(_ context.Context, dir string, args []string, stdin []byte) ([]byte, error) {
	var msg struct {
		Message struct {
			Content []Block `json:"content"`
		} `json:"message"`
	}
	if err := json.Unmarshal(stdin, &msg); err != nil {
		return nil, err
	}
	var turn strings.Builder
	for _, b := range msg.Message.Content {
		turn.WriteString(b.Text)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		return nil, fmt.Errorf("run in a directory that is not empty")
	}

	f.mu.Lock()
	n := len(f.calls)
	f.calls = append(f.calls, call{dir, args, turn.String()})
	f.mu.Unlock()
	var out bytes.Buffer
	if err := json.Compact(&out, []byte(f.reply(n, turn.String()))); err != nil {
		return nil, err
	}
	return []byte(fmt.Sprintf(`{"type":"system","subtype":"init"}
{"type":"rate_limit_event","rate_limit_info":{"unifiedWindows":{"five_hour":{"utilization":%v,"resetsAt":1790000000}}}}
{"type":"result","subtype":"success","is_error":false,"total_cost_usd":0.01,"structured_output":%s}
`, f.window, out.String())), nil
}

func (f *fake) client() *Client { return &Client{Run: f.run} }

func arg(args []string, flag string) (string, bool) {
	for i, a := range args {
		if a == flag && i+1 < len(args) {
			return args[i+1], true
		}
	}
	return "", false
}

func TestParseEvents(t *testing.T) {
	out := []byte(`not json
{"type":"rate_limit_event","rate_limit_info":{"unifiedWindows":{"five_hour":{"utilization":0.42,"resetsAt":1790000000}}}}
{"type":"result","subtype":"success","is_error":false,"total_cost_usd":0.03,"structured_output":{"answers":[]}}`)
	res, err := parseEvents(out, true)
	if err != nil {
		t.Fatal(err)
	}
	if !res.windowKnown || res.window != 0.42 || res.cost != 0.03 || string(res.answer) != `{"answers":[]}` {
		t.Errorf("parsed %+v", res)
	}

	for name, out := range map[string]string{
		"error result": `{"type":"result","subtype":"error_max_turns","is_error":true}`,
		"no output":    `{"type":"result","subtype":"success","is_error":false,"structured_output":null}`,
		"no result":    `{"type":"system"}`,
	} {
		if _, err := parseEvents([]byte(out), true); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}

func TestClientPausesPastTheWindow(t *testing.T) {
	f := &fake{window: 0.85, reply: func(int, string) string { return `{"answers":[]}` }}
	c := f.client()
	if _, err := c.Call(context.Background(), Request{Model: "m", Schema: pickSchema}); err != nil {
		t.Fatal(err)
	}
	if !c.Paused() {
		t.Fatalf("not paused at a window of 85%%")
	}
	if _, err := c.Call(context.Background(), Request{Model: "m"}); err != ErrPaused {
		t.Errorf("second call gave %v, want ErrPaused", err)
	}
	if len(f.calls) != 1 {
		t.Errorf("%d calls made, want 1", len(f.calls))
	}
}

func cloudTasks(t *testing.T, kind design.Kind) ([]design.Task, *kb.KB) {
	t.Helper()
	k, err := kb.LoadFile("../../data/clouds.json")
	if err != nil {
		t.Fatal(err)
	}
	tasks, err := design.FromKB(k, kind)
	if err != nil {
		t.Fatal(err)
	}
	return tasks, k
}

// Every question gets its first option, at high confidence, except questions
// listed in skip, which are left out of the reply.
func firstOptions(turn string, skip map[int]bool) string {
	var answers []string
	for n := 1; strings.Contains(turn, fmt.Sprintf("Question %d:", n)); n++ {
		if !skip[n] {
			answers = append(answers, fmt.Sprintf(`{"question":%d,"choice":[1],"confidence":"high","cant_tell":null}`, n))
		}
	}
	return `{"answers":[` + strings.Join(answers, ",") + `]}`
}

func TestAssignEndToEnd(t *testing.T) {
	tasks, k := cloudTasks(t, design.Assign)
	f := &fake{reply: func(_ int, turn string) string { return firstOptions(turn, nil) }}
	e := &Expert{Client: f.client(), Model: "claude-opus-5", Effort: "low", Variant: "b", Domain: "clouds"}
	st, _ := design.OpenStore(filepath.Join(t.TempDir(), "v.jsonl"))

	n, err := design.Consult(context.Background(), e, tasks, st)
	if err != nil {
		t.Fatal(err)
	}
	if n != len(tasks) {
		t.Errorf("stored %d verdicts, want %d", n, len(tasks))
	}
	if len(f.calls) != len(k.Entities) {
		t.Errorf("%d calls, want one per entity (%d)", len(f.calls), len(k.Entities))
	}

	c := f.calls[0]
	if tools, ok := arg(c.args, "--tools"); !ok || tools != "" {
		t.Errorf("--tools is %q, %v; want empty", tools, ok)
	}
	if sys, _ := arg(c.args, "--system-prompt"); !strings.Contains(sys, "field guide to clouds") {
		t.Errorf("system prompt is not variant b about clouds:\n%s", sys)
	}
	if schema, _ := arg(c.args, "--json-schema"); !json.Valid([]byte(schema)) {
		t.Errorf("--json-schema is not JSON: %s", schema)
	}
	if !strings.HasPrefix(c.turn, k.Entities[0].Name+"\n") {
		t.Errorf("first call is not about %s:\n%s", k.Entities[0].Name, c.turn)
	}

	v, ok := st.Get(tasks[0].ID, "claude:claude-opus-5@v1b")
	if !ok || v.P[tasks[0].Options[0].Value] != 1 {
		t.Errorf("first task's verdict is %+v, %v", v, ok)
	}
}

func TestMissingAnswersAreAskedOnceMore(t *testing.T) {
	tasks, k := cloudTasks(t, design.Assign)
	one := tasks[:len(k.Attributes)] // the first entity's tasks: one batch
	f := &fake{reply: func(n int, turn string) string {
		if n == 0 {
			return firstOptions(turn, map[int]bool{2: true, 5: true})
		}
		return firstOptions(turn, nil)
	}}
	e := &Expert{Client: f.client(), Model: "m", Variant: "a"}
	var got []design.Verdict
	err := e.Answer(context.Background(), one, func(v design.Verdict) error {
		got = append(got, v)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(one) {
		t.Errorf("%d verdicts, want %d", len(got), len(one))
	}
	if len(f.calls) != 2 || !strings.Contains(f.calls[1].turn, one[1].Question) ||
		strings.Contains(f.calls[1].turn, "Question 3:") {
		t.Errorf("the retry should ask the two missing questions only; calls: %d, retry:\n%s",
			len(f.calls), f.calls[len(f.calls)-1].turn)
	}
}

func TestPerceiveCountsBecomeProbabilities(t *testing.T) {
	tasks, k := cloudTasks(t, design.Perceive)
	colour := k.Attr("colour")
	var batch []design.Task
	for _, task := range tasks {
		if task.Attribute == "colour" {
			batch = append(batch, task)
		}
	}
	f := &fake{reply: func(n int, turn string) string {
		// Case 1: 7 right, 2 on the next option, 1 cannot say. Case 2 does
		// not add up to ten, and again on the retry, where it is case 1, so
		// it is left out.
		if n > 0 {
			return `{"answers":[{"case":1,"counts":[5,5,5],"cannot_answer":0,"reason":null}]}`
		}
		return `{"answers":[
		  {"case":1,"counts":[7,2,0],"cannot_answer":1,"reason":"ambiguous"},
		  {"case":2,"counts":[5,5,5],"cannot_answer":0,"reason":null},
		  {"case":3,"counts":[0,0,10],"cannot_answer":0,"reason":null}]}`
	}}
	e := &Expert{Client: f.client(), Model: "m", Variant: "c"}
	got := map[string]design.Verdict{}
	err := e.Answer(context.Background(), batch, func(v design.Verdict) error {
		got[v.Task] = v
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(f.calls) != 2 {
		t.Errorf("%d calls, want one batch and one retry", len(f.calls))
	}
	if !strings.Contains(f.calls[0].turn, "What is really there: "+colour.Label(colour.Domain[0])) {
		t.Errorf("the turn does not state the truth:\n%s", f.calls[0].turn)
	}
	v := got[batch[0].ID]
	if v.P[colour.Domain[0]] != 0.7 || v.P[colour.Domain[1]] != 0.2 || v.Abstain != 0.1 || v.Reason != design.Ambiguous {
		t.Errorf("case 1 gave %+v", v)
	}
	if _, ok := got[batch[1].ID]; ok {
		t.Errorf("case 2 was kept though its counts add up to 15")
	}
	if len(got) != 2 {
		t.Errorf("%d verdicts, want 2", len(got))
	}
}

func TestAcceptsByVariantAndSubject(t *testing.T) {
	assign := design.NewTask(design.Assign, "x", design.Subject{Entity: "a"}, "q", nil, 2)
	observe := design.NewTask(design.Observe, "x", design.Subject{Photos: []string{"p.jpg"}}, "q", nil, 2)
	blind := design.NewTask(design.Observe, "x", design.Subject{Text: "no photo"}, "q", nil, 2)
	a, b := &Expert{Variant: "a"}, &Expert{Variant: "b"}
	switch {
	case !a.Accepts(assign) || !b.Accepts(assign):
		t.Errorf("assign refused")
	case !a.Accepts(observe):
		t.Errorf("observe refused by variant a")
	case b.Accepts(observe):
		t.Errorf("observe accepted by variant b, which has no observe prompt")
	case a.Accepts(blind):
		t.Errorf("observe accepted without a photo")
	}
}

// Every prompt renders, names the domain, and a new wording needs a new
// PromptVersion; this lists what v1 is, so a change shows up here.
func TestPromptsRender(t *testing.T) {
	for kind, variants := range Variants {
		for _, v := range variants {
			e := &Expert{Variant: v, Domain: "clouds"}
			s, err := e.system(kind)
			if err != nil {
				t.Fatalf("%s-%s: %v", kind, v, err)
			}
			if !strings.Contains(s, "clouds") || strings.Contains(s, "{{") {
				t.Errorf("%s-%s did not render:\n%s", kind, v, s)
			}
			if !strings.Contains(s, "unclear_question") {
				t.Errorf("%s-%s does not explain the abstain reasons", kind, v)
			}
		}
	}
}
