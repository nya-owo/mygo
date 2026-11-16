//go:build windows && (amd64 || arm64)

package ui

import "github.com/egoist/mygo/internal/scene"

func (p *Painter) msdfAvailable(a *scene.Atlas) bool {
	h, ok := p.rt.host.(interface{ prepareMSDF(*scene.Atlas) bool })
	return ok && h.prepareMSDF(a)
}

func (h *windowHost) prepareMSDF(a *scene.Atlas) bool {
	r, ok := h.gpu.(interface{ PrepareMSDF(*scene.Atlas) bool })
	return ok && r.PrepareMSDF(a)
}
