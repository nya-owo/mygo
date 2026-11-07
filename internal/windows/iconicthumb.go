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
	wmDwmSendIconicThumbnail         = 0x0323
	wmDwmSendIconicLivePreviewBitmap = 0x0326

	dwmwaForceIconicRepresentation = 7
	dwmwaHasIconicBitmap           = 10
)

// iconicImage is the type of window.thumbnail.
type iconicImage = image.Image

var (
	procDwmSetIconicThumbnail      = dwmapi.NewProc("DwmSetIconicThumbnail")
	procDwmInvalidateIconicBitmaps = dwmapi.NewProc("DwmInvalidateIconicBitmaps")
	procDwmSetIconicLivePreview    = dwmapi.NewProc("DwmSetIconicLivePreviewBitmap")
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

// sendLivePreview answers WM_DWMSENDICONICLIVEPREVIEWBITMAP. An iconic
// window owes DWM the bitmap Aero Peek shows as the pointer rests on the
// thumbnail too, else peek shows a blank screen and a spinner: the window
// as it is, which PrintWindow draws with what DirectComposition shows.
// A minimized window has nothing to show, and gives an empty bitmap.
func (w *window) sendLivePreview() {
	var r rect
	procGetWindowRect.Call(w.hwnd, uintptr(unsafe.Pointer(&r)))
	width, height := r.Right-r.Left, r.Bottom-r.Top
	iconic, _, _ := procIsIconic.Call(w.hwnd)
	if iconic != 0 || width <= 0 || height <= 0 {
		width, height = 1, 1
	}
	hdr := bitmapInfoHeader{Width: width, Height: -height, Planes: 1, BitCount: 32}
	hdr.Size = uint32(unsafe.Sizeof(hdr))
	var bits uintptr
	bmp, _, _ := procCreateDIBSection.Call(0, uintptr(unsafe.Pointer(&hdr)), 0, uintptr(unsafe.Pointer(&bits)), 0, 0)
	if bmp == 0 || bits == 0 {
		return
	}
	defer procDeleteObject.Call(bmp)
	if width > 1 {
		dc, _, _ := procCreateCompatibleDC.Call(0)
		old, _, _ := procSelectObject.Call(dc, bmp)
		const pwRenderFullContent = 2
		procPrintWindow.Call(w.hwnd, dc, pwRenderFullContent)
		procSelectObject.Call(dc, old)
		procDeleteDC.Call(dc)
		// GDI leaves alpha 0, which DWM would show as nothing: the visible
		// frame is opaque, the invisible resize border around it clear.
		var f rect
		if failed(dwmGetFrameBounds(w.hwnd, &f)) {
			f = r
		}
		px := unsafe.Slice((*byte)(native(bits)), int(width)*int(height)*4)
		for y := int32(0); y < height; y++ {
			for x := int32(0); x < width; x++ {
				i := (int(y)*int(width) + int(x)) * 4
				if sx, sy := r.Left+x, r.Top+y; sx >= f.Left && sx < f.Right && sy >= f.Top && sy < f.Bottom {
					px[i+3] = 255
				} else {
					px[i], px[i+1], px[i+2], px[i+3] = 0, 0, 0, 0
				}
			}
		}
	}
	procDwmSetIconicLivePreview.Call(w.hwnd, bmp, 0, 0)
}

// dwmGetFrameBounds is the window's visible frame on the screen, without
// the invisible resize border.
func dwmGetFrameBounds(hwnd uintptr, r *rect) uintptr {
	hr, _, _ := procDwmGetWindowAttribute.Call(hwnd, dwmwaExtendedFrameBounds, uintptr(unsafe.Pointer(r)), unsafe.Sizeof(*r))
	return hr
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
