# Monday

Monday is a terminal tool that turns a set of [Claude Code](https://github.com/anthropics/claude-code)
agents into a multi-agent pipeline, driven through [herdr](./herdr.md). Pick which
agents run, in what order, give the pipeline a requirement, and Monday splits herdr
panes, starts each agent, hands the requirement from one agent to the next, and gives
you a place to answer any questions an agent raises mid-task.

## Requirements

- macOS or Linux
- [`herdr`](./herdr.md) and `claude` (Claude Code) on `PATH`
- Monday must be launched from inside an active herdr pane - it has no standalone
  pane-creation path of its own

## Usage

```
monday
```

Launches the full-screen TUI. From there:

1. **Launch screen** - pick a saved pipeline, or "Create an orchestration".
2. **Agent picker / pipeline ordering** - select agents (discovered from
   `~/.claude/agents` and the project's `.claude/agents`) and arrange them into a
   linear chain, optionally saving the pipeline for reuse.
3. **Requirement entry** - describe the task.
4. **Graphify prompt** - optionally inject a saved `graphify-out` knowledge graph into
   the project before the pipeline starts.
5. **Stepped progress view** - watch each agent run, answer any questions it raises,
   and see the pipeline hand off from one agent to the next automatically.

Agents signal Monday with:

```
monday notify --session <session-id> --pane <pane-id> --type QUES|DONE
```

`QUES` surfaces the agent's questions (from `interactions/<agent>.json`) in the TUI
for you to answer; `DONE` advances the pipeline to the next agent, or ends it.

## State

Everything Monday remembers lives under `~/.monday/`:

```
~/.monday/
├── pipelines/<name>.json        # saved pipelines
├── sessions/<uuid>/             # one directory per run
│   ├── session.json
│   ├── history.json
│   ├── docs/                    # documents generated during the pipeline
│   ├── interactions/<agent>.json
│   └── monday.sock
└── graphs/<label>/              # saved graphify-out folders available for injection
```

## Architecture

Three layers, one direction of dependency:

```
tui (presentation)  --->  domain (core logic)  <---  infra (outside world)
```

- `internal/domain` - every persisted struct, every state transition, every
  validation rule and pure algorithm (pipeline validation, pane layout, interaction
  bookkeeping). No IO.
- `internal/infra` - implements domain ports: the filesystem store, the herdr CLI
  wrapper, the notify Unix socket, Claude agent discovery, the Claude skill writer.
- `internal/tui` - BubbleTea screens, wired to domain services only through
  `internal/app.Services`, Monday's single dependency-injection point.

See `docs/architect-plan.md` for the full technical design, `docs/analyst-report.md`
for the original scope, `docs/developer-questions.md` for implementation deviations
discovered against the real herdr binary, and `docs/review-report.md` for the code
review.

## Development

```
go build ./...
go vet ./...
go test ./...
```

A `monday` Claude skill is installed to `~/.claude/skills/monday/SKILL.md` on every
launch, teaching any Claude agent running inside a Monday-managed pane how to notify
Monday, where to save generated documents, and how to use an injected `graphify-out`
graph.
