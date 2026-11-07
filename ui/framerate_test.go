package ui

import "testing"

// Under a most frames a second, what asks for every refresh gets the most,
// and what asks for fewer keeps to its own.
func TestCapRate(t *testing.T) {
	defer SetMaxFrameRate(0)
	if got := capRate(0); got != 0 {
		t.Errorf("no cap: %v", got)
	}
	SetMaxFrameRate(60)
	for _, c := range []struct{ in, want float32 }{{0, 60}, {30, 30}, {60, 60}, {120, 60}} {
		if got := capRate(c.in); got != c.want {
			t.Errorf("capRate(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}
