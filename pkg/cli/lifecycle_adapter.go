package cli

import (
	"os"
	"path/filepath"

	"github.com/devports/devpt/pkg/lifecycle"
	"github.com/devports/devpt/pkg/models"
)

// appDeps adapts the CLI App's existing infrastructure to the lifecycle.Deps interface.
type appDeps struct {
	app *App
}

func (d *appDeps) GetService(name string) *models.ManagedService {
	return d.app.registry.GetService(name)
}

func (d *appDeps) UpdateServicePID(name string, pid int) error {
	return d.app.registry.UpdateServicePID(name, pid)
}

func (d *appDeps) ClearServicePID(name string) error {
	return d.app.registry.ClearServicePID(name)
}

func (d *appDeps) StartProcess(svc *models.ManagedService) (int, error) {
	return d.app.processManager.Start(svc)
}

func (d *appDeps) StopProcess(pid int) error {
	result := StopProcess(d.app.processManager, pid, defaultStopTimeout)
	if result.ClearError != nil {
		return result.ClearError
	}
	return nil
}

func (d *appDeps) IsRunning(pid int) bool {
	return d.app.processManager.IsRunning(pid)
}

func (d *appDeps) ScanProcesses() ([]*models.ProcessRecord, error) {
	return d.app.scanner.ScanListeningPorts()
}

func (d *appDeps) ListServices() []*models.ManagedService {
	return d.app.registry.ListServices()
}

func (d *appDeps) CheckHealth(port int) bool {
	hc := d.app.healthChecker.Check(port)
	return hc.Status == "ok" || hc.Status == "slow"
}

func (d *appDeps) GetLogTail(name string, lines int) []string {
	logs, err := d.app.processManager.Tail(name, lines)
	if err != nil {
		return nil
	}
	return logs
}

func (d *appDeps) AcquireLock(serviceName string) error {
	lk := lifecycle.NewFileLock(d.lockDir())
	return lk.Acquire(serviceName, os.Getpid())
}

func (d *appDeps) ReleaseLock(serviceName string) {
	lk := lifecycle.NewFileLock(d.lockDir())
	_ = lk.Release(serviceName)
}

func (d *appDeps) ResolveProjectRoot(cwd string) string {
	return d.app.resolver.FindProjectRoot(cwd)
}

// lockDir returns the directory for lock files.
// Uses the config dir when available; otherwise derives from the registry
// file path so that tests with unique temp dirs get unique lock dirs.
func (d *appDeps) lockDir() string {
	if d.app.config.ConfigDir != "" {
		return d.app.config.ConfigDir
	}
	// Try to derive from registry file path
	if fp := d.app.registry.FilePath(); fp != "" {
		return filepath.Dir(fp)
	}
	return os.TempDir()
}
