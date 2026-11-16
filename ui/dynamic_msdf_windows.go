//go:build windows && amd64

package ui

import (
	"crypto/sha256"
	"fmt"
	"math"
	"os"
	"sync/atomic"
	"unsafe"

	"github.com/egoist/mygo/internal/scene"
	"github.com/egoist/mygo/internal/text"
)

type distanceKey [32]byte
type outlineKey struct {
	font  uint64
	glyph uint32
}
type outlineValue struct {
	segments []text.OutlineSegment
	fill     uint32
	key      distanceKey
	used     uint64
	reserve  int
}
type distanceValue struct {
	page    *scene.Atlas
	metrics text.DistanceGlyph
	used    uint64
}
type distanceJob struct {
	key      distanceKey
	segments []text.OutlineSegment
	fill     uint32
	epoch    uint64
}

// DynamicMSDF is UI-owned, bounded immutable glyph pages plus one generation
// worker. No worker ever sees a Font, TextLayout, Painter, Context or COM object.
type DynamicMSDF struct {
	bridge              *text.NativeDistanceBridge
	post                func(func())
	jobs                chan distanceJob
	closed              atomic.Bool
	epoch               atomic.Uint64
	song                int64
	scope               string
	clock               uint64
	captureBudget       int
	outlines            map[outlineKey]outlineValue
	ready               map[distanceKey]distanceValue
	pending             map[distanceKey]bool
	failed              map[distanceKey]bool
	rejected            map[outlineKey]bool
	wanted              map[distanceKey]uint64
	prefetch            bool
	outlineBytes, bytes int
}

// OpenDynamicMSDF performs disk/library work before publication, never in paint.
// post must asynchronously enqueue fn on the owning MyGo UI/main thread. It
// must not execute fn inline on the generation worker; UI state is thread-owned.
func OpenDynamicMSDF(path, expectedSHA string, post func(func())) (*DynamicMSDF, error) {
	if post == nil {
		return nil, fmt.Errorf("dynamic MSDF requires a non-nil UI post callback")
	}
	data, e := os.ReadFile(path)
	if e != nil {
		return nil, e
	}
	if fmt.Sprintf("%X", sha256.Sum256(data)) != expectedSHA {
		return nil, fmt.Errorf("distance DLL identity mismatch")
	}
	bridge, e := text.OpenNativeDistanceBridge(path)
	if e != nil {
		return nil, e
	}
	d := &DynamicMSDF{bridge: bridge, post: post, jobs: make(chan distanceJob, 32), outlines: map[outlineKey]outlineValue{}, ready: map[distanceKey]distanceValue{}, pending: map[distanceKey]bool{}, failed: map[distanceKey]bool{}}
	d.epoch.Store(1)
	d.rejected = map[outlineKey]bool{}
	d.wanted = map[distanceKey]uint64{}
	go d.work()
	return d, nil
}
func (d *DynamicMSDF) work() {
	defer d.bridge.Close()
	for j := range d.jobs {
		if d.closed.Load() || d.epoch.Load() != j.epoch {
			continue
		}
		metrics, pix, e := d.bridge.Generate(j.segments, j.fill)
		if d.closed.Load() {
			continue
		}
		d.post(func() {
			if d.closed.Load() || d.epoch.Load() != j.epoch {
				return
			}
			delete(d.pending, j.key)
			if e != nil {
				delete(d.wanted, j.key)
				if len(d.failed) >= 128 {
					clear(d.failed)
				}
				d.failed[j.key] = true
				return
			}
			for d.bytes+len(pix) > 16<<20 || len(d.ready) >= 384 {
				if d.wanted[j.key] == 0 && !d.canEvictPrefetch() {
					delete(d.wanted, j.key)
					return
				}
				if !d.evict() {
					delete(d.wanted, j.key)
					return
				}
			}
			var page *scene.Atlas
			if len(pix) > 0 {
				page = scene.NewAtlas(4, int(metrics.Width), int(metrics.Height))
				page.Put(0, 0, int(metrics.Width), int(metrics.Height), pix, int(metrics.Width)*4)
			}
			d.ready[j.key] = distanceValue{page, metrics, d.wanted[j.key]}
			d.bytes += len(pix)
		})
	}
}
func (d *DynamicMSDF) canEvictPrefetch() bool {
	for _, v := range d.ready {
		if v.used == 0 || (d.clock > 8 && v.used < d.clock-8) {
			return true
		}
	}
	return false
}
func (d *DynamicMSDF) evict() bool {
	var key distanceKey
	var oldest uint64 = ^uint64(0)
	found := false
	for k, v := range d.ready {
		if v.used < oldest {
			key, oldest, found = k, v.used, true
		}
	}
	if !found {
		return false
	}
	v := d.ready[key]
	if v.page != nil {
		d.bytes -= len(v.page.Pix)
	}
	delete(d.ready, key)
	delete(d.wanted, key)
	return true
}

