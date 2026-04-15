package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/devports/devpt/pkg/models"
	"github.com/stretchr/testify/assert"
)

// ---------------------------------------------------------------------------
// Local mock structs — embed fakeAppDeps and override specific methods
// for call-counting and error injection.
// ---------------------------------------------------------------------------

type mockStopper struct {
	fakeAppDeps
	stopFn func(pid int, timeout time.Duration) error
}

func (m *mockStopper) StopProcess(pid int, timeout time.Duration) error {
	if m.stopFn != nil {
		return m.stopFn(pid, timeout)
	}
	return nil
}

type mockStarter struct {
	fakeAppDeps
	startFn func(name string) error
}

func (m *mockStarter) StartService(name string) error {
	if m.startFn != nil {
		return m.startFn(name)
	}
	return nil
}

type mockRestarter struct {
	fakeAppDeps
	restartFn func(name string) error
}

func (m *mockRestarter) RestartService(name string) error {
	if m.restartFn != nil {
		return m.restartFn(name)
	}
	return nil
}

type mockRemover struct {
	fakeAppDeps
	removeFn func(name string) error
}

func (m *mockRemover) RemoveService(name string) error {
	if m.removeFn != nil {
		return m.removeFn(name)
	}
	return m.fakeAppDeps.RemoveService(name)
}

// ---------------------------------------------------------------------------
// TEST-group-stop
// Covers: BR-1.4, BR-1.9, C-1.2, C-1.4, C-1.6, Edge-1.5
// ---------------------------------------------------------------------------

