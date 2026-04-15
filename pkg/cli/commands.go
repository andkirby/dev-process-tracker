package cli

import (
	"fmt"
	"strconv"
	"strings"
	"github.com/devports/devpt/pkg/health"
	"github.com/devports/devpt/pkg/lifecycle"
	"github.com/devports/devpt/pkg/models"
	"github.com/devports/devpt/pkg/process"
)

func (a *App) ListCmd(detailed bool) error {
	servers, err := a.discoverServers()
	if err != nil { return err }
	return PrintServerTable(a.outWriter(), servers, detailed)
}
func (a *App) AddCmd(name, cwd, command string, ports []int) error {
	if err := validateManagedCommand(command); err != nil { return err }
	svc := &models.ManagedService{Name: name, CWD: cwd, Command: command, Ports: ports}
	if err := a.registry.AddService(svc); err != nil { return err }
	fmt.Fprintf(a.outWriter(), "Service %q registered successfully\n", name)
	return nil
}
func (a *App) RemoveCmd(name string) error { return a.registry.RemoveService(name) }

// lifecycleManager returns a lifecycle.LifecycleManager wired to the App's dependencies.
func (a *App) lifecycleManager() *lifecycle.LifecycleManager {
	return lifecycle.NewLifecycleManager(&appDeps{app: a})
}

func (a *App) StartCmd(name string) error {
	svc, errs := LookupServiceWithFallback(name, a.registry.ListServices())
	if svc == nil { return fmt.Errorf("service %q not found: %s", name, strings.Join(errs, "; ")) }

	mgr := a.lifecycleManager()
	result := mgr.Start(svc)

	fmt.Fprintln(a.outWriter(), result.Message)

	if result.Outcome == lifecycle.OutcomeFailed || result.Outcome == lifecycle.OutcomeInvalid || result.Outcome == lifecycle.OutcomeBlocked {
		return fmt.Errorf("%s", result.Message)
	}
	return nil
}

func (a *App) StopCmd(identifier string) error {
	// Try to resolve as a managed service first
	if svc, _ := LookupServiceWithFallback(identifier, a.registry.ListServices()); svc != nil {
		mgr := a.lifecycleManager()
		result := mgr.Stop(svc)

		fmt.Fprintln(a.outWriter(), result.Message)

		if result.Outcome == lifecycle.OutcomeFailed || result.Outcome == lifecycle.OutcomeInvalid || result.Outcome == lifecycle.OutcomeBlocked {
			return fmt.Errorf("%s", result.Message)
		}
		return nil
	}

	// Fall back to raw PID stop by port (for unmanaged/manual processes)
	port, err := strconv.Atoi(identifier)
	if err != nil { return fmt.Errorf("invalid service name or port: %s", identifier) }

	servers, err := a.discoverServers()
	if err != nil { return err }

	var targetPID int
	for _, srv := range servers {
		if srv.ProcessRecord != nil && srv.ProcessRecord.Port == port {
			targetPID = srv.ProcessRecord.PID
			break
		}
	}
	if targetPID == 0 { return fmt.Errorf("no process found on port %d", port) }

	fmt.Fprintf(a.outWriter(), "Stopping PID %d...\n", targetPID)
	result := StopProcess(a.processManager, targetPID, defaultStopTimeout)
	if result.SudoRequired { return fmt.Errorf("requires sudo to terminate PID %d", targetPID) }
	if result.AlreadyDead { return nil }
	if result.Stopped { fmt.Fprintf(a.outWriter(), "Process %d stopped\n", targetPID); return nil }
	if result.ClearError != nil { return result.ClearError }
	return fmt.Errorf("failed to stop process PID %d", targetPID)
}

func (a *App) RestartCmd(name string) error {
	svc, errs := LookupServiceWithFallback(name, a.registry.ListServices())
	if svc == nil { return fmt.Errorf("service %q not found: %s", name, strings.Join(errs, "; ")) }

	mgr := a.lifecycleManager()
	result := mgr.Restart(svc)

	fmt.Fprintln(a.outWriter(), result.Message)

	if result.Outcome == lifecycle.OutcomeFailed || result.Outcome == lifecycle.OutcomeInvalid || result.Outcome == lifecycle.OutcomeBlocked {
		return fmt.Errorf("%s", result.Message)
	}
	return nil
}

func (a *App) BatchStartCmd(names []string) error {
	mgr := a.lifecycleManager()
	summary := RunLifecycleBatch(names, mgr.Start, a.registry)
	fmt.Fprint(a.outWriter(), FormatBatchSummary(summary))
	if summary.Failed > 0 || summary.Invalid > 0 || summary.NotFound > 0 {
		return fmt.Errorf("batch start completed with %d failure(s)", summary.Failed+summary.Invalid+summary.NotFound)
	}
	return nil
}

func (a *App) BatchStopCmd(names []string) error {
	mgr := a.lifecycleManager()
	summary := RunLifecycleBatch(names, mgr.Stop, a.registry)
	fmt.Fprint(a.outWriter(), FormatBatchSummary(summary))
	if summary.Failed > 0 || summary.Invalid > 0 || summary.NotFound > 0 {
		return fmt.Errorf("batch stop completed with %d failure(s)", summary.Failed+summary.Invalid+summary.NotFound)
	}
	return nil
}

func (a *App) BatchRestartCmd(names []string) error {
	mgr := a.lifecycleManager()
	summary := RunLifecycleBatch(names, mgr.Restart, a.registry)
	fmt.Fprint(a.outWriter(), FormatBatchSummary(summary))
	if summary.Failed > 0 || summary.Invalid > 0 || summary.NotFound > 0 {
		return fmt.Errorf("batch restart completed with %d failure(s)", summary.Failed+summary.Invalid+summary.NotFound)
	}
	return nil
}

func (a *App) LogsCmd(name string, lines int) error {
	svc, errs := LookupServiceWithFallback(name, a.registry.ListServices())
	if svc == nil { return fmt.Errorf("service %q not found: %s", name, strings.Join(errs, "; ")) }
	logLines, err := a.processManager.Tail(svc.Name, lines)
	if err != nil { return err }
	fmt.Printf("Logs for service %q:\n", svc.Name)
	for _, line := range logLines { fmt.Println(line) }
	return nil
}
func (a *App) StatusCmd(identifiers []string) error {
	servers, err := a.discoverServers()
	if err != nil { return err }
	allServices := a.registry.ListServices()
	var matched []*models.ServerInfo
	for _, id := range identifiers {
		if strings.Contains(id, "*") {
			for _, name := range ExpandPatterns([]string{id}, allServices) {
				for _, srv := range servers {
					if srv.ManagedService != nil && srv.ManagedService.Name == name {
						matched = append(matched, srv); break
					}
				}
			}
		} else {
			for _, srv := range servers {
				if srv.ManagedService != nil && srv.ManagedService.Name == id { matched = append(matched, srv); break }
				if srv.ProcessRecord != nil && fmt.Sprintf("%d", srv.ProcessRecord.Port) == id { matched = append(matched, srv); break }
			}
		}
	}
	if len(matched) == 0 { return fmt.Errorf("no servers found matching %s", strings.Join(identifiers, ", ")) }
	for _, srv := range matched {
		var hc *health.HealthCheck
		if srv.ProcessRecord != nil { hc = a.healthChecker.Check(srv.ProcessRecord.Port) }
		if err := PrintServerStatus(a.outWriter(), srv, hc); err != nil { return err }
	}
	return nil
}

var _ = process.ErrNeedSudo
