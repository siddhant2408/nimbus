# Symphony — Go + TypeScript Implementation

Symphony is a long-running orchestration service that polls a Linear project tracker for issues, creates isolated workspaces, and dispatches Codex coding agents to work on them autonomously. This directory contains the complete Go backend + TypeScript frontend implementation, faithful to `SPEC.md` (~2975 lines).

## Quick Reference

```bash
# Build (frontend + Go binary with embedded assets)
make build

# Dev mode (Go only, no frontend embed)
make dev ARGS="--port 8080 WORKFLOW.md"

# Run tests
make test

# Type-check + lint
make vet
cd web && npx tsc --noEmit

# Clean all artifacts
make clean
```

**Binary:** `./bin/symphony [--port PORT] [--logs-root DIR] [path-to-WORKFLOW.md]`

Default workflow path is `./WORKFLOW.md`. If `--port` is set (or `server.port` is in config), the dashboard is served at that port.

## Architecture

**Core design principle:** Single-goroutine orchestrator consuming typed channels — the Go equivalent of Elixir's GenServer. All state mutations are serialized through one goroutine's `select` loop. No mutexes on orchestrator state.

```
WORKFLOW.md ──► Config Watcher ──► Orchestrator (single goroutine)
                                      │
                  ┌───────────────────┼───────────────────┐
                  ▼                   ▼                   ▼
            Linear Client      Workspace Manager    Agent Runner (goroutines)
            (GraphQL poll)     (dir lifecycle)           │
                                                         ▼
                                                   Codex App-Server
                                                   (JSON-RPC stdio)
```

Events flow into the orchestrator via typed channels:

| Channel | Event Type | Source |
|---|---|---|
| `WorkerExitCh` | `WorkerExitEvent` | Agent goroutine completion |
| `CodexUpdateCh` | `CodexUpdateEvent` | Codex session stream events |
| `RuntimeInfoCh` | `WorkerRuntimeInfoEvent` | Worker startup (path, host, persona) |
| `RetryTimerCh` | `RetryTimerEvent` | `time.AfterFunc` retry timers |
| `SnapshotCh` | `SnapshotRequest` | HTTP API `GET /api/v1/state` |
| `RefreshCh` | `RefreshRequest` | HTTP API `POST /api/v1/refresh` |

## Project Structure

