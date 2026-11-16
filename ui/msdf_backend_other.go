//go:build !windows || (!amd64 && !arm64)

package ui

import "github.com/egoist/mygo/internal/scene"

func (p *Painter) msdfAvailable(a *scene.Atlas) bool { return false }
