package engine

import (
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/lpapez/ribice/kb"
)

func dataKB(t *testing.T, name string) *kb.KB {
	t.Helper()
	k, err := kb.LoadFile("../data/" + name + ".json")
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	return k
}

// truthSightings records every entity answering every question straight from
// the knowledge base -- the simulator's honest user, written down. Entities
// with several values for some attribute are left out: the simulator picks
// one of them at random per question, which a fixed record cannot mirror.
func truthSightings(k *kb.KB) []Sighting {
	var out []Sighting
	for _, e := range k.Entities {
		s := Sighting{ID: e.Name, Target: e, Answers: map[string]Response{}}
		single := true
		for _, a := range k.Attributes {
			vals := e.Values(a.Name)
			if len(vals) != 1 {
				single = false
				break
			}
			for v := range vals {
				s.Answers[a.Name] = Response{Values: []kb.Value{v}}
			}
		}
		if single {
			out = append(out, s)
		}
	}
	return out
}

// Replaying the knowledge base's own answers must be the simulator: the same
// questions in the same order, the same options, the same guess. This is what
// makes a replay of real answers comparable with -simulate.
//
// Probabilities are compared to within float rounding, not to the bit. The
// engine sums over an entity's value set in map order, so on a knowledge base
// with multi-valued attributes even two runs of Simulate differ in the last
// bits (never, so far, in a question or a guess).
func TestReplayOfTheTruthIsSimulate(t *testing.T) {
	for _, name := range []string{"clouds", "adriatic-fish", "dogs", "example"} {
		t.Run(name, func(t *testing.T) {
			k := dataKB(t, name)
			sightings := truthSightings(k)
			if len(sightings) == 0 {
				t.Skip("every entity has a multi-valued attribute")
			}
			if name == "clouds" && len(sightings) != len(k.Entities) {
				t.Fatalf("%d/%d clouds are single-valued; the gate needs all of them",
					len(sightings), len(k.Entities))
			}

			want := map[*kb.Entity]SimResult{}
			for _, r := range Simulate(k, DefaultConfig(), SimOptions{}).Results {
				want[r.Target] = r
			}
			for _, g := range Replay(k, DefaultConfig(), sightings).Results {
				if diff := gameDiff(g, want[g.Target]); diff != "" {
					t.Errorf("%s: replay and simulate differ: %s", g.Target.Name, diff)
				}
			}
		})
	}
}

// gameDiff describes the first way two games differ, or returns "" if they
// are the same game.
func gameDiff(a, b SimResult) string {
	const eps = 1e-9
	switch {
	case a.GuessName != b.GuessName || a.Rank != b.Rank:
		return fmt.Sprintf("guessed %s (#%d) against %s (#%d)", a.GuessName, a.Rank, b.GuessName, b.Rank)
	case math.Abs(a.Prob-b.Prob) > eps:
		return fmt.Sprintf("probability %v against %v", a.Prob, b.Prob)
	case len(a.Steps) != len(b.Steps):
		return fmt.Sprintf("%d questions against %d", len(a.Steps), len(b.Steps))
	}
	for i := range a.Steps {
		x, y := a.Steps[i], b.Steps[i]
		if math.Abs(x.Entropy-y.Entropy) > eps {
			return fmt.Sprintf("step %d: entropy %v against %v", i+1, x.Entropy, y.Entropy)
		}
		x.Entropy, y.Entropy = 0, 0
		if !reflect.DeepEqual(x, y) {
			return fmt.Sprintf("step %d: %+v against %+v", i+1, x, y)
		}
	}
	return ""
}

func TestOptionsForMapsValuesOntoWhatIsOffered(t *testing.T) {
	q := &Question{Options: []Option{
		{Values: []kb.Value{"a"}},
		{Values: []kb.Value{"b"}},
		{Values: []kb.Value{"c", "d"}, Other: true},
	}}
	for _, tc := range []struct {
		values []kb.Value
		want   []int
	}{
		{[]kb.Value{"b"}, []int{1}},
		{[]kb.Value{"d"}, []int{2}},         // pooled: "something else"
		{[]kb.Value{"c", "d"}, []int{2}},    // both pooled: that option once
		{[]kb.Value{"a", "c"}, []int{0, 2}}, // torn between two options
	} {
		if got := optionsFor(q, tc.values); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("optionsFor(%v) = %v, want %v", tc.values, got, tc.want)
		}
	}
}

