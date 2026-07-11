# ALLinker — Cross-Agent Collaboration Gateway

> A unified collaboration entry point for different AI Agent software, enabling cross-agent collaborative work.

![Go Version](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go)
![License](https://img.shields.io/badge/License-Apache%202.0-green)
![Platform](https://img.shields.io/badge/platform-Windows%20|%20Linux%20|%20macOS-lightgrey)

[简体中文](docs/README.zh-CN.md) · [日本語](docs/README.ja.md) · [한국어](docs/README.ko.md) · [Français](docs/README.fr.md)

---

## Overview

ALLinker is a **CLI-based collaboration gateway** designed for multiple AI Agent tools (such as Cline, CodeX, custom agents, etc.) working in the same project directory.

When multiple agents operate in the same project independently, they commonly face:

- **File conflicts** — Multiple agents editing the same file simultaneously
- **Information silos** — No direct communication between agents
- **Untraceable operations** — Impossible to audit who did what and when

ALLinker solves these with **four collaboration primitives**:

| Primitive | What It Solves |
|-----------|----------------|
| **File Locking** | Agents acquire a lock before editing a file to prevent conflicts |
| **Messaging** | Agents send messages to each other, with `@` mentions |
| **File Watching** | Agents register watchpoints to track progress of peers |
| **Account Management** | Identity signing + 3-level permissions + full audit trail |

---

## Quick Start

### Build

```bash
git clone <repo-url>
cd ALLinker
go build -o allinker.exe .
```

Pre-built binaries are also available for Windows (x64/x86), Linux (x64/ARM64), and macOS (Intel/ARM).

### Register Agents

```bash
./allinker register --name TRAE --role agent
./allinker register --name CodeX --role agent
./allinker register --name admin --role admin
```

### File Locking

```bash
./allinker lock -f PLAN_001.md -t 30 --user TRAE    # Blocking lock (max 30s)
./allinker tryLock -f PLAN_001.md --user TRAE        # Non-blocking attempt
./allinker unlock -f PLAN_001.md --user TRAE         # Release lock
./allinker status -f PLAN_001.md                     # Check lock status
./allinker status --all                              # List all locks
```

### Messaging

```bash
./allinker send --at CodeX --msg "Please implement the auth module" --user TRAE
./allinker send --at All --msg "Broadcast message" --user TRAE
./allinker recv                                                   # Receive messages
./allinker history --with CodeX --limit 10                        # View history
```

### File Watching — Wait for Peer Response

Agent A asks Agent B to complete a task. Agent A sets up a watchpoint to detect when B's response file appears:

```bash
# Agent A: Register a watchpoint for the expected response file
./allinker watch add --name "resp-auth-module" -d ./CodeX -p "RESP_*.md" --user TRAE

# Agent A: Block until the file appears (or timeout after 300s)
./allinker wait -d ./CodeX -f "RESP_*.md" -t 300

# Agent A: Check if the response has arrived
./allinker watch check --name "resp-auth-module"

# List all active watchpoints
./allinker watch list

# Remove a watchpoint when done
./allinker watch remove --name "resp-auth-module"
```

---

## Server Mode — Cross-Host LAN Collaboration

ALLinker can run as a long-lived HTTP service, allowing agents across **different hosts on the same LAN** to call it over the network. This is the core mechanism for multi-machine team collaboration.

```bash
# Start the server
./allinker -server --port 8080

# Client mode (connect to a remote server)
./allinker --connect http://127.0.0.1:8080 lock -f PLAN_001.md --user TRAE

# Auto mode: use the network if a server is available, otherwise execute locally
./allinker --auto send --at CodeX --msg "hi" --user TRAE

# Server management
./allinker -server --stop
./allinker -server --status
```

### HTTP API

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/v1/command` | POST | Execute a command remotely |
| `/api/v1/health` | GET | Health check |
| `/api/v1/status` | GET | Service status |

---

## Build for All Platforms

On Windows, run the included build script to produce cross-platform binaries:

```bat
build.bat
```

This generates:

| Binary | Platform |
|--------|----------|
| `ALLinker_windows_amd64.exe` | Windows x64 |
| `ALLinker_windows_386.exe` | Windows x86 |
| `ALLinker_linux_amd64` | Linux x64 |
| `ALLinker_linux_arm64` | Linux ARM64 |
| `ALLinker_darwin_amd64` | macOS Intel |
| `ALLinker_darwin_arm64` | macOS Apple Silicon |

---

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | General error |
| 2 | Timeout (wait) |
| 3 | Lock acquisition failed (tryLock) |
| 4 | Account does not exist |
| 5 | Insufficient permissions |
| 6 | File does not exist |

---

## Data Storage

All data is stored in the `.alf/` directory (configurable via `--data-dir`):

```
.alf/
├── users.json        # User accounts
├── config.json       # Tool configuration
├── counter.json      # ID counter
├── watchlist.json    # Watchpoint registry
├── allinker.db       # SQLite database (messages + locks + watchpoints)
└── Logs/             # Log files (daily rotation: YYYY-MM-DD.log)
```

Write operations use **atomic writes** (temp file → rename) to prevent data corruption.

---

## Project Structure

```
.
├── main.go        # Entry point
├── go.mod
├── build.bat      # Cross-platform build script
├── account/       # Account management
├── cli/           # CLI command routing
├── config/        # Configuration management
├── core/          # Global singletons
├── init/          # Data directory & database initialization
├── lock/          # File locking
├── logutil/       # Logging & audit
├── message/       # Messaging
├── model/         # Data models
├── storage/       # JSON persistence
├── wait/          # Blocking file wait
└── watch/         # File watching
```

---

## License

[Apache License 2.0](LICENCE)

---

*Want to contribute? See the [Contributing Guide](docs/CONTRIBUTING.md).*
