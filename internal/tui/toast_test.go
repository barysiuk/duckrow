package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
)

func TestNewToastModel(t *testing.T) {
	m := newToastModel()
	if m.active {
		t.Error("new toast should not be active")
	}
	if m.message != "" {
		t.Errorf("message = %q, want empty", m.message)
	}
	if m.nextID != 0 {
		t.Errorf("nextID = %d, want 0", m.nextID)
	}
}

func TestToastShow_Success(t *testing.T) {
	m := newToastModel()
	m, cmd := m.show("Installed skill-x", toastSuccess)

	if !m.active {
		t.Error("toast should be active after show")
	}
	if m.message != "Installed skill-x" {
		t.Errorf("message = %q, want %q", m.message, "Installed skill-x")
	}
	if m.kind != toastSuccess {
		t.Errorf("kind = %d, want toastSuccess (%d)", m.kind, toastSuccess)
	}
	if m.id != 0 {
		t.Errorf("id = %d, want 0", m.id)
	}
	if m.nextID != 1 {
		t.Errorf("nextID = %d, want 1", m.nextID)
	}
	if cmd == nil {
		t.Error("show(success) should return a cmd for auto-dismiss timer")
	}
}

func TestToastShow_Error(t *testing.T) {
	m := newToastModel()
	m, cmd := m.show("Error: something broke", toastError)

	if !m.active {
		t.Error("toast should be active after show")
	}
	if m.kind != toastError {
		t.Errorf("kind = %d, want toastError (%d)", m.kind, toastError)
	}
	if cmd == nil {
		t.Error("show(error) should return a cmd for auto-dismiss timer")
	}
}

func TestToastShow_Loading(t *testing.T) {
	m := newToastModel()
	m, cmd := m.show("Adding registry...", toastLoading)

	if !m.active {
		t.Error("toast should be active after show")
	}
	if m.kind != toastLoading {
		t.Errorf("kind = %d, want toastLoading (%d)", m.kind, toastLoading)
	}
	if cmd == nil {
		t.Error("show(loading) should return a cmd for spinner tick")
	}
}

func TestToastDismiss(t *testing.T) {
	m := newToastModel()
	m, _ = m.show("hello", toastSuccess)
	if !m.active {
		t.Fatal("toast should be active before dismiss")
	}

	m = m.dismiss()
	if m.active {
		t.Error("toast should not be active after dismiss")
	}
	if m.message != "" {
		t.Errorf("message = %q, want empty after dismiss", m.message)
	}
}

func TestToastUpdate_DismissMatchingID(t *testing.T) {
	m := newToastModel()
	m, _ = m.show("hello", toastSuccess)
	id := m.id

	m, _ = m.update(toastDismissMsg{id: id})
	if m.active {
		t.Error("toast should be dismissed when ID matches")
	}
}

func TestToastUpdate_DismissStaleID(t *testing.T) {
	m := newToastModel()
	m, _ = m.show("first", toastSuccess)
	staleID := m.id

	// Show a second toast, which gets a new ID.
	m, _ = m.show("second", toastSuccess)
	if m.id == staleID {
		t.Fatal("second toast should have a different ID")
	}

	// Dismiss with the stale ID — should NOT dismiss the active toast.
	m, _ = m.update(toastDismissMsg{id: staleID})
	if !m.active {
		t.Error("toast should still be active when dismiss ID is stale")
	}
	if m.message != "second" {
		t.Errorf("message = %q, want %q", m.message, "second")
	}
}

func TestToastShow_ReplacesExisting(t *testing.T) {
	m := newToastModel()
	m, _ = m.show("first", toastSuccess)
	if m.message != "first" {
		t.Fatalf("message = %q, want %q", m.message, "first")
	}
	firstID := m.id

	m, _ = m.show("second", toastError)
	if m.message != "second" {
		t.Errorf("message = %q, want %q", m.message, "second")
	}
	if m.kind != toastError {
		t.Errorf("kind = %d, want toastError (%d)", m.kind, toastError)
	}
	if m.id == firstID {
		t.Error("new toast should have a different ID from the replaced one")
	}
}

