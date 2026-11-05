package ui

import (
	"testing"
	"time"

	"github.com/egoist/mygo/internal/surface"
)

type idleGPU struct {
	testGPU
	idles int
}

func (g *idleGPU) Idle() { g.idles++ }

// Idle windows used to retain GPU cover caches until another frame arrived.
func TestIdleReclaimsGPUImages(t *testing.T) {
	g := &idleGPU{}
	h := &windowHost{gpu: g, conn: &surface.Conn{Surface: &testSurface{}}, lastFrame: time.Time{}}
	h.idle()
	if g.idles != 1 || g.released != 0 {
		t.Fatal("idle did not trim the GPU cache while keeping the renderer usable")
	}
	if err := g.Render(nil); err != nil {
		t.Fatal(err)
	}
}
