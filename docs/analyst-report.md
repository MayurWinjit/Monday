# Monday: Requirement Analysis Report

Date: 2026-10-22
Source: REQUIREMENT.md, herdr.md, live inspection of `~/.claude/agents`, `~/.claude/skills/graphify`, and the three sample `graphify-out` folders in this repository.

## 1. Scope Summary

Monday is a Go and BubbleTea terminal tool that lets a developer pick a set of Claude Code agents, arrange them into a step by step pipeline, and have Monday drive that pipeline through `herdr`. Monday splits panes inside the current herdr tab, starts an agent in each pane, carries the user's requirement from one agent to the next, and gives the user a place to answer questions an agent raises mid task. Pipelines can be saved and reused. Everything Monday needs to remember lives under `~/.monday`. The tool runs on macOS and Linux only, and must be launched from inside an active herdr pane since it relies on herdr to create and target panes.

## 2. Resolved Open Questions

These were raised during the interview and are now settled for planning purposes.

1. Tool detection checks only that `herdr` and `claude` exist on PATH. Monday also requires it is running inside an active herdr pane, and exits with a clear message if not.
2. Agent discovery merges global `~/.claude/agents/*.md` with project level `.claude/agents/*.md` when the project folder has one, with project agents taking priority on a name clash. Each agent file is read for its YAML frontmatter (`name`, `description`, and so on).
3. Pipelines are a strict ordered chain of agents for MVP, no branching and no parallel steps. The same agent may appear more than once in one chain.
4. Saved pipelines are small JSON files under `~/.monday/pipelines/`, holding a name, the ordered agent list, and timestamps. The user is prompted for a name at save time and duplicate names are blocked rather than silently overwritten. Picking a saved pipeline on launch skips straight to the requirement text box. The "Create an orchestration" button always runs the full agent select and ordering flow.
5. Pane layout uses a simple row based arrangement rather than a forced square grid, so agent counts that are not perfect squares still look clean (three agents as one row of three, five as two rows of three and two, and so on). The session folder holds `session.json` (pipeline used, agent to pane mapping, status, timestamps), `history.json` (the original requirement plus the full back and forth log), a `docs/` folder for generated files, and `interactions/<agent_name>.json` for questions and answers.
6. Answers are appended into the same `interactions/<agent_name>.json` file as an `"answer"` field added to each question, rather than a separate file. Every question always shows a free text "other" choice in addition to any fixed options.
7. The `monday notify` command talks to the running TUI through a Unix domain socket opened at `~/.monday/sessions/<uuid>/monday.sock`, not through file polling or through `herdr notification`. This gives instant delivery with no polling delay and fits macOS and Linux only support.
8. Graphify injection builds its selection list by scanning whatever has been saved under `~/.monday/graphs/*` at the time, not a fixed list of sample folders. The symlink target is the working directory of the herdr pane running the pipeline. Running the `/graphify` update after the pipeline finishes asks the user for confirmation rather than running automatically.
9. The UI defaults to the catppuccin mocha color flavor with no flavor picker in MVP. The stepped progress indicator's time column is a live elapsed timer counting up per step, not a predicted estimate.
10. The `monday` Claude skill covers how to call `monday notify`, where to save generated documents, the interactions JSON schema and where to write it, how to wait for and read back an answered file, and also a short section on graphify: how to tell whether a `graphify-out` symlink is present in the project root and how to use `graphify query` against it while working, since graphify injection is part of the same pipeline flow the skill is teaching.
11. A crashed or closed pane marks that pipeline step as failed in `session.json` and stops the pipeline rather than continuing past it. A `herdr` or `claude` binary disappearing after the initial launch check is out of scope for MVP, the check only runs once at startup. Closing the Monday TUI leaves any spawned agent panes running in herdr rather than killing them.
12. The MVP boundary below was confirmed as is.

## 3. Functional Requirement Breakdown by Layer

### 3.1 Presentation Layer (BubbleTea UI)

