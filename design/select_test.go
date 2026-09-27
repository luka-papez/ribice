package design

import (
	"strings"
	"testing"

	"github.com/lpapez/ribice/engine"
	"github.com/lpapez/ribice/kb"
)

// Four entities that one question splits into two pairs, and nothing splits
// further: a proposal that splits each pair is the obvious move.
const pairsKB = `{
 "name": "pairs",
 "attributes": {
  "side": {
   "question": "Which side?",
   "noise": 0.05,
   "cost": 1
  }
 },
 "entities": [
  {"name": "a", "side": "left", "_note": "a"},
  {"name": "b", "side": "left"},
  {"name": "c", "side": "right"},
  {"name": "d", "side": "right"}
 ]
}
`

func pairsPool(answerRate float64, disputed []string) PoolEntry {
	p := Proposal{ID: "p01", Name: "top", Kind: "categorical", Question: "Which way up?",
		Values: []Option{{"up", "up"}, {"down", "down"}},
		Assign: map[string][]kb.Value{"a": {"up"}, "b": {"down"}, "c": {"up"}, "d": {"down"}}}
	m := ErrorModel{Noise: 0.02, AnswerRate: answerRate, Cost: 1,
		Rows: []Row{{Truth: "up", Weight: 2}, {Truth: "down", Weight: 2}}}
	return PoolEntry{Proposal: p, Agreed: 4 - len(disputed), Disputed: disputed, Errors: m}
}

func TestSelectTakesTheQuestionThatHelps(t *testing.T) {
	base, err := ParseKBFile([]byte(pairsKB))
	if err != nil {
		t.Fatal(err)
	}
	r, err := Select(base, []PoolEntry{pairsPool(1, nil)}, engine.DefaultConfig(), SelectOptions{Seeds: 5})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Steps) == 0 || r.Steps[0].Move != (Move{Add: "top"}) {
		t.Fatalf("steps %+v, want adding top first", r.Steps)
	}
	if r.After.Accuracy <= r.Before.Accuracy || r.After.Value <= r.Before.Value {
		t.Errorf("before %+v, after %+v", r.Before, r.After)
	}
	k, err := kb.Load(r.File)
	if err != nil {
		t.Fatal(err)
	}
	if k.Attr("top") == nil || !k.Entities[1].Has("top", "down") {
		t.Errorf("the candidate does not hold the new question")
	}
	if !strings.Contains(string(r.File), `"top": "up",
   "_note": "a"`) {
		t.Errorf("the value is not written before the entity's underscored keys:\n%s", r.File)
	}

	again, _ := Select(base, []PoolEntry{pairsPool(1, nil)}, engine.DefaultConfig(), SelectOptions{Seeds: 5})
	if string(again.File) != string(r.File) || again.After != r.After {
		t.Errorf("a second selection differs from the first")
	}
}

func TestSelectLeavesOutWhatItMayNotUse(t *testing.T) {
	base, _ := ParseKBFile([]byte(pairsKB))
	for name, c := range map[string]struct {
		entry PoolEntry
		opts  SelectOptions
		why   string
	}{
		"few can answer":  {pairsPool(0.3, nil), SelectOptions{Seeds: 2}, "only 30% could answer"},
		"disputed":        {pairsPool(1, []string{"b"}), SelectOptions{Seeds: 2}, "1 values to settle"},
		"no perceive yet": {PoolEntry{Proposal: pairsPool(1, nil).Proposal}, SelectOptions{Seeds: 2}, "no perceive answers"},
	} {
		r, err := Select(base, []PoolEntry{c.entry}, engine.DefaultConfig(), c.opts)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(r.Skipped["top"], c.why) || len(r.Added) != 0 {
			t.Errorf("%s: skipped %q, added %v", name, r.Skipped["top"], r.Added)
		}
	}

	// Allowed in, a disputed question is added and its values are listed.
	r, _ := Select(base, []PoolEntry{pairsPool(1, []string{"b"})}, engine.DefaultConfig(),
		SelectOptions{Seeds: 2, AllowDisputed: true})
	if len(r.Added) != 1 || r.Pending["top"] != 1 {
		t.Errorf("added %v, pending %v", r.Added, r.Pending)
	}
	// Without perceive answers, it goes in with the default error rates.
	r, _ = Select(base, []PoolEntry{{Proposal: pairsPool(1, nil).Proposal}}, engine.DefaultConfig(),
		SelectOptions{Seeds: 2, AllowGuessed: true})
	if len(r.Guessed) != 1 {
		t.Errorf("guessed %v", r.Guessed)
	}
	if !strings.Contains(SelectReport(r, nil), "default error rates") {
		t.Errorf("the report does not say the rates were guessed")
	}
}

// Scores reported are the confirmed ones, and every move confirmed is
// listed, taken or not.
func TestSelectConfirmsOnMoreGames(t *testing.T) {
	base, _ := ParseKBFile([]byte(pairsKB))
	r, err := Select(base, []PoolEntry{pairsPool(1, nil)}, engine.DefaultConfig(),
		SelectOptions{Seeds: 3, ConfirmSeeds: 25, MinGain: 2})
	if err != nil {
		t.Fatal(err)
	}
	if r.Before.Games != 25*4 || r.After.Games != 25*4 {
		t.Errorf("before %d games, after %d; want 100 each, the confirming run", r.Before.Games, r.After.Games)
	}
	if len(r.Steps) == 0 || len(r.Tried) < len(r.Steps) || r.Tried[0].Move != r.Steps[0].Move {
		t.Errorf("steps %+v, tried %+v", r.Steps, r.Tried)
	}

	// A gain no move can reach is never taken, though moves were tried.
	r, _ = Select(base, []PoolEntry{pairsPool(1, nil)}, engine.DefaultConfig(),
		SelectOptions{Seeds: 3, ConfirmSeeds: 25, MinGain: 1000})
	if len(r.Steps) != 0 || len(r.Tried) == 0 {
		t.Errorf("steps %v, tried %v; want nothing taken after trying", r.Steps, r.Tried)
	}
}

// Dropping the one question that tells two entities apart is never taken,
// whatever it saves in questions.
func TestSelectKeepsEntitiesSeparable(t *testing.T) {
	base, _ := ParseKBFile([]byte(pairsKB))
	e := pairsPool(1, nil)
	r, err := Select(base, []PoolEntry{e}, engine.DefaultConfig(), SelectOptions{Seeds: 3, ConfirmSeeds: 10})
	if err != nil {
		t.Fatal(err)
	}
	k, _ := kb.Load(r.File)
	if g := k.Inseparable(); len(g) != 0 {
		t.Errorf("the candidate leaves %v inseparable", g)
	}
	for _, s := range r.Tried {
		if s.Move.Drop == "side" || s.Move.Drop == "top" {
			t.Errorf("tried %s, which leaves two entities alike", s.Move)
		}
	}
}
