package human

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lpapez/ribice/design"
	"github.com/lpapez/ribice/kb"
)

var sky = []design.Option{{Value: "white", Label: "white"}, {Value: "grey", Label: "grey"}, {Value: "dark_grey", Label: "dark grey"}}

func tasks(n int) []design.Task {
	var out []design.Task
	for i := 0; i < n; i++ {
		s := design.Subject{Entity: string(rune('A' + i))}
		out = append(out, design.NewTask(design.Assign, "colour", s, "What colour was it?", sky, 2))
	}
	return out
}

// answer runs the expert over tasks with typed as its input, and returns the
// verdicts in the order emitted and what it printed.
func answer(t *testing.T, ts []design.Task, typed string) ([]design.Verdict, string) {
	t.Helper()
	var out strings.Builder
	e := &Expert{Name: "luka", In: strings.NewReader(typed), Out: &out}
	var got []design.Verdict
	err := e.Answer(context.Background(), ts, func(v design.Verdict) error {
		for _, task := range ts {
			if task.ID == v.Task {
				if err := v.Check(task); err != nil {
					t.Errorf("invalid verdict: %v", err)
				}
			}
		}
		got = append(got, v)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return got, out.String()
}

func TestAnswers(t *testing.T) {
	ts := tasks(4)
	got, out := answer(t, ts, "2\n1,3?\nn\n9\n1,2,3\nd\n")
	if len(got) != 4 {
		t.Fatalf("%d verdicts, want 4:\n%s", len(got), out)
	}
	want := []struct {
		p       map[kb.Value]float64
		abstain float64
		reason  design.Reason
	}{
		{map[kb.Value]float64{"white": 0, "grey": 1, "dark_grey": 0}, 0, ""},
		{map[kb.Value]float64{"white": 0.25, "grey": 0.5, "dark_grey": 0.25}, 0, ""},
		{nil, 1, design.NotObservable},
		{nil, 1, design.Unknown},
	}
	for i, w := range want {
		v := got[i]
		if v.Expert != "human:luka" || v.Task != ts[i].ID || v.Abstain != w.abstain || v.Reason != w.reason {
			t.Errorf("verdict %d = %+v", i+1, v)
		}
		for val, p := range w.p {
			if v.P[val] != p {
				t.Errorf("verdict %d: p[%s] = %v, want %v", i+1, val, v.P[val], p)
			}
		}
	}
	if !strings.Contains(out, "at most 2 here") {
		t.Errorf("three picks were not refused:\n%s", out)
	}
	if !strings.Contains(out, "[assign 1/4]  A") || !strings.Contains(out, " 3) dark grey") {
		t.Errorf("tasks not shown as expected:\n%s", out)
	}
}

func TestBackReplacesTheLastAnswer(t *testing.T) {
	ts := tasks(2)
	st, _ := design.OpenStore(filepath.Join(t.TempDir(), "v.jsonl"))
	e := &Expert{Name: "luka", In: strings.NewReader("b\n1\nb\n3\n2\n"), Out: &strings.Builder{}}
	n, err := design.Consult(context.Background(), e, ts, st)
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Errorf("stored %d verdicts, want 3 (the first answer, its correction, the second)", n)
	}
	v, _ := st.Get(ts[0].ID, e.ID())
	if v.P["dark_grey"] != 1 {
		t.Errorf("first task kept %+v, want the corrected answer", v.P)
	}
}

func TestQuitKeepsWhatWasAnswered(t *testing.T) {
	ts := tasks(3)
	st, _ := design.OpenStore(filepath.Join(t.TempDir(), "v.jsonl"))
	e := &Expert{Name: "luka", In: strings.NewReader("1\nq\n"), Out: &strings.Builder{}}
	if n, err := design.Consult(context.Background(), e, ts, st); err != nil || n != 1 {
		t.Fatalf("stored %d, %v; want 1", n, err)
	}

	// The next run starts where the last stopped.
	var out strings.Builder
	e = &Expert{Name: "luka", In: strings.NewReader("2\n2\n"), Out: &out}
	if n, err := design.Consult(context.Background(), e, ts, st); err != nil || n != 2 {
		t.Fatalf("second run stored %d, %v; want 2", n, err)
	}
	if !strings.Contains(out.String(), "2 to answer") || !strings.Contains(out.String(), "  B\n") {
		t.Errorf("second run did not start at the second task:\n%s", out.String())
	}
}

func TestShowsEachKind(t *testing.T) {
	perceive := design.NewTask(design.Perceive, "colour", design.Subject{Value: "grey", Text: "grey"},
		"What colour was it?", sky, 2)
	observe := design.NewTask(design.Observe, "colour", design.Subject{Photos: []string{"sky.jpg"}},
		"What colour was it?", sky, 2)
	var opened []string
	var out strings.Builder
	e := &Expert{Name: "x", In: strings.NewReader("2\n1\n"), Out: &out,
		Open: func(p string) error { opened = append(opened, p); return nil }}
	if err := e.Answer(context.Background(), []design.Task{perceive, observe}, func(design.Verdict) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "What is really there: grey") || !strings.Contains(out.String(), "photo  sky.jpg") {
		t.Errorf("unexpected display:\n%s", out.String())
	}
	if len(opened) != 1 || opened[0] != "sky.jpg" {
		t.Errorf("opened %v, want the photo", opened)
	}
}

func TestEndOfInputSaysSo(t *testing.T) {
	_, out := answer(t, tasks(3), "1\n")
	if !strings.Contains(out, "input ended with 1 of 3 answered") {
		t.Errorf("no word about the input ending:\n%s", out)
	}
}
