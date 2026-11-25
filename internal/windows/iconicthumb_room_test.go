//go:build windows && (amd64 || arm64)

package windows

import "testing"

// The room a thumbnail is made in is one pixel less high than DWM asks
// for, and no more than thumbnailMaxArea: Windows 11 24H2 refused the
// sizes it asked for (nya-owo/cloudy#52).
func TestThumbnailRoomWithinWhatDWMTakes(t *testing.T) {
	for _, c := range []struct{ w, h, ww, wh int }{
		{200, 102, 200, 101},
		{400, 203, 200, 101},
		{100, 50, 100, 49},
	} {
		w, h := thumbnailRoom(c.w, c.h)
		if w != c.ww || h != c.wh {
			t.Errorf("room %dx%d: %dx%d, want %dx%d", c.w, c.h, w, h, c.ww, c.wh)
		}
		if w*h > thumbnailMaxArea {
			t.Errorf("room %dx%d: %dx%d is over %d pixels", c.w, c.h, w, h, thumbnailMaxArea)
		}
	}
	// A square cover in the room of 400x203 fits in 101x101 or so, well
	// under what 24H2 refused.
	w, h := thumbnailRoom(400, 203)
	if cw, ch := fitIn(1000, 1000, w, h); cw != ch || cw > 142 {
		t.Errorf("square cover: %dx%d", cw, ch)
	}
}
