// Package kb loads and normalises a knowledge base: a set of entities, each
// described by an arbitrary bag of properties.
//
// Nothing in this package is specific to any domain -- fish, mushrooms and
// aircraft all load the same way.
//
// Closed-world assumption: a property an entity does not declare is treated as
// not present (the value "no" for boolean properties, "none" otherwise).
package kb

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
)

// Value is a canonical attribute value: trimmed and lower-cased.
type Value string

// Distinguished values. Yes/No are the domain of boolean attributes; Absent is
// what a categorical attribute holds when the entity does not declare it.
const (
	Yes    Value = "yes"
	No     Value = "no"
	Absent Value = "none"
)

// DefaultNoise is the probability that a user misremembers an attribute and
// names something unrelated, used for attributes that do not declare their own.
const DefaultNoise = 0.12

// DefaultConfusion is the probability mass that leaks to look-alike values, for
// attributes that list "confusable" groups without naming their own rate.
const DefaultConfusion = 0.25

// Kind distinguishes yes/no attributes from multiple-choice ones.
type Kind int

const (
	Categorical Kind = iota
	Boolean
)

func (k Kind) String() string {
	if k == Boolean {
		return "boolean"
	}
	return "categorical"
}

// Attribute is one property that entities may hold, plus the presentation and
// noise metadata the engine needs to turn it into a question.
type Attribute struct {
	Name     string
	Kind     Kind
	Domain   []Value // every value the attribute can take, including Absent
	Question string  // human phrasing; auto-generated when the KB omits it
	Noise    float64 // P(user gives the wrong answer to this question)
	NoiseSet bool    // true when the KB declared Noise explicitly
	Cost     float64 // relative effort of asking; questions are ranked by gain/cost

	// Multi marks an attribute an entity can genuinely hold several of at once
	// -- where a fish lives, say, as opposed to what shape it is. It changes
	// what picking several answers means: see engine.Session.answerLikelihood.
	Multi bool

	// Confusion is the probability the user names a look-alike value instead of
	// the true one, and Confusable lists which values look alike. Together they
	// replace the assumption that a mistake is equally likely to be any other
	// value: see Report.
	Confusion  float64
	Confusable map[Value][]Value

	labels            map[Value]string
	holders           map[Value]int // how many entities hold each value
	unknownConfusable []string      // declared look-alikes that are not real values
}

// Report is P(the user answers "reported" | the true value is "actual").
//
// A mistake is not uniform. Told that a fish is torpedo-shaped, people commonly
// say "elongated" and practically never say "seahorse", so the attribute splits
// its error mass in two: Confusion goes to the values declared to look like the
// true one, and Noise to everything else. An attribute that declares no
// look-alikes keeps the old uniform behaviour.
//
// Whatever the true value, these sum to one across the domain.
func (a *Attribute) Report(reported, actual Value) float64 {
	d := len(a.Domain)
	if d <= 1 {
		return 1
	}
	near := a.Confusable[actual]
	noise, confusion := a.Noise, a.Confusion
	far := d - 1 - len(near)
	if len(near) == 0 {
		confusion = 0
	}
	if far <= 0 {
		noise = 0 // nowhere for it to go; the look-alikes take everything
	}

	if reported == actual {
		return 1 - noise - confusion
	}
	for _, v := range near {
		if v == reported {
			return confusion / float64(len(near))
		}
	}
	if far <= 0 {
		return 0
	}
	return noise / float64(far)
}

// Label renders a value for display.
func (a *Attribute) Label(v Value) string {
	if s, ok := a.labels[v]; ok && s != "" {
		return s
	}
	if a.Kind == Boolean {
		switch v {
		case Yes:
			return "yes"
		case No:
			return "no"
		}
	}
	if v == Absent {
		return "none of these / not present"
	}
	return prettify(string(v))
}

