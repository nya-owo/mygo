package ui

import "testing"

// The most frames a second set is what the windows read back; a rate
// that is no rate is none.
func TestMaxFrameRate(t *testing.T) {
	defer SetMaxFrameRate(0)
	SetMaxFrameRate(120)
	if got := maxFrameRate(); got != 120 {
		t.Errorf("set 120, read %v", got)
	}
	SetMaxFrameRate(-5)
	if got := maxFrameRate(); got != 0 {
		t.Errorf("set -5, read %v", got)
	}
}
