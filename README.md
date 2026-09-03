# henkaipan-cli

The standalone, client-agnostic companion to
[`dyallab/henkaipan-action`](https://github.com/Dyallab/henkaipan-action).
`henkaipan` is a single static Go binary that drives HenKaiPan from any
shell, script, or CI step using the same `X-API-Key` authentication and
the same scan / findings endpoints — no Docker, no MCP handshake, no
client protocol version negotiation.

> Companion to the GitHub Action — not a replacement. Use the action for
> `on: pull_request` PR comments and the `$GITHUB_STEP_SUMMARY`; use the
> CLI for terminal workflows, arbitrary CI runners, cron jobs, and ad-hoc
> scriptable queries.

## Install

```bash
# Latest release (recommended)
go install github.com/dyallab/henkaipan-cli/cmd/henkaipan@latest

# Pin to a tag
go install github.com/dyallab/henkaipan-cli/cmd/henkaipan@v0.1.0
```

Or grab a prebuilt binary from the
[Releases](https://github.com/Dyallab/henkaipan-cli/releases) page.
Binaries are published for `linux`/`darwin`/`windows` × `amd64`/`arm64`.

## Quick start

```bash
export HENKAIPAN_API_URL=https://henkaipan.dyallab.com.ar
export HENKAIPAN_API_KEY=hkp_xxxxxxxxxxxxxxxxxxxx

# Trigger a scan and wait for it to complete, failing on high+ findings
henkaipan scan run \
  --repo-url https://github.com/owner/repo \
  --scanners all \
  --branch main \
  --wait \
  --fail-on high

# Query an in-flight scan
henkaipan scan status <scan-id> --wait

# List findings
henkaipan findings list --severity high --project-id <uuid> --page 1 --page-size 50

# Export findings for an external tool
henkaipan findings export --format json --severity critical --output findings.json
henkaipan findings export --format csv --project-id <uuid> --output findings.csv
```

## Configuration

Configuration is resolved in this order (highest priority first):

1. CLI flags (`--api-url`, `--api-key`, ...)
2. Environment variables (`HENKAIPAN_API_URL`, `HENKAIPAN_API_KEY`, ...)
3. Config file (`~/.config/henkaipan/config.toml`, TOML — see `config.example.toml`)

No built-in defaults — missing file or missing required keys returns an error. Use `--config` to point to a custom file.

| Flag                        | Env var                          | Config key (TOML)          | Purpose                                   |
| --------------------------- | -------------------------------- | -------------------------- | ----------------------------------------- |
| `--api-url`                 | `HENKAIPAN_API_URL`              | `api_url`                  | Base URL of the HenKaiPan API             |
| `--api-key`                 | `HENKAIPAN_API_KEY`              | `api_key`                  | API token used as `X-API-Key`             |
| `--cf-access-client-id`     | `HENKAIPAN_CF_ACCESS_CLIENT_ID`  | `cf_access_client_id`      | Cloudflare Access Service Token client ID |
| `--cf-access-client-secret` | `HENKAIPAN_CF_ACCESS_CLIENT_SECRET` | `cf_access_client_secret` | Cloudflare Access Service Token client secret |
| `--output`                  | `HENKAIPAN_OUTPUT`               | `output`                   | Output format (`table`, `json`, `yaml`)   |
| `--timeout`                 | `HENKAIPAN_TIMEOUT`              | `timeout_seconds`          | HTTP request timeout in seconds           |
| `--config`                  | —                                | —                          | Path to TOML config file (default `~/.config/henkaipan/config.toml`) |

```bash
# Create config from example
mkdir -p ~/.config/henkaipan
cp config.example.toml ~/.config/henkaipan/config.toml
# edit api_key, then run without env vars
henkaipan scan run --repo-url https://github.com/owner/repo --wait --fail-on high
```

The API key is **never printed** — it is wrapped in a typed value that
masks itself on every format verb (`%s`, `%v`, `String()`, `GoString()`).
If you ever see `***` in debug output, that is by design.

## Commands

### `henkaipan scan run`

Trigger a scan against `POST /api/v1/scans/external`.

```text
Flags:
      --repo-url string        Repo URL (auto-creates the project on first run)
      --project-id string      UUID of an existing project (mutually exclusive with --repo-url)
      --scanners string        Comma-separated scanners or pack: all, sast, sca, secrets, vuln, containers
      --branch string          Git branch to scan
      --auto-create-project    Auto-create the project from --repo-url when --project-id is empty (default true)
      --wait                   Block until all scans reach a terminal state
      --poll-interval int      Seconds between status polls when --wait is set (default 15)
      --fail-on string         Exit 1 if any finding meets or exceeds this severity (critical, high, medium, low, none)
```

### `henkaipan scan status <scan-id>`

Read `GET /api/v1/scans/{id}/status` (or poll with `--wait`).

```text
Flags:
      --wait               Block until the scan reaches a terminal state
      --poll-interval int  Seconds between status polls (default 15)
      --fail-on string     Exit 1 if any finding meets or exceeds this severity
```

### `henkaipan findings list`

Query `GET /api/findings` with filters and pagination.

```text
Flags:
      --severity string    Filter by severity (critical|high|medium|low)
      --status string      Filter by finding status (e.g. open, fixed)
      --scanner string     Filter by scanner name (semgrep, trivy, gitleaks, grype, nuclei)
      --project-id string  Scope to a project UUID
      --page int           Page number (default 1)
      --page-size int      Results per page (default 50)
      --fail-on string     Exit 1 if any finding meets or exceeds this severity
```

### `henkaipan findings export`

Stream `GET /api/findings/export` to stdout or to a file.

```text
Flags:
      --format string    Export format: json or csv (default json)
      --severity string  Filter by severity
      --status string    Filter by finding status
      --scanner string   Filter by scanner name
      --project-id string  Scope to a project UUID
      --output string    Write to this file instead of stdout
```

## Severity weights

`--fail-on` uses the same severity weights as `henkaipan-action`:

| Severity | Weight |
| -------- | -----: |
| critical |      4 |
| high     |      3 |
| medium   |      2 |
| low      |      1 |

`--fail-on high` means "fail if **any** finding is high or critical" — it
is not "fail if all findings are high". This matches `gh`, `golangci-lint`,
and the action's contract.

## CI usage

### Generic Linux runner

```yaml
- name: Run HenKaiPan security scan
  env:
    HENKAIPAN_API_URL: ${{ secrets.HENKAIPAN_API_URL }}
    HENKAIPAN_API_KEY: ${{ secrets.HENKAIPAN_API_KEY }}
  run: |
    henkaipan scan run \
      --repo-url "$GITHUB_SERVER_URL/$GITHUB_REPOSITORY.git" \
      --branch "$GITHUB_REF_NAME" \
      --wait \
      --fail-on high
```

### Behind Cloudflare Access

```yaml
- name: Run HenKaiPan security scan
  env:
    HENKAIPAN_API_URL: ${{ secrets.HENKAIPAN_API_URL }}
    HENKAIPAN_API_KEY: ${{ secrets.HENKAIPAN_API_KEY }}
    HENKAIPAN_CF_ACCESS_CLIENT_ID: ${{ secrets.CF_CLIENT_ID }}
    HENKAIPAN_CF_ACCESS_CLIENT_SECRET: ${{ secrets.CF_CLIENT_SECRET }}
  run: henkaipan scan run --repo-url "$GITHUB_SERVER_URL/$GITHUB_REPOSITORY.git" --wait --fail-on high
```

## Troubleshooting

**`received HTML (HTTP 403) from <api-url>; likely a proxy/firewall challenge`**
This means a reverse proxy (commonly Cloudflare) returned an HTML
challenge page instead of JSON. Either:
- configure the proxy to skip challenges on `/api/v1/*`, or
- pass `--cf-access-client-id` / `--cf-access-client-secret` if the
  instance is behind Cloudflare Access with a Service Token policy.

**`api: HTTP 401 from <api-url>`**
The API key is missing or invalid. Confirm `HENKAIPAN_API_KEY` is set
and the token has not been revoked.

**`api: HTTP 403 ... token is not scoped to this project`**
You used a project-scoped token against a different project. Generate a
new unscoped token, or scope the token to the correct project.

## Development

```bash
# Run the CLI against a local instance
go run ./cmd/henkaipan --api-url http://localhost:8080 scan run --repo-url ...

# Run tests
go test ./...

# Build all platforms
make build   # or: go build -o bin/henkaipan ./cmd/henkaipan
```

## License

MIT — see [LICENSE](./LICENSE).