// Package config holds runtime configuration with sane defaults, overridable via
// environment variables (and an optional .env file in the current directory).
package config

import (
	"bufio"
	"os"
	"strings"
)

// Config carries every tunable value the CLI needs.
type Config struct {
	SonarImage   string
	PostgresImage string
	ScannerImage string

	Port             string
	PostgresUser     string
	PostgresPassword string
	PostgresDB       string

	Network         string
	ServerContainer string
	DBContainer     string
	VolData         string
	VolLogs         string
	VolExt          string
	VolPg           string

	RemoteURL       string
	RemoteToken     string
	RuntimeOverride string

	AdminPassword string
}

// Load builds a Config from defaults, an optional ./.env file, and process env.
// Process environment always wins over the .env file.
func Load() *Config {
	loadDotEnv(".env")
	return &Config{
		// Fully-qualified, pinned refs: Podman with short-name-mode=enforcing cannot resolve
		// bare names (e.g. "sonarqube:community") without a TTY prompt. Docker accepts these too.
		SonarImage:    env("SONAR_IMAGE", "docker.io/library/sonarqube:26.6.0.123539-community"),
		PostgresImage: env("POSTGRES_IMAGE", "docker.io/library/postgres:17.2-alpine"),
		ScannerImage:  env("SCANNER_IMAGE", "docker.io/sonarsource/sonar-scanner-cli:12.1"),

		Port:             env("SONAR_PORT", "9000"),
		PostgresUser:     env("POSTGRES_USER", "sonar"),
		PostgresPassword: env("POSTGRES_PASSWORD", "sonar"),
		PostgresDB:       env("POSTGRES_DB", "sonar"),

		Network:         "sonar-network",
		ServerContainer: "sonarqube",
		DBContainer:     "sonarqube-db",
		VolData:         "sonar-data",
		VolLogs:         "sonar-logs",
		VolExt:          "sonar-extensions",
		VolPg:           "sonar-db",

		RemoteURL:       env("SONAR_HOST_URL", ""),
		RemoteToken:     env("SONAR_TOKEN", ""),
		RuntimeOverride: env("SONAR_RUNTIME", ""),

		AdminPassword: env("SONAR_ADMIN_PASSWORD", ""),
	}
}

// LocalURL is the host-facing URL of the local server.
func (c *Config) LocalURL() string { return "http://localhost:" + c.Port }

// InternalURL is the URL the scanner container uses to reach the server on the shared network.
func (c *Config) InternalURL() string { return "http://" + c.ServerContainer + ":9000" }

func env(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

// loadDotEnv reads KEY=VALUE lines into the process env without overriding existing vars.
func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.Trim(strings.TrimSpace(v), `"'`)
		if _, exists := os.LookupEnv(k); !exists {
			_ = os.Setenv(k, v)
		}
	}
}