```
go/
├── cmd/symphony/main.go              # CLI entry, wiring, signal handling
├── internal/
│   ├── domain/                        # Core types (no business logic)
│   │   ├── issue.go                   #   Issue, BlockerRef
│   │   ├── session.go                 #   LiveSession, RunningEntry, RetryEntry, CodexTotals
│   │   ├── events.go                  #   Channel event types, Snapshot, RefreshResponse
│   │   └── workspace.go              #   WorkspaceResult
│   ├── config/                        # WORKFLOW.md parsing + hot reload
│   │   ├── schema.go                  #   Config structs, DefaultConfig()
│   │   ├── loader.go                  #   YAML front matter parser, env/path resolution
│   │   ├── validate.go                #   ValidateForDispatch (pre-poll checks)
│   │   └── watcher.go                 #   fsnotify + polling fallback, sha256 change detection
│   ├── tracker/
│   │   ├── tracker.go                 #   Tracker interface
│   │   └── linear/
│   │       ├── client.go              #   GraphQL client, pagination (page=50), 30s timeout
│   │       └── normalize.go           #   Issue normalization (state.name, labels, blockers)
│   ├── workspace/
│   │   ├── manager.go                 #   CreateForIssue, Remove, hook lifecycle
│   │   ├── safety.go                  #   SafeIdentifier, ValidateWorkspacePath (symlink-safe)
│   │   └── hooks.go                   #   sh -lc execution with timeout, 2048B output cap
│   ├── codex/
│   │   ├── appserver.go               #   Session: StartSession, RunTurn, Stop, streamLoop
│   │   ├── protocol.go                #   protoWriter/protoReader, 10MB max line, JSON-RPC types
│   │   ├── approval.go                #   Auto-approve handlers for all approval request types
│   │   ├── tools.go                   #   ToolSpec, toolDispatcher, linear_graphql handler
│   │   └── tokens.go                  #   tokenTracker, delta computation, rate limit extraction
│   ├── agent/
│   │   ├── runner.go                  #   Per-issue goroutine: workspace → hooks → turn loop
│   │   └── prompt.go                  #   Liquid templates, persona+workflow composition
│   ├── persona/
│   │   ├── persona.go                 #   Persona struct, MergeConfig, FilterTools
│   │   ├── registry.go                #   Load .md files, CRUD, YAML front matter, validation
│   │   └── assignment.go              #   Label-based resolution, local JSON persistence
│   ├── orchestrator/
│   │   ├── orchestrator.go            #   Event loop (Run), channel select, buildSnapshot
│   │   ├── dispatch.go                #   Eligibility checks, goroutine spawn, sort by priority
│   │   ├── reconcile.go               #   Stall detection, tracker state refresh, termination
│   │   ├── retry.go                   #   Exponential backoff: min(10000*2^(n-1), max)
│   │   └── tokens.go                  #   applyTokenDelta, accumulateGlobalTokens
│   └── server/
│       ├── server.go                  #   chi router, API handlers, error envelope
│       └── frontend.go                #   Embedded SPA serving with index.html fallback
├── web/                               # TypeScript frontend
│   ├── embed.go                       #   //go:embed all:dist
│   ├── package.json                   #   React 18, TanStack Query, react-router-dom, Tailwind v4
│   ├── vite.config.ts                 #   Dev proxy /api → localhost:8080
│   ├── tsconfig.json                  #   Strict mode, path aliases
│   ├── index.html
│   └── src/
│       ├── main.tsx                   #   React root, QueryClientProvider, BrowserRouter
│       ├── App.tsx                    #   Router: / (Dashboard), /personas (PersonaManager)
│       ├── app.css                    #   Tailwind v4 import
│       ├── types/api.ts               #   Full typed interfaces matching Go JSON responses
│       ├── api/client.ts              #   Typed fetch wrappers for all endpoints
│       ├── hooks/useQueries.ts        #   TanStack Query hooks (2s auto-refresh), mutations
│       └── components/
│           ├── Dashboard.tsx          #   Metrics + RunningTable + RetryTable + rate limits
│           ├── MetricsCards.tsx        #   4-card grid (running, retrying, tokens, runtime)
│           ├── RunningTable.tsx        #   Live sessions with persona, turns, events, tokens
│           ├── RetryTable.tsx          #   Retry queue with countdown timers
│           └── PersonaManager.tsx      #   Full CRUD, overrides JSON editor, assignments
├── go.mod
├── go.sum
└── Makefile
```

**Total:** 33 Go files (5,960 lines) + 11 TypeScript/CSS files (1,304 lines). Binary: 14MB with embedded frontend.

## Go Dependencies

| Package | Purpose |
|---|---|
| `gopkg.in/yaml.v3` | WORKFLOW.md and persona front matter parsing |
| `github.com/osteele/liquid` | Liquid template rendering for prompts |
| `github.com/fsnotify/fsnotify` | File system watching for config hot reload |
| `github.com/go-chi/chi/v5` | HTTP router with middleware |
| `log/slog` (stdlib) | Structured logging (text to stderr, JSON to file) |

## Frontend Stack

| Package | Purpose |
|---|---|
| React 18 | UI framework |
| TanStack Query v5 | Server state management, 2-second auto-refresh polling |
| react-router-dom v6 | Client-side SPA routing |
| Tailwind CSS v4 | Utility-first styling (via `@tailwindcss/vite` plugin) |
| Vite 6 | Build tool, dev server with API proxy |
| TypeScript 5 (strict) | Type safety |

## Key Subsystems

### Config (`internal/config/`)

WORKFLOW.md is a markdown file with YAML front matter between `---` fences. The YAML section maps to `config.Config`; everything after the closing `---` is the Liquid prompt template.

- **Hot reload:** `fsnotify` watches the file + 5-second polling fallback. Changes are atomically swapped via `atomic.Pointer[WorkflowDefinition]`. SHA-256 dedup prevents redundant reloads.
- **Environment variables:** `$VAR_NAME` in `api_key` fields are resolved to `os.Getenv`.
- **Path resolution:** `~` expands to home dir; relative paths resolve against the WORKFLOW.md directory.
- **Validation:** `ValidateForDispatch` checks required fields before every poll cycle.

### Linear Client (`internal/tracker/linear/`)

Implements `tracker.Tracker` interface. Two primary GraphQL queries:

