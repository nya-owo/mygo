package scene

// MSDFDependency snapshots an atlas resource. As Atlas, it is main-thread only.
// It never resolves a global font/page pointer: each scene owns its snapshot.
type MSDFDependency struct {
	Atlas               *Atlas
	Generation, Version uint64
}

// Old display-list storage may retain unused glyphs and page pointers after
// paintPart replaces an op. Only glyphs actually referenced by live ops useSRV.
func (s *Scene) UsesMSDF() bool {
	if s == nil {
		return false
	}
	for _, op := range s.Ops {
		if op.Kind == OpGlyphs {
			for _, g := range s.Glyphs[op.Start:op.End] {
				if g.MSDF {
					return true
				}
			}
		}
	}
	return false
}

// MSDFResources is the resource closure of exactly the canvas graph that the
// fixed renderer draws, with pointer identity preserved and duplicates removed.
func (s *Scene) MSDFResources() []*Atlas {
	if s == nil {
		return nil
	}
	var out []*Atlas
	seen := map[*Atlas]bool{}
	add := func(sc *Scene) {
		for _, op := range sc.Ops {
			if op.Kind != OpGlyphs {
				continue
			}
			for _, g := range sc.Glyphs[op.Start:op.End] {
				if !g.MSDF {
					continue
				}
				a := g.MSDFPage
				if a == nil {
					a = sc.MSDFAtlas
				}
				if a != nil && !seen[a] {
					seen[a] = true
					out = append(out, a)
				}
			}
		}
	}
	add(s)
	s.EachCanvas(func(im *Image) { add(im.Scene) })
	return out
}

// Version's resource epoch makes the existing parent Differ and canvas GPU
// version cache observe invalidation even when Canvas.drawn/Changed was not
// called. There is no unconditional per-frame version bump or canvas repaint.
func (m *Image) refreshMSDFDependencies() {
	var resources []*Atlas
	if m.Scene != nil {
		resources = m.Scene.MSDFResources()
	}
	same := len(resources) == len(m.msdfDeps)
	if same {
		for i, a := range resources {
			if m.msdfDeps[i] != (MSDFDependency{a, a.Generation(), a.Version()}) {
				same = false
				break
			}
		}
	}
	if same {
		return
	}
	clear(m.msdfDeps)
	m.msdfDeps = m.msdfDeps[:0]
	for _, a := range resources {
		m.msdfDeps = append(m.msdfDeps, MSDFDependency{a, a.Generation(), a.Version()})
	}
	m.version++
}
