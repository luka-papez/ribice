package design

import (
	"os"
	"strings"
	"testing"

	"github.com/lpapez/ribice/kb"
)

func TestSetErrorsRewritesOnlyTheErrorModel(t *testing.T) {
	data, _ := os.ReadFile("../testdata/clouds-2026-09-26.json")
	f, _ := ParseKBFile(data)
	m := ErrorModel{Noise: 0.0512, Confusion: 0.3, Confusable: [][2]kb.Value{{"white", "grey"}},
		AnswerRate: 0.8, Cost: 1.25}
	if err := f.SetErrors("colour", m); err != nil {
		t.Fatal(err)
	}
	k, err := kb.Load(f.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	a := k.Attr("colour")
	if a.Noise != 0.051 || a.Confusion != 0.3 || a.AnswerRate != 0.8 || a.Cost != 1.25 ||
		len(a.Confusable["white"]) != 1 || a.Title() != "What colour was it?" {
		t.Errorf("colour loaded as noise %v, confusion %v, answer rate %v, cost %v, look-alikes %v, %q",
			a.Noise, a.Confusion, a.AnswerRate, a.Cost, a.Confusable, a.Title())
	}

	// Back to defaults: the optional fields go away.
	if err := f.SetErrors("colour", ErrorModel{Noise: 0.1, AnswerRate: 1, Cost: 1}); err != nil {
		t.Fatal(err)
	}
	out := string(f.Bytes())
	i := strings.Index(out, `"colour": {`)
	block := out[i : i+strings.Index(out[i:], "\n  },")]
	for _, gone := range []string{"confusion", "confusable", "answer_rate"} {
		if strings.Contains(block, gone) {
			t.Errorf("%s left in:\n%s", gone, block)
		}
	}
	// Nothing else in the file moved.
	g, _ := ParseKBFile(data)
	g.SetErrors("colour", ErrorModel{Noise: 0.1, AnswerRate: 1, Cost: 1})
	if strings.Count(string(g.Bytes()), "\n") != strings.Count(out, "\n") {
		t.Errorf("the rewrite changed the file's length")
	}
}

func TestReplaceAttributeKeepsItsPlace(t *testing.T) {
	data, _ := os.ReadFile("../testdata/clouds-2026-09-26.json")
	f, _ := ParseKBFile(data)
	before := f.Attributes()
	p := Proposal{Name: "colour", Kind: "categorical", Question: "How bright?",
		Values: []Option{{"white", "white"}, {"grey", "grey"}},
		Assign: map[string][]kb.Value{}}
	if err := f.ReplaceAttribute(p, ErrorModel{Noise: 0.1, AnswerRate: 1, Cost: 1}); err != nil {
		t.Fatal(err)
	}
	after := f.Attributes()
	if len(after) != len(before) {
		t.Fatalf("%d attributes, want %d", len(after), len(before))
	}
	for i := range before {
		if before[i] != after[i] {
			t.Errorf("attribute %d is %s, want %s", i, after[i], before[i])
		}
	}
	k, _ := kb.Load(f.Bytes())
	if k.Attr("colour").Title() != "How bright?" {
		t.Errorf("colour was not replaced")
	}
}

func TestRefit(t *testing.T) {
	k := cloudKB(t)
	tasks, err := FromKB(k, Perceive)
	if err != nil {
		t.Fatal(err)
	}
	// Everyone answers right, except that halo has no answers at all and
	// one of colour's values is left out.
	byTask := map[string][]Verdict{}
	for _, task := range tasks {
		if task.Attribute == "halo" || (task.Attribute == "colour" && task.Subject.Value == "dark_grey") {
			continue
		}
		byTask[task.ID] = []Verdict{{Task: task.ID, Expert: "x", P: map[kb.Value]float64{task.Subject.Value: 1}}}
	}
	fitted, missing := Refit(k, tasks, func(id string) []Verdict { return byTask[id] })
	if _, ok := fitted["shape"]; !ok || fitted["shape"].Noise != minNoise {
		t.Errorf("shape fitted as %+v", fitted["shape"])
	}
	if _, ok := fitted["colour"]; ok || len(missing["colour"]) != 1 {
		t.Errorf("colour, with a value unanswered, fitted anyway; missing %v", missing["colour"])
	}
	if _, ok := fitted["halo"]; ok {
		t.Errorf("halo fitted without any answers")
	}
	if len(fitted)+len(missing) != len(k.Attributes) {
		t.Errorf("%d fitted and %d missing of %d", len(fitted), len(missing), len(k.Attributes))
	}
}
