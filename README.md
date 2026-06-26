# Local SonarQube

A small cross-platform CLI (`sonar`) that runs a persistent **SonarQube Community** server
(PostgreSQL) in containers and scans any project on your machine with one command. A scan
targets the local server by default, or a real remote SonarQube with `--remote`.

- **One static binary** — Linux/macOS/Windows, amd64/arm64. No bash, no compose.
- **Docker or Podman** — auto-detected (override with `SONAR_RUNTIME`).
- **Interactive TUI** (Bubble Tea) with a plain fallback for non-TTY/CI.

## Requirements

- **Docker** or **Podman**.
- **`vm.max_map_count >= 524288`** on Linux (embedded Elasticsearch):
  ```bash
  sudo sysctl -w vm.max_map_count=524288
  echo 'vm.max_map_count=524288' | sudo tee /etc/sysctl.d/99-sonarqube.conf   # persistent
  ```
  (Not needed on macOS/Windows: handled inside the Docker Desktop VM.)
- A **Go** toolchain to build (1.22+).

## Build

```bash
go mod tidy        # first time: fetches bubbletea/bubbles/lipgloss, writes go.sum
make build         # or: go build -o sonar .
```

Put it on your PATH:

```bash
sudo install ./sonar /usr/local/bin/sonar      # or copy anywhere on PATH
```

Cross-compiled binaries for all O/arch: `make release` → `dist/`.

## Usage

```bash
sonar up                       # start the stack (progress + spinner)
sonar scan [PATH]              # scan a project (default: current dir, local server)
sonar status                   # container + server status (table)
sonar down [--purge]           # stop (--purge also wipes volumes)
sonar                          # interactive menu (TTY only)
```

On first `up`, the default `admin/admin` password is replaced by a random one stored in
your user config dir (`os.UserConfigDir()/sonar-local/admin-password`). The UI is at
<http://localhost:9000> (login `admin`).

### Scan

```bash
sonar scan                     # current directory
sonar scan ~/code/api          # a given path
sonar scan --key my-key ~/app  # explicit project key
sonar scan -- -Dsonar.sources=src -Dsonar.exclusions='**/*.test.js'
```

The project key defaults to the (cleaned) directory name. On the local target an
**ephemeral analysis token** is minted before the scan and **revoked right after** — no
long-lived token is stored. A `sonar-project.properties` at the project root is honored.

### Remote SonarQube

`--remote` sends the report to a real server using env vars (your own token, no
mint/revoke):

```bash
export SONAR_HOST_URL=https://sonar.example.com
export SONAR_TOKEN=sqp_xxxxxxxx
sonar scan --remote ~/code/api
```

These can also live in a `.env` file in the current directory.

### Options & flags

```
sonar scan [PATH] [options] [-- scanner-args...]
  --remote        Target the remote SonarQube (SONAR_HOST_URL / SONAR_TOKEN)
  --key NAME      Project key (default: cleaned directory name)
  --name NAME     Displayed project name
  --plain         Disable the TUI (also auto-disabled when stdout isn't a terminal)
```

## Configuration

All values have defaults and can be set via env or a `.env` file in the current directory
(process env wins). See `.env.example`. Notable: `SONAR_PORT`, `SONAR_IMAGE`,
`SCANNER_IMAGE`, `SONAR_RUNTIME` (force `docker`/`podman`).

## Persistence

Data lives in named volumes (`pg_data`, `sonar_data`, ...): it survives `sonar down` and
restarts. Only `sonar down --purge` removes the volumes, the network and the stored admin
password.

## Troubleshooting

- **`vm.max_map_count` too low** → see Requirements.
- **Server never reaches `UP`** → `sonar status`, then `docker logs sonarqube`
  (or `podman logs sonarqube`).
- **Admin bootstrap fails on first `up`** → the admin password was already changed; write
  it to the file shown by `sonar status`/the `up` output.
- **Port 9000 in use** → set `SONAR_PORT` and re-run `sonar up`.
