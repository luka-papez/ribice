// Package engine runs the identification game over a knowledge base.
//
// It holds a probability distribution over the candidate entities rather than a
// shrinking set of survivors, so a wrong answer costs the true candidate some
// probability mass instead of eliminating it for good. Questions are chosen
// greedily by expected information gain per unit of asking cost, which is the
// standard O(log n)-approximation to the (NP-hard) optimal decision tree.
package engine

import (
	"math"
	"sort"
	"strings"

	"github.com/lpapez/ribice/kb"
)

// Config tunes when to stop and how questions are shaped.
type Config struct {
	Threshold    float64 // stop once the leading candidate reaches this probability
	MinGain      float64 // stop once the best question is worth less than this, in bits
	MaxOptions   int     // largest number of choices to offer for one question
	MaxQuestions int     // hard cap; 0 means unlimited
	MinSupport   float64 // choices held by less belief mass than this fold into "something else"

	// UnknownPrior is the probability, before any question, that the thing is
	// not in the knowledge base at all. Zero drops the unknown candidate, and
	// the session goes back to assuming the answer is always in there.
	UnknownPrior float64

	// SkipCooldown is how many further questions must be answered before a
	// skipped question may be offered again.
	SkipCooldown int

	// Confirm is how many questions to spend testing the leading candidate
	// before declaring it. Picking questions by expected gain alone never asks
	// these: once one candidate leads, the answer to its own distinctive marks
	// is confidently predicted, so the expected gain is small even though a
	// contradiction would overturn everything.
	//
	// It defaults to zero -- off. On this knowledge base it costs about 1.2
	// questions and does not measurably improve accuracy, and it raises
	// confidence in answers that were already right, which makes calibration
	// worse rather than better. Kept because the idea is sound and the choice
	// of which attribute to confirm is the part that needs work; measure with
	// -confirm before turning it on.
	Confirm int
}

func DefaultConfig() Config {
	return Config{
		Threshold:    0.90,
		MinGain:      0.02,
		MaxOptions:   8,
		MinSupport:   0.005,
		UnknownPrior: 0.15,
		SkipCooldown: 3,
		Confirm:      0,
	}
}

// minConfirmGain is the least a confirmation question must be worth, in bits of
// uncertainty about whether the leading candidate is right.
const minConfirmGain = 0.005

// unknownFaunaWeight is how much the unknown candidate predicts like the
// knowledge base's own population, the rest being a flat distribution.
const unknownFaunaWeight = 0.5

// How the unknown candidate presents itself.
const (
	UnknownName = "Something not in this guide"
	UnknownNote = "A species this guide does not cover."
)

// Option is one answer the user may give. Values is the set of attribute values
// the answer covers -- a single value normally, several for the catch-all
// "something else" option.
type Option struct {
	Label  string
	Values []kb.Value
	Other  bool
	Prob   float64 // probability the user picks this, under the current belief
}

// Question is a fully formed question, ready to ask.
type Question struct {
	Attr    *kb.Attribute
	Text    string
	Options []Option
	Gain    float64 // expected reduction in entropy, in bits
	Score   float64 // Gain / Attr.Cost -- what questions are ranked by

	// Reoffered marks a question the user skipped earlier and is being asked
	// again because it now matters.
	Reoffered bool

	// Confirming marks a question asked to test the leading candidate rather
	// than to separate the field.
	Confirming bool
}

// Answer records what the user said, so the session can be replayed or undone.
type Answer struct {
	Attr       string
	Label      string
	Chosen     [][]kb.Value // the values covered by each option the user picked
	Skipped    bool
	Confirming bool
}

// Candidate is one hypothesis with its current probability. Entity is nil for
// the unknown candidate, so read Name and Note rather than reaching through it.
type Candidate struct {
	Entity  *kb.Entity
	Name    string
	Note    string
	Link    string
	Image   *kb.Image // nil for the unknown candidate, and where none is recorded
	Prob    float64
	Unknown bool
}

// Session is one run of the quiz.
type Session struct {
	KB  *kb.KB
	Cfg Config

	p         []float64 // one slot per entity, plus a final slot for "unknown"
	marginal  map[string]map[kb.Value]float64
	history   []Answer
	asked     map[string]bool
	skipped   map[string]int // attribute -> questions answered when it was skipped
	confirmed int            // confirmation questions put to the user so far
	next      *Question
	nextOK    bool
}

// unknownIndex is the slot in p holding the "not in the knowledge base" mass.
func (s *Session) unknownIndex() int { return len(s.KB.Entities) }

