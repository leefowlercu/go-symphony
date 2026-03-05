---
tracker:
  kind: linear
  endpoint: https://api.linear.app/graphql
  api_key: $LINEAR_API_KEY
  project_slug: your-project-slug
  active_states: "Todo, In Progress"
  terminal_states: "Closed, Cancelled, Canceled, Duplicate, Done"

polling:
  interval_ms: 30000

workspace:
  root: $SYMPHONY_WORKSPACE_ROOT

hooks:
  after_create: |
    # Example: bootstrap repository into a fresh workspace
    # git clone https://github.com/your-org/your-repo.git .
    echo "workspace created"
  before_run: |
    # Example: install deps/build before each attempt
    # make setup
    echo "before run"
  after_run: |
    echo "after run"
  before_remove: |
    echo "before remove"
  timeout_ms: 60000

agent:
  max_concurrent_agents: 10
  max_turns: 20
  max_retry_backoff_ms: 300000
  max_concurrent_agents_by_state:
    todo: 3

codex:
  command: codex app-server
  approval_policy:
    reject:
      sandbox_approval: true
      rules: true
      mcp_elicitations: true
  thread_sandbox: workspace-write
  # If omitted, turn_sandbox_policy defaults to workspaceWrite rooted to the issue workspace.
  turn_timeout_ms: 3600000
  read_timeout_ms: 5000
  stall_timeout_ms: 300000

# Optional HTTP extension field (CLI --port overrides this)
server:
  port: 0
---

You are working on a Linear issue.

Issue: {{ issue.identifier }}
Title: {{ issue.title }}
State: {{ issue.state }}
Priority: {{ issue.priority }}
URL: {{ issue.url }}

Description:
{{ issue.description }}

Labels:
{% for label in issue.labels %}
- {{ label }}
{% endfor %}

Blockers:
{% for blocker in issue.blocked_by %}
- id={{ blocker.id }} identifier={{ blocker.identifier }} state={{ blocker.state }}
{% endfor %}

Attempt: {{ attempt }}

Execution expectations:
1. Work only within the current workspace.
2. Keep changes minimal and scoped to the issue.
3. Add or update tests when behavior changes.
4. Stop at a handoff-ready state and summarize what changed.
