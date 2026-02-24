package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
)

func TestNewStatusBarModel(t *testing.T) {
	m := newStatusBarModel()
	if m.msg != "" {
		t.Errorf("msg = %q, want empty", m.msg)
	}
	if m.nextID != 0 {
		t.Errorf("nextID = %d, want 0", m.nextID)
	}
	if m.tasks != 0 {
		t.Errorf("tasks = %d, want 0", m.tasks)
	}
	if m.done != 0 {
		t.Errorf("done = %d, want 0", m.done)
	}
	if m.tasksRunning() {
		t.Error("new status bar should not have tasks running")
	}
}

func TestStatusBar_ShowMsg_Success(t *testing.T) {
	m := newStatusBarModel()
	m, cmd := m.showMsg("Installed skill-x", statusSuccess)

	if m.msg != "Installed skill-x" {
		t.Errorf("msg = %q, want %q", m.msg, "Installed skill-x")
	}
	if m.msgKind != statusSuccess {
		t.Errorf("msgKind = %d, want statusSuccess (%d)", m.msgKind, statusSuccess)
	}
	if m.msgID != 0 {
		t.Errorf("msgID = %d, want 0", m.msgID)
	}
	if m.nextID != 1 {
		t.Errorf("nextID = %d, want 1", m.nextID)
	}
	if cmd == nil {
		t.Error("showMsg(success) should return a cmd for auto-dismiss timer")
	}
}

func TestStatusBar_ShowMsg_Error(t *testing.T) {
	m := newStatusBarModel()
	m, cmd := m.showMsg("Error: something broke", statusError)

	if m.msg != "Error: something broke" {
		t.Errorf("msg = %q, want %q", m.msg, "Error: something broke")
	}
	if m.msgKind != statusError {
		t.Errorf("msgKind = %d, want statusError (%d)", m.msgKind, statusError)
	}
	if cmd == nil {
		t.Error("showMsg(error) should return a cmd for auto-dismiss timer")
	}
}

func TestStatusBar_ShowMsg_Warning(t *testing.T) {
	m := newStatusBarModel()
	m, cmd := m.showMsg("2 warnings", statusWarning)

	if m.msgKind != statusWarning {
		t.Errorf("msgKind = %d, want statusWarning (%d)", m.msgKind, statusWarning)
	}
	if cmd == nil {
		t.Error("showMsg(warning) should return a cmd for auto-dismiss timer")
	}
}

func TestStatusBar_DismissMsg(t *testing.T) {
	m := newStatusBarModel()
	m, _ = m.showMsg("hello", statusSuccess)
	if m.msg == "" {
		t.Fatal("msg should not be empty after showMsg")
	}

	m = m.dismissMsg()
	if m.msg != "" {
		t.Errorf("msg = %q, want empty after dismissMsg", m.msg)
	}
}

func TestStatusBar_Update_DismissMatchingID(t *testing.T) {
	m := newStatusBarModel()
	m, _ = m.showMsg("hello", statusSuccess)
	id := m.msgID

	m, _ = m.update(statusDismissMsg{id: id})
	if m.msg != "" {
		t.Errorf("msg = %q, want empty when dismiss ID matches", m.msg)
	}
}

func TestStatusBar_Update_DismissStaleID(t *testing.T) {
	m := newStatusBarModel()
	m, _ = m.showMsg("first", statusSuccess)
	staleID := m.msgID

	// Show a second message, which gets a new ID.
	m, _ = m.showMsg("second", statusSuccess)
	if m.msgID == staleID {
		t.Fatal("second message should have a different ID")
	}

	// Dismiss with the stale ID — should NOT dismiss the active message.
	m, _ = m.update(statusDismissMsg{id: staleID})
	if m.msg != "second" {
		t.Errorf("msg = %q, want %q (stale dismiss should be ignored)", m.msg, "second")
	}
}

func TestStatusBar_ShowMsg_ReplacesExisting(t *testing.T) {
	m := newStatusBarModel()
	m, _ = m.showMsg("first", statusSuccess)
	firstID := m.msgID

	m, _ = m.showMsg("second", statusError)
	if m.msg != "second" {
		t.Errorf("msg = %q, want %q", m.msg, "second")
	}
	if m.msgKind != statusError {
		t.Errorf("msgKind = %d, want statusError (%d)", m.msgKind, statusError)
	}
	if m.msgID == firstID {
		t.Error("new message should have a different ID from the replaced one")
	}
}