func (s *Session) hasUnknown() bool { return s.Cfg.UnknownPrior > 0 }

// New starts a session at the knowledge base's prior.
func New(k *kb.KB, cfg Config) *Session {
	s := &Session{KB: k, Cfg: cfg}
	s.buildMarginals()
	s.rewind()
	return s
}

// buildMarginals records how common each value is across the knowledge base,
// weighted by prior. It is the answer distribution of a species drawn from this
// fauna, which is what the unknown candidate predicts with.
func (s *Session) buildMarginals() {
	s.marginal = make(map[string]map[kb.Value]float64, len(s.KB.Attributes))
	for _, a := range s.KB.Attributes {
		m := make(map[kb.Value]float64, len(a.Domain))
		total := 0.0
		for _, e := range s.KB.Entities {
			vals := e.Values(a.Name)
			if len(vals) == 0 {
				continue
			}
			w := e.Prior / float64(len(vals))
			for v := range vals {
				m[v] += w
				total += w
			}
		}
		// Mix with a flat distribution. Without this, a value that is rare in
		// the KB gets predicted worse by the unknown candidate than by a known
		// entity that simply has it wrong -- boolean noise of 0.2 beats a
		// population rate of 0.05 -- and a mismatch would become evidence for
		// the known entity. Half and half keeps the unknown candidate at least
		// as good an explanation as a plain mistake, whatever the value.
		flat := 1.0 / float64(len(a.Domain))
		for _, v := range a.Domain {
			share := 0.0
			if total > 0 {
				share = m[v] / total
			}
			m[v] = unknownFaunaWeight*share + (1-unknownFaunaWeight)*flat
		}
		s.marginal[a.Name] = m
	}
}

func (s *Session) rewind() {
	s.p = make([]float64, len(s.KB.Entities)+1)
	share := 1.0
	if s.hasUnknown() {
		share = 1 - s.Cfg.UnknownPrior
		s.p[s.unknownIndex()] = s.Cfg.UnknownPrior
	}
	for i, e := range s.KB.Entities {
		s.p[i] = e.Prior * share
	}
	s.asked = map[string]bool{}
	s.skipped = map[string]int{}
	s.confirmed = 0
	s.nextOK = false
}

// Ask records one chosen option and updates the belief.
func (s *Session) Ask(q *Question, option int) { s.AskMany(q, []int{option}) }

// AskMany records several chosen options at once. What that means depends on
// the attribute -- see answerLikelihood. Duplicates are ignored, and an empty
// selection is treated as a skip.
func (s *Session) AskMany(q *Question, options []int) {
	var chosen [][]kb.Value
	var labels []string
	seen := map[int]bool{}
	for _, i := range options {
		if i < 0 || i >= len(q.Options) || seen[i] {
			continue
		}
		seen[i] = true
		chosen = append(chosen, q.Options[i].Values)
		labels = append(labels, q.Options[i].Label)
	}
	if len(chosen) == 0 {
		s.Skip(q)
		return
	}
	join := " or "
	if q.Attr.Multi {
		join = " and "
	}
	s.record(Answer{Attr: q.Attr.Name, Label: strings.Join(labels, join),
		Chosen: chosen, Confirming: q.Confirming})
}

// Skip records that the user does not know, which rules out nothing.
func (s *Session) Skip(q *Question) {
	s.record(Answer{Attr: q.Attr.Name, Label: "not sure", Skipped: true, Confirming: q.Confirming})
}

func (s *Session) record(a Answer) {
	s.history = append(s.history, a)
	s.apply(a)
	s.nextOK = false
}

func (s *Session) apply(a Answer) {
	if a.Confirming {
		s.confirmed++
	}
	if a.Skipped {
		// Not asked, just deferred: it may become the decisive question later.
		s.skipped[a.Attr] = len(s.history)
		return
	}
	s.asked[a.Attr] = true
	delete(s.skipped, a.Attr)
	attr := s.KB.Attr(a.Attr)
	total := 0.0
	for i := range s.p {
		s.p[i] *= s.answerLikelihoodAt(i, attr, a.Chosen)
		total += s.p[i]
	}
	if total <= 0 {
		// Every candidate was ruled out, which the noise model should make
		// impossible. Fall back to the prior rather than producing NaNs.
		s.rewind()
		return
	}
	for i := range s.p {
		s.p[i] /= total
	}
}

// Undo removes the most recent answer.
func (s *Session) Undo() bool {
	if len(s.history) == 0 {
		return false
	}
	s.history = s.history[:len(s.history)-1]
	replay := s.history
	s.history = nil
	s.rewind()
	for _, a := range replay {
		s.record(a)
	}
	return true
}

