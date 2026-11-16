package text

import "math"

// DistanceFieldPen uses the same fractional pen placement as native glyphs.
func (s *System) DistanceFieldPen(f *Font, dpi, x float32) float32 {
	s.mu.Lock()
	defer s.mu.Unlock()
	q, n, carry := s.place(f, dpi, false, x)
	return float32(math.Floor(float64(x))) + carry + float32(q)/float32(n)
}
