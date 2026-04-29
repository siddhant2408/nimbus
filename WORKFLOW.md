---
tracker:
  kind: linear
  api_key: $LINEAR_API_KEY        # or paste key directly (not recommended)
  project_slug: nimbus        # the slugId of your Linear project
  # assignee: me                  # optional: only pick up issues assigned to you

polling:
  interval_ms: 30000              # how often to check Linear for new issues (30s)

workspace:
  root: ~/nimbus-workspaces       # where per-issue directories are created

agent:
  max_concurrent_agents: 3        # how many issues to work on simultaneously
  max_turns: 20                   # max LLM turns per issue before giving up
  max_retry_backoff_ms: 300000    # cap on exponential retry delay (5 min)

codex:
  command: codex app-server
  approval_policy: never          # auto-approve all tool calls
  turn_timeout_ms: 3600000        # 1 hour per turn before stall detection

server:
  port: 8080                      # dashboard at http://localhost:8080

personas:
  directory: ./personas           # optional: load persona definitions from here
  label_prefix: persona           # issues labelled "persona:backend" get that persona
---

You are working on issue {{ issue.identifier }}: {{ issue.title }}.

{% if issue.description %}
## Description

{{ issue.description }}
{% endif %}

## Instructions

1. Read the issue carefully and understand what is being asked.
2. Create a branch named after the issue: `git checkout -b {{ issue.branch_name }}`.
3. Implement the changes. Write clean, idiomatic code that fits the existing style.
4. Commit your work with a descriptive message referencing the issue identifier.
5. Push the branch and open a pull request.

If you are blocked or need clarification, leave a comment on the Linear issue explaining what you need.
