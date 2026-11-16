//go:build windows && amd64

package ui

import (
	"fmt"
	"testing"

	"github.com/egoist/mygo/internal/gpu"
	"github.com/egoist/mygo/internal/scene"
	"github.com/egoist/mygo/internal/text"
)

// msdfOpacityHost admits an already-ready synthetic page; it never starts a GPU or worker.
type msdfOpacityHost struct{ host }

func (msdfOpacityHost) prepareMSDF(*scene.Atlas) bool { return true }

// Dynamic MSDF once passed the original alpha to Wide, letting a wide GPU target ignore Painter opacity.
func TestDynamicMSDFPainterOpacity(t *testing.T) {
	sys := text.Shared()
	layout := ShapeText("M", Font{Size: 24}, 0)
	if len(layout.l.Lines) != 1 || len(layout.l.Lines[0].Glyphs) != 1 {
		t.Fatal("expected one real shaped glyph")
	}
	glyph := layout.l.Lines[0].Glyphs[0]
	if glyph.Font == nil || glyph.ID == 0 {
		t.Fatal("shaped glyph has no usable font or ID")
	}
	wide := Oklch(0.85, 0.3, 145)
	if !wide.wide.ok() {
		t.Fatal("wide input must be outside sRGB")
	}
	for _, colorCase := range []struct {
		name  string
		color Color
	}{{"srgb", RGB(30, 80, 150)}, {"wide", wide}, {"wide-srgb-fallback", wide.SRGB()}} {
		for _, alphaCase := range []struct {
			alpha uint8
			want  [3]uint8
		}{{255, [3]uint8{0, 128, 255}}, {191, [3]uint8{0, 96, 191}}} {
			for i, opacity := range []float32{0, 0.5, 1} {
				t.Run(fmt.Sprintf("%s/source-%d/opacity-%g", colorCase.name, alphaCase.alpha, opacity), func(t *testing.T) {
					color := colorCase.color
					color.A = alphaCase.alpha
					wantA := alphaCase.want[i]
					wantColor := scene.Color{R: color.R, G: color.G, B: color.B, A: wantA}
					page := scene.NewAtlas(4, 8, 8)
					key := distanceKey{1}
					d := &DynamicMSDF{
						clock:    1,
						outlines: map[outlineKey]outlineValue{{sys.DistanceFontID(glyph.Font), glyph.ID}: {key: key}},
						ready:    map[distanceKey]distanceValue{key: {page: page, metrics: text.DistanceGlyph{Width: 8, Height: 8, Right: 1, Top: 1}}},
						wanted:   map[distanceKey]uint64{},
					}
					rt := &engine{text: sys, host: msdfOpacityHost{}}
					s := scene.Scene{Width: 128, Height: 128, Scale: 1}
					p := Painter{rt: rt, s: &s, scale: 1, opacity: opacity, clip: Rect{W: 128, H: 128}}
					if !p.DynamicMSDFTextLayout(d, layout, 10, 10, color) {
						t.Fatal("ready layout unexpectedly fell back")
					}
					if len(s.Glyphs) != 1 || len(s.Ops) != 1 || !s.Glyphs[0].MSDF || s.Glyphs[0].MSDFPage != page {
						t.Fatal("ready layout did not emit its page and glyph op")
					}
					if s.Glyphs[0].Color != wantColor {
						t.Fatalf("Color=%+v, want %+v", s.Glyphs[0].Color, wantColor)
					}
					if !color.wide.ok() || wantA == 0 {
						if s.Glyphs[0].Wide != 0 || len(s.Wide) != 0 {
							t.Fatal("sRGB or fully transparent glyph retained a Wide override")
						}
					} else {
						if s.Glyphs[0].Wide == 0 || len(s.Wide) != 1 {
							t.Fatal("visible wide glyph lost its Wide payload")
						}
						w := s.Wide[s.Glyphs[0].Wide-1]
						r, g, b := color.extended()
						if w.Set != scene.WideColor || w.Color != [4]float32{r, g, b, float32(wantA) / 255} {
							t.Fatalf("Wide=%+v, want original wide RGB and effective alpha %d/255", w, wantA)
						}
					}
					for _, wideTarget := range []bool{false, true} {
						builder := gpu.Builder{Wide: wideTarget, NoFold: true}
						builder.Build(&s, nil)
						if len(builder.Instances) != 1 || len(builder.Batches) != 1 || builder.Batches[0].MSDFPage != page {
							t.Fatal("GPU builder did not retain one MSDF instance/page")
						}
						want := [4]float32{float32(color.R) / 255, float32(color.G) / 255, float32(color.B) / 255, float32(wantA) / 255}
						if wideTarget && color.wide.ok() && wantA > 0 {
							want[0], want[1], want[2] = color.extended()
						}
						in := builder.Instances[0]
						if in.Color != want || in.Params[0] != 3 || in.Params[1] != 1 {
							t.Fatalf("Wide target=%v: GPU color=%v, want %v; params=%v", wideTarget, in.Color, want, in.Params)
						}
						builder.Release()
					}
					// The ordinary native text path remains an independent control for effective alpha.
					native := scene.Scene{Width: 128, Height: 128, Scale: 1}
					p.s = &native
					p.TextLayout(layout, 10, 10, color)
					if len(native.Glyphs) != 1 || native.Glyphs[0].MSDF || native.Glyphs[0].Color != wantColor {
						t.Fatalf("ordinary native text changed: %+v", native.Glyphs)
					}
					if (native.Glyphs[0].Wide == 0) != (s.Glyphs[0].Wide == 0) {
						t.Fatal("native and MSDF paths disagree on Wide presence")
					}
					if native.Glyphs[0].Wide != 0 && native.Wide[native.Glyphs[0].Wide-1] != s.Wide[s.Glyphs[0].Wide-1] {
						t.Fatal("native and MSDF paths disagree on effective Wide color")
					}
					t.Logf("source alpha=%d opacity=%g: Scene and both GPU targets alpha=%d/255; native control agrees", color.A, opacity, wantA)
				})
			}
		}
	}
}
