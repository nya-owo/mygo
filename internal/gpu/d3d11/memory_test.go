//go:build windows

package d3d11

import (
	"runtime"
	"testing"

	"github.com/egoist/mygo/internal/gpu/gputest"
	"github.com/egoist/mygo/internal/scene"
)

// Old cover textures used to wait for hundreds of frames with no byte bound.
func TestImageCacheBudget(t *testing.T) {
	r := &Renderer{frame: 5, images: map[uint64]*texture{
		1: {w: 1024, h: 1024, lastFrame: 1},
		2: {w: 1024, h: 1024, lastFrame: 2},
		3: {w: 1024, h: 1024, lastFrame: 5},
	}}
	r.trimImages(8 << 20)
	if len(r.images) != 2 || r.images[1] != nil || r.images[2] == nil || r.images[3] == nil {
		t.Fatal("the budget did not evict only the oldest cover")
	}
	r.trimImages(1)
	if len(r.images) != 1 || r.images[3] == nil {
		t.Fatal("the budget discarded a texture in the current frame")
	}
}

// An idle renderer must release old covers without requiring further frames.
func TestIdleDropsOldImages(t *testing.T) {
	r := &Renderer{frame: 3, images: map[uint64]*texture{
		1: {w: 64, h: 64, lastFrame: 2},
		2: {w: 64, h: 64, lastFrame: 3},
	}}
	r.Idle()
	if len(r.images) != 1 || r.images[2] == nil {
		t.Fatal("idle kept an old cover or dropped the displayed one")
	}
}

// Idle windows used to keep peak-sized scratch targets and atlas copies.
func TestIdleReleasesScratchAndRedraws(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	s := gputest.Scene()
	r, err := New(hiddenWindow(t, s.Width, s.Height))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Release()
	if err := r.draw(s); err != nil {
		t.Fatal(err)
	}
	if r.mask.tex == 0 || r.color.tex == 0 || r.grab.tex == 0 || len(r.blurred) == 0 {
		t.Fatal("the scene did not exercise text, backdrops and blurred images")
	}
	r.held = true
	r.Idle()
	if r.held || len(r.blurred) != 0 {
		t.Fatal("idle retained partial drawing or blurred targets")
	}
	for _, tex := range []*texture{&r.mask, &r.color, &r.grab, &r.backdrop[0], &r.backdrop[1], &r.blurTmp[0], &r.blurTmp[1]} {
		if *tex != (texture{}) {
			t.Fatal("idle retained a scratch or atlas texture")
		}
	}
	drawn, err := r.drawFrame(s)
	if err != nil || !drawn {
		t.Fatalf("redraw after idle: drawn=%v, err=%v", drawn, err)
	}
	if r.redrew != redrewWhole {
		t.Fatal("redraw after idle did not restore the entire frame")
	}
	pix, stride := r.readBack(t)
	gputest.Compare(t, "d3d11-after-idle", pix, stride, s)
}

// A frame waiting to present still needs its renderer state.
func TestIdlePreservesPendingFrame(t *testing.T) {
	r := &Renderer{pending: true, held: true, mask: texture{w: 512, h: 512}}
	r.Idle()
	if !r.held || r.mask.w != 512 {
		t.Fatal("idle disturbed a pending frame")
	}
}

// A canvas no longer displayed can release its texture before Go collects it.
func TestIdleReleasesUnusedCanvases(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	s := gputest.CanvasScene()
	r, err := New(hiddenWindow(t, s.Width, s.Height))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Release()
	if err := r.draw(s); err != nil {
		t.Fatal(err)
	}
	count := len(r.canvases)
	if count == 0 {
		t.Fatal("the scene has no canvas textures")
	}
	r.Idle()
	if len(r.canvases) != count {
		t.Fatal("idle dropped a canvas in the displayed frame")
	}
	if err := r.draw(&scene.Scene{Width: s.Width, Height: s.Height}); err != nil {
		t.Fatal(err)
	}
	r.Idle()
	if len(r.canvases) != 0 {
		t.Fatal("idle retained canvases from the previous view")
	}
	if err := r.draw(s); err != nil {
		t.Fatal(err)
	}
	pix, stride := r.readBack(t)
	gputest.Compare(t, "d3d11-canvas-after-idle", pix, stride, s)
}
