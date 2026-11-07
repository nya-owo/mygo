package ui

import (
	"math"
	"sync/atomic"
)

// maxRate is the most frames a second the app's windows draw, as
// math.Float32bits, 0 for each of the display's refreshes.
var maxRate atomic.Uint32

// SetMaxFrameRate has the app's windows show at most fps frames a second,
// or one at each of the display's refreshes for 0, the default: a setting
// for listeners on a 144 or 240 Hz display who would rather spare the GPU.
// The display paces them, each frame shown for as many refreshes as keep
// to fps, up to four (Direct3D's present interval): a 240 Hz display shows
// 120, 80 or 60 frames a second, evenly. Elsewhere it changes nothing yet.
func SetMaxFrameRate(fps float32) {
	if fps < 0 || math.IsNaN(float64(fps)) {
		fps = 0
	}
	maxRate.Store(math.Float32bits(fps))
}

// maxFrameRate is the most frames a second set, 0 for none.
func maxFrameRate() float32 { return math.Float32frombits(maxRate.Load()) }
