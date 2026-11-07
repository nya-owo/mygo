//go:build windows && (amd64 || arm64)

package windows

import (
	"image"
	"testing"
	"unsafe"
)

// A thumbnail keeps its proportions within the space DWM gives.
func TestFitIn(t *testing.T) {
	for _, c := range []struct{ w, h, mw, mh, ww, wh int }{
		{600, 600, 200, 120, 120, 120}, // a square cover in a wide space
		{600, 600, 120, 200, 120, 120},
		{800, 400, 200, 200, 200, 100},
		{400, 800, 200, 200, 100, 200},
		{10, 10, 0, 100, 0, 0},
	} {
		if w, h := fitIn(c.w, c.h, c.mw, c.mh); w != c.ww || h != c.wh {
			t.Errorf("fitIn(%d, %d, %d, %d) = %d×%d, want %d×%d", c.w, c.h, c.mw, c.mh, w, h, c.ww, c.wh)
		}
	}
}

// The bitmap holds the image's pixels in BGRA, as DWM reads them.
func TestThumbnailBitmap(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 40, 40))
	for i := range img.Pix {
		img.Pix[i] = []byte{200, 30, 10, 255}[i%4] // red
	}
	bmp := thumbnailBitmap(img, 20, 50)
	if bmp == 0 {
		t.Fatal("no bitmap")
	}
	defer procDeleteObject.Call(bmp)
	var bm struct {
		typ, width, height, widthBytes int32
		planes, bitsPixel              uint16
		bits                           uintptr
	}
	procGetObjectW := gdi32.NewProc("GetObjectW")
	procGetObjectW.Call(bmp, unsafe.Sizeof(bm), uintptr(unsafe.Pointer(&bm)))
	if bm.width != 20 || bm.height != 20 || bm.bitsPixel != 32 {
		t.Fatalf("bitmap %d×%d at %d bits", bm.width, bm.height, bm.bitsPixel)
	}
	px := unsafe.Slice((*byte)(native(bm.bits)), 4)
	if px[0] != 10 || px[1] != 30 || px[2] != 200 || px[3] != 255 {
		t.Errorf("first pixel % d, want BGRA 10 30 200 255", px)
	}
}
