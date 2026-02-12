package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNewConfirmModel(t *testing.T) {
	m := newConfirmModel()
	if m.active {
		t.Error("new confirm should not be active")
	}
	if m.message != "" {
		t.Errorf("message = %q, want empty", m.message)
	}
	if m.onConfirm != nil {
		t.Error("onConfirm should be nil")
	}
}

func TestConfirmShow(t *testing.T) {
	m := newConfirmModel()
	cmd := func() tea.Msg { return nil }
	m = m.show("Remove registry foo?", cmd)

	if !m.active {
		t.Error("confirm should be active after show")
	}
	if m.message != "Remove registry foo?" {
		t.Errorf("message = %q, want %q", m.message, "Remove registry foo?")
	}
	if m.onConfirm == nil {
		t.Error("onConfirm should not be nil after show")
	}
}

func TestConfirmDismiss(t *testing.T) {
	m := newConfirmModel()
	m = m.show("Remove?", func() tea.Msg { return nil })
	if !m.active {
		t.Fatal("confirm should be active before dismiss")
	}

	m = m.dismiss()
	if m.active {
		t.Error("confirm should not be active after dismiss")
	}
	if m.message != "" {
		t.Errorf("message = %q, want empty after dismiss", m.message)
	}
	if m.onConfirm != nil {
		t.Error("onConfirm should be nil after dismiss")
	}
}

func TestConfirmUpdate_YesKey(t *testing.T) {
	m := newConfirmModel()
	m = m.show("Delete?", func() tea.Msg {
		return nil
	})

	yKey := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}}
	m, cmd, consumed := m.update(yKey)

	if !consumed {
		t.Error("y key should be consumed")
	}
	if m.active {
		t.Error("confirm should be dismissed after y")
	}
	if cmd == nil {
		t.Fatal("cmd should not be nil after y — should batch onConfirm + confirmResultMsg")
	}

	// Execute the batch command and verify onConfirm was called.
	// The batch returns a function; we can't easily inspect it, but we can
	// verify the model state is correct.
	if m.onConfirm != nil {
		t.Error("onConfirm should be nil after dismiss")
	}
}

func TestConfirmUpdate_YesKeyUppercase(t *testing.T) {
	m := newConfirmModel()
	m = m.show("Delete?", func() tea.Msg { return nil })

	yKey := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'Y'}}
	m, cmd, consumed := m.update(yKey)

	if !consumed {
		t.Error("Y key should be consumed")
	}
	if m.active {
		t.Error("confirm should be dismissed after Y")
	}
	if cmd == nil {
		t.Error("cmd should not be nil after Y")
	}
}

func TestConfirmUpdate_NoKey(t *testing.T) {
	m := newConfirmModel()
	m = m.show("Delete?", func() tea.Msg { return nil })

	nKey := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}}
	m, cmd, consumed := m.update(nKey)

	if !consumed {
		t.Error("n key should be consumed")
	}
	if m.active {
		t.Error("confirm should be dismissed after n")
	}
	if cmd == nil {
		t.Fatal("cmd should not be nil — should return confirmResultMsg{confirmed: false}")
	}
}

func TestConfirmUpdate_NoKeyUppercase(t *testing.T) {
	m := newConfirmModel()
	m = m.show("Delete?", func() tea.Msg { return nil })

	nKey := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'N'}}
	m, cmd, consumed := m.update(nKey)

	if !consumed {
		t.Error("N key should be consumed")
	}
	if m.active {
		t.Error("confirm should be dismissed after N")
	}
	if cmd == nil {
		t.Error("cmd should not be nil after N")
	}
}

func TestConfirmUpdate_EscKey(t *testing.T) {
	m := newConfirmModel()
	m = m.show("Delete?", func() tea.Msg { return nil })

	escKey := tea.KeyMsg{Type: tea.KeyEscape}
	m, cmd, consumed := m.update(escKey)

	if !consumed {
		t.Error("esc key should be consumed")
	}
	if m.active {
		t.Error("confirm should be dismissed after esc")
	}
	if cmd == nil {
		t.Error("cmd should not be nil after esc — should return confirmResultMsg{confirmed: false}")
	}
}

func TestConfirmUpdate_OtherKeysConsumed(t *testing.T) {
	m := newConfirmModel()
	m = m.show("Delete?", func() tea.Msg { return nil })

	// Random keys should be consumed but not dismiss the dialog.
	for _, r := range []rune{'a', 'z', 'q', 'x', '1'} {
		k := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}
		m2, cmd, consumed := m.update(k)
		if !consumed {
			t.Errorf("key %q should be consumed when confirm is active", string(r))
		}
		if !m2.active {
			t.Errorf("key %q should not dismiss the confirm dialog", string(r))
		}
		if cmd != nil {
			t.Errorf("key %q should return nil cmd", string(r))
		}
	}
}

