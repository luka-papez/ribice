package engine

import (
	"math"
	"math/rand"
	"reflect"
	"testing"

	"github.com/lpapez/ribice/kb"
)

// With no errors to make and every question answerable, the error model is
// the honest simulator, game for game, for every entity with one value per
// question. One with several draws among them in a different order, so its
// games may differ and prove nothing either way.
func TestErrorModelWithoutErrorsIsSimulate(t *testing.T) {
	k := pinnedClouds(t)
	for _, a := range k.Attributes {
		a.Noise, a.Confusion, a.AnswerRate = 0, 0, 1
	}
	single := map[*kb.Entity]bool{}
	for _, s := range truthSightings(k) {
		single[s.Target] = true
	}
	honest := Simulate(k, DefaultConfig(), SimOptions{}).Results
	model := Simulate(k, DefaultConfig(), SimOptions{ErrorModel: true}).Results
	for i := range honest {
		if !single[honest[i].Target] {
			continue
		}
		if diff := gameDiff(model[i], honest[i]); diff != "" {
			t.Errorf("%s: error model and honest simulation differ: %s", honest[i].Target.Name, diff)
		}
	}
}

// The simulated person names each value as often as Attribute.Report says.
func TestSampleReportFollowsReport(t *testing.T) {
	a := pinnedClouds(t).Attr("shape")
	const truth, draws = kb.Value("heaped"), 200000
	rng := rand.New(rand.NewSource(1))
	got := map[kb.Value]int{}
	for i := 0; i < draws; i++ {
		got[sampleReport(a, truth, rng)]++
	}
	for _, v := range a.Domain {
		want := a.Report(v, truth)
		if share := float64(got[v]) / draws; math.Abs(share-want) > 0.005 {
			t.Errorf("%s named %.3f of the time, want %.3f", v, share, want)
		}
	}
}

// A question the person could not answer stays unanswerable for the whole
// game, and a game with only such questions left ends instead of circling.
func TestErrorModelSkipsForTheWholeGame(t *testing.T) {
	k := pinnedClouds(t)
	for _, a := range k.Attributes {
		a.AnswerRate = 0.5
	}
	skips, gaveUp := 0, 0
	for seed := int64(0); seed < 5; seed++ {
		for _, g := range Simulate(k, DefaultConfig(), SimOptions{ErrorModel: true, Seed: seed}).Results {
			if g.Questions > 2*len(k.Attributes) {
				t.Errorf("%s: %d questions from %d attributes", g.Target.Name, g.Questions, len(k.Attributes))
			}
			if g.GaveUp {
				gaveUp++
			}
			skipped := map[string]bool{}
			for i, st := range g.Steps {
				if st.Skipped {
					skipped[st.Attr] = true
					skips++
				} else if skipped[st.Attr] {
					t.Errorf("%s: step %d answers %s, skipped earlier in the game", g.Target.Name, i+1, st.Attr)
				}
			}
		}
	}
	if skips == 0 || gaveUp == 0 {
		t.Errorf("half the questions unanswerable gave %d skips and %d games given up; want some of each", skips, gaveUp)
	}
}

func TestErrorModelIsRepeatable(t *testing.T) {
	k := pinnedClouds(t)
	opts := SimOptions{ErrorModel: true, Seed: 7}
	first := Simulate(k, DefaultConfig(), opts)
	again := Simulate(k, DefaultConfig(), opts)
	for i := range first.Results {
		a, b := first.Results[i], again.Results[i]
		if a.Prob != b.Prob || !reflect.DeepEqual(a.Steps, b.Steps) {
			t.Fatalf("%s: a second run differs from the first", a.Target.Name)
		}
	}
}
