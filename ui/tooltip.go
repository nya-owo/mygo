package ui

import (
	"slices"
	"time"
)

const (
	// tooltipDelay is how long the pointer rests on an element before its
	// tip shows, and tooltipWarm how soon after a tip goes another shows at
	// once, as the pointer moves along a toolbar: Base UI's.
	tooltipDelay = 600 * time.Millisecond
	tooltipWarm  = 400 * time.Millisecond
)

// tooltips is which element's tip shows, which the frame decides once
// (show), from what the last frame left.
type tooltips struct {
	frame uint64
	pass  int
	// shown is the element whose tip shows in the frame, byFocus telling
	// that the keyboard focus brought it rather than the pointer.
	shown   uint64
	byFocus bool
	// hover is the innermost element with a tip under the pointer since
	// hoverSince, focus the one with the keyboard focus since focusSince.
	hover, focus           uint64
	hoverSince, focusSince time.Time
	// dismissed is the element whose tip a press or Escape hid, until
	// neither the pointer nor the focus is on it.
	dismissed uint64
	// posed is the element whose tip shows, since tipMotion brought it
	// in: at once if warmShow.
	posed    uint64
	warmShow bool
	// lastShown is when a tip last showed, or went.
	lastShown time.Time
}

// TooltipBase shows a tip without a look by anchor: fn styles the tip and
// builds its content. It shows once the pointer has rested on anchor for
// 0.6 seconds, at once as anchor gets the keyboard focus from the
// keyboard, and at once too within 0.4 seconds of another tip going, as
// the pointer moves along a toolbar. It goes as the pointer leaves anchor
// or presses it, and with Escape, until the pointer comes back or the
// focus does. Over elements inside one another, the innermost one's tip
// shows; one shows at a time.
//
// The tip goes above anchor, centered, or below it where there is no room
// above; fn may place it elsewhere with AttachTo, as after it:
//
//	ui.TooltipBase(c, b, func(tip ui.Element) {
//		tip.AttachTo(b, ui.AnchorEnd, ui.AnchorStart).MarginStart(6)
//		tip.Padding(4, 8).Radius(6).Background(dark).TextColor(light)
//		tip.Children(func() { ui.Text(c, "Share") })
//	})
//
// The pointer goes through the tip, and assistive technology sees it as a
// tooltip, which it does not read: give anchor a Description to tell what
// the tip does. It returns the tip, or nil while it does not show;
// Element.Tooltip is TooltipBase with the theme's look.
func coreTooltipBase(c *context, anchor *node, fn func(tip *node)) *node {
	tip, _ := tooltipBase(c, anchor, fn)
	return tip
}

// tooltipBase is TooltipBase, which also reports whether the keyboard
// focus brought the tip rather than the pointer.
func tooltipBase(c *context, anchor *node, fn func(tip *node)) (*node, bool) {
	anchor.flags |= flagHover
	anchor.tip = true
	rt := c.rt
	// A press or a click on the anchor, by the pointer, the keys or
	// assistive technology, hides its tip, as Base UI's closeOnClick: a
	// menu button's menu opens as the pointer goes down, without a click.
	if anchor.st.clicks > 0 || slices.ContainsFunc(rt.downs, func(id uint64) bool { return rt.pressedWithin(id, anchor) }) {
		rt.tips.dismissed = anchor.id
	}
	if c.inert || rt.showTooltip(c) != anchor.id || rt.tips.dismissed == anchor.id {
		return nil, false
	}
	var tip *node
	coreOverlay(c, func() {
		// Keyed by its anchor: the tip of another element is another tip,
		// which comes in as its own.
		c.nextKey, c.keySet = tipKey{anchor.id}, true
		tip = coreBox(c).Absolute().PassThrough().Role(RoleTooltip)
		fn(tip)
		if tip.attach == 0 {
			tip.AttachTo(anchor, AnchorTop, AnchorBottom)
		}
		rt.tipMotion(tip, anchor.id)
		if tip.OverlayShortcut(0, KeyEscape) {
			rt.tips.dismissed = anchor.id
		}
	})
	return tip, rt.tips.byFocus
}

// tipKey is the key of the tip of the element of id.
type tipKey struct{ id uint64 }

