package engine

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/lpapez/ribice/kb"
)

// Sighting is one recorded look at a known entity -- a photograph, say -- and
// the answer someone gave to each question about it. Replaying sightings shows
// how the quiz does against real answers rather than the knowledge base's own.
type Sighting struct {
	ID     string
	Target *kb.Entity
	Split  string // free-form group, such as "tune" or "heldout"

	// Answers is keyed by attribute name. An attribute missing here was never
	// asked, which replays as a skip and is reported as a gap.
	Answers map[string]Response
}

// Response is the recorded answer to one question.
type Response struct {
	Values   []kb.Value // what they said: one value, or two when torn between them
	CantTell string     // why they could not say; set exactly when Values is empty
}

type rawSighting struct {
	Photo   string                 `json:"photo"`
	Target  string                 `json:"target"`
	Split   string                 `json:"split"`
	Answers map[string]rawResponse `json:"answers"`
}

type rawResponse struct {
	Values     []string `json:"values"`
	CantTell   string   `json:"cant_tell"`
	Confidence string   `json:"confidence"` // for the harness's report; the engine does not weigh it
}

// LoadSightings reads sightings as JSON objects, one after another (JSON
// Lines), checking every name and value against the knowledge base. A value
// the attribute cannot take is an error rather than a skip: it means the
// answers were recorded against a different version of the knowledge base.
func LoadSightings(r io.Reader, k *kb.KB) ([]Sighting, error) {
	byName := map[string]*kb.Entity{}
	for _, e := range k.Entities {
		byName[e.Name] = e
	}
	seen := map[string]bool{}

	var out []Sighting
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	for n := 1; ; n++ {
		var raw rawSighting
		if err := dec.Decode(&raw); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return nil, fmt.Errorf("sighting %d: %w", n, err)
		}
		if raw.Photo == "" {
			return nil, fmt.Errorf("sighting %d: missing \"photo\"", n)
		}
		where := fmt.Sprintf("sighting %q", raw.Photo)
		if seen[raw.Photo] {
			return nil, fmt.Errorf("%s: appears twice", where)
		}
		seen[raw.Photo] = true
		target := byName[raw.Target]
		if target == nil {
			return nil, fmt.Errorf("%s: target %q is not in the knowledge base", where, raw.Target)
		}

		s := Sighting{ID: raw.Photo, Target: target, Split: raw.Split, Answers: map[string]Response{}}
		for name, rr := range raw.Answers {
			a := k.Attr(name)
			if a == nil {
				return nil, fmt.Errorf("%s: no attribute %q in the knowledge base", where, name)
			}
			switch {
			case len(rr.Values) > 0 && rr.CantTell != "":
				return nil, fmt.Errorf("%s.%s: has both values and cant_tell", where, name)
			case len(rr.Values) == 0 && rr.CantTell == "":
				return nil, fmt.Errorf("%s.%s: has neither values nor cant_tell", where, name)
			}
			var resp Response
			resp.CantTell = rr.CantTell
			for _, raw := range rr.Values {
				v, ok := a.Parse(raw)
				if !ok {
					return nil, fmt.Errorf("%s.%s: %q is not one of %s", where, name, raw, domain(a))
				}
				resp.Values = append(resp.Values, v)
			}
			s.Answers[a.Name] = resp
		}
		out = append(out, s)
	}
	return out, nil
}

func domain(a *kb.Attribute) string {
	vs := make([]string, len(a.Domain))
	for i, v := range a.Domain {
		vs[i] = string(v)
	}
	return strings.Join(vs, ", ")
}

// Replay plays one game per sighting, answering every question the way that
// sighting recorded. Results[i] is the game for sightings[i].
//
// A question the sighting cannot answer is skipped, exactly as a person would
// skip it. When the engine comes back to such a question and nothing has been
// answered since it was last skipped, only unanswerable questions are left,
// and the game ends there as if the person had given up.
func Replay(k *kb.KB, cfg Config, sightings []Sighting) SimReport {
	results := make([]SimResult, 0, len(sightings))
	for _, sg := range sightings {
		results = append(results, play(k, cfg, sg.Target, func(s *Session, q *Question) reply {
			resp, recorded := sg.Answers[q.Attr.Name]
			if recorded && len(resp.Values) > 0 {
				return reply{options: optionsFor(q, resp.Values)}
			}
			if stuck(s, q.Attr.Name) {
				return reply{giveUp: true}
			}
			return reply{gap: !recorded}
		}))
	}
	return summarise(results)
}

// optionsFor finds the options covering the given values. A value pooled into
// "something else" picks that option, which is what a person whose answer is
// not listed would do; two values in the same option pick it once.
func optionsFor(q *Question, values []kb.Value) []int {
	var out []int
	for _, v := range values {
		for i, opt := range q.Options {
			if contains(opt.Values, v) && !containsInt(out, i) {
				out = append(out, i)
			}
		}
	}
	return out
}

func containsInt(xs []int, want int) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}

// stuck reports whether attr was skipped and every answer since was a skip
// too, so offering it again can only go round in a circle.
func stuck(s *Session, attr string) bool {
	h := s.History()
	for i := len(h) - 1; i >= 0; i-- {
		if !h[i].Skipped {
			return false
		}
		if h[i].Attr == attr {
			return true
		}
	}
	return false
}
