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

func (m *topModel) countVisible() int { return len(m.visibleServers()) }

func (m *topModel) currentFilterQuery() string {
	if m.mode == viewModeSearch {
		return m.searchInput.Value()
	}
	return m.searchQuery
}

func (m *topModel) visibleServers() []*models.ServerInfo {
	q := strings.ToLower(strings.TrimSpace(m.currentFilterQuery()))
	if m.cachedVisible != nil &&
		m.cachedVisibleQuery == q &&
		m.cachedVisibleSortBy == m.sortBy &&
		m.cachedVisibleReverse == m.sortReverse &&
		m.cachedVisibleVersion == m.serversVersion {
		return m.cachedVisible
	}

	visible := make([]*models.ServerInfo, 0, len(m.servers))
	for _, srv := range m.servers {
		if srv == nil || srv.ProcessRecord == nil {
			continue
		}
		if srv.ManagedService == nil {
			if srv.ProcessRecord.Port == 0 || !isRuntimeCommand(srv.ProcessRecord.Command) {
				continue
			}
		}
		if q != "" && !matchesServerQuery(m, srv, q) {
			continue
		}
		visible = append(visible, srv)
	}
	m.sortServers(visible)
	m.cachedVisible = visible
	m.cachedVisibleQuery = q
	m.cachedVisibleSortBy = m.sortBy
	m.cachedVisibleReverse = m.sortReverse
	m.cachedVisibleVersion = m.serversVersion
	return visible
}

func (m *topModel) managedServices() []*models.ManagedService {
	q := strings.ToLower(strings.TrimSpace(m.currentFilterQuery()))
	if m.cachedManaged != nil &&
		m.cachedManagedQuery == q &&
		m.cachedManagedVersion == m.servicesVersion {
		return m.cachedManaged
	}

	services := m.app.ListServices()
	filtered := make([]*models.ManagedService, 0, len(services))
	for _, svc := range services {
		if q == "" || strings.Contains(strings.ToLower(svc.Name+" "+svc.CWD+" "+svc.Command), q) {
			filtered = append(filtered, svc)
		}
	}
	sort.Slice(filtered, func(i, j int) bool { return strings.ToLower(filtered[i].Name) < strings.ToLower(filtered[j].Name) })
	m.cachedManaged = filtered
	m.cachedManagedQuery = q
	m.cachedManagedVersion = m.servicesVersion
	return filtered
}

func matchesServerQuery(m *topModel, srv *models.ServerInfo, q string) bool {
	var b strings.Builder
	name := strings.ToLower(m.serviceNameFor(srv))
	project := strings.ToLower(projectOf(srv))
	command := strings.ToLower(srv.ProcessRecord.Command)
	cwd := strings.ToLower(srv.ProcessRecord.CWD)
	projectRoot := strings.ToLower(srv.ProcessRecord.ProjectRoot)
	port := strconv.Itoa(srv.ProcessRecord.Port)

	b.Grow(len(name) + len(project) + len(command) + len(port) + len(cwd) + len(projectRoot) + 5)
	b.WriteString(name)
	b.WriteByte(' ')
	b.WriteString(project)
	b.WriteByte(' ')
	b.WriteString(command)
	b.WriteByte(' ')
	b.WriteString(port)
	b.WriteByte(' ')
	b.WriteString(cwd)
	b.WriteByte(' ')
	b.WriteString(projectRoot)
	return strings.Contains(b.String(), q)
}