// How a tip comes and goes: in quickly from a little smaller, as AppKit's
// and SwiftUI's, and out more quickly still. A tip shown as the pointer goes
// on from another, within tooltipWarm, shows at once, as they do.
var (
	tipIn  = Animation{Duration: 120 * time.Millisecond, Ease: EaseOut}
	tipOut = Animation{Duration: 90 * time.Millisecond, Ease: EaseIn}
)

// tipMotion brings tip, the tip of the element of id, in as it first shows,
// and has it fade as it goes.
func (rt *engine) tipMotion(tip *node, id uint64) {
	t := &rt.tips
	first := t.posed != id
	if first {
		// It shows from this frame: at once if another tip just went.
		d := t.hoverSince.Sub(t.lastShown)
		t.warmShow = !t.byFocus && !t.lastShown.IsZero() && d >= 0 && d <= tooltipWarm
	}
	t.posed = id
	tip.Transition(ElementTransition{Animation: &tipOut, Size: true, Exit: &Motion{}})
	if t.warmShow {
		return
	}
	if first {
		tip.AnimatePose(Pose{Fade: 1, Scale: 0.96}, tipIn)
		// A frame to build next, which sends it on its way: a pose at
		// rest asks for none.
		tip.Jump(tipKey{}, 0)
		tip.AnimateTo(tipKey{}, 1, Animation{Duration: time.Millisecond})
		return
	}
	tip.AnimatePose(Pose{}, tipIn)
}

// Tooltip shows s by the element when the pointer rests on it, or the
// keyboard focus comes to it, as TooltipBase does: above the element,
// centered, or below it where there is no room above. It describes the
// element to assistive technology where Description does not.
func (e *node) Tooltip(s string) *node {
	if e.description == "" {
		e.description = s
	}
	if s == "" {
		return e
	}
	c := e.c
	t := c.theme
	fill, text := t.inverse()
	tooltipBase(c, e, func(tip *node) {
		tip.MaxWidth(t.Space(80)).Padding(t.Space(1.25), t.Space(2)).Radius(t.Space(1.25)).
			Background(fill).TextColor(text).FontSize(t.FontSize - 1)
		tip.Shadow(0, 2, 8, 0, RGBA(0, 0, 0, 0.2))
		tip.Children(func() { coreText(c, s) })
		tip.AttachTo(e, AnchorTop, AnchorBottom).Margin(0, 0, t.Space(1.5), 0)
	})
	return e
}

// showTooltip returns the element whose tip shows in the frame, deciding it
// once: the innermost element with a tip under the pointer once it has
// rested there long enough, or the one with the keyboard focus from the
// keyboard, whichever came last.
func (rt *engine) showTooltip(c *context) uint64 {
	t := &rt.tips
	if t.frame == rt.frame && t.pass == rt.pass {
		return t.shown
	}
	t.frame, t.pass = rt.frame, rt.pass
	now := c.now
	if t.shown != 0 {
		// The tip of the last frame showed until now.
		t.lastShown = now
	}
	tipped := func(id uint64) bool {
		s := rt.states[id]
		return s != nil && s.tip && s.flags&flagDisabled == 0
	}
	var hover uint64
	for _, id := range rt.hover {
		if tipped(id) {
			hover = id
			break
		}
	}
	if hover != t.hover {
		t.hover, t.hoverSince = hover, now
	}
	var focus uint64
	if rt.focusVisible && rt.windowFocus() && tipped(rt.focused) {
		focus = rt.focused
	}
	if focus != t.focus {
		t.focus, t.focusSince = focus, now
	}
	if t.dismissed != hover && t.dismissed != focus {
		t.dismissed = 0
	}
	if rt.pressed != nil && hover != 0 {
		t.dismissed = hover
	}
	if hover == t.dismissed {
		hover = 0
	}
	if focus == t.dismissed {
		focus = 0
	}
	t.shown, t.byFocus = 0, false
	defer func() {
		if t.shown == 0 {
			t.posed = 0 // the next to show comes in
		}
	}()
	switch {
	case focus != 0 && (hover == 0 || !t.focusSince.Before(t.hoverSince)):
		t.shown, t.byFocus = focus, true
	case hover != 0:
		warm := !t.lastShown.IsZero() && t.hoverSince.Sub(t.lastShown) <= tooltipWarm
		if wait := tooltipDelay - now.Sub(t.hoverSince); wait > 0 && !warm {
			rt.scheduleAt(now.Add(wait))
			break
		}
		t.shown = hover
	}
	return t.shown
}
