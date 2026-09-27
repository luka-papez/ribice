package design

import (
	"fmt"
	"strings"

	"github.com/lpapez/ribice/kb"
)

// PoolEntry is one proposed question after consultation: every entity's
// settled value, how people answer it, and whether Select may use it.
type PoolEntry struct {
	Proposal Proposal `json:"proposal"` // Assign holds the settled values

	Agreed        int        `json:"agreed"`
	Corrected     []string   `json:"corrected,omitempty"` // entities a person gave another value than the proposer's
	Disputed      []string   `json:"disputed,omitempty"`  // entities whose value the experts dispute
	Abstained     []string   `json:"abstained,omitempty"`
	Unasked       int        `json:"unasked,omitempty"`
	NotObservable int        `json:"not_observable"` // entities most experts abstained on as not observable
	Errors        ErrorModel `json:"errors"`

	Ready bool     `json:"ready"`
	Why   []string `json:"why,omitempty"` // why it is not ready
}

// maxNotObservable is the share of entities an attribute may be unobservable
// for before it is dropped.
const maxNotObservable = 0.25

// minAnswerRate is the least share of people who must be able to answer.
const minAnswerRate = 0.5

// BuildPool settles each proposal's values from its assign verdicts and fits
// its error model from its perceive verdicts. A value the experts back, or
// fill in where the proposer did not know, is settled; a disputed or
// abstained one keeps the proposer's value but holds the question back
// until a person settles it. A question is also held back when too few
// people could answer it, or it is unobservable for too many entities.
func BuildPool(props []Proposal, assign, perceive []Task, verdicts func(string) []Verdict) []PoolEntry {
	byAttr := func(tasks []Task, name string) []Task {
		var out []Task
		for _, t := range tasks {
			if t.Attribute == name {
				out = append(out, t)
			}
		}
		return out
	}

	var pool []PoolEntry
	for _, p := range props {
		e := PoolEntry{Proposal: p}
		e.Proposal.Assign = map[string][]kb.Value{}
		for name, vs := range p.Assign {
			e.Proposal.Assign[name] = vs
		}

		settled := Settle(byAttr(assign, p.Name), ProposedValues([]Proposal{p}), verdicts)
		for _, s := range settled {
			name := s.Task.Subject.Entity
			switch s.Status {
			case Agreed:
				e.Agreed++
				e.Proposal.Assign[name] = s.Value
			case Corrected:
				e.Corrected = append(e.Corrected, name)
				e.Proposal.Assign[name] = s.Value
			case Disputed:
				e.Disputed = append(e.Disputed, name)
			case Abstained:
				e.Abstained = append(e.Abstained, name)
				if s.MainReason() == NotObservable {
					e.NotObservable++
				}
			case Unasked:
				e.Unasked++
			}
		}

		holders := map[kb.Value]int{}
		for _, vs := range e.Proposal.Assign {
			for _, v := range vs {
				holders[v]++
			}
		}
		e.Errors = FitErrors(p.Values, holders, byAttr(perceive, p.Name), verdicts)

		entities := len(p.Assign)
		switch {
		case len(settled) == 0:
			e.Why = append(e.Why, "no assign tasks")
		case e.Unasked > 0:
			e.Why = append(e.Why, fmt.Sprintf("%d values not asked yet", e.Unasked))
		}
		if n := len(e.Disputed) + len(e.Abstained); n > 0 {
			e.Why = append(e.Why, fmt.Sprintf("%d values for a person to settle", n))
		}
		if entities > 0 && float64(e.NotObservable) > maxNotObservable*float64(entities) {
			e.Why = append(e.Why, fmt.Sprintf("not observable for %d of %d", e.NotObservable, entities))
		}
		if len(e.Errors.Rows) == 0 || len(e.Errors.Missing) > 0 {
			e.Why = append(e.Why, "not every value has perceive answers")
		} else if e.Errors.AnswerRate < minAnswerRate {
			e.Why = append(e.Why, fmt.Sprintf("only %.0f%% could answer", 100*e.Errors.AnswerRate))
		}
		e.Ready = len(e.Why) == 0
		pool = append(pool, e)
	}
	return pool
}

// PoolReport is the pool as a markdown table, one row per question.
func PoolReport(pool []PoolEntry) string {
	var b strings.Builder
	b.WriteString("| question | agreed | corrected | to settle | answer rate | noise | confusion | look-alikes | ready |\n")
	b.WriteString("| --- | --- | --- | --- | --- | --- | --- | --- | --- |\n")
	for _, e := range pool {
		var pairs []string
		for _, c := range e.Errors.Confusable {
			pairs = append(pairs, string(c[0])+"~"+string(c[1]))
		}
		ready := "yes"
		if !e.Ready {
			ready = "no: " + strings.Join(e.Why, "; ")
		}
		fmt.Fprintf(&b, "| %s %s | %d | %d | %d | %.0f%% | %.2f | %.2f | %s | %s |\n",
			e.Proposal.ID, e.Proposal.Name, e.Agreed, len(e.Corrected), len(e.Disputed)+len(e.Abstained),
			100*e.Errors.AnswerRate, e.Errors.Noise, e.Errors.Confusion, strings.Join(pairs, ", "), ready)
	}
	return b.String()
}
