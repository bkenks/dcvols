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

All logic lives in `main.go` (~260 lines). Key functions:

- `findComposeFiles()` — locates compose files (`compose.yaml`, `compose.yml`, `docker-compose.yaml`, `docker-compose.yml`)
- `processComposeFile()` — orchestrates: load env → expand vars → parse YAML → create dirs → chown
- `extractBindMountDirs()` — filters bind mounts from the parsed volumes list
- `loadEnv()` — walks up the directory tree to git root, merging `.env` files (deeper overrides parent)
- `chownTree()` + `firstMissingAncestor()` — efficiently chowns only newly created dirs

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
