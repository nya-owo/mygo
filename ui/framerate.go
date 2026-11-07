package ui

import (
	"math"
	"sync/atomic"
)

// maxRate is the most frames a second the app's windows draw, as
// math.Float32bits, 0 for each of the display's refreshes.
var maxRate atomic.Uint32

// SetMaxFrameRate has the app's windows draw at most fps frames a second,
// or at each of the display's refreshes for 0, the default: a setting for
// listeners on a 144 or 240 Hz display who would rather spare the GPU.
// Frames come evenly, every so many refreshes, the most that still give
// fps, as a 240 Hz display draws 120, 80 or 60 frames a second; drawings
// that ask for fewer (Painter.AnimationFrameRate) keep to theirs.
func SetMaxFrameRate(fps float32) {
	if fps < 0 || math.IsNaN(float64(fps)) {
		fps = 0
	}
	maxRate.Store(math.Float32bits(fps))
}

// maxFrameRate is the most frames a second set, 0 for none.
func maxFrameRate() float32 { return math.Float32frombits(maxRate.Load()) }

// capRate is fps under the most frames a second set: fps itself, 0 for
// each refresh, when that is fewer.
func capRate(fps float32) float32 {
	limit := maxFrameRate()
	if limit <= 0 || fps > 0 && fps <= limit {
		return fps
	}
	return limit
}