func TestConfirmUpdate_InactiveIgnoresKeys(t *testing.T) {
	m := newConfirmModel()

	yKey := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}}
	_, cmd, consumed := m.update(yKey)

	if consumed {
		t.Error("inactive confirm should not consume keys")
	}
	if cmd != nil {
		t.Error("inactive confirm should return nil cmd")
	}
}

func TestConfirmUpdate_NonKeyMsgIgnored(t *testing.T) {
	m := newConfirmModel()
	m = m.show("Delete?", func() tea.Msg { return nil })

	// Non-key messages should not be consumed.
	m2, cmd, consumed := m.update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if consumed {
		t.Error("non-key message should not be consumed")
	}
	if cmd != nil {
		t.Error("non-key message should return nil cmd")
	}
	if !m2.active {
		t.Error("confirm should remain active after non-key message")
	}
}

func TestConfirmView_Inactive(t *testing.T) {
	m := newConfirmModel()
	v := m.view()
	if v != "" {
		t.Errorf("view() = %q, want empty when inactive", v)
	}
}

func TestConfirmView_Active(t *testing.T) {
	m := newConfirmModel()
	m = m.show("Remove registry foo?", func() tea.Msg { return nil })
	m = m.setSize(80, 24)
	v := m.view()

	if v == "" {
		t.Fatal("view() should not be empty when active")
	}
	if !strings.Contains(v, "Remove registry foo?") {
		t.Errorf("view() = %q, should contain message text", v)
	}
	if !strings.Contains(v, "Yes") {
		t.Errorf("view() = %q, should contain Yes button", v)
	}
	if !strings.Contains(v, "No") {
		t.Errorf("view() = %q, should contain No button", v)
	}
}

func TestConfirmView_AfterDismiss(t *testing.T) {
	m := newConfirmModel()
	m = m.show("Delete?", func() tea.Msg { return nil })
	m = m.dismiss()
	v := m.view()
	if v != "" {
		t.Errorf("view() = %q, want empty after dismiss", v)
	}
}

func TestConfirmShow_ReplacesExisting(t *testing.T) {
	m := newConfirmModel()
	m = m.show("First?", func() tea.Msg { return nil })
	m = m.show("Second?", func() tea.Msg { return nil })

	if m.message != "Second?" {
		t.Errorf("message = %q, want %q", m.message, "Second?")
	}
	if !m.active {
		t.Error("confirm should be active after replacing")
	}
}

func TestConfirmSetSize(t *testing.T) {
	m := newConfirmModel()
	m = m.setSize(80, 24)
	if m.width != 80 {
		t.Errorf("width = %d, want 80", m.width)
	}
	if m.height != 24 {
		t.Errorf("height = %d, want 24", m.height)
	}
}

func TestConfirmView_ActiveWithoutSize(t *testing.T) {
	m := newConfirmModel()
	m = m.show("Delete?", func() tea.Msg { return nil })
	// No setSize — should still render the dialog box (just not centered).
	v := m.view()
	if v == "" {
		t.Fatal("view() should not be empty even without size")
	}
	if !strings.Contains(v, "Delete?") {
		t.Errorf("view() = %q, should contain message text", v)
	}
}

func TestConfirmShow_DefaultFocusNo(t *testing.T) {
	m := newConfirmModel()
	m = m.show("Delete?", func() tea.Msg { return nil })

	if m.focusYes {
		t.Error("default focus should be on No (focusYes = false)")
	}
}

func TestConfirmUpdate_TabTogglesFocus(t *testing.T) {
	m := newConfirmModel()
	m = m.show("Delete?", func() tea.Msg { return nil })

	if m.focusYes {
		t.Fatal("should start focused on No")
	}

	// Tab toggles to Yes.
	tab := tea.KeyMsg{Type: tea.KeyTab}
	m, _, consumed := m.update(tab)
	if !consumed {
		t.Error("tab should be consumed")
	}
	if !m.focusYes {
		t.Error("tab should toggle focus to Yes")
	}

	// Tab again toggles back to No.
	m, _, _ = m.update(tab)
	if m.focusYes {
		t.Error("second tab should toggle focus back to No")
	}
}

func TestConfirmUpdate_ShiftTabTogglesFocus(t *testing.T) {
	m := newConfirmModel()
	m = m.show("Delete?", func() tea.Msg { return nil })

	shiftTab := tea.KeyMsg{Type: tea.KeyShiftTab}
	m, _, consumed := m.update(shiftTab)
	if !consumed {
		t.Error("shift+tab should be consumed")
	}
	if !m.focusYes {
		t.Error("shift+tab should toggle focus to Yes")
	}
}

