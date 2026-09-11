package engine

import (
	"fmt"
	"math"
	"testing"

	"github.com/lpapez/ribice/kb"
)

func load(t *testing.T, src string) *kb.KB {
	t.Helper()
	k, err := kb.Load([]byte(src))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return k
}

func fishKB(t *testing.T) *kb.KB {
	t.Helper()
	k, err := kb.LoadFile("../data/adriatic-fish.json")
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	return k
}

// The noise model must be a proper distribution: whatever the entity really is,
// the probabilities of the answers it could give add up to one.
func TestLikelihoodIsADistribution(t *testing.T) {
	k := fishKB(t)
	s := New(k, DefaultConfig())
	for _, a := range k.Attributes {
		for e := range k.Entities {
			total := 0.0
			for _, v := range a.Domain {
				total += s.likelihood(e, a, []kb.Value{v})
			}
			if math.Abs(total-1) > 1e-9 {
				t.Fatalf("%s / %s: likelihoods sum to %v, want 1", k.Entities[e].Name, a.Name, total)
			}
		}
	}
}

// Options partition the domain, so their probabilities must also sum to one,
// and the gain they imply must be real (non-negative, and no more information
// than there is uncertainty to remove).
func TestQuestionsArePartitions(t *testing.T) {
	k := fishKB(t)
	s := New(k, DefaultConfig())
	for _, q := range s.Rank() {
		total := 0.0
		for _, opt := range q.Options {
			total += opt.Prob
		}
		if math.Abs(total-1) > 1e-9 {
			t.Errorf("%s: option probabilities sum to %v, want 1", q.Attr.Name, total)
		}
		if q.Gain < 0 || q.Gain > s.Entropy()+1e-9 {
			t.Errorf("%s: gain %v is outside [0, %v]", q.Attr.Name, q.Gain, s.Entropy())
		}
	}
}

func TestRankedBestFirst(t *testing.T) {
	s := New(fishKB(t), DefaultConfig())
	ranked := s.Rank()
	if len(ranked) < 2 {
		t.Fatal("expected several questions")
	}
	for i := 1; i < len(ranked); i++ {
		if ranked[i-1].Score < ranked[i].Score {
			t.Fatalf("questions are not ordered by score: %v before %v", ranked[i-1].Score, ranked[i].Score)
		}
	}
	if s.Next().Attr != ranked[0].Attr {
		t.Errorf("Next asked about %q, want the highest-scoring %q", s.Next().Attr.Name, ranked[0].Attr.Name)
	}
}

// An answer must move belief towards the entities that match it.
func TestAnswerShiftsBelief(t *testing.T) {
	k := load(t, `[
	  {"name": "red one", "colour": "red"},
	  {"name": "blue one", "colour": "blue"}
	]`)
	s := New(k, DefaultConfig())
	q := s.Next()
	if q == nil {
		t.Fatal("no question")
	}
	var red int
	for i, opt := range q.Options {
		if opt.Values[0] == "red" {
			red = i
		}
	}
	s.Ask(q, red)
	if top := s.Top(1)[0]; top.Name != "red one" {
		t.Fatalf("leader is %q, want %q", top.Name, "red one")
	}
}

// Nothing is ever eliminated, so a contradicted answer stays recoverable.
func TestWrongAnswerIsRecoverable(t *testing.T) {
	k := fishKB(t)
	s := New(k, DefaultConfig())
	q := s.Next()
	s.Ask(q, len(q.Options)-1)
	for _, c := range s.Top(0) {
		if c.Prob <= 0 {
			t.Fatalf("%s was eliminated outright; the noise model should keep it alive", c.Name)
		}
	}
}

// Soft elimination is the point of the Bayesian update: a single wrong answer
// should cost the true candidate some questions, not rule it out for good.
func TestRecoversFromOneWrongAnswer(t *testing.T) {
	k := fishKB(t)
	salema := entity(t, k, "Salema")

	s := New(k, DefaultConfig())
	for {
		if done, _ := s.Done(); done {
			break
		}
		q := s.Next()
		want := func(v kb.Value) bool { return salema.Has(q.Attr.Name, v) }
		if q.Attr.Name == "shape" {
			want = func(v kb.Value) bool { return v == "elongated" } // the lie
		}
		pick := -1
		for i, opt := range q.Options {
			for _, v := range opt.Values {
				if want(v) && (pick < 0 || !opt.Other) {
					pick = i
				}
			}
		}
		if pick < 0 {
			t.Fatalf("no option for %q matches a salema", q.Attr.Name)
		}
		s.Ask(q, pick)
	}

	top := s.Top(1)[0]
	if top.Entity != salema {
		t.Fatalf("after one wrong answer the leader is %q at %.1f%%, want Salema",
			top.Name, top.Prob*100)
	}
}

