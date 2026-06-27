package ops

import (
	"os/exec"
	"strings"
	"testing"

	"sonar-local/internal/config"
)

func TestProjectKey(t *testing.T) {
	cases := []struct {
		name   string
		dir    string
		branch string
		want   string
	}{
		{"base only", "/home/user/myproj", "", "myproj"},
		{"special chars cleaned", "/tmp/my proj@1", "", "my_proj_1"},
		{"with branch", "/home/user/myproj", "main", "myproj:main"},
		{"branch cleaned", "/home/user/myproj", "feature/x", "myproj:feature_x"},
		{"allowed chars kept", "/srv/a.b_c-d", "", "a.b_c-d"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ProjectKey(tc.dir, tc.branch); got != tc.want {
				t.Errorf("ProjectKey(%q, %q) = %q, want %q", tc.dir, tc.branch, got, tc.want)
			}
		})
	}
}

func newOps() *Ops {
	return &Ops{Cfg: &config.Config{ScannerImage: "scanner:test"}}
}

func contains(args []string, want string) bool {
	for _, a := range args {
		if a == want {
			return true
		}
	}
	return false
}

// hasPair reports whether flag is immediately followed by value in args.
func hasPair(args []string, flag, value string) bool {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == flag && args[i+1] == value {
			return true
		}
	}
	return false
}

func TestBuildScanArgsLocal(t *testing.T) {
	o := newOps()
	target := ScanTarget{HostURL: "http://sonarqube:9000", Token: "tok", Network: "sonar-network"}
	args := o.BuildScanArgs("/code/proj", "proj", "Proj", target, []string{"-Dsonar.sources=src"})

	if !hasPair(args, "--network", "sonar-network") {
		t.Errorf("expected --network sonar-network in %v", args)
	}
	if !hasPair(args, "-e", "SONAR_HOST_URL=http://sonarqube:9000") {
		t.Errorf("missing SONAR_HOST_URL in %v", args)
	}
	if !hasPair(args, "-e", "SONAR_TOKEN=tok") {
		t.Errorf("missing SONAR_TOKEN in %v", args)
	}
	if !hasPair(args, "-v", "/code/proj:/usr/src:z") {
		t.Errorf("missing source mount in %v", args)
	}
	if !contains(args, "scanner:test") {
		t.Errorf("missing scanner image in %v", args)
	}
	if !contains(args, "-Dsonar.projectKey=proj") || !contains(args, "-Dsonar.projectName=Proj") {
		t.Errorf("missing project key/name in %v", args)
	}
	if args[len(args)-1] != "-Dsonar.sources=src" {
		t.Errorf("extra arg should be appended last, got %v", args)
	}
}

func TestBuildScanArgsRemoteHasNoNetwork(t *testing.T) {
	o := newOps()
	target := ScanTarget{HostURL: "https://sonar.example.com", Token: "tok"} // Network empty
	args := o.BuildScanArgs("/code/proj", "proj", "Proj", target, nil)

	if contains(args, "--network") {
		t.Errorf("remote scan must not set --network, got %v", args)
	}
}

func TestGitBranch(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	run("init", "-b", "main")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "Test")
	run("commit", "--allow-empty", "-m", "init")

	if got := GitBranch(dir); got != "main" {
		t.Errorf("GitBranch = %q, want main", got)
	}

	// Detached HEAD -> "".
	head, err := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	run("checkout", strings.TrimSpace(string(head)))
	if got := GitBranch(dir); got != "" {
		t.Errorf("GitBranch (detached) = %q, want empty", got)
	}

	// Non-git directory -> "".
	if got := GitBranch(t.TempDir()); got != "" {
		t.Errorf("GitBranch (non-repo) = %q, want empty", got)
	}
}
