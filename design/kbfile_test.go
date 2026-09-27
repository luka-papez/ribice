package design

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/lpapez/ribice/kb"
)

// The generated knowledge bases read and write back byte for byte, so a
// candidate's diff shows only what was changed. The fish one is formatted by
// hand, blank lines and one-line objects included, which no JSON writer
// keeps; editing it would reformat it.
func TestKBFileRoundTrip(t *testing.T) {
	for _, name := range []string{"clouds", "dogs"} {
		data, err := os.ReadFile("../data/" + name + ".json")
		if err != nil {
			t.Fatal(err)
		}
		f, err := ParseKBFile(data)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got := f.Bytes(); !bytes.Equal(got, data) {
			i := 0
			for i < len(got) && i < len(data) && got[i] == data[i] {
				i++
			}
			t.Errorf("%s changed at byte %d: %q against %q", name, i,
				got[max(0, i-40):min(len(got), i+40)], data[max(0, i-40):min(len(data), i+40)])
		}
	}
}

func TestKBFileEdits(t *testing.T) {
	data, _ := os.ReadFile("../testdata/clouds-2026-09-26.json")
	f, err := ParseKBFile(data)
	if err != nil {
		t.Fatal(err)
	}
	f.DropAttribute("hooks")
	p := Proposal{Name: "streak_ends", Kind: "categorical", Question: "Did the ends curl up?",
		Values: []Option{{"none", "no streaks"}, {"straight", "straight"}, {"curled", "curled up & over"}},
		Assign: map[string][]kb.Value{"Cirrus uncinus": {"curled"}, "Cirrus fibratus": {"straight", "curled"}}}
	for _, e := range f.entities {
		var name string
		_ = jsonString(e.vals["name"], &name)
		if _, ok := p.Assign[name]; !ok && name != "Nimbostratus" {
			p.Assign[name] = []kb.Value{"none"}
		}
	}
	m := ErrorModel{Noise: 0.0412345, Confusion: 0.2, Confusable: [][2]kb.Value{{"straight", "curled"}},
		AnswerRate: 0.9, Cost: 1.1111}
	if err := f.AddAttribute(p, m); err != nil {
		t.Fatal(err)
	}
	if err := f.AddAttribute(p, m); err == nil {
		t.Errorf("added the same attribute twice")
	}
	out := string(f.Bytes())
	for _, want := range []string{`"streak_ends": {
   "question": "Did the ends curl up?",
   "noise": 0.041,
   "confusion": 0.2,`, `"answer_rate": 0.9`, `"curled up & over"`,
		`"streak_ends": "curled",
   "_prior"`, `"streak_ends": [
    "straight",
    "curled"
   ]`} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q", want)
		}
	}
	if strings.Contains(out, `"hooks"`) {
		t.Errorf("hooks is still there")
	}

	k, err := kb.Load(f.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	a := k.Attr("streak_ends")
	if a == nil || a.Noise != 0.041 || a.AnswerRate != 0.9 || len(a.Confusable["curled"]) != 1 {
		t.Fatalf("streak_ends loaded as %+v", a)
	}
	for _, e := range k.Entities {
		if e.Name == "Nimbostratus" && len(e.Values("streak_ends")) != 3 {
			t.Errorf("an entity without a value should hold every value, has %v", e.Values("streak_ends"))
		}
	}
}

func TestKBFileBooleanIsWrittenOnlyWhenTrue(t *testing.T) {
	f, err := ParseKBFile([]byte(`{"name": "n", "entities": [{"name": "a"}, {"name": "b", "_note": "x"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	p := Proposal{Name: "tails", Kind: "boolean", Question: "Tails?",
		Values: []Option{{"yes", "yes"}, {"no", "no"}},
		Assign: map[string][]kb.Value{"a": {"no"}, "b": {"yes"}}}
	if err := f.AddAttribute(p, ErrorModel{Noise: 0.1, Cost: 1, AnswerRate: 1}); err != nil {
		t.Fatal(err)
	}
	got := string(f.Bytes())
	if !strings.Contains(got, `{
   "name": "b",
   "tails": true,
   "_note": "x"
  }`) || strings.Count(got, `"tails": true`) != 1 || strings.Contains(got, "labels") ||
		strings.Contains(got, "answer_rate") {
		t.Errorf("wrote:\n%s", got)
	}
}
