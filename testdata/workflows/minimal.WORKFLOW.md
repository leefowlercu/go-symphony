---
tracker:
  kind: linear
  project_slug: demo
polling:
  interval_ms: 30000
workspace:
  root: ./tmp/symphony_workspaces
agent:
  max_concurrent_agents: 10
  max_turns: 20
codex:
  command: codex app-server
---

You are working on issue {{ issue.identifier }}.

Title: {{ issue.title }}
State: {{ issue.state }}
