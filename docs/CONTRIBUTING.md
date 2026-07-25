# Contributing

## Prerequisites

- Go 1.22+
- `just` (optional, for build commands)

## Build

```bash
just build
```

Or directly:

```bash
go build -ldflags "-s -w -X main.version=dev -X main.commit=$(git rev-parse --short HEAD) -X main.date=$(date -u +%Y-%m-%dT%H:%M:%SZ)" -o oc-vpn-go ./cmd/oc-vpn
```

## Test

```bash
just test
```

## Deploy locally

```bash
just deploy    # builds, tests, symlinks to /usr/local/bin
```

## Release

```bash
just bump 1.0.0   # tags v1.0.0 and pushes
```

CI runs goreleaser and creates a GitHub release with binaries.

## Project Layout

- `cmd/oc-vpn/` — CLI entry point
- `internal/` — Private Go packages (profiles, namespace, wireguard, doctor, tui)
- `docs/` — Architecture and contributing docs

## Conventions

- Standard Go project layout with `cmd/` and `internal/`
- Root required for namespace/WireGuard operations
- Profiles stored at `~/.config/oc-vpn/profiles/`
- ANSI escape codes for colored terminal output
