package engine

import (
	"math/rand"
	"sort"

	"github.com/lpapez/ribice/kb"
)

// SimOptions controls a self-test run.
type SimOptions struct {
	Noise float64 // probability the simulated user answers wrongly

	// ErrorModel makes the simulated user answer the way each attribute's own
	// error model says people do: wrongly as often as Attribute.Report gives,
	// and not at all for a share 1 - AnswerRate of games. Noise is ignored.
	// This is what makes a question's measured error rates count when
	// comparing one knowledge base with another.
	ErrorModel bool

	Seed int64
}

// SimResult is one game played against a known entity, by the simulator or by
// replaying a recorded sighting.
type SimResult struct {
	Target    *kb.Entity
	Guess     *kb.Entity // nil when the leading candidate was "unknown"
	GuessName string
	Correct   bool
	Unsure    bool // the run ended with "not in this guide" leading
	Rank      int  // position of the true entity in the final ranking, 1-based
	Prob      float64
	Questions int
	Skipped   int     // questions answered "not sure", counted in Questions too
	GaveUp    bool    // the answerer stopped because only questions it could not answer were left
	Start     float64 // uncertainty before the first question, in bits
	Steps     []Step
}

// Step is one question put during a game and what came back.
type Step struct {
	Attr      string
	Offered   [][]kb.Value // the values each option covered, in the order shown
	Picked    []int        // indexes into Offered; empty when skipped
	Skipped   bool
	Gap       bool // skipped because the sighting had no answer recorded at all
	Reoffered bool
	Entropy   float64 // uncertainty left after the answer, in bits
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
	var results []SimResult
	for _, target := range k.Entities {
		answer := func(_ *Session, q *Question) reply {
			return reply{options: []int{answerAs(target, q, opts.Noise, rng)}}
		}
		if opts.ErrorModel {
			answer = modelAnswerer(target, rng)
		}
		results = append(results, play(k, cfg, target, answer))
	}
	return summarise(results)
}

// reply is what an answerer gives back for one question: the options it picks,
// none meaning "not sure".
type reply struct {
	options []int
	gap     bool // not sure because nothing was recorded, rather than by choice
	giveUp  bool // stop the game here instead of answering
}

// play runs one game against target, asking answer for every question.
func play(k *kb.KB, cfg Config, target *kb.Entity, answer func(*Session, *Question) reply) SimResult {
	s := New(k, cfg)
	res := SimResult{Target: target, Start: s.Entropy()}
	for {
		if done, _ := s.Done(); done {
			break
		}
		q := s.Next()
		if q == nil {
			break
		}
		r := answer(s, q)
		if r.giveUp {
			res.GaveUp = true
			break
		}
		if len(r.options) == 0 {
			s.Skip(q)
		} else {
			s.AskMany(q, r.options)
		}
		step := Step{Attr: q.Attr.Name, Picked: r.options, Skipped: len(r.options) == 0,
			Gap: r.gap && len(r.options) == 0, Reoffered: q.Reoffered, Entropy: s.Entropy()}
		for _, opt := range q.Options {
			step.Offered = append(step.Offered, opt.Values)
		}
		if step.Skipped {
			res.Skipped++
		}
		res.Steps = append(res.Steps, step)
	}
	res.Questions = s.Asked()

	ranking := s.Top(0)
	if len(ranking) > 0 {
		res.Guess, res.GuessName = ranking[0].Entity, ranking[0].Name
		res.Correct = res.Guess == target
		res.Unsure = ranking[0].Unknown
	}
	for i, c := range ranking {
		if c.Entity == target {
			res.Rank, res.Prob = i+1, c.Prob
			break
		}
	}
	return res
}

// summarise totals a set of games into a report.
func summarise(results []SimResult) SimReport {
	report := SimReport{Results: results}
	totalAsked := 0
	for _, res := range results {
		if res.Unsure {
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
	}
	if n := len(results); n > 0 {
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

// modelAnswerer answers as target, making the mistakes each attribute's error
// model predicts. Whether a question can be answered at all is settled the
// first time it is asked and holds for the rest of the game: someone who could
// not see the sun the first time cannot see it the second time either. As in
// Replay, the game ends once only such questions are left.
func modelAnswerer(target *kb.Entity, rng *rand.Rand) func(*Session, *Question) reply {
	answerable := map[string]bool{}
	return func(s *Session, q *Question) reply {
		a := q.Attr
		can, decided := answerable[a.Name]
		if !decided {
			can = rng.Float64() < a.AnswerRate
			answerable[a.Name] = can
		}
		if !can {
			if stuck(s, a.Name) {
				return reply{giveUp: true}
			}
			return reply{}
		}
		said := sampleReport(a, pickValue(target.Values(a.Name), rng), rng)
		return reply{options: optionsFor(q, []kb.Value{said})}
	}
}

// sampleReport draws the value a person names when the truth is actual, with
// the probabilities Attribute.Report gives.
func sampleReport(a *kb.Attribute, actual kb.Value, rng *rand.Rand) kb.Value {
	x := rng.Float64()
	for _, v := range a.Domain {
		x -= a.Report(v, actual)
		if x < 0 {
			return v
		}
	}
	return actual // rounding left a sliver past the last value
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
