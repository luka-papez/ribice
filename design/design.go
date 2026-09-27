// Package design finds better questions for a knowledge base: a proposer
// invents attributes, experts vet them, and the engine keeps the set that
// identifies entities in the fewest questions. See specs/question-design.md.
//
// Every consultation of an expert is a closed choice: a subject, one question
// and the options the proposer offered. The expert answers with a probability
// for each option and for abstaining, and nothing else, so an expert can be a
// language model, a person at a terminal or a classifier alike.
package design

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"

	"github.com/lpapez/ribice/kb"
)

// Kind is what a task asks, and so what its answer means.
type Kind string

const (
	// Assign asks which option an entity shows: its value in the knowledge base.
	Assign Kind = "assign"
	// Perceive gives the true value and asks what someone untrained would
	// answer: one row of the question's mix-up table.
	Perceive Kind = "perceive"
	// Observe asks which option a photo shows: the same row, measured.
	Observe Kind = "observe"
)

// Reason is why an expert abstained.
type Reason string

const (
	NotObservable   Reason = "not_observable"   // cannot be seen or known this way
	Ambiguous       Reason = "ambiguous"        // two options fit and neither better
	UnclearQuestion Reason = "unclear_question" // the wording is the problem
	Unknown         Reason = "unknown"          // the expert does not know
)

// Reasons lists every reason, in the order they are offered.
var Reasons = []Reason{NotObservable, Ambiguous, UnclearQuestion, Unknown}

// Option is one answer a task offers.
type Option struct {
	Value kb.Value `json:"value"`
	Label string   `json:"label"`
}

// Subject is what a task is about, in every form the harness has. An expert
// reads the forms it understands and ignores the rest.
type Subject struct {
	Entity string   `json:"entity,omitempty"` // assign: whose value is asked
	Value  kb.Value `json:"value,omitempty"`  // perceive: the true value
	Text   string   `json:"text,omitempty"`   // what a reader is told
	Photos []string `json:"photos,omitempty"` // what a viewer is shown
}

// Task is one closed-choice question put to an expert.
type Task struct {
	ID         string   `json:"id"`
	Kind       Kind     `json:"kind"`
	Attribute  string   `json:"attribute"`
	Subject    Subject  `json:"subject"`
	Question   string   `json:"question"`
	Options    []Option `json:"options"`
	MaxChoices int      `json:"max_choices"` // for experts that pick; a classifier may spread P over all options
}

// NewTask fills in the task's id: a hash of everything the expert is shown,
// so a reworded question is asked again and an unchanged one never is.
func NewTask(kind Kind, attribute string, subject Subject, question string, options []Option, maxChoices int) Task {
	t := Task{Kind: kind, Attribute: attribute, Subject: subject, Question: question,
		Options: options, MaxChoices: maxChoices}
	b, err := json.Marshal(t) // ID is still empty, so it does not hash itself
	if err != nil {
		panic(err) // plain strings and slices always marshal
	}
	sum := sha256.Sum256(b)
	t.ID = hex.EncodeToString(sum[:8])
	return t
}

// Verdict is one expert's answer to one task.
type Verdict struct {
	Task    string               `json:"task"`
	Expert  string               `json:"expert"`
	P       map[kb.Value]float64 `json:"p"`
	Abstain float64              `json:"abstain"`
	Reason  Reason               `json:"reason,omitempty"` // set exactly when Abstain > 0
}

// tolerance is how far P and Abstain may sum from one, for rounding.
const tolerance = 1e-6

// Check reports whether v is a well-formed answer to t: probabilities for t's
// options only, none negative, summing with Abstain to one, and a reason
// exactly when abstaining.
func (v Verdict) Check(t Task) error {
	if v.Task != t.ID {
		return fmt.Errorf("verdict for task %s checked against task %s", v.Task, t.ID)
	}
	offered := map[kb.Value]bool{}
	for _, o := range t.Options {
		offered[o.Value] = true
	}
	sum := v.Abstain
	for val, p := range v.P {
		if !offered[val] {
			return fmt.Errorf("task %s: %q is not one of its options", t.ID, val)
		}
		if p < 0 || math.IsNaN(p) {
			return fmt.Errorf("task %s: probability %v for %q", t.ID, p, val)
		}
		sum += p
	}
	if v.Abstain < 0 || v.Abstain > 1 {
		return fmt.Errorf("task %s: abstain %v is not between 0 and 1", t.ID, v.Abstain)
	}
	if math.Abs(sum-1) > tolerance {
		return fmt.Errorf("task %s: probabilities sum to %v, not 1", t.ID, sum)
	}
	switch {
	case v.Abstain > 0 && !validReason(v.Reason):
		return fmt.Errorf("task %s: abstains with reason %q", t.ID, v.Reason)
	case v.Abstain == 0 && v.Reason != "":
		return fmt.Errorf("task %s: gives reason %q without abstaining", t.ID, v.Reason)
	}
	return nil
}