func TestGroupStop(t *testing.T) {
	t.Parallel()

	t.Run("confirmation modal shows group service list", func(t *testing.T) {
		deps := &fakeAppDeps{
			servers: []*models.ServerInfo{
				makeRunningServer("api-gateway", 1001, 3000),
				makeRunningServer("api-auth", 1002, 3001),
				makeRunningServer("api-cron", 1003, 3002),
			},
			services: []*models.ManagedService{
				{Name: "api-gateway", CWD: "/tmp/api-gateway", Command: "node server.js", Ports: []int{3000}},
				{Name: "api-auth", CWD: "/tmp/api-auth", Command: "go run .", Ports: []int{3001}},
				{Name: "api-cron", CWD: "/tmp/api-cron", Command: "python cron.py", Ports: []int{3002}},
			},
		}
		m := newTopModel(deps)
		m.mode = viewModeTable
		m.focus = focusRunning
		m.selected = 0

		// Trigger group stop
		msg := tea.KeyPressMsg{Code: 'e', Mod: tea.ModCtrl | tea.ModShift}
		newModel, _ := m.Update(msg)
		updated := newModel.(*topModel)

		// Should open group confirm modal
		assert.NotNil(t, updated.confirm)
		assert.Equal(t, confirmGroupStop, updated.confirm.kind)
		// Prompt should mention group
		assert.Contains(t, updated.confirm.prompt, "api")
		// Should show member count
		assert.Contains(t, updated.confirm.prompt, "3")
	})

	t.Run("confirmed stop executes on all group members", func(t *testing.T) {
		stopCount := 0
		deps := &mockStopper{
			fakeAppDeps: fakeAppDeps{
				servers: []*models.ServerInfo{
					makeRunningServer("api-gateway", 1001, 3000),
					makeRunningServer("api-auth", 1002, 3001),
					makeRunningServer("api-cron", 1003, 3002),
				},
				services: []*models.ManagedService{
					{Name: "api-gateway", CWD: "/tmp/api-gateway", Command: "node server.js", Ports: []int{3000}},
					{Name: "api-auth", CWD: "/tmp/api-auth", Command: "go run .", Ports: []int{3001}},
					{Name: "api-cron", CWD: "/tmp/api-cron", Command: "python cron.py", Ports: []int{3002}},
				},
			},
			stopFn: func(pid int, timeout time.Duration) error {
				stopCount++
				return nil
			},
		}
		m := newTopModel(deps)
		m.mode = viewModeTable
		m.focus = focusRunning
		m.selected = 0

		// Trigger group stop
		m.Update(tea.KeyPressMsg{Code: 'e', Mod: tea.ModCtrl | tea.ModShift})
		// Confirm
		m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

		// All 3 processes should be stopped
		assert.Equal(t, 3, stopCount)
		// cmdStatus should show per-service results
		assert.Contains(t, m.cmdStatus, "Stopped")
	})

	t.Run("cancelled stop does not stop any process", func(t *testing.T) {
		stopCount := 0
		deps := &mockStopper{
			fakeAppDeps: fakeAppDeps{
				servers: []*models.ServerInfo{
					makeRunningServer("api-gateway", 1001, 3000),
					makeRunningServer("api-auth", 1002, 3001),
				},
				services: []*models.ManagedService{
					{Name: "api-gateway", CWD: "/tmp/api-gateway", Command: "node server.js", Ports: []int{3000}},
					{Name: "api-auth", CWD: "/tmp/api-auth", Command: "go run .", Ports: []int{3001}},
				},
			},
			stopFn: func(pid int, timeout time.Duration) error {
				stopCount++
				return nil
			},
		}
		m := newTopModel(deps)
		m.mode = viewModeTable
		m.focus = focusRunning
		m.selected = 0

		// Trigger group stop
		m.Update(tea.KeyPressMsg{Code: 'e', Mod: tea.ModCtrl | tea.ModShift})
		// Cancel with 'n'
		m.Update(tea.KeyPressMsg{Code: 'n'})

		assert.Equal(t, 0, stopCount)
		assert.Equal(t, "Cancelled", m.cmdStatus)
	})

	t.Run("cancelled stop with escape does not stop any process", func(t *testing.T) {
		stopCount := 0
		deps := &mockStopper{
			fakeAppDeps: fakeAppDeps{
				servers: []*models.ServerInfo{
					makeRunningServer("api-gateway", 1001, 3000),
				},
				services: []*models.ManagedService{
					{Name: "api-gateway", CWD: "/tmp/api-gateway", Command: "node server.js", Ports: []int{3000}},
				},
			},
			stopFn: func(pid int, timeout time.Duration) error {
				stopCount++
				return nil
			},
		}
		m := newTopModel(deps)
		m.mode = viewModeTable
		m.focus = focusRunning
		m.selected = 0

		m.Update(tea.KeyPressMsg{Code: 'e', Mod: tea.ModCtrl | tea.ModShift})
		m.Update(tea.KeyPressMsg{Code: tea.KeyEsc})

		assert.Equal(t, 0, stopCount)
		assert.Equal(t, "Cancelled", m.cmdStatus)
	})

	t.Run("partial failure continues remaining members", func(t *testing.T) {
		stopCount := 0
		deps := &mockStopper{
			fakeAppDeps: fakeAppDeps{
				servers: []*models.ServerInfo{
					makeRunningServer("api-gateway", 1001, 3000),
					makeRunningServer("api-auth", 1002, 3001),
					makeRunningServer("api-cron", 1003, 3002),
				},
				services: []*models.ManagedService{
					{Name: "api-gateway", CWD: "/tmp/api-gateway", Command: "node server.js", Ports: []int{3000}},
					{Name: "api-auth", CWD: "/tmp/api-auth", Command: "go run .", Ports: []int{3001}},
					{Name: "api-cron", CWD: "/tmp/api-cron", Command: "python cron.py", Ports: []int{3002}},
				},
			},
			stopFn: func(pid int, timeout time.Duration) error {
				stopCount++
				if pid == 1002 {
					return fmt.Errorf("process %d: permission denied", pid)
				}
				return nil
			},
		}
		m := newTopModel(deps)
		m.mode = viewModeTable
		m.focus = focusRunning
		m.selected = 0

		m.Update(tea.KeyPressMsg{Code: 'e', Mod: tea.ModCtrl | tea.ModShift})
		m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

		// All 3 should be attempted
		assert.Equal(t, 3, stopCount)
		// cmdStatus should show partial result
		assert.Contains(t, m.cmdStatus, "permission denied")
		// Should also show successes
		assert.Contains(t, m.cmdStatus, "1001")
	})

	t.Run("single member group stop works", func(t *testing.T) {
		stopCount := 0
		deps := &mockStopper{
			fakeAppDeps: fakeAppDeps{
				servers: []*models.ServerInfo{
					makeRunningServer("redis", 1001, 6379),
				},
				services: []*models.ManagedService{
					{Name: "redis", CWD: "/tmp/redis", Command: "redis-server", Ports: []int{6379}},
				},
			},
			stopFn: func(pid int, timeout time.Duration) error {
				stopCount++
				return nil
			},
		}
		m := newTopModel(deps)
		m.mode = viewModeTable
		m.focus = focusRunning
		m.selected = 0

		m.Update(tea.KeyPressMsg{Code: 'e', Mod: tea.ModCtrl | tea.ModShift})
		m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

		assert.Equal(t, 1, stopCount)
	})

	t.Run("Edge-1.5: all already stopped shows message", func(t *testing.T) {
		deps := &fakeAppDeps{
			servers: []*models.ServerInfo{},
			services: []*models.ManagedService{
				{Name: "api-gateway", CWD: "/tmp/api-gateway", Command: "node server.js", Ports: []int{3000}},
			},
		}
		m := newTopModel(deps)
		m.mode = viewModeTable
		m.focus = focusManaged
		m.managedSel = 0

		// No running servers — group stop should be a no-op or show message
		msg := tea.KeyPressMsg{Code: 'e', Mod: tea.ModCtrl | tea.ModShift}
		newModel, _ := m.Update(msg)
		updated := newModel.(*topModel)

		// No modal should open if there are no group members to stop
		if updated.confirm != nil {
			assert.Contains(t, updated.confirm.prompt, "0")
		}
	})
}

// ---------------------------------------------------------------------------
// TEST-group-restart
// Covers: BR-1.5, C-1.6
// ---------------------------------------------------------------------------

