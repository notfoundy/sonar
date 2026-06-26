// Package container abstracts the container runtime (Docker or Podman) behind a small
// command-execution wrapper.
package container

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"sonar-local/internal/config"
)

// Runtime is a resolved container CLI (docker or podman).
type Runtime struct {
	Bin string
}

// Detect picks the runtime: SONAR_RUNTIME override, otherwise docker, otherwise podman.
func Detect(cfg *config.Config) (*Runtime, error) {
	candidates := []string{"docker", "podman"}
	if cfg.RuntimeOverride != "" {
		candidates = []string{cfg.RuntimeOverride}
	}
	for _, name := range candidates {
		if path, err := exec.LookPath(name); err == nil {
			return &Runtime{Bin: path}, nil
		}
	}
	return nil, fmt.Errorf(
		"no container runtime found (looked for: %s); install Docker or Podman, or set SONAR_RUNTIME",
		strings.Join(candidates, ", "))
}

// Run executes a runtime command, streaming its output to the current process.
func (r *Runtime) Run(args ...string) error {
	cmd := exec.Command(r.Bin, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// Output executes a runtime command and returns its trimmed stdout.
func (r *Runtime) Output(args ...string) (string, error) {
	out, err := exec.Command(r.Bin, args...).Output()
	return strings.TrimSpace(string(out)), err
}

// RunCaptured runs a command and, on failure, returns an error carrying the runtime's own
// stderr/stdout so the real cause (e.g. port in use, image pull failure) is visible even
// when the TUI owns the terminal.
func (r *Runtime) RunCaptured(args ...string) error {
	out, err := exec.Command(r.Bin, args...).CombinedOutput()
	if err == nil {
		return nil
	}
	msg := strings.TrimSpace(string(out))
	if msg == "" {
		return fmt.Errorf("%s %s: %w", filepath.Base(r.Bin), strings.Join(args, " "), err)
	}
	return fmt.Errorf("%s %s: %s", filepath.Base(r.Bin), strings.Join(args, " "), msg)
}

// quiet runs a command discarding output and reports whether it succeeded.
func (r *Runtime) quiet(args ...string) bool {
	return exec.Command(r.Bin, args...).Run() == nil
}

// NetworkEnsure creates the user network if it does not already exist.
func (r *Runtime) NetworkEnsure(name string) error {
	if r.quiet("network", "inspect", name) {
		return nil
	}
	return r.RunCaptured("network", "create", name)
}

// ImageExists reports whether an image is already present locally.
func (r *Runtime) ImageExists(ref string) bool {
	return r.quiet("image", "inspect", ref)
}

// Pull downloads an image, streaming the runtime's own progress to the terminal.
func (r *Runtime) Pull(ref string) error {
	return r.Run("pull", ref)
}

// ContainerExists reports whether a container with this name exists (any state).
func (r *Runtime) ContainerExists(name string) bool {
	return r.quiet("container", "inspect", name)
}

// ContainerRunning reports whether the named container is currently running.
func (r *Runtime) ContainerRunning(name string) bool {
	out, err := r.Output("inspect", "-f", "{{.State.Running}}", name)
	return err == nil && out == "true"
}

// Remove force-removes a container (no error if absent).
func (r *Runtime) Remove(name string) { r.quiet("rm", "-f", name) }

// VolumeRemove force-removes a named volume (no error if absent).
func (r *Runtime) VolumeRemove(name string) { r.quiet("volume", "rm", "-f", name) }

// NetworkRemove removes a network (no error if absent or in use).
func (r *Runtime) NetworkRemove(name string) { r.quiet("network", "rm", name) }

// RunStream starts a runtime command and streams its combined stdout/stderr line by line
// on the returned channel, which closes when the output ends. Call cmd.Wait() afterwards
// to obtain the exit status.
func (r *Runtime) RunStream(args ...string) (*exec.Cmd, <-chan string, error) {
	cmd := exec.Command(r.Bin, args...)

	pr, pw, err := os.Pipe()
	if err != nil {
		return nil, nil, err
	}
	cmd.Stdout = pw
	cmd.Stderr = pw

	if err := cmd.Start(); err != nil {
		pw.Close()
		pr.Close()
		return nil, nil, err
	}
	// The child holds its own copy of the write end; close ours so EOF propagates.
	pw.Close()

	lines := make(chan string)
	go func() {
		defer close(lines)
		sc := bufio.NewScanner(pr)
		sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for sc.Scan() {
			lines <- sc.Text()
		}
		pr.Close()
	}()

	return cmd, lines, nil
}