func TestStatusBar_ShowMsg_MonotonicIDs(t *testing.T) {
	m := newStatusBarModel()
	var ids []int
	for i := 0; i < 5; i++ {
		m, _ = m.showMsg("msg", statusSuccess)
		ids = append(ids, m.msgID)
	}
	for i := 1; i < len(ids); i++ {
		if ids[i] <= ids[i-1] {
			t.Errorf("IDs not monotonically increasing: %v", ids)
			break
		}
	}
}

func TestStatusBar_TaskStarted_SingleTask(t *testing.T) {
	m := newStatusBarModel()
	m, cmd := m.update(taskStartedMsg{})

	if m.tasks != 1 {
		t.Errorf("tasks = %d, want 1", m.tasks)
	}
	if m.done != 0 {
		t.Errorf("done = %d, want 0", m.done)
	}
	if !m.tasksRunning() {
		t.Error("tasksRunning() should be true after taskStartedMsg")
	}
	if cmd == nil {
		t.Error("first taskStartedMsg should return a spinner tick cmd")
	}
}

func TestStatusBar_TaskStarted_MultipleTasks(t *testing.T) {
	m := newStatusBarModel()
	m, _ = m.update(taskStartedMsg{})
	m, cmd := m.update(taskStartedMsg{})

	if m.tasks != 2 {
		t.Errorf("tasks = %d, want 2", m.tasks)
	}
	if cmd != nil {
		t.Error("second taskStartedMsg should return nil cmd (spinner already ticking)")
	}
}

func TestStatusBar_TaskDone_SingleTask(t *testing.T) {
	m := newStatusBarModel()
	m, _ = m.update(taskStartedMsg{})
	m, _ = m.update(taskDoneMsg{})

	if m.tasks != 0 {
		t.Errorf("tasks = %d, want 0 (reset after all done)", m.tasks)
	}
	if m.done != 0 {
		t.Errorf("done = %d, want 0 (reset after all done)", m.done)
	}
	if m.tasksRunning() {
		t.Error("tasksRunning() should be false after all tasks complete")
	}
}

func TestStatusBar_TaskDone_PartialBatch(t *testing.T) {
	m := newStatusBarModel()
	m, _ = m.update(taskStartedMsg{})
	m, _ = m.update(taskStartedMsg{})
	m, _ = m.update(taskStartedMsg{})
	m, _ = m.update(taskDoneMsg{})

	if m.tasks != 3 {
		t.Errorf("tasks = %d, want 3", m.tasks)
	}
	if m.done != 1 {
		t.Errorf("done = %d, want 1", m.done)
	}
	if !m.tasksRunning() {
		t.Error("tasksRunning() should be true with partial completion")
	}
}

func TestStatusBar_TaskDone_AllComplete(t *testing.T) {
	m := newStatusBarModel()
	m, _ = m.update(taskStartedMsg{})
	m, _ = m.update(taskStartedMsg{})
	m, _ = m.update(taskDoneMsg{})
	m, _ = m.update(taskDoneMsg{})

	if m.tasks != 0 {
		t.Errorf("tasks = %d, want 0 (reset after all done)", m.tasks)
	}
	if m.done != 0 {
		t.Errorf("done = %d, want 0 (reset after all done)", m.done)
	}
	if m.tasksRunning() {
		t.Error("tasksRunning() should be false after all tasks complete")
	}
}

func TestStatusBar_SpinnerTick_IgnoredWhenNoTasks(t *testing.T) {
	m := newStatusBarModel()
	tick := spinner.TickMsg{Time: time.Now()}
	m2, cmd := m.update(tick)
	if cmd != nil {
		t.Error("spinner tick with no tasks should return nil cmd")
	}
	if m2.tasks != m.tasks {
		t.Error("spinner tick with no tasks should not change state")
	}
}