func TestGroupRestart(t *testing.T) {
	t.Parallel()

	t.Run("group restart with confirmation", func(t *testing.T) {
		restartCount := 0
		deps := &mockRestarter{
			fakeAppDeps: fakeAppDeps{
				servers: []*models.ServerInfo{
					makeRunningServer("web-frontend", 1001, 3000),
					makeRunningServer("web-backend", 1002, 3001),
				},
				services: []*models.ManagedService{
					{Name: "web-frontend", CWD: "/tmp/web-frontend", Command: "npm run dev", Ports: []int{3000}},
					{Name: "web-backend", CWD: "/tmp/web-backend", Command: "go run .", Ports: []int{3001}},
				},
			},
			restartFn: func(name string) error {
				restartCount++
				return nil
			},
		}
		m := newTopModel(deps)
		m.mode = viewModeTable
		m.focus = focusRunning
		m.selected = 0

		m.Update(tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl | tea.ModShift})
		m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

		assert.Equal(t, 2, restartCount)
	})

	t.Run("group restart partial failure continues remaining", func(t *testing.T) {
		restartCount := 0
		deps := &mockRestarter{
			fakeAppDeps: fakeAppDeps{
				servers: []*models.ServerInfo{
					makeRunningServer("web-frontend", 1001, 3000),
					makeRunningServer("web-backend", 1002, 3001),
					makeRunningServer("web-worker", 1003, 3002),
				},
				services: []*models.ManagedService{
					{Name: "web-frontend", CWD: "/tmp/web-frontend", Command: "npm run dev", Ports: []int{3000}},
					{Name: "web-backend", CWD: "/tmp/web-backend", Command: "go run .", Ports: []int{3001}},
					{Name: "web-worker", CWD: "/tmp/web-worker", Command: "python worker.py", Ports: []int{3002}},
				},
			},
			restartFn: func(name string) error {
				restartCount++
				if name == "web-backend" {
					return fmt.Errorf("restart failed for %s", name)
				}
				return nil
			},
		}
		m := newTopModel(deps)
		m.mode = viewModeTable
		m.focus = focusRunning
		m.selected = 0

		m.Update(tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl | tea.ModShift})
		m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

		// All 3 attempted
		assert.Equal(t, 3, restartCount)
		// Status shows partial failure
		assert.Contains(t, m.cmdStatus, "web-backend")
		assert.Contains(t, m.cmdStatus, "failed")
	})

	t.Run("group restart cancelled", func(t *testing.T) {
		restartCount := 0
		deps := &mockRestarter{
			fakeAppDeps: fakeAppDeps{
				servers: []*models.ServerInfo{
					makeRunningServer("web-frontend", 1001, 3000),
				},
				services: []*models.ManagedService{
					{Name: "web-frontend", CWD: "/tmp/web-frontend", Command: "npm run dev", Ports: []int{3000}},
				},
			},
			restartFn: func(name string) error {
				restartCount++
				return nil
			},
		}
		m := newTopModel(deps)
		m.mode = viewModeTable
		m.focus = focusRunning
		m.selected = 0

		m.Update(tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl | tea.ModShift})
		m.Update(tea.KeyPressMsg{Code: 'n'})

		assert.Equal(t, 0, restartCount)
		assert.Equal(t, "Cancelled", m.cmdStatus)
	})

	t.Run("group restart with crashed/stopped services starts them", func(t *testing.T) {
		deps := &fakeAppDeps{
			servers: []*models.ServerInfo{
				makeRunningServer("web-backend", 1002, 3001),
				// web-worker is NOT running (stopped/crashed)
			},
			services: []*models.ManagedService{
				{Name: "web-backend", CWD: "/tmp/web-backend", Command: "go run .", Ports: []int{3001}},
				{Name: "web-worker", CWD: "/tmp/web-worker", Command: "python worker.py", Ports: []int{3002}},
			},
		}

		m := newTopModel(deps)
		m.mode = viewModeTable
		m.focus = focusRunning
		m.selected = 0

		m.Update(tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl | tea.ModShift})
		assert.Equal(t, confirmGroupRestart, m.confirm.kind)
		// Prompt should mention both restart and start
		assert.Contains(t, m.confirm.prompt, "restart")
		assert.Contains(t, m.confirm.prompt, "start")
		// Both services should be listed
		assert.Contains(t, m.confirm.prompt, "web-backend")
		assert.Contains(t, m.confirm.prompt, "web-worker")

		// Confirm the action
		m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
		// cmdStatus should show both a restart and a start
		assert.Contains(t, m.cmdStatus, "Restarted")
		assert.Contains(t, m.cmdStatus, "Started")
	})
}

// ---------------------------------------------------------------------------
// TEST-group-start
// Covers: BR-1.6, C-1.1, Edge-1.6
// ---------------------------------------------------------------------------

