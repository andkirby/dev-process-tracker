package scanner

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/devports/devpt/pkg/models"
)

// PrereqError is returned when required external tools are missing.
type PrereqError struct {
	Missing []string
	Hint    string
}

func (e *PrereqError) Error() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "missing required tool(s): %s\n", strings.Join(e.Missing, ", "))
	if e.Hint != "" {
		sb.WriteString(e.Hint)
	}
	return sb.String()
}

// CheckPrereqs verifies that all required external tools are available.
// Returns nil if everything is present, or a PrereqError with install hints.
// On Linux, /proc/net/tcp is accepted as an alternative to lsof.
func CheckPrereqs() error {
	missing := make([]string, 0, 2)

	if _, err := exec.LookPath("lsof"); err != nil {
		// On Linux, /proc/net/tcp can replace lsof for port scanning
		if runtime.GOOS != "linux" || !procNetTCPAvailable() {
			missing = append(missing, "lsof")
		}
	}

	if len(missing) == 0 {
		return nil
	}

	hint := prereqHint(missing)
	return &PrereqError{Missing: missing, Hint: hint}
}

func procNetTCPAvailable() bool {
	_, err := os.Stat("/proc/net/tcp")
	return err == nil
}

func prereqHint(missing []string) string {
	switch runtime.GOOS {
	case "linux":
		var sb strings.Builder
		fmt.Fprintln(&sb, "")
		fmt.Fprintln(&sb, "Install with:")
		// Debian/Ubuntu
		fmt.Fprintln(&sb, "  sudo apt install lsof")
		// Fedora/RHEL
		fmt.Fprintln(&sb, "  # or: sudo dnf install lsof")
		// Arch
		fmt.Fprintln(&sb, "  # or: sudo pacman -S lsof")
		fmt.Fprintln(&sb, "")
		fmt.Fprintln(&sb, "devpt uses lsof to discover listening ports and match them to your services.")
		return sb.String()
	case "darwin":
		return "\nlsof should be pre-installed on macOS. If missing, reinstall Xcode Command Line Tools:\n  xcode-select --install\n"
	default:
		return fmt.Sprintf("\nPlease install %s and ensure it is in your PATH.\n", strings.Join(missing, " and "))
	}
}

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

// ScanListeningPorts discovers all TCP listening ports.
// Uses lsof first; on Linux falls back to /proc/net/tcp if lsof is unavailable or fails.
func (ps *ProcessScanner) ScanListeningPorts() ([]*models.ProcessRecord, error) {
	// Try lsof first (works on macOS and Linux with root)
	if _, err := exec.LookPath("lsof"); err == nil {
		cmd := exec.Command("lsof", "-nP", "-iTCP", "-sTCP:LISTEN")
		output, err := cmd.Output()
		if err == nil {
			records, parseErr := ps.parseLsofOutput(string(output))
			if parseErr == nil {
				ps.enrichWithCommands(records)
				return records, nil
			}
			// parse failed but we got output — return what we have
			if len(records) > 0 {
				ps.enrichWithCommands(records)
				return records, nil
			}
		}
		// lsof failed — fall through to /proc on Linux
	}

	if runtime.GOOS == "linux" {
		records, err := ps.scanListeningPortsProc()
		if err != nil {
			return nil, fmt.Errorf("lsof failed and /proc/net/tcp fallback failed: %w", err)
		}
		return records, nil
	}

	return nil, fmt.Errorf("failed to run lsof")
}

// scanListeningPortsProc reads /proc/net/tcp (and tcp6) to find LISTEN sockets.
// Works without root for all users on Linux.
func (ps *ProcessScanner) scanListeningPortsProc() ([]*models.ProcessRecord, error) {
	inodeMap, err := buildInodeToPID()
	if err != nil {
		// Non-fatal: we'll have ports but no PIDs
		inodeMap = make(map[uint64]int)
	}

	records := make([]*models.ProcessRecord, 0)
	seen := make(map[string]bool)

	for _, path := range []string{"/proc/net/tcp", "/proc/net/tcp6"} {
		file, err := os.Open(path)
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(file)
		scanner.Scan() // skip header

		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			fields := strings.Fields(line)
			if len(fields) < 10 {
				continue
			}

			// State 0A = LISTEN
			if fields[3] != "0A" {
				continue
			}

			addrPort := strings.Split(fields[1], ":")
			if len(addrPort) != 2 {
				continue
			}

			port, err := strconv.ParseInt(addrPort[1], 16, 32)
			if err != nil || port == 0 {
				continue
			}

			inode, _ := strconv.ParseUint(fields[9], 10, 64)

			pid := 0
			command := ""
			if inode > 0 {
				if p, ok := inodeMap[inode]; ok {
					pid = p
					command = getProcCommand(p)
				}
			}

			key := fmt.Sprintf("%d:%d", pid, port)
			if !seen[key] {
				seen[key] = true
				records = append(records, &models.ProcessRecord{
					PID:      pid,
					Port:     int(port),
					Command:  command,
					Protocol: "tcp",
				})
			}
		}
		file.Close()
	}

	// Enrich with CWD where possible
	ps.enrichWithCommands(records)
	return records, nil
}

// buildInodeToPID scans /proc/<pid>/fd/ to map socket inodes to PIDs.
// Only works for processes owned by the current user.
func buildInodeToPID() (map[uint64]int, error) {
	result := make(map[uint64]int)

	procDir, err := os.Open("/proc")
	if err != nil {
		return nil, err
	}
	defer procDir.Close()

	entries, err := procDir.Readdirnames(-1)
	if err != nil {
		return nil, err
	}

	for _, name := range entries {
		pid, err := strconv.Atoi(name)
		if err != nil {
			continue
		}

		fdDir := filepath.Join("/proc", name, "fd")
		fdEntries, err := os.ReadDir(fdDir)
		if err != nil {
			// Permission denied for other users' processes — skip silently
			continue
		}

		for _, fd := range fdEntries {
			link, err := os.Readlink(filepath.Join(fdDir, fd.Name()))
			if err != nil {
				continue
			}
			// Socket links look like: socket:[12345]
			if !strings.HasPrefix(link, "socket:[") {
				continue
			}
			inodeStr := strings.TrimSuffix(strings.TrimPrefix(link, "socket:["), "]")
			inode, err := strconv.ParseUint(inodeStr, 10, 64)
			if err != nil {
				continue
			}
			result[inode] = pid
		}
	}

	return result, nil
}

// getProcCommand reads /proc/<pid>/cmdline to get the process command.
func getProcCommand(pid int) string {
	data, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "cmdline"))
	if err != nil {
		return ""
	}
	// cmdline is null-byte separated
	parts := strings.Split(string(data), "\x00")
	if len(parts) == 0 || parts[0] == "" {
		return ""
	}
	return parts[0]
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

	// On Linux, read /proc/<pid>/cwd symlink directly — no lsof needed
	if runtime.GOOS == "linux" {
		link, err := os.Readlink(filepath.Join("/proc", strconv.Itoa(pid), "cwd"))
		if err == nil && link != "" {
			ps.mu.Lock()
			ps.cwdCache[pid] = link
		ps.mu.Unlock()
			return link, true
		}
	}

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
