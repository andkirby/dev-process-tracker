package cli

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/devports/devpt/pkg/health"
	"github.com/devports/devpt/pkg/models"
)

// PrintServerTable prints servers in tabular format.
func PrintServerTable(w io.Writer, servers []*models.ServerInfo, detailed bool) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)

	if detailed {
		fmt.Fprintln(tw, "Name\tPort\tPID\tProject\tCommand\tSource\tStatus")
		for _, srv := range servers {
			fmt.Fprintln(tw, FormatServerRow(srv, true))
		}
	} else {
		fmt.Fprintln(tw, "Name\tPort\tPID\tProject\tSource\tStatus")
		for _, srv := range servers {
			fmt.Fprintln(tw, FormatServerRow(srv, false))
		}
	}

	return tw.Flush()
}

// FormatServerRow formats a server as a table row string.
func FormatServerRow(srv *models.ServerInfo, detailed bool) string {
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

// PrintServerStatus prints detailed status for a server.
func PrintServerStatus(w io.Writer, srv *models.ServerInfo, hc *health.HealthCheck) error {
	line := "============================================================"
	fmt.Fprintln(w, "\n"+line)
	fmt.Fprintln(w, "SERVER DETAILS")
	fmt.Fprintln(w, line)

	if srv.ManagedService != nil {
		fmt.Fprintf(w, "Name:    %s\n", srv.ManagedService.Name)
		fmt.Fprintf(w, "Command: %s\n", srv.ManagedService.Command)
		fmt.Fprintf(w, "CWD:     %s\n", srv.ManagedService.CWD)
		fmt.Fprintf(w, "Ports:   ")
		for i, p := range srv.ManagedService.Ports {
			if i > 0 {
				fmt.Fprint(w, ", ")
			}
			fmt.Fprintf(w, "%d", p)
		}
		fmt.Fprintln(w)
	}

	if srv.ProcessRecord != nil {
		fmt.Fprintf(w, "\nPort:    %d\n", srv.ProcessRecord.Port)
		fmt.Fprintf(w, "PID:     %d\n", srv.ProcessRecord.PID)
		fmt.Fprintf(w, "PPID:    %d\n", srv.ProcessRecord.PPID)
		fmt.Fprintf(w, "User:    %s\n", srv.ProcessRecord.User)
		fmt.Fprintf(w, "Command: %s\n", srv.ProcessRecord.Command)
		fmt.Fprintf(w, "CWD:     %s\n", srv.ProcessRecord.CWD)
		if srv.ProcessRecord.ProjectRoot != "" {
			fmt.Fprintf(w, "Project: %s\n", srv.ProcessRecord.ProjectRoot)
		}

		// Health check
		dashes := "------------------------------------------------------------"
		fmt.Fprintln(w, "\n"+dashes)
		fmt.Fprintln(w, "HEALTH STATUS")
		fmt.Fprintln(w, dashes)

		if hc != nil {
			icon := health.StatusIcon(hc.Status)
			fmt.Fprintf(w, "Status:   %s %s\n", icon, hc.Status)
			fmt.Fprintf(w, "Response: %dms\n", hc.ResponseMs)
			fmt.Fprintf(w, "Message:  %s\n", hc.Message)
		} else {
			fmt.Fprintln(w, "Status:   (not checked)")
		}

		// Agent detection
		if srv.ProcessRecord.AgentTag != nil {
			fmt.Fprintln(w, "\n"+dashes)
			fmt.Fprintln(w, "AI AGENT DETECTION")
			fmt.Fprintln(w, dashes)
			fmt.Fprintf(w, "Source:     %s\n", srv.ProcessRecord.AgentTag.Source)
			fmt.Fprintf(w, "Agent:      %s\n", srv.ProcessRecord.AgentTag.AgentName)
			fmt.Fprintf(w, "Confidence: %s\n", srv.ProcessRecord.AgentTag.Confidence)
		}
	}

	if srv.Status == "crashed" {
		dashes := "------------------------------------------------------------"
		fmt.Fprintln(w, "\n"+dashes)
		fmt.Fprintln(w, "CRASH DETAILS")
		fmt.Fprintln(w, dashes)
		if srv.CrashReason != "" {
			fmt.Fprintf(w, "Reason: %s\n", srv.CrashReason)
		} else {
			fmt.Fprintln(w, "Reason: unavailable")
		}
		if len(srv.CrashLogTail) > 0 {
			fmt.Fprintln(w, "Recent logs:")
			for _, l := range srv.CrashLogTail {
				if strings.TrimSpace(l) == "" {
					continue
				}
				fmt.Fprintf(w, "  %s\n", l)
			}
		}
	}

	fmt.Fprintf(w, "\nStatus:   %s\n", srv.Status)
	fmt.Fprintf(w, "Source:   %s\n", srv.Source)
	fmt.Fprintln(w, line+"\n")

	return nil
}