// History returns the answers given so far.
func (s *Session) History() []Answer { return s.history }

// Asked reports how many questions have been put to the user.
func (s *Session) Asked() int { return len(s.history) }

// answerLikelihoodAt dispatches to the entity model or the unknown one.
func (s *Session) answerLikelihoodAt(i int, a *kb.Attribute, chosen [][]kb.Value) float64 {
	if i == s.unknownIndex() {
		return s.unknownLikelihood(a, chosen)
	}
	return s.answerLikelihood(i, a, chosen)
}

// likelihoodAt is answerLikelihoodAt for a single option.
func (s *Session) likelihoodAt(i int, a *kb.Attribute, values []kb.Value) float64 {
	if i == s.unknownIndex() {
		return s.unknownLikelihood(a, [][]kb.Value{values})
	}
	return s.likelihood(i, a, values)
}

// unknownLikelihood is P(this answer | the thing is not in the knowledge base).
//
// The unknown candidate is not a thing with no properties; it is a species from
// the same fauna that the knowledge base happens to be missing. So it predicts
// answers at the rate they occur across the KB: "oval" is a likely answer for
// an unrecorded Adriatic fish, "seahorse" is not.
//
// That makes it a detector for unattested *combinations*. Every individual
// answer stays plausible under it, so it barely loses ground when a real
// candidate matches, but it pulls ahead as soon as the answers stop fitting any
// single entity. A flat distribution instead of this one would pay a heavy
// penalty on every high-cardinality match and could never catch up.
func (s *Session) unknownLikelihood(a *kb.Attribute, chosen [][]kb.Value) float64 {
	m := s.marginal[a.Name]
	if m == nil {
		return 1
	}
	mass := func(values []kb.Value) float64 {
		total := 0.0
		for _, v := range values {
			total += m[v]
		}
		return total
	}
	if len(chosen) == 1 {
		return mass(chosen[0])
	}
	if a.Multi {
		p := 1.0
		for _, opt := range chosen {
			p *= mass(opt)
		}
		return p
	}
	total := 0.0
	for _, opt := range chosen {
		total += mass(opt)
	}
	return total
}

// answerLikelihood is P(the user gives this answer | the entity is e).
//
// Picking several options means two different things, and the attribute decides
// which. For an ordinary attribute the entity holds exactly one value, so
// several picks say "it was one of these, I could not tell which": a
// disjunction, and the probabilities add. That is weaker than a single answer
// but still rules out everything unpicked.
//
// For a Multi attribute -- one an entity can genuinely hold several of at once,
// like where a fish lives -- each pick is a separate observation. They are
// independent, so the probabilities multiply, which favours the entities that
// hold all of them over any entity that holds just one.
func (s *Session) answerLikelihood(e int, a *kb.Attribute, chosen [][]kb.Value) float64 {
	if len(chosen) == 1 {
		return s.likelihood(e, a, chosen[0])
	}
	if a.Multi {
		p := 1.0
		for _, opt := range chosen {
			p *= s.likelihood(e, a, opt)
		}
		return p
	}
	// Options never overlap, so concatenating them gives the union.
	var union []kb.Value
	for _, opt := range chosen {
		union = append(union, opt...)
	}
	return s.likelihood(e, a, union)
}

// likelihood is P(the user answers with one of these values | the entity is e).
//
// The per-value error model lives on the attribute (see kb.Attribute.Report),
// which is where look-alike values are handled. An entity that can present
// several values -- grey or silver, say -- is taken to be equally likely to
// show any of them.
func (s *Session) likelihood(e int, a *kb.Attribute, values []kb.Value) float64 {
	if len(a.Domain) <= 1 {
		return 1
	}
	truth := s.KB.Entities[e].Values(a.Name)
	if len(truth) == 0 {
		return 1
	}
	total := 0.0
	for actual := range truth {
		for _, reported := range values {
			total += a.Report(reported, actual)
		}
	}
	return total / float64(len(truth))
}

// Entropy is the current uncertainty over the candidates, in bits.
func (s *Session) Entropy() float64 { return entropy(s.p) }

func entropy(p []float64) float64 {
	h := 0.0
	for _, x := range p {
		if x > 0 {
			h -= x * math.Log2(x)
		}
	}
	return h
}

