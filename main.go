// Command sonar runs a local containerized SonarQube and scans any project with one command.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"sonar-local/internal/cli"
	"sonar-local/internal/config"
	"sonar-local/internal/ops"
	"sonar-local/internal/tui"
)

const version = "0.1.0"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error: "+err.Error())
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return runMenu()
	}
	cmd, rest := args[0], args[1:]
	switch cmd {
	case "up":
		return cmdUp(rest)
	case "down":
		return cmdDown(rest)
	case "status":
		return cmdStatus(rest)
	case "scan":
		return cmdScan(rest)
	case "version", "--version", "-v":
		fmt.Println("sonar " + version)
		return nil
	case "help", "-h", "--help":
		usage()
		return nil
	default:
		usage()
		return fmt.Errorf("unknown command: %s", cmd)
	}
}

func cmdUp(args []string) error {
	fs := flag.NewFlagSet("up", flag.ContinueOnError)
	plain := fs.Bool("plain", false, "disable the interactive UI")
	if err := fs.Parse(args); err != nil {
		return err
	}
	o, err := ops.New(config.Load())
	if err != nil {
		return err
	}
	return cli.Up(o, isTTY() && !*plain)
}

func cmdDown(args []string) error {
	fs := flag.NewFlagSet("down", flag.ContinueOnError)
	purge := fs.Bool("purge", false, "also remove volumes and the network")
	if err := fs.Parse(args); err != nil {
		return err
	}
	o, err := ops.New(config.Load())
	if err != nil {
		return err
	}
	return cli.Down(o, *purge)
}

func cmdStatus(args []string) error {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	plain := fs.Bool("plain", false, "disable the interactive UI")
	if err := fs.Parse(args); err != nil {
		return err
	}
	o, err := ops.New(config.Load())
	if err != nil {
		return err
	}
	return cli.Status(o, isTTY() && !*plain)
}

// cmdScan parses scan flags by hand so the [PATH] positional may appear anywhere and
// everything after "--" is forwarded verbatim to sonar-scanner.
func cmdScan(args []string) error {
	var opts cli.ScanOptions
	plain := false

	if i := indexOf(args, "--"); i >= 0 {
		opts.ScannerArgs = args[i+1:]
		args = args[:i]
	}

	var positionals []string
	for j := 0; j < len(args); j++ {
		switch args[j] {
		case "--remote":
			opts.Remote = true
		case "--plain":
			plain = true
		case "--key":
			j++
			if j < len(args) {
				opts.Key = args[j]
			}
		case "--name":
			j++
			if j < len(args) {
				opts.Name = args[j]
			}
		default:
			if strings.HasPrefix(args[j], "-") {
				return fmt.Errorf("unknown option: %s", args[j])
			}
			positionals = append(positionals, args[j])
		}
	}
	if len(positionals) > 1 {
		return fmt.Errorf("unexpected extra argument: %s", positionals[1])
	}
	if len(positionals) == 1 {
		opts.Path = positionals[0]
	} else {
		opts.Path = "."
	}

	o, err := ops.New(config.Load())
	if err != nil {
		return err
	}
	return cli.Scan(o, opts, isTTY() && !plain)
}

func runMenu() error {
	if !isTTY() {
		usage()
		return nil
	}
	choice, err := tui.RunMenu()
	if err != nil {
		return err
	}
	switch choice {
	case "up":
		return cmdUp(nil)
	case "scan":
		return cmdScan(nil)
	case "status":
		return cmdStatus(nil)
	case "down":
		return cmdDown(nil)
	default:
		return nil
	}
}

// isTTY reports whether stdout is a terminal, using only the standard library.
func isTTY() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

func indexOf(s []string, target string) int {
	for i, v := range s {
		if v == target {
			return i
		}
	}
	return -1
}

func usage() {
	fmt.Print(`sonar — local SonarQube + one-command scan

Usage:
  sonar up [--plain]                 Start the local stack
  sonar scan [PATH] [options]        Scan a project (default: current dir, local server)
  sonar status [--plain]             Show stack status
  sonar down [--purge]               Stop the stack (--purge wipes volumes)
  sonar                              Interactive menu (TTY only)
  sonar version

scan options:
  --remote          Target the remote SonarQube via SONAR_HOST_URL / SONAR_TOKEN
  --key NAME        Project key (default: cleaned directory name)
  --name NAME       Displayed project name
  --plain           Disable the interactive UI
  -- <args...>      Everything after -- is passed to sonar-scanner
`)
}
