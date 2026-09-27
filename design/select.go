package design

import (
	"fmt"
	"runtime"
	"slices"
	"sort"
	"strings"
	"sync"

	"github.com/lpapez/ribice/engine"
	"github.com/lpapez/ribice/kb"
)

// SelectOptions controls Select.
type SelectOptions struct {
	Seeds   int     // games per entity for each candidate; more is steadier and slower
	MinGain float64 // score points a move must add to be taken
	Workers int     // candidates scored at once; 0 means one per CPU

	// AllowDisputed lets a question with values not yet settled in, using
	// the proposer's values for them; the report lists them for a person.
	AllowDisputed bool
	// AllowGuessed lets a question in without perceive answers for every
	// value, with the knowledge base's default error rates; the report
	// marks it.
	AllowGuessed bool
}

// Move is one change to the question set: adding a proposed question,
// dropping a current one, or both at once for a question that replaces
// another.
type Move struct {
	Add  string `json:"add,omitempty"`
	Drop string `json:"drop,omitempty"`
}

func (m Move) String() string {
	switch {
	case m.Add != "" && m.Drop != "":
		return "replace " + m.Drop + " with " + m.Add
	case m.Add != "":
		return "add " + m.Add
	}
	return "drop " + m.Drop
}

// SelectStep is a move taken, and the score after it.
type SelectStep struct {
	Move  Move  `json:"move"`
	Score Score `json:"score"`
}

// SelectResult is what Select chose.
type SelectResult struct {
	Before  Score             `json:"before"`
	After   Score             `json:"after"`
	Steps   []SelectStep      `json:"steps"`
	Added   []string          `json:"added"`
	Dropped []string          `json:"dropped"`
	Guessed []string          `json:"guessed,omitempty"` // added with default error rates
	Pending map[string]int    `json:"pending,omitempty"` // added with this many values to settle
	Skipped map[string]string `json:"skipped,omitempty"` // proposals never tried, and why
	File    []byte            `json:"-"`                 // the candidate knowledge base
}

type candidate struct {
	entry   PoolEntry
	model   ErrorModel
	guessed bool
}

