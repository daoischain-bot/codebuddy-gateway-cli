# CodeBuddy CLI Gateway

## Overview
Go + Bubbletea TUI gateway for CodeBuddy Global API. Single binary, zero runtime deps.
- **Stack**: Go 1.23+, Bubbletea v1, Lipgloss
- **Port**: 2069
- **Binary**: `/usr/local/bin/buddy`

## Commands
```bash
# Setup
go mod tidy

# Build
go build -o /usr/local/bin/buddy .

# Test
go test -v ./...

# Lint
go vet ./...

# Run
buddy
```

## Conventions
- `internal/` packages: `config/`, `router/`, `tui/`
- Model registry in `internal/router/models.go` — `map[string]Model`
- All model IDs verified live against CodeBuddy API
- TUI: tea.Batch(tea.ClearScreen, drain(ch)) for resize handling
- Non-blocking channel sends: `select { case ch <- x: default: }`

## Boundaries
- **NEVER** commit `.env` with real keys
- **NEVER** hardcode API keys in code
- **NEVER** remove `chmod +x` from install instructions
- **ALWAYS** backup before refactor: `~/backup/<project>-backup-<ts>/`

## Config
| Var | Description |
|-----|-------------|
| `PORT` | Listening port (default: 2069) |
| `API_KEYS` | Comma-separated `ck_...` keys |
| `PROXY` | HTTP/HTTPS/SOCKS5 proxy URL |
| RouterKey | Auto-generated `rtr_` from SHA256(hostname+machine-id) |

## Troubleshooting
- `"service info not found"` — model ID not deployed to CodeBuddy backend yet
- `"all keys exhausted"` — API keys expired or exhausted
- `"unauthorized: invalid router key"` — wrong `rtr_` key
- TUI stuck at "Loading..." — run with terminal width ≥ 30 cols