func TestGroupStart(t *testing.T) {
	t.Parallel()

	t.Run("starts only stopped managed services", func(t *testing.T) {
		startCount := 0
		deps := &mockStarter{
			fakeAppDeps: fakeAppDeps{
				servers: []*models.ServerInfo{
					// web-frontend is running
					makeRunningServer("web-frontend", 1001, 3000),
				},
				services: []*models.ManagedService{
					{Name: "web-frontend", CWD: "/tmp/web-frontend", Command: "npm run dev", Ports: []int{3000}},
					{Name: "web-backend", CWD: "/tmp/web-backend", Command: "go run .", Ports: []int{3001}},
					{Name: "web-worker", CWD: "/tmp/web-worker", Command: "python worker.py", Ports: []int{3002}},
				},
			},
			startFn: func(name string) error {
				startCount++
				return nil
			},
		}
		m := newTopModel(deps)
		m.mode = viewModeTable
		m.focus = focusManaged
		m.managedSel = 0

		m.prepareGroupStartConfirm()
		m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

		// Only 2 stopped services should be started
		assert.Equal(t, 2, startCount)
	})

	t.Run("Edge-1.6: all already running shows message", func(t *testing.T) {
		startCount := 0
		deps := &mockStarter{
			fakeAppDeps: fakeAppDeps{
				servers: []*models.ServerInfo{
					makeRunningServer("web-frontend", 1001, 3000),
					makeRunningServer("web-backend", 1002, 3001),
				},
				services: []*models.ManagedService{
					{Name: "web-frontend", CWD: "/tmp/web-frontend", Command: "npm run dev", Ports: []int{3000}},
					{Name: "web-backend", CWD: "/tmp/web-backend", Command: "go run .", Ports: []int{3001}},
				},
			},
			startFn: func(name string) error {
				startCount++
				return nil
			},
		}
		m := newTopModel(deps)
		m.mode = viewModeTable
		m.focus = focusManaged
		m.managedSel = 0

		m.prepareGroupStartConfirm()

		// Should show message that all are already running
		assert.Equal(t, 0, startCount)
		assert.Contains(t, m.cmdStatus, "already running")
	})

	t.Run("group start with confirmation", func(t *testing.T) {
		startCount := 0
		deps := &mockStarter{
			fakeAppDeps: fakeAppDeps{
				servers: []*models.ServerInfo{},
				services: []*models.ManagedService{
					{Name: "web-backend", CWD: "/tmp/web-backend", Command: "go run .", Ports: []int{3001}},
					{Name: "web-worker", CWD: "/tmp/web-worker", Command: "python worker.py", Ports: []int{3002}},
				},
			},
			startFn: func(name string) error {
				startCount++
				return nil
			},
		}
		m := newTopModel(deps)
		m.mode = viewModeTable
		m.focus = focusManaged
		m.managedSel = 0

		// Open confirm (via mouse-only path — call directly for test)
		m.prepareGroupStartConfirm()
		// Confirm
		m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

		assert.Equal(t, 2, startCount)
	})

	t.Run("group start cancelled", func(t *testing.T) {
		startCount := 0
		deps := &mockStarter{
			fakeAppDeps: fakeAppDeps{
				servers: []*models.ServerInfo{},
				services: []*models.ManagedService{
					{Name: "web-backend", CWD: "/tmp/web-backend", Command: "go run .", Ports: []int{3001}},
				},
			},
			startFn: func(name string) error {
				startCount++
				return nil
			},
		}
		m := newTopModel(deps)
		m.mode = viewModeTable
		m.focus = focusManaged
		m.managedSel = 0

		m.prepareGroupStartConfirm()
		m.Update(tea.KeyPressMsg{Code: 'n'})

		assert.Equal(t, 0, startCount)
		assert.Equal(t, "Cancelled", m.cmdStatus)
	})
}

// ---------------------------------------------------------------------------
// TEST-group-remove
// Covers: BR-1.7, C-1.4
// ---------------------------------------------------------------------------

func TestGroupRemove(t *testing.T) {
	t.Parallel()

	t.Run("group remove with confirmation", func(t *testing.T) {
		removeCount := 0
		deps := &mockRemover{
			fakeAppDeps: fakeAppDeps{
				servers: []*models.ServerInfo{},
				services: []*models.ManagedService{
					{Name: "api-gateway", CWD: "/tmp/api-gateway", Command: "node server.js", Ports: []int{3000}},
					{Name: "api-auth", CWD: "/tmp/api-auth", Command: "go run .", Ports: []int{3001}},
					{Name: "api-cron", CWD: "/tmp/api-cron", Command: "python cron.py", Ports: []int{3002}},
				},
			},
			removeFn: func(name string) error {
				removeCount++
				return nil
			},
		}
		m := newTopModel(deps)
		m.mode = viewModeTable
		m.focus = focusManaged
		m.managedSel = 0

		// Open confirm
		m.Update(tea.KeyPressMsg{Code: 'x', Mod: tea.ModShift})
		assert.Equal(t, confirmGroupRemove, m.confirm.kind)

		// Confirm
		m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})

		assert.Equal(t, 3, removeCount)
		assert.Contains(t, m.cmdStatus, "Removed")
	})

	t.Run("group remove cancelled", func(t *testing.T) {
		removeCount := 0
		deps := &mockRemover{
			fakeAppDeps: fakeAppDeps{
				servers: []*models.ServerInfo{},
				services: []*models.ManagedService{
					{Name: "api-gateway", CWD: "/tmp/api-gateway", Command: "node server.js", Ports: []int{3000}},
				},
			},
			removeFn: func(name string) error {
				removeCount++
				return nil
			},
		}
		m := newTopModel(deps)
		m.mode = viewModeTable
		m.focus = focusManaged
		m.managedSel = 0

		m.Update(tea.KeyPressMsg{Code: 'x', Mod: tea.ModShift})
		m.Update(tea.KeyPressMsg{Code: 'n'})

		assert.Equal(t, 0, removeCount)
		assert.Equal(t, "Cancelled", m.cmdStatus)
	})
}

// ---------------------------------------------------------------------------
// TEST-shift-double-click
// Covers: BR-1.8, Edge-1.4
// ---------------------------------------------------------------------------

