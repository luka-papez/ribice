package design

import (
	"math"
	"reflect"
	"testing"

	"github.com/lpapez/ribice/kb"
)

var abcd = []Option{{"a", "A"}, {"b", "B"}, {"c", "C"}, {"d", "nothing to judge by"}}

// perceive makes one perceive task per truth, and a verdict source giving
// each the verdicts listed for it.
func perceive(rows map[kb.Value][]Verdict) ([]Task, func(string) []Verdict) {
	var tasks []Task
	byTask := map[string][]Verdict{}
	for _, o := range abcd {
		vs, ok := rows[o.Value]
		if !ok {
			continue
		}
		t := NewTask(Perceive, "q", Subject{Value: o.Value, Text: o.Label}, "Q?", abcd, 2)
		tasks = append(tasks, t)
		for _, v := range vs {
			v.Task = t.ID
			byTask[t.ID] = append(byTask[t.ID], v)
		}
	}
	return tasks, func(id string) []Verdict { return byTask[id] }
}

func near(x, y float64) bool { return math.Abs(x-y) < 1e-9 }

func TestFitErrors(t *testing.T) {
	// Two experts on a; one each on b and c. d is held by nobody, so c's
	// 0.2 on it counts as not answering.
	tasks, verdicts := perceive(map[kb.Value][]Verdict{
		"a": {{Expert: "x", P: map[kb.Value]float64{"a": 0.9, "b": 0.1}},
			{Expert: "y", P: map[kb.Value]float64{"a": 0.8, "b": 0.1, "c": 0.1}}},
		"b": {{Expert: "x", P: map[kb.Value]float64{"b": 0.5, "a": 0.3, "c": 0.2}}},
		"c": {{Expert: "x", P: map[kb.Value]float64{"c": 0.6, "d": 0.2}, Abstain: 0.2, Reason: Ambiguous}},
	})
	m := FitErrors("Q?", abcd, map[kb.Value]int{"a": 2, "b": 1, "c": 1}, tasks, verdicts)

	// Rows: a answers a .85, b .1, c .05; b answers b .5, a .3, c .2;
	// c answers c 1, with .4 not answering.
	if len(m.Rows) != 3 || !near(m.Rows[2].Abstain, 0.4) || !near(m.Rows[2].Answers["c"], 1) {
		t.Fatalf("rows %+v", m.Rows)
	}
	// a-b (.1 one way, .3 the other) and b-c (.2) look alike; a-c (.05 and
	// 0) does not.
	if want := [][2]kb.Value{{"a", "b"}, {"b", "c"}}; !reflect.DeepEqual(m.Confusable, want) {
		t.Errorf("confusable %v, want %v", m.Confusable, want)
	}
	// Weights: a 2, b 1, c 1 x 0.6 answered. Off look-alikes: only a's .05
	// on c, so noise = 2(.05)/3.6. On look-alikes: a .1, b .5, c 0.
	if want := 0.1 / 3.6; !near(m.Noise, want) {
		t.Errorf("noise %v, want %v", m.Noise, want)
	}
	if want := 0.7 / 3.6; !near(m.Confusion, want) {
		t.Errorf("confusion %v, want %v", m.Confusion, want)
	}
	// Games meet a twice as often; only c's .4 goes unanswered: 1 - .4/4.
	// Reading: "Q?" and three one-word options, each 1 + 2 words: 10 words.
	if want := 10.0 / UnitWords / 0.9; !near(m.AnswerRate, 0.9) || !near(m.Cost, want) {
		t.Errorf("answer rate %v, cost %v; want 0.9 and %v", m.AnswerRate, m.Cost, want)
	}
	if len(m.Missing) != 0 {
		t.Errorf("missing %v", m.Missing)
	}
}

func TestFitErrorsWithTwoValuesIsAllNoise(t *testing.T) {
	tasks, verdicts := perceive(map[kb.Value][]Verdict{
		"a": {{Expert: "x", P: map[kb.Value]float64{"a": 0.7, "b": 0.3}}},
		"b": {{Expert: "x", P: map[kb.Value]float64{"b": 0.9, "a": 0.1}}},
	})
	m := FitErrors("Q?", abcd, map[kb.Value]int{"a": 1, "b": 1}, tasks, verdicts)
	if len(m.Confusable) != 0 || m.Confusion != 0 || !near(m.Noise, 0.2) {
		t.Errorf("fit %+v, want noise 0.2 and nothing else", m)
	}
}

