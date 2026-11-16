//go:build windows && amd64

package text

import (
	"fmt"
	"runtime"
	"syscall"
	"unsafe"
)

// OutlineSegment is ABI1 POD in em units, original DWrite Y-down coordinates.
type OutlineSegment struct {
	Kind                   uint32
	X0, Y0, X1, Y1, X2, Y2 float32
}
type DistanceGlyph struct {
	Width, Height            uint32
	Left, Bottom, Right, Top float32
}
type NativeDistanceBridge struct {
	dll                *syscall.DLL
	snapshot, generate *syscall.Proc
}

func (s *System) DistanceFontID(f *Font) uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	if f.distanceID == 0 {
		s.distanceSequence++
		f.distanceID = s.distanceSequence
	}
	return f.distanceID
}
func OpenNativeDistanceBridge(path string) (*NativeDistanceBridge, error) {
	d, e := syscall.LoadDLL(path)
	if e != nil {
		return nil, e
	}
	abi, e := d.FindProc("cm_abi")
	if e != nil {
		d.Release()
		return nil, e
	}
	v, _, _ := abi.Call()
	if v != 1 {
		d.Release()
		return nil, fmt.Errorf("unsupported distance ABI %d", v)
	}
	snap, e := d.FindProc("cm_snapshot")
	if e != nil {
		d.Release()
		return nil, e
	}
	gen, e := d.FindProc("cm_generate")
	if e != nil {
		d.Release()
		return nil, e
	}
	return &NativeDistanceBridge{d, snap, gen}, nil
}
func (b *NativeDistanceBridge) Close() {
	if b.dll != nil {
		b.dll.Release()
		b.dll = nil
	}
}

// Capture is called only with a currently engine-owned Font on the UI thread.
func (s *System) CaptureDistanceOutline(b *NativeDistanceBridge, f *Font, gid uint32) ([]OutlineSegment, uint32, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var count, fill uint32
	if f == nil || f.native == 0 {
		return nil, 0, fmt.Errorf("invalid face")
	}
	if engine, ok := s.engine().(*dwrite); !ok || engine.isColor(f.native) {
		return nil, 0, fmt.Errorf("native color/unsupported face")
	}
	v, _, _ := b.snapshot.Call(f.native, uintptr(gid), 0, 0, uintptr(unsafe.Pointer(&count)), uintptr(unsafe.Pointer(&fill)))
	if int32(v) != 1 && int32(v) != 0 {
		return nil, 0, fmt.Errorf("outline snapshot: %d", int32(v))
	}
	if count == 0 {
		return nil, fill, nil
	}
	if count > 8192 {
		return nil, 0, fmt.Errorf("outline too large")
	}
	segments := make([]OutlineSegment, count)
	v, _, _ = b.snapshot.Call(f.native, uintptr(gid), uintptr(unsafe.Pointer(&segments[0])), uintptr(count), uintptr(unsafe.Pointer(&count)), uintptr(unsafe.Pointer(&fill)))
	runtime.KeepAlive(f)
	if int32(v) != 0 {
		return nil, 0, fmt.Errorf("outline snapshot: %d", int32(v))
	}
	return segments, fill, nil
}

// Generate accepts no COM/font pointers and is safe on one bounded worker.
func (b *NativeDistanceBridge) Generate(segments []OutlineSegment, fill uint32) (DistanceGlyph, []byte, error) {
	var g DistanceGlyph
	if len(segments) == 0 {
		return g, nil, nil
	}
	v, _, _ := b.generate.Call(uintptr(unsafe.Pointer(&segments[0])), uintptr(len(segments)), uintptr(fill), 0, 0, uintptr(unsafe.Pointer(&g)))
	if int32(v) != 1 {
		return g, nil, fmt.Errorf("distance measure: %d", int32(v))
	}
	if g.Width == 0 || g.Height == 0 || g.Width > 512 || g.Height > 512 {
		return g, nil, fmt.Errorf("invalid distance image")
	}
	pix := make([]byte, int(g.Width*g.Height)*4)
	v, _, _ = b.generate.Call(uintptr(unsafe.Pointer(&segments[0])), uintptr(len(segments)), uintptr(fill), uintptr(unsafe.Pointer(&pix[0])), uintptr(len(pix)), uintptr(unsafe.Pointer(&g)))
	runtime.KeepAlive(segments)
	if int32(v) != 0 {
		return g, nil, fmt.Errorf("distance generation: %d", int32(v))
	}
	return g, pix, nil
}
