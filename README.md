# Go Symphony

**Badges**: ![Build](https://img.shields.io/badge/build-N%2FA-lightgrey) ![Coverage](https://img.shields.io/badge/coverage-N%2FA-lightgrey) ![License](https://img.shields.io/badge/license-N%2FA-lightgrey) ![Go](https://img.shields.io/badge/go-1.26%2B-00ADD8?logo=go)

**Project Tagline**: A Go orchestration service for running Codex agent sessions against Linear issue queues.

**Current Version**: v0.1.0

## Table of Contents

- [Overview](#overview)
- [Quick Start](#quick-start)
- [Workflow Configuration](#workflow-configuration)
- [HTTP Observability](#http-observability)
- [Testing](#testing)
- [Safety Model](#safety-model)
- [Repository Layout](#repository-layout)

## Overview

`go-symphony` is a Go implementation of the Symphony service model: poll an issue tracker, claim eligible issues, run coding-agent sessions in isolated per-issue workspaces, and continuously reconcile retries and state transitions.

High-level capabilities in this repository:

- Loads runtime policy and prompt template from repository-owned `WORKFLOW.md`.
- Uses typed configuration with defaults, env indirection (`$VAR`), and runtime reload.
- Integrates with Linear GraphQL for candidate fetch, state refresh, and terminal cleanup sweeps.
- Runs Codex app-server sessions over stdio and forwards runtime telemetry into orchestrator state.
- Supports optional HTTP observability endpoints and an optional `linear_graphql` dynamic tool.

## Quick Start

1. Set required environment variables.

```bash
export LINEAR_API_KEY="<your-linear-token>"
export SYMPHONY_WORKSPACE_ROOT="$HOME/symphony-workspaces"
```

2. Create a workflow file from the provided full example.

```bash
cp ./testdata/workflows/full.WORKFLOW.md ./WORKFLOW.md
```

3. Edit `./WORKFLOW.md` and set at least:

- `tracker.project_slug`
- any repository bootstrap hooks you need (for example `hooks.after_create`)
- optional Codex policy overrides

4. Run tests.

```bash
go test ./...
```

5. Start the service.

```bash
mkdir -p ./bin
go build -o ./bin/symphony .
./bin/symphony run ./WORKFLOW.md --port 7777
```

6. Check live state.

```bash
curl http://127.0.0.1:7777/api/v1/state
```

## Workflow Configuration

Workflow settings and prompt template are sourced from `WORKFLOW.md`.

- Front matter: runtime config (`tracker`, `polling`, `workspace`, `hooks`, `agent`, `codex`, optional `server`).
- Markdown body: strict prompt template rendered with `issue` and `attempt` variables.
- Missing/invalid workflow content blocks dispatch and surfaces operator-visible errors.

See examples in `testdata/workflows/`:

- `minimal.WORKFLOW.md`
- `full.WORKFLOW.md`

## HTTP Observability

When enabled via `--port` (or `server.port` in workflow), the service binds to loopback and exposes:

- `GET /`
- `GET /api/v1/state`
- `GET /api/v1/{issue_identifier}`
- `POST /api/v1/refresh`

CLI `--port` takes precedence over `server.port`, including explicit `--port 0` to disable HTTP.

## Testing

This repository includes unit tests and acceptance tests:

- Unit tests: package-level behavior across config, workflow parsing, tracker normalization, prompt rendering, orchestration, workspace, and HTTP handlers.
- Acceptance tests: `godog` scenarios under `acceptance/features`.

Run all tests with:

```bash
go test ./...
```

## Safety Model

Default runtime posture is safer-by-default unless overridden in workflow:

- restrictive approval policy object under `codex.approval_policy`
- `codex.thread_sandbox: workspace-write`
- workspace-write turn sandbox policy scoped to the issue workspace

Baseline safety invariants:

- agent subprocess working directory is the issue workspace
- workspace keys are sanitized (`[A-Za-z0-9._-]`)
- workspace paths are validated to remain under configured root
- hook scripts execute in workspace context with configured timeouts

## Repository Layout

- `cmd/`: Cobra CLI entrypoints (`symphony run ...`)
- `internal/`: runtime packages (config, workflow, tracker, codex, runner, orchestrator, workspace, HTTP)
- `acceptance/`: `godog` feature files and steps
- `testdata/`: workflow and protocol fixtures