// Top returns the n most likely candidates, best first.
func (s *Session) Top(n int) []Candidate {
	out := make([]Candidate, 0, len(s.p))
	for i, e := range s.KB.Entities {
		out = append(out, Candidate{
			Entity: e, Name: e.Name, Note: e.Note, Link: e.Link, Image: e.Image, Prob: s.p[i],
		})
	}
	if s.hasUnknown() {
		out = append(out, Candidate{
			Name: UnknownName, Note: UnknownNote, Unknown: true, Prob: s.p[s.unknownIndex()],
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Prob != out[j].Prob {
			return out[i].Prob > out[j].Prob
		}
		return out[i].Name < out[j].Name
	})
	if n > 0 && n < len(out) {
		out = out[:n]
	}
	return out
}

// Next returns the question to put to the user, or nil if none is left.
func (s *Session) Next() *Question {
	if !s.nextOK {
		s.next = s.pick()
		s.nextOK = true
	}
	return s.next
}

func (s *Session) pick() *Question {
	// Once the ordinary stopping rule is satisfied, spend a couple of questions
	// testing the leader before declaring it.
	if stop, why := s.stopping(); stop && why != "question limit reached" {
		if q := s.confirmation(); q != nil {
			return q
		}
	}
	if ranked := s.rank(false); len(ranked) > 0 {
		return ranked[0]
	}
	return nil
}

// confirmation picks the unasked question that would most sharply test whether
// the leading candidate is right, or nil when the budget is spent or nothing
// left would test it.
//
// It is scored as a binary question -- is it the leader, or is it anything else
// -- rather than as a question about which of the whole field it is. That is
// the difference that makes it ask: separating the also-rans is worth almost
// nothing once one candidate leads, so ordinary scoring goes quiet exactly when
// the leader's own distinctive marks are still untested.
func (s *Session) confirmation() *Question {
	if s.Cfg.Confirm <= 0 || s.confirmed >= s.Cfg.Confirm {
		return nil
	}
	top := s.Top(1)
	if len(top) == 0 || top[0].Unknown {
		return nil
	}
	leader := -1
	for i, e := range s.KB.Entities {
		if e == top[0].Entity {
			leader = i
		}
	}
	if leader < 0 {
		return nil
	}

	var best *Question
	bestGain := minConfirmGain
	for _, q := range s.rank(true) {
		if g := s.confirmGain(q, leader); g > bestGain {
			best, bestGain = q, g
		}
	}
	if best != nil {
		best.Confirming = true
	}
	return best
}

// confirmGain is the expected reduction in uncertainty about the single claim
// "the leading candidate is the right one", in bits.
func (s *Session) confirmGain(q *Question, leader int) float64 {
	before := binaryEntropy(s.p[leader])
	expected := 0.0
	for _, opt := range q.Options {
		total, lead := 0.0, 0.0
		for e := range s.p {
			l := s.p[e] * s.likelihoodAt(e, q.Attr, opt.Values)
			total += l
			if e == leader {
				lead = l
			}
		}
		if total > 0 {
			expected += total * binaryEntropy(lead/total)
		}
	}
	if gain := before - expected; gain > 0 {
		return gain
	}
	return 0
}

func binaryEntropy(p float64) float64 {
	if p <= 0 || p >= 1 {
		return 0
	}
	return -p*math.Log2(p) - (1-p)*math.Log2(1-p)
}

// Rank builds every remaining question and orders them by score, best first.
func (s *Session) Rank() []*Question { return s.rank(false) }

// rank collects the askable questions. A question the user skipped is held back
// for SkipCooldown further answers rather than dropped for good -- skipping
// means "I cannot say right now", not "never ask me this". Once everything else
// is exhausted the cooldown is waived, since re-asking beats giving up.
func (s *Session) rank(waiveCooldown bool) []*Question {
	var out []*Question
	for _, a := range s.KB.Attributes {
		if s.asked[a.Name] {
			continue
		}
		reoffered := false
		if at, skipped := s.skipped[a.Name]; skipped {
			if !waiveCooldown && len(s.history)-at < s.Cfg.SkipCooldown {
				continue
			}
			reoffered = true
		}
		if q := s.build(a); q != nil {
			q.Reoffered = reoffered
			out = append(out, q)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].Attr.Name < out[j].Attr.Name
	})
	if len(out) == 0 && !waiveCooldown && len(s.skipped) > 0 {
		return s.rank(true)
	}
	return out
}

// build turns an attribute into a question: it picks which values are worth
// offering as choices, then scores the resulting partition.
func (s *Session) build(a *kb.Attribute) *Question {
	q := &Question{Attr: a, Text: a.Title()}
	for _, vals := range s.choices(a) {
		q.Options = append(q.Options, Option{
			Label:  s.optionLabel(a, vals),
			Values: vals,
			Other:  len(vals) > 1,
		})
	}
	if len(q.Options) < 2 {
		return nil
	}
	s.score(q)
	return q
}

