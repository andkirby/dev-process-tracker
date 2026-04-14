package cli

import (
	"fmt"
	"strconv"
	"strings"
	"github.com/devports/devpt/pkg/health"
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
func (a *App) StartCmd(name string) error {
	svc, errs := LookupServiceWithFallback(name, a.registry.ListServices())
	if svc == nil { return fmt.Errorf("service %q not found: %s", name, strings.Join(errs, "; ")) }
	fmt.Fprintf(a.outWriter(), "Starting %q...\n", svc.Name)
	pid, err := a.processManager.Start(svc)
	if err != nil { return fmt.Errorf("failed to start service: %w", err) }
	if err := a.registry.UpdateServicePID(svc.Name, pid); err != nil {
		fmt.Fprintf(a.errWriter(), "Warning: failed to update registry: %v\n", err)
	}
	fmt.Fprintf(a.outWriter(), "Started %q\n", svc.Name)
	return nil
}
func (a *App) StopCmd(identifier string) error {
	var targetPID int
	var svcName string
	if svc, _ := LookupServiceWithFallback(identifier, a.registry.ListServices()); svc != nil {
		svcName = svc.Name
		pid, err := a.validatedManagedPID(svc)
		if err != nil { return err }
		targetPID = pid
	} else {
		port, err := strconv.Atoi(identifier)
		if err != nil { return fmt.Errorf("invalid service name or port: %s", identifier) }
		servers, err := a.discoverServers()
		if err != nil { return err }
		for _, srv := range servers {
			if srv.ProcessRecord != nil && srv.ProcessRecord.Port == port {
				targetPID = srv.ProcessRecord.PID
				if srv.ManagedService != nil { svcName = srv.ManagedService.Name }
				break
			}
		}
		if targetPID == 0 { return fmt.Errorf("no process found on port %d", port) }
	}
	if targetPID == 0 { return fmt.Errorf("cannot determine PID to stop") }
	fmt.Fprintf(a.outWriter(), "Stopping PID %d...\n", targetPID)
	result := StopProcess(a.processManager, targetPID, defaultStopTimeout)
	if result.SudoRequired { return fmt.Errorf("requires sudo to terminate PID %d", targetPID) }
	if svcName != "" {
		if clrErr := a.registry.ClearServicePID(svcName); clrErr != nil {
			fmt.Fprintf(a.errWriter(), "Warning: failed to clear PID for %q: %v\n", svcName, clrErr)
		}
	}
	if result.AlreadyDead { return nil }
	if result.Stopped { fmt.Fprintf(a.outWriter(), "Process %d stopped\n", targetPID); return nil }
	if result.ClearError != nil { return result.ClearError }
	return fmt.Errorf("failed to stop process PID %d", targetPID)
}
func (a *App) RestartCmd(name string) error {
	svc, errs := LookupServiceWithFallback(name, a.registry.ListServices())
	if svc == nil { return fmt.Errorf("service %q not found: %s", name, strings.Join(errs, "; ")) }
	pid, err := a.validatedManagedPID(svc)
	if err != nil { return err }
	if pid > 0 {
		fmt.Fprintf(a.outWriter(), "Stopping service %q...\n", svc.Name)
		result := StopProcess(a.processManager, pid, defaultStopTimeout)
		if !result.Stopped && !result.AlreadyDead && result.ClearError != nil {
			fmt.Fprintf(a.errWriter(), "Warning: failed to stop service: %v\n", result.ClearError)
		}
	}
	fmt.Fprintf(a.outWriter(), "Starting %q...\n", svc.Name)
	newPID, err := a.processManager.Start(svc)
	if err != nil { return fmt.Errorf("failed to start service: %w", err) }
	if err := a.registry.UpdateServicePID(svc.Name, newPID); err != nil {
		fmt.Fprintf(a.errWriter(), "Warning: failed to update registry: %v\n", err)
	}
	fmt.Fprintf(a.outWriter(), "Restarted %q\n", svc.Name)
	return nil
}
func (a *App) BatchStartCmd(names []string) error {
	servers, _ := a.discoverServers()
	results := RunBatch(names, func(ctx BatchContext) BatchOpResult {
		pid, err := ValidateRunningPID(ctx.Service, servers, a.processManager.IsRunning)
		if err != nil { return BatchOpResult{Name: ctx.Name, Warning: err.Error()} }
		if pid > 0 { return BatchOpResult{Name: ctx.Name, Warning: fmt.Sprintf("already running (PID %d)", pid)} }
		startPID, err := a.processManager.Start(ctx.Service)
		if err != nil { return BatchOpResult{Name: ctx.Name, Error: fmt.Sprintf("failed to start: %v", err)} }
		a.registry.UpdateServicePID(ctx.Service.Name, startPID)
		return BatchOpResult{Name: ctx.Name, Success: true, PID: startPID}
	}, a.registry)
	return a.renderBatchResults(results)
}
func (a *App) BatchStopCmd(names []string) error {
	servers, _ := a.discoverServers()
	results := RunBatch(names, func(ctx BatchContext) BatchOpResult {
		pid, err := ValidateRunningPID(ctx.Service, servers, a.processManager.IsRunning)
		if err != nil { return BatchOpResult{Name: ctx.Name, Error: err.Error()} }
		if pid == 0 { return BatchOpResult{Name: ctx.Name, Warning: "not running"} }
		fmt.Printf("Stopping service %q (PID %d)...\n", ctx.Name, pid)
		result := StopProcess(a.processManager, pid, defaultStopTimeout)
		if result.SudoRequired { return BatchOpResult{Name: ctx.Name, Error: fmt.Sprintf("requires sudo (PID %d)", pid)} }
		a.registry.ClearServicePID(ctx.Service.Name)
		if result.AlreadyDead { return BatchOpResult{Name: ctx.Name, Success: true, Warning: "already stopped"} }
		if result.Stopped { return BatchOpResult{Name: ctx.Name, Success: true, PID: pid} }
		return BatchOpResult{Name: ctx.Name, Error: fmt.Sprintf("failed to stop: %v", result.ClearError)}
	}, a.registry)
	return a.renderBatchResults(results)
}
func (a *App) BatchRestartCmd(names []string) error {
	servers, _ := a.discoverServers()
	results := RunBatch(names, func(ctx BatchContext) BatchOpResult {
		pid, err := ValidateRunningPID(ctx.Service, servers, a.processManager.IsRunning)
		if err != nil { return BatchOpResult{Name: ctx.Name, Error: err.Error()} }
		if pid > 0 {
			fmt.Printf("Stopping service %q (PID %d)...\n", ctx.Name, pid)
			result := StopProcess(a.processManager, pid, defaultStopTimeout)
			if !result.Stopped && !result.AlreadyDead && result.ClearError != nil {
				fmt.Fprintf(a.errWriter(), "Warning: failed to stop %q: %v\n", ctx.Name, result.ClearError)
			}
		}
		startPID, err := a.processManager.Start(ctx.Service)
		if err != nil { return BatchOpResult{Name: ctx.Name, Error: fmt.Sprintf("failed to start: %v", err)} }
		a.registry.UpdateServicePID(ctx.Service.Name, startPID)
		return BatchOpResult{Name: ctx.Name, Success: true, PID: startPID}
	}, a.registry)
	return a.renderBatchResults(results)
}
func (a *App) renderBatchResults(results []BatchOpResult) error {
	var firstErr error
	for _, r := range results {
		switch {
		case r.Error != "":
			fmt.Fprintf(a.errWriter(), "Error: service %q: %s\n", r.Name, r.Error)
			if firstErr == nil { firstErr = fmt.Errorf("service %q: %s", r.Name, r.Error) }
		case r.Warning != "":
			fmt.Fprintf(a.errWriter(), "Warning: service %q: %s\n", r.Name, r.Warning)
		case r.Success:
			fmt.Fprintf(a.outWriter(), "Service %q succeeded\n", r.Name)
		}
	}
	return firstErr
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
func (a *App) validatedManagedPID(svc *models.ManagedService) (int, error) {
	servers, err := a.discoverServers()
	if err != nil { return 0, err }
	return ValidateRunningPID(svc, servers, a.processManager.IsRunning)
}
var _ = process.ErrNeedSudo
