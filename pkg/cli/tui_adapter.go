package cli

import (
	"io"
	"time"

	tuipkg "github.com/devports/devpt/pkg/cli/tui"
	"github.com/devports/devpt/pkg/models"
)

type tuiAdapter struct {
	app *App
}

func NewTUIAdapter(app *App) tuipkg.AppDeps {
	return tuiAdapter{app: app.withOutput(io.Discard, io.Discard)}
}

func (a tuiAdapter) DiscoverServers() ([]*models.ServerInfo, error) {
	return a.app.discoverServers()
}

func (a tuiAdapter) ListServices() []*models.ManagedService {
	return a.app.registry.ListServices()
}

func (a tuiAdapter) GetService(name string) *models.ManagedService {
	return a.app.registry.GetService(name)
}

func (a tuiAdapter) ClearServicePID(name string) error {
	return a.app.registry.ClearServicePID(name)
}

func (a tuiAdapter) AddCmd(name, cwd, command string, ports []int) error {
	return a.app.AddCmd(name, cwd, command, ports)
}

func (a tuiAdapter) RemoveCmd(name string) error {
	return a.app.RemoveCmd(name)
}

func (a tuiAdapter) StartCmd(name string) error {
	return a.app.StartCmd(name)
}

func (a tuiAdapter) StopCmd(identifier string) error {
	return a.app.StopCmd(identifier)
}

func (a tuiAdapter) RestartCmd(name string) error {
	return a.app.RestartCmd(name)
}

func (a tuiAdapter) StopProcess(pid int, timeout time.Duration) error {
	return a.app.processManager.Stop(pid, timeout)
}

func (a tuiAdapter) TailServiceLogs(name string, lines int) ([]string, error) {
	return a.app.processManager.Tail(name, lines)
}

func (a tuiAdapter) TailProcessLogs(pid int, lines int) ([]string, error) {
	return a.app.processManager.TailProcess(pid, lines)
}