func TestToastShow_MonotonicIDs(t *testing.T) {
	m := newToastModel()
	var ids []int
	for i := 0; i < 5; i++ {
		m, _ = m.show("msg", toastSuccess)
		ids = append(ids, m.id)
	}
	for i := 1; i < len(ids); i++ {
		if ids[i] <= ids[i-1] {
			t.Errorf("IDs not monotonically increasing: %v", ids)
			break
		}
	}
}

func TestToastView_Inactive(t *testing.T) {
	m := newToastModel()
	v := m.view()
	if v != "" {
		t.Errorf("view() = %q, want empty when inactive", v)
	}
}

func TestToastView_Success(t *testing.T) {
	m := newToastModel()
	m, _ = m.show("Installed skill-x", toastSuccess)
	v := m.view()

	if v == "" {
		t.Fatal("view() should not be empty when active")
	}
	if !strings.Contains(v, "Installed skill-x") {
		t.Errorf("view() = %q, should contain message text", v)
	}
	// Should start with a space (1 char indent).
	if !strings.HasPrefix(v, " ") {
		t.Errorf("view() = %q, should start with space indent", v)
	}
}

func TestToastView_Error(t *testing.T) {
	m := newToastModel()
	m, _ = m.show("something failed", toastError)
	v := m.view()

	if !strings.Contains(v, "something failed") {
		t.Errorf("view() = %q, should contain message text", v)
	}
}

func TestToastView_Loading(t *testing.T) {
	m := newToastModel()
	m, _ = m.show("Adding registry...", toastLoading)
	v := m.view()

	if !strings.Contains(v, "Adding registry...") {
		t.Errorf("view() = %q, should contain message text", v)
	}
}

func TestToastUpdate_SpinnerTickIgnoredWhenNotLoading(t *testing.T) {
	m := newToastModel()
	m, _ = m.show("done", toastSuccess)

	// Sending a spinner tick to a non-loading toast should be a no-op.
	tick := spinner.TickMsg{Time: time.Now()}
	m2, cmd := m.update(tick)
	if cmd != nil {
		t.Error("spinner tick on non-loading toast should return nil cmd")
	}
	if m2.message != m.message || m2.active != m.active {
		t.Error("spinner tick on non-loading toast should not change state")
	}
}

func TestToastUpdate_SpinnerTickHandledWhenLoading(t *testing.T) {
	m := newToastModel()
	m, _ = m.show("Loading...", toastLoading)

	// Sending a spinner tick to a loading toast should update the spinner.
	// The tick won't match the spinner's internal tag, but the code path
	// should still be exercised (spinner.Update is called).
	tick := spinner.TickMsg{Time: time.Now()}
	m2, _ := m.update(tick)
	if !m2.active {
		t.Error("loading toast should still be active after spinner tick")
	}
	if m2.kind != toastLoading {
		t.Errorf("kind = %d, want toastLoading (%d)", m2.kind, toastLoading)
	}
}

func TestToastDismiss_AfterDismiss_ViewEmpty(t *testing.T) {
	m := newToastModel()
	m, _ = m.show("hello", toastSuccess)
	m = m.dismiss()
	v := m.view()
	if v != "" {
		t.Errorf("view() = %q, want empty after dismiss", v)
	}
}

func TestToastUpdate_DismissInactiveToast(t *testing.T) {
	m := newToastModel()
	// Sending a dismiss message to an inactive toast should be a no-op.
	m, _ = m.update(toastDismissMsg{id: 0})
	if m.active {
		t.Error("inactive toast should remain inactive after dismiss msg")
	}
}

func TestToastShow_SuccessThenLoading_ReplacesKind(t *testing.T) {
	m := newToastModel()
	m, _ = m.show("done", toastSuccess)
	m, _ = m.show("working...", toastLoading)

	if m.kind != toastLoading {
		t.Errorf("kind = %d, want toastLoading (%d)", m.kind, toastLoading)
	}
	if m.message != "working..." {
		t.Errorf("message = %q, want %q", m.message, "working...")
	}
}

func TestToastShow_LoadingThenSuccess_ReplacesKind(t *testing.T) {
	m := newToastModel()
	m, _ = m.show("working...", toastLoading)
	m, _ = m.show("done", toastSuccess)

	if m.kind != toastSuccess {
		t.Errorf("kind = %d, want toastSuccess (%d)", m.kind, toastSuccess)
	}
	if m.message != "done" {
		t.Errorf("message = %q, want %q", m.message, "done")
	}
}
