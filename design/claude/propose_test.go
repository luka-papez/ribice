package claude

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/lpapez/ribice/design"
	"github.com/lpapez/ribice/kb"
)

// streaks is a valid proposal for the cloud guide: every cloud gets "none"
// except the two cirrus that the question is for.
func streaks(k *kb.KB) proposedAttr {
	var a proposedAttr
	raw := `{"name": "streak_ends", "replaces": "hooks", "kind": "categorical",
	  "question": "Did the ends of the streaks curl up?",
	  "values": [{"value": "none", "label": "no streaks"}, {"value": "straight", "label": "straight"},
	             {"value": "curled", "label": "curled up"}],
	  "confusable": [["straight", "curled"]],
	  "targets": [["Cirrus fibratus", "Cirrus uncinus"], ["Cirrus fibratus", "Nowhere"]],
	  "rationale": "hooks is asked as yes/no; this keeps 'no streaks' apart.", "assign": []}`
	if err := json.Unmarshal([]byte(raw), &a); err != nil {
		panic(err)
	}
	for _, e := range k.Entities {
		v := "none"
		switch e.Name {
		case "Cirrus fibratus":
			v = "straight"
		case "Cirrus uncinus":
			v = "curled"
		}
		a.Assign = append(a.Assign, struct {
			Entity string   `json:"entity"`
			Values []string `json:"values"`
		}{e.Name, []string{v}})
	}
	return a
}

func TestConvert(t *testing.T) {
	_, k := cloudTasks(t, design.Assign)
	p, err := convert(streaks(k), k, "x")
	if err != nil {
		t.Fatal(err)
	}
	if p.Replaces != "hooks" || len(p.Values) != 3 || p.Assign["Cirrus uncinus"][0] != "curled" {
		t.Errorf("converted %+v", p)
	}
	if len(p.Targets) != 1 {
		t.Errorf("targets %v: the pair with an unknown cloud should be dropped", p.Targets)
	}

	for name, spoil := range map[string]func(*proposedAttr){
		"bad name":        func(a *proposedAttr) { a.Name = "Streak Ends" },
		"replaces nobody": func(a *proposedAttr) { s := "wings"; a.Replaces = &s },
		"unknown value":   func(a *proposedAttr) { a.Assign[0].Values = []string{"hooked"} },
		"three values":    func(a *proposedAttr) { a.Assign[0].Values = []string{"none", "straight", "curled"} },
		"missing cloud":   func(a *proposedAttr) { a.Assign = a.Assign[1:] },
		"cloud twice":     func(a *proposedAttr) { a.Assign = append(a.Assign, a.Assign[0]) },
		"unknown cloud":   func(a *proposedAttr) { a.Assign[0].Entity = "Cumulus maximus" },
		"bad look-alike":  func(a *proposedAttr) { a.Confusable = [][]string{{"straight", "hooked"}} },
		"boolean":         func(a *proposedAttr) { a.Kind = "boolean" },
		"one value":       func(a *proposedAttr) { a.Values = a.Values[:1] },
	} {
		a := streaks(k)
		spoil(&a)
		if _, err := convert(a, k, "x"); err == nil {
			t.Errorf("%s: converted without an error", name)
		}
	}

	// Reusing a current question's name is rewriting it.
	a := streaks(k)
	a.Name, a.Replaces = "hooks", nil
	if p, err := convert(a, k, "x"); err != nil || p.Replaces != "hooks" {
		t.Errorf("reusing the name hooks gave %q, %v", p.Replaces, err)
	}
}

func TestProposeKeepsTheGoodProposals(t *testing.T) {
	_, k := cloudTasks(t, design.Assign)
	good := streaks(k)
	bad := streaks(k)
	bad.Name = "streak_colour"
	bad.Assign = bad.Assign[1:]
	again := streaks(k)
	reply, _ := json.Marshal(proposeReply{Attributes: []proposedAttr{good, bad, again}})
	f := &fake{reply: func(int, string) string { return string(reply) }}
	var log strings.Builder
	p := &Proposer{Client: f.client(), Model: "m", Domain: "clouds", Log: &log}

	b := design.Brief{KB: k, Targets: []design.Pair{{A: "Cirrus fibratus", B: "Cirrus uncinus", Mixups: 9,
		SeparatedBy: []string{"hooks"}}}, Evidence: "hooks agreed on all 32"}
	props, err := p.Propose(context.Background(), b)
	if err != nil {
		t.Fatal(err)
	}
	if len(props) != 1 || props[0].ID != "p01" || props[0].Proposer != "claude:m@propose-v1" {
		t.Errorf("kept %+v, want the first proposal only", props)
	}
	if !strings.Contains(log.String(), "streak_colour") || !strings.Contains(log.String(), "second question named") {
		t.Errorf("log does not say what was dropped:\n%s", log.String())
	}

	c := f.calls[0]
	if effort, _ := arg(c.args, "--effort"); effort != "high" {
		t.Errorf("effort %q, want high", effort)
	}
	for _, want := range []string{"Cirrus fibratus / Cirrus uncinus: 9 mix-ups", "hooks agreed on all 32",
		"## shape (categorical)", fmt.Sprintf("%d entries", len(k.Entities))} {
		if !strings.Contains(c.turn, want) {
			t.Errorf("the brief does not contain %q", want)
		}
	}
}

func TestProposalTasksAreBlind(t *testing.T) {
	_, k := cloudTasks(t, design.Assign)
	p, err := convert(streaks(k), k, "x")
	if err != nil {
		t.Fatal(err)
	}
	tasks, err := design.FromProposals(k, []design.Proposal{p}, design.Assign)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != len(k.Entities) {
		t.Fatalf("%d tasks, want one per cloud", len(tasks))
	}
	raw, _ := json.Marshal(tasks)
	if strings.Contains(string(raw), "rationale") || strings.Contains(string(raw), p.Rationale) {
		t.Errorf("tasks carry the proposer's rationale")
	}
	values := design.ProposedValues([]design.Proposal{p})
	for _, task := range tasks {
		if task.Subject.Entity == "Cirrus uncinus" && values(task)[0] != "curled" {
			t.Errorf("proposed value for Cirrus uncinus is %v", values(task))
		}
	}
}