// Title is the question text shown to the user.
func (a *Attribute) Title() string {
	if a.Question != "" {
		return a.Question
	}
	return upperFirst(prettify(a.Name)) + "?"
}

// Holders reports how many entities hold the given value.
func (a *Attribute) Holders(v Value) int { return a.holders[v] }

// Image is a canonical picture of an entity, with what is needed to display it
// lawfully: most licences require naming the author and the licence itself.
type Image struct {
	URL     string // the picture
	Source  string // page it came from, where the licence is stated in full
	Credit  string // author, as the source names them
	License string // e.g. "CC BY-SA 4.0"
}

// Entity is one identifiable thing -- a fish species, say.
type Entity struct {
	Name  string
	Prior float64 // relative frequency; normalised across the KB
	Note  string  // free text shown with the result
	Link  string  // somewhere to read more
	Image *Image  // nil when the knowledge base has no picture

	values map[string]map[Value]bool
}

// Values returns the set of values this entity may present for an attribute.
// A set with more than one member means "any of these", e.g. a fish that looks
// either grey or silver. Every attribute in the KB is present in the map.
func (e *Entity) Values(attr string) map[Value]bool { return e.values[attr] }

// Has reports whether the entity may present v for attr.
func (e *Entity) Has(attr string, v Value) bool { return e.values[attr][v] }

// KB is a loaded knowledge base.
type KB struct {
	Name       string
	Entities   []*Entity
	Attributes []*Attribute

	byAttr map[string]*Attribute
}

// Attr looks up an attribute by name.
func (k *KB) Attr(name string) *Attribute { return k.byAttr[name] }

// --- loading ---------------------------------------------------------------

type rawFile struct {
	Name       string             `json:"name"`
	Attributes map[string]rawAttr `json:"attributes"`
	Entities   []map[string]any   `json:"entities"`
}

type rawAttr struct {
	Question   string            `json:"question"`
	Labels     map[string]string `json:"labels"`
	Noise      *float64          `json:"noise"`
	Cost       *float64          `json:"cost"`
	Multi      bool              `json:"multi"`
	Confusion  *float64          `json:"confusion"`
	Confusable [][]string        `json:"confusable"`
}

