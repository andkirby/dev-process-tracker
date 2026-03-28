package process

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/devports/devpt/pkg/models"
)

// Manager handles starting and stopping of managed services
type Manager struct {
	logsDir string
}

var ErrNoLogs = errors.New("no logs available")
var ErrNeedSudo = errors.New("requires sudo to terminate process")
var ErrNoProcessLogs = errors.New("no process logs available")

// NewManager creates a new process manager
func NewManager(logsDir string) *Manager {
	return &Manager{
		logsDir: logsDir,
	}
}

// Start starts a managed service
func (m *Manager) Start(service *models.ManagedService) (int, error) {
	// Validate working directory and bind process execution to it.
	if fi, err := os.Stat(service.CWD); err != nil || !fi.IsDir() {
		if err != nil {
			return 0, fmt.Errorf("invalid working directory: %w", err)
		}
		return 0, fmt.Errorf("invalid working directory: not a directory")
	}

	// Create log file
	logFile, err := m.createLogFile(service.Name)
	if err != nil {
		return 0, fmt.Errorf("failed to create log file: %w", err)
	}
	defer logFile.Close()

	// Execute commands directly (no implicit shell) for safer defaults.
	argv, err := parseCommandArgs(service.Command)
	if err != nil {
		return 0, fmt.Errorf("invalid command: %w", err)
	}
	if len(argv) == 0 {
		return 0, fmt.Errorf("invalid command: empty")
	}
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir = service.CWD

	// Set up process group to manage all child processes (platform-specific)
	setProcessGroup(cmd)

	// Redirect output to log file
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	// Start process
	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("failed to start process: %w", err)
	}

	return cmd.Process.Pid, nil
}

// Stop gracefully stops a process with timeout, then force-kills if needed
func (m *Manager) Stop(pid int, timeout time.Duration) error {
	if pid <= 0 {
		return fmt.Errorf("invalid pid: %d", pid)
	}
	if !m.isAlive(pid) {
		return nil
	}

	// First attempt graceful termination. For non-child processes we cannot use Wait(),
	// so we send signals and poll for liveness.
	if err := terminateProcess(pid); err != nil {
		if err := terminateProcessFallback(pid); err != nil {
			return fmt.Errorf("failed to send termination signal: %w", err)
		}
	}

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !isProcessAlive(pid) {
			return nil
		}
		time.Sleep(120 * time.Millisecond)
	}

	// Escalate to hard kill.
	if err := killProcess(pid); err != nil {
		_ = killProcessFallback(pid)
	}
	time.Sleep(200 * time.Millisecond)
	if isProcessAlive(pid) {
		return ErrNeedSudo
	}
	return nil
}

func (m *Manager) isAlive(pid int) bool {
	if !isProcessAlive(pid) {
		return false
	}
	if st, stateErr := m.processState(pid); stateErr == nil {
		// Zombie processes still respond to signal 0 but are not runnable.
		if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(st)), "Z") {
			return false
		}
	}
	return true
}

// Restart stops and starts a process
func (m *Manager) Restart(service *models.ManagedService) (int, error) {
	// Stop existing process if running
	if service.LastPID != nil && *service.LastPID > 0 {
		m.Stop(*service.LastPID, 5*time.Second)
	}

	// Start new process
	return m.Start(service)
}

// IsRunning checks if a process is still running
func (m *Manager) IsRunning(pid int) bool {
	if pid <= 0 {
		return false
	}
	return m.isAlive(pid)
}

// createLogFile creates a new log file for a service
func (m *Manager) createLogFile(serviceName string) (*os.File, error) {
	// Create service log directory
	serviceLogDir := filepath.Join(m.logsDir, serviceName)
	if err := os.MkdirAll(serviceLogDir, 0755); err != nil {
		return nil, err
	}

	// Create timestamped log file
	timestamp := time.Now().Format("2006-01-02T15-04-05")
	logPath := filepath.Join(serviceLogDir, timestamp+".log")

	return os.Create(logPath)
}

// GetLogs retrieves recent logs for a service
func (m *Manager) GetLogs(serviceName string, lines int) ([]string, error) {
	return m.Tail(serviceName, lines)
}

// LatestLogPath returns the most recent log file path for a service.
func (m *Manager) LatestLogPath(serviceName string) (string, error) {
	serviceLogDir := filepath.Join(m.logsDir, serviceName)
	entries, err := os.ReadDir(serviceLogDir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", ErrNoLogs
		}
		return "", fmt.Errorf("failed to read log directory: %w", err)
	}
	if len(entries) == 0 {
		return "", ErrNoLogs
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})
	latestLog := entries[len(entries)-1]
	return filepath.Join(serviceLogDir, latestLog.Name()), nil
}

