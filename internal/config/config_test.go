package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	// Clear any env that could override the defaults under test.
	for _, k := range []string{"SONAR_IMAGE", "SONAR_PORT", "POSTGRES_USER", "SONAR_HOST_URL", "SONAR_ADMIN_PASSWORD"} {
		t.Setenv(k, "")
		os.Unsetenv(k)
	}

	c := Load()
	if c.Port != "9000" {
		t.Errorf("Port = %q, want 9000", c.Port)
	}
	if c.PostgresUser != "sonar" {
		t.Errorf("PostgresUser = %q, want sonar", c.PostgresUser)
	}
	if c.SonarImage == "" {
		t.Error("SonarImage should have a default")
	}
	if c.RemoteURL != "" {
		t.Errorf("RemoteURL = %q, want empty", c.RemoteURL)
	}
}

func TestLoadEnvOverride(t *testing.T) {
	t.Setenv("SONAR_PORT", "9999")
	if got := Load().Port; got != "9999" {
		t.Errorf("Port = %q, want 9999", got)
	}
}

func TestLoadEmptyEnvFallsBackToDefault(t *testing.T) {
	// An empty value must not shadow the default (config.go: `ok && v != ""`).
	t.Setenv("SONAR_PORT", "")
	if got := Load().Port; got != "9000" {
		t.Errorf("Port = %q, want default 9000", got)
	}
}

func TestLocalAndInternalURL(t *testing.T) {
	c := &Config{Port: "9000", ServerContainer: "sonarqube"}
	if got := c.LocalURL(); got != "http://localhost:9000" {
		t.Errorf("LocalURL = %q", got)
	}
	if got := c.InternalURL(); got != "http://sonarqube:9000" {
		t.Errorf("InternalURL = %q", got)
	}
}

func TestLoadDotEnv(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	content := "" +
		"# a comment\n" +
		"\n" +
		"DOTENV_PLAIN=hello\n" +
		`DOTENV_QUOTED="quoted value"` + "\n" +
		"DOTENV_SINGLE='single'\n" +
		"  DOTENV_SPACED = spaced \n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	// Ensure a clean slate for the keys we assert on.
	for _, k := range []string{"DOTENV_PLAIN", "DOTENV_QUOTED", "DOTENV_SINGLE", "DOTENV_SPACED"} {
		os.Unsetenv(k)
		t.Cleanup(func() { os.Unsetenv(k) })
	}

	loadDotEnv(path)

	cases := map[string]string{
		"DOTENV_PLAIN":  "hello",
		"DOTENV_QUOTED": "quoted value",
		"DOTENV_SINGLE": "single",
		"DOTENV_SPACED": "spaced",
	}
	for k, want := range cases {
		if got := os.Getenv(k); got != want {
			t.Errorf("%s = %q, want %q", k, got, want)
		}
	}
}

func TestLoadDotEnvDoesNotOverrideProcessEnv(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte("DOTENV_PRESET=from_file\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("DOTENV_PRESET", "from_process")
	loadDotEnv(path)
	if got := os.Getenv("DOTENV_PRESET"); got != "from_process" {
		t.Errorf("DOTENV_PRESET = %q, want from_process (process env must win)", got)
	}
}

func TestLoadDotEnvMissingFileIsNoop(t *testing.T) {
	loadDotEnv(filepath.Join(t.TempDir(), "does-not-exist"))
}
