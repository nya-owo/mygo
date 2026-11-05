//go:build windows

package d3d11

import "testing"

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
