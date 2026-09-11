package kb

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

// Severity of a lint finding.
type Severity string

const (
	Error   Severity = "error"
	Warning Severity = "warning"
	Note    Severity = "note"
)

// Issue is one finding about the knowledge base.
type Issue struct {
	Severity Severity
	Message  string
}

// Lint reports problems that would make the knowledge base behave badly: typos
// that split one attribute in two, entities nothing can tell apart, attributes
// that never discriminate.
func (k *KB) Lint() []Issue {
	var issues []Issue
	add := func(s Severity, format string, args ...any) {
		issues = append(issues, Issue{s, fmt.Sprintf(format, args...)})
	}

	// Duplicate entity names.
	seen := map[string]int{}
	for _, e := range k.Entities {
		seen[e.Name]++
	}
	for _, name := range sortedKeys(seen) {
		if seen[name] > 1 {
			add(Error, "entity %q is defined %d times", name, seen[name])
		}
	}

	// Pictures: optional, but a licence without its author cannot be honoured.
	var noImage []string
	for _, e := range k.Entities {
		if e.Image == nil {
			noImage = append(noImage, e.Name)
			continue
		}
		if e.Image.License == "" {
			add(Warning, "%s: picture has no licence recorded; it may not be safe to publish", e.Name)
		}
		if e.Image.Credit == "" {
			add(Warning, "%s: picture has no author recorded; most licences require naming one", e.Name)
		}
		if e.Image.Source == "" {
			add(Note, "%s: picture has no source page, so its licence cannot be checked", e.Name)
		}
	}
	if len(noImage) > 0 && len(noImage) < len(k.Entities) {
		add(Note, "no picture for: %s", strings.Join(noImage, ", "))
	}

	// Attributes that cannot split anything.
	for _, a := range k.Attributes {
		if len(a.Domain) < 2 {
			add(Warning, "attribute %q has a single possible value; it can never discriminate", a.Name)
			continue
		}
		held := 0
		for _, v := range a.Domain {
			if v != Absent && v != No && a.holders[v] > 0 {
				held += a.holders[v]
			}
		}
		switch {
		case held == 0:
			add(Warning, "attribute %q is not present on any entity", a.Name)
		case held == 1:
			add(Note, "attribute %q is held by a single entity; useful only to confirm it", a.Name)
		case held == len(k.Entities) && len(a.Domain) == 2 && a.Kind == Boolean:
			add(Warning, "attribute %q is true for every entity; it can never discriminate", a.Name)
		}
		if a.Multi && a.Kind == Boolean {
			add(Warning, "attribute %q is boolean, so \"multi\" has no effect; yes and no are always exclusive", a.Name)
		}
		for _, v := range a.unknownConfusable {
			add(Warning, "attribute %q lists %q as a look-alike, but no entity has that value", a.Name, v)
		}
		if a.Kind == Boolean && len(a.Confusable) > 0 {
			add(Note, "attribute %q is boolean, so its look-alike group just restates the noise", a.Name)
		}
		if a.Noise <= 0 || a.Noise >= 0.5 {
			add(Warning, "attribute %q has noise %.2f; useful values are between 0 and 0.5", a.Name, a.Noise)
		}
	}

	// Attribute names that look like typos of one another ("color"/"colour").
	for i := 0; i < len(k.Attributes); i++ {
		for j := i + 1; j < len(k.Attributes); j++ {
			x, y := k.Attributes[i].Name, k.Attributes[j].Name
			if similar(x, y) {
				add(Warning, "attributes %q and %q look like the same property spelled two ways", x, y)
			}
		}
	}

	// Values within one attribute that look like typos of one another.
	for _, a := range k.Attributes {
		for i := 0; i < len(a.Domain); i++ {
			for j := i + 1; j < len(a.Domain); j++ {
				x, y := string(a.Domain[i]), string(a.Domain[j])
				if similar(x, y) {
					add(Warning, "attribute %q has values %q and %q, which look like the same value spelled two ways", a.Name, x, y)
				}
			}
		}
	}

	// Entities nothing in the KB can tell apart.
	groups := map[string][]string{}
	var order []string
	for _, e := range k.Entities {
		sig := k.signature(e)
		if _, ok := groups[sig]; !ok {
			order = append(order, sig)
		}
		groups[sig] = append(groups[sig], e.Name)
	}
	for _, sig := range order {
		if names := groups[sig]; len(names) > 1 {
			add(Warning, "no question can tell these apart: %s", strings.Join(names, ", "))
		}
	}

	return issues
}

// signature is a stable encoding of everything the KB knows about an entity.
func (k *KB) signature(e *Entity) string {
	var b strings.Builder
	for _, a := range k.Attributes {
		vals := make([]string, 0, len(e.values[a.Name]))
		for v := range e.values[a.Name] {
			vals = append(vals, string(v))
		}
		sort.Strings(vals)
		fmt.Fprintf(&b, "%s=%s;", a.Name, strings.Join(vals, "|"))
	}
	return b.String()
}

// Stats summarises the knowledge base.
type Stats struct {
	Entities   int
	Attributes int
	Values     int
	Boolean    int

	// PriorEntropy is the uncertainty before any question is asked, in bits.
	// It is below log2(entities) whenever the priors are uneven.
	PriorEntropy float64
	// MaxOptions is the largest domain in the KB, so log2(MaxOptions) is the
	// most information a single question could possibly carry.
	MaxOptions int
	// MinQuestions is the information-theoretic floor on questions asked: no
	// strategy, this one included, can beat it.
	MinQuestions float64
}

func (k *KB) Stats() Stats {
	s := Stats{Entities: len(k.Entities), Attributes: len(k.Attributes)}
	for _, a := range k.Attributes {
		s.Values += len(a.Domain)
		if a.Kind == Boolean {
			s.Boolean++
		}
		if len(a.Domain) > s.MaxOptions {
			s.MaxOptions = len(a.Domain)
		}
	}
	for _, e := range k.Entities {
		if e.Prior > 0 {
			s.PriorEntropy -= e.Prior * math.Log2(e.Prior)
		}
	}
	if s.MaxOptions > 1 {
		s.MinQuestions = s.PriorEntropy / math.Log2(float64(s.MaxOptions))
	}
	return s
}

// similar reports whether two identifiers are close enough that one is probably
// a typo or spelling variant of the other ("color"/"colour"). Short names are
// excluded, and a single edit is the threshold: "green" and "grey" are two
// edits apart and genuinely different.
func similar(a, b string) bool {
	if a == b || len(a) < 4 || len(b) < 4 {
		return false
	}
	if abs(len(a)-len(b)) > 1 {
		return false
	}
	return levenshtein(a, b) <= 1
}

func levenshtein(a, b string) int {
	prev := make([]int, len(b)+1)
	curr := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		curr[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[j] = min3(curr[j-1]+1, prev[j]+1, prev[j-1]+cost)
		}
		prev, curr = curr, prev
	}
	return prev[len(b)]
}

func min3(a, b, c int) int {
	if b < a {
		a = b
	}
	if c < a {
		a = c
	}
	return a
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
