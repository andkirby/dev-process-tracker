package tui

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/devports/devpt/pkg/health"
	"github.com/devports/devpt/pkg/models"
	"github.com/devports/devpt/pkg/process"
)

func (m topModel) countVisible() int { return len(m.visibleServers()) }

func (m topModel) currentFilterQuery() string {
	if m.mode == viewModeSearch {
		return m.searchInput.Value()
	}
	return m.searchQuery
}

func (m topModel) visibleServers() []*models.ServerInfo {
	var visible []*models.ServerInfo
	q := strings.ToLower(strings.TrimSpace(m.currentFilterQuery()))
	for _, srv := range m.servers {
		if srv == nil || srv.ProcessRecord == nil {
			continue
		}
		if srv.ManagedService == nil {
			if srv.ProcessRecord.Port == 0 || !isRuntimeCommand(srv.ProcessRecord.Command) {
				continue
			}
		}
		if q != "" {
			hay := strings.ToLower(fmt.Sprintf("%s %s %s %d %s %s",
				m.serviceNameFor(srv), projectOf(srv), srv.ProcessRecord.Command, srv.ProcessRecord.Port, srv.ProcessRecord.CWD, srv.ProcessRecord.ProjectRoot))
			if !strings.Contains(hay, q) {
				continue
			}
		}
		visible = append(visible, srv)
	}
	m.sortServers(visible)
	return visible
}

func (m topModel) managedServices() []*models.ManagedService {
	services := m.app.ListServices()
	q := strings.ToLower(strings.TrimSpace(m.currentFilterQuery()))
	var filtered []*models.ManagedService
	for _, svc := range services {
		if q == "" || strings.Contains(strings.ToLower(svc.Name+" "+svc.CWD+" "+svc.Command), q) {
			filtered = append(filtered, svc)
		}
	}
	sort.Slice(filtered, func(i, j int) bool { return strings.ToLower(filtered[i].Name) < strings.ToLower(filtered[j].Name) })
	return filtered
}

func (m topModel) serviceNameFor(srv *models.ServerInfo) string {
	if srv == nil {
		return "-"
	}
	if srv.ManagedService != nil && srv.ManagedService.Name != "" {
		return srv.ManagedService.Name
	}
	if srv.ProcessRecord != nil {
		if srv.ProcessRecord.ProjectRoot != "" {
			return pathBase(srv.ProcessRecord.ProjectRoot)
		}
		if srv.ProcessRecord.CWD != "" {
			return pathBase(srv.ProcessRecord.CWD)
		}
		if srv.ProcessRecord.Command != "" {
			return pathBase(srv.ProcessRecord.Command)
		}
	}
	return "-"
}

func (m *topModel) runCommand(input string) string {
	if input == "" {
		return ""
	}
	args, err := parseArgs(input)
	if err != nil || len(args) == 0 {
		return "Invalid command"
	}
	switch args[0] {
	case "help":
		m.openHelpModal()
		return ""
	case "list":
		services := m.app.ListServices()
		if len(services) == 0 {
			return "No managed services"
		}
		names := make([]string, 0, len(services))
		for _, svc := range services {
			names = append(names, svc.Name)
		}
		sort.Strings(names)
		return "Managed services: " + strings.Join(names, ", ")
	case "add":
		if len(args) < 4 {
			return "Usage: add <name> <cwd> \"<cmd>\" [ports...]"
		}
		name, cwd, cmd := args[1], args[2], args[3]
		var ports []int
		for _, p := range args[4:] {
			port, perr := strconv.Atoi(p)
			if perr != nil {
				return "Invalid port: " + p
			}
			ports = append(ports, port)
		}
		if err := m.app.AddCmd(name, cwd, cmd, ports); err != nil {
			return err.Error()
		}
		return fmt.Sprintf("Added %q", name)
	case "remove", "rm":
		if len(args) < 2 {
			return "Usage: remove <name>"
		}
		svc := m.app.GetService(args[1])
		if svc == nil {
			return fmt.Sprintf("service %q not found", args[1])
		}
		m.openConfirmModal(&confirmState{kind: confirmRemoveService, prompt: fmt.Sprintf("Remove %q from registry?", svc.Name), name: svc.Name})
		return ""
	case "restore":
		if len(args) < 2 {
			return "Usage: restore <name>"
		}
		svc := m.removed[args[1]]
		if svc == nil {
			return fmt.Sprintf("no removed service %q in this session", args[1])
		}
		if err := m.app.AddCmd(svc.Name, svc.CWD, svc.Command, svc.Ports); err != nil {
			return err.Error()
		}
		delete(m.removed, args[1])
		return fmt.Sprintf("Restored %q", args[1])
	case "start":
		if len(args) < 2 {
			return "Usage: start <name>"
		}
		if err := m.app.StartCmd(args[1]); err != nil {
			return err.Error()
		}
		m.starting[args[1]] = time.Now()
		return fmt.Sprintf("Started %q", args[1])
	case "stop":
		if len(args) < 2 {
			return "Usage: stop <name|--port PORT>"
		}
		if args[1] == "--port" {
			if len(args) < 3 {
				return "Usage: stop --port PORT"
			}
			if err := m.app.StopCmd(args[2]); err != nil {
				return err.Error()
			}
			return fmt.Sprintf("Stopped port %s", args[2])
		}
		if err := m.app.StopCmd(args[1]); err != nil {
			return err.Error()
		}
		return fmt.Sprintf("Stopped %q", args[1])
	default:
		return "Unknown command (type :help)"
	}
}

