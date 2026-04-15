package cli

import (
	"bytes"
	"fmt"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/devports/devpt/pkg/models"
	"github.com/devports/devpt/pkg/process"
	"github.com/devports/devpt/pkg/registry"
	"github.com/devports/devpt/pkg/scanner"
)

func TestTUIAdapterLatestServiceLogPath_ReturnsManagedLogFile(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	reg := registry.NewRegistry(filepath.Join(tmp, "registry.json"))
	if err := reg.Load(); err != nil {
		t.Fatalf("load registry: %v", err)
	}

	now := time.Now()
	port := reserveTestPort(t)
	if err := reg.AddService(&models.ManagedService{
		Name:      "worker",
		CWD:       tmp,
		Command:   fmt.Sprintf("/usr/bin/python3 -m http.server %d --bind 127.0.0.1", port),
		Ports:     []int{port},
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("add service: %v", err)
	}

	app := &App{
		registry:       reg,
		scanner:        scanner.NewProcessScanner(),
		resolver:       scanner.NewProjectResolver(),
		detector:       scanner.NewAgentDetector(),
		processManager: process.NewManager(filepath.Join(tmp, "logs")),
	}

	// Ensure cleanup runs even if test fails mid-flight
	t.Cleanup(func() {
		svc := reg.GetService("worker")
		if svc != nil && svc.LastPID != nil && *svc.LastPID > 0 {
			if err := app.processManager.Stop(*svc.LastPID, 2*time.Second); err != nil && err != process.ErrNeedSudo {
				t.Logf("cleanup stop pid %d: %v", *svc.LastPID, err)
			}
		}
	})

	if err := app.StartCmd("worker"); err != nil {
		t.Fatalf("start service: %v", err)
	}
	waitForTCPListener(t, port)

	adapter, ok := NewTUIAdapter(app).(tuiAdapter)
	if !ok {
		t.Fatalf("expected tuiAdapter type")
	}

	logPath, err := adapter.LatestServiceLogPath("worker")
	if err != nil {
		t.Fatalf("latest log path: %v", err)
	}
	if logPath == "" {
		t.Fatalf("expected non-empty log path")
	}

	svc := reg.GetService("worker")
	if svc == nil || svc.LastPID == nil || *svc.LastPID <= 0 {
		t.Fatalf("expected started service PID, got %#v", svc)
	}
}

func TestTUIAdapterRestartCmd_SuppressesCLIProgressOutput(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	reg := registry.NewRegistry(filepath.Join(tmp, "registry.json"))
	if err := reg.Load(); err != nil {
		t.Fatalf("load registry: %v", err)
	}

	now := time.Now()
	port := reserveTestPort(t)
	if err := reg.AddService(&models.ManagedService{
		Name:      "worker",
		CWD:       tmp,
		Command:   fmt.Sprintf("/usr/bin/python3 -m http.server %d --bind 127.0.0.1", port),
		Ports:     []int{port},
		CreatedAt: now,
		UpdatedAt: now,
	}); err != nil {
		t.Fatalf("add service: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	app := &App{
		registry:       reg,
		scanner:        scanner.NewProcessScanner(),
		resolver:       scanner.NewProjectResolver(),
		detector:       scanner.NewAgentDetector(),
		processManager: process.NewManager(filepath.Join(tmp, "logs")),
		stdout:         &stdout,
		stderr:         &stderr,
	}

	// Ensure cleanup runs even if test fails mid-flight
	t.Cleanup(func() {
		svc := reg.GetService("worker")
		if svc != nil && svc.LastPID != nil && *svc.LastPID > 0 {
			if err := app.processManager.Stop(*svc.LastPID, 2*time.Second); err != nil && err != process.ErrNeedSudo {
				t.Logf("cleanup stop pid %d: %v", *svc.LastPID, err)
			}
		}
	})

	if err := app.StartCmd("worker"); err != nil {
		t.Fatalf("start service: %v", err)
	}
	waitForTCPListener(t, port)

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
	if err := adapter.RestartService("worker"); err != nil {
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
}

func reserveTestPort(t *testing.T) int {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve port: %v", err)
	}
	defer ln.Close()

	addr, ok := ln.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatalf("unexpected listener address type: %T", ln.Addr())
	}
	return addr.Port
}

func waitForTCPListener(t *testing.T, port int) {
	t.Helper()

	deadline := time.Now().Add(8 * time.Second)
	address := fmt.Sprintf("127.0.0.1:%d", port)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", address, 100*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return
		}
		time.Sleep(50 * time.Millisecond)
	}

	t.Fatalf("listener on %s did not become ready", address)
}
