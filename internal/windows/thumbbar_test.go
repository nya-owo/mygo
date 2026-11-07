//go:build windows && (amd64 || arm64)

package windows

import (
	"testing"
	"unsafe"
)

// thumbButton is laid out as THUMBBUTTON on the 64-bit targets: hIcon at
// 16, szTip at 24, dwFlags at 544, 552 bytes in all.
func TestThumbButtonLayout(t *testing.T) {
	var b thumbButton
	if s, i, tip, f := unsafe.Sizeof(b), unsafe.Offsetof(b.icon), unsafe.Offsetof(b.tip), unsafe.Offsetof(b.flags); s != 552 || i != 16 || tip != 24 || f != 544 {
		t.Errorf("size %d, hIcon at %d, szTip at %d, dwFlags at %d", s, i, tip, f)
	}
}
