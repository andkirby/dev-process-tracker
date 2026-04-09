package scanner

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/devports/devpt/pkg/models"
)

// ProcessScanner discovers listening ports using macOS tools
type ProcessScanner struct {
	cwdCache map[int]string
	mu       sync.RWMutex
}

// NewProcessScanner creates a new scanner instance
func NewProcessScanner() *ProcessScanner {
	return &ProcessScanner{
		cwdCache: make(map[int]string),
	}
}

// ScanListeningPorts discovers all TCP listening ports
func (ps *ProcessScanner) ScanListeningPorts() ([]*models.ProcessRecord, error) {
	cmd := exec.Command("lsof", "-nP", "-iTCP", "-sTCP:LISTEN")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to run lsof: %w", err)
	}

	records, err := ps.parseLsofOutput(string(output))
	if err != nil {
		return records, err
	}

	// Enrich records with command information
	ps.enrichWithCommands(records)
	return records, nil
}

// parseLsofOutput parses lsof output into ProcessRecords
func (ps *ProcessScanner) parseLsofOutput(output string) ([]*models.ProcessRecord, error) {
	scanner := bufio.NewScanner(strings.NewReader(output))
	records := make([]*models.ProcessRecord, 0)
	seen := make(map[string]bool)

	// Skip header
	if !scanner.Scan() {
		return records, nil
	}

	for scanner.Scan() {
		line := scanner.Text()
		record, err := ps.parseLsofLine(line)
		if err != nil {
			continue
		}

		if record != nil {
			key := fmt.Sprintf("%d:%d", record.PID, record.Port)
			if !seen[key] {
				seen[key] = true
				records = append(records, record)
			}
		}
	}

	return records, nil
}

// parseLsofLine parses a single lsof output line
func (ps *ProcessScanner) parseLsofLine(line string) (*models.ProcessRecord, error) {
	fields := strings.Fields(line)
	if len(fields) < 9 {
		return nil, fmt.Errorf("insufficient fields")
	}

	command := fields[0]
	pidStr := fields[1]
	nameField := fields[8]

	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return nil, fmt.Errorf("invalid pid")
	}

	port, err := extractPort(nameField)
	if err != nil {
		return nil, fmt.Errorf("no port")
	}

	return &models.ProcessRecord{
		PID:      pid,
		Port:     port,
		Command:  command, // Preserve lsof command name as fallback if ps lookup fails
		CWD:      "",      // Skip for now - was causing hangs
		Protocol: "tcp",
	}, nil
}

// extractPort extracts port from NAME field
func extractPort(name string) (int, error) {
	parts := strings.Split(name, ":")
	if len(parts) < 2 {
		return 0, fmt.Errorf("no port")
	}

	portStr := parts[len(parts)-1]
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return 0, fmt.Errorf("invalid port")
	}

	return port, nil
}

// enrichWithCommands fetches command information for each PID
func (ps *ProcessScanner) enrichWithCommands(records []*models.ProcessRecord) {
	for _, record := range records {
		if record == nil {
			continue
		}

		cmd := exec.Command("ps", "-p", fmt.Sprintf("%d", record.PID), "-o", "command=")
		output, err := cmd.Output()
		if err == nil {
			if fullCmd := strings.TrimSpace(string(output)); fullCmd != "" {
				record.Command = fullCmd
			}
		}

		if record.CWD == "" {
			if cwd, ok := ps.getCWD(record.PID); ok {
				record.CWD = cwd
			}
		}
	}
}

func (ps *ProcessScanner) getCWD(pid int) (string, bool) {
	ps.mu.RLock()
	if cached, ok := ps.cwdCache[pid]; ok {
		ps.mu.RUnlock()
		if cached == "" {
			return "", false
		}
		return cached, true
	}
	ps.mu.RUnlock()

	ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer cancel()

	cmd := exec.CommandContext(ctx, "lsof", "-a", "-p", fmt.Sprintf("%d", pid), "-d", "cwd", "-Fn")
	output, err := cmd.Output()
	if err != nil || ctx.Err() != nil {
		ps.mu.Lock()
		ps.cwdCache[pid] = ""
		ps.mu.Unlock()
		return "", false
	}

	cwd := ""
	for _, line := range strings.Split(string(output), "\n") {
		if strings.HasPrefix(line, "n") {
			cwd = strings.TrimPrefix(line, "n")
			break
		}
	}

	ps.mu.Lock()
	ps.cwdCache[pid] = cwd
	ps.mu.Unlock()

	if cwd == "" {
		return "", false
	}
	return cwd, true
}

// DetectFrameworkInfo detects the framework and language of a process
func (ps *ProcessScanner) DetectFrameworkInfo(pid int, command string, cwd string) *FrameworkInfo {
	return DetectFramework(pid, command, cwd)
}
