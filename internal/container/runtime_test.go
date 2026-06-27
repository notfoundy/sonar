package container

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"sonar-local/internal/config"
)

// writeFakeBin creates an executable file named bin in dir and returns its full path.
func writeFakeBin(t *testing.T, dir, bin, body string) string {
	t.Helper()
	path := filepath.Join(dir, bin)
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestDetectOverride(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-script fake bin not supported on Windows")
	}
	dir := t.TempDir()
	writeFakeBin(t, dir, "myruntime", "#!/bin/sh\n")
	t.Setenv("PATH", dir)

	rt, err := Detect(&config.Config{RuntimeOverride: "myruntime"})
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(rt.Bin) != "myruntime" {
		t.Errorf("Bin = %q, want myruntime", rt.Bin)
	}
}

func TestDetectDefaultOrder(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-script fake bin not supported on Windows")
	}
	dir := t.TempDir()
	writeFakeBin(t, dir, "docker", "#!/bin/sh\n")
	writeFakeBin(t, dir, "podman", "#!/bin/sh\n")
	t.Setenv("PATH", dir)

	rt, err := Detect(&config.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(rt.Bin) != "docker" {
		t.Errorf("Bin = %q, want docker (preferred first)", rt.Bin)
	}
}

func TestDetectNoneFound(t *testing.T) {
	t.Setenv("PATH", t.TempDir()) // empty dir: nothing resolvable

	_, err := Detect(&config.Config{})
	if err == nil {
		t.Fatal("expected error when no runtime is found")
	}
	if !strings.Contains(err.Error(), "docker") || !strings.Contains(err.Error(), "podman") {
		t.Errorf("error should mention candidates, got %v", err)
	}
}

func TestOutputTrims(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-script fake bin not supported on Windows")
	}
	bin := writeFakeBin(t, t.TempDir(), "echoer", "#!/bin/sh\nprintf '  hello  \\n'\n")
	r := &Runtime{Bin: bin}

	got, err := r.Output("anything")
	if err != nil {
		t.Fatal(err)
	}
	if got != "hello" {
		t.Errorf("Output = %q, want trimmed \"hello\"", got)
	}
}

func TestRunCapturedSurfacesOutput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell-script fake bin not supported on Windows")
	}
	bin := writeFakeBin(t, t.TempDir(), "failer", "#!/bin/sh\necho boom >&2\nexit 1\n")
	r := &Runtime{Bin: bin}

	err := r.RunCaptured("do", "stuff")
	if err == nil {
		t.Fatal("expected error from failing command")
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Errorf("error should surface combined output, got %v", err)
	}
}
