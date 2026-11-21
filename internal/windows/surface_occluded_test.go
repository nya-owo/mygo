//go:build windows && (amd64 || arm64)

package windows

import (
	"testing"

	"github.com/egoist/mygo/internal/platform"
)

type occludedHandler struct {
	platform.WindowHandler
	shown int
}

func (h *occludedHandler) SurfaceEvent(ev platform.SurfaceEvent) bool {
	if ev.Kind == platform.SurfaceShown {
		h.shown++
	}
	return false
}

// TestSurfaceOccluded checks that a Windows surface reports occluded when
// closed or not visible, and implements platform.OccludableSurface.
func TestSurfaceOccluded(t *testing.T) {
	h := &occludedHandler{}
	s := &surface{w: &window{h: h}}
	var occ platform.OccludableSurface = s
	if !occ.Occluded() {
		t.Fatal("invisible window surface reported not occluded")
	}
	if !s.occluded {
		t.Fatal("Occluded did not record occluded state")
	}
	s.w.closed = true
	if !occ.Occluded() {
		t.Fatal("closed window surface reported not occluded")
	}
	// updateOcclusion while still occluded must not emit SurfaceShown.
	s.updateOcclusion()
	if h.shown != 0 {
		t.Fatalf("emitted %d SurfaceShown events while occluded", h.shown)
	}
}
