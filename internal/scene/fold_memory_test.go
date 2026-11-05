package scene

import (
	"runtime"
	"testing"
	"weak"
)

// Leaving a translated scene used to retain its old images in the folder.
func TestFoldReleasesPreviousImages(t *testing.T) {
	for _, translated := range []bool{false, true} {
		t.Run(map[bool]string{false: "no-translation", true: "shorter-translation"}[translated], func(t *testing.T) {
			var f TranslationFolder
			old := foldImage(&f)
			next := &Scene{Width: 100, Height: 100}
			if translated {
				next.Ops = []Op{{Kind: OpFill, Rect: Rect{W: 10, H: 10}, Transform: Translation(1, 1)}}
			}
			f.fold(next)
			runtime.GC()
			if old.Value() != nil {
				t.Fatal("the previous scene retained an image after it disappeared")
			}
			runtime.KeepAlive(&f)
		})
	}
}

func foldImage(f *TranslationFolder) weak.Pointer[Image] {
	img := NewImageRGBA(512, 512, make([]byte, 512*512*4))
	f.fold(&Scene{Width: 100, Height: 100, Ops: []Op{
		{Kind: OpFill, Rect: Rect{W: 10, H: 10}, Transform: Translation(1, 1)},
		{Kind: OpImage, Rect: Rect{W: 20, H: 20}, Image: img},
	}})
	return weak.Make(img)
}