func entity(t *testing.T, k *kb.KB, name string) *kb.Entity {
	t.Helper()
	for _, e := range k.Entities {
		if e.Name == name {
			return e
		}
	}
	t.Fatalf("entity %q not in the knowledge base", name)
	return nil
}

func TestSkipChangesNothing(t *testing.T) {
	s := New(fishKB(t), DefaultConfig())
	before := s.Top(0)
	s.Skip(s.Next())
	for i, c := range s.Top(0) {
		if math.Abs(c.Prob-before[i].Prob) > 1e-12 {
			t.Fatal("skipping a question must not change the belief")
		}
	}
}

// Skipping means "I cannot say right now", not "never ask me this": the
// question must be held back briefly and then become available again.
func TestSkippedQuestionComesBack(t *testing.T) {
	cfg := DefaultConfig()
	cfg.SkipCooldown = 3
	s := New(fishKB(t), cfg)

	skipped := s.Next().Attr.Name
	s.Skip(s.Next())

	for i := 0; i < cfg.SkipCooldown; i++ {
		if offered(s, skipped) != nil {
			t.Fatalf("%q was offered again after only %d answers, want a cooldown of %d",
				skipped, i, cfg.SkipCooldown)
		}
		s.Ask(s.Next(), 0)
	}
	q := offered(s, skipped)
	if q == nil {
		t.Fatalf("%q never came back after its cooldown", skipped)
	}
	if !q.Reoffered {
		t.Error("a question returning after a skip should be marked Reoffered")
	}

	// Skipping it again restarts the wait.
	s.Skip(q)
	if offered(s, skipped) != nil {
		t.Error("skipping a second time should start the cooldown over")
	}
}

// Rather than give up, the cooldown is waived once nothing else is left.
func TestCooldownWaivedWhenNothingElseIsLeft(t *testing.T) {
	cfg := DefaultConfig()
	cfg.SkipCooldown = 1000
	k := load(t, `[{"name": "a", "colour": "red"}, {"name": "b", "colour": "blue"}]`)
	s := New(k, cfg)

	only := s.Next().Attr.Name
	s.Skip(s.Next())
	q := s.Next()
	if q == nil || q.Attr.Name != only {
		t.Fatal("the last remaining question should be re-offered rather than ending the quiz")
	}
	if !q.Reoffered {
		t.Error("it should still be marked as previously skipped")
	}
}

// offered reports the question for attr if the session would ask it now.
func offered(s *Session, attr string) *Question {
	for _, q := range s.Rank() {
		if q.Attr.Name == attr {
			return q
		}
	}
	return nil
}

func TestUndoRestoresBelief(t *testing.T) {
	s := New(fishKB(t), DefaultConfig())
	before := s.Top(0)
	s.Ask(s.Next(), 0)
	s.Ask(s.Next(), 1)
	if !s.Undo() || !s.Undo() {
		t.Fatal("Undo should succeed twice")
	}
	if s.Undo() {
		t.Error("Undo should fail with an empty history")
	}
	for i, c := range s.Top(0) {
		if c.Entity != before[i].Entity || math.Abs(c.Prob-before[i].Prob) > 1e-12 {
			t.Fatal("undoing every answer should return to the prior")
		}
	}
	if s.Asked() != 0 {
		t.Errorf("asked = %d, want 0", s.Asked())
	}
}

// The knowledge base shipped with the project must actually separate its fish.
func TestFishKnowledgeBaseSeparates(t *testing.T) {
	k := fishKB(t)
	r := Simulate(k, DefaultConfig(), SimOptions{})
	if r.Correct != len(k.Entities) {
		t.Errorf("identified %d/%d with honest answers; misses: %v",
			r.Correct, len(k.Entities), names(r.Worst))
	}
	if r.MeanAsked > 6 {
		t.Errorf("average %.1f questions is more than expected", r.MeanAsked)
	}
}

