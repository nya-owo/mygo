//go:build !windows || !amd64

package ui

import "fmt"

type DynamicMSDF struct{}

func OpenDynamicMSDF(path, sha string, post func(func())) (*DynamicMSDF, error) {
	return nil, fmt.Errorf("dynamic MSDF native bridge is currently windows/amd64 only")
}
func (d *DynamicMSDF) BeginFrame(song int64)                  {}
func (d *DynamicMSDF) BeginFrameFor(song int64, scope string) {}
func (d *DynamicMSDF) Suspend()                               {}
func (d *DynamicMSDF) Preload(l *TextLayout)                  {}
func (d *DynamicMSDF) Close()                                 {}
func (p *Painter) DynamicMSDFTextLayout(d *DynamicMSDF, l *TextLayout, x, y float32, c Color) bool {
	return false
}
