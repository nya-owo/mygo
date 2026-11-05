package gpu

import (
	"runtime"
	"testing"
	"weak"

	"github.com/egoist/mygo/internal/scene"
)

// A shorter frame must not keep images through unused shader batches.
func TestShortFrameReleasesShaderImages(t *testing.T) {
	b := &Builder{NoFold: true}
	old := buildImageEffect(b)
	b.Build(&scene.Scene{Width: 100, Height: 100}, nil)
	runtime.GC()
	if old.Value() != nil {
		t.Fatal("a batch outside the new frame retained its image")
	}
	runtime.KeepAlive(b)
}

func buildImageEffect(b *Builder) weak.Pointer[scene.Image] {
	img := scene.NewImageRGBA(512, 512, make([]byte, 512*512*4))
	s := &scene.Scene{Width: 100, Height: 100,
		Ops:     []scene.Op{{Kind: scene.OpEffect, Rect: scene.Rect{W: 80, H: 80}}},
		Effects: []scene.EffectOp{{Effect: &scene.Effect{Shader: &scene.ShaderLayout{}}, Images: [4]*scene.Image{img}}}}
	b.Build(s, func(*scene.Image) uintptr { return 1 })
	return weak.Make(img)
}

// Masks no longer drawn used to retain up to 32 MiB after a window went idle.
func TestIdleReleasesUnusedClipMasks(t *testing.T) {
	var b Builder
	var old weak.Pointer[scene.Image]
	s := &scene.Scene{Width: 100, Height: 100, Ops: []scene.Op{
		{Kind: scene.OpPushClip, Rect: scene.Rect{W: 80, H: 80}, Radii: scene.Uniform(8)},
		{Kind: scene.OpFill, Rect: scene.Rect{W: 50, H: 50}, Transform: scene.Rotation(5), Color: scene.Color{A: 255}},
		{Kind: scene.OpPopClip},
	}}
	b.Build(s, func(img *scene.Image) uintptr { old = weak.Make(img); return 1 })
	if old.Value() == nil {
		t.Fatal("the frame did not create a clip mask")
	}
	b.Idle()
	if len(b.maskSlots) != 1 {
		t.Fatal("idle discarded the displayed frame's mask")
	}
	b.Build(&scene.Scene{Width: 100, Height: 100}, nil)
	b.Idle()
	runtime.GC()
	if old.Value() != nil {
		t.Fatal("idle retained a mask from the previous view")
	}
	runtime.KeepAlive(&b)
}

// An animating window must not retain a departed page's masks indefinitely.
func TestUnusedClipMasksExpireWhileDrawing(t *testing.T) {
	var b Builder
	old := keepOldMask(&b)
	for range 12 {
		b.Build(&scene.Scene{Width: 100, Height: 100}, nil)
	}
	runtime.GC()
	if old.Value() != nil {
		t.Fatal("new frames kept a mask from a departed page")
	}
	runtime.KeepAlive(&b)
}

func keepOldMask(b *Builder) weak.Pointer[scene.Image] {
	img := scene.NewImageRGBA(100, 100, make([]byte, 100*100*4))
	b.maskSlots = []clipMaskSlot{{image: img, frame: b.frame}}
	return weak.Make(img)
}