// Select starts from the current questions and takes, one at a time, the
// add, drop or replacement that raises the score most, until none raises it
// by MinGain. A move is never taken that would leave the candidate failing
// to load or -lint, identifying fewer entities than the current questions
// do, or identifying fewer with perfectly honest answers (-simulate): a
// question set that cannot separate two entities even when every answer is
// right has lost something the error model can hide. Every candidate is scored on the same seeds, so two differ
// only in their questions.
func Select(base *KBFile, pool []PoolEntry, cfg engine.Config, opts SelectOptions) (SelectResult, error) {
	res := SelectResult{Skipped: map[string]string{}, Pending: map[string]int{}}
	if opts.Seeds <= 0 {
		opts.Seeds = 10
	}
	workers := opts.Workers
	if workers <= 0 {
		workers = runtime.NumCPU()
	}

	cands := map[string]candidate{}
	var order []string
	for _, e := range pool {
		name := e.Proposal.Name
		c := candidate{entry: e, model: e.Errors}
		complete := len(e.Errors.Rows) > 0 && len(e.Errors.Missing) == 0
		pending := len(e.Disputed) + len(e.Abstained) + e.Unasked
		switch {
		case complete && e.Errors.AnswerRate < minAnswerRate:
			res.Skipped[name] = fmt.Sprintf("only %.0f%% could answer", 100*e.Errors.AnswerRate)
			continue
		case float64(e.NotObservable) > maxNotObservable*float64(len(e.Proposal.Assign)):
			res.Skipped[name] = fmt.Sprintf("not observable for %d", e.NotObservable)
			continue
		case !complete && !opts.AllowGuessed:
			res.Skipped[name] = "no perceive answers for every value"
			continue
		case pending > 0 && !opts.AllowDisputed:
			res.Skipped[name] = fmt.Sprintf("%d values to settle", pending)
			continue
		}
		if !complete {
			c.model, c.guessed = guessedModel(e.Proposal), true
		}
		cands[name] = c
		order = append(order, name)
	}

	baseKB, err := kb.Load(base.Bytes())
	if err != nil {
		return res, err
	}
	var current []string
	for _, a := range baseKB.Attributes {
		current = append(current, a.Name)
	}

	var honestFloor int

	// A state is which current questions are dropped and which proposed ones
	// added, in the order they were.
	type state struct {
		dropped map[string]bool
		added   []string
	}
	build := func(s state) (*KBFile, *kb.KB, error) {
		f := base.Clone()
		for name := range s.dropped {
			f.DropAttribute(name)
		}
		for _, name := range s.added {
			c := cands[name]
			if err := f.AddAttribute(c.entry.Proposal, c.model); err != nil {
				return nil, nil, err
			}
		}
		k, err := kb.Load(f.Bytes())
		if err != nil {
			return nil, nil, err
		}
		for _, is := range k.Lint() {
			if is.Severity == kb.Error {
				return nil, nil, fmt.Errorf("lint: %s", is.Message)
			}
		}
		if honest := engine.Simulate(k, cfg, engine.SimOptions{}).Correct; honest < honestFloor {
			return nil, nil, fmt.Errorf("identifies %d with honest answers, fewer than %d", honest, honestFloor)
		}
		return f, k, nil
	}
	present := func(s state) map[string]bool {
		p := map[string]bool{}
		for _, n := range current {
			if !s.dropped[n] {
				p[n] = true
			}
		}
		for _, n := range s.added {
			p[n] = true
		}
		return p
	}
	apply := func(s state, m Move) state {
		t := state{dropped: map[string]bool{}}
		for n := range s.dropped {
			t.dropped[n] = true
		}
		t.added = append([]string(nil), s.added...)
		if m.Drop != "" {
			if slices.Contains(t.added, m.Drop) {
				t.added = slices.DeleteFunc(t.added, func(n string) bool { return n == m.Drop })
			} else {
				t.dropped[m.Drop] = true
			}
		}
		if m.Add != "" {
			t.added = append(t.added, m.Add)
		}
		return t
	}

	honestFloor = engine.Simulate(baseKB, cfg, engine.SimOptions{}).Correct
	now := state{dropped: map[string]bool{}}
	_, k0, err := build(now)
	if err != nil {
		return res, fmt.Errorf("the current knowledge base: %w", err)
	}
	res.Before = Analyse(k0, cfg, opts.Seeds).Score
	best := res.Before
	floor := res.Before.Accuracy

	for {
		p := present(now)
		var moves []Move
		for _, name := range order {
			if p[name] {
				continue
			}
			c := cands[name]
			switch r := c.entry.Proposal.Replaces; {
			case r != "" && p[r]:
				moves = append(moves, Move{Add: name, Drop: r})
			case r == "" || !p[r]:
				moves = append(moves, Move{Add: name})
			}
		}
		for _, n := range current {
			if p[n] {
				moves = append(moves, Move{Drop: n})
			}
		}
		for _, n := range now.added {
			moves = append(moves, Move{Drop: n})
		}

		scores := make([]Score, len(moves))
		ok := make([]bool, len(moves))
		var wg sync.WaitGroup
		jobs := make(chan int)
		for w := 0; w < workers; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for i := range jobs {
					if _, k, err := build(apply(now, moves[i])); err == nil {
						scores[i], ok[i] = Analyse(k, cfg, opts.Seeds).Score, true
					}
				}
			}()
		}
		for i := range moves {
			jobs <- i
		}
		close(jobs)
		wg.Wait()

		pick := -1
		for i := range moves {
			if !ok[i] || scores[i].Accuracy < floor {
				continue
			}
			if pick < 0 || scores[i].Value > scores[pick].Value {
				pick = i
			}
		}
		if pick < 0 || scores[pick].Value < best.Value+opts.MinGain {
			break
		}
		now = apply(now, moves[pick])
		best = scores[pick]
		res.Steps = append(res.Steps, SelectStep{Move: moves[pick], Score: best})
	}

	f, _, err := build(now)
	if err != nil {
		return res, err
	}
	res.File, res.After = f.Bytes(), best
	res.Added = append(res.Added, now.added...)
	for n := range now.dropped {
		res.Dropped = append(res.Dropped, n)
	}
	sort.Strings(res.Dropped)
	for _, n := range now.added {
		c := cands[n]
		if c.guessed {
			res.Guessed = append(res.Guessed, n)
		}
		if pending := len(c.entry.Disputed) + len(c.entry.Abstained) + c.entry.Unasked; pending > 0 {
			res.Pending[n] = pending
		}
	}
	return res, nil
}