// A quarter of answers wrong is a bad day at sea; most fish should still land.
func TestToleratesWrongAnswers(t *testing.T) {
	k := fishKB(t)
	correct, runs := 0, 0
	for seed := int64(0); seed < 8; seed++ {
		r := Simulate(k, DefaultConfig(), SimOptions{Noise: 0.25, Seed: seed})
		correct += r.Correct
		runs += len(r.Results)
	}
	if rate := float64(correct) / float64(runs); rate < 0.6 {
		t.Errorf("identified %.0f%% with a quarter of answers wrong, want at least 60%%", rate*100)
	}
}

func names(rs []SimResult) []string {
	out := make([]string, len(rs))
	for i, r := range rs {
		out[i] = r.Target.Name
	}
	return out
}

// probOf is the current probability of a named entity.
func probOf(t *testing.T, s *Session, name string) float64 {
	t.Helper()
	for _, c := range s.Top(0) {
		if c.Name == name {
			return c.Prob
		}
	}
	t.Fatalf("entity %q not found", name)
	return 0
}

// pick finds the option covering a value.
func pick(t *testing.T, q *Question, v kb.Value) int {
	t.Helper()
	for i, opt := range q.Options {
		for _, ov := range opt.Values {
			if ov == v && !opt.Other {
				return i
			}
		}
	}
	t.Fatalf("no option offers %q for %q", v, q.Attr.Name)
	return 0
}

// question builds the question for one attribute regardless of its rank.
func question(t *testing.T, s *Session, attr string) *Question {
	t.Helper()
	for _, q := range s.Rank() {
		if q.Attr.Name == attr {
			return q
		}
	}
	t.Fatalf("no question for %q", attr)
	return nil
}

// "I saw it over rocks and in seagrass" is two observations, so it should
// favour the fish that live in both over one that only lives in one.
func TestMultiSelectIsConjunctive(t *testing.T) {
	k := fishKB(t)
	if !k.Attr("where").Multi {
		t.Fatal("where should be a multi attribute")
	}
	s := New(k, DefaultConfig())
	q := question(t, s, "where")

	bothBefore := probOf(t, s, "Salema")          // over_rocks + in_seagrass
	rocksOnlyBefore := probOf(t, s, "Damselfish") // over_rocks only

	s.AskMany(q, []int{pick(t, q, "over_rocks"), pick(t, q, "in_seagrass")})

	both := probOf(t, s, "Salema") / bothBefore
	rocksOnly := probOf(t, s, "Damselfish") / rocksOnlyBefore
	if both <= rocksOnly {
		t.Errorf("picking both habitats favoured the single-habitat fish: "+
			"salema x%.2f, damselfish x%.2f", both, rocksOnly)
	}
	if surface := probOf(t, s, "Sand smelt"); surface >= probOf(t, s, "Damselfish") {
		t.Error("a fish in neither named habitat should fall behind one in a named habitat")
	}
}