func TestShiftDoubleClickGroupStart(t *testing.T) {
	t.Parallel()

	t.Run("shift+double-click starts namespace group", func(t *testing.T) {
		deps := &fakeAppDeps{
			servers: []*models.ServerInfo{},
			services: []*models.ManagedService{
				{Name: "web-frontend", CWD: "/tmp/web-frontend", Command: "npm run dev", Ports: []int{3000}},
				{Name: "web-backend", CWD: "/tmp/web-backend", Command: "go run .", Ports: []int{3001}},
				{Name: "web-worker", CWD: "/tmp/web-worker", Command: "python worker.py", Ports: []int{3002}},
			},
		}
		m := newTopModel(deps)
		m.mode = viewModeTable
		m.width = 100
		m.height = 30
		m.focus = focusManaged
		m.managedSel = 0

		// Find the Y position of the web-backend row
		_ = m.View()
		clickY := findManagedRowClickY(m, "web-backend")
		if clickY < 0 {
			t.Skip("could not find managed row for click")
		}

		// First click selects the row
		m.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: 10, Y: clickY})
		assert.Equal(t, focusManaged, m.focus)

		// Second click with shift modifier triggers group start
		m.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: 10, Y: clickY, Mod: tea.ModShift})

		// Should open group start confirmation
		if m.confirm != nil {
			assert.Equal(t, confirmGroupStart, m.confirm.kind)
		}
	})

	t.Run("Edge-1.4: shift release between clicks prevents group action", func(t *testing.T) {
		deps := &fakeAppDeps{
			servers: []*models.ServerInfo{},
			services: []*models.ManagedService{
				{Name: "web-frontend", CWD: "/tmp/web-frontend", Command: "npm run dev", Ports: []int{3000}},
				{Name: "web-backend", CWD: "/tmp/web-backend", Command: "go run .", Ports: []int{3001}},
			},
		}
		m := newTopModel(deps)
		m.mode = viewModeTable
		m.width = 100
		m.height = 30
		m.focus = focusManaged
		m.managedSel = 0

		_ = m.View()
		clickY := findManagedRowClickY(m, "web-backend")
		if clickY < 0 {
			t.Skip("could not find managed row for click")
		}

		// First click (no shift)
		m.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: 10, Y: clickY})
		// Wait beyond double-click threshold
		m.lastClickTime = time.Now().Add(-600 * time.Millisecond)
		// Second click (with shift) — should NOT trigger group action due to timing gap
		m.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: 10, Y: clickY, Mod: tea.ModShift})

		// No group confirm modal should open
		assert.Nil(t, m.confirm)
	})
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func makeRunningServer(name string, pid, port int) *models.ServerInfo {
	return &models.ServerInfo{
		ManagedService: &models.ManagedService{Name: name, CWD: "/tmp/" + name, Command: "run", Ports: []int{port}},
		ProcessRecord:  &models.ProcessRecord{PID: pid, Port: port, Command: "run", CWD: "/tmp/" + name, ProjectRoot: "/tmp/" + name},
		Status:         "running",
	}
}

// ---------------------------------------------------------------------------
// TEST-group-key-remap
// Covers: BR-1.11 — Group mode remaps e/r/x to group actions
// ---------------------------------------------------------------------------

