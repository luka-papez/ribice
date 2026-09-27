package prompt

import (
	"reflect"
	"testing"
)

func TestParsePicks(t *testing.T) {
	for _, c := range []struct {
		line string
		want []int
	}{
		{"2", []int{1}},
		{"1,3", []int{0, 2}},
		{"1 3", []int{0, 2}},
		{"1+ 3", []int{0, 2}},
		{"", nil},
		{"0", nil},
		{"4", nil},
		{"1,x", nil},
	} {
		if got := ParsePicks(c.line, 3); !reflect.DeepEqual(got, c.want) {
			t.Errorf("ParsePicks(%q, 3) = %v, want %v", c.line, got, c.want)
		}
	}
}
