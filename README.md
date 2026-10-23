# Monday

**A terminal UI that turns a set of [Claude Code](https://github.com/anthropics/claude-code) agents into a self-driving multi-agent pipeline.**

[![CI](https://github.com/MayurWinjit/Monday/actions/workflows/ci.yml/badge.svg)](https://github.com/MayurWinjit/Monday/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/MayurWinjit/monday.svg)](https://pkg.go.dev/github.com/MayurWinjit/monday)
[![Go Report Card](https://goreportcard.com/badge/github.com/MayurWinjit/monday)](https://goreportcard.com/report/github.com/MayurWinjit/monday)
[![Go Version](https://img.shields.io/github/go-mod/go-version/MayurWinjit/Monday)](go.mod)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

Monday picks a chain of Claude Code agents, wires them together through [`herdr`](https://github.com/herdrdev/herdr),
and runs them one after another against a single requirement - splitting panes, handing
context from one agent to the next, surfacing any questions an agent raises, and
tracking the whole run in a full-screen terminal UI.

<br/>

## Contents

- [Why Monday](#why-monday)
- [Features](#features)
- [Requirements](#requirements)
- [Installation](#installation)
- [Usage](#usage)
- [How a pipeline runs](#how-a-pipeline-runs)
- [State on disk](#state-on-disk)
- [Architecture](#architecture)
- [Development](#development)
- [Contributing](#contributing)
- [License](#license)

## Why Monday

Running several Claude Code agents on one task today means manually opening panes,
copy-pasting context between them, and babysitting each one for questions. Monday
automates that handoff: describe the task once, pick the agents and their order, and
Monday runs the whole chain for you - pausing only when an agent actually needs your
input.

## Features

- **Multi-agent pipelines** - chain any number of Claude Code agents (from
  `~/.claude/agents` or a project's `.claude/agents`) into a linear, ordered flow.
- **Save and reuse pipelines** - name a pipeline once and launch it again later without
  re-picking agents.
- **Automatic handoff** - each agent's output becomes the next agent's input; Monday
  tracks progress per step and advances the pipeline on its own.
- **Inline Q&A** - when an agent needs a decision, Monday surfaces its questions in the
  TUI and writes your answers straight back to the agent.
- **Knowledge graph injection** - optionally inject a saved [`graphify`](https://github.com/safishamsi/graphify)
  knowledge graph into the project before a pipeline starts, so every agent has
  structured context from the first prompt.
- **Self-documenting agents** - Monday installs a Claude skill on every launch that
  teaches any agent running in one of its panes how to notify Monday, where to save
  generated documents, and how to use an injected graph.
- **A real terminal UI** - built on [Bubble Tea](https://github.com/charmbracelet/bubbletea),
  responsive to terminal resizes, with no business logic leaking into the presentation
  layer.

## Requirements

- macOS or Linux
- [`herdr`](https://github.com/herdrdev/herdr) and `claude` (Claude Code) available on `PATH`
- Monday must be launched from inside an active `herdr` pane - it has no
  standalone pane-creation path of its own

Monday checks all of the above on startup and tells you exactly what's missing if
anything isn't satisfied.

## Installation

### With `go install`

```bash
go install github.com/MayurWinjit/monday/cmd/monday@latest
```

### From source

```bash
git clone https://github.com/MayurWinjit/Monday.git
cd Monday
make build-macos   # or: make build-linux
make install       # installs to /usr/local/bin
```

Run `make help` to see all available targets.

## Usage

```bash
monday
```

This launches the full-screen TUI. From there:

1. **Launch** - pick a saved pipeline, or start a new one.
2. **Agent picker** - select which agents take part.
3. **Pipeline order** - arrange the agents into a sequence, optionally saving it for
   reuse.
4. **Requirement** - describe the task the pipeline should accomplish.
5. **Graphify prompt** *(optional)* - inject a saved knowledge graph into the project.
6. **Progress** - watch each agent run in turn, answer any questions it raises, and see
   the pipeline hand off automatically until it's done.

### Agent notifications

Agents signal Monday from inside their own pane:

```bash
monday notify --session <session-id> --pane <pane-id> --type QUES|DONE
```

- `QUES` surfaces the agent's questions (from `interactions/<agent>.json`) in the TUI
  for you to answer.
- `DONE` advances the pipeline to the next agent, or finishes it if it was the last
  step.

## How a pipeline runs

```
 you                 Monday                      herdr panes
 ───                 ──────                      ───────────
  │  describe task      │                               │
  ├────────────────────▶│                               │
  │                      │  split pane, start agent 1    │
  │                      ├──────────────────────────────▶│
  │                      │                     DONE ◀────┤
  │                      │  hand off, start agent 2       │
  │                      ├──────────────────────────────▶│
  │                      │                     QUES ◀────┤
  │   answer question    │                               │
  │◀─────────────────────┤                               │
  │─────────────────────▶│  resume agent 2                │
  │                      ├──────────────────────────────▶│
  │                      │                     DONE ◀────┤
  │   pipeline complete  │                               │
  │◀─────────────────────┤                               │
```

## State on disk

Everything Monday remembers lives under `~/.monday/`:

```
~/.monday/
├── pipelines/<name>.json        # saved pipelines
├── sessions/<uuid>/              # one directory per run
│   ├── session.json
│   ├── history.json
│   ├── docs/                     # documents generated during the pipeline
│   ├── interactions/<agent>.json
│   └── monday.sock
└── graphs/<label>/               # saved graphify-out folders available for injection
```

## Architecture

Monday is layered with a single, enforced direction of dependency:

```
internal/tui (presentation)  --->  internal/domain (core logic)  <---  internal/infra (outside world)
```

| Layer              | Responsibility                                                                 |
|--------------------|---------------------------------------------------------------------------------|
| `internal/domain`  | Persisted types, state transitions, validation, pure algorithms. No I/O.        |
| `internal/infra`   | Implements domain ports: filesystem store, the `herdr` client, the notify Unix socket, Claude agent discovery, the Claude skill writer. |
| `internal/tui`     | Bubble Tea screens, wired to domain services only through `internal/app.Services` - Monday's single dependency-injection point. |

```
cmd/monday             entrypoint: TUI launcher + `monday notify` subcommand
internal/
├── app                Services: wires domain + infra together
├── buildinfo          version metadata
├── domain
│   ├── agent          agent discovery/catalog
│   ├── graphify       saved-graph bookkeeping
│   ├── history        per-session run history
│   ├── interaction    Q&A file format and bookkeeping
│   ├── layout         pane layout rules
│   ├── notifyproto    wire format for `monday notify`
│   ├── pipeline       saved pipeline CRUD
│   └── session        the pipeline run state machine
├── infra
│   ├── claudeagents   discovers agents under ~/.claude and <project>/.claude
│   ├── clock          injectable system clock
│   ├── fsstore        filesystem-backed persistence
│   ├── herdr          herdr CLI wrapper
│   ├── idgen          UUID generation
│   ├── skillwriter    installs the `monday` Claude skill
│   ├── socket         the QUES/DONE notify Unix socket
│   └── toolcheck      startup environment checks
└── tui
    ├── app            screen router, top-level Model
    ├── app/screens    toolcheck, launch, agentpicker, pipelineorder,
    │                  requirement, graphifyprompt, progress, qa
    ├── brand          shared ASCII wordmark
    ├── components     shared, reusable view helpers
    └── theme          Catppuccin-based color palette and shared styles
```

## Development

```bash
go build ./...
go vet ./...
go test -race ./...
```

Build a release binary with the Makefile:

```bash
make build-macos   # or make build-linux
```

A `monday` Claude skill is written to `~/.claude/skills/monday/SKILL.md` on every
launch, teaching any Claude agent running inside a Monday-managed pane how to notify
Monday, where to save generated documents, and how to use an injected `graphify-out`
graph.

## Contributing

Contributions are welcome.

1. Fork the repository and create a branch from `main`.
2. Make your change, keeping domain logic, infrastructure, and TUI presentation in
   their existing layers.
3. Run `go build ./...`, `go vet ./...`, and `go test -race ./...` before opening a PR.
4. Open a pull request describing what changed and why.

Please open an issue first for larger changes so the direction can be agreed on before
you invest the time.

## License

Monday is released under the [MIT License](LICENSE).
