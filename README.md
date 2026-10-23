<div align="center">

```
 /$$      /$$                           /$$                    
| $$$    /$$$                          | $$                    
| $$$$  /$$$$  /$$$$$$  /$$$$$$$   /$$$$$$$  /$$$$$$  /$$   /$$
| $$ $$/$$ $$ /$$__  $$| $$__  $$ /$$__  $$ |____  $$| $$  | $$
| $$  $$$| $$| $$  \ $$| $$  \ $$| $$  | $$  /$$$$$$$| $$  | $$
| $$\  $ | $$| $$  | $$| $$  | $$| $$  | $$ /$$__  $$| $$  | $$
| $$ \/  | $$|  $$$$$$/| $$  | $$|  $$$$$$$|  $$$$$$$|  $$$$$$$
|__/     |__/ \______/ |__/  |__/ \_______/ \_______/ \____  $$
                                                      /$$  | $$
                                                     |  $$$$$$/
                                                      \______/ 
```

### A terminal UI that turns Claude Code agents into a self-driving multi-agent pipeline

<p>For developers orchestrating multiple <a href="https://github.com/anthropics/claude-code">Claude Code</a> agents on one task, over <a href="https://github.com/herdrdev/herdr">herdr</a></p>

[![CI](https://img.shields.io/github/actions/workflow/status/MayurWinjit/Monday/ci.yml?style=for-the-badge&logo=githubactions&logoColor=white&label=CI)](https://github.com/MayurWinjit/Monday/actions/workflows/ci.yml)
[![Go Version](https://img.shields.io/github/go-mod/go-version/MayurWinjit/Monday?style=for-the-badge&logo=go&logoColor=white&color=00ADD8)](go.mod)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg?style=for-the-badge)](LICENSE)

[![Claude Code Compatible](https://img.shields.io/badge/Claude%20Code-Compatible-6366f1?style=for-the-badge&logo=anthropic&logoColor=white)](https://github.com/anthropics/claude-code)
[![herdr Required](https://img.shields.io/badge/herdr-Required-f97316?style=for-the-badge)](https://github.com/herdrdev/herdr)
[![Built with Bubble Tea](https://img.shields.io/badge/Built%20with-Bubble%20Tea-ff6ac1?style=for-the-badge)](https://github.com/charmbracelet/bubbletea)

</div>

Monday picks a chain of Claude Code agents, wires them together through `herdr` panes, and
runs them one after another against a single requirement - splitting panes, handing context
from one agent to the next, surfacing any questions an agent raises, and tracking the whole
run in a full-screen terminal UI.

---

## Table of Contents

- [Features](#features)
- [Requirements](#requirements)
- [Installation](#installation)
  - [Build from Source](#build-from-source)
  - [Install Globally](#install-globally)
- [Quick Start](#quick-start)
- [TUI Screens](#tui-screens)
- [Keyboard Shortcuts](#keyboard-shortcuts)
- [`monday notify`](#monday-notify)
- [How a Pipeline Runs](#how-a-pipeline-runs)
- [State on Disk](#state-on-disk)
- [Architecture](#architecture)
- [Development](#development)
- [Contributing](#contributing)
- [License](#license)

---

## Features

| Feature | Description |
|---|---|
| Multi-agent pipelines | Chain any number of Claude Code agents (from `~/.claude/agents` or a project's `.claude/agents`) into a linear, ordered flow |
| Save & reuse pipelines | Name a pipeline once and launch it again later without re-picking agents |
| Automatic handoff | Each agent's output becomes the next agent's input; Monday tracks progress per step and advances the pipeline on its own |
| Inline Q&A | When an agent needs a decision, Monday surfaces its questions in the TUI and writes your answers straight back to the agent |
| Knowledge graph injection | Optionally inject a saved [`graphify`](https://github.com/safishamsi/graphify) knowledge graph into the project before a pipeline starts |
| Self-documenting agents | Monday installs a Claude skill on every launch that teaches any agent how to notify Monday, where to save documents, and how to use an injected graph |
| Full-screen TUI | Built on [Bubble Tea](https://github.com/charmbracelet/bubbletea), responsive to terminal resizes, with no business logic in the presentation layer |

---

## Requirements

- **Go 1.26+** (only if building from source)
- **macOS or Linux**
- [`herdr`](https://github.com/herdrdev/herdr) and `claude` (Claude Code) available on `PATH`
- Monday must be launched from inside an active `herdr` pane - it has no standalone
  pane-creation path of its own

Monday checks all of the above on startup and tells you exactly what's missing if anything
isn't satisfied.

---

## Installation

### Build from Source

```bash
# Clone the repository
git clone https://github.com/MayurWinjit/Monday.git
cd Monday

# Build the binary
make build-macos   # or: make build-linux
```

### Install Globally

```bash
go install github.com/MayurWinjit/monday/cmd/monday@latest
```

Or, after building from source:

```bash
make install   # installs to /usr/local/bin

# Verify
monday --help
```

Run `make help` to see all available targets.

---

## Quick Start

```bash
# 1. Start herdr and open a pane
herdr

# 2. Launch Monday from inside that pane
monday

# 3. Pick a saved pipeline, or create one - select agents and arrange their order

# 4. Describe the requirement for the pipeline to work on

# 5. (Optional) Inject a saved graphify-out knowledge graph into the project

# 6. Watch the pipeline run - answer any questions an agent raises, Monday
#    advances to the next agent automatically
```

---

## TUI Screens

| Screen | Purpose |
|---|---|
| Launch | Pick a saved pipeline, or start a new one |
| Agent picker | Select which discovered agents take part |
| Pipeline order | Arrange the agents into a sequence, optionally saving it for reuse |
| Requirement | Describe the task the pipeline should accomplish |
| Graphify prompt | Optionally inject a saved knowledge graph into the project |
| Progress | Watch each agent run in turn, answer questions, and track handoff until done |
| Q&A | Answer the questions an agent raised, inline, without leaving the TUI |

---

## Keyboard Shortcuts

| Screen | Keys |
|---|---|
| Launch | `↑`/`↓` move · `enter` select · `q` quit |
| Agent picker | `↑`/`↓` move · `space` toggle · `enter` continue · `q` quit |
| Pipeline order | `↑`/`↓` move cursor · `shift+↑`/`↓` reorder · `a` add again · `d` remove · `s` save as... · `enter` continue · `q` quit |
| Requirement | `tab` switch focus · `ctrl+s` submit · `ctrl+c` quit |
| Graphify prompt | `←`/`→` choose · `↑`/`↓` move · `enter` select · `q` quit |
| Progress | `enter` open questions (when a row needs input) · `q` quit |
| Q&A | `↑`/`↓` option · `←`/`→` question · `enter` select · `tab` submit (once all answered) · `esc` close |

---

## `monday notify`

The one command-line entry point agents use to signal Monday from inside their own pane:

```bash
monday notify --session <session-id> --pane <pane-id> --type QUES|DONE
```

**Flags:**

| Flag | Description |
|---|---|
| `--session` | Session UUID (required) |
| `--pane` | Pane ID this notification is from (required) |
| `--type` | `QUES` or `DONE` (required) |

- `QUES` surfaces the agent's questions (from `interactions/<agent>.json`) in the TUI for you to answer.
- `DONE` advances the pipeline to the next agent, or finishes it if it was the last step.

---

## How a Pipeline Runs

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

---

## State on Disk

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

---

## Architecture

Monday is layered with a single, enforced direction of dependency:

```
internal/tui (presentation)  --->  internal/domain (core logic)  <---  internal/infra (outside world)
```

| Layer | Responsibility |
|---|---|
| `internal/domain` | Persisted types, state transitions, validation, pure algorithms. No I/O. |
| `internal/infra` | Implements domain ports: filesystem store, the `herdr` client, the notify Unix socket, Claude agent discovery, the Claude skill writer. |
| `internal/tui` | Bubble Tea screens, wired to domain services only through `internal/app.Services` - Monday's single dependency-injection point. |

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

---

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

A `monday` Claude skill is written to `~/.claude/skills/monday/SKILL.md` on every launch,
teaching any Claude agent running inside a Monday-managed pane how to notify Monday, where
to save generated documents, and how to use an injected `graphify-out` graph.

---

## Contributing

Contributions are welcome.

1. Fork the repository and create a branch from `main`.
2. Make your change, keeping domain logic, infrastructure, and TUI presentation in their
   existing layers.
3. Run `go build ./...`, `go vet ./...`, and `go test -race ./...` before opening a PR.
4. Open a pull request describing what changed and why.

Please open an issue first for larger changes so the direction can be agreed on before you
invest the time.

---

## License

[MIT License](LICENSE)

---

<div align="center">
  <p><sub>Built by <a href="https://github.com/MayurWinjit">MayurWinjit</a></sub></p>
</div>
