// Package ops contains the UI-agnostic operations: starting/stopping containers, waiting
// for the server, managing the admin password and analysis tokens, building scan args.
package ops

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"sonar-local/internal/config"
	"sonar-local/internal/container"
	"sonar-local/internal/sonar"
)

// MaxMapCount is the minimum vm.max_map_count required by SonarQube's embedded Elasticsearch.
const MaxMapCount = 524288

// Ops bundles the config, the resolved runtime and the local API client.
type Ops struct {
	Cfg    *config.Config
	Rt     *container.Runtime
	Client *sonar.Client
}

// New resolves the container runtime and returns the operations facade.
func New(cfg *config.Config) (*Ops, error) {
	rt, err := container.Detect(cfg)
	if err != nil {
		return nil, err
	}
	return &Ops{Cfg: cfg, Rt: rt, Client: sonar.New(cfg.LocalURL())}, nil
}

// Step is one named unit of work in the startup sequence.
type Step struct {
	Label string
	Run   func() error
}

// DefaultUpSteps is the ordered startup sequence shared by the TUI and plain paths.
// Preflight and image pulls run before these (see cli.Up) so their output is visible.
func DefaultUpSteps(o *Ops) []Step {
	return []Step{
		{"Ensuring network", o.EnsureNetwork},
		{"Starting database", o.StartDB},
		{"Starting SonarQube", o.StartServer},
		{"Waiting for server to be UP", func() error { return o.WaitUp(5 * time.Minute) }},
		{"Initializing admin", o.BootstrapAdmin},
	}
}

// EnsureImages pulls the database and server images if missing, streaming the runtime's
// download progress so the first run isn't a silent wait.
func (o *Ops) EnsureImages() error {
	for _, img := range []string{o.Cfg.PostgresImage, o.Cfg.SonarImage} {
		if o.Rt.ImageExists(img) {
			continue
		}
		fmt.Println("Pulling " + img + " ...")
		if err := o.Rt.Pull(img); err != nil {
			return err
		}
	}
	return nil
}

// Preflight verifies vm.max_map_count on native Linux (Docker Desktop VMs handle it elsewhere).
func (o *Ops) Preflight() error {
	if runtime.GOOS != "linux" {
		return nil
	}
	data, err := os.ReadFile("/proc/sys/vm/max_map_count")
	if err != nil {
		return nil // unreadable: let the server attempt to start anyway
	}
	cur, _ := strconv.Atoi(strings.TrimSpace(string(data)))
	if cur < MaxMapCount {
		return fmt.Errorf("vm.max_map_count=%d < %d (required by SonarQube/Elasticsearch)\n"+
			"  fix now:        sudo sysctl -w vm.max_map_count=%d\n"+
			"  make permanent: echo 'vm.max_map_count=%d' | sudo tee /etc/sysctl.d/99-sonarqube.conf",
			cur, MaxMapCount, MaxMapCount, MaxMapCount)
	}
	return nil
}

// EnsureNetwork creates the shared network if needed.
func (o *Ops) EnsureNetwork() error {
	return o.Rt.NetworkEnsure(o.Cfg.Network)
}

// StartDB starts (or reuses) the PostgreSQL container.
func (o *Ops) StartDB() error {
	c := o.Cfg
	if o.Rt.ContainerRunning(c.DBContainer) {
		return nil
	}
	if o.Rt.ContainerExists(c.DBContainer) {
		return o.Rt.RunCaptured("start", c.DBContainer)
	}
	return o.Rt.RunCaptured(
		"run", "-d",
		"--name", c.DBContainer,
		"--network", c.Network,
		"--restart", "unless-stopped",
		"-e", "POSTGRES_USER="+c.PostgresUser,
		"-e", "POSTGRES_PASSWORD="+c.PostgresPassword,
		"-e", "POSTGRES_DB="+c.PostgresDB,
		"-v", c.VolPg+":/var/lib/postgresql/data",
		c.PostgresImage,
	)
}

// StartServer starts (or reuses) the SonarQube container.
func (o *Ops) StartServer() error {
	c := o.Cfg
	if o.Rt.ContainerRunning(c.ServerContainer) {
		return nil
	}
	if o.Rt.ContainerExists(c.ServerContainer) {
		return o.Rt.RunCaptured("start", c.ServerContainer)
	}
	jdbc := fmt.Sprintf("jdbc:postgresql://%s:5432/%s", c.DBContainer, c.PostgresDB)
	return o.Rt.RunCaptured(
		"run", "-d",
		"--name", c.ServerContainer,
		"--network", c.Network,
		"--restart", "unless-stopped",
		"-e", "SONAR_JDBC_URL="+jdbc,
		"-e", "SONAR_JDBC_USERNAME="+c.PostgresUser,
		"-e", "SONAR_JDBC_PASSWORD="+c.PostgresPassword,
		"-p", c.Port+":9000",
		"-v", c.VolData+":/opt/sonarqube/data",
		"-v", c.VolLogs+":/opt/sonarqube/logs",
		"-v", c.VolExt+":/opt/sonarqube/extensions",
		"--ulimit", "nofile=65536:65536",
		c.SonarImage,
	)
}

