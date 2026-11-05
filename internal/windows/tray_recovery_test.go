//go:build windows && (amd64 || arm64)

package windows

import (
	"testing"
	"time"
)

// A refused registration used to leave added true, suppressing later adds.
func TestTrayAddFailureThenRecovery(t *testing.T) {
	old := notifyTray
	t.Cleanup(func() { notifyTray = old })
	var calls []uintptr
	accept := false
	notifyTray = func(message uintptr, d *notifyIconData) bool {
		calls = append(calls, message)
		return accept
	}
	tr := &tray{b: &Backend{}, id: 7}
	tr.add()
	if tr.added {
		t.Fatal("a failed registration was marked added")
	}
	accept = true
	tr.add()
	if !tr.added || len(calls) != 2 || calls[1] != nimAdd {
		t.Fatal("the icon did not register on retry")
	}
}

// A missing shell icon must be added again when modification fails.
func TestTrayModifyFailureReadds(t *testing.T) {
	old := notifyTray
	t.Cleanup(func() { notifyTray = old })
	var calls []uintptr
	notifyTray = func(message uintptr, d *notifyIconData) bool {
		calls = append(calls, message)
		return message == nimAdd
	}
	tr := &tray{b: &Backend{}, id: 7, added: true}
	tr.update()
	if !tr.added || len(calls) != 2 || calls[0] != nimModify || calls[1] != nimAdd {
		t.Fatal("a missing icon was not registered again")
	}
}

// Bounds queries used to retry on every flyout while a shell retry was pending.
func TestTrayPendingRetryDoesNotFloodShell(t *testing.T) {
	old := notifyTray
	t.Cleanup(func() { notifyTray = old })
	calls := 0
	notifyTray = func(uintptr, *notifyIconData) bool { calls++; return false }
	tr := &tray{b: &Backend{}, retry: time.NewTimer(time.Hour)}
	defer tr.retry.Stop()
	for range 20 {
		tr.add()
	}
	if calls != 0 || tr.added {
		t.Fatal("a pending retry repeatedly called the shell")
	}
}
