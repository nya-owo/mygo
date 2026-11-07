package mygo

import "testing"

// The thumbnail toolbar's buttons reach the window, at most seven, and a
// click on one calls its Click.
func TestThumbButtons(t *testing.T) {
	w, fw := testWindow(t, WindowOptions{})
	var clicked []int
	buttons := make([]ThumbButton, 9)
	for i := range buttons {
		buttons[i] = ThumbButton{Tooltip: string(rune('a' + i)), Click: func() { clicked = append(clicked, i) }}
	}
	buttons[1].Disabled = true
	w.SetThumbButtons(buttons)
	got := onMainValue(func() int { return len(fw.ThumbButtons) })
	if got != 7 {
		t.Fatalf("%d buttons, want 7", got)
	}
	if !onMainValue(func() bool { return fw.ThumbButtons[1].Disabled && fw.ThumbButtons[2].Tooltip == "c" }) {
		t.Error("buttons not passed as given")
	}
	onMain(func() { fw.ClickThumbButton(2); fw.ClickThumbButton(8); fw.ClickThumbButton(-1) })
	if onMainValue(func() bool { return len(clicked) != 1 || clicked[0] != 2 }) {
		t.Errorf("clicked %v, want [2]", clicked)
	}
	w.SetThumbButtons(nil)
	onMain(func() { fw.ClickThumbButton(2) })
	if onMainValue(func() bool { return len(fw.ThumbButtons) != 0 || len(clicked) != 1 }) {
		t.Error("buttons not removed")
	}
}

// The thumbnail image reaches the window, and nil takes it away.
func TestThumbnail(t *testing.T) {
	w, fw := testWindow(t, WindowOptions{})
	w.SetThumbnail([]byte("png"))
	if got := onMainValue(func() string { return string(fw.Thumbnail) }); got != "png" {
		t.Errorf("thumbnail %q", got)
	}
	w.SetThumbnail(nil)
	if onMainValue(func() bool { return fw.Thumbnail != nil }) {
		t.Error("thumbnail not removed")
	}
}
