package cli

import (
	"bytes"
	"path/filepath"
	"testing"
	"time"

	"github.com/devports/devpt/pkg/models"
	"github.com/devports/devpt/pkg/process"
	"github.com/devports/devpt/pkg/registry"
)

func TestTUIAdapterRestartCmd_SuppressesCLIProgressOutput(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	reg := registry.NewRegistry(filepath.Join(tmp, "registry.json"))
	if err := reg.Load(); err != nil {
		t.Fatalf("load registry: %v", err)
	}

	now := time.Now()
	if err := reg.AddService(&models.ManagedService{
		Name:      "worker",
		CWD:       tmp,
		Command:   "/bin/sleep 5",
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("add service: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	app := &App{
		registry:       reg,
		processManager: process.NewManager(filepath.Join(tmp, "logs")),
		stdout:         &stdout,
		stderr:         &stderr,
	}

	if err := app.StartCmd("worker"); err != nil {
		t.Fatalf("start service: %v", err)
	}

	svc := reg.GetService("worker")
	if svc == nil || svc.LastPID == nil || *svc.LastPID <= 0 {
		t.Fatalf("expected started service PID, got %#v", svc)
	}
	startPID := *svc.LastPID

	stdout.Reset()
	stderr.Reset()

	adapter, ok := NewTUIAdapter(app).(tuiAdapter)
	if !ok {
		t.Fatalf("expected tuiAdapter type")
	}
	if err := adapter.RestartCmd("worker"); err != nil {
		t.Fatalf("restart via TUI adapter: %v", err)
	}

	if stdout.Len() != 0 {
		t.Fatalf("expected no stdout leakage during TUI restart, got: %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr leakage during TUI restart, got: %q", stderr.String())
	}

	svc = reg.GetService("worker")
	if svc == nil || svc.LastPID == nil || *svc.LastPID <= 0 {
		t.Fatalf("expected restarted service PID, got %#v", svc)
	}
	if *svc.LastPID == startPID {
		t.Fatalf("expected restart to update PID, still %d", *svc.LastPID)
	}

	if err := app.processManager.Stop(*svc.LastPID, 2*time.Second); err != nil {
		t.Fatalf("cleanup stop: %v", err)
	}
}
