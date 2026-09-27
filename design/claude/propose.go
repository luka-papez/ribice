package claude

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
	"text/template"

	"github.com/lpapez/ribice/design"
	"github.com/lpapez/ribice/kb"
)

// ProposeVersion names the proposer's prompt; change it with the prompt.
// v2 (2026-09-27): judged on reading, short questions and options, at most
// five options, where v1 allowed eight.
const ProposeVersion = "propose-v2"

// Proposer is Claude inventing questions.
type Proposer struct {
	Client *Client
	Model  string
	Effort string // "high" by default: this is the one call that designs
	Domain string
	Log    io.Writer
}

func (p *Proposer) ID() string { return "claude:" + p.Model + "@" + ProposeVersion }

// Propose asks for new questions and keeps those that pass validate; the rest
// are reported to Log and dropped, so one bad question does not waste the call.
func (p *Proposer) Propose(ctx context.Context, b design.Brief) ([]design.Proposal, error) {
	system, err := p.system()
	if err != nil {
		return nil, err
	}
	effort := p.Effort
	if effort == "" {
		effort = "high"
	}
	req := Request{System: system, Model: p.Model, Effort: effort, Schema: proposeSchema,
		Content: []Block{TextBlock(BriefText(b))}}
	reply, err := p.Client.Call(ctx, req)
	if err != nil {
		return nil, err
	}
	var r proposeReply
	if err := json.Unmarshal(reply, &r); err != nil {
		return nil, err
	}

	var out []design.Proposal
	names := map[string]bool{}
	for i, a := range r.Attributes {
		prop, err := convert(a, b.KB, p.ID())
		if err == nil && names[prop.Name] {
			err = fmt.Errorf("a second question named %q", prop.Name)
		}
		if err != nil {
			p.logf("  dropped proposal %d (%s): %v\n", i+1, a.Name, err)
			continue
		}
		names[prop.Name] = true
		prop.ID = fmt.Sprintf("p%02d", len(out)+1)
		out = append(out, prop)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("none of %d proposals was usable", len(r.Attributes))
	}
	return out, nil
}

func (p *Proposer) logf(format string, args ...any) {
	if p.Log != nil {
		fmt.Fprintf(p.Log, format, args...)
	}
}

func (p *Proposer) system() (string, error) {
	t, err := template.ParseFS(promptFiles, "prompts/"+ProposeVersion+".txt")
	if err != nil {
		return "", err
	}
	domain := p.Domain
	if domain == "" {
		domain = "the things in a field guide"
	}
	var s strings.Builder
	if err := t.Execute(&s, map[string]string{"Domain": domain}); err != nil {
		return "", err
	}
	return strings.TrimSpace(s.String()), nil
}

// BriefText is everything the proposer is told, as one user turn.
func BriefText(b design.Brief) string {
	k := b.KB
	var s strings.Builder
	fmt.Fprintf(&s, "# The guide: %s, %d entries\n\n", k.Name, len(k.Entities))
	for _, e := range k.Entities {
		fmt.Fprintf(&s, "- %s", e.Name)
		if e.Note != "" {
			fmt.Fprintf(&s, ": %s", e.Note)
		}
		s.WriteString("\n")
	}

	s.WriteString("\n# The questions it asks now\n\n")
	for _, a := range k.Attributes {
		fmt.Fprintf(&s, "## %s (%s)\n%s\n", a.Name, a.Kind, a.Title())
		for _, v := range a.Domain {
			fmt.Fprintf(&s, "- %s: %s\n", v, a.Label(v))
		}
		s.WriteString("\n")
	}

	s.WriteString("# Every entry's answers now\n\nentry")
	for _, a := range k.Attributes {
		fmt.Fprintf(&s, " | %s", a.Name)
	}
	s.WriteString("\n")
	for _, e := range k.Entities {
		s.WriteString(e.Name)
		for _, a := range k.Attributes {
			var vs []string
			for _, v := range e.ValueList(a.Name) {
				vs = append(vs, string(v))
			}
			fmt.Fprintf(&s, " | %s", strings.Join(vs, "/"))
		}
		s.WriteString("\n")
	}

	if len(b.Targets) > 0 {
		s.WriteString("\n# Pairs the quiz mixes up\n\nFrom simulated games where people answer with each question's own error rates.\n\n")
		for _, pr := range b.Targets {
			fmt.Fprintf(&s, "- %s / %s: %d mix-ups; answers never overlap on: %s\n",
				pr.A, pr.B, pr.Mixups, strings.Join(pr.SeparatedBy, ", "))
		}
	}
	if b.Evidence != "" {
		s.WriteString("\n# Evidence on the current questions\n\n")
		s.WriteString(b.Evidence)
		s.WriteString("\n")
	}
	return s.String()
}

