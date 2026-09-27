package design

import (
	"github.com/lpapez/ribice/kb"
)

// Row is what people answer about one question when the truth is one value.
type Row struct {
	Truth   kb.Value             `json:"truth"`
	Answers map[kb.Value]float64 `json:"answers"` // among those who answer; sums to 1 unless nobody does
	Abstain float64              `json:"abstain"` // share who cannot answer at all
	Weight  int                  `json:"weight"`  // entities holding the true value
	Experts []string             `json:"experts"`
}

// ErrorModel is how one question is answered, in the terms the engine uses
// (kb.Attribute's Noise, Confusion, Confusable and AnswerRate), with the
// mix-up table it was fitted from.
type ErrorModel struct {
	Noise      float64       `json:"noise"`
	Confusion  float64       `json:"confusion"`
	Confusable [][2]kb.Value `json:"confusable,omitempty"`
	AnswerRate float64       `json:"answer_rate"`
	Cost       float64       `json:"cost"`
	Rows       []Row         `json:"rows"`
	Missing    []kb.Value    `json:"missing,omitempty"` // values some entity holds but no perceive verdict covers
}

const (
	// lookalikeShare is how often one value must be answered for another,
	// in either direction, for the two to be declared look-alikes. Perceive
	// answers count ten people, so this is two of them: one alone is too
	// easily one imagined outlier.
	lookalikeShare = 0.20
	minNoise       = 0.02
	maxError       = 0.95 // noise plus confusion; the engine needs some chance of a right answer
	minCost        = 0.7
	maxCost        = 3.0
)

// FitErrors fits the engine's error model to the perceive verdicts on one
// question. values are its options in order, holders how many entities hold
// each, and tasks its perceive tasks, one per true value.
//
// The engine knows only values some entity holds, so an answer naming a
// value nobody holds ("nothing tall enough in view to judge by") counts as
// not answering, which is how the quiz would take it. Each true value's row
// counts as often as entities hold it, times the share who answer, since
// that is how often a game meets it.
//
//   - Confusable: pairs where either is answered for the other at least 20%
//     of the time. With only two values held, there is nothing to single
//     out, and every mistake is noise.
//   - Confusion: the share of answers on a declared look-alike, over true
//     values that have look-alikes, the only ones it applies to in
//     kb.Attribute.Report.
//   - Noise: the share on any other wrong value, floored at 0.02.
//   - AnswerRate: one less the abstentions; Cost is its inverse, clamped to
//     0.7-3, so a question few can answer is asked late.
func FitErrors(values []Option, holders map[kb.Value]int, tasks []Task, verdicts func(string) []Verdict) ErrorModel {
	var held []kb.Value
	for _, o := range values {
		if holders[o.Value] > 0 {
			held = append(held, o.Value)
		}
	}
	isHeld := map[kb.Value]bool{}
	for _, v := range held {
		isHeld[v] = true
	}

	m := ErrorModel{AnswerRate: 1, Cost: 1}
	byTruth := map[kb.Value]Row{}
	for _, t := range tasks {
		truth := t.Subject.Value
		if t.Kind != Perceive || !isHeld[truth] {
			continue
		}
		c, ok := Combine(t, verdicts(t.ID))
		if !ok {
			continue
		}
		row := Row{Truth: truth, Answers: map[kb.Value]float64{}, Abstain: c.Abstain,
			Weight: holders[truth], Experts: c.Experts}
		answered := 0.0
		for v, p := range c.P {
			if isHeld[v] {
				answered += p
			} else {
				row.Abstain += p
			}
		}
		if answered > 0 {
			for _, v := range held {
				row.Answers[v] = c.P[v] / answered
			}
		}
		byTruth[truth] = row
	}
	for _, v := range held {
		row, ok := byTruth[v]
		if !ok {
			m.Missing = append(m.Missing, v)
			continue
		}
		m.Rows = append(m.Rows, row)
	}
	if len(m.Rows) == 0 {
		return m
	}

	// How often a game meets each row, and how often it meets it answered.
	total, abstain := 0.0, 0.0
	for _, r := range m.Rows {
		total += float64(r.Weight)
		abstain += float64(r.Weight) * r.Abstain
	}
	m.AnswerRate = 1 - abstain/total
	m.Cost = clamp(1/max(m.AnswerRate, 1e-9), minCost, maxCost)

	near := map[kb.Value][]kb.Value{}
	if len(held) > 2 {
		for i, a := range held {
			for _, b := range held[i+1:] {
				if byTruth[a].Answers[b] >= lookalikeShare || byTruth[b].Answers[a] >= lookalikeShare {
					m.Confusable = append(m.Confusable, [2]kb.Value{a, b})
					near[a] = append(near[a], b)
					near[b] = append(near[b], a)
				}
			}
		}
	}

	var farSum, farW, nearSum, nearW float64
	for _, r := range m.Rows {
		if len(r.Answers) == 0 {
			continue // nobody answered: it counts in AnswerRate only
		}
		w := float64(r.Weight) * (1 - r.Abstain)
		wrong := 1 - r.Answers[r.Truth]
		onNear := 0.0
		for _, v := range near[r.Truth] {
			onNear += r.Answers[v]
		}
		farSum += w * (wrong - onNear)
		farW += w
		if len(near[r.Truth]) > 0 {
			nearSum += w * onNear
			nearW += w
		}
	}
	if farW > 0 {
		m.Noise = farSum / farW
	}
	if nearW > 0 {
		m.Confusion = nearSum / nearW
	}
	m.Noise = max(m.Noise, minNoise)
	if sum := m.Noise + m.Confusion; sum > maxError {
		m.Noise *= maxError / sum
		m.Confusion *= maxError / sum
	}
	return m
}

func clamp(x, lo, hi float64) float64 {
	return min(max(x, lo), hi)
}