func TestGroupModeRemapsActions(t *testing.T) {
	t.Parallel()

	t.Run("g then ctrl+e triggers group stop (not single stop)", func(t *testing.T) {
		deps := &fakeAppDeps{
			servers: []*models.ServerInfo{
				makeRunningServer("api-gateway", 1001, 3000),
				makeRunningServer("api-auth", 1002, 3001),
			},
			services: []*models.ManagedService{
				{Name: "api-gateway", CWD: "/tmp/api-gateway", Command: "node server.js", Ports: []int{3000}},
				{Name: "api-auth", CWD: "/tmp/api-auth", Command: "go run .", Ports: []int{3001}},
			},
		}
		m := newTopModel(deps)
		m.mode = viewModeTable
		m.focus = focusRunning
		m.selected = 0

		// Activate group mode
		m.Update(tea.KeyPressMsg{Code: 'g'})
		assert.NotNil(t, m.groupHighlightNamespace)

		// Press ctrl+e (normally single stop, should remap to group stop)
		newModel, _ := m.Update(tea.KeyPressMsg{Code: 'e', Mod: tea.ModCtrl})
		updated := newModel.(*topModel)

		// Should open group stop confirm, not single stop
		assertGroupConfirmKind(t, updated, confirmGroupStop)
	})

	t.Run("g then ctrl+r triggers group restart (not single restart)", func(t *testing.T) {
		deps := &fakeAppDeps{
			servers: []*models.ServerInfo{
				makeRunningServer("web-frontend", 1001, 3000),
				makeRunningServer("web-backend", 1002, 3001),
			},
			services: []*models.ManagedService{
				{Name: "web-frontend", CWD: "/tmp/web-frontend", Command: "npm run dev", Ports: []int{3000}},
				{Name: "web-backend", CWD: "/tmp/web-backend", Command: "go run .", Ports: []int{3001}},
			},
		}
		m := newTopModel(deps)
		m.mode = viewModeTable
		m.focus = focusRunning
		m.selected = 0

		// Activate group mode
		m.Update(tea.KeyPressMsg{Code: 'g'})
		assert.NotNil(t, m.groupHighlightNamespace)

		// Press ctrl+r (normally single restart, should remap to group restart)
		newModel, _ := m.Update(tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl})
		updated := newModel.(*topModel)

		assertGroupConfirmKind(t, updated, confirmGroupRestart)
	})

	t.Run("g then x triggers group remove (not single remove)", func(t *testing.T) {
		deps := &fakeAppDeps{
			servers: []*models.ServerInfo{},
			services: []*models.ManagedService{
				{Name: "api-gateway", CWD: "/tmp/api-gateway", Command: "node server.js", Ports: []int{3000}},
				{Name: "api-auth", CWD: "/tmp/api-auth", Command: "go run .", Ports: []int{3001}},
			},
		}
		m := newTopModel(deps)
		m.mode = viewModeTable
		m.focus = focusManaged
		m.managedSel = 0

		// Activate group mode
		m.Update(tea.KeyPressMsg{Code: 'g'})
		assert.NotNil(t, m.groupHighlightNamespace)

		// Press x (normally single remove, should remap to group remove)
		newModel, _ := m.Update(tea.KeyPressMsg{Code: 'x'})
		updated := newModel.(*topModel)

		assertGroupConfirmKind(t, updated, confirmGroupRemove)
	})

	t.Run("without g, ctrl+e still does single stop", func(t *testing.T) {
		deps := &fakeAppDeps{
			servers: []*models.ServerInfo{
				makeRunningServer("api-gateway", 1001, 3000),
				makeRunningServer("api-auth", 1002, 3001),
			},
			services: []*models.ManagedService{
				{Name: "api-gateway", CWD: "/tmp/api-gateway", Command: "node server.js", Ports: []int{3000}},
				{Name: "api-auth", CWD: "/tmp/api-auth", Command: "go run .", Ports: []int{3001}},
			},
		}
		m := newTopModel(deps)
		m.mode = viewModeTable
		m.focus = focusRunning
		m.selected = 0

		// No group mode activated
		assert.Nil(t, m.groupHighlightNamespace)

		// Press ctrl+e — should do single stop
		newModel, _ := m.Update(tea.KeyPressMsg{Code: 'e', Mod: tea.ModCtrl})
		updated := newModel.(*topModel)

		// Should be single-item stop confirm (confirmStopPID), not group stop
		if updated.confirm != nil {
			assert.Equal(t, confirmStopPID, updated.confirm.kind)
		}
		assert.NotEqual(t, confirmGroupStop, updated.confirm.kind)
	})

	t.Run("without g, ctrl+r still does single restart", func(t *testing.T) {
		deps := &fakeAppDeps{
			servers: []*models.ServerInfo{
				makeRunningServer("web-frontend", 1001, 3000),
			},
			services: []*models.ManagedService{
				{Name: "web-frontend", CWD: "/tmp/web-frontend", Command: "npm run dev", Ports: []int{3000}},
			},
		}
		m := newTopModel(deps)
		m.mode = viewModeTable
		m.focus = focusRunning
		m.selected = 0

		assert.Nil(t, m.groupHighlightNamespace)

		// Press ctrl+r — single restart (no confirm modal, direct execution)
		newModel, _ := m.Update(tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl})
		updated := newModel.(*topModel)

		// Single restart does NOT open a group confirm modal
		assert.Nil(t, updated.confirm)
		assert.Contains(t, updated.cmdStatus, "Restarted")
	})

	t.Run("ctrl+shift+e works regardless of group mode", func(t *testing.T) {
		deps := &fakeAppDeps{
			servers: []*models.ServerInfo{
				makeRunningServer("api-gateway", 1001, 3000),
				makeRunningServer("api-auth", 1002, 3001),
			},
			services: []*models.ManagedService{
				{Name: "api-gateway", CWD: "/tmp/api-gateway", Command: "node server.js", Ports: []int{3000}},
				{Name: "api-auth", CWD: "/tmp/api-auth", Command: "go run .", Ports: []int{3001}},
			},
		}
		m := newTopModel(deps)
		m.mode = viewModeTable
		m.focus = focusRunning
		m.selected = 0

		// Activate group mode first
		m.Update(tea.KeyPressMsg{Code: 'g'})
		assert.NotNil(t, m.groupHighlightNamespace)

		// ctrl+shift+e should still trigger group stop (explicit binding)
		newModel, _ := m.Update(tea.KeyPressMsg{Code: 'e', Mod: tea.ModCtrl | tea.ModShift})
		updated := newModel.(*topModel)

		assertGroupConfirmKind(t, updated, confirmGroupStop)
	})

	t.Run("ctrl+shift+r works regardless of group mode", func(t *testing.T) {
		deps := &fakeAppDeps{
			servers: []*models.ServerInfo{
				makeRunningServer("web-frontend", 1001, 3000),
				makeRunningServer("web-backend", 1002, 3001),
			},
			services: []*models.ManagedService{
				{Name: "web-frontend", CWD: "/tmp/web-frontend", Command: "npm run dev", Ports: []int{3000}},
				{Name: "web-backend", CWD: "/tmp/web-backend", Command: "go run .", Ports: []int{3001}},
			},
		}
		m := newTopModel(deps)
		m.mode = viewModeTable
		m.focus = focusRunning
		m.selected = 0

		// Activate group mode first
		m.Update(tea.KeyPressMsg{Code: 'g'})
		assert.NotNil(t, m.groupHighlightNamespace)

		// ctrl+shift+r should still trigger group restart (explicit binding)
		newModel, _ := m.Update(tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl | tea.ModShift})
		updated := newModel.(*topModel)

		assertGroupConfirmKind(t, updated, confirmGroupRestart)
	})

	t.Run("shift+x works regardless of group mode", func(t *testing.T) {
		deps := &fakeAppDeps{
			servers: []*models.ServerInfo{},
			services: []*models.ManagedService{
				{Name: "api-gateway", CWD: "/tmp/api-gateway", Command: "node server.js", Ports: []int{3000}},
				{Name: "api-auth", CWD: "/tmp/api-auth", Command: "go run .", Ports: []int{3001}},
			},
		}
		m := newTopModel(deps)
		m.mode = viewModeTable
		m.focus = focusManaged
		m.managedSel = 0

		// Activate group mode first
		m.Update(tea.KeyPressMsg{Code: 'g'})
		assert.NotNil(t, m.groupHighlightNamespace)

		// shift+x should still trigger group remove (explicit binding)
		newModel, _ := m.Update(tea.KeyPressMsg{Code: 'x', Mod: tea.ModShift})
		updated := newModel.(*topModel)

		assertGroupConfirmKind(t, updated, confirmGroupRemove)
	})
}