// On an ordinary attribute the entity has exactly one value, so picking several
// means "one of these, I could not tell which". That should lift the named
// groups together without preferring either of them, and should be weaker than
// naming one group outright.
func TestMultiSelectIsDisjunctiveOnOrdinaryAttribute(t *testing.T) {
	k := fishKB(t)
	if k.Attr("shape").Multi {
		t.Fatal("shape should not be a multi attribute")
	}
	// Measured as a share of the entity mass, so the unknown candidate taking
	// its cut of a deliberately vague answer does not confuse the comparison.
	named := func(s *Session) float64 {
		var total, entities float64
		for _, c := range s.Top(0) {
			if c.Unknown {
				continue
			}
			entities += c.Prob
			if c.Entity.Has("shape", "eel") || c.Entity.Has("shape", "needle") {
				total += c.Prob
			}
		}
		return total / entities
	}

	both := New(k, DefaultConfig())
	before := named(both)
	q := question(t, both, "shape")
	both.AskMany(q, []int{pick(t, q, "eel"), pick(t, q, "needle")})
	after := named(both)
	if after < 5*before {
		t.Errorf("eel-or-needle holds %.0f%% of belief, up from only %.0f%%; expected a much sharper lift",
			after*100, before*100)
	}

	// The rest of the belief should sit on shapes declared to look like the two
	// named ones, not scattered over the whole KB.
	shape := k.Attr("shape")
	plausible := map[kb.Value]bool{"eel": true, "needle": true}
	for _, v := range append(shape.Confusable["eel"], shape.Confusable["needle"]...) {
		plausible[v] = true
	}
	var accounted, entityMass float64
	for _, c := range both.Top(0) {
		if c.Unknown {
			continue
		}
		entityMass += c.Prob
		for v := range plausible {
			if c.Entity.Has("shape", v) {
				accounted += c.Prob
				break
			}
		}
	}
	accounted /= entityMass
	// The remainder is the flat noise floor spread over every other species.
	if accounted < 0.85 {
		t.Errorf("the named shapes and their look-alikes hold %.0f%% of belief, want nearly all of it",
			accounted*100)
	}

	// A disjunction says nothing about which of the two it was, so the odds
	// between an eel and a needle must stay exactly as the priors had them.
	moray, garfish := entity(t, k, "Mediterranean moray"), entity(t, k, "Garfish")
	wantOdds := moray.Prior / garfish.Prior
	gotOdds := probOf(t, both, moray.Name) / probOf(t, both, garfish.Name)
	if math.Abs(gotOdds-wantOdds) > 1e-9 {
		t.Errorf("odds between the two named shapes moved to %.3f, want the prior %.3f", gotOdds, wantOdds)
	}

	// And it must be weaker evidence than naming the eel alone.
	one := New(k, DefaultConfig())
	q = question(t, one, "shape")
	one.Ask(q, pick(t, q, "eel"))
	if probOf(t, both, moray.Name) >= probOf(t, one, moray.Name) {
		t.Error("hedging across two shapes should be weaker evidence than naming one")
	}
}

// Picking one option must behave exactly as it did before multi-select existed.
func TestAskManyWithOneOptionMatchesAsk(t *testing.T) {
	k := fishKB(t)
	one, many := New(k, DefaultConfig()), New(k, DefaultConfig())
	one.Ask(one.Next(), 2)
	many.AskMany(many.Next(), []int{2})
	for i, c := range one.Top(0) {
		if d := many.Top(0)[i]; d.Entity != c.Entity || math.Abs(d.Prob-c.Prob) > 1e-12 {
			t.Fatal("Ask and AskMany with one option must agree")
		}
	}
}

func TestAskManyIgnoresDuplicatesAndEmpty(t *testing.T) {
	s := New(fishKB(t), DefaultConfig())
	q := s.Next()
	s.AskMany(q, []int{1, 1, 1})
	if got := s.History()[0].Chosen; len(got) != 1 {
		t.Errorf("chose %d options, want 1 after removing duplicates", len(got))
	}
	q = s.Next()
	s.AskMany(q, nil)
	if !s.History()[1].Skipped {
		t.Error("an empty selection should be recorded as a skip")
	}
}

// Selecting every option on an ordinary attribute says nothing at all, which is
// the same as skipping.
func TestSelectingEverythingIsASkip(t *testing.T) {
	s := New(fishKB(t), DefaultConfig())
	q := question(t, s, "shape")
	before := s.Top(0)
	all := make([]int, len(q.Options))
	for i := range all {
		all[i] = i
	}
	s.AskMany(q, all)
	for i, c := range s.Top(0) {
		if math.Abs(c.Prob-before[i].Prob) > 1e-12 {
			t.Fatalf("selecting every option changed the belief for %s", c.Name)
		}
	}
}

// The point of look-alike groups: answering "torpedo" should barely dent a fish
// that is really elongated, while still ruling out one shaped nothing like it.
func TestLookalikesSurviveAWrongAnswer(t *testing.T) {
	k := fishKB(t)
	if len(k.Attr("shape").Confusable) == 0 {
		t.Fatal("the fish KB should declare look-alike shapes")
	}
	s := New(k, DefaultConfig())
	q := question(t, s, "shape")

	// Bogue is elongated, seahorse is not remotely torpedo-shaped.
	bogueBefore := probOf(t, s, "Bogue")
	horseBefore := probOf(t, s, "Long-snouted seahorse")
	s.Ask(q, pick(t, q, "torpedo"))

	bogue := probOf(t, s, "Bogue") / bogueBefore
	horse := probOf(t, s, "Long-snouted seahorse") / horseBefore
	if bogue < 5*horse {
		t.Errorf("a look-alike shape should survive far better than an unrelated one: "+
			"elongated fish x%.3f, seahorse x%.3f", bogue, horse)
	}
	if bogue >= 1 {
		t.Error("a wrong answer should still cost the look-alike something")
	}
}