func (m *topModel) serviceNameFor(srv *models.ServerInfo) string {
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
		if err := m.app.RegisterService(name, cwd, cmd, ports); err != nil {
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
		if err := m.app.RegisterService(svc.Name, svc.CWD, svc.Command, svc.Ports); err != nil {
			return err.Error()
		}
		delete(m.removed, args[1])
		return fmt.Sprintf("Restored %q", args[1])
	case "start":
		if len(args) < 2 {
			return "Usage: start <name>"
		}
		if err := m.app.StartService(args[1]); err != nil {
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
			if err := m.app.StopService(args[2]); err != nil {
				return err.Error()
			}
			return fmt.Sprintf("Stopped port %s", args[2])
		}
		if err := m.app.StopService(args[1]); err != nil {
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
	if err := m.app.StartService(srv.ManagedService.Name); err != nil {
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
	if err := m.app.RestartService(srv.ManagedService.Name); err != nil {
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
		m.groupHighlightNamespace = nil
		m.cmdStatus = "Cancelled"
		return nil
	}
	switch c.kind {
	case confirmGroupStop, confirmGroupRestart, confirmGroupStart, confirmGroupRemove:
		m.groupHighlightNamespace = nil
		m.executeGroupConfirm(c)
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
		if err := m.app.RemoveService(c.name); err != nil {
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

// ---------------------------------------------------------------------------
// Group actions (namespace-based process clustering)
// ---------------------------------------------------------------------------

func (m *topModel) prepareGroupStopConfirm() {
	if m.mode != viewModeTable {
		return
	}
	namespace := namespaceOfSelected(m)
	m.groupHighlightNamespace = &namespace
	if namespace == "-" {
		return
	}
	group := groupForNamespace(m, namespace)
	if len(group) == 0 {
		m.cmdStatus = "No group members found for namespace \"" + namespace + "\""
		return
	}
	names := groupServiceNames(group)
	pids := groupPIDs(group)
	prompt := fmt.Sprintf("Stop %d process(es) in namespace \"%s\"?\n%s", len(group), namespace, strings.Join(names, ", "))
	m.openConfirmModal(&confirmState{
		kind:         confirmGroupStop,
		prompt:       prompt,
		namespace:    namespace,
		serviceNames: names,
		pids:         pids,
	})
}

func (m *topModel) prepareGroupRestartConfirm() {
	if m.mode != viewModeTable {
		return
	}
	namespace := namespaceOfSelected(m)
	m.groupHighlightNamespace = &namespace
	if namespace == "-" {
		return
	}

	// Find all namespace members: managed services (running, crashed, stopped)
	// plus any unmanaged running servers in the namespace.
	managed := m.managedServices()
	managedSet := make(map[string]bool)
	var toRestart []string
	var toStart []string
	var pids []int
	for _, svc := range managed {
		if extractNamespace(svc.Name) != namespace {
			continue
		}
		managedSet[svc.Name] = true
		if m.isServiceRunning(svc.Name) {
			toRestart = append(toRestart, svc.Name)
			for _, srv := range m.servers {
				if srv.ManagedService != nil && srv.ManagedService.Name == svc.Name && srv.ProcessRecord != nil && srv.ProcessRecord.PID > 0 {
					pids = append(pids, srv.ProcessRecord.PID)
				}
			}
		} else {
			toStart = append(toStart, svc.Name)
		}
	}

	// Also include unmanaged running servers in the namespace
	for _, srv := range m.visibleServers() {
		if srv == nil || srv.ProcessRecord == nil {
			continue
		}
		name := m.serviceNameFor(srv)
		if extractNamespace(name) != namespace {
			continue
		}
		if srv.ManagedService != nil {
			continue // already handled above
		}
		toRestart = append(toRestart, name)
		pids = append(pids, srv.ProcessRecord.PID)
	}

	if len(toRestart) == 0 && len(toStart) == 0 {
		m.cmdStatus = "No group members found for namespace \"" + namespace + "\""
		return
	}

	// Build descriptive prompt
	var parts []string
	allNames := append(toRestart, toStart...)
	if len(toRestart) > 0 {
		parts = append(parts, fmt.Sprintf("restart %d", len(toRestart)))
	}
	if len(toStart) > 0 {
		parts = append(parts, fmt.Sprintf("start %d stopped", len(toStart)))
	}
	prompt := fmt.Sprintf("%s service(s) in namespace \"%s\"?\n%s",
		strings.Join(parts, " and "),
		namespace,
		strings.Join(allNames, ", "))

	m.openConfirmModal(&confirmState{
		kind:         confirmGroupRestart,
		prompt:       prompt,
		namespace:    namespace,
		serviceNames: allNames,
		pids:         pids,
	})
}

func (m *topModel) prepareGroupStartConfirm() {
	if m.mode != viewModeTable {
		return
	}
	if m.focus == focusRunning {
		// C-1.5 / C-1.8: Shift+Enter on running list is no-op (view logs not groupable)
		return
	}
	namespace := namespaceOfSelected(m)
	m.groupHighlightNamespace = &namespace
	if namespace == "-" {
		return
	}

	// Group start targets only stopped managed services in the namespace
	managed := m.managedServices()
	var stopped []string
	for _, svc := range managed {
		if extractNamespace(svc.Name) != namespace {
			continue
		}
		if !m.isServiceRunning(svc.Name) {
			stopped = append(stopped, svc.Name)
		}
	}

	if len(stopped) == 0 {
		m.cmdStatus = "All services in namespace \"" + namespace + "\" are already running"
		return
	}

	prompt := fmt.Sprintf("Start %d stopped service(s) in namespace \"%s\"?\n%s", len(stopped), namespace, strings.Join(stopped, ", "))
	m.openConfirmModal(&confirmState{
		kind:         confirmGroupStart,
		prompt:       prompt,
		namespace:    namespace,
		serviceNames: stopped,
	})
}

func (m *topModel) prepareGroupRemoveConfirm() {
	if m.mode != viewModeTable {
		return
	}
	if m.focus != focusManaged {
		return
	}
	namespace := namespaceOfSelected(m)
	m.groupHighlightNamespace = &namespace
	if namespace == "-" {
		return
	}

	// Group remove targets all managed services in the namespace
	managed := m.managedServices()
	var targets []string
	for _, svc := range managed {
		if extractNamespace(svc.Name) == namespace {
			targets = append(targets, svc.Name)
		}
	}

	if len(targets) == 0 {
		m.cmdStatus = "No managed services found for namespace \"" + namespace + "\""
		return
	}

	prompt := fmt.Sprintf("Remove %d service(s) from registry in namespace \"%s\"?\n%s", len(targets), namespace, strings.Join(targets, ", "))
	m.openConfirmModal(&confirmState{
		kind:         confirmGroupRemove,
		prompt:       prompt,
		namespace:    namespace,
		serviceNames: targets,
	})
}

// executeGroupConfirm handles the confirmed group action by iterating over
// each member and calling the existing single-item functions.
func (m *topModel) executeGroupConfirm(c confirmState) {
	switch c.kind {
	case confirmGroupStop:
		var results []string
		for i, pid := range c.pids {
			name := ""
			if i < len(c.serviceNames) {
				name = c.serviceNames[i]
			}
			if err := m.app.StopProcess(pid, 5*time.Second); err != nil {
				if isProcessFinishedErr(err) {
					results = append(results, fmt.Sprintf("PID %d already exited", pid))
					if name != "" {
						_ = m.app.ClearServicePID(name)
					}
				} else {
					results = append(results, fmt.Sprintf("PID %d: %v", pid, err))
				}
			} else {
				results = append(results, fmt.Sprintf("Stopped PID %d", pid))
				if name != "" {
					_ = m.app.ClearServicePID(name)
				}
			}
		}
		m.cmdStatus = strings.Join(results, "; ")

	case confirmGroupRestart:
		var results []string
		for _, name := range c.serviceNames {
			if m.isServiceRunning(name) {
				if err := m.app.RestartService(name); err != nil {
					results = append(results, fmt.Sprintf("%s: %v", name, err))
				} else {
					results = append(results, fmt.Sprintf("Restarted %q", name))
					m.starting[name] = time.Now()
				}
			} else {
				// Stopped/crashed service — start it instead
				if err := m.app.StartService(name); err != nil {
					results = append(results, fmt.Sprintf("%s: %v", name, err))
				} else {
					results = append(results, fmt.Sprintf("Started %q", name))
					m.starting[name] = time.Now()
				}
			}
		}
		m.cmdStatus = strings.Join(results, "; ")

	case confirmGroupStart:
		var results []string
		for _, name := range c.serviceNames {
			if err := m.app.StartService(name); err != nil {
				results = append(results, fmt.Sprintf("%s: %v", name, err))
			} else {
				results = append(results, fmt.Sprintf("Started %q", name))
				m.starting[name] = time.Now()
			}
		}
		m.cmdStatus = strings.Join(results, "; ")

	case confirmGroupRemove:
		var results []string
		for _, name := range c.serviceNames {
			svc := m.app.GetService(name)
			if svc != nil {
				copySvc := *svc
				m.removed[name] = &copySvc
			}
			if err := m.app.RemoveService(name); err != nil {
				results = append(results, fmt.Sprintf("%s: %v", name, err))
			} else {
				results = append(results, fmt.Sprintf("Removed %q", name))
			}
		}
		m.cmdStatus = strings.Join(results, "; ")
	}

	m.refresh()
}
