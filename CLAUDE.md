# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build

```bash
go build -o dcvols .
```

Cross-platform release builds (outputs to `.builds/`):

```bash
./scripts/build.sh
```

## Project Overview

`dcvols` is a Go CLI tool that pre-creates bind mount directories for Docker Compose before Docker does, so they're owned by the current user rather than `root`. It finds compose files, loads `.env` files (walking up to the git root), expands `${VAR}` references in volume paths, extracts bind mounts, creates missing directories with `mkdir -p`, and optionally `chown`s them.

## Architecture

`main.go` stays at the repo root (the build scripts run `go build -o dcvols .`,
so the `main` package must live here). It is intentionally thin — flag parsing
plus orchestration — and delegates the real work to three `internal/` packages,
one per concern:

```
main.go                     # CLI: parse flags, orchestrate, print output
internal/
  compose/                  # the Docker Compose domain
    discover.go             #   Find()        — locate compose files
    parse.go                #   Parse(), File/Service/VolumeEntry types
    mounts.go               #   File.BindMounts() — extract bind-mount host paths
  envfile/
    envfile.go              #   Load()  — merge .env files up to git root
                            #   Expand() — expand ${VAR}/$VAR in compose bytes
  fsops/
    fsops.go                #   EnsureDir()/EnsureFile() + chown of new subtrees
```

Orchestration flow in `main.go` (`run` → `processComposeFile` → `create`):
`compose.Find` → for each file: `envfile.Load` → `envfile.Expand` →
`compose.Parse` → `File.BindMounts` → `fsops.EnsureDir`/`EnsureFile` (which
returns an `fsops.Outcome` the orchestrator turns into the `created`/`chowned`
output lines).

The packages do no printing themselves — `fsops` reports what it did via
`Outcome` and `main.go` owns all stdout/stderr — which keeps the internal
packages reusable and testable. Every exported symbol has a doc comment; run
`go doc ./internal/...` to read them.

## Dependencies

- `github.com/joho/godotenv` — `.env` file parsing
- `gopkg.in/yaml.v3` — Docker Compose YAML parsing

## CLI Flags

```
dcvols [flags] [path]
  -r          Recursively search for compose files
  --uid N     Chown created dirs to this UID
  --gid N     Chown created dirs to this GID
  --dry-run   Preview without creating
```

## Notes

- No tests or linter configuration exist in this repo.
- The compiled binary is gitignored; do not commit it.
- **Known bug (`.env` precedence):** `envfile.Load` currently merges root-level
  `.env` values *last*, so they override deeper, compose-adjacent `.env` files —
  the opposite of what the README and "How It Works" describe ("deeper files
  take precedence"). The fix is one line (reverse the merge loop); see the
  `BUG(envfile)` note in `internal/envfile/envfile.go`. Left as-is for now so the
  package split stayed behavior-neutral.
