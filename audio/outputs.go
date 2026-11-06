package audio

import (
	"sync"
	"sync/atomic"
)

// An Output is one of the system's audio outputs: speakers, headphones, a
// display's, a USB DAC.
type Output struct {
	// ID names the output to SetOutput, and stays the same across
	// launches while the output is there.
	ID string
	// Name is what the system calls it, as its sound settings do.
	Name string
	// Default reports whether it is the system's default output.
	Default bool
}

// Outputs lists the outputs sound can play through, now: on Windows, the
// active render endpoints. Elsewhere it returns none and an error, and
// sound plays through the system's default output.
func Outputs() ([]Output, error) { return listOutputs() }

var (
	outputMu sync.Mutex
	chosen   string // the output asked for, "" for the default
)

// SetOutput has sound play through the output of id, from Outputs, or
// through the system's default for "", following it as it changes. An
// output that is gone, or cannot be opened, plays through the default
// instead, until it is back. What plays moves to it within a few
// milliseconds, as the output opens it.
func SetOutput(id string) {
	outputMu.Lock()
	changed := chosen != id
	chosen = id
	outputMu.Unlock()
	if changed {
		outputMoved()
	}
}

// chosenOutput is the output asked for, "" for the default.
func chosenOutput() string {
	outputMu.Lock()
	defer outputMu.Unlock()
	return chosen
}

// moved says the output asked for changed, for the device open to give
// way to it.
var moved atomic.Bool

func outputMoved() { moved.Store(true) }
