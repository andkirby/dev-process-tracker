package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"

	"github.com/devports/devpt/pkg/buildinfo"
	"github.com/devports/devpt/pkg/cli"
)

func main() {
	app, err := cli.NewApp()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	if len(os.Args) < 2 {
		if err := app.TopCmd(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	command := os.Args[1]

	switch command {
	case "ls":
		err = handleLS(app, os.Args[2:])
	case "add":
		err = handleAdd(app, os.Args[2:])
	case "start":
		err = handleStart(app, os.Args[2:])
	case "stop":
		err = handleStop(app, os.Args[2:])
	case "restart":
		err = handleRestart(app, os.Args[2:])
	case "logs":
		err = handleLogs(app, os.Args[2:])
	case "status":
		err = handleStatus(app, os.Args[2:])
	case "--help", "-h", "help":
		printUsage()
		os.Exit(0)
	case "--version", "-v":
		fmt.Printf("devpt version %s\n", buildinfo.Version)
		os.Exit(0)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func handleLS(app *cli.App, args []string) error {
	fs := flag.NewFlagSet("ls", flag.ContinueOnError)
	detailed := fs.Bool("details", false, "Show extended metadata")

	if err := fs.Parse(args); err != nil {
		return err
	}

	return app.ListCmd(*detailed)
}

func handleAdd(app *cli.App, args []string) error {
	if len(args) < 3 {
		fmt.Println("Usage: devpt add <name> <cwd> <command> [ports...]")
		return fmt.Errorf("insufficient arguments")
	}

	name := args[0]
	cwd := args[1]
	command := args[2]

	var ports []int
	for i := 3; i < len(args); i++ {
		port, err := strconv.Atoi(args[i])
		if err != nil {
			return fmt.Errorf("invalid port: %s", args[i])
		}
		ports = append(ports, port)
	}

	return app.AddCmd(name, cwd, command, ports)
}

func handleStart(app *cli.App, args []string) error {
	if len(args) < 1 {
		fmt.Println("Usage: devpt start <name> [name...]")
		return fmt.Errorf("service name required")
	}

	return app.BatchStartCmd(args)
}

func handleStop(app *cli.App, args []string) error {
	if len(args) < 1 {
		fmt.Println("Usage: devpt stop <name|--port PORT> [name...]")
		return fmt.Errorf("service name or port required")
	}

	// Check if --port flag is used (not supported with batch mode yet)
	if args[0] == "--port" {
		if len(args) > 2 {
			return fmt.Errorf("--port flag only supports single service")
		}
		if len(args) < 2 {
			return fmt.Errorf("port required after --port")
		}
		return app.StopCmd(args[1])
	}

	return app.BatchStopCmd(args)
}

func handleRestart(app *cli.App, args []string) error {
	if len(args) < 1 {
		fmt.Println("Usage: devpt restart <name> [name...]")
		return fmt.Errorf("service name required")
	}

	return app.BatchRestartCmd(args)
}

func handleLogs(app *cli.App, args []string) error {
	if len(args) < 1 {
		fmt.Println("Usage: devpt logs <name> [--lines N]")
		return fmt.Errorf("service name required")
	}

	name := args[0]
	lines := 50

	// Parse optional --lines flag
	for i := 1; i < len(args); i++ {
		if args[i] == "--lines" && i+1 < len(args) {
			if n, err := strconv.Atoi(args[i+1]); err == nil {
				lines = n
			}
		}
	}

	return app.LogsCmd(name, lines)
}

func handleStatus(app *cli.App, args []string) error {
	if len(args) < 1 {
		fmt.Println("Usage: devpt status <name|port>")
		return fmt.Errorf("service name or port required")
	}

	return app.StatusCmd(args[0])
}

func printUsage() {
	usage := `Dev Process Tracker

Default:
  devpt                             Open interactive top UI

Manage services:
  devpt add <name> <cwd> "<cmd>" [ports...]
  devpt start <name> [name...]
  devpt stop <name|--port PORT> [name...]
  devpt restart <name> [name...]
  devpt logs <name> [--lines N]

Patterns (quote to prevent shell expansion):
  '*'              Match any sequence of characters
  'service*'       Match services starting with "service"
  '*-api'          Match services ending with "-api"
  '*web*'          Match services containing "web"

name:port format:
  web-api:3000     Target service "web-api" on port 3000
  "some:thing"     Literal service name containing a colon

Inspect:
  devpt ls [--details]
  devpt status <name|port>

Meta:
  devpt help
  devpt --version

Options:
  --details       Show extended metadata in ls output
  --lines N       Number of log lines to show (default: 50)

Quick start:
  devpt
  devpt add my-app ~/projects/my-app "npm run dev" 3000
  devpt start my-app
  devpt stop my-app

Batch operations:
  devpt start api worker frontend
  devpt stop 'web-*'         # Quote patterns to prevent shell expansion
  devpt restart '*-api' worker
  devpt stop web-api:3000    # Target specific port

Top UI tips:
  Tab switch lists, Enter actions/start, / filter, ? help, ^A add
`
	fmt.Print(usage)
}
