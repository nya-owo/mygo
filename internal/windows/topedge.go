//go:build windows && (amd64 || arm64)

package windows

import (
	"os"
	"sync"
	"unsafe"
)

// A window without a caption, frameless or with a hidden title bar, has no
// border at the top: frameCalcSize keeps the left, right and bottom ones,
// which Windows draws invisible outside the window and which still resize
// it, and gives the top to the client area, where the webview's or the
// surface's window answers HTCLIENT. The top edge is an invisible window
// (WS_EX_NOREDIRECTIONBITMAP) above them, as tall as the resize border,
// along the top: across the whole width of a frameless window, and up to
// the window controls of a hidden title bar. It answers HTTOP, and
// HTTOPLEFT and HTTOPRIGHT near the corners, and hands presses to the
// window, which then resizes from its top edge as one with a caption does.
// Below it, a drag region of the page or of native UI still moves the
// window, as below the top border of other apps' frameless windows.

// resizeCornerSize is how far from a corner the top edge resizes the
// corner, as Chromium's frames do.
const resizeCornerSize = 16

// newTopEdge creates the top edge of a window without a caption. Flyouts,
// which never resize, have none.
func (w *window) newTopEdge() {
	if !w.captionless() || w.opts.Flyout != nil {
		return
	}
	e := createWindow(wsExLayered|wsExNoRedirect, captionClass, "", wsChild|wsClipSiblings, 0, 0, 0, 0, w.hwnd)
	if e == 0 {
		return
	}
	// A layered window shows, and takes the mouse, once it has attributes;
	// this one has no pixels to show.
	procSetLayeredWindowAttributes.Call(e, 0, 255, lwaAlpha)
	w.topEdge = e
	w.b.topEdges[e] = w
	if !clientFrame() {
		return
	}
	for i := range w.sideEdges {
		s := createWindow(wsExLayered|wsExNoRedirect, captionClass, "", wsChild|wsClipSiblings, 0, 0, 0, 0, w.hwnd)
		if s == 0 {
			continue
		}
		procSetLayeredWindowAttributes.Call(s, 0, 255, lwaAlpha)
		w.sideEdges[i] = s
		w.b.topEdges[s] = w
	}
}

// clientFrame reports whether a window without a caption gives the client
// area its left, right and bottom borders too, which edges like the top
// one then resize from. On Windows 10 DWM drew those borders in the
// appearance, white or black, beside the content, which changed color
// apart from them: a page in the cover's colors, or the content turned
// dark before DWM drew the frame again. MYGO_CLIENT_FRAME=1 has Windows
// 11 do the same, to try it there.
var clientFrame = sync.OnceValue(func() bool {
	return !windows11() || os.Getenv("MYGO_CLIENT_FRAME") == "1"
})

// sideBorder is how wide the left, right and bottom edges are over the
// content: narrower than the frame, so that they take less of what is at
// the window's edges, a scroll bar among them.
func sideBorder(dpi int) int32 { return toPx(5, dpi) }

// layoutTopEdge puts the top edge along the top of the window, above the
// webview or the surface, up to the window controls, or hides it while
// nothing resizes the window. It runs when the window changes size, state
// or DPI, when the webview appears, and when the window controls change.
func (w *window) layoutTopEdge() {
	if w.topEdge == 0 || w.IsMinimized() {
		return
	}
	dpi := dpiOf(w.hwnd)
	var r rect
	procGetClientRect.Call(w.hwnd, uintptr(unsafe.Pointer(&r)))
	width := r.Right
	if w.caption != nil {
		width -= toPx(captionButtonWidth, dpi) * int32(len(w.shownButtons()))
	}
	show := uintptr(swpShowWindow)
	if w.fullScreen || w.IsMaximized() || !w.hasStyle(wsThickFrame) {
		show = swpHideWindow
	}
	// HWND_TOP (0): above the webview's window and the surface's.
	procSetWindowPos.Call(w.topEdge, 0, 0, 0, uintptr(max(width, 0)), uintptr(resizeBorder(dpi)), swpNoActivate|show)
	b := sideBorder(dpi)
	at := [3]rect{
		{Left: 0, Top: 0, Right: b, Bottom: r.Bottom},
		{Left: r.Right - b, Top: 0, Right: r.Right, Bottom: r.Bottom},
		{Left: 0, Top: r.Bottom - b, Right: r.Right, Bottom: r.Bottom},
	}
	for i, e := range w.sideEdges {
		if e == 0 {
			continue
		}
		p := at[i]
		procSetWindowPos.Call(e, 0, uintptr(p.Left), uintptr(p.Top), uintptr(max(p.Right-p.Left, 0)), uintptr(max(p.Bottom-p.Top, 0)), swpNoActivate|show)
	}
}

