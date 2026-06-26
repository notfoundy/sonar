// Package cli wires the operations layer to either the Bubble Tea UI (TTY) or a plain
// line-based output (non-TTY / --plain).
package cli

import (
	"fmt"
	"path/filepath"
	"time"

	"sonar-local/internal/ops"
	"sonar-local/internal/tui"
)

// Up starts the stack.
func Up(o *ops.Ops, tty bool) error {
	// Run preflight and pull images before the TUI takes over the terminal, so the
	// (potentially long) download progress stays visible in both modes.
	if err := o.Preflight(); err != nil {
		return err
	}
	if err := o.EnsureImages(); err != nil {
		return err
	}
	if tty {
		return tui.RunUp(o)
	}
	for _, s := range ops.DefaultUpSteps(o) {
		fmt.Println("==> " + s.Label)
		if err := s.Run(); err != nil {
			return err
		}
	}
	fmt.Println("Ready. UI: " + o.Cfg.LocalURL() + " (login: admin)")
	fmt.Println("Admin password stored in " + o.AdminPasswordPath())
	return nil
}

// Down stops the stack, optionally purging volumes and network.
func Down(o *ops.Ops, purge bool) error {
	if purge {
		fmt.Println("Removing containers, volumes and network...")
	} else {
		fmt.Println("Stopping containers...")
	}
	if err := o.Teardown(purge); err != nil {
		return err
	}
	if purge {
		fmt.Println("Done (full reset).")
	} else {
		fmt.Println("Done (volumes kept). Use 'sonar down --purge' to wipe everything.")
	}
	return nil
}

// Status shows the stack status.
func Status(o *ops.Ops, tty bool) error {
	if tty {
		return tui.RunStatus(o)
	}
	fmt.Printf("%-18s %s\n", o.Cfg.ServerContainer, o.ContainerState(o.Cfg.ServerContainer))
	fmt.Printf("%-18s %s\n", o.Cfg.DBContainer, o.ContainerState(o.Cfg.DBContainer))
	fmt.Println("server: " + o.CurrentStatus() + " (" + o.Cfg.LocalURL() + ")")
	return nil
}

// ScanOptions carries the parsed flags for a scan.
type ScanOptions struct {
	Path        string
	Key         string
	Name        string
	Remote      bool
	ScannerArgs []string
}

// Scan analyzes a project against the local (default) or remote target.
func Scan(o *ops.Ops, opts ScanOptions, tty bool) error {
	projectDir, err := filepath.Abs(opts.Path)
	if err != nil {
		return err
	}

	key := opts.Key
	if key == "" {
		key = ops.ProjectKey(projectDir)
	}
	name := opts.Name
	if name == "" {
		name = key
	}

	var target ops.ScanTarget
	if opts.Remote {
		if o.Cfg.RemoteURL == "" || o.Cfg.RemoteToken == "" {
			return fmt.Errorf("--remote requires SONAR_HOST_URL and SONAR_TOKEN to be set")
		}
		target = ops.ScanTarget{
			HostURL: o.Cfg.RemoteURL,
			Token:   o.Cfg.RemoteToken,
			DashURL: o.Cfg.RemoteURL,
		}
	} else {
		if o.CurrentStatus() != "UP" {
			return fmt.Errorf("local server is not UP; run 'sonar up' first")
		}
		tokName := fmt.Sprintf("ephemeral-%d", time.Now().UnixNano())
		token, err := o.MintToken(tokName)
		if err != nil {
			return err
		}
		defer func() { _ = o.RevokeToken(tokName) }()
		target = ops.ScanTarget{
			HostURL: o.Cfg.InternalURL(),
			Token:   token,
			DashURL: o.Cfg.LocalURL(),
			Network: o.Cfg.Network,
		}
	}

	args := o.BuildScanArgs(projectDir, key, name, target, opts.ScannerArgs)
	dashboard := target.DashURL + "/dashboard?id=" + key
	title := fmt.Sprintf("Scanning %s (key=%s)", projectDir, key)

	if tty {
		if err := tui.RunScan(o, args, title, dashboard); err != nil {
			return err
		}
	} else {
		fmt.Println("==> " + title)
		if err := o.Rt.Run(args...); err != nil {
			return err
		}
	}

	fmt.Println("Dashboard: " + dashboard)
	return nil
}