// WaitUp polls the API until the status is UP or the timeout elapses.
func (o *Ops) WaitUp(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if s, err := o.Client.Status(); err == nil && s == "UP" {
			return nil
		}
		time.Sleep(3 * time.Second)
	}
	return fmt.Errorf("SonarQube did not reach UP within %s", timeout)
}

// CurrentStatus returns the live status string, or "unreachable".
func (o *Ops) CurrentStatus() string {
	s, err := o.Client.Status()
	if err != nil || s == "" {
		return "unreachable"
	}
	return s
}

// ContainerState reports a container as running, stopped or absent.
func (o *Ops) ContainerState(name string) string {
	switch {
	case o.Rt.ContainerRunning(name):
		return "running"
	case o.Rt.ContainerExists(name):
		return "stopped"
	default:
		return "absent"
	}
}

// BootstrapAdmin sets the admin password from SONAR_ADMIN_PASSWORD on first run.
// Idempotent: if those credentials already work, it does nothing.
func (o *Ops) BootstrapAdmin() error {
	pass := o.Cfg.AdminPassword
	if pass == "" {
		return fmt.Errorf("SONAR_ADMIN_PASSWORD is not set (define it in .env)")
	}
	if o.Client.CheckAdmin(pass) {
		return nil // already configured with this password
	}
	if err := o.Client.ChangeAdminPassword("admin", pass); err != nil {
		return fmt.Errorf("failed to set the admin password: %w\n"+
			"  - HTTP 400: the password does not meet SonarQube's policy\n"+
			"  - HTTP 401: admin was already changed to another value; reset with 'sonar down --purge' then 'sonar up'", err)
	}
	return nil
}

// AdminPassword returns the configured admin password.
func (o *Ops) AdminPassword() (string, error) {
	if o.Cfg.AdminPassword == "" {
		return "", fmt.Errorf("SONAR_ADMIN_PASSWORD is not set (define it in .env)")
	}
	return o.Cfg.AdminPassword, nil
}

// MintToken generates an ephemeral analysis token.
func (o *Ops) MintToken(name string) (string, error) {
	pass, err := o.AdminPassword()
	if err != nil {
		return "", err
	}
	return o.Client.GenerateToken(pass, name)
}

// RevokeToken revokes a previously minted token.
func (o *Ops) RevokeToken(name string) error {
	pass, err := o.AdminPassword()
	if err != nil {
		return err
	}
	return o.Client.RevokeToken(pass, name)
}

// Teardown stops and removes the containers; with purge it also drops the volumes and network.
func (o *Ops) Teardown(purge bool) error {
	c := o.Cfg
	o.Rt.Remove(c.ServerContainer)
	o.Rt.Remove(c.DBContainer)
	if purge {
		for _, v := range []string{c.VolData, c.VolLogs, c.VolExt, c.VolPg} {
			o.Rt.VolumeRemove(v)
		}
		o.Rt.NetworkRemove(c.Network)
	}
	return nil
}

// ScanTarget describes where a scan sends its report.
type ScanTarget struct {
	HostURL string // value handed to the scanner (internal URL for local)
	Token   string
	DashURL string // browser URL used for the dashboard link
	Network string // network the scanner joins, empty for remote
}

// BuildScanArgs assembles the runtime "run" arguments for the scanner container.
func (o *Ops) BuildScanArgs(projectDir, key, name string, t ScanTarget, extra []string) []string {
	args := []string{"run", "--rm"}
	if t.Network != "" {
		args = append(args, "--network", t.Network)
	}
	args = append(
		args,
		"-e", "SONAR_HOST_URL="+t.HostURL,
		"-e", "SONAR_TOKEN="+t.Token,
		"-v", projectDir+":/usr/src:z",
		o.Cfg.ScannerImage,
		"-Dsonar.projectKey="+key,
		"-Dsonar.projectName="+name,
	)
	return append(args, extra...)
}

var keyClean = regexp.MustCompile(`[^A-Za-z0-9._:-]`)

// ProjectKey derives a valid project key from a directory path.
func ProjectKey(path string) string {
	return keyClean.ReplaceAllString(filepath.Base(path), "_")
}