- Launch screen: ASCII `Monday` header, a list of saved pipelines when any exist (hidden entirely when there are none), and a "Create an orchestration" button below that.
- Tool check screen: shown only when `herdr` or `claude` is missing, with install guidance, no further navigation until resolved.
- Agent picker: multi select list built from discovered agents, each row shows agent name and description.
- Pipeline ordering: lets the user arrange the picked agents into a sequence (first to run, then next, and so on), with a save option that prompts for a pipeline name.
- Requirement entry: a text area for the user's requirement plus a submit button that starts a new session.
- Graphify prompt: a yes or no question, followed by a list of available saved `graphify-out` folders when yes is chosen.
- Stepped progress view: one row per pipeline agent, showing agent name, current activity, status (waiting, running, needs input, done, failed), and a live elapsed timer.
- Question and answer view: renders the current agent's pending questions from its interactions file, lets the user pick an option or type a free text "other" answer for each.
- Theming: catppuccin mocha palette applied across all screens.

### 3.2 Domain Layer (core logic)

- Agent catalog: reads and merges global and project level `.claude/agents` files into an in memory agent list.
- Pipeline model: ordered agent list, validation (at least one agent, known agent names), save and load against `~/.monday/pipelines/`.
- Session lifecycle: create a session with a new UUID, track current step, advance the pipeline when a step finishes, mark steps failed when a pane closes unexpectedly.
- Pane layout calculator: turns an agent count into a row based herdr pane split plan.
- Interactions handling: reads an `interactions/<agent_name>.json` file when a `QUES` notify arrives, collects answers, writes them back into the same file, and resumes the agent.
- Notify listener: owns the Unix socket for a running session, decodes `QUES` and `DONE` messages, and triggers the matching domain action.
- Graphify workflow: lists saved graph folders, creates and removes the project root symlink, and calls the graphify skill update step on confirmation.

### 3.3 Infrastructure Layer (formats domain for presentation, talks to the outside world)

- herdr client: wraps the `herdr` CLI calls from herdr.md (`pane split`, `pane run`, `agent prompt`, `pane send-keys`, `pane list`, `tab list`, `agent list`) behind small Go functions the domain layer calls.
- Filesystem store: creates and reads `~/.monday`, `~/.monday/pipelines`, `~/.monday/sessions/<uuid>`, `~/.monday/graphs`, and writes the JSON files the domain layer produces.
- `monday notify` subcommand: a thin CLI entry point that connects to the session's Unix socket and sends a `QUES` or `DONE` message, matching the `--session`, `--pane`, `--type` flags from the requirement.
- Claude skill writer: installs `~/.claude/skills/monday/SKILL.md` with the instructions described in point 10 above.

## 4. Non Functional Constraints

- Platform: macOS and Linux only, no Windows support planned.
- Dependency: both `herdr` and `claude` must already be installed and on PATH, Monday does not install them.
- Runtime context: Monday must be launched from inside an active herdr pane, it has no standalone pane creation path of its own.
- Storage: all state lives under `~/.monday`, nothing is written outside that folder or the target project's root (only the graphify symlink touches the project root, and it is removed after use).
- Responsiveness: the notify path must feel instant to the user, this is why a socket was chosen over file polling.
- Resilience: a single failed pipeline step must not corrupt `session.json` or `interactions` files for the other steps.
- No destructive side effects: Monday never kills a herdr pane or an agent process on its own exit.

## 5. Suggested MVP Boundary

In scope: `~/.monday` directory bootstrap, `herdr` and `claude` presence checks, agent discovery from `.claude/agents`, linear pipeline builder with save and load, row based pane spawning through herdr, session and interactions persistence, the `monday notify` subcommand over a Unix socket, graphify injection with a project root symlink, the `monday` Claude skill file, and a catppuccin mocha themed TUI throughout.

Out of scope for this MVP: branching or parallel pipeline steps, editing or deleting a saved pipeline from the UI, resuming a session after a crash, Windows support, running more than one Monday session at a time on a machine, live cost or token usage tracking, and automatic retry of a failed agent step.

## 6. Suggested Next Step

Hand this report to the developer agent to produce a detailed technical design (package layout, BubbleTea model and message types, JSON schemas as Go structs, and the Unix socket protocol) before any implementation starts.