// guessedModel is the knowledge base's default error model, for a question
// with no perceive answers yet: kb.DefaultNoise, and kb.DefaultConfusion
// over the look-alikes the proposer named. A yes/no question has nothing to
// single out, as in FitErrors, so all its error is noise.
func guessedModel(p Proposal) ErrorModel {
	m := ErrorModel{Noise: kb.DefaultNoise, AnswerRate: 1, Cost: 1}
	if len(p.Values) <= 2 {
		return m
	}
	for _, c := range p.Confusable {
		if len(c) == 2 {
			m.Confusable = append(m.Confusable, [2]kb.Value{c[0], c[1]})
		}
	}
	if len(m.Confusable) > 0 {
		m.Confusion = kb.DefaultConfusion
	}
	return m
}

// SelectReport describes a selection for the person who reviews it.
func SelectReport(r SelectResult, pool []PoolEntry) string {
	var b strings.Builder
	line := func(s Score) string {
		return fmt.Sprintf("identified %.0f%%, %.1f questions, gave up %.0f%%: score %.1f",
			100*s.Accuracy, s.Questions, 100*s.GaveUp, s.Value)
	}
	fmt.Fprintf(&b, "Before: %s  \nAfter: %s\n\n", line(r.Before), line(r.After))
	b.WriteString("Score: percent identified minus 5 per question, in games answered with each question's error rates.\n\n")
	if len(r.Steps) > 0 {
		b.WriteString("## Moves, in the order taken\n\n| move | identified | questions | score |\n| --- | --- | --- | --- |\n")
		for _, s := range r.Steps {
			fmt.Fprintf(&b, "| %s | %.0f%% | %.1f | %.1f |\n", s.Move, 100*s.Score.Accuracy, s.Score.Questions, s.Score.Value)
		}
	} else {
		b.WriteString("No move raised the score; the candidate is the current knowledge base.\n")
	}

	byName := map[string]PoolEntry{}
	for _, e := range pool {
		byName[e.Proposal.Name] = e
	}
	if len(r.Added) > 0 {
		b.WriteString("\n## Questions added\n\n")
		for _, n := range r.Added {
			e := byName[n]
			fmt.Fprintf(&b, "- **%s**: %s", n, e.Proposal.Question)
			if e.Proposal.Replaces != "" {
				fmt.Fprintf(&b, " (replaces %s)", e.Proposal.Replaces)
			}
			b.WriteString("\n")
		}
	}
	if len(r.Guessed) > 0 {
		fmt.Fprintf(&b, "\nWith default error rates, for want of perceive answers: %s.\n", strings.Join(r.Guessed, ", "))
	}
	if len(r.Pending) > 0 {
		b.WriteString("\n## Values for a person to settle\n\nThese kept the proposer's value in the candidate. Settle them with `ribice-design consult -tasks settle.jsonl -expert human`.\n\n")
		for _, n := range r.Added {
			e := byName[n]
			if r.Pending[n] == 0 {
				continue
			}
			var who []string
			for _, ent := range append(append([]string(nil), e.Disputed...), e.Abstained...) {
				var vs []string
				for _, v := range e.Proposal.Assign[ent] {
					vs = append(vs, string(v))
				}
				who = append(who, fmt.Sprintf("%s (%s)", ent, strings.Join(vs, "/")))
			}
			fmt.Fprintf(&b, "- **%s**: %s\n", n, strings.Join(who, "; "))
		}
	}
	if len(r.Skipped) > 0 {
		b.WriteString("\n## Proposals never tried\n\n")
		var names []string
		for n := range r.Skipped {
			names = append(names, n)
		}
		sort.Strings(names)
		for _, n := range names {
			fmt.Fprintf(&b, "- %s: %s\n", n, r.Skipped[n])
		}
	}
	return b.String()
}
