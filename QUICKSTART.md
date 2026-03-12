# Dev Process Tracker - Quick Start Guide

## What is Dev Process Tracker?

Dev Process Tracker is a macOS CLI tool that helps you discover, track, and manage local development servers and ports. It answers three key questions:

1. **What servers are running?** - Lists all TCP listening ports on your machine
2. **Which project owns each server?** - Associates ports with their project roots
3. **Who started each server?** - Detects if an AI agent started the server

## Installation

Build from source:
```bash
cd ~/path/to/dev-process-tracker
go build -o devpt ./cmd/devpt/main.go
```

Optionally install to PATH:
```bash
sudo mv devpt /usr/local/bin/devpt
```

Then use from anywhere:
```bash
devpt ls
```

## First Steps

### See what's currently running

```bash
devpt ls
```

Shows all discovered listening ports with their PID, project, and source.

### Register a service you manage

```bash
devpt add myapp ~/myapp "npm start" 3000
```

This stores `myapp` in your registry so you can control it with devpt.

### List with details

```bash
devpt ls --details
```

Shows the full command that each process is running.

### Check your registered services

```bash
cat ~/.config/devpt/registry.json
```

Your services are stored here and can be edited manually.

## Common Workflows

### Start a managed service

```bash
devpt start myapp
```

Logs are captured to: `~/.config/devpt/logs/myapp/<timestamp>.log`

### Start multiple services at once

```bash
# Start multiple specific services
devpt start api frontend worker

# Use glob patterns to match services (quote to prevent shell expansion)
devpt start 'web-*'        # Starts all services matching 'web-*'
devpt start '*-test'       # Starts all services ending with '-test'

# Target specific service by port
devpt start web-api:3000   # Start web-api on port 3000 only

# Mix patterns and specific names
devpt start api 'web-*' worker
```

Batch operations show per-service status and a summary:
```
api: started (PID 12345)
frontend: started (PID 12346)
worker: started (PID 12347)

All services started successfully
```

### Stop a service by name

```bash
devpt stop myapp
```

### Stop multiple services at once

```bash
# Stop multiple specific services
devpt stop api frontend

# Use glob patterns (quote to prevent shell expansion)
devpt stop 'web-*'        # Stops all services matching 'web-*'

# Target specific service by port
devpt stop web-api:3000   # Stop web-api on port 3000 only
devpt stop *-test         # Stops all services ending with '-test'
```

### Stop a service by port

```bash
devpt stop --port 3000
```

### Restart a service

```bash
devpt restart myapp
```

### Restart multiple services at once

```bash
# Restart multiple specific services
devpt restart api frontend worker

# Use glob patterns
devpt restart web-*       # Restarts all services matching 'web-*'
devpt restart claude-*    # Restarts all services starting with 'claude-'
```

### View logs

```bash
devpt logs myapp
devpt logs myapp --lines 100
```

## Key Concepts

### Server Sources

Each server is tagged with a source:

- **manual** - Running but not in your managed registry
- **managed** - In your registry (may or may not be running)
- **agent:xxx** - Started by an AI coding agent

### Project Detection

Dev Process Tracker walks up the directory tree looking for:
- `.git` (Git repos)
- `package.json` (Node.js)
- `go.mod` (Go)
- `Gemfile` (Ruby)
- `composer.json` (PHP)
- And more...

### Agent Detection

Detects servers likely started by:
- OpenCode
- Cursor
- Claude
- Gemini
- Copilot

Uses heuristics like parent process name, TTY attachment, and environment variables.

## File Locations

```
~/.config/devpt/
├── registry.json          # Your managed services
└── logs/
    ├── myapp/
    │   ├── 2026-02-09T16-00-01.log
    │   └── 2026-02-09T16-05-30.log
    └── otherapp/
        └── 2026-02-09T16-10-00.log
```

## Tips & Tricks

1. **Edit registry manually** - `~/.config/devpt/registry.json` is just JSON
2. **Check what's using a port** - `devpt ls --details | grep :3000`
3. **Find projects** - `devpt ls | grep "my-project"`
4. **See processes without names** - `devpt ls --details | grep -v "^-"`

## Troubleshooting

**"lsof: command not found"**
```bash
brew install lsof
```

**Registry file seems broken**
```bash
rm ~/.config/devpt/registry.json
# It will be recreated next time you add a service
```

**Process won't stop**
```bash
# Find the PID
devpt ls | grep myapp

# Force kill it (use carefully!)
kill -9 <PID>
```

## Performance

- `devpt ls` typically completes in 1-2 seconds
- No background daemon (everything is on-demand)
- Results are fresh on each run

## What's Next?

- Register your frequently-used dev servers
- Check the `README.md` for full documentation
- Explore the `--details` flag to see more info
- Set up the servers you manage with `devpt add`

## Need Help?

```bash
devpt help
devpt ls --help
devpt add --help
```

Or see the full README.md for detailed documentation.