var proposeSchema = json.RawMessage(`{
  "type": "object",
  "properties": {"attributes": {"type": "array", "items": {
    "type": "object",
    "properties": {
      "name": {"type": "string"},
      "replaces": {"type": ["string", "null"]},
      "kind": {"type": "string", "enum": ["categorical", "boolean"]},
      "question": {"type": "string"},
      "values": {"type": "array", "items": {"type": "object",
        "properties": {"value": {"type": "string"}, "label": {"type": "string"}},
        "required": ["value", "label"], "additionalProperties": false}},
      "confusable": {"type": "array", "items": {"type": "array", "items": {"type": "string"}}},
      "targets": {"type": "array", "items": {"type": "array", "items": {"type": "string"}}},
      "rationale": {"type": "string"},
      "assign": {"type": "array", "items": {"type": "object",
        "properties": {"entity": {"type": "string"}, "values": {"type": "array", "items": {"type": "string"}}},
        "required": ["entity", "values"], "additionalProperties": false}}
    },
    "required": ["name", "replaces", "kind", "question", "values", "confusable", "targets", "rationale", "assign"],
    "additionalProperties": false}}},
  "required": ["attributes"],
  "additionalProperties": false}`)

type proposeReply struct {
	Attributes []proposedAttr `json:"attributes"`
}

type proposedAttr struct {
	Name     string  `json:"name"`
	Replaces *string `json:"replaces"`
	Kind     string  `json:"kind"`
	Question string  `json:"question"`
	Values   []struct {
		Value string `json:"value"`
		Label string `json:"label"`
	} `json:"values"`
	Confusable [][]string `json:"confusable"`
	Targets    [][]string `json:"targets"`
	Rationale  string     `json:"rationale"`
	Assign     []struct {
		Entity string   `json:"entity"`
		Values []string `json:"values"`
	} `json:"assign"`
}

var snake = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// convert checks one proposed attribute against the knowledge base and turns
// it into a Proposal. Every entity must be assigned exactly once, and only
// values the attribute declares; a target naming an unknown entity is
// dropped rather than failing the question.
func convert(a proposedAttr, k *kb.KB, proposer string) (design.Proposal, error) {
	p := design.Proposal{Proposer: proposer, Name: strings.TrimSpace(a.Name), Kind: a.Kind,
		Question: strings.TrimSpace(a.Question), Rationale: a.Rationale, Assign: map[string][]kb.Value{}}
	if !snake.MatchString(p.Name) {
		return p, fmt.Errorf("name %q is not snake_case", p.Name)
	}
	if a.Replaces != nil && *a.Replaces != "" {
		if k.Attr(*a.Replaces) == nil {
			return p, fmt.Errorf("replaces %q, which is not a current question", *a.Replaces)
		}
		p.Replaces = *a.Replaces
	}
	if k.Attr(p.Name) != nil && p.Replaces == "" {
		p.Replaces = p.Name // reusing a current name is rewriting that question
	}
	if p.Question == "" {
		return p, fmt.Errorf("no question")
	}

	declared := map[kb.Value]bool{}
	for _, v := range a.Values {
		val := kb.Value(strings.ToLower(strings.TrimSpace(v.Value)))
		if !snake.MatchString(string(val)) || declared[val] {
			return p, fmt.Errorf("value %q is not a new snake_case id", v.Value)
		}
		declared[val] = true
		p.Values = append(p.Values, design.Option{Value: val, Label: strings.TrimSpace(v.Label)})
	}
	switch {
	case a.Kind == "boolean" && !(len(p.Values) == 2 && declared[kb.Yes] && declared[kb.No]):
		return p, fmt.Errorf("a boolean question needs exactly the values yes and no")
	case a.Kind == "categorical" && (len(p.Values) < 2 || len(p.Values) > design.MaxOptions):
		return p, fmt.Errorf("%d values, want 2 to %d", len(p.Values), design.MaxOptions)
	case a.Kind != "boolean" && a.Kind != "categorical":
		return p, fmt.Errorf("kind %q", a.Kind)
	}

	for _, pair := range a.Confusable {
		var group []kb.Value
		for _, v := range pair {
			val := kb.Value(v)
			if !declared[val] {
				return p, fmt.Errorf("look-alike %q is not one of its values", v)
			}
			group = append(group, val)
		}
		p.Confusable = append(p.Confusable, group)
	}

	known := map[string]bool{}
	for _, e := range k.Entities {
		known[e.Name] = true
	}
	for _, as := range a.Assign {
		if !known[as.Entity] {
			return p, fmt.Errorf("assigns %q, which is not in the guide", as.Entity)
		}
		if _, twice := p.Assign[as.Entity]; twice {
			return p, fmt.Errorf("assigns %q twice", as.Entity)
		}
		if len(as.Values) > 2 {
			return p, fmt.Errorf("%s: %d values, at most 2", as.Entity, len(as.Values))
		}
		vals := []kb.Value{}
		for _, v := range as.Values {
			val := kb.Value(strings.ToLower(strings.TrimSpace(v)))
			if !declared[val] {
				return p, fmt.Errorf("%s: %q is not one of its values", as.Entity, v)
			}
			vals = append(vals, val)
		}
		p.Assign[as.Entity] = vals
	}
	var missing []string
	for _, e := range k.Entities {
		if _, ok := p.Assign[e.Name]; !ok {
			missing = append(missing, e.Name)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return p, fmt.Errorf("no value for %s", strings.Join(missing, ", "))
	}

	for _, t := range a.Targets {
		if len(t) == 2 && known[t[0]] && known[t[1]] {
			p.Targets = append(p.Targets, [2]string{t[0], t[1]})
		}
	}
	return p, nil
}