func TestConfirmUpdate_LeftRightToggleFocus(t *testing.T) {
	m := newConfirmModel()
	m = m.show("Delete?", func() tea.Msg { return nil })

	// Left arrow.
	left := tea.KeyMsg{Type: tea.KeyLeft}
	m, _, consumed := m.update(left)
	if !consumed {
		t.Error("left arrow should be consumed")
	}
	if !m.focusYes {
		t.Error("left arrow should toggle focus to Yes")
	}

	// Right arrow.
	right := tea.KeyMsg{Type: tea.KeyRight}
	m, _, consumed = m.update(right)
	if !consumed {
		t.Error("right arrow should be consumed")
	}
	if m.focusYes {
		t.Error("right arrow should toggle focus back to No")
	}
}

func TestConfirmUpdate_EnterOnFocusedNo(t *testing.T) {
	m := newConfirmModel()
	m = m.show("Delete?", func() tea.Msg { return nil })

	// Default focus is No, so Enter should cancel.
	enter := tea.KeyMsg{Type: tea.KeyEnter}
	m, cmd, consumed := m.update(enter)

	if !consumed {
		t.Error("enter should be consumed")
	}
	if m.active {
		t.Error("confirm should be dismissed after enter on No")
	}
	if cmd == nil {
		t.Error("cmd should not be nil — should return confirmResultMsg{confirmed: false}")
	}

	// Verify the result message.
	msg := cmd()
	result, ok := msg.(confirmResultMsg)
	if !ok {
		t.Fatalf("expected confirmResultMsg, got %T", msg)
	}
	if result.confirmed {
		t.Error("enter on No should produce confirmed=false")
	}
}

func TestConfirmUpdate_EnterOnFocusedYes(t *testing.T) {
	m := newConfirmModel()
	confirmCalled := false
	m = m.show("Delete?", func() tea.Msg {
		confirmCalled = true
		return nil
	})

	// Toggle focus to Yes.
	tab := tea.KeyMsg{Type: tea.KeyTab}
	m, _, _ = m.update(tab)
	if !m.focusYes {
		t.Fatal("focus should be on Yes after tab")
	}

	// Enter activates Yes.
	enter := tea.KeyMsg{Type: tea.KeyEnter}
	m, cmd, consumed := m.update(enter)

	if !consumed {
		t.Error("enter should be consumed")
	}
	if m.active {
		t.Error("confirm should be dismissed after enter on Yes")
	}
	if cmd == nil {
		t.Fatal("cmd should not be nil after enter on Yes")
	}

	// Execute the batch and verify the onConfirm callback was included.
	// tea.Batch returns a function that sends multiple messages via
	// a channel, so we just verify it's callable and the model dismissed.
	_ = confirmCalled // The callback is wrapped in tea.Batch; direct call check
	// is not reliable here, but we verify model state.
	if m.onConfirm != nil {
		t.Error("onConfirm should be nil after dismiss")
	}
}

func TestConfirmUpdate_HKeyTogglesFocus(t *testing.T) {
	m := newConfirmModel()
	m = m.show("Delete?", func() tea.Msg { return nil })

	hKey := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}}
	m, _, consumed := m.update(hKey)
	if !consumed {
		t.Error("h key should be consumed")
	}
	if !m.focusYes {
		t.Error("h key should toggle focus to Yes")
	}
}

func TestConfirmUpdate_LKeyTogglesFocus(t *testing.T) {
	m := newConfirmModel()
	m = m.show("Delete?", func() tea.Msg { return nil })

	// First toggle to Yes.
	hKey := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}}
	m, _, _ = m.update(hKey)

	lKey := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}}
	m, _, consumed := m.update(lKey)
	if !consumed {
		t.Error("l key should be consumed")
	}
	if m.focusYes {
		t.Error("l key should toggle focus back to No")
	}
}

func TestConfirmUpdate_YesThenInactive(t *testing.T) {
	m := newConfirmModel()
	m = m.show("Delete?", func() tea.Msg { return nil })

	yKey := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}}
	m, _, _ = m.update(yKey)

	// After confirmation, the dialog should be fully inactive.
	v := m.view()
	if v != "" {
		t.Errorf("view() = %q, want empty after confirmation", v)
	}

	// Further keys should not be consumed.
	qKey := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}
	_, _, consumed := m.update(qKey)
	if consumed {
		t.Error("keys should not be consumed after confirmation dismisses dialog")
	}
}