// choices partitions the attribute's domain into the value sets each option
// will cover. Values almost no remaining candidate holds are pooled into a
// single catch-all so the user is not asked to scan twenty dead choices.
func (s *Session) choices(a *kb.Attribute) [][]kb.Value {
	if a.Kind == kb.Boolean {
		return [][]kb.Value{{kb.Yes}, {kb.No}}
	}

	// Support is the belief mass currently behind each value, ignoring noise.
	// It is measured over the entities alone and renormalised, so a large
	// unknown mass does not quietly bucket real options into "something else".
	entityMass := 0.0
	for i := range s.KB.Entities {
		entityMass += s.p[i]
	}
	if entityMass <= 0 {
		entityMass = 1
	}
	support := map[kb.Value]float64{}
	for i, e := range s.KB.Entities {
		vals := e.Values(a.Name)
		if len(vals) == 0 {
			continue
		}
		w := s.p[i] / float64(len(vals)) / entityMass
		for v := range vals {
			support[v] += w
		}
	}

	ordered := append([]kb.Value(nil), a.Domain...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if support[ordered[i]] != support[ordered[j]] {
			return support[ordered[i]] > support[ordered[j]]
		}
		return ordered[i] < ordered[j]
	})

	var keep, rest []kb.Value
	for i, v := range ordered {
		remaining := len(ordered) - i
		switch {
		case len(keep) == 0: // always offer the leading value
			keep = append(keep, v)
		case support[v] < s.Cfg.MinSupport:
			rest = append(rest, v)
		case s.Cfg.MaxOptions > 0 && len(keep) >= s.Cfg.MaxOptions-1 && remaining > 1:
			rest = append(rest, v)
		default:
			keep = append(keep, v)
		}
	}
	// A catch-all covering one value is just that value.
	if len(rest) == 1 {
		keep, rest = append(keep, rest[0]), nil
	}

	out := make([][]kb.Value, 0, len(keep)+1)
	for _, v := range keep {
		out = append(out, []kb.Value{v})
	}
	if len(rest) > 0 {
		out = append(out, rest)
	}
	return out
}

func (s *Session) optionLabel(a *kb.Attribute, vals []kb.Value) string {
	if len(vals) == 1 {
		return a.Label(vals[0])
	}
	return "something else"
}

// score computes each option's probability and the question's information gain.
// The options partition the domain, so their probabilities sum to one and the
// gain is the entropy the answer is expected to remove.
//
// This assumes the user picks exactly one option. For a Multi attribute the
// true gain is higher, since several picks say more than one does, so the score
// is a lower bound there -- good enough to rank by, and it never oversells a
// question.
func (s *Session) score(q *Question) {
	before := entropy(s.p)
	posterior := make([]float64, len(s.p))
	expected := 0.0

	for i := range q.Options {
		opt := &q.Options[i]
		total := 0.0
		for e := range s.p {
			posterior[e] = s.p[e] * s.likelihoodAt(e, q.Attr, opt.Values)
			total += posterior[e]
		}
		opt.Prob = total
		if total <= 0 {
			continue
		}
		for e := range posterior {
			posterior[e] /= total
		}
		expected += total * entropy(posterior)
	}

	q.Gain = before - expected
	if q.Gain < 0 {
		q.Gain = 0 // floating-point dust
	}
	q.Score = q.Gain / q.Attr.Cost
}

// Done reports whether the quiz should stop, and why.
func (s *Session) Done() (bool, string) {
	stop, why := s.stopping()
	if !stop || why == "question limit reached" {
		return stop, why
	}
	if s.confirmation() != nil {
		return false, ""
	}
	return true, why
}

// stopping is the ordinary stopping rule, before the confirmation budget is
// taken into account.
func (s *Session) stopping() (bool, string) {
	if s.Cfg.MaxQuestions > 0 && len(s.history) >= s.Cfg.MaxQuestions {
		return true, "question limit reached"
	}
	if top := s.Top(1); len(top) > 0 && top[0].Prob >= s.Cfg.Threshold {
		if top[0].Unknown {
			return true, "nothing in this guide matches"
		}
		return true, "confident enough"
	}
	ranked := s.rank(false)
	if len(ranked) == 0 {
		return true, "no questions left to ask"
	}
	if ranked[0].Gain < s.Cfg.MinGain {
		return true, "no remaining question would tell us much"
	}
	return false, ""
}
