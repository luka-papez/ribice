package design

import (
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/lpapez/ribice/kb"
)

// Status is how an assign task was settled.
type Status string

const (
	Agreed    Status = "agreed"    // the experts back the proposed value
	Corrected Status = "corrected" // a person answered, and not with the proposed value
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
// least half the mass is abstention, and disputed otherwise.
//
// A person's verdict outweighs any model's: once one exists, only people
// count, and what they answered is the value. That is how a dispute is
// settled, by asking a person the same task, blind. When they name another
// value than the proposer's, it is Corrected rather than Agreed, so reports
// show how often people overrule the proposer.
func Settle(tasks []Task, proposed func(Task) []kb.Value, verdicts func(task string) []Verdict) []Settlement {
	var out []Settlement
	for _, t := range tasks {
		if t.Kind != Assign {
			continue
		}
		s := Settlement{Task: t, Proposed: proposed(t)}
		c, ok := Combine(t, verdicts(t.ID))
		if !ok {
			s.Status = Unasked
			out = append(out, s)
			continue
		}
		s.Combined, s.Abstain, s.Reasons, s.Experts, s.ByPerson = c.P, c.Abstain, c.Reasons, c.Experts, c.ByPerson

		held := 0.0
		for _, v := range s.Proposed {
			held += s.Combined[v]
		}
		top := s.Top()
		switch {
		case s.Abstain >= 0.5:
			s.Status = Abstained
		case s.ByPerson:
			picks, mass := s.tops()
			switch {
			case mass <= 0.5:
				s.Status = Disputed
			case sameValues(picks, s.Proposed):
				s.Status, s.Value = Agreed, s.Proposed
			default:
				s.Status, s.Value = Corrected, picks
			}
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

// Combined is several experts' verdicts on one task, as one.
type Combined struct {
	P        map[kb.Value]float64 // averaged, one vote per expert
	Abstain  float64
	Reasons  map[Reason]int // how many experts abstained for each reason
	Experts  []string       // whose verdicts counted
	ByPerson bool           // only people's verdicts counted
}

// Combine averages verdicts on t, one vote per expert. A person's verdict
// outweighs any model's: once one exists, only people count. An expert that
// answered under several versions of its instructions counts once, with its
// latest (see Latest). It reports false when there is no verdict at all.
func Combine(t Task, vs []Verdict) (Combined, bool) {
	c := Combined{P: map[kb.Value]float64{}, Reasons: map[Reason]int{}}
	vs = Latest(vs)
	var people []Verdict
	for _, v := range vs {
		if strings.HasPrefix(v.Expert, "human:") {
			people = append(people, v)
		}
	}
	if len(people) > 0 {
		vs, c.ByPerson = people, true
	}
	if len(vs) == 0 {
		return c, false
	}
	for _, v := range vs {
		c.Experts = append(c.Experts, v.Expert)
		for _, o := range t.Options {
			c.P[o.Value] += v.P[o.Value] / float64(len(vs))
		}
		c.Abstain += v.Abstain / float64(len(vs))
		if v.Abstain > 0 {
			c.Reasons[v.Reason]++
		}
	}
	return c, true
}

// versioned reads an expert id of the form "backend:model@v<N><wording>",
// such as "claude:claude-opus-5@v2a", into the expert apart from its version
// ("claude:claude-opus-5@a") and the version (2). Other ids, such as a
// person's, have no version.
var versioned = regexp.MustCompile(`^(.*@)v(\d+)([a-z]*)$`)

// Latest keeps, of verdicts by the same expert under different versions of
// its instructions, only the latest version's: a revised prompt is a
// correction of the old one, not a second opinion. Verdicts are returned in
// the order given.
func Latest(vs []Verdict) []Verdict {
	newest := map[string]int{}
	for _, v := range vs {
		if m := versioned.FindStringSubmatch(v.Expert); m != nil {
			n, _ := strconv.Atoi(m[2])
			if key := m[1] + m[3]; n > newest[key] {
				newest[key] = n
			}
		}
	}
	var out []Verdict
	for _, v := range vs {
		if m := versioned.FindStringSubmatch(v.Expert); m != nil {
			if n, _ := strconv.Atoi(m[2]); n < newest[m[1]+m[3]] {
				continue
			}
		}
		out = append(out, v)
	}
	return out
}

// tops is every option tied for the most weight, and their weight together:
// both of a person's two picks, say.
func (s Settlement) tops() ([]kb.Value, float64) {
	best := s.Combined[s.Top()]
	var picks []kb.Value
	mass := 0.0
	for _, o := range s.Task.Options {
		if p := s.Combined[o.Value]; p > 0 && best-p < 1e-9 {
			picks = append(picks, o.Value)
			mass += p
		}
	}
	return picks, mass
}

func sameValues(a, b []kb.Value) bool {
	if len(a) != len(b) {
		return false
	}
	for _, v := range a {
		if !contains(b, v) {
			return false
		}
	}
	return true
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