func (m topModel) startSelected() string {
	visible := m.visibleServers()
	if m.selected < 0 || m.selected >= len(visible) {
		return "No service selected"
	}
	srv := visible[m.selected]
	if srv.ManagedService == nil {
		return "Selected process is not a managed service"
	}
	if err := m.app.StartCmd(srv.ManagedService.Name); err != nil {
		return err.Error()
	}
	m.starting[srv.ManagedService.Name] = time.Now()
	return fmt.Sprintf("Started %q", srv.ManagedService.Name)
}

func (m topModel) restartSelected() string {
	visible := m.visibleServers()
	if m.selected < 0 || m.selected >= len(visible) {
		return "No service selected"
	}
	srv := visible[m.selected]
	if srv.ManagedService == nil {
		return "Selected process is not a managed service"
	}
	if err := m.app.RestartCmd(srv.ManagedService.Name); err != nil {
		return err.Error()
	}
	m.starting[srv.ManagedService.Name] = time.Now()
	return fmt.Sprintf("Restarted %q", srv.ManagedService.Name)
}

func (m *topModel) prepareStopConfirm() {
	visible := m.visibleServers()
	if m.selected < 0 || m.selected >= len(visible) {
		m.cmdStatus = "No service selected"
		return
	}
	srv := visible[m.selected]
	if srv.ProcessRecord == nil || srv.ProcessRecord.PID == 0 {
		m.cmdStatus = "No PID to stop"
		return
	}
	prompt := fmt.Sprintf("Stop PID %d?", srv.ProcessRecord.PID)
	serviceName := ""
	if srv.ManagedService != nil {
		prompt = fmt.Sprintf("Stop %q (PID %d)?", srv.ManagedService.Name, srv.ProcessRecord.PID)
		serviceName = srv.ManagedService.Name
	}
	m.openConfirmModal(&confirmState{kind: confirmStopPID, prompt: prompt, pid: srv.ProcessRecord.PID, serviceName: serviceName})
}

func (m *topModel) executeConfirm(yes bool) tea.Cmd {
	if m.confirm == nil {
		m.closeModal()
		return nil
	}
	c := *m.confirm
	m.closeModal()
	if !yes {
		m.cmdStatus = "Cancelled"
		return nil
	}
	switch c.kind {
	case confirmStopPID:
		if err := m.app.StopProcess(c.pid, 5*time.Second); err != nil {
			if errors.Is(err, process.ErrNeedSudo) {
				m.openConfirmModal(&confirmState{kind: confirmSudoKill, prompt: fmt.Sprintf("Run sudo kill -9 %d now?", c.pid), pid: c.pid})
				return nil
			}
			if isProcessFinishedErr(err) {
				m.cmdStatus = fmt.Sprintf("Process %d already exited", c.pid)
				if c.serviceName != "" {
					_ = m.app.ClearServicePID(c.serviceName)
				}
			} else {
				m.cmdStatus = err.Error()
			}
		} else {
			m.cmdStatus = fmt.Sprintf("Stopped PID %d", c.pid)
			if c.serviceName != "" {
				if clrErr := m.app.ClearServicePID(c.serviceName); clrErr != nil {
					m.cmdStatus = fmt.Sprintf("Stopped PID %d (warning: %v)", c.pid, clrErr)
				}
			}
		}
	case confirmRemoveService:
		svc := m.app.GetService(c.name)
		if svc != nil {
			copySvc := *svc
			m.removed[c.name] = &copySvc
		}
		if err := m.app.RemoveCmd(c.name); err != nil {
			m.cmdStatus = err.Error()
		} else {
			m.cmdStatus = fmt.Sprintf("Removed %q (use :restore %s)", c.name, c.name)
		}
	case confirmSudoKill:
		m.cmdStatus = fmt.Sprintf("Run manually: sudo kill -9 %d", c.pid)
	}
	m.refresh()
	return nil
}

func (m topModel) tailLogsCmd() tea.Cmd {
	return func() tea.Msg {
		if m.logSvc != nil {
			lines, err := m.app.TailServiceLogs(m.logSvc.Name, 200)
			return logMsg{lines: lines, err: err}
		}
		if m.logPID > 0 {
			lines, err := m.app.TailProcessLogs(m.logPID, 200)
			return logMsg{lines: lines, err: err}
		}
		return logMsg{err: fmt.Errorf("no service selected")}
	}
}

func (m topModel) healthCmd() tea.Cmd {
	visible := m.visibleServers()
	return func() tea.Msg {
		icons := make(map[int]string)
		details := make(map[int]*health.HealthCheck)
		for _, srv := range visible {
			if srv.ProcessRecord == nil || srv.ProcessRecord.Port <= 0 {
				continue
			}
			check := m.healthChk.Check(srv.ProcessRecord.Port)
			icons[srv.ProcessRecord.Port] = health.StatusIcon(check.Status)
			details[srv.ProcessRecord.Port] = check
		}
		return healthMsg{icons: icons, details: details}
	}
}