// Admitting that people confuse two answers must make the question genuinely
// less informative -- that is the honest cost of the model.
func TestConfusionLowersInformationGain(t *testing.T) {
	src := `{
	  "attributes": {"shape": {"noise": 0.06%s}},
	  "entities": [
	    {"name": "a", "shape": "torpedo"}, {"name": "b", "shape": "elongated"},
	    {"name": "c", "shape": "eel"},     {"name": "d", "shape": "seahorse"}
	  ]
	}`
	plain := New(load(t, fmt.Sprintf(src, "")), DefaultConfig())
	grouped := New(load(t, fmt.Sprintf(src,
		`, "confusion": 0.3, "confusable": [["torpedo", "elongated"]]`)), DefaultConfig())

	if grouped.Next().Gain >= plain.Next().Gain {
		t.Errorf("declaring two answers confusable should reduce the question's gain: "+
			"%.3f bits with groups, %.3f without", grouped.Next().Gain, plain.Next().Gain)
	}
}

// A real species answered honestly must leave the unknown candidate flat.
func TestUnknownStaysQuietOnKnownSpecies(t *testing.T) {
	k := fishKB(t)
	r := Simulate(k, DefaultConfig(), SimOptions{})
	if r.Unsure > 0 {
		t.Errorf("%d honest sessions ended in \"nothing matches\", want none", r.Unsure)
	}
	if r.Correct != len(k.Entities) {
		t.Errorf("identified %d/%d with the unknown candidate enabled", r.Correct, len(k.Entities))
	}
}

// Answers that no single entity explains should push belief to "unknown"
// rather than to whichever species happens to fit least badly.
func TestUnknownWinsOnAnUnattestedCombination(t *testing.T) {
	k := load(t, `{
	  "attributes": {"colour": {"noise": 0.1}, "shape": {"noise": 0.1}, "size": {"noise": 0.1}},
	  "entities": [
	    {"name": "red round big",    "colour": "red",   "shape": "round", "size": "big"},
	    {"name": "blue flat small",  "colour": "blue",  "shape": "flat",  "size": "small"},
	    {"name": "green long medium","colour": "green", "shape": "long",  "size": "medium"}
	  ]
	}`)
	cfg := DefaultConfig()
	cfg.Confirm = 0
	s := New(k, cfg)

	// Each answer is ordinary on its own; the combination belongs to nothing.
	for _, want := range []kb.Value{"red", "flat", "medium"} {
		q := questionFor(s, want)
		if q == nil {
			t.Fatalf("no question offers %q", want)
		}
		s.Ask(q, pick(t, q, want))
	}
	if top := s.Top(1)[0]; !top.Unknown {
		t.Errorf("leader is %q at %.0f%%, want the unknown candidate", top.Name, top.Prob*100)
	}
}

func questionFor(s *Session, v kb.Value) *Question {
	for _, q := range s.Rank() {
		for _, opt := range q.Options {
			for _, ov := range opt.Values {
				if ov == v {
					return q
				}
			}
		}
	}
	return nil
}

func TestUnknownCanBeDisabled(t *testing.T) {
	cfg := DefaultConfig()
	cfg.UnknownPrior = 0
	s := New(fishKB(t), cfg)
	total := 0.0
	for _, c := range s.Top(0) {
		if c.Unknown {
			t.Fatal("no unknown candidate should be offered when UnknownPrior is zero")
		}
		total += c.Prob
	}
	if math.Abs(total-1) > 1e-9 {
		t.Errorf("entity probabilities sum to %v, want 1 when unknown is disabled", total)
	}
}

// Whatever the truth, the unknown candidate's predictions over one question's
// options must be a distribution, or information gain is computed wrong.
func TestUnknownLikelihoodIsADistribution(t *testing.T) {
	k := fishKB(t)
	s := New(k, DefaultConfig())
	for _, q := range s.Rank() {
		total := 0.0
		for _, opt := range q.Options {
			total += s.unknownLikelihood(q.Attr, [][]kb.Value{opt.Values})
		}
		if math.Abs(total-1) > 1e-9 {
			t.Errorf("%s: unknown's option probabilities sum to %v, want 1", q.Attr.Name, total)
		}
	}
}
