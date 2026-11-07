//go:build windows && (amd64 || arm64)

package windows

import (
	"bytes"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"unsafe"

	xdraw "golang.org/x/image/draw"
)

// A window's taskbar thumbnail as an image of the app's (SetThumbnail), as
// media players show the cover of what plays. DWM asks for it with
// WM_DWMSENDICONICTHUMBNAIL, giving the largest size it takes, and the
// window answers with a bitmap that fits.

const (
	wmDwmSendIconicThumbnail = 0x0323

	dwmwaForceIconicRepresentation = 7
	dwmwaHasIconicBitmap           = 10
)

// iconicImage is the type of window.thumbnail.
type iconicImage = image.Image

var (
	procDwmSetIconicThumbnail      = dwmapi.NewProc("DwmSetIconicThumbnail")
	procDwmInvalidateIconicBitmaps = dwmapi.NewProc("DwmInvalidateIconicBitmaps")
)

func (w *window) SetThumbnail(data []byte) {
	var img image.Image
	if len(data) > 0 {
		if m, _, err := image.Decode(bytes.NewReader(data)); err == nil && !m.Bounds().Empty() {
			img = m
		}
	}
	had := w.thumbnail != nil
	w.thumbnail = img
	if !has(procDwmSetIconicThumbnail) {
		return
	}
	if (img != nil) != had {
		on := uint32(0)
		if img != nil {
			on = 1
		}
		procDwmSetWindowAttribute.Call(w.hwnd, dwmwaForceIconicRepresentation, uintptr(unsafe.Pointer(&on)), 4)
		procDwmSetWindowAttribute.Call(w.hwnd, dwmwaHasIconicBitmap, uintptr(unsafe.Pointer(&on)), 4)
	}
	if img != nil {
		// Shown already: DWM asks again.
		procDwmInvalidateIconicBitmaps.Call(w.hwnd)
	}
}

// sendIconicThumbnail answers WM_DWMSENDICONICTHUMBNAIL: lp holds the
// largest width in its high word and height in its low one.
func (w *window) sendIconicThumbnail(lp uintptr) {
	if w.thumbnail == nil {
		return
	}
	bmp := thumbnailBitmap(w.thumbnail, int(hiword(lp)), int(loword(lp)))
	if bmp == 0 {
		return
	}
	procDwmSetIconicThumbnail.Call(w.hwnd, bmp, 0)
	procDeleteObject.Call(bmp)
}

// thumbnailBitmap scales img to fit maxW×maxH, keeping its proportions,
// into a 32-bit top-down DIB section of premultiplied BGRA, as DWM takes.
func thumbnailBitmap(img image.Image, maxW, maxH int) uintptr {
	w, h := fitIn(img.Bounds().Dx(), img.Bounds().Dy(), maxW, maxH)
	if w <= 0 || h <= 0 {
		return 0
	}
	rgba := image.NewRGBA(image.Rect(0, 0, w, h)) // premultiplied
	xdraw.CatmullRom.Scale(rgba, rgba.Bounds(), img, img.Bounds(), xdraw.Src, nil)
	hdr := bitmapInfoHeader{Width: int32(w), Height: -int32(h), Planes: 1, BitCount: 32}
	hdr.Size = uint32(unsafe.Sizeof(hdr))
	var bits uintptr
	bmp, _, _ := procCreateDIBSection.Call(0, uintptr(unsafe.Pointer(&hdr)), 0, uintptr(unsafe.Pointer(&bits)), 0, 0)
	if bmp == 0 || bits == 0 {
		return 0
	}
	dst := unsafe.Slice((*byte)(native(bits)), w*h*4)
	for i := 0; i < len(dst); i += 4 {
		dst[i], dst[i+1], dst[i+2], dst[i+3] = rgba.Pix[i+2], rgba.Pix[i+1], rgba.Pix[i], rgba.Pix[i+3]
	}
	return bmp
}

// fitIn is the largest size of w×h's proportions within maxW×maxH.
func fitIn(w, h, maxW, maxH int) (int, int) {
	if w <= 0 || h <= 0 || maxW <= 0 || maxH <= 0 {
		return 0, 0
	}
	if w*maxH > h*maxW { // wider than the space: as wide as it
		return maxW, max(h*maxW/w, 1)
	}
	return max(w*maxH/h, 1), maxH
}
