// Package prompt reads what a person types at a terminal prompt. It is shared
// by the quiz and by the human expert in package design, so both accept
// answers the same way.
package prompt

import (
	"strconv"
	"strings"
)

// ParsePicks reads "2", "1,3" or "1 3" into 0-based indices of n options, or
// nil if the line is not a valid selection.
func ParsePicks(line string, n int) []int {
	fields := strings.FieldsFunc(line, func(r rune) bool {
		return r == ',' || r == ' ' || r == '+' || r == '\t'
	})
	if len(fields) == 0 {
		return nil
	}
	out := make([]int, 0, len(fields))
	for _, f := range fields {
		v, err := strconv.Atoi(f)
		if err != nil || v < 1 || v > n {
			return nil
		}
		out = append(out, v-1)
	}
	return out
}