// BeginFrame cancels obsolete song work; normalized glyph pages survive DPI/
// scale changes and remain keyed by the exact captured outline, not a GID alone.
func (d *DynamicMSDF) BeginFrame(song int64) {
	d.BeginFrameFor(song, d.scope)
}
func (d *DynamicMSDF) BeginFrameFor(song int64, scope string) {
	if d.closed.Load() {
		return
	}
	d.clock++
	d.captureBudget = 8
	if song != d.song || scope != d.scope {
		d.song = song
		d.scope = scope
		d.epoch.Add(1)
		clear(d.pending)
		clear(d.failed)
		clear(d.rejected)
		clear(d.wanted)
		for key, v := range d.ready {
			v.used = 0
			d.ready[key] = v
		}
	}
}
func (d *DynamicMSDF) Suspend() {
	if !d.closed.Load() && len(d.pending) > 0 {
		d.epoch.Add(1)
		for key := range d.pending {
			delete(d.wanted, key)
		}
		clear(d.pending)
	}
}
func (d *DynamicMSDF) Close() {
	if d.closed.Swap(true) {
		return
	}
	close(d.jobs)
	clear(d.ready)
	clear(d.outlines)
	clear(d.pending)
	clear(d.failed)
	clear(d.rejected)
	clear(d.wanted)
	d.bytes, d.outlineBytes = 0, 0
}

func (d *DynamicMSDF) request(sys *text.System, f *text.Font, gid uint32) (distanceValue, bool) {
	if d.closed.Load() || gid == 0 || f == nil {
		return distanceValue{}, false
	}
	id := sys.DistanceFontID(f)
	key := outlineKey{id, gid}
	if d.rejected[key] {
		return distanceValue{}, false
	}
	v, ok := d.outlines[key]
	if !ok {
		if d.captureBudget <= 0 {
			return distanceValue{}, false
		}
		d.captureBudget--
		segments, fill, e := sys.CaptureDistanceOutline(d.bridge, f, gid)
		if e != nil {
			if len(d.rejected) >= 1024 {
				clear(d.rejected)
			}
			d.rejected[key] = true
			return distanceValue{}, false
		}
		var hash distanceKey
		if len(segments) > 0 {
			raw := unsafe.Slice((*byte)(unsafe.Pointer(&segments[0])), len(segments)*28)
			hash = sha256.Sum256(append([]byte{byte(fill)}, raw...))
		} else {
			hash = sha256.Sum256([]byte("empty-outline"))
		}
		size := len(segments) * 28
		for d.outlineBytes+size > 4<<20 || len(d.outlines) >= 1024 {
			var k outlineKey
			oldest := ^uint64(0)
			found := false
			for x, y := range d.outlines {
				if y.used < oldest {
					k, oldest, found = x, y.used, true
				}
			}
			if !found {
				return distanceValue{}, false
			}
			d.outlineBytes -= len(d.outlines[k].segments) * 28
			delete(d.outlines, k)
		}
		minX, minY, maxX, maxY := float32(0), float32(0), float32(0), float32(0)
		for _, s := range segments {
			n := 1
			if s.Kind == 3 {
				n = 3
			}
			if s.Kind == 4 {
				continue
			}
			xs, ys := [3]float32{s.X0, s.X1, s.X2}, [3]float32{s.Y0, s.Y1, s.Y2}
			for i := 0; i < n; i++ {
				minX = min(minX, xs[i])
				maxX = max(maxX, xs[i])
				minY = min(minY, ys[i])
				maxY = max(maxY, ys[i])
			}
		}
		reserve := min(512, int(math.Ceil(float64(maxX-minX)*128))+10) * min(512, int(math.Ceil(float64(maxY-minY)*128))+10) * 4
		if len(segments) == 0 {
			reserve = 0
		}
		v = outlineValue{segments: segments, fill: fill, key: hash, used: d.clock, reserve: reserve}
		d.outlines[key] = v
		d.outlineBytes += size
	} else {
		v.used = d.clock
		d.outlines[key] = v
	}
	if ready, ok := d.ready[v.key]; ok {
		if !d.prefetch {
			ready.used = d.clock
			d.wanted[v.key] = d.clock
		}
		d.ready[v.key] = ready
		return ready, true
	}
	if !d.pending[v.key] && !d.failed[v.key] && len(d.pending) < 32 {
		if d.prefetch && (d.bytes+v.reserve > 16<<20 || len(d.ready) >= 384) && !d.canEvictPrefetch() {
			return distanceValue{}, false
		}
		select {
		case d.jobs <- distanceJob{v.key, v.segments, v.fill, d.epoch.Load()}:
			d.pending[v.key] = true
		default:
		}
	}
	if !d.prefetch && d.pending[v.key] {
		d.wanted[v.key] = d.clock
	}
	return distanceValue{}, false
}

