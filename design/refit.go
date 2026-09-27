package design

import (
	"github.com/lpapez/ribice/kb"
)

// Refit fits the error model of each current attribute that has perceive
// answers for every value some entity holds, from those answers and the
// knowledge base's own values. Attributes it could not fit are listed
// with the values missing, so a partial consultation never passes for a
// measurement.
func Refit(k *kb.KB, perceive []Task, verdicts func(string) []Verdict) (map[string]ErrorModel, map[string][]kb.Value) {
	byAttr := map[string][]Task{}
	for _, t := range perceive {
		if t.Kind == Perceive {
			byAttr[t.Attribute] = append(byAttr[t.Attribute], t)
		}
	}
	fitted := map[string]ErrorModel{}
	missing := map[string][]kb.Value{}
	for _, a := range k.Attributes {
		tasks := byAttr[a.Name]
		if len(tasks) == 0 {
			continue
		}
		holders := map[kb.Value]int{} // counting the closed world's implicit none and no
		for _, v := range a.Domain {
			holders[v] = a.Holders(v)
		}
		m := FitErrors(tasks[0].Options, holders, tasks, verdicts)
		if len(m.Rows) == 0 || len(m.Missing) > 0 {
			missing[a.Name] = m.Missing
			continue
		}
		fitted[a.Name] = m
	}
	return fitted, missing
}
