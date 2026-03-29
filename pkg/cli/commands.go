package cli

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/devports/devpt/pkg/health"
	"github.com/devports/devpt/pkg/models"
	"github.com/devports/devpt/pkg/process"
)

// ListCmd handles the 'ls' command
func (a *App) ListCmd(detailed bool) error {
	servers, err := a.discoverServers()
	if err != nil {
		return err
	}

	return a.printServerTable(servers, detailed)
}

// printServerTable prints servers in tabular format
func (a *App) printServerTable(servers []*models.ServerInfo, detailed bool) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)

	if detailed {
		fmt.Fprintln(w, "Name\tPort\tPID\tProject\tCommand\tSource\tStatus")
		for _, srv := range servers {
			fmt.Fprintln(w, a.formatServerRow(srv, true))
		}
	} else {
		fmt.Fprintln(w, "Name\tPort\tPID\tProject\tSource\tStatus")
		for _, srv := range servers {
			fmt.Fprintln(w, a.formatServerRow(srv, false))
		}
	}

	return w.Flush()
}

// formatServerRow formats a server as a table row
func (a *App) formatServerRow(srv *models.ServerInfo, detailed bool) string {
	name := "-"
	port := "-"
	pid := "-"
	project := "-"
	command := "-"
	source := string(srv.Source)
	status := srv.Status

	if srv.ManagedService != nil {
		name = srv.ManagedService.Name
		if len(srv.ManagedService.Ports) > 0 {
			port = fmt.Sprintf("%d", srv.ManagedService.Ports[0])
		}
		command = srv.ManagedService.Command
	}

	if srv.ProcessRecord != nil {
		pid = fmt.Sprintf("%d", srv.ProcessRecord.PID)
		port = fmt.Sprintf("%d", srv.ProcessRecord.Port)
		project = srv.ProcessRecord.ProjectRoot
		if command == "-" {
			command = srv.ProcessRecord.Command
		}

		// Determine source
		if srv.ProcessRecord.AgentTag != nil {
			source = fmt.Sprintf("%s:%s", srv.ProcessRecord.AgentTag.Source, srv.ProcessRecord.AgentTag.AgentName)
		} else {
			source = string(models.SourceManual)
		}
	}

	if detailed {
		return fmt.Sprintf("%s\t%s\t%s\t%s\t%s\t%s\t%s", name, port, pid, project, command, source, status)
	}

	return fmt.Sprintf("%s\t%s\t%s\t%s\t%s\t%s", name, port, pid, project, source, status)
}

// AddCmd registers a new managed service
func (a *App) AddCmd(name, cwd, command string, ports []int) error {
	if err := validateManagedCommand(command); err != nil {
		return err
	}

	svc := &models.ManagedService{
		Name:    name,
		CWD:     cwd,
		Command: command,
		Ports:   ports,
	}

	if err := a.registry.AddService(svc); err != nil {
		return err
	}

	fmt.Printf("Service %q registered successfully\n", name)
	return nil
}

// RemoveCmd removes a managed service
func (a *App) RemoveCmd(name string) error {
	return a.registry.RemoveService(name)
}

// StartCmd starts a managed service
func (a *App) StartCmd(name string) error {
	svc := a.registry.GetService(name)
	if svc == nil {
		return fmt.Errorf("service %q not found", name)
	}

	fmt.Printf("Starting service %q...\n", name)
	pid, err := a.processManager.Start(svc)
	if err != nil {
		return fmt.Errorf("failed to start service: %w", err)
	}

	// Update registry with new PID
	if err := a.registry.UpdateServicePID(name, pid); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to update registry: %v\n", err)
	}

	fmt.Printf("Service %q started with PID %d\n", name, pid)
	return nil
}

