package tui

import (
	"fmt"
	"time"

	"github.com/devports/devpt/pkg/models"
)

type fakeAppDeps struct {
	servers           []*models.ServerInfo
	services          []*models.ManagedService
	logPaths          map[string]string
	listServicesCalls int
	discoverCalls     int
}

func newTestModel() *topModel {
	return newTopModel(&fakeAppDeps{
		servers: []*models.ServerInfo{
			{
				ProcessRecord: &models.ProcessRecord{
					PID:         1001,
					Port:        3000,
					Command:     "node server.js",
					CWD:         "/tmp/app",
					ProjectRoot: "/tmp/app",
				},
				Status: "running",
				Source: models.SourceManual,
			},
		},
	})
}

func (f *fakeAppDeps) DiscoverServers() ([]*models.ServerInfo, error) {
	f.discoverCalls++
	return f.servers, nil
}

func (f *fakeAppDeps) ListServices() []*models.ManagedService {
	f.listServicesCalls++
	return f.services
}

func (f *fakeAppDeps) GetService(name string) *models.ManagedService {
	for _, svc := range f.services {
		if svc.Name == name {
			return svc
		}
	}
	return nil
}

func (f *fakeAppDeps) ClearServicePID(string) error {
	return nil
}

func (f *fakeAppDeps) RegisterService(name, cwd, command string, ports []int) error {
	f.services = append(f.services, &models.ManagedService{Name: name, CWD: cwd, Command: command, Ports: ports})
	return nil
}

func (f *fakeAppDeps) RemoveService(name string) error {
	for i, svc := range f.services {
		if svc.Name == name {
			f.services = append(f.services[:i], f.services[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("service %q not found", name)
}

func (f *fakeAppDeps) StartService(string) error {
	return nil
}

func (f *fakeAppDeps) StopService(string) error {
	return nil
}

func (f *fakeAppDeps) RestartService(string) error {
	return nil
}

func (f *fakeAppDeps) StopProcess(int, time.Duration) error {
	return nil
}

func (f *fakeAppDeps) TailServiceLogs(string, int) ([]string, error) {
	return nil, nil
}

func (f *fakeAppDeps) TailProcessLogs(int, int) ([]string, error) {
	return nil, nil
}

func (f *fakeAppDeps) LatestServiceLogPath(name string) (string, error) {
	if path, ok := f.logPaths[name]; ok {
		return path, nil
	}
	return "", fmt.Errorf("no logs for %q", name)
}