// LoadFile reads a knowledge base from a JSON file.
func LoadFile(path string) (*KB, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	k, err := Load(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return k, nil
}

// Load parses a knowledge base. Two shapes are accepted: a bare JSON array of
// entities, or an object with optional "name"/"attributes" metadata and an
// "entities" array.
func Load(data []byte) (*KB, error) {
	var f rawFile
	if trimmed := bytes.TrimLeft(data, " \t\r\n"); len(trimmed) > 0 && trimmed[0] == '[' {
		if err := json.Unmarshal(data, &f.Entities); err != nil {
			return nil, fmt.Errorf("parsing entity list: %w", err)
		}
	} else {
		if err := json.Unmarshal(data, &f); err != nil {
			return nil, fmt.Errorf("parsing knowledge base: %w", err)
		}
	}
	if len(f.Entities) == 0 {
		return nil, errors.New("knowledge base contains no entities")
	}

	// Pass 1: canonicalise every declared value and observe each attribute.
	type observation struct {
		values   map[Value]int
		boolish  int
		declared int
	}
	obs := map[string]*observation{}
	declared := make([]map[string][]Value, len(f.Entities))
	kbase := &KB{Name: f.Name, byAttr: map[string]*Attribute{}}

	for i, raw := range f.Entities {
		name, _ := raw["name"].(string)
		name = strings.TrimSpace(name)
		if name == "" {
			return nil, fmt.Errorf("entity %d: missing or empty \"name\"", i)
		}
		ent := &Entity{Name: name, Prior: 1, values: map[string]map[Value]bool{}}
		if p, ok := raw["_prior"].(float64); ok {
			if p < 0 {
				return nil, fmt.Errorf("%s: _prior must not be negative", name)
			}
			ent.Prior = p
		}
		if n, ok := raw["_note"].(string); ok {
			ent.Note = n
		}
		if l, ok := raw["_link"].(string); ok {
			ent.Link = strings.TrimSpace(l)
		}
		if img, ok := raw["_image"].(map[string]any); ok {
			ent.Image = &Image{
				URL:     metaString(img, "url"),
				Source:  metaString(img, "source"),
				Credit:  metaString(img, "credit"),
				License: metaString(img, "license"),
			}
			if ent.Image.URL == "" {
				return nil, fmt.Errorf("%s: _image has no \"url\"", name)
			}
		}
		kbase.Entities = append(kbase.Entities, ent)

		declared[i] = map[string][]Value{}
		for key, val := range raw {
			if key == "name" || strings.HasPrefix(key, "_") {
				continue
			}
			attr := canonAttrName(key)
			if attr == "" {
				return nil, fmt.Errorf("%s: empty property name", name)
			}
			vs, boolish, err := canonValues(val)
			if err != nil {
				return nil, fmt.Errorf("%s.%s: %w", name, attr, err)
			}
			if _, dup := declared[i][attr]; dup {
				return nil, fmt.Errorf("%s: property %q declared twice", name, attr)
			}
			declared[i][attr] = vs

			o := obs[attr]
			if o == nil {
				o = &observation{values: map[Value]int{}}
				obs[attr] = o
			}
			o.declared++
			if boolish {
				o.boolish++
			}
			for _, v := range vs {
				o.values[v]++
			}
		}
	}

	// Pass 2: decide each attribute's kind and domain.
	for name, o := range obs {
		a := &Attribute{Name: name, Noise: DefaultNoise, Cost: 1, labels: map[Value]string{}, holders: map[Value]int{}}
		if o.boolish > 0 && onlyBoolish(o.values) {
			a.Kind = Boolean
			a.Domain = []Value{Yes, No}
		} else {
			a.Kind = Categorical
			for v := range o.values {
				if v != Absent {
					a.Domain = append(a.Domain, v)
				}
			}
			sort.Slice(a.Domain, func(x, y int) bool { return a.Domain[x] < a.Domain[y] })
			if o.values[Absent] > 0 || o.declared < len(f.Entities) {
				a.Domain = append(a.Domain, Absent)
			}
		}
		kbase.Attributes = append(kbase.Attributes, a)
		kbase.byAttr[name] = a
	}
	sort.Slice(kbase.Attributes, func(i, j int) bool {
		return kbase.Attributes[i].Name < kbase.Attributes[j].Name
	})

	// Pass 3: materialise every entity's value set for every attribute, filling
	// in the closed-world default where the entity said nothing.
	for i, ent := range kbase.Entities {
		for _, a := range kbase.Attributes {
			set := map[Value]bool{}
			vs, ok := declared[i][a.Name]
			if !ok {
				vs = []Value{Absent}
			}
			for _, v := range vs {
				if a.Kind == Boolean && v == Absent {
					v = No
				}
				set[v] = true
			}
			ent.values[a.Name] = set
			for v := range set {
				a.holders[v]++
			}
		}
	}

	// Metadata.
	for name, meta := range f.Attributes {
		a := kbase.byAttr[canonAttrName(name)]
		if a == nil {
			continue // reported by Lint
		}
		a.Question = strings.TrimSpace(meta.Question)
		for v, label := range meta.Labels {
			cv, _ := canonString(v)
			a.labels[cv] = label
		}
		if meta.Noise != nil {
			a.Noise, a.NoiseSet = *meta.Noise, true
		}
		if meta.Cost != nil && *meta.Cost > 0 {
			a.Cost = *meta.Cost
		}
		a.Multi = meta.Multi
		if err := a.setConfusable(meta); err != nil {
			return nil, err
		}
	}

	if err := kbase.normalisePriors(); err != nil {
		return nil, err
	}
	return kbase, nil
}

// setConfusable turns the declared look-alike groups into a neighbour list per
// value. Every value in a group is taken to look like every other value in it,
// but the relation does not chain across groups: listing a-b and b-c makes b a
// look-alike of both without making a and c look alike.
func (a *Attribute) setConfusable(meta rawAttr) error {
	if meta.Confusion != nil {
		a.Confusion = *meta.Confusion
	} else if len(meta.Confusable) > 0 {
		a.Confusion = DefaultConfusion
	}
	if a.Confusion < 0 || a.Noise+a.Confusion >= 1 {
		return fmt.Errorf("attribute %q: noise %.2f plus confusion %.2f leaves no chance of a correct answer",
			a.Name, a.Noise, a.Confusion)
	}

	inDomain := map[Value]bool{}
	for _, v := range a.Domain {
		inDomain[v] = true
	}
	for _, group := range meta.Confusable {
		canon := make([]Value, 0, len(group))
		for _, raw := range group {
			if v, _ := canonString(raw); inDomain[v] {
				canon = append(canon, v)
			} else {
				a.unknownConfusable = append(a.unknownConfusable, raw)
			}
		}
		for _, v := range canon {
			for _, other := range canon {
				if other != v && !contains(a.Confusable[v], other) {
					if a.Confusable == nil {
						a.Confusable = map[Value][]Value{}
					}
					a.Confusable[v] = append(a.Confusable[v], other)
				}
			}
		}
	}
	return nil
}

func contains(vs []Value, want Value) bool {
	for _, v := range vs {
		if v == want {
			return true
		}
	}
	return false
}

func (k *KB) normalisePriors() error {
	total := 0.0
	for _, e := range k.Entities {
		total += e.Prior
	}
	if total <= 0 {
		return errors.New("all entity priors are zero")
	}
	for _, e := range k.Entities {
		e.Prior /= total
	}
	return nil
}

func onlyBoolish(values map[Value]int) bool {
	for v := range values {
		if v != Yes && v != No && v != Absent {
			return false
		}
	}
	return true
}

// canonValues turns a JSON value into a set of canonical values. The second
// result reports whether the value looked boolean.
func canonValues(raw any) ([]Value, bool, error) {
	switch t := raw.(type) {
	case nil:
		return []Value{Absent}, false, nil
	case bool:
		if t {
			return []Value{Yes}, true, nil
		}
		return []Value{No}, true, nil
	case string:
		v, boolish := canonString(t)
		return []Value{v}, boolish, nil
	case float64:
		return []Value{Value(strconv.FormatFloat(t, 'g', -1, 64))}, false, nil
	case []any:
		var out []Value
		seen := map[Value]bool{}
		for _, item := range t {
			if _, nested := item.([]any); nested {
				return nil, false, errors.New("nested arrays are not supported")
			}
			vs, _, err := canonValues(item)
			if err != nil {
				return nil, false, err
			}
			for _, v := range vs {
				if !seen[v] {
					seen[v] = true
					out = append(out, v)
				}
			}
		}
		if len(out) == 0 {
			return []Value{Absent}, false, nil
		}
		return out, false, nil
	default:
		return nil, false, fmt.Errorf("unsupported value type %T", raw)
	}
}

func canonString(s string) (Value, bool) {
	c := strings.ToLower(strings.TrimSpace(s))
	switch c {
	case "", "none", "absent", "n/a", "null":
		return Absent, false
	case "yes", "true":
		return Yes, true
	case "no", "false":
		return No, true
	}
	return Value(strings.ReplaceAll(c, " ", "_")), false
}

func metaString(m map[string]any, key string) string {
	v, _ := m[key].(string)
	return strings.TrimSpace(v)
}

func canonAttrName(s string) string {
	return strings.ReplaceAll(strings.ToLower(strings.TrimSpace(s)), " ", "_")
}

func prettify(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "_", " "), "-", " ")
}

func upperFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
