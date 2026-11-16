package d3d11

import (
	"github.com/egoist/mygo/internal/scene"
	"unsafe"
)

func (r *Renderer) unbindMSDF() {
	if r.ctx != 0 {
		none := uintptr(0)
		call(r.ctx, ctxPSSetShaderResources, 11, 1, uintptr(unsafe.Pointer(&none)))
	}
}

// Map entries own GPU pages; msdf is a non-owning observation alias.
func (r *Renderer) syncMSDFReference(s *scene.Scene) error {
	a := s.MSDFAtlas
	if !s.UsesMSDF() {
		a = nil
	}
	if a == nil {
		r.unbindMSDF()
		r.msdf = texture{}
		r.msdfSource = nil
		return nil
	}
	if r.msdfPages == nil {
		r.msdfPages = map[*scene.Atlas]*texture{}
	}
	t := r.msdfPages[a]
	if t == nil {
		t = &texture{}
		r.msdfPages[a] = t
	}
	if t.gen != a.Generation() || t.w != a.W || t.h != a.H {
		r.unbindMSDF()
	}
	if e := r.syncAtlas(t, a, formatR8G8B8A8Unorm); e != nil {
		return e
	}
	r.msdf = *t
	r.msdfSource = a
	return nil
}
func (r *Renderer) retainMSDFPages(s *scene.Scene) {
	live := map[*scene.Atlas]bool{}
	for _, a := range s.MSDFResources() {
		live[a] = true
	}
	for a, t := range r.msdfPages {
		if !live[a] {
			r.unbindMSDF()
			free(&t.srv)
			free(&t.tex)
			delete(r.msdfPages, a)
			if r.msdfSource == a {
				r.msdf = texture{}
				r.msdfSource = nil
			}
		}
	}
	if len(live) == 0 {
		r.unbindMSDF()
		r.msdf = texture{}
		r.msdfSource = nil
	}
}
func (r *Renderer) releaseMSDFPages() {
	r.unbindMSDF()
	for a, t := range r.msdfPages {
		free(&t.srv)
		free(&t.tex)
		delete(r.msdfPages, a)
	}
	r.msdf = texture{}
	r.msdfSource = nil
}
