package design

import (
	"testing"

	"github.com/lpapez/ribice/engine"
	"github.com/lpapez/ribice/kb"
)

func TestReadingWords(t *testing.T) {
	// "Was there a ring?" is 4 words; yes and no are 1 + 2 each.
	if got := ReadingWords("Was there a ring?", []string{"yes", "no"}); got != 10 {
		t.Errorf("a short yes/no reads as %d words, want 10", got)
	}
	long := []string{"thin streaks or brush strokes, like combed hair",
		"a layer broken up into many repeated lumps, rolls or ripples"}
	if got := ReadingWords("Which of these was it most like?", long); got != 7+8+2+11+2 {
		t.Errorf("two long options read as %d words", got)
	}
}

// A game's reading counts each question asked with the options it offered,
// "something else" included, and the score charges for it.
func TestAnalyseCountsWordsRead(t *testing.T) {
	k := cloudKB(t)
	a := Analyse(k, engine.DefaultConfig(), 2)
	if a.Score.Words <= 0 {
		t.Fatalf("no words read: %+v", a.Score)
	}
	if want := 100*a.Score.Accuracy - WordWeight*a.Score.Words; a.Score.Value != want {
		t.Errorf("score %v, want %v", a.Score.Value, want)
	}

	// Recount one game by hand.
	g := engine.Simulate(k, engine.DefaultConfig(), engine.SimOptions{ErrorModel: true}).Results[0]
	words := 0
	for _, st := range g.Steps {
		attr := k.Attr(st.Attr)
		words += Words(attr.Title())
		for _, vals := range st.Offered {
			label := "something else"
			if len(vals) == 1 {
				label = attr.Label(vals[0])
			}
			words += Words(label) + OptionWords
		}
	}
	if words == 0 || words > int(a.Score.Words)*len(k.Entities)*2 {
		t.Errorf("first game read %d words", words)
	}
}

// Cost follows reading: a longer question with the same answer rate costs
// more, and one fewer can answer costs more again.
func TestCostFollowsReading(t *testing.T) {
	short := []Option{{"yes", "yes"}, {"no", "no"}}
	long := []Option{{"a", "thin streaks or brush strokes, like combed hair"},
		{"b", "a layer broken up into many repeated lumps, rolls or ripples"},
		{"c", "a puffy heap or tower, bulging on top"}}
	held := map[kb.Value]int{"yes": 1, "no": 1, "a": 1, "b": 1, "c": 1}
	none := func(string) []Verdict { return nil }
	cs := FitErrors("Was there a ring?", short, held, nil, none).Cost
	cl := FitErrors("Which of these was it most like?", long, held, nil, none).Cost
	if !near(cs, 10.0/UnitWords) || !near(cl, float64(ReadingWords("Which of these was it most like?", optionLabels(long)))/UnitWords) || cl <= 3*cs {
		t.Errorf("short question costs %v, long %v; want 10/12 and several times that", cs, cl)
	}
}

// A pool written before the cost formula changed carries a stale cost;
// ModelFor works it out afresh.
func TestModelForRecomputesCost(t *testing.T) {
	e := pairsPool(1, nil)
	e.Errors.Cost = 99
	m, guessed := ModelFor(e)
	want := float64(ReadingWords(e.Proposal.Question, optionLabels(e.Proposal.Values))) / UnitWords
	if guessed || !near(m.Cost, max(want, minCost)) {
		t.Errorf("cost %v (guessed %v), want %v", m.Cost, guessed, want)
	}
}
