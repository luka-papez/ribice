package design

import (
	"sort"
	"strings"

	"github.com/lpapez/ribice/kb"
)

// Status is how an assign task was settled.
type Status string

const (
	Agreed    Status = "agreed"    // the experts back the proposed value
	Disputed  Status = "disputed"  // they lean elsewhere; a person decides
	Abstained Status = "abstained" // most of them could not answer at all
	Unasked   Status = "unasked"   // no verdict yet
)

// Settlement is what the experts made of one entity's proposed value.
type Settlement struct {
	Task     Task
	Proposed []kb.Value // empty when the proposer did not know
	Status   Status
	Value    []kb.Value // what the entity is settled to hold, when Agreed

	// Combined is the experts' p averaged, one vote per expert; Abstain is
	// their abstentions averaged the same way. When a person has answered,
	// only people's verdicts count.
	Combined map[kb.Value]float64
	Abstain  float64
	Reasons  map[Reason]int // how many experts abstained for each reason
	Experts  []string       // whose verdicts counted
	ByPerson bool
}

// Settle compares each assign task's verdicts with the value proposed for its
// entity.
//
// A value is agreed when the proposed values together hold more than half of
// the combined answer and one of them is its top option; an even split backs
// nothing. When the proposer did not know, the experts' top option is agreed
// if it holds more than half the answer on its own. It is abstained when at
// least half the mass is abstention, and disputed otherwise. A person's
// verdict outweighs any model's: once one exists, only people count. That is
// how a dispute is settled, by asking a person the same task, blind.
func Settle(tasks []Task, proposed func(Task) []kb.Value, verdicts func(task string) []Verdict) []Settlement {
	var out []Settlement
	for _, t := range tasks {
		if t.Kind != Assign {
			continue
		}
		s := Settlement{Task: t, Proposed: proposed(t), Reasons: map[Reason]int{}}
		vs := verdicts(t.ID)
		var people []Verdict
		for _, v := range vs {
			if strings.HasPrefix(v.Expert, "human:") {
				people = append(people, v)
			}
		}
		if len(people) > 0 {
			vs, s.ByPerson = people, true
		}
		if len(vs) == 0 {
			s.Status = Unasked
			out = append(out, s)
			continue
		}

		s.Combined = map[kb.Value]float64{}
		for _, v := range vs {
			s.Experts = append(s.Experts, v.Expert)
			for _, o := range t.Options {
				s.Combined[o.Value] += v.P[o.Value] / float64(len(vs))
			}
			s.Abstain += v.Abstain / float64(len(vs))
			if v.Abstain > 0 {
				s.Reasons[v.Reason]++
			}
		}

		held := 0.0
		for _, v := range s.Proposed {
			held += s.Combined[v]
		}
		top := s.Top()
		switch {
		case s.Abstain >= 0.5:
			s.Status = Abstained
		case len(s.Proposed) == 0 && s.Combined[top] > 0.5:
			s.Status, s.Value = Agreed, []kb.Value{top}
		case held > 0.5 && contains(s.Proposed, top):
			s.Status, s.Value = Agreed, s.Proposed
		default:
			s.Status = Disputed
		}
		out = append(out, s)
	}
	return out
}

// Top is the option the experts gave most weight, first in the task's order
// on a tie.
func (s Settlement) Top() kb.Value {
	var best kb.Value
	for _, o := range s.Task.Options {
		if best == "" || s.Combined[o.Value] > s.Combined[best] {
			best = o.Value
		}
	}
	return best
}

// MainReason is the reason most experts abstained for.
func (s Settlement) MainReason() Reason {
	var best Reason
	for _, r := range Reasons {
		if s.Reasons[r] > s.Reasons[best] {
			best = r
		}
	}
	return best
}

func contains(vs []kb.Value, want kb.Value) bool {
	for _, v := range vs {
		if v == want {
			return true
		}
	}
	return false
}

// KBValues proposes, for tasks made by FromKB, the values the knowledge base
// already holds: the current answer key, vetted like any proposal.
func KBValues(k *kb.KB) func(Task) []kb.Value {
	byName := map[string]*kb.Entity{}
	for _, e := range k.Entities {
		byName[e.Name] = e
	}
	return func(t Task) []kb.Value {
		e := byName[t.Subject.Entity]
		if e == nil {
			return nil
		}
		vs := append([]kb.Value(nil), e.ValueList(t.Attribute)...)
		sort.Slice(vs, func(i, j int) bool { return vs[i] < vs[j] })
		return vs
	}
}