func TestStatusBar_SpinnerTick_HandledWhenTasksRunning(t *testing.T) {
	m := newStatusBarModel()
	m, _ = m.update(taskStartedMsg{})

	tick := spinner.TickMsg{Time: time.Now()}
	m2, _ := m.update(tick)
	if !m2.tasksRunning() {
		t.Error("tasks should still be running after spinner tick")
	}
}

func TestStatusBar_View_NoMessage_NoTasks(t *testing.T) {
	m := newStatusBarModel()
	m.width = 80
	v := m.view("help text here")

	if !strings.Contains(v, "help text here") {
		t.Errorf("view() = %q, should contain help text", v)
	}
}

func TestStatusBar_View_WithMessage(t *testing.T) {
	m := newStatusBarModel()
	m.width = 80
	m, _ = m.showMsg("Installed skill-x", statusSuccess)
	v := m.view("help text")

	if !strings.Contains(v, "Installed skill-x") {
		t.Errorf("view() = %q, should contain message text", v)
	}
}

func TestStatusBar_View_WithTasks(t *testing.T) {
	m := newStatusBarModel()
	m.width = 80
	m, _ = m.update(taskStartedMsg{})
	v := m.view("help text")

	// Should show "fetching" label.
	if !strings.Contains(v, "fetching") {
		t.Errorf("view() = %q, should contain 'syncing'", v)
	}
}

func TestStatusBar_View_MessageHidesHelp(t *testing.T) {
	m := newStatusBarModel()
	m.width = 80
	m, _ = m.showMsg("Installed skill-x", statusSuccess)
	v := m.view("help text here")

	if !strings.Contains(v, "Installed skill-x") {
		t.Errorf("view() = %q, should contain message", v)
	}
	if strings.Contains(v, "help text here") {
		t.Error("help text should be hidden when a message is active")
	}
}

func TestStatusBar_RenderLeft_Empty(t *testing.T) {
	m := newStatusBarModel()
	left := m.renderLeft()
	if left != "" {
		t.Errorf("renderLeft() = %q, want empty when no message", left)
	}
}

func TestStatusBar_RenderLeft_Success(t *testing.T) {
	m := newStatusBarModel()
	m, _ = m.showMsg("done", statusSuccess)
	left := m.renderLeft()
	if !strings.Contains(left, "done") {
		t.Errorf("renderLeft() = %q, should contain message", left)
	}
}

func TestStatusBar_RenderLeft_Error(t *testing.T) {
	m := newStatusBarModel()
	m, _ = m.showMsg("failed", statusError)
	left := m.renderLeft()
	if !strings.Contains(left, "failed") {
		t.Errorf("renderLeft() = %q, should contain message", left)
	}
}

func TestStatusBar_RenderLeft_Warning(t *testing.T) {
	m := newStatusBarModel()
	m, _ = m.showMsg("caution", statusWarning)
	left := m.renderLeft()
	if !strings.Contains(left, "caution") {
		t.Errorf("renderLeft() = %q, should contain message", left)
	}
}

func TestStatusBar_RenderRight_NoTasks(t *testing.T) {
	m := newStatusBarModel()
	right := m.renderRight()
	if right != "" {
		t.Errorf("renderRight() = %q, want empty when no tasks", right)
	}
}

func TestStatusBar_RenderRight_WithTasks(t *testing.T) {
	m := newStatusBarModel()
	m, _ = m.update(taskStartedMsg{})
	right := m.renderRight()
	if !strings.Contains(right, "fetching") {
		t.Errorf("renderRight() = %q, should contain 'syncing'", right)
	}
}

func TestStatusBar_TasksRunning_EdgeCases(t *testing.T) {
	m := newStatusBarModel()

	// Zero tasks.
	if m.tasksRunning() {
		t.Error("tasksRunning() should be false with 0 tasks")
	}

	// One task started.
	m, _ = m.update(taskStartedMsg{})
	if !m.tasksRunning() {
		t.Error("tasksRunning() should be true with 1 started, 0 done")
	}

	// One task done (all complete).
	m, _ = m.update(taskDoneMsg{})
	if m.tasksRunning() {
		t.Error("tasksRunning() should be false after all tasks done")
	}
}