// topEdgeMessage handles a message of the top edge's window; ok false lets
// DefWindowProc run.
func (w *window) topEdgeMessage(hwnd uintptr, m uint32, wp, lp uintptr) (uintptr, bool) {
	switch m {
	case wmNCHitTest:
		pt := point{getXParam(lp), getYParam(lp)}
		procScreenToClient.Call(w.hwnd, uintptr(unsafe.Pointer(&pt)))
		for i, e := range w.sideEdges {
			if e == hwnd {
				var client rect
				procGetClientRect.Call(w.hwnd, uintptr(unsafe.Pointer(&client)))
				return sideEdgeHit(i, pt.X, pt.Y, client.Right, client.Bottom, toPx(resizeCornerSize, dpiOf(w.hwnd))), true
			}
		}
		var edge, client rect
		procGetClientRect.Call(w.topEdge, uintptr(unsafe.Pointer(&edge)))
		procGetClientRect.Call(w.hwnd, uintptr(unsafe.Pointer(&client)))
		return topEdgeHit(pt.X, edge.Right, client.Right, toPx(resizeCornerSize, dpiOf(w.hwnd))), true
	case wmNCLButtonDown, wmNCLButtonDblClk:
		// The window resizes from its top edge, or a double click
		// stretches it to the height of the screen, as from a caption.
		r, _, _ := procSendMessageW.Call(w.hwnd, uintptr(m), wp, lp)
		return r, true
	}
	return 0, false
}

// topEdgeHit returns the hit-test code at x, in pixels from the left of
// the window's client area, on a top edge edgeWidth pixels wide in a
// client area clientWidth wide: the corner within corner pixels of the
// left, or of the right where the edge reaches it (no window controls
// there), and the top elsewhere.
func topEdgeHit(x, edgeWidth, clientWidth, corner int32) uintptr {
	switch {
	case x < corner:
		return htTopLeft
	case edgeWidth >= clientWidth && x >= clientWidth-corner:
		return htTopRight
	}
	return htTop
}

// sideEdgeHit returns the hit-test code at x, y, in pixels of the client
// area width by height, on the left (0), right (1) or bottom (2) edge: the
// corners within corner pixels of them, the side elsewhere.
func sideEdgeHit(side int, x, y, width, height, corner int32) uintptr {
	switch side {
	case 0, 1:
		top, bottom, mid := uintptr(htTopLeft), uintptr(htBottomLeft), uintptr(htLeft)
		if side == 1 {
			top, bottom, mid = htTopRight, htBottomRight, htRight
		}
		switch {
		case y < corner:
			return top
		case y >= height-corner:
			return bottom
		}
		return mid
	}
	switch {
	case x < corner:
		return htBottomLeft
	case x >= width-corner:
		return htBottomRight
	}
	return htBottom
}

// resizeBorder is the height of the band along the top edge that resizes the
// window: the thickness of its sizing frame, as above a caption.
func resizeBorder(dpi int) int32 {
	if has(procGetSystemMetricsForDpi) {
		frame, _, _ := procGetSystemMetricsForDpi.Call(smCyFrame, uintptr(dpi))
		padding, _, _ := procGetSystemMetricsForDpi.Call(smCxPaddedBorder, uintptr(dpi))
		if frame+padding > 0 {
			return int32(frame + padding)
		}
	}
	return toPx(8, dpi)
}