// Tail returns the last N lines from the most recent log file.
func (m *Manager) Tail(serviceName string, lines int) ([]string, error) {
	if lines <= 0 {
		return []string{}, nil
	}

	logPath, err := m.LatestLogPath(serviceName)
	if err != nil {
		return nil, err
	}

	file, err := os.Open(logPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	buf := make([]byte, 0, 1024*1024)
	scanner.Buffer(buf, 1024*1024)

	linesBuf := make([]string, 0, lines)
	for scanner.Scan() {
		if len(linesBuf) < lines {
			linesBuf = append(linesBuf, scanner.Text())
		} else {
			copy(linesBuf, linesBuf[1:])
			linesBuf[len(linesBuf)-1] = scanner.Text()
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read log file: %w", err)
	}
	return linesBuf, nil
}

// TailProcess tries to retrieve logs for a non-managed process.
// Strategy:
// 1) Tail an open *.log file owned by the process, if any.
// 2) Fall back to macOS unified logs for that PID.
func (m *Manager) TailProcess(pid int, lines int) ([]string, error) {
	if pid <= 0 {
		return nil, fmt.Errorf("invalid pid: %d", pid)
	}
	if lines <= 0 {
		return []string{}, nil
	}

	if path, ok := m.pickProcessLogFile(pid); ok {
		out, err := m.tailFile(path, lines)
		if err == nil && len(out) > 0 {
			return out, nil
		}
	}

	pred := fmt.Sprintf("processID == %d", pid)
	cmd := exec.Command("log", "show", "--last", "2m", "--style", "compact", "--predicate", pred)
	output, err := cmd.Output()
	if err == nil {
		linesOut := lastNLines(strings.Split(string(output), "\n"), lines)
		if len(linesOut) > 0 {
			return linesOut, nil
		}
	}

	return nil, ErrNoProcessLogs
}

func (m *Manager) pickProcessLogFile(pid int) (string, bool) {
	cmd := exec.Command("lsof", "-nP", "-p", strconv.Itoa(pid), "-Fn")
	output, err := cmd.Output()
	if err != nil {
		return "", false
	}

	var candidates []string
	for _, line := range strings.Split(string(output), "\n") {
		if !strings.HasPrefix(line, "n") {
			continue
		}
		path := strings.TrimSpace(strings.TrimPrefix(line, "n"))
		if path == "" {
			continue
		}
		lower := strings.ToLower(path)
		if !strings.Contains(lower, ".log") && !strings.Contains(lower, "/log") {
			continue
		}
		fi, statErr := os.Stat(path)
		if statErr != nil || fi.IsDir() {
			continue
		}
		candidates = append(candidates, path)
	}
	if len(candidates) == 0 {
		return "", false
	}

	sort.Slice(candidates, func(i, j int) bool {
		fi, errI := os.Stat(candidates[i])
		fj, errJ := os.Stat(candidates[j])
		if errI != nil || errJ != nil {
			return candidates[i] < candidates[j]
		}
		return fi.ModTime().After(fj.ModTime())
	})
	return candidates[0], true
}

func (m *Manager) tailFile(path string, lines int) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	buf := make([]byte, 0, 1024*1024)
	scanner.Buffer(buf, 1024*1024)

	linesBuf := make([]string, 0, lines)
	for scanner.Scan() {
		if len(linesBuf) < lines {
			linesBuf = append(linesBuf, scanner.Text())
		} else {
			copy(linesBuf, linesBuf[1:])
			linesBuf[len(linesBuf)-1] = scanner.Text()
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return linesBuf, nil
}

func lastNLines(in []string, n int) []string {
	out := make([]string, 0, n)
	for _, l := range in {
		if strings.TrimSpace(l) == "" {
			continue
		}
		if len(out) < n {
			out = append(out, l)
		} else {
			copy(out, out[1:])
			out[len(out)-1] = l
		}
	}
	return out
}

func (m *Manager) processState(pid int) (string, error) {
	cmd := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "state=")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func parseCommandArgs(input string) ([]string, error) {
	var args []string
	var buf strings.Builder
	inQuotes := false
	var quote rune
	escaped := false

	for _, r := range input {
		if escaped {
			buf.WriteRune(r)
			escaped = false
			continue
		}
		switch r {
		case '\\':
			escaped = true
		case '"', '\'':
			if inQuotes && r == quote {
				inQuotes = false
				quote = 0
			} else if !inQuotes {
				inQuotes = true
				quote = r
			} else {
				buf.WriteRune(r)
			}
		case ' ', '\t':
			if inQuotes {
				buf.WriteRune(r)
			} else if buf.Len() > 0 {
				args = append(args, buf.String())
				buf.Reset()
			}
		default:
			buf.WriteRune(r)
		}
	}
	if escaped || inQuotes {
		return nil, fmt.Errorf("unterminated escape or quote")
	}
	if buf.Len() > 0 {
		args = append(args, buf.String())
	}
	return args, nil
}
