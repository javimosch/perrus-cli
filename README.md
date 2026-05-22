# perrus-cli

**Clean-room Go reimplementation of [Gatus](https://github.com/TwiN/gatus) (10k+ stars)** — built for agent-first, no-human CLI usage.

> **Architecture**: Follows the opinionated agent-first CLI pattern from [boilerplate-cli-ui-go](https://github.com/javimosch/boilerplate-cli-ui-go). Every command produces machine-readable output, uses non-zero exit codes to signal failure, and supports fully unattended daemon operation — no prompts, no interactive menus.

## Why perrus-cli

[Gatus](https://github.com/TwiN/gatus) is an excellent health-monitoring platform, but its daemon and web UI are tightly coupled and not designed for scripted/agent use. perrus-cli reimplements the same YAML config format and condition syntax in a single Go binary with a purpose-built agent CLI layer:

- **`check`** — one-shot endpoint probe, exits `0` (pass) or `2` (fail), `--json` for structured output
- **`results`** — print current history as table or JSON, pipe directly to jq/agents
- **`start -daemon`** — fully daemonized with PID file, log file, no TTY required
- **`status` / `stop`** — scriptable lifecycle management

## Quick start

```bash
go build -o perrus-cli .
sudo mv perrus-cli /usr/local/bin/

perrus-cli config-init          # generate config.yaml
perrus-cli config-validate      # validate before starting
perrus-cli start -daemon        # run in background (http://localhost:8080)
perrus-cli status               # check daemon health
```

Requires Go 1.21+. Cross-compile for Linux: `GOOS=linux GOARCH=amd64 go build -o perrus-cli-linux .`

## Config

```yaml
endpoints:
  - name: "my-api"
    group: "Web Services"
    url: "https://api.example.com/health"
    interval: "30s"
    conditions:
      - "[STATUS] == 200"
      - "[RESPONSE_TIME] < 500"

  - name: "my-db"
    group: "Infrastructure"
    url: "tcp://db.example.com:5432"
    interval: "60s"
    conditions:
      - "[CONNECTED] == true"
```

### Supported URL schemes

| Scheme | Description |
|--------|-------------|
| `https://`, `http://` | HTTP/HTTPS request |
| `tcp://host:port`     | TCP connect check  |
| `dns://hostname`      | DNS resolution     |

### Condition placeholders

| Placeholder | Operators | Description |
|-------------|-----------|-------------|
| `[STATUS]`        | `==` `!=` `<` `<=` `>` `>=` | HTTP status code |
| `[RESPONSE_TIME]` | `==` `!=` `<` `<=` `>` `>=` | Response time in ms |
| `[CONNECTED]`     | `==` `!=`                    | TCP/DNS connected |
| `[BODY]`          | `==` `!=` `contains`         | Response body text |

## CLI reference

```
perrus-cli start [-port 8080] [-config config.yaml] [-daemon]
perrus-cli stop
perrus-cli status
perrus-cli check [-config config.yaml] [-json] <endpoint-name>
perrus-cli results [-config config.yaml] [-format table|json]
perrus-cli config-init [-out config.yaml]
perrus-cli config-validate [-config config.yaml]
perrus-cli version
```

`check` exits `0` (all conditions pass), `2` (condition failed), `1` (error). Designed for use in shell scripts, CI pipelines, and AI agent tool calls.

## REST API

| Endpoint | Description |
|----------|-------------|
| `GET /` | Web dashboard (dark-mode, grouped, auto-refresh 15s) |
| `GET /api/v1/endpoints/statuses` | All endpoint history (JSON) |
| `GET /api/v1/endpoints/{name}/statuses` | Single endpoint history |
| `GET /api/v1/health` | Liveness probe |

## Architecture

Built on [boilerplate-cli-ui-go](https://github.com/javimosch/boilerplate-cli-ui-go):

| File | Role |
|------|------|
| `main.go` | CLI dispatch — all commands, flags, exit codes |
| `daemon.go` | PID-file daemon management |
| `config.go` | YAML config loading + validation |
| `conditions.go` | Gatus-compatible condition evaluator |
| `checker.go` | HTTP / TCP / DNS probers |
| `monitor.go` | Per-endpoint goroutine scheduler |
| `store.go` | In-memory circular result buffer (RWMutex) |
| `server.go` | HTTP API + embedded dashboard |
| `dashboard.html` | Dark-mode SPA, groups, 20-dot history |

## Credits

This is a clean-room reimplementation of [Gatus](https://github.com/TwiN/gatus) by TwiN. The config format and condition syntax are intentionally compatible. No Gatus source code was used.
