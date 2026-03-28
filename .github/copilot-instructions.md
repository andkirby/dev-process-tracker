# DevPortTrack Copilot Instructions

A macOS CLI tool for discovering, tracking, and managing local development servers. ~3,900 lines of Go across 9 packages.

## Quick Reference

### Build & Run
```bash
# Build the CLI
go build -o devpt ./cmd/devpt

# Run with no args (opens interactive TUI)
./devpt

# List running services
./devpt ls

# Run tests (all packages)
go test ./...

# Run single test package with verbose output
go test -v ./pkg/cli

# Run specific test
go test -v ./pkg/cli -run TestWarnLegacyManagedCommands
```

### Key Directories
- **cmd/devpt/main.go** - CLI entry point (~170 lines). Routes commands and prints results to stdout/stderr.
- **pkg/cli/** - Command handlers (commands.go), TUI app controller (app.go), and Bubble Tea UI (tui.go). ~50KB of code.
- **pkg/scanner/** - Process discovery via `lsof`, project root detection, and AI agent detection.
- **pkg/registry/** - Service registry (JSON at ~/.config/devpt/registry.json). Thread-safe CRUD operations.
- **pkg/process/** - Process lifecycle management: spawning, log capture, graceful shutdown.
- **pkg/models/** - Core data structures (ProcessRecord, ManagedService, AgentTag) and config paths.
- **pkg/health/** - Health check utilities (basic placeholder for future expansion).

## Architecture Overview

### Data Flow
1. **Scanner** (`pkg/scanner`) discovers listening TCP ports via `lsof`
2. **Resolver** walks filesystem to find project roots (.git, go.mod, package.json, etc.)
3. **Detector** analyzes parent process/env to identify AI-agent-started servers
4. **Registry** (`pkg/registry`) manages user-registered managed services (JSON at ~/.config/devpt/registry.json)
5. **Process Manager** (`pkg/process`) handles spawning, stopping, and log capture
6. **CLI/TUI** presents the unified list and command interface

### Key Models
- **ProcessRecord**: Discovered listening process (PID, port, command, project root, agent detection)
- **ManagedService**: User-registered service (name, cwd, command, ports, timestamps)
- **AgentTag**: Detection result (source, agent name, confidence level)
- **Registry**: Container for all managed services (versioned JSON format)

### Command Routing
Entry point (cmd/devpt/main.go) routes commands:
- No args → `app.TopCmd()` (opens interactive TUI in pkg/cli/tui.go)
- `ls`, `add`, `start`, `stop`, `restart`, `logs`, `status` → handler functions

## Critical Implementation Details

### ProcessRecord vs ManagedService Merging
When listing services:
1. Merge discovered processes with managed registry entries
2. Managed service appears as "running" if its PID is in discovered processes
3. Source field shows: "manual" (discovered but unmanaged), "managed" (registered), or "agent:xxx" (detected)

### Agent Detection Confidence
Detector uses multiple heuristics (parent process name, TTY, env vars, command patterns).
Returns confidence level: low, medium, or high. Code uses these intelligently for display.

### Registry Persistence
- Registry is thread-safe (uses RWMutex)
- File-based JSON at ~/.config/devpt/registry.json
- Must handle concurrent reads (multiple CLI invocations)
- Atomic writes via atomic.Value or temp file + rename pattern

### Process Spawning & Logging
- Processes spawn in separate process groups (setpgid)
- stdout/stderr redirected to ~/.config/devpt/logs/{serviceName}/{timestamp}.log
- Graceful shutdown: SIGTERM with timeout, then SIGKILL
- PID and start time tracked in registry after spawn

### Directory Caching
Project resolver caches directory → project root mappings.
Cache can be invalidated selectively. Important for performance (lsof calls are slow).

## Dependencies

**Key Libraries:**
- **Charmbracelet stack** (Bubble Tea + Lipgloss): TUI rendering and styling. Bubble Tea is event-driven; Lipgloss handles text styling.
- **stdlib only** for core logic: Uses `os`, `exec`, `json`, `sync` packages. No external deps for scanning/registry/process management.

**Platform-specific:**
- Uses `lsof` (macOS system utility) for port discovery
- Uses `ps` for process metadata
- Not tested on Linux/Windows

## Testing

### Test Locations
- **pkg/cli/**: app_warning_test.go (TestWarnLegacyManagedCommands), command_validation_test.go (TestValidateManagedCommand, TestFirstBlockedShellPattern) - 3 tests total
- **pkg/process/**: manager_parse_test.go (TestParseCommandArgs, TestParseCommandArgs_UnterminatedQuote) - 2 tests total

### Test Patterns
- Table-driven tests for command parsing and validation
- No external dependencies; tests use pure Go (no mocking framework)
- Run full suite: `go test ./...` (2 seconds)
- Run single package: `go test -v ./pkg/cli`
- Run specific test: `go test -v ./pkg/cli -run TestWarnLegacyManagedCommands`

## Conventions

### Naming
- Packages use lowercase, no underscores (Go convention)
- Function names: `CommandName()` pattern for exported, `helperName()` for unexported
- Registry keys: lowercase with underscores in JSON (matched by struct tags)
- File suffixes: `_test.go` for tests (standard), no other suffixes

### Error Handling
- Errors propagated up the call stack (not swallowed)
- Print to stderr and exit(1) at CLI boundary (cmd/devpt/main.go)
- Use `fmt.Fprintf(os.Stderr, ...)` for error output
- TUI errors shown as status messages (bottom of screen)

### Process Management
- Always check if process still running before operations (PID validation)
- Use PID 0 sentinel for "not running" 
- Registry tracks LastPID + LastStart for validation
- Spawned processes use separate process groups (setpgid) for clean shutdown

### Configuration Paths
- ConfigPaths struct in pkg/models/config.go centralizes all paths
- Registry, logs, and temp files all use ~/.config/devpt
- Create dirs if missing with MkdirAll (mode 0755)
- Log files timestamped as: ~/.config/devpt/logs/{serviceName}/{ISO8601}.log

### TUI-Specific (pkg/cli/tui.go)
- Model-based architecture (Bubble Tea): Cmd returns effects, Model contains state
- Top-level ListModel has tabs for "Running" and "Managed" lists
- Never mutate Model state directly—use Cmd/Update pattern
- Exit conditions: user presses 'q', or explicit quit() command
- Key handlers prioritized: modal state (logs/input) takes precedence over list navigation

## Before Submitting Changes

Always run these checks before considering work complete:

```bash
# 1. Build succeeds
go build ./...

# 2. All tests pass
go test ./...

# 3. CLI runs without error
go build -o devpt ./cmd/devpt && ./devpt ls
```

If adding user-facing features, also update README.md and QUICKSTART.md.

## Common Tasks

## Common Tasks

### Add a New CLI Command
1. Add handler function in cmd/devpt/main.go switch statement (e.g., `case "mycommand"`)
2. Call existing app methods (app.ListServices(), app.StartService(), etc.) or create new methods in pkg/cli/app.go
3. Add command parsing logic to cmd/devpt/main.go if needed (arg validation, flags, etc.)
4. Test with: `go build -o devpt ./cmd/devpt && ./devpt mycommand [args]`

### Add a TUI Command (colon-mode)
1. Add command pattern to tui.go parseTUICommand() function
2. Handle in handleTUICommand() to update Model state
3. Test via TUI: press `:` to enter command mode

### Modify Process Discovery
- Edit pkg/scanner/scanner.go (runs `lsof` and parses output)
- Update ProcessRecord fields in pkg/models/models.go if schema changes
- Remember: deduplication by (PID, Port) to avoid duplicates
- Test with: `go test ./...`

### Change Registry Format
- Update schema version in pkg/registry/registry.go (if breaking change)
- Handle migration logic if changing JSON structure
- Keep backward compatibility where possible (new fields optional with default values)
- Test by: `devpt add myapp /tmp "sleep 999" 9999 && devpt ls`

### Update Output Formatting
- CLI table output: pkg/cli/commands.go (PrintServices function). Uses tab-width for alignment.
- TUI table output: pkg/cli/tui.go (RenderRunningList, RenderManagedList). Uses Lipgloss for styling.
- Remember: both must maintain consistent column ordering (Name, Port, PID, Project, Command, Source, Status)

## Documentation Files
- **README.md** - Full user documentation and CLI reference
- **QUICKSTART.md** - Getting started guide for new users
- **IMPLEMENTATION_SUMMARY.md** - Architecture and feature overview (reference only)
- **techspec.md** - Original technical specification
- **.agents/skills/devpt-release/SKILL.md** - Release workflow (changelog + version bump)

Update README and QUICKSTART when adding user-facing features or commands.

## Known Gotchas & Limitations

- **macOS only**: Uses `lsof` and `ps`; untested on Linux/Windows
- **TUI doesn't persist state**: Closing and reopening refreshes the view from registry/system
- **Registry JSON manual edits**: Must maintain proper JSON format; bad formatting breaks on next write
- **Agent detection heuristics**: Not exhaustive; may miss or misidentify some agents
- **Log capture**: Only available for managed services. Unmanaged process logs are best-effort (if available in /proc or similar).
- **No daemon mode**: Each `devpt` invocation is a fresh scan; no background monitoring
- **Working directory caching**: Cache is per-invocation; invalidation happens manually in code (not automatic on filesystem changes)
