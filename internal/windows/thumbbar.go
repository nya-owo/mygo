//go:build windows && (amd64 || arm64)

package windows

import (
	"unsafe"

	"github.com/egoist/mygo/internal/platform"
)

// The toolbar under a window's taskbar thumbnail (Window.SetThumbButtons).
// Its buttons can only be added once per taskbar button, and their number
// is fixed then: all seven are added, those not used hidden, so that a
// later call with another number of buttons only updates them. Explorer
// makes a new taskbar button when it restarts, or when the window comes
// back to the taskbar (SetSkipTaskbar), and the buttons are added again.

// ITaskbarList3 vtable indices (ShObjIdl_core.h).
const (
	tbThumbBarAddButtons    = 15
	tbThumbBarUpdateButtons = 16
)

// THUMBBUTTON masks and flags, and the notification of a click, which
// comes in WM_COMMAND's high word with the button's id in the low one.
const (
	thbIcon    = 0x2
	thbTooltip = 0x4
	thbFlags   = 0x8

	thbfDisabled = 0x1
	thbfHidden   = 0x8

	thbnClicked = 0x1800

	// thumbIDBase is the id of the first button.
	thumbIDBase = 0xFF00
)

// thumbButton is THUMBBUTTON: its layout is that of the C struct on the
// 64-bit targets, hIcon aligned to 8 bytes.
type thumbButton struct {
	mask   uint32
	id     uint32
	bitmap uint32
	icon   uintptr
	tip    [260]uint16
	flags  uint32
}

func (w *window) SetThumbButtons(buttons []platform.ThumbButton) {
	old := w.thumbIcons
	w.thumbs = append([]platform.ThumbButton(nil), buttons...)
	w.thumbIcons = make([]uintptr, len(buttons))
	size := smallIconSize(w.hwnd)
	for i, b := range buttons {
		if len(b.Icon) > 0 {
			if icon, err := iconFromPNG(b.Icon, size); err == nil {
				w.thumbIcons[i] = icon
			}
		}
	}
	w.applyThumbs()
	// The old icons go once the taskbar has the new ones.
	destroyIcons(old)
}

// applyThumbs puts the buttons on the toolbar once the taskbar button is
// there.
func (w *window) applyThumbs() {
	if !w.buttonCreated || w.skipTaskbar || (len(w.thumbs) == 0 && !w.thumbsAdded) {
		return
	}
	tl := w.b.taskbar()
	if tl == 0 {
		return
	}
	var bs [platform.MaxThumbButtons]thumbButton
	for i := range bs {
		b := &bs[i]
		b.mask = thbIcon | thbTooltip | thbFlags
		b.id = thumbIDBase + uint32(i)
		if i >= len(w.thumbs) {
			b.flags = thbfHidden
			continue
		}
		b.icon = w.thumbIcons[i]
		copyUTF16(b.tip[:], w.thumbs[i].Tooltip)
		if w.thumbs[i].Disabled {
			b.flags = thbfDisabled
		}
	}
	n, p := uintptr(len(bs)), uintptr(unsafe.Pointer(&bs[0]))
	if !w.thumbsAdded && !failed(comCall(tl, tbThumbBarAddButtons, w.hwnd, n, p)) {
		w.thumbsAdded = true
		return
	}
	// Added already, as when Explorer kept them through a call that did
	// not know: an update does.
	if !failed(comCall(tl, tbThumbBarUpdateButtons, w.hwnd, n, p)) {
		w.thumbsAdded = true
	}
}

// thumbClicked reports the click of a thumbnail button, from WM_COMMAND.
func (w *window) thumbClicked(wp uintptr) bool {
	if hiword(wp) != thbnClicked {
		return false
	}
	i := int(loword(wp)) - thumbIDBase
	if i >= 0 && i < len(w.thumbs) {
		w.h.ThumbButtonClicked(i)
	}
	return true
}

// smallIconSize is the size of a small icon at the window's DPI.
func smallIconSize(hwnd uintptr) int {
	const smCxSmIcon = 49
	dpi := uintptr(dpiOf(hwnd))
	if has(procGetSystemMetricsForDpi) {
		if n, _, _ := procGetSystemMetricsForDpi.Call(smCxSmIcon, dpi); n != 0 {
			return int(n)
		}
	}
	return 16 * int(dpi) / 96
}

func destroyIcons(icons []uintptr) {
	for _, icon := range icons {
		if icon != 0 {
			procDestroyIcon.Call(icon)
		}
	}
}