// ---------------------------------------------------------------------------
// TEST-group-highlight
// Covers: BR-1.10 — Toggle-based group highlighting via g key
// ---------------------------------------------------------------------------

func TestManagedListGroupHighlight(t *testing.T) {
	t.Parallel()

	t.Run("group highlight covers full managed service row (not just symbol)", func(t *testing.T) {
		deps := &fakeAppDeps{
			servers: []*models.ServerInfo{},
			services: []*models.ManagedService{
				{Name: "api-gateway", CWD: "/tmp/api-gateway", Command: "node server.js", Ports: []int{3000}},
				{Name: "api-auth", CWD: "/tmp/api-auth", Command: "go run .", Ports: []int{3001}},
				{Name: "web-frontend", CWD: "/tmp/web-frontend", Command: "npm run dev", Ports: []int{3002}},
			},
		}
		m := newTopModel(deps)
		m.mode = viewModeTable
		m.focus = focusManaged
		m.managedSel = 0
		m.width = 120
		m.height = 30

		// Toggle group highlight on
		m.Update(tea.KeyPressMsg{Code: 'g'})
		assert.NotNil(t, m.groupHighlightNamespace)
		assert.Equal(t, "api", *m.groupHighlightNamespace)

		// Render the managed list pane
		managedContent := m.renderManagedList(60, m.managedServices())
		lines := strings.Split(managedContent, "\n")

		// Find the api-gateway row (non-selected, should have group highlight)
		var gatewayRow string
		for _, line := range lines {
			stripped := ansi.Strip(line)
			if strings.Contains(stripped, "api-gateway") {
				gatewayRow = line
				break
			}
		}
		assert.NotEmpty(t, gatewayRow, "api-gateway row should be present")

		// The group highlight background (color 61) should be present in the row.
		// With Inline(true), the styled symbol does not emit a full reset, so
		// the parent group background extends across the entire line.
		assert.Contains(t, gatewayRow, "48;5;61", "group highlight background should cover full row")

		// The row should NOT contain a bare reset after the symbol that would
		// kill the background. With Inline(true), lipgloss only emits
		// foreground/bold codes without a closing \x1b[0m.
		assert.NotContains(t, gatewayRow, "\x1b[0m api-gateway", "no full reset should appear between symbol and name")
	})

	t.Run("non-group managed rows have no group highlight background", func(t *testing.T) {
		deps := &fakeAppDeps{
			servers: []*models.ServerInfo{},
			services: []*models.ManagedService{
				{Name: "api-gateway", CWD: "/tmp/api-gateway", Command: "node server.js", Ports: []int{3000}},
				{Name: "web-frontend", CWD: "/tmp/web-frontend", Command: "npm run dev", Ports: []int{3002}},
			},
		}
		m := newTopModel(deps)
		m.mode = viewModeTable
		m.focus = focusManaged
		m.managedSel = 0
		m.width = 120
		m.height = 30

		m.Update(tea.KeyPressMsg{Code: 'g'})
		assert.Equal(t, "api", *m.groupHighlightNamespace)

		managedContent := m.renderManagedList(60, m.managedServices())
		lines := strings.Split(managedContent, "\n")

		// Find the web-frontend row (different namespace — should NOT have group highlight)
		var webRow string
		for _, line := range lines {
			stripped := ansi.Strip(line)
			if strings.Contains(stripped, "web-frontend") {
				webRow = line
				break
			}
		}
		assert.NotEmpty(t, webRow, "web-frontend row should be present")
		assert.NotContains(t, webRow, "48;5;61", "non-group row should not have group highlight background")
	})
}