func TestFitErrorsFloorsAndCaps(t *testing.T) {
	perfect, verdicts := perceive(map[kb.Value][]Verdict{
		"a": {{Expert: "x", P: map[kb.Value]float64{"a": 1}}},
		"b": {{Expert: "x", P: map[kb.Value]float64{"b": 1}}},
	})
	m := FitErrors("Q?", abcd, map[kb.Value]int{"a": 1, "b": 1, "c": 1}, perfect, verdicts)
	if m.Noise != minNoise || !reflect.DeepEqual(m.Missing, []kb.Value{"c"}) {
		t.Errorf("perfect answers gave noise %v, missing %v; want the floor, and c missing", m.Noise, m.Missing)
	}

	hopeless, verdicts := perceive(map[kb.Value][]Verdict{
		"a": {{Expert: "x", P: map[kb.Value]float64{"b": 0.5, "c": 0.5}}},
		"b": {{Expert: "x", P: map[kb.Value]float64{"a": 0.5, "c": 0.5}}},
		"c": {{Expert: "x", P: map[kb.Value]float64{"a": 0.1}, Abstain: 0.9, Reason: NotObservable}},
	})
	m = FitErrors("Q?", abcd, map[kb.Value]int{"a": 1, "b": 1, "c": 1}, hopeless, verdicts)
	if m.Noise+m.Confusion > maxError+1e-12 {
		t.Errorf("noise %v plus confusion %v is over %v", m.Noise, m.Confusion, maxError)
	}
	if !near(m.AnswerRate, 0.7) || !near(m.Cost, 10.0/UnitWords/0.7) {
		t.Errorf("answer rate %v, cost %v", m.AnswerRate, m.Cost)
	}

	none := FitErrors("Q?", abcd, map[kb.Value]int{"a": 1}, nil, func(string) []Verdict { return nil })
	if len(none.Rows) != 0 || none.AnswerRate != 1 || len(none.Missing) != 1 {
		t.Errorf("no verdicts gave %+v", none)
	}
}

func TestBuildPool(t *testing.T) {
	k := cloudKB(t)
	var p Proposal
	p.ID, p.Name, p.Kind, p.Question = "p01", "streaks", "boolean", "Streaks?"
	p.Values = []Option{{"yes", "yes"}, {"no", "no"}}
	p.Assign = map[string][]kb.Value{}
	for _, e := range k.Entities {
		p.Assign[e.Name] = []kb.Value{"no"}
	}
	p.Assign["Cirrus fibratus"] = []kb.Value{"yes"}
	p.Assign["Cirrus uncinus"] = nil // the proposer did not know

	assign, _ := FromProposals(k, []Proposal{p}, Assign)
	perc, _ := FromProposals(k, []Proposal{p}, Perceive)
	byTask := map[string][]Verdict{}
	say := func(t Task, v kb.Value) {
		byTask[t.ID] = append(byTask[t.ID], Verdict{Task: t.ID, Expert: "x", P: map[kb.Value]float64{v: 1}})
	}
	for _, task := range assign {
		switch task.Subject.Entity {
		case "Cirrus fibratus", "Cirrus uncinus":
			say(task, "yes")
		case "Stratus nebulosus":
			say(task, "yes") // disputes the proposer's no
		default:
			say(task, "no")
		}
	}
	for _, task := range perc {
		say(task, task.Subject.Value)
	}
	verdicts := func(id string) []Verdict { return byTask[id] }

	pool := BuildPool([]Proposal{p}, assign, perc, verdicts)
	e := pool[0]
	if e.Agreed != len(k.Entities)-1 || !reflect.DeepEqual(e.Disputed, []string{"Stratus nebulosus"}) {
		t.Errorf("agreed %d, disputed %v", e.Agreed, e.Disputed)
	}
	if got := e.Proposal.Assign["Cirrus uncinus"]; len(got) != 1 || got[0] != "yes" {
		t.Errorf("Cirrus uncinus settled to %v, want the experts' yes", got)
	}
	if e.Ready || len(e.Why) != 1 {
		t.Errorf("ready %v, why %v: want held back for the one dispute", e.Ready, e.Why)
	}
	if e.Errors.AnswerRate != 1 || e.Errors.Noise != minNoise {
		t.Errorf("errors %+v", e.Errors)
	}
	if p.Assign["Cirrus uncinus"] != nil {
		t.Errorf("BuildPool changed the proposal it was given")
	}

	// Once a person agrees with the proposer, the question is ready.
	for _, task := range assign {
		if task.Subject.Entity == "Stratus nebulosus" {
			byTask[task.ID] = append(byTask[task.ID], Verdict{Task: task.ID, Expert: "human:luka",
				P: map[kb.Value]float64{"no": 1}})
		}
	}
	if e := BuildPool([]Proposal{p}, assign, perc, verdicts)[0]; !e.Ready {
		t.Errorf("still held back after a person settled it: %v", e.Why)
	}
}