- **`SymphonyLinearPoll`:** Cursor-paginated (page_size=50) fetch of issues by project slug + active states. Extracts `state.name`, lowercased labels, blockers from `inverseRelations` where `type=="blocks"`.
- **`SymphonyLinearIssuesById`:** Batch fetch by IDs (batched in groups of 50) for reconciliation state refresh.

The `GraphQL()` method is exported and reused by the `linear_graphql` dynamic tool in the Codex session.

### Workspace Manager (`internal/workspace/`)

- **SafeIdentifier:** Replaces `[^A-Za-z0-9._-]` with `_` for directory names.
- **Path containment:** Segment-by-segment symlink resolution to ensure workspace stays under root.
- **Hooks:** Executed via `sh -lc <script>` with configurable timeout. Output truncated at 2048 bytes.
  - `after_create`: fatal on failure (workspace removed)
  - `before_run`: fatal on failure (attempt aborted)
  - `after_run` / `before_remove`: failures logged and ignored

### Codex App-Server (`internal/codex/`)

Manages a subprocess communicating via line-delimited JSON-RPC 2.0 over stdin/stdout.

**Protocol sequence:**
1. Launch subprocess: `bash -lc <codex.command>` with `Dir=workspace`
2. `initialize` (id=1) → await response → `initialized` notification
3. `thread/start` (id=2) → await response → extract `thread.id`
4. Per-turn: `turn/start` (id=3) → stream loop until `turn/completed` / `turn/failed` / `turn/cancelled`

**Stream loop handles:**
- Approval requests → auto-approve (`acceptForSession`, `approved_for_session`)
- Tool calls → dispatch to registered handlers (e.g., `linear_graphql`)
- User input requests → find "Approve this Session" option or return non-interactive message
- Token usage extraction from nested payload paths, delta computation
- Rate limit extraction

**Key constants:** 10MB max line size, configurable read timeout (default 5s), turn timeout (default 1h).

### Agent Runner (`internal/agent/`)

Per-issue goroutine lifecycle:
1. Resolve persona (label-based) → compute effective config with overrides
2. Create/reuse workspace → run `before_run` hook
3. Start Codex session with tool specs (filtered by persona)
4. **Turn loop:** Turn 1 uses full Liquid-rendered prompt (persona + workflow composed); turns 2+ use continuation prompt
5. After each turn: check context cancellation (orchestrator kills context on terminal state)
6. Stop session → run `after_run` hook

**Prompt composition:** `persona_prompt + "\n\n---\n\n" + workflow_prompt` (SPEC B.7).

### Orchestrator (`internal/orchestrator/`)

Single goroutine consuming a `select` over 7 channels + a ticker.

**Tick cycle:**
1. **Reconcile** running issues (stall detection + tracker state refresh)
2. **Validate** config
3. **Fetch** candidate issues from Linear
4. **Sort** by priority (asc) → created_at (oldest first) → identifier (lexicographic)
5. **Dispatch** while global + per-state slots available

**Dispatch eligibility:** has required fields, active state, not terminal, not claimed, not running, not blocked (Todo + non-terminal blockers), global slot available, per-state slot available.

**Worker exit handling:**
- Normal exit → continuation retry (1s delay, attempt=1)
- Abnormal exit → exponential backoff: `min(10000 * 2^(attempt-1), max_retry_backoff_ms)`

**Reconciliation:**
- Stall detection: `elapsed > stall_timeout_ms` → kill + retry
- Tracker refresh: terminal → terminate + workspace cleanup; active → update snapshot; other → terminate without cleanup

**Startup:** Fetches terminal-state issues and removes their workspaces.

### Persona Extension (`internal/persona/`)

Implements SPEC Appendix B.

- **Definition format:** Markdown files with YAML front matter (`name`, `description`, `overrides`) and body as the persona prompt template.
- **Registry:** Scans a directory for `.md` files. Name must match filename stem. Validates `[a-z0-9-]` format and no simultaneous allow+deny tool lists. Supports full CRUD (writes back to `.md` files).
- **Assignment resolution (B.5.3):**
  1. Check issue labels for `persona:<name>` prefix → resolve against registry
  2. Fall back to local JSON persistence (`<workspace_root>/.symphony/persona_assignments.json`)
  3. No persona = backward-compatible (no overrides, no persona prompt)
- **Config merge (B.6):** Shallow overlay — persona overrides `max_turns`, `approval_policy`, `model`, `turn_timeout_ms`.
- **Tool filtering (B.12.5):** Allowlist mode (only named tools) or denylist mode (all except named).