// StopCmd stops a service by name or port
func (a *App) StopCmd(identifier string) error {
	var targetPID int
	targetServiceName := ""

	// Check if identifier is a service name
	if svc := a.registry.GetService(identifier); svc != nil {
		targetServiceName = svc.Name
		servers, err := a.discoverServers()
		if err != nil {
			return err
		}
		targetPID = managedServicePID(servers, svc.Name)
		if targetPID == 0 && svc.LastPID != nil && *svc.LastPID > 0 && a.processManager.IsRunning(*svc.LastPID) {
			return fmt.Errorf("cannot safely determine PID for service %q; stored PID is no longer validated against a live managed process", identifier)
		}
	} else {
		// Try parsing as port number
		port, err := strconv.Atoi(identifier)
		if err != nil {
			return fmt.Errorf("invalid service name or port: %s", identifier)
		}

		// Find process by port
		servers, err := a.discoverServers()
		if err != nil {
			return err
		}

		for _, srv := range servers {
			if srv.ProcessRecord != nil && srv.ProcessRecord.Port == port {
				targetPID = srv.ProcessRecord.PID
				if srv.ManagedService != nil {
					targetServiceName = srv.ManagedService.Name
				}
				break
			}
		}

		if targetPID == 0 {
			return fmt.Errorf("no process found on port %d", port)
		}
	}

	if targetPID == 0 {
		return fmt.Errorf("cannot determine PID to stop")
	}

	// Stop the process
	fmt.Printf("Stopping PID %d...\n", targetPID)
	if err := a.processManager.Stop(targetPID, 5000000000); err != nil { // 5 second timeout
		if errors.Is(err, process.ErrNeedSudo) {
			return fmt.Errorf("requires sudo to terminate PID %d", targetPID)
		}
		if isProcessFinishedErr(err) {
			if targetServiceName != "" {
				if clrErr := a.registry.ClearServicePID(targetServiceName); clrErr != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to clear PID for %q: %v\n", targetServiceName, clrErr)
				}
			}
			return nil
		}
		return fmt.Errorf("failed to stop process: %w", err)
	}

	fmt.Printf("Process %d stopped\n", targetPID)
	if targetServiceName != "" {
		if err := a.registry.ClearServicePID(targetServiceName); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to clear PID for %q: %v\n", targetServiceName, err)
		}
	}
	return nil
}

// RestartCmd restarts a managed service
func (a *App) RestartCmd(name string) error {
	svc := a.registry.GetService(name)
	if svc == nil {
		return fmt.Errorf("service %q not found", name)
	}

	// Stop if running
	if pid, err := a.validatedManagedPID(svc); err != nil {
		return err
	} else if pid > 0 {
		fmt.Printf("Stopping service %q...\n", name)
		if err := a.processManager.Stop(pid, 5000000000); err != nil { // 5 second timeout
			fmt.Fprintf(os.Stderr, "Warning: failed to stop service: %v\n", err)
		}
	}

	// Start
	fmt.Printf("Starting service %q...\n", name)
	pid, err := a.processManager.Start(svc)
	if err != nil {
		return fmt.Errorf("failed to start service: %w", err)
	}

	// Update registry
	if err := a.registry.UpdateServicePID(name, pid); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to update registry: %v\n", err)
	}

	fmt.Printf("Service %q restarted with PID %d\n", name, pid)
	return nil
}

// LogsCmd displays recent logs for a service
func (a *App) LogsCmd(name string, lines int) error {
	svc := a.registry.GetService(name)
	if svc == nil {
		return fmt.Errorf("service %q not found", name)
	}

	logLines, err := a.processManager.Tail(svc.Name, lines)
	if err != nil {
		return err
	}

	fmt.Printf("Logs for service %q:\n", name)
	for _, line := range logLines {
		fmt.Println(line)
	}

	return nil
}

func isProcessFinishedErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "process already finished") || strings.Contains(msg, "no such process")
}

func managedServicePID(servers []*models.ServerInfo, serviceName string) int {
	for _, srv := range servers {
		if srv == nil || srv.ManagedService == nil || srv.ProcessRecord == nil {
			continue
		}
		if srv.ManagedService.Name == serviceName {
			return srv.ProcessRecord.PID
		}
	}
	return 0
}

func (a *App) validatedManagedPID(svc *models.ManagedService) (int, error) {
	if svc == nil {
		return 0, nil
	}
	servers, err := a.discoverServers()
	if err != nil {
		return 0, err
	}
	pid := managedServicePID(servers, svc.Name)
	if pid != 0 {
		return pid, nil
	}
	if svc.LastPID != nil && *svc.LastPID > 0 && a.processManager.IsRunning(*svc.LastPID) {
		return 0, fmt.Errorf("cannot safely determine PID for service %q; stored PID is no longer validated against a live managed process", svc.Name)
	}
	return 0, nil
}

