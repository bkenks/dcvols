# dcvols

<p align="center">
  A small utility that reads your Docker Compose files and pre-creates bind mount directories before containers start — so you stop hitting permission errors on first boot.
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat-square&logo=go&logoColor=white" alt="Go version"/>
  <img src="https://img.shields.io/badge/Docker-Compose-2496ED?style=flat-square&logo=docker&logoColor=white" alt="Docker Compose"/>
</p>

---

## Why

When Docker Compose creates a bind mount directory that doesn't exist yet, it creates it owned by `root`. If your container runs as a non-root user, it can't write to it and fails on startup.

`dcvols` solves this by pre-creating the directories as your current user (or a specified UID/GID) before you bring the stack up.

---

## Installation

Requires Go and git. Builds from source and installs to `~/.local/bin/dcvols`:

```bash
curl -fsSL https://raw.githubusercontent.com/bkenks/dcvols/main/scripts/install.sh | sh
```

If `~/.local/bin` isn't on your `$PATH`, add this to your shell config (`~/.bashrc`, `~/.zshrc`, etc.):

```bash
export PATH="$HOME/.local/bin:$PATH"
```

### Uninstall

```bash
curl -fsSL https://raw.githubusercontent.com/bkenks/dcvols/main/scripts/uninstall.sh | sh
```

---

## Usage

```
dcvols [flags] [path]
```

Run from a directory containing a compose file, or pass a path as an argument.

| Flag        | Description                                         |
| ----------- | --------------------------------------------------- |
| `-r`        | Recursively search subdirectories for compose files |
| `--uid N`   | Chown created directories to this user ID           |
| `--gid N`   | Chown created directories to this group ID          |
| `--dry-run` | Print directories that would be created without creating them |

### Examples

```bash
# Create dirs for the compose file in the current directory
dcvols

# Recursively process all compose files under a directory
dcvols -r /opt/stacks

# Create dirs and chown to UID/GID 1000 (requires sudo if not root)
sudo dcvols -r --uid 1000 --gid 1000

# Preview what would be created without touching anything
dcvols --dry-run
```

---

## How It Works

1. Finds `compose.yaml`, `compose.yml`, `docker-compose.yaml`, or `docker-compose.yml`
2. Loads `.env` files by walking up the directory tree to the repo root — deeper files take precedence, so app-level `.env` values override root-level ones
3. Expands `${VAR}` references in volume paths using loaded env + shell environment
4. Filters to bind mounts only — named volumes (e.g. `pgdata`) are skipped
5. `mkdir -p`s each host path, optionally followed by `chown`

---

## Building from Source

```bash
git clone https://github.com/bkenks/dcvols.git
cd dcvols
go build -o dcvols .
```

To build release zips for Linux amd64 and arm64:

```bash
./scripts/build.sh
# Output: .builds/linux_amd64.zip, .builds/linux_arm64.zip
```