func validReason(r Reason) bool {
	for _, x := range Reasons {
		if r == x {
			return true
		}
	}
	return false
}

// Confidence is how sure an expert that picks options says it is.
type Confidence string

const (
	Low    Confidence = "low"
	Medium Confidence = "medium"
	High   Confidence = "high"
)

// confidenceMass is the probability the picked options share at each
// confidence; the rest is spread evenly over the options not picked.
var confidenceMass = map[Confidence]float64{High: 1, Medium: 0.8, Low: 0.5}

// Picks turns chosen options into probabilities, the same way for every
// expert that picks, so a person and a model answering alike give the same
// verdict. picks are indices into options.
func Picks(options []Option, picks []int, c Confidence) (map[kb.Value]float64, error) {
	mass, ok := confidenceMass[c]
	if !ok {
		return nil, fmt.Errorf("confidence %q is not low, medium or high", c)
	}
	if len(picks) == 0 {
		return nil, fmt.Errorf("no option picked")
	}
	picked := map[int]bool{}
	for _, i := range picks {
		if i < 0 || i >= len(options) {
			return nil, fmt.Errorf("option %d of %d", i+1, len(options))
		}
		picked[i] = true
	}
	rest := len(options) - len(picked)
	if rest == 0 {
		mass = 1 // nothing else to spread the doubt over
	}
	p := map[kb.Value]float64{}
	for i, o := range options {
		if picked[i] {
			p[o.Value] = mass / float64(len(picked))
		} else {
			p[o.Value] = (1 - mass) / float64(rest)
		}
	}
	return p, nil
}

// Abstention is the verdict of an expert that cannot answer at all.
func Abstention(task, expert string, r Reason) Verdict {
	return Verdict{Task: task, Expert: expert, P: map[kb.Value]float64{}, Abstain: 1, Reason: r}
}

// Expert answers tasks.
type Expert interface {
	// ID names the expert and the version of its instructions, such as
	// "claude:claude-opus-5@v1a" or "human:luka". Two experts never share one.
	ID() string

	// Accepts reports whether the expert can answer t at all: a text model
	// cannot look at a photo, a classifier cannot read a description.
	Accepts(t Task) bool

	// Answer answers tasks, calling emit once per verdict as soon as it is
	// ready, so an interrupted run keeps everything answered so far. A task
	// it did not get to is left out, to be asked on the next run; stopping
	// early that way is not an error.
	Answer(ctx context.Context, tasks []Task, emit func(Verdict) error) error
}

// Proposer invents attributes. It is the only component that writes free
// text: questions, values and labels.
type Proposer interface {
	Propose(ctx context.Context, b Brief) ([]Proposal, error)
}

// Brief is everything a proposer is told.
type Brief struct {
	KB       *kb.KB
	Targets  []Pair
	Evidence string // what photos showed about the current questions, in prose
}

// Pair is two entities the quiz fails to tell apart.
type Pair struct {
	A           string   `json:"a"`
	B           string   `json:"b"`
	Mixups      int      `json:"mixups"`       // simulated games of one that ended on the other
	SeparatedBy []string `json:"separated_by"` // current attributes on which they differ
}

// Proposal is one candidate attribute, with every entity's value.
type Proposal struct {
	ID         string                `json:"id"`
	Proposer   string                `json:"proposer"`
	Name       string                `json:"name"`
	Replaces   string                `json:"replaces,omitempty"`
	Kind       string                `json:"kind"` // "categorical" or "boolean"
	Question   string                `json:"question"`
	Values     []Option              `json:"values"`
	Confusable [][]kb.Value          `json:"confusable,omitempty"`
	Targets    [][2]string           `json:"targets,omitempty"`
	Rationale  string                `json:"rationale,omitempty"`
	Assign     map[string][]kb.Value `json:"assign"` // entity name to its values; empty when unknown
}
