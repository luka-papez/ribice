package design

import (
	"sort"

	"github.com/lpapez/ribice/engine"
	"github.com/lpapez/ribice/kb"
)

// Score is how well a knowledge base identifies its own entities when people
// answer the way its error model says, over several seeds.
type Score struct {
	Games     int     `json:"games"`
	Accuracy  float64 `json:"accuracy"` // share of games naming the right entity first
	Questions float64 `json:"questions"`
	Words     float64 `json:"words"`   // reading per game: every question asked and the options offered with it
	GaveUp    float64 `json:"gave_up"` // share of games that ran out of answerable questions
	Value     float64 `json:"score"`   // accuracy in percent less WordWeight per word read
}

// WordWeight is how many points of accuracy reading one word costs: 5 per
// UnitWords, so a short yes/no question costs what any question did when the
// score counted questions. The quiz is judged on the reading it takes, not the
// number of questions: eight long options cost as much as several yes/nos.
const WordWeight = 5.0 / UnitWords

// Analysis is the score, and the pairs the quiz mixes up.
type Analysis struct {
	KB     string `json:"kb"`
	Seeds  int    `json:"seeds"`
	Score  Score  `json:"score"`
	Pairs  []Pair `json:"pairs"`
	Missed []Miss `json:"missed"` // entities most often not named, however the game ended
}

// Miss is how often one entity was not named.
type Miss struct {
	Entity string  `json:"entity"`
	Rate   float64 `json:"rate"`
}

// Analyse plays one game per entity per seed with the knowledge base's own
// error model, and counts every game that named the wrong entity against the
// pair of them.
func Analyse(k *kb.KB, cfg engine.Config, seeds int) Analysis {
	a := Analysis{Seeds: seeds}
	mix := map[[2]string]int{}
	missed := map[string]int{}
	correct, asked, gaveUp, words := 0, 0, 0, 0
	for seed := 0; seed < seeds; seed++ {
		r := engine.Simulate(k, cfg, engine.SimOptions{ErrorModel: true, Seed: int64(seed)})
		for _, g := range r.Results {
			a.Score.Games++
			asked += g.Questions
			for _, st := range g.Steps {
				attr := k.Attr(st.Attr)
				words += ReadingWords(attr.Title(), offeredLabels(attr, st.Offered))
			}
			if g.GaveUp {
				gaveUp++
			}
			if g.Correct {
				correct++
				continue
			}
			missed[g.Target.Name]++
			if g.Guess != nil {
				mix[pairKey(g.Target.Name, g.Guess.Name)]++
			}
		}
	}
	n := float64(a.Score.Games)
	a.Score.Accuracy = float64(correct) / n
	a.Score.Questions = float64(asked) / n
	a.Score.GaveUp = float64(gaveUp) / n
	a.Score.Words = float64(words) / n
	a.Score.Value = 100*a.Score.Accuracy - WordWeight*a.Score.Words

	byName := map[string]*kb.Entity{}
	for _, e := range k.Entities {
		byName[e.Name] = e
	}
	for key, count := range mix {
		a.Pairs = append(a.Pairs, Pair{A: key[0], B: key[1], Mixups: count,
			SeparatedBy: separatedBy(k, byName[key[0]], byName[key[1]])})
	}
	sort.Slice(a.Pairs, func(i, j int) bool {
		if a.Pairs[i].Mixups != a.Pairs[j].Mixups {
			return a.Pairs[i].Mixups > a.Pairs[j].Mixups
		}
		return a.Pairs[i].A+a.Pairs[i].B < a.Pairs[j].A+a.Pairs[j].B
	})
	for _, e := range k.Entities {
		if m := missed[e.Name]; m > 0 {
			a.Missed = append(a.Missed, Miss{e.Name, float64(m) / float64(seeds)})
		}
	}
	sort.SliceStable(a.Missed, func(i, j int) bool { return a.Missed[i].Rate > a.Missed[j].Rate })
	return a
}

func pairKey(x, y string) [2]string {
	if y < x {
		x, y = y, x
	}
	return [2]string{x, y}
}

// separatedBy lists the attributes on which two entities can never give the
// same answer.
func separatedBy(k *kb.KB, x, y *kb.Entity) []string {
	out := []string{}
	for _, a := range k.Attributes {
		overlap := false
		for v := range x.Values(a.Name) {
			if y.Has(a.Name, v) {
				overlap = true
				break
			}
		}
		if !overlap {
			out = append(out, a.Name)
		}
	}
	return out
}
