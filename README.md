# Nimbus

Nimbus is a long-running orchestration service that connects a project tracker (Linear) to autonomous coding agents. It polls for issues, creates isolated workspaces, dispatches [Codex](https://openai.com/index/introducing-codex/) agents to work on them, and manages the full lifecycle — retries, stall detection, workspace cleanup, and graceful shutdown.

A built-in dashboard provides real-time visibility into running sessions, retry queues, token usage, and persona assignments.

## How It Works

```
Linear Issues ──► Nimbus Orchestrator ──► Codex Agents (one per issue)
                        │                       │
                   poll + reconcile         JSON-RPC stdio
                        │                       │
                   Dashboard ◄──────── events + tokens
```

1. **Poll:** Nimbus polls Linear for issues in configured active states (e.g., "Todo", "In Progress").
2. **Dispatch:** Eligible issues are sorted by priority and dispatched to available agent slots. Each agent runs in an isolated workspace directory.
3. **Execute:** The agent starts a Codex app-server subprocess, renders a prompt from a Liquid template, and runs multi-turn conversations until the work is done or max turns are reached.
4. **Reconcile:** Running sessions are continuously checked for stalls and tracker state changes. Terminal issues trigger workspace cleanup. Stalled sessions are killed and retried.
5. **Retry:** Normal exits schedule a 1-second continuation retry. Failures use exponential backoff up to a configurable maximum.

## Personas

Nimbus supports **personas** — named agent identities (e.g., `backend-engineer`, `qa-tester`, `devops`) that customize how an agent approaches work. Each persona is a markdown file with YAML front matter defining config overrides and a prompt template.

Personas are assigned to issues via Linear labels (`persona:backend-engineer`) and can override max turns, approval policy, model, turn timeout, and tool visibility. The persona prompt is composed with the workflow prompt, not replaced.

Personas can be managed through the dashboard UI or the REST API.

## Quick Start

### Prerequisites

- Go 1.25+
- Node.js 20+
- A Linear API key
- Codex CLI installed (`codex app-server` available in PATH)

### Setup

```bash
# Install dependencies and build
make build
```

### Configure

Create a `WORKFLOW.md` file:

```yaml
---
tracker:
  kind: linear
  api_key: $LINEAR_API_KEY
  project_slug: my-project
workspace:
  root: ~/nimbus-workspaces
agent:
  max_concurrent_agents: 5
  max_turns: 10
codex:
  command: codex app-server
  approval_policy: never
server:
  port: 8080
hooks:
  after_create: git clone git@github.com:org/repo.git .
  before_run: git checkout main && git pull
---

You are working on issue {{ issue.identifier }}: {{ issue.title }}.

{{ issue.description }}

Work in the repository. Create a branch, implement the changes, commit, and push.
```

### Run

```bash
# Set your Linear API key
export LINEAR_API_KEY="lin_api_..."

# Start Nimbus
./bin/nimbus --port 8080 WORKFLOW.md
```

Open `http://localhost:8080` for the dashboard.

## CLI

```
nimbus [OPTIONS] [path-to-WORKFLOW.md]
```

| Flag | Default | Description |
|---|---|---|
| `--port` | config value | HTTP server port (overrides `server.port`) |
| `--logs-root` | *(stderr)* | Directory for JSON log files |
| *(positional)* | `WORKFLOW.md` | Path to workflow configuration file |

## API

| Endpoint | Method | Description |
|---|---|---|
| `/api/v1/state` | GET | Full orchestrator snapshot |
| `/api/v1/{issue_identifier}` | GET | Single issue detail |
| `/api/v1/refresh` | POST | Trigger immediate poll cycle |
| `/api/v1/personas` | GET | List all personas |
| `/api/v1/personas` | POST | Create a persona |
| `/api/v1/personas/{name}` | GET / PUT / DELETE | Persona CRUD |
| `/api/v1/personas/assignments` | GET | List persona-to-issue assignments |

## Development

```bash
# Run Go backend in dev mode
make dev ARGS="--port 8080 WORKFLOW.md"

# Run frontend dev server (proxies API to :8080)
cd web && npm run dev

# Run tests
make test

# Lint
make vet
```

## Architecture

The core is a **single-goroutine orchestrator** that serializes all state mutations through a `select` loop over typed channels — no mutexes on shared state. Worker agents run as separate goroutines, communicating back via channels for events, token updates, and exit signals.

See [CLAUDE.md](CLAUDE.md) for detailed implementation documentation and [SPEC.md](SPEC.md) for the full specification.

## License

See the LICENSE file for details.