// StatusCmd shows detailed info for a specific server
func (a *App) StatusCmd(identifier string) error {
	servers, err := a.discoverServers()
	if err != nil {
		return err
	}

	var target *models.ServerInfo

	// Find by name or port
	for _, srv := range servers {
		if srv.ManagedService != nil && srv.ManagedService.Name == identifier {
			target = srv
			break
		}
		if srv.ProcessRecord != nil && fmt.Sprintf("%d", srv.ProcessRecord.Port) == identifier {
			target = srv
			break
		}
	}

	if target == nil {
		return fmt.Errorf("server %q not found", identifier)
	}

	return a.printServerStatus(target)
}

// printServerStatus prints detailed status for a server
func (a *App) printServerStatus(srv *models.ServerInfo) error {
	line := "============================================================"
	fmt.Println("\n" + line)
	fmt.Println("SERVER DETAILS")
	fmt.Println(line)

	if srv.ManagedService != nil {
		fmt.Printf("Name:    %s\n", srv.ManagedService.Name)
		fmt.Printf("Command: %s\n", srv.ManagedService.Command)
		fmt.Printf("CWD:     %s\n", srv.ManagedService.CWD)
		fmt.Printf("Ports:   ")
		for i, p := range srv.ManagedService.Ports {
			if i > 0 {
				fmt.Print(", ")
			}
			fmt.Printf("%d", p)
		}
		fmt.Println()
	}

	if srv.ProcessRecord != nil {
		fmt.Printf("\nPort:    %d\n", srv.ProcessRecord.Port)
		fmt.Printf("PID:     %d\n", srv.ProcessRecord.PID)
		fmt.Printf("PPID:    %d\n", srv.ProcessRecord.PPID)
		fmt.Printf("User:    %s\n", srv.ProcessRecord.User)
		fmt.Printf("Command: %s\n", srv.ProcessRecord.Command)
		fmt.Printf("CWD:     %s\n", srv.ProcessRecord.CWD)
		if srv.ProcessRecord.ProjectRoot != "" {
			fmt.Printf("Project: %s\n", srv.ProcessRecord.ProjectRoot)
		}

		// Health check
		dashes := "------------------------------------------------------------"
		fmt.Println("\n" + dashes)
		fmt.Println("HEALTH STATUS")
		fmt.Println(dashes)
		check := a.healthChecker.Check(srv.ProcessRecord.Port)
		icon := health.StatusIcon(check.Status)
		fmt.Printf("Status:   %s %s\n", icon, check.Status)
		fmt.Printf("Response: %dms\n", check.ResponseMs)
		fmt.Printf("Message:  %s\n", check.Message)

		// Agent detection
		if srv.ProcessRecord.AgentTag != nil {
			fmt.Println("\n" + dashes)
			fmt.Println("AI AGENT DETECTION")
			fmt.Println(dashes)
			fmt.Printf("Source:     %s\n", srv.ProcessRecord.AgentTag.Source)
			fmt.Printf("Agent:      %s\n", srv.ProcessRecord.AgentTag.AgentName)
			fmt.Printf("Confidence: %s\n", srv.ProcessRecord.AgentTag.Confidence)
		}
	}

	if srv.Status == "crashed" {
		dashes := "------------------------------------------------------------"
		fmt.Println("\n" + dashes)
		fmt.Println("CRASH DETAILS")
		fmt.Println(dashes)
		if srv.CrashReason != "" {
			fmt.Printf("Reason: %s\n", srv.CrashReason)
		} else {
			fmt.Println("Reason: unavailable")
		}
		if len(srv.CrashLogTail) > 0 {
			fmt.Println("Recent logs:")
			for _, line := range srv.CrashLogTail {
				if strings.TrimSpace(line) == "" {
					continue
				}
				fmt.Printf("  %s\n", line)
			}
		}
	}

	fmt.Printf("\nStatus:   %s\n", srv.Status)
	fmt.Printf("Source:   %s\n", srv.Source)
	fmt.Println(line + "\n")

	return nil
}
