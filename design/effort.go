package design

import (
	"strings"

	"github.com/lpapez/ribice/kb"
)

// Reading effort is what a question asks of the person answering it: the
// words of the question and of every option they are offered, plus a little
// for each option, since each one is something to weigh against the rest.
// The quiz is judged on how much a person has to read to get their answer,
// not on how many questions it asks: a yes/no is cheap, eight long options
// are not.
const (
	// OptionWords is what considering one option costs, in words, on top of
	// reading it.
	OptionWords = 2

	// UnitWords is the reading in one unit of effort: about a short yes/no
	// question ("Was there a ring of light around the sun?", yes, no).
	UnitWords = 12

	// MaxOptions is the most answers a question may offer. More is too much
	// to read, and people stop reading before the end. It is also how many
	// the quiz shows at once: analyse and select play with it as the engine's
	// MaxOptions, as the site does, and values past it fold into "something
	// else".
	MaxOptions = 5
)

// Words counts the words in s.
func Words(s string) int { return len(strings.Fields(s)) }

// ReadingWords is the reading a question asks for when offered with these
// option labels: its words, and each option's words plus OptionWords.
func ReadingWords(question string, labels []string) int {
	n := Words(question)
	for _, l := range labels {
		n += Words(l) + OptionWords
	}
	return n
}

// Effort is ReadingWords in units of a short yes/no question.
func Effort(question string, labels []string) float64 {
	return float64(ReadingWords(question, labels)) / UnitWords
}

// optionLabels lists the labels of every option.
func optionLabels(opts []Option) []string {
	out := make([]string, len(opts))
	for i, o := range opts {
		out[i] = o.Label
	}
	return out
}

// offeredLabels is what a person read as the options of one question in a
// game: each value's label, or "something else" for values pooled into one,
// as the engine words them.
func offeredLabels(a *kb.Attribute, offered [][]kb.Value) []string {
	out := make([]string, len(offered))
	for i, vals := range offered {
		if len(vals) == 1 {
			out[i] = a.Label(vals[0])
		} else {
			out[i] = "something else"
		}
	}
	return out
}