func TestReplayTwoValuesPicksBothOptions(t *testing.T) {
	k := load(t, `{"entities": [
	  {"name": "r", "colour": "red",   "size": "big"},
	  {"name": "b", "colour": "blue",  "size": "small"},
	  {"name": "g", "colour": "green", "size": "big"}
	]}`)
	sg := Sighting{ID: "p1", Target: k.Entities[0], Answers: map[string]Response{
		"colour": {Values: []kb.Value{"red", "blue"}},
		"size":   {Values: []kb.Value{"big"}},
	}}
	res := Replay(k, DefaultConfig(), []Sighting{sg}).Results[0]
	if !res.Correct {
		t.Errorf("red-or-blue and big should be r, guessed %s", res.GuessName)
	}
	for _, st := range res.Steps {
		if st.Attr == "colour" && len(st.Picked) != 2 {
			t.Errorf("colour picked %v, want two options", st.Picked)
		}
	}
}

// A sighting that can answer nothing must end, not circle through skipped
// questions forever.
func TestReplayGivesUpWhenOnlyUnanswerableQuestionsAreLeft(t *testing.T) {
	k := dataKB(t, "clouds")
	answers := map[string]Response{}
	for _, a := range k.Attributes {
		answers[a.Name] = Response{CantTell: "not_in_photo"}
	}
	res := Replay(k, DefaultConfig(), []Sighting{{ID: "fog", Target: k.Entities[0], Answers: answers}}).Results[0]
	if !res.GaveUp {
		t.Error("an unanswerable sighting should give up")
	}
	if res.Questions != res.Skipped || res.Questions > len(k.Attributes) {
		t.Errorf("asked %d, skipped %d, with %d attributes", res.Questions, res.Skipped, len(k.Attributes))
	}
	for _, st := range res.Steps {
		if st.Gap {
			t.Errorf("%s: a recorded cant-tell is not a gap", st.Attr)
		}
	}
}

// A question the record has no answer for is a gap: skipped, and flagged so a
// stale cache is not mistaken for a person who could not tell.
func TestReplayMissingAnswerIsAGap(t *testing.T) {
	k := dataKB(t, "clouds")
	target := k.Entities[0]
	answers := map[string]Response{}
	for _, a := range k.Attributes {
		if a.Name == "shape" {
			continue
		}
		for v := range target.Values(a.Name) {
			answers[a.Name] = Response{Values: []kb.Value{v}}
		}
	}
	res := Replay(k, DefaultConfig(), []Sighting{{ID: "p", Target: target, Answers: answers}}).Results[0]
	gaps := 0
	for _, st := range res.Steps {
		if st.Gap {
			gaps++
			if st.Attr != "shape" || !st.Skipped {
				t.Errorf("gap on %s, skipped %v; want only shape, skipped", st.Attr, st.Skipped)
			}
		}
	}
	if gaps == 0 {
		t.Skip("the game never reached shape")
	}
}

func TestLoadSightings(t *testing.T) {
	k := dataKB(t, "clouds")
	good := `{"photo": "a1", "target": "Cumulus mediocris", "split": "tune",
	  "answers": {"shape": {"values": ["heaped"], "confidence": "high"},
	              "shading": {"values": ["Yes"]},
	              "halo": {"cant_tell": "not_in_photo"}}}
	{"photo": "a2", "target": "Nimbostratus", "answers": {}}`
	ss, err := LoadSightings(strings.NewReader(good), k)
	if err != nil {
		t.Fatalf("LoadSightings: %v", err)
	}
	if len(ss) != 2 || ss[0].Split != "tune" || ss[0].Target.Name != "Cumulus mediocris" {
		t.Fatalf("parsed %+v", ss)
	}
	if got := ss[0].Answers["shading"].Values; !reflect.DeepEqual(got, []kb.Value{kb.Yes}) {
		t.Errorf("shading = %v, want [yes]", got)
	}
	if ss[0].Answers["halo"].CantTell != "not_in_photo" {
		t.Errorf("halo = %+v", ss[0].Answers["halo"])
	}

	for _, tc := range []struct{ line, want string }{
		{`{"target": "Nimbostratus", "answers": {}}`, `missing "photo"`},
		{`{"photo": "x", "target": "Cumulus giganticus", "answers": {}}`, "not in the knowledge base"},
		{`{"photo": "x", "target": "Nimbostratus", "answers": {"smell": {"values": ["damp"]}}}`, `no attribute "smell"`},
		{`{"photo": "x", "target": "Nimbostratus", "answers": {"shape": {"values": ["square"]}}}`, `"square" is not one of`},
		{`{"photo": "x", "target": "Nimbostratus", "answers": {"shape": {"values": ["sheet"], "cant_tell": "ambiguous"}}}`, "both"},
		{`{"photo": "x", "target": "Nimbostratus", "answers": {"shape": {}}}`, "neither"},
		{`{"photo": "x", "target": "Nimbostratus", "answers": {}, "notes": "hi"}`, "unknown field"},
		{`{"photo": "x", "target": "Nimbostratus", "answers": {}} {"photo": "x", "target": "Nimbostratus", "answers": {}}`, "appears twice"},
	} {
		_, err := LoadSightings(strings.NewReader(tc.line), k)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s\n  error %v, want one containing %q", tc.line, err, tc.want)
		}
	}
}
