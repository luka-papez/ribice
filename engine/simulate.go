package engine

import (
	"math/rand"
	"sort"

	"github.com/lpapez/ribice/kb"
)

// SimOptions controls a self-test run.
type SimOptions struct {
	Noise float64 // probability the simulated user answers wrongly
	Seed  int64
}

// SimResult is one simulated run against a known entity.
type SimResult struct {
	Target    *kb.Entity
	Guess     *kb.Entity // nil when the leading candidate was "unknown"
	GuessName string
	Correct   bool
	Unsure    bool // the run ended with "not in this guide" leading
	Rank      int  // position of the true entity in the final ranking, 1-based
	Prob      float64
	Questions int
}

// SimReport aggregates a full self-test.
type SimReport struct {
	Results []SimResult
	Correct int
	// Unsure counts runs that ended by saying nothing matched. For a target
	// that is in the knowledge base those are misses, but they are the harmless
	// kind: the alternative is naming the wrong species with confidence.
	Unsure    int
	MeanAsked float64
	MaxAsked  int
	Worst     []SimResult // runs where the true entity did not come first
}

// Simulate plays the quiz once per entity, answering as that entity truthfully
// (or with the configured probability of error). It is the quickest way to tell
// whether a knowledge base actually separates the things in it.
func Simulate(k *kb.KB, cfg Config, opts SimOptions) SimReport {
	rng := rand.New(rand.NewSource(opts.Seed))
	report := SimReport{}
	totalAsked := 0

	for _, target := range k.Entities {
		s := New(k, cfg)
		for {
			if done, _ := s.Done(); done {
				break
			}
			q := s.Next()
			if q == nil {
				break
			}
			s.Ask(q, answerAs(target, q, opts.Noise, rng))
		}

		ranking := s.Top(0)
		res := SimResult{Target: target, Questions: s.Asked()}
		if len(ranking) > 0 {
			res.Guess, res.GuessName = ranking[0].Entity, ranking[0].Name
			res.Correct = res.Guess == target
		}
		for i, c := range ranking {
			if c.Entity == target {
				res.Rank, res.Prob = i+1, c.Prob
				break
			}
		}
		if len(ranking) > 0 && ranking[0].Unknown {
			res.Unsure = true
			report.Unsure++
		}
		if res.Correct {
			report.Correct++
		} else {
			report.Worst = append(report.Worst, res)
		}
		totalAsked += res.Questions
		if res.Questions > report.MaxAsked {
			report.MaxAsked = res.Questions
		}
		report.Results = append(report.Results, res)
	}

	if n := len(report.Results); n > 0 {
		report.MeanAsked = float64(totalAsked) / float64(n)
	}
	sort.SliceStable(report.Worst, func(i, j int) bool {
		return report.Worst[i].Prob < report.Worst[j].Prob
	})
	return report
}

// answerAs picks the option a perfectly honest sighting of target would choose,
// then gets it wrong with probability noise. A wrong answer prefers a declared
// look-alike of the truth, since that is how people actually err; it falls back
// to any other option when the attribute declares no look-alikes.
func answerAs(target *kb.Entity, q *Question, noise float64, rng *rand.Rand) int {
	truth := pickValue(target.Values(q.Attr.Name), rng)
	honest := 0
	for i, opt := range q.Options {
		for _, v := range opt.Values {
			if v == truth {
				honest = i
			}
		}
	}
	if noise <= 0 || len(q.Options) < 2 || rng.Float64() >= noise {
		return honest
	}

	var lookalikes []int
	for i, opt := range q.Options {
		if i == honest {
			continue
		}
		for _, v := range opt.Values {
			if contains(q.Attr.Confusable[truth], v) {
				lookalikes = append(lookalikes, i)
				break
			}
		}
	}
	if len(lookalikes) > 0 {
		return lookalikes[rng.Intn(len(lookalikes))]
	}

	wrong := rng.Intn(len(q.Options) - 1)
	if wrong >= honest {
		wrong++
	}
	return wrong
}

func contains(vs []kb.Value, want kb.Value) bool {
	for _, v := range vs {
		if v == want {
			return true
		}
	}
	return false
}

func pickValue(set map[kb.Value]bool, rng *rand.Rand) kb.Value {
	vals := make([]kb.Value, 0, len(set))
	for v := range set {
		vals = append(vals, v)
	}
	if len(vals) == 0 {
		return kb.Absent
	}
	sort.Slice(vals, func(i, j int) bool { return vals[i] < vals[j] })
	return vals[rng.Intn(len(vals))]
}