// Preload captures bounded real resolved requests for the current/next lyric.
func (d *DynamicMSDF) Preload(l *TextLayout) {
	if l == nil {
		return
	}
	d.prefetch = true
	defer func() { d.prefetch = false }()
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, line := range l.l.Lines {
		for _, g := range line.Glyphs {
			if !g.Object {
				d.request(text.Shared(), g.Font, g.ID)
			}
		}
	}
}

// DynamicMSDFTextLayout preflights the whole layout before changing the Scene.
// Pages are immutable and referenced per glyph; mixed faces/pages can coexist.
func (p *Painter) DynamicMSDFTextLayout(d *DynamicMSDF, l *TextLayout, x, y float32, c Color) bool {
	if d == nil || l == nil || d.closed.Load() || p.transform.Perspective() || p.inlineTransforms() {
		return false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.spans) != 0 {
		return false
	}
	values := []distanceValue{}
	all := true
	for _, line := range l.l.Lines {
		for _, g := range line.Glyphs {
			if g.Object {
				all = false
				continue
			}
			v, ok := d.request(p.rt.text, g.Font, g.ID)
			if !ok {
				all = false
			}
			values = append(values, v)
		}
	}
	if !all || len(values) == 0 {
		return false
	}
	for _, v := range values {
		if v.page != nil && !p.msdfAvailable(v.page) {
			return false
		}
	}
	effective := c.Alpha(p.opacity)
	start := int32(len(p.s.Glyphs))
	s := p.scale
	i := 0
	for _, line := range l.l.Lines {
		baseline := p.rt.text.Baseline((y + line.Baseline) * s)
		for _, g := range line.Glyphs {
			v := values[i]
			i++
			if v.page == nil {
				continue
			}
			m := v.metrics
			f := g.Font.Size * s
			pen := p.rt.text.DistanceFieldPen(g.Font, s, (x+g.X)*s)
			p.s.Glyphs = append(p.s.Glyphs, scene.Glyph{X: pen + m.Left*f, Y: baseline + (g.Y-line.Baseline)*s - m.Top*f, W: (m.Right - m.Left) * f, H: (m.Top - m.Bottom) * f, U: 0, V: 0, UW: uint16(m.Width - 1), VH: uint16(m.Height - 1), Color: effective.scene(), Wide: p.glyphWide(effective), MSDF: true, MSDFPage: v.page})
		}
	}
	if int32(len(p.s.Glyphs)) > start {
		p.add(scene.Op{Kind: scene.OpGlyphs, Start: start, End: int32(len(p.s.Glyphs))})
	}
	return true
}