### HTTP Server (`internal/server/`)

chi router with:

| Endpoint | Method | Purpose |
|---|---|---|
| `/api/v1/state` | GET | Full orchestrator snapshot |
| `/api/v1/{issue_identifier}` | GET | Single issue detail (running or retrying) |
| `/api/v1/refresh` | POST | Trigger immediate poll, returns 202 |
| `/api/v1/personas` | GET | List all personas |
| `/api/v1/personas` | POST | Create persona |
| `/api/v1/personas/{name}` | GET | Get persona by name |
| `/api/v1/personas/{name}` | PUT | Update persona |
| `/api/v1/personas/{name}` | DELETE | Delete persona |
| `/api/v1/personas/assignments` | GET | List all persona assignments |
| `/*` | GET | Embedded SPA (or fallback HTML) |

Error envelope: `{"error": {"code": "...", "message": "..."}}`.

### Frontend (`web/`)

Two pages:
- **Dashboard (`/`):** 4 metric cards (running, retrying, total tokens, runtime), running sessions table with persona/turns/events/tokens, retry queue table, rate limits JSON panel, refresh button with last-update timestamp.
- **Personas (`/personas`):** Persona list with create/edit/delete, form with name validation + description + prompt textarea + overrides JSON editor, assignments table.

Auto-refresh via TanStack Query `refetchInterval: 2000`. Dark theme (`bg-gray-950`). Responsive grid layouts. Vite dev server proxies `/api` to `localhost:8080`.

## Config Defaults (SPEC Section 6.4)

| Key | Default |
|---|---|
| `tracker.endpoint` | `https://api.linear.app/graphql` |
| `tracker.active_states` | `["Todo", "In Progress"]` |
| `tracker.terminal_states` | `["Closed", "Cancelled", "Canceled", "Duplicate", "Done"]` |
| `polling.interval_ms` | `30000` |
| `agent.max_concurrent_agents` | `10` |
| `agent.max_turns` | `20` |
| `agent.max_retry_backoff_ms` | `300000` (5 min) |
| `codex.command` | `codex app-server` |
| `codex.thread_sandbox` | `workspace-write` |
| `codex.turn_timeout_ms` | `3600000` (1 hour) |
| `codex.read_timeout_ms` | `5000` |
| `codex.stall_timeout_ms` | `300000` (5 min) |
| `hooks.timeout_ms` | `60000` |
| `server.host` | `127.0.0.1` |
| `personas.directory` | `personas` |
| `personas.label_prefix` | `persona` |

## WORKFLOW.md Example

```yaml
---
tracker:
  kind: linear
  api_key: $LINEAR_API_KEY
  project_slug: my-project
polling:
  interval_ms: 15000
workspace:
  root: ~/symphony-workspaces
agent:
  max_concurrent_agents: 5
  max_turns: 10
codex:
  command: codex app-server
  approval_policy: never
server:
  port: 8080
personas:
  directory: ./personas
  label_prefix: persona
hooks:
  after_create: git clone git@github.com:org/repo.git .
  before_run: git checkout main && git pull
---

You are working on issue {{ issue.identifier }}: {{ issue.title }}.

{{ issue.description }}

Work in the repository cloned to your workspace. Create a branch, implement the changes, commit, and push.
```

## Common Development Tasks

**Adding a new API endpoint:** Add the route in `internal/server/server.go` inside the `/api/v1` route group. Follow the existing pattern of handler methods on `*Server`.

**Adding a new config field:** Add the field to the appropriate struct in `config/schema.go`, set its default in `DefaultConfig()`, parse it in `loader.go`'s `parseConfig()`, and optionally validate it in `validate.go`.

**Adding a new tracker backend:** Implement the `tracker.Tracker` interface (3 methods) in a new package under `internal/tracker/`. Wire it in `main.go` based on `cfg.Tracker.Kind`.

**Adding persona overrides:** Add the field to the appropriate `*Overrides` struct in `persona/persona.go`, handle the merge in `MergeConfig()`, and parse it in `registry.go`'s `parseOverrides()`.

**Frontend development:** Run `cd web && npm run dev` for Vite dev server (auto-proxies API to `:8080`). Run the Go backend separately with `make dev ARGS="--port 8080 WORKFLOW.md"`.
