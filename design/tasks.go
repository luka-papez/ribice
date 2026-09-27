package design

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"

	"github.com/lpapez/ribice/kb"
)

// FromKB makes tasks of one kind for the attributes a knowledge base already
// has, so the current questions are vetted on the same terms as proposals.
// Each task's Attribute is the attribute's name.
//
//   - Assign: one task per entity and attribute, told the entity's name and
//     note.
//   - Perceive: one task per attribute and value, told that value's label as
//     what is really there.
func FromKB(k *kb.KB, kind Kind) ([]Task, error) {
	var out []Task
	switch kind {
	case Assign:
		for _, e := range k.Entities {
			for _, a := range k.Attributes {
				s := Subject{Entity: e.Name, Text: e.Note}
				out = append(out, NewTask(Assign, a.Name, s, a.Title(), options(a), 2))
			}
		}
	case Perceive:
		for _, a := range k.Attributes {
			for _, v := range a.Domain {
				s := Subject{Value: v, Text: a.Label(v)}
				out = append(out, NewTask(Perceive, a.Name, s, a.Title(), options(a), 2))
			}
		}
	default:
		return nil, fmt.Errorf("cannot make %q tasks from a knowledge base alone", kind)
	}
	return out, nil
}

// ProposalSet is what proposals.json holds.
type ProposalSet struct {
	Proposer   string     `json:"proposer"`
	KB         string     `json:"kb"`
	Attributes []Proposal `json:"attributes"`
}

// FromProposals makes tasks of one kind for proposed attributes, the same way
// FromKB does for current ones. Each task's Attribute is the proposal's name.
// Assign tasks never carry the proposer's values: the expert answers blind.
func FromProposals(k *kb.KB, props []Proposal, kind Kind) ([]Task, error) {
	var out []Task
	switch kind {
	case Assign:
		for _, e := range k.Entities {
			for _, p := range props {
				s := Subject{Entity: e.Name, Text: e.Note}
				out = append(out, NewTask(Assign, p.Name, s, p.Question, p.Values, 2))
			}
		}
	case Perceive:
		for _, p := range props {
			for _, o := range p.Values {
				s := Subject{Value: o.Value, Text: o.Label}
				out = append(out, NewTask(Perceive, p.Name, s, p.Question, p.Values, 2))
			}
		}
	default:
		return nil, fmt.Errorf("cannot make %q tasks from proposals alone", kind)
	}
	return out, nil
}

// ProposedValues gives, for assign tasks made by FromProposals, the values
// the proposer gave, for Settle to compare the experts' answers with.
func ProposedValues(props []Proposal) func(Task) []kb.Value {
	byName := map[string]Proposal{}
	for _, p := range props {
		byName[p.Name] = p
	}
	return func(t Task) []kb.Value { return byName[t.Attribute].Assign[t.Subject.Entity] }
}

// options lists every value of a in the knowledge base's order, with the
// labels the quiz shows.
func options(a *kb.Attribute) []Option {
	out := make([]Option, len(a.Domain))
	for i, v := range a.Domain {
		out[i] = Option{Value: v, Label: a.Label(v)}
	}
	return out
}

// WriteTasks writes tasks as JSON Lines.
func WriteTasks(w io.Writer, tasks []Task) error {
	enc := json.NewEncoder(w)
	for _, t := range tasks {
		if err := enc.Encode(t); err != nil {
			return err
		}
	}
	return nil
}

// ReadTasks reads tasks written by WriteTasks, checking that each id still
// matches what the task shows: a hand-edited task must be asked afresh, not
// matched with verdicts given to its old wording.
func ReadTasks(r io.Reader) ([]Task, error) {
	var out []Task
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for n := 1; sc.Scan(); n++ {
		if len(sc.Bytes()) == 0 {
			continue
		}
		var t Task
		if err := json.Unmarshal(sc.Bytes(), &t); err != nil {
			return nil, fmt.Errorf("task %d: %w", n, err)
		}
		if len(t.Options) < 2 {
			return nil, fmt.Errorf("task %d (%s): %d options", n, t.ID, len(t.Options))
		}
		if want := NewTask(t.Kind, t.Attribute, t.Subject, t.Question, t.Options, t.MaxChoices).ID; t.ID != want {
			return nil, fmt.Errorf("task %d: id %s does not match its content (%s); was it edited?", n, t.ID, want)
		}
		out = append(out, t)
	}
	return out, sc.Err()
}