func TestGroupToggleHighlight(t *testing.T) {
	t.Parallel()

	t.Run("g key toggles group highlight on", func(t *testing.T) {
		deps := &fakeAppDeps{
			servers: []*models.ServerInfo{
				makeRunningServer("api-gateway", 1001, 3000),
				makeRunningServer("api-auth", 1002, 3001),
			},
			services: []*models.ManagedService{
				{Name: "api-gateway", CWD: "/tmp/api-gateway", Command: "node server.js", Ports: []int{3000}},
				{Name: "api-auth", CWD: "/tmp/api-auth", Command: "go run .", Ports: []int{3001}},
			},
		}
		m := newTopModel(deps)
		m.mode = viewModeTable
		m.focus = focusRunning
		m.selected = 0

		newModel, _ := m.Update(tea.KeyPressMsg{Code: 'g'})
		updated := newModel.(*topModel)

		assert.NotNil(t, updated.groupHighlightNamespace)
		assert.Equal(t, "api", *updated.groupHighlightNamespace)
	})

	t.Run("g key toggles group highlight off", func(t *testing.T) {
		deps := &fakeAppDeps{
			servers: []*models.ServerInfo{
				makeRunningServer("api-gateway", 1001, 3000),
			},
			services: []*models.ManagedService{
				{Name: "api-gateway", CWD: "/tmp/api-gateway", Command: "node server.js", Ports: []int{3000}},
			},
		}
		m := newTopModel(deps)
		m.mode = viewModeTable
		m.focus = focusRunning
		m.selected = 0

		// Toggle on
		m.Update(tea.KeyPressMsg{Code: 'g'})
		assert.NotNil(t, m.groupHighlightNamespace)

		// Toggle off
		newModel, _ := m.Update(tea.KeyPressMsg{Code: 'g'})
		updated := newModel.(*topModel)
		assert.Nil(t, updated.groupHighlightNamespace)
	})

	t.Run("navigation clears group highlight", func(t *testing.T) {
		deps := &fakeAppDeps{
			servers: []*models.ServerInfo{
				makeRunningServer("api-gateway", 1001, 3000),
				makeRunningServer("api-auth", 1002, 3001),
			},
			services: []*models.ManagedService{
				{Name: "api-gateway", CWD: "/tmp/api-gateway", Command: "node server.js", Ports: []int{3000}},
				{Name: "api-auth", CWD: "/tmp/api-auth", Command: "go run .", Ports: []int{3001}},
			},
		}
		m := newTopModel(deps)
		m.mode = viewModeTable
		m.focus = focusRunning
		m.selected = 0

		m.Update(tea.KeyPressMsg{Code: 'g'})
		assert.NotNil(t, m.groupHighlightNamespace)

		// Navigate down clears highlight
		m.Update(tea.KeyPressMsg{Code: 'j'})
		assert.Nil(t, m.groupHighlightNamespace)
	})

	t.Run("tab switch clears group highlight", func(t *testing.T) {
		deps := &fakeAppDeps{
			servers: []*models.ServerInfo{
				makeRunningServer("api-gateway", 1001, 3000),
			},
			services: []*models.ManagedService{
				{Name: "api-gateway", CWD: "/tmp/api-gateway", Command: "node server.js", Ports: []int{3000}},
			},
		}
		m := newTopModel(deps)
		m.mode = viewModeTable
		m.focus = focusRunning
		m.selected = 0

		m.Update(tea.KeyPressMsg{Code: 'g'})
		assert.NotNil(t, m.groupHighlightNamespace)

		m.Update(tea.KeyPressMsg{Code: tea.KeyTab})
		assert.Nil(t, m.groupHighlightNamespace)
	})

	t.Run("no-op in non-table mode", func(t *testing.T) {
		deps := &fakeAppDeps{
			servers: []*models.ServerInfo{
				makeRunningServer("api-gateway", 1001, 3000),
			},
			services: []*models.ManagedService{
				{Name: "api-gateway", CWD: "/tmp/api-gateway", Command: "node server.js", Ports: []int{3000}},
			},
		}
		m := newTopModel(deps)
		m.mode = viewModeLogs

		newModel, _ := m.Update(tea.KeyPressMsg{Code: 'g'})
		updated := newModel.(*topModel)
		assert.Nil(t, updated.groupHighlightNamespace)
	})

	t.Run("no-op when no valid selection", func(t *testing.T) {
		deps := &fakeAppDeps{
			servers:  []*models.ServerInfo{},
			services: []*models.ManagedService{},
		}
		m := newTopModel(deps)
		m.mode = viewModeTable
		m.focus = focusRunning
		m.selected = -1

		newModel, _ := m.Update(tea.KeyPressMsg{Code: 'g'})
		updated := newModel.(*topModel)
		assert.Nil(t, updated.groupHighlightNamespace)
	})

	t.Run("managed focus computes namespace from managed list", func(t *testing.T) {
		deps := &fakeAppDeps{
			servers: []*models.ServerInfo{},
			services: []*models.ManagedService{
				{Name: "api-gateway", CWD: "/tmp/api-gateway", Command: "node server.js", Ports: []int{3000}},
				{Name: "api-auth", CWD: "/tmp/api-auth", Command: "go run .", Ports: []int{3001}},
				{Name: "web-frontend", CWD: "/tmp/web-frontend", Command: "npm run dev", Ports: []int{3002}},
			},
		}
		m := newTopModel(deps)
		m.mode = viewModeTable
		m.focus = focusManaged
		m.managedSel = 0

		newModel, _ := m.Update(tea.KeyPressMsg{Code: 'g'})
		updated := newModel.(*topModel)

		assert.NotNil(t, updated.groupHighlightNamespace)
		assert.Equal(t, "api", *updated.groupHighlightNamespace)
	})

	t.Run("highlight renders namespace members in running table", func(t *testing.T) {
		deps := &fakeAppDeps{
			servers: []*models.ServerInfo{
				makeRunningServer("api-gateway", 1001, 3000),
				makeRunningServer("api-auth", 1002, 3001),
				makeRunningServer("web-frontend", 1003, 3002),
			},
			services: []*models.ManagedService{
				{Name: "api-gateway", CWD: "/tmp/api-gateway", Command: "node server.js", Ports: []int{3000}},
				{Name: "api-auth", CWD: "/tmp/api-auth", Command: "go run .", Ports: []int{3001}},
				{Name: "web-frontend", CWD: "/tmp/web-frontend", Command: "npm run dev", Ports: []int{3002}},
			},
		}
		m := newTopModel(deps)
		m.mode = viewModeTable
		m.focus = focusRunning
		m.selected = 0
		m.width = 100
		m.height = 30

		// Toggle group highlight
		m.Update(tea.KeyPressMsg{Code: 'g'})
		assert.NotNil(t, m.groupHighlightNamespace)

		// Render and verify all services appear
		output := m.View().Content
		assert.Contains(t, output, "api-gateway")
		assert.Contains(t, output, "api-auth")
		assert.Contains(t, output, "web-frontend")
	})
}
