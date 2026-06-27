package tui

import (
	"testing"
)

func TestAnimDots(t *testing.T) {
	cases := []struct {
		step   int
		expect string
	}{
		{0, "."},
		{1, ".."},
		{2, "..."},
		{3, "."},
		{4, ".."},
	}
	for _, c := range cases {
		got := animDots(c.step)
		if got != c.expect {
			t.Errorf("animDots(%d) = %s, want %s", c.step, got, c.expect)
		}
	}
}
