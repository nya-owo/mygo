//go:build windows && (amd64 || arm64)

package windows

import (
	"image/color"
	"os"
	"testing"
)

// A valid PNG tray handle must draw visible pixels with Windows' icon renderer.
func TestTrayPNGDrawsWithWindows(t *testing.T) {
	data := testIcon(t, 64, 64, color.NRGBA{220, 60, 40, 255}, false).PNG
	if path := os.Getenv("MYGO_TEST_TRAY_PNG"); path != "" {
		var err error
		data, err = os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
	}
	draw := user32.NewProc("DrawIconEx")
	for _, size := range []int{16, 24, 32, 64} {
		icon, err := iconFromPNG(data, size)
		if err != nil {
			t.Fatal(err)
		}
		bmp := newBitmap(0, int32(size), int32(size))
		if bmp == nil {
			procDestroyIcon.Call(icon)
			t.Fatal("no bitmap")
		}
		r, _, err := draw.Call(bmp.dc, 0, 0, icon, uintptr(size), uintptr(size), 0, 0, 3)
		visible := 0
		for _, p := range bmp.px {
			if p != 0 {
				visible++
			}
		}
		bmp.free()
		procDestroyIcon.Call(icon)
		if r == 0 || visible == 0 {
			t.Fatalf("%dpx icon was blank: %v", size, err)
		}
		t.Logf("%dpx: Windows drew %d nonzero pixels", size, visible)
	}
}
