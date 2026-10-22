# Monday - Technical Design & Implementation Plan

Date: 2026-10-22
Inputs: `docs/analyst-report.md` (finalized scope), `REQUIREMENT.md`, `herdr.md`.
Audience: `godeveloper` agent. This document is the single source of truth for package layout, types, protocols, screen behavior, and build order. Do not re-derive scope from `REQUIREMENT.md` directly - where the two disagree, `docs/analyst-report.md` wins, and where this plan adds a decision not present in either, it is called out explicitly as a design decision.

Module: `github.com/MayurWinjit/monday`, Go 1.26, macOS and Linux only.

---

## 1. Architecture Overview

Three layers, one direction of dependency:

```
tui (presentation)  --->  domain (core logic)  <---  infra (outside world)
```

- **domain** depends on nothing outside the standard library plus its own `ports.go` interfaces. It defines every JSON-persisted struct, every state transition, every validation rule, and every algorithm (pipeline validation, pane layout math, interaction bookkeeping). It never imports `os/exec`, `net`, or `bubbletea`.
- **infra** implements domain ports: talking to the filesystem, shelling out to `herdr`, listening on a Unix socket, parsing `.claude/agents/*.md` frontmatter, writing the Claude skill file. Infra imports domain (for the types/ports) but domain never imports infra.
- **tui** imports domain (for types and service interfaces) and is wired to concrete infra implementations only once, in `cmd/monday/main.go`. Screens never call `os/exec`, `os.ReadFile`, or `net.Dial` directly - they call domain services through `tea.Cmd` closures.

This is what "avoid code duplication across layers" means concretely in this plan:

1. **One struct per concept.** `Pipeline`, `Session`, `PipelineStep`, `Question`, `InteractionFile`, `NotifyMessage` are defined exactly once, in `internal/domain/*/types.go`. Infra encodes/decodes them with `encoding/json`. TUI renders them. Nobody redeclares a "DTO" version for transport or a "view model" version for display - screens format domain values directly with small pure helper functions (e.g. `formatElapsed(d time.Duration) string`) that live in `internal/tui/theme` or next to the screen, not duplicated per screen.
2. **One code path for every mutation.** Both the running TUI process and the separate `monday notify` CLI process end up calling the same `internal/domain/session` and `internal/domain/interaction` service methods - the CLI path is thinner (no UI) but never reimplements file writing or socket framing; it reuses `internal/infra/socket` client code.
3. **One herdr wrapper.** Every herdr CLI invocation goes through `internal/infra/herdr.Client`. No screen, service, or subcommand calls `exec.Command("herdr", ...)` itself.
4. **One theme.** Catppuccin mocha colors and the common component styles (card, badge, button, list row) are defined once in `internal/tui/theme` and composed by every screen, never copy-pasted.

---

## 2. Package Layout

```
monday/
├── cmd/
│   └── monday/
│       └── main.go                 # dispatches: `monday` -> run TUI, `monday notify ...` -> notify CLI
├── internal/
│   ├── domain/
│   │   ├── agent/
│   │   │   └── types.go            # Agent, AgentSource
│   │   ├── pipeline/
│   │   │   ├── types.go            # Pipeline
│   │   │   └── validate.go         # pure validation rules
│   │   ├── session/
│   │   │   ├── types.go            # Session, PipelineStep, Status enums
│   │   │   ├── service.go          # SessionService (lifecycle, advance, fail)
│   │   │   └── notify.go           # NotifyEvent, NotifyRouter
│   │   ├── interaction/
│   │   │   ├── types.go            # InteractionFile, Question
│   │   │   └── service.go          # InteractionService
│   │   ├── history/
│   │   │   └── types.go            # History, HistoryEntry
│   │   ├── layout/
│   │   │   └── planner.go          # pane layout calculator (pure math)
│   │   ├── graphify/
│   │   │   ├── types.go            # GraphRef
│   │   │   └── service.go          # GraphifyService
│   │   ├── notifyproto/
│   │   │   └── types.go            # NotifyMessage, NotifyAck, NotifyType (shared wire schema)
│   │   └── ports.go                # every interface domain depends on
│   ├── infra/
│   │   ├── fsstore/                # ~/.monday layout, read/write JSON files
│   │   ├── herdr/                  # exec.Command wrapper implementing ports.HerdrClient
│   │   ├── socket/                 # Unix socket server + client implementing ports.Notify*
│   │   ├── claudeagents/           # frontmatter parsing + global/project merge
│   │   ├── skillwriter/            # writes ~/.claude/skills/monday/SKILL.md
│   │   └── toolcheck/              # PATH + "inside a herdr pane" checks
│   ├── app/
│   │   └── wiring.go               # builds the Services struct used by both cmd entry points
│   └── tui/
│       ├── theme/                  # catppuccin mocha palette + shared lipgloss styles
│       ├── components/             # reusable: list row, badge, button, stepper row, spinner wrap
│       └── app/
│           ├── model.go            # root model, Screen interface, switchScreenMsg
│           ├── messages.go         # every cross-screen tea.Msg type
│           └── screens/
│               ├── toolcheck/
│               ├── launch/
│               ├── agentpicker/
│               ├── pipelineorder/
│               ├── requirement/
│               ├── graphifyprompt/
│               ├── progress/
│               └── qa/
├── docs/
├── go.mod
└── main.go -> replaced by cmd/monday/main.go (see step 1 of the build order)
```

`internal/app.Services` is the one and only dependency-injection point:

```go
// internal/app/wiring.go
type Services struct {
    Bootstrap   *fsstore.Bootstrap
    ToolCheck   *toolcheck.Checker
    Agents      *claudeagents.Catalog
    Pipelines   *pipeline.Store      // thin wrapper over fsstore, domain-owned interface
    Sessions    *session.Service
    Interactions *interaction.Service
    Graphify    *graphify.Service
    Herdr       ports.HerdrClient
    SkillWriter *skillwriter.Writer
}

func Build(homeDir string) (*Services, error) { /* constructs infra, injects into domain services */ }
```

`cmd/monday/main.go` calls `app.Build` once, then either hands `Services` to `tui/app.New(services)` or to the notify subcommand.

---

## 3. Domain Types (Go structs = JSON schemas)

All timestamps are `time.Time`, marshaled as RFC3339 (Go default). All IDs are strings (`google/uuid` for session IDs, short human-readable slugs for question IDs supplied by the calling agent).

### 3.1 Agent (`internal/domain/agent/types.go`)

Not persisted by Monday itself (it lives in `.claude/agents/*.md`); this is the in-memory discovery result.

```go
type AgentSource string

const (
    AgentSourceGlobal  AgentSource = "global"
    AgentSourceProject AgentSource = "project"
)

type Agent struct {
    Name        string      `json:"name"`
    Description string      `json:"description"`
    Source      AgentSource `json:"source"`
    FilePath    string      `json:"file_path"`
}
```

Merge rule (domain, `agent.Merge(global, project []Agent) []Agent`): start from `global`, then overlay every `project` entry whose `Name` matches - replacing it - and append project-only entries. Project always wins on name clash. Pure function, no IO.

### 3.2 Pipeline (`internal/domain/pipeline/types.go`, file: `~/.monday/pipelines/<name>.json`)

```go
type Pipeline struct {
    Name      string    `json:"name"`
    Agents    []string  `json:"agents"`     // ordered agent names, duplicates allowed
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}
```

Validation (`pipeline.Validate`): at least one agent; every name must resolve against the currently discovered agent catalog (if an agent file was deleted since the pipeline was saved, surface a clear "unknown agent: X" error rather than silently dropping the step); name itself must be non-empty, trimmed, and unique against existing saved pipeline file names (checked by the store, not here, since it needs a directory listing).

### 3.3 Session (`internal/domain/session/types.go`, file: `~/.monday/sessions/<uuid>/session.json`)

```go
type SessionStatus string

const (
    SessionRunning SessionStatus = "running"
    SessionDone    SessionStatus = "done"
    SessionFailed  SessionStatus = "failed"
)

type StepStatus string

const (
    StepWaiting    StepStatus = "waiting"
    StepRunning    StepStatus = "running"
    StepNeedsInput StepStatus = "needs_input"
    StepDone       StepStatus = "done"
    StepFailed     StepStatus = "failed"
)

type PipelineStep struct {
    Index              int        `json:"index"`
    AgentName          string     `json:"agent_name"`
    InteractionsFile   string     `json:"interactions_file"`   // resolved file name, see section 9.1
    PaneID             string     `json:"pane_id"`
    Status             StepStatus `json:"status"`
    Activity           string     `json:"activity,omitempty"`  // last known one-line status, free text
    StartedAt          *time.Time `json:"started_at,omitempty"`
    FinishedAt         *time.Time `json:"finished_at,omitempty"`
}

type Session struct {
    ID               string         `json:"id"`
    PipelineName     string         `json:"pipeline_name,omitempty"` // empty when built ad hoc, not saved
    Pipeline         pipeline.Pipeline `json:"pipeline"`              // snapshot at session start
    Requirement      string         `json:"requirement"`
    HerdrTabID       string         `json:"herdr_tab_id"`
    MondayPaneID     string         `json:"monday_pane_id"`
    WorkAreaPaneID   string         `json:"work_area_pane_id"`
    WorkingDir       string         `json:"working_dir"`
    Steps            []PipelineStep `json:"steps"`
    CurrentStepIndex int            `json:"current_step_index"`
    Status           SessionStatus  `json:"status"`
    GraphifyGraph    string         `json:"graphify_graph,omitempty"`
    GraphifySymlink  string         `json:"graphify_symlink,omitempty"` // absolute path removed on cleanup
    CreatedAt        time.Time      `json:"created_at"`
    UpdatedAt        time.Time      `json:"updated_at"`
}
```

### 3.4 History (`internal/domain/history/types.go`, file: `~/.monday/sessions/<uuid>/history.json`)

```go
type HistoryRole string

const (
    HistoryUser  HistoryRole = "user"
    HistoryAgent HistoryRole = "agent"
    HistorySystem HistoryRole = "system"
)

type HistoryEntry struct {
    Timestamp time.Time   `json:"timestamp"`
    StepIndex int         `json:"step_index"`
    AgentName string      `json:"agent_name,omitempty"`
    Role      HistoryRole `json:"role"`
    Content   string      `json:"content"`
}

type History struct {
    Requirement string         `json:"requirement"`
    Entries     []HistoryEntry `json:"entries"`
}
```

### 3.5 Interactions (`internal/domain/interaction/types.go`, file: `~/.monday/sessions/<uuid>/interactions/<file>.json`)

Matches `REQUIREMENT.md` exactly, with the answer merged into each question per analyst resolution #6.

```go
type Question struct {
    ID          string   `json:"id"`
    Question    string   `json:"question"`
    Description string   `json:"description,omitempty"`
    Options     []string `json:"options,omitempty"`
    Answer      *string  `json:"answer,omitempty"`
}

type InteractionFile struct {
    Agent     string     `json:"agent"`
    Questions []Question `json:"questions"`
}
```

No separate "answers file" and no separate "other" flag in the schema - the UI always offers a free-text "Other" row alongside `Options`; whatever the user picks or types becomes the single `Answer` string. Keeping the schema this flat is deliberate: the agent-side skill only has to read one file and check which questions now have a non-nil `Answer`.

### 3.6 Notify wire schema (`internal/domain/notifyproto/types.go`)

```go
type NotifyType string

const (
    NotifyQuestion NotifyType = "QUES"
    NotifyDone     NotifyType = "DONE"
)

type NotifyMessage struct {
    Type      NotifyType `json:"type"`
    SessionID string     `json:"session_id"`
    PaneID    string     `json:"pane_id"`
}

type NotifyAck struct {
    OK    bool   `json:"ok"`
    Error string `json:"error,omitempty"`
}
```

This is the only place the wire format is defined. Both `infra/socket` (server + client) and `cmd/monday`'s notify subcommand import it - never redeclared.

---

## 4. Domain Ports (`internal/domain/ports.go`)

```go
type HerdrClient interface {
    CurrentPaneID() (string, error)
    CurrentTabID() (string, error)
    ListPanes(tabID string) ([]PaneInfo, error)
    SplitPane(paneID string, dir Direction) (newPaneID string, err error)
    StartAgent(paneID, agentName string) error             // herdr agent start <name> --kind claude --pane <pane_id>
    PromptAgent(paneID, prompt string) error                // herdr agent prompt <pane_id> <text>
    SendKeys(paneID, keys string) error                     // herdr pane send-keys <pane_id> <keys>
    ReadAgent(paneID string) (string, error)                // herdr agent read <pane_id>
    ExplainAgent(paneID string) (AgentExplain, error)       // herdr agent explain --json <pane_id>
    ClosePane(paneID string) error
}

type Store interface {
    EnsureLayout() error                                    // ~/.monday, pipelines/, sessions/, graphs/
    ListPipelines() ([]pipeline.Pipeline, error)
    SavePipeline(p pipeline.Pipeline) error
    PipelineNameTaken(name string) (bool, error)
    CreateSessionDir(id string) error
    SaveSession(s session.Session) error
    LoadSession(id string) (session.Session, error)
    SaveHistory(id string, h history.History) error
    LoadHistory(id string) (history.History, error)
    SaveInteractions(sessionID, file string, f interaction.InteractionFile) error
    LoadInteractions(sessionID, file string) (interaction.InteractionFile, error)
    DocsDir(sessionID string) string
    WriteDoc(sessionID, relPath string, content []byte) error
    ListSavedGraphs() ([]string, error)
}

type NotifyServer interface {
    Listen(sessionID string) (events <-chan notifyproto.NotifyMessage, closeFn func() error, err error)
}

type NotifyClient interface {
    Send(socketPath string, msg notifyproto.NotifyMessage) (notifyproto.NotifyAck, error)
}

type AgentCatalog interface {
    Discover(projectDir string) ([]agent.Agent, error)
}

type SkillInstaller interface {
    Install() error // writes ~/.claude/skills/monday/SKILL.md, idempotent
}

type Clock interface {
    Now() time.Time
}

type IDGenerator interface {
    NewID() string // uuid v4
}
```

Everything above is implemented once in `internal/infra/*` and consumed by `internal/domain/*/service.go` constructors (`session.NewService(store, herdr, notify, clock, ids)`), never reimplemented by `tui`.

---

## 5. Infrastructure Components

### 5.1 Filesystem store (`internal/infra/fsstore`)

Directory bootstrap on every launch (idempotent, `os.MkdirAll` with `0700`):

```
~/.monday/
├── pipelines/<name>.json
├── sessions/<uuid>/
│   ├── session.json
│   ├── history.json
│   ├── docs/
│   ├── interactions/<file>.json
│   └── monday.sock            (created/removed by infra/socket, not fsstore)
└── graphs/<label>/             (user-populated graphify-out contents, see section 5.2 design note)
```

`SavePipeline` writes to a temp file in the same directory and `os.Rename`s over the target - this is the one place that guards the "a failed step must not corrupt `session.json`" NFR: every write in `fsstore` (session, history, interactions) uses write-temp-then-rename, never a truncate-in-place write, so a crash mid-write never leaves a half-written JSON file behind.

`PipelineNameTaken` is a `os.Stat` on `pipelines/<name>.json` - duplicate names are rejected at the domain layer using this check before `SavePipeline` is called.

### 5.2 herdr client (`internal/infra/herdr`)

All calls go through `exec.Command("herdr", args...)`, capturing stdout, decoding a generic envelope:

```go
type envelope[T any] struct {
    ID     string `json:"id"`
    Result T      `json:"result"`
}
```

Known response shapes from `herdr.md` (`tab list`, `pane list`, `agent list`) decode cleanly into this envelope. Two calls are **not** documented with a JSON example in `herdr.md`: `pane split` and `agent start`. Treat their exact output as an integration risk to validate immediately (build order step 3) rather than guessed at compile time:

- Attempt to decode the same envelope shape with a `result.pane_id` (or `result.new_pane_id`) field.
- If that fails, fall back to a **diff strategy**: call `ListPanes(tabID)` before the split, call `herdr pane split`, call `ListPanes(tabID)` again, and the pane ID present in the "after" list but not the "before" list is the new pane. This fallback is implemented from day one (it is strictly more robust than trusting undocumented JSON), and the direct-parse path is only used as an optimization if it turns out to work.

**Current pane / tab detection.** Neither `herdr.md` nor the requirement says how a process running inside a herdr pane discovers its own pane/tab ID. Design decision: check for a `HERDR_PANE_ID` / `HERDR_TAB_ID` environment variable first (the most likely mechanism a multiplexer uses to tell a child process where it lives); if absent, call `herdr pane list` across all tabs and use the pane where `"focused": true`; if neither resolves to exactly one pane, show the Tool Check screen's sibling error state: "Monday could not determine its own herdr pane - please launch Monday from inside an active herdr pane" and exit. This triple fallback is implemented as `toolcheck.CurrentPane()` and reused by `herdr.Client.CurrentPaneID()`.

**Spawning the pipeline grid.** Monday's own pane is never reused as an agent pane. Sequence:

1. `work := SplitPane(mondayPaneID, DirectionRight)` - one split, creating a dedicated work area. Monday's own pane shrinks and keeps showing the Monday TUI for the rest of the session.
2. `layout.Plan(n)` (pure domain function, section 5.2a below) returns an ordered list of `SplitInstruction{FromLogical, Direction, NewLogical}` where logical index `0` is `work`.
3. The herdr executor walks the instructions in order, resolving each `FromLogical` to a real pane ID (a map built incrementally, seeded with `0 -> work`), calling `SplitPane`, and recording the result under `NewLogical`.
4. The final `logical -> real pane ID` map, read in logical order `0..n-1`, is the row-major pane assignment for `Steps[0..n-1]`.
5. For every step, call `StartAgent(paneID, agentName)` (`herdr agent start <name> --kind claude --pane <pane_id>`) so every agent process is alive and idle before any prompting starts.
6. Only step 0 is then given its task immediately: `PromptAgent(pane0, initialPrompt)` followed by `SendKeys(pane0, "enter")`.

If `agent start` turns out not to exist or not to behave as documented during step-3 validation, the fallback is `herdr pane run <pane_id> "claude --agent <name>"` with no trailing prompt text (start idle), then the same `PromptAgent` + `SendKeys` call once it is that step's turn. Both paths end at the same place: an idle `claude --agent X` process in its own pane, prompted only when it is its turn. This keeps the "spawn everything, activate sequentially" behavior required by `REQUIREMENT.md` steps 12-14 regardless of which concrete herdr primitive wins.

**Prompt payload design decision.** Never inline the user's (potentially multi-line) requirement or a prior agent's full output into the shell command text. Instead:

- `fsstore.WriteDoc(sessionID, "requirement.md", []byte(requirement))` is written once at session creation.
- Every `PromptAgent` call sends a short, single-line, fixed-shape instruction, e.g.:

```
You are running as part of a Monday pipeline. SESSION_ID=<uuid> PANE_ID=<pane_id> AGENT_NAME=<name> STEP=<index+1>/<total>. Read ~/.monday/sessions/<uuid>/docs/requirement.md for the task, ~/.monday/sessions/<uuid>/history.json for prior steps, and ~/.monday/sessions/<uuid>/docs/ for any generated files. When you need user input, follow the monday skill. When you are completely done with your part, run: monday notify --session <uuid> --pane <pane_id> --type DONE
```

This avoids shell-escaping of newlines/quotes entirely and matches `REQUIREMENT.md` step 15's instruction to hand off via the docs folder rather than a giant inline string.

#### 5.2a Pane layout calculator (`internal/domain/layout/planner.go`)

Pure function, unit-testable without herdr:

```go
type Direction string

const (
    DirectionRight Direction = "right"
    DirectionDown  Direction = "down"
)

type SplitInstruction struct {
    FromLogical int
    Direction   Direction
    NewLogical  int
}

const MaxPerRow = 3

func Plan(n int) []SplitInstruction
```

Algorithm: `rows := chunk(n, MaxPerRow)` (e.g. `n=5 -> [3,2]`, `n=3 -> [3]`, `n=7 -> [3,3,1]`). Walk rows; the first pane of row 0 is logical `0` (already exists, no instruction needed). For every subsequent row, emit one `Direction: Down` split from the first pane of the previous row to create the first pane of the new row, then emit `len(row)-1` `Direction: Right` splits walking left to right within the row. This produces exactly `n-1` instructions for `n` panes and matches the analyst's examples verbatim (3 agents -> one row of three, 5 agents -> rows of three and two).

### 5.3 Unix socket notify protocol (`internal/infra/socket`)

Socket path: `~/.monday/sessions/<uuid>/monday.sock`, created with the session directory (mode `0700` directory), removed (`os.Remove`, ignore `ENOENT`) both defensively before `net.ListenUnix` and on session teardown.

Framing: **one `NotifyMessage` per connection.** The client dials, writes one newline-terminated JSON object, reads one newline-terminated `NotifyAck` JSON object, then closes. No multiplexing, no length-prefixing, no keep-alive - this is the simplest protocol that satisfies "instant delivery, no polling" and is trivial to implement correctly on both ends with `encoding/json.Encoder`/`Decoder` over `net.Conn`.

Server (`socket.Server`, implements `ports.NotifyServer`):

```go
func (s *Server) Listen(sessionID string) (<-chan notifyproto.NotifyMessage, func() error, error) {
    // net.ListenUnix("unix", &net.UnixAddr{Name: path, Net: "unix"})
    // goroutine: Accept loop; each connection handled in its own goroutine:
    //   decode NotifyMessage, validate msg.SessionID == sessionID (defense against stale sockets),
    //   push to buffered channel, encode+write NotifyAck{OK:true}, close conn.
    // malformed input -> NotifyAck{OK:false, Error:"..."}, connection closed, nothing pushed to channel.
}
```

Client (`socket.Client`, implements `ports.NotifyClient`), used by the `monday notify` subcommand:

```go
func (c *Client) Send(socketPath string, msg notifyproto.NotifyMessage) (notifyproto.NotifyAck, error) {
    // net.DialTimeout("unix", socketPath, 2*time.Second)
    // encode msg, read one ack line, return it
}
```

**Single-writer rule (concurrency safety).** The socket Accept loop and its per-connection goroutines never touch the filesystem and never mutate `session.Session` directly - they only push a `NotifyMessage` onto a channel. All domain mutation (marking a step `done`, loading the interactions file, writing `session.json`) happens inside the BubbleTea `Update` loop, which is single-threaded by construction. The bridge is one long-lived `tea.Cmd`:

```go
func listenForNotify(events <-chan notifyproto.NotifyMessage) tea.Cmd {
    return func() tea.Msg { return notifyEventMsg{msg: <-events} }
}
```

`Update` re-issues `listenForNotify` every time it handles a `notifyEventMsg`, so exactly one listener command is in flight at any time and every event is processed strictly in arrival order, in the same goroutine as every other state mutation. This is what guarantees a single failed step cannot corrupt another step's files.

### 5.4 Claude agent discovery (`internal/infra/claudeagents`)

Reads `~/.claude/agents/*.md` (global) and, when present, `<projectDir>/.claude/agents/*.md` (project). Frontmatter is the `---\n...\n---` block at the top of the file, parsed with `gopkg.in/yaml.v3` into:

```go
type frontmatter struct {
    Name        string `yaml:"name"`
    Description string `yaml:"description"`
}
```

Files with no frontmatter, an unterminated frontmatter block, or an empty `name` are skipped with a warning collected into a non-fatal `[]error` returned alongside the successfully parsed agents - discovery must never hard-fail the whole app because one agent file is malformed. The merge itself delegates to the pure `agent.Merge` function from section 3.1.

### 5.5 Claude skill writer (`internal/infra/skillwriter`)

Writes `~/.claude/skills/monday/SKILL.md` (creating `~/.claude/skills/monday/` if needed) every time Monday starts - the file is small, static, and versioned with the binary, so idempotent overwrite is simpler and safer than diffing. Exact content is in section 9.

---

## 6. Domain Services (use cases)

Each of these lives in `internal/domain/<pkg>/service.go`, constructed with the ports it needs, and is the only place business rules are encoded.

- **`agent.Catalog`** (thin, wraps `ports.AgentCatalog` + `agent.Merge`): `Discover(projectDir string) ([]Agent, []error)`.
- **`pipeline.Store`**: `List()`, `Save(name string, agents []string) error` (validates non-empty name, uniqueness via `Store.PipelineNameTaken`, resolves `agents` against a provided catalog, timestamps, delegates write to `Store`), `Load(name string) (Pipeline, error)`.
- **`session.Service`**: the biggest one.
  - `Create(req CreateSessionInput) (Session, error)`: allocates UUID, resolves working dir (herdr pane's `cwd`), computes `layout.Plan`, drives the herdr executor (section 5.2) to realize the pane grid, writes `requirement.md`, builds the initial `Steps` slice (`Status: StepWaiting` for all, flips step 0 to `StepRunning` with `StartedAt`), starts the notify socket listener (`ports.NotifyServer.Listen`), persists `session.json` and `history.json`, sends the first prompt.
  - `HandleDone(evt NotifyMessage) error`: resolve step by `PaneID`, set `StepDone`/`FinishedAt`, append a `HistoryEntry` (best-effort `herdr.ReadAgent` tail capture), if there is a next step: write a handoff prompt (same template as section 5.2, `STEP` incremented) and `PromptAgent`+`SendKeys` it, flip its status to `StepRunning`/`StartedAt`; if there was no next step: `Session.Status = SessionDone`. Persist.
  - `HandleQuestion(evt NotifyMessage) (InteractionFile, error)`: resolve step by `PaneID`, set `StepNeedsInput`, load that step's interactions file via `interaction.Service`, persist, return the file for the TUI to render.
  - `PollForCrashes(tabID string) ([]int, error)`: compare live `herdr pane list` output against every step currently `StepRunning` or `StepNeedsInput`; any step whose `PaneID` is missing (or whose `ExplainAgent` reports a crashed/closed state) is flipped to `StepFailed`, and - per analyst resolution #11 - `Session.Status` is set to `SessionFailed` and no further steps are activated. Returns the indices that changed so the TUI can surface which row failed.
- **`interaction.Service`**: `Load(sessionID, file string) (InteractionFile, error)`, `SubmitAnswers(sessionID, file string, answers map[string]string) (InteractionFile, error)` (merges each answer into the matching `Question.Answer`, write-temp-then-rename back to disk), and `ResumePrompt(agentName, file string) string` (the fixed template: *"Your questions have been answered. Read `interactions/<file>.json` and continue."*) which `session.Service` sends via `PromptAgent`+`SendKeys` once all pending questions are answered.
- **`graphify.Service`**: `ListGraphs() ([]string, error)` (directory names under `~/.monday/graphs`), `Inject(label, projectRoot string) (symlinkPath string, err error)` (refuses if `projectRoot/graphify-out` already exists and is not a symlink Monday itself would own - surfaces a clear conflict error instead of overwriting a real folder), `RequestUpdate(session Session) error` (sends a `/graphify update .` prompt into the **last** pipeline step's pane via `PromptAgent`+`SendKeys` - see section 8.4 for why this is the chosen mechanism), `Cleanup(symlinkPath string) error` (verifies `os.Lstat` mode has `ModeSymlink` before removing - never deletes a real directory).

---

## 7. BubbleTea TUI Design

### 7.1 Root model and screen routing

```go
// internal/tui/app/model.go
type Screen interface {
    Init() tea.Cmd
    Update(tea.Msg) (Screen, tea.Cmd)
    View() string
}

type ScreenID int

const (
    ScreenToolCheck ScreenID = iota
    ScreenLaunch
    ScreenAgentPicker
    ScreenPipelineOrder
    ScreenRequirement
    ScreenGraphifyPrompt
    ScreenProgress
)

type switchScreenMsg struct {
    to      ScreenID
    payload any
}

type Model struct {
    services *app.Services
    theme    theme.Theme
    width, height int
    active   Screen
    activeID ScreenID
    banner   string // transient error banner, cleared on next keypress
}
```

`Model.Update` first intercepts global messages (`tea.WindowSizeMsg` -> store size, forward to active screen; `ctrl+c`/`q` at top level where the active screen allows quitting; `switchScreenMsg` -> construct the next screen with `payload`, call its `Init()`), otherwise delegates to `m.active.Update(msg)` and replaces `m.active` with whatever is returned. This is the standard "model-per-screen with a router" pattern and is the only place screen transitions happen - no screen ever imports another screen's package, they only ever return a `switchScreenMsg` wrapped in a `tea.Cmd`.

The **Q&A view is not a separate `ScreenID`** - it renders as a full-screen overlay owned by the Progress screen (`progressModel.qa *qaModel`, nil when closed), because it is only ever reachable from, and always returns to, the progress view, and it needs the progress screen's live session state underneath it to resume correctly. This avoids a duplicate "go back to progress" transition path.

### 7.2 Theme (`internal/tui/theme`)

```go
const (
    Rosewater = lipgloss.Color("#f5e0dc")
    Flamingo  = lipgloss.Color("#f2cdcd")
    Pink      = lipgloss.Color("#f5c2e7")
    Mauve     = lipgloss.Color("#cba6f7")
    Red       = lipgloss.Color("#f38ba8")
    Peach     = lipgloss.Color("#fab387")
    Yellow    = lipgloss.Color("#f9e2af")
    Green     = lipgloss.Color("#a6e3a1")
    Teal      = lipgloss.Color("#94e2d5")
    Sky       = lipgloss.Color("#89dceb")
    Blue      = lipgloss.Color("#89b4fa")
    Lavender  = lipgloss.Color("#b4befe")
    Text      = lipgloss.Color("#cdd6f4")
    Subtext0  = lipgloss.Color("#a6adc8")
    Overlay0  = lipgloss.Color("#6c7086")
    Surface0  = lipgloss.Color("#313244")
    Base      = lipgloss.Color("#1e1e2e")
    Mantle    = lipgloss.Color("#181825")
)

var (
    StatusColor = map[string]lipgloss.Color{
        "waiting":     Overlay0,
        "running":     Blue,
        "needs_input": Yellow,
        "done":        Green,
        "failed":      Red,
    }
)
```

Shared components in `internal/tui/components`: `Badge(status string) string`, `Card(title, body string, focused bool) string`, `Button(label string, focused bool) string`, `ProgressRow(step session.PipelineStep, elapsed time.Duration, selected bool) string`, `HelpBar(bindings ...string) string` (every screen ends with a one-line dim help bar, e.g. `↑/↓ move · enter select · q quit` - this single habit is what makes the whole app "idiot friendly": the user is never on a screen without being told what the keys do).

### 7.3 Per-screen design

Every screen: full-height layout, ASCII-art or heading at top in `Mauve`/`Lavender`, content centered, a one-line help bar pinned at the bottom in `Overlay0`. No screen requires the mouse. No screen requires memorizing a key not shown on screen.

#### Tool Check (`screens/toolcheck`)

- **Model**: `{missingHerdr, missingClaude, notInHerdrPane bool}`
- **Init**: runs `toolcheck.Run()` as a `tea.Cmd`, which execs `which herdr` / `which claude` (or `exec.LookPath`) and the current-pane detection from section 5.2.
- **Messages**: `toolCheckResultMsg{missingHerdr, missingClaude, notInHerdrPane bool}`.
- **View (failure state)**: big `Red` heading "Monday needs a couple of things first", one bullet per missing requirement with the exact install hint (`brew install herdr` / see herdr docs; `npm install -g @anthropic-ai/claude-code` style line for claude), and if `notInHerdrPane`: "Launch Monday from inside an active herdr pane." No further navigation is possible except `r` (re-check) and `q` (quit) - this is deliberate per the analyst report ("no further navigation until resolved").
- **View (success)**: never actually shown - on success this screen immediately emits `switchScreenMsg{ScreenLaunch}` from `Update` the moment the result message arrives, so the user never sees a flash of a "you're all set" screen.

#### Launch (`screens/launch`)

- **Model**: `{pipelines []pipeline.Pipeline, cursor int, loading bool}`.
- **Init**: `tea.Cmd` to list saved pipelines.
- **Messages**: `pipelinesLoadedMsg{pipelines []pipeline.Pipeline, err error}`.
- **View**: ASCII `MONDAY` banner (rendered once via a static multi-line string constant, `Mauve`, centered). If `len(pipelines) > 0`: a "Saved pipelines" section listing each by name with its agent count and last-updated date, cursor-highlighted. Always, directly below (or alone if no saved pipelines): a single focusable `Button("Create an orchestration")`. The whole screen is one list: saved pipelines first, "Create an orchestration" always last - up/down cycles through all of it, enter activates whichever row is highlighted.
- **Update**: `enter` on a saved pipeline row -> load agent catalog (needed later to resolve pane spawn), `switchScreenMsg{ScreenRequirement, payload: pipelineRequirementPayload{pipeline: selected}}` (skips agent picker and ordering, per analyst resolution #4). `enter` on "Create an orchestration" -> load agent catalog -> `switchScreenMsg{ScreenAgentPicker}`.

#### Agent Picker (`screens/agentpicker`)

- **Model**: `{agents []agent.Agent, cursor int, selected map[int]bool}`.
- **View**: one row per discovered agent: `[x] name - description` (description dimmed, truncated to terminal width with `...`), a project-sourced agent gets a small `(project)` tag in `Peach` so users can tell it overrides a global one of the same name.
- **Update**: `up/down` move cursor, `space` toggles `selected[cursor]`, `enter` requires `len(selected) >= 1` (otherwise sets a transient "pick at least one agent" banner) then builds the initial sequence (selected agents in catalog order) and emits `switchScreenMsg{ScreenPipelineOrder, payload: initialSequence}`.

#### Pipeline Ordering (`screens/pipelineorder`)

- **Model**: `{available []agent.Agent, sequence []string, cursor int, nameInput textinput.Model, savePromptOpen bool}`. `sequence` is the list of agent **names** in run order; it is the thing being built, independent from the `available` set.
- **View**: numbered vertical list, `1. architect`, `2. developer`, ... with the highlighted row shown in `Blue`. Help bar: `↑/↓ move cursor · shift+↑/↓ reorder · a add again · d remove · s save as... · enter continue`.
- **Update**:
  - `up/down`: move cursor within `sequence`.
  - `shift+up` / `shift+down`: swap `sequence[cursor]` with its neighbor, cursor follows.
  - `a`: append another copy of the currently highlighted agent name to the end of `sequence` - this is how the same agent can appear more than once in the chain (analyst resolution #3), modeled as an explicit, visible action rather than a hidden multi-select count.
  - `d`: remove the highlighted entry, as long as `len(sequence) > 1`.
  - `s`: opens an inline name prompt (`textinput.Model`, focused); `enter` while it is open validates non-empty and not-already-taken (service call) - on conflict shows "a pipeline named X already exists" inline rather than silently overwriting (analyst resolution #4), on success calls `pipeline.Store.Save` and closes the prompt with a confirmation toast, staying on the same screen so the user can still continue to the requirement screen afterward.
  - `enter` (name prompt closed): emits `switchScreenMsg{ScreenRequirement, payload: pipelineRequirementPayload{pipeline: adHocPipelineFrom(sequence)}}`. Saving is optional - you can continue unsaved.

#### Requirement Entry (`screens/requirement`)

- **Model**: `{pipeline pipeline.Pipeline, textarea textarea.Model, focusOnButton bool}`.
- **View**: shows the resolved pipeline as a breadcrumb (`architect -> developer -> reviewer`) above a large bordered `textarea`, a `Button("Submit")` below it.
- **Update**: `tab` toggles focus between textarea and button; while textarea focused, all keys go to it (including literal enter for newlines); `enter` while the button is focused (or `ctrl+s` at any time) validates non-empty trimmed text, then emits `switchScreenMsg{ScreenGraphifyPrompt, payload: {pipeline, requirement}}`.

#### Graphify Prompt (`screens/graphifyprompt`)

- **Model**: `{phase enum{askYesNo, pickGraph}, choice bool, graphs []string, cursor int, pipeline, requirement}`.
- **View (askYesNo)**: "Inject a saved graphify-out into this project before starting? [ Yes ] [ No ]" with left/right toggling which button is highlighted.
- **Update (askYesNo)**: `enter` with `No` highlighted -> `switchScreenMsg{ScreenProgress, payload: progressPayload{..., graph: ""}}`. `enter` with `Yes` highlighted -> load `graphify.ListGraphs()`, move to `pickGraph`.
- **View (pickGraph)**: list of labels found under `~/.monday/graphs`. If empty: "No saved graphify-out folders found under ~/.monday/graphs yet." plus a `Back` option (never a dead end).
- **Update (pickGraph)**: `enter` on a label -> `switchScreenMsg{ScreenProgress, payload: progressPayload{..., graph: label}}`.

#### Stepped Progress (`screens/progress`) - the primary "idiot friendly" screen

- **Model**:
```go
type progressModel struct {
    session   session.Session
    events    <-chan notifyproto.NotifyMessage
    now       time.Time
    spinner   spinner.Model
    qa        *qaModel // nil when no overlay is open
    err       string
}
```
- **Init**: issues the session creation command (if arriving from Requirement/Graphify screens) or loads an existing one, starts the 1-second tick (`tea.Tick`), starts `listenForNotify(events)`.
- **View**: one `ProgressRow` per step, top to bottom in pipeline order:

```
┌─ MONDAY - pipeline running ───────────────────────────────┐
│                                                            │
│  1. architect     ● running       drafting the plan   00:42 │
│  2. developer      waiting                                 │
│  3. reviewer       waiting                                 │
│                                                            │
│  ⠋ waiting on architect...                                │
│                                                            │
│  enter: open questions (when a row needs input) · q: quit  │
└────────────────────────────────────────────────────────────┘
```
  Status badge text + color per `theme.StatusColor`; a filled dot/spinner glyph for `running`, a check for `done`, a cross for `failed`, a `?` badge in `Yellow` plus "- press enter to answer" inline hint for `needs_input` (so the user is never left wondering why a row stopped moving). The elapsed column is `now.Sub(*StartedAt)` truncated to seconds, blank until a step starts, frozen at `FinishedAt-StartedAt` once done/failed.
- **Update**:
  - `tickMsg`: update `now`; every 3rd tick additionally fires `pollForCrashesCmd(tabID)`.
  - `notifyEventMsg{DONE}`: call `session.Service.HandleDone`, replace `m.session`, re-issue `listenForNotify`.
  - `notifyEventMsg{QUES}`: call `session.Service.HandleQuestion` to get the `InteractionFile`, construct `m.qa = newQAModel(file)`, re-issue `listenForNotify`.
  - `crashCheckResultMsg{failedIndices []int}`: replace `m.session`, if `m.session.Status == SessionFailed` render a terminal failure banner naming the step that crashed, no further action possible except `q`.
  - `enter` with no overlay open and the current step is `needs_input`: open `m.qa` directly (covers the user dismissing the auto-popup or missing it).
  - When `m.qa != nil`, every key is forwarded to `m.qa.Update` first; a `qaSubmittedMsg` closes the overlay and flips the step back to `running`.
  - Once `Session.Status` becomes `SessionDone`: show the `GraphifyPrompt`-style confirmation inline ("Pipeline finished. Update the graphify graph with the new context? [ Yes ] [ No ]") only if `session.GraphifyGraph != ""`; `Yes` calls `graphify.Service.RequestUpdate` then shows "Press enter once the graph update finishes in <agent>'s pane" before calling `graphify.Service.Cleanup`; `No` or no graph injected skips straight to a plain "Pipeline complete" state with `q` to exit (panes are left running per the no-destructive-side-effects NFR).

#### Q&A Overlay (`screens/qa`, embedded in progress)

- **Model**:
```go
type qaModel struct {
    agentName string
    file      interaction.InteractionFile
    cursor    int                // which question, 0-based
    optCursor int                // which option row, last index = "Other"
    other     textinput.Model
  typingOther bool
    answers   map[string]string  // question id -> answer text, filled as user confirms each
}
```
- **View**: "Question 2 of 4" progress dots at top, the question text + optional description, a vertical list of its `Options` plus a trailing `Other (type your own)` row, the currently selected row highlighted in `Mauve`; when `typingOther` is true the row expands into a focused `textinput`. A bottom summary line shows how many of the file's questions are answered so far, and the `Button("Submit answers")` only becomes enabled/highlightable once every question has a non-empty answer.
- **Update**: `up/down` move `optCursor` (wraps, includes the Other row); `left/right` move between questions (`cursor`) without losing already-chosen answers; `enter` on a normal option -> `answers[question.ID] = option text`, auto-advance to the next unanswered question; `enter` on the Other row -> `typingOther = true`, focuses `other`; `enter` again while typing -> commits `other.Value()` as the answer, unfocuses; `enter` on the Submit button (only reachable once all answered) -> calls `interaction.Service.SubmitAnswers` then `interaction.Service.ResumePrompt`-driven `PromptAgent`/`SendKeys`, returns `qaSubmittedMsg` up to the progress screen. `esc` closes the overlay without submitting (answers already picked are kept in memory, not lost, in case the user reopens via `enter` on the progress screen before the agent times out - there is no server-side timeout to race against since the agent is simply idle, not polling).

---

## 8. End-to-end flow walkthroughs

### 8.1 Happy path, 3-agent ad hoc pipeline

1. User runs `monday` inside a herdr pane. `cmd/monday/main.go` builds `Services`, launches `tea.NewProgram`.
2. Tool Check passes silently -> Launch screen. No saved pipelines yet, so only "Create an orchestration" shows.
3. Agent Picker -> user selects architect, developer, reviewer. Pipeline Ordering -> already in that order, user presses `enter` without saving.
4. Requirement Entry -> user types the task, submits.
5. Graphify Prompt -> user picks "No".
6. Progress screen `Init` calls `session.Service.Create`: splits a work-area pane off Monday's own pane, plans a 1-row-of-3 grid, starts all three `claude --agent` processes idle, writes `requirement.md`, prompts architect only, persists `session.json`/`history.json`, opens the notify socket.
7. Architect works, eventually runs `monday notify --session <id> --pane <architect-pane> --type DONE`. The notify CLI dials the socket, sends `NotifyMessage{DONE,...}`, gets `{"ok":true}` back, exits 0.
8. Progress screen receives `notifyEventMsg{DONE}` -> `HandleDone` marks architect `done`, prompts developer, flips developer to `running`.
9. Developer needs input mid-task: writes `interactions/developer.json` with its questions, runs `monday notify ... --type QUES`, then stops/ends its turn (does **not** poll).
10. Progress screen receives `notifyEventMsg{QUES}` -> loads the file, opens the Q&A overlay automatically. User answers all questions (including one typed "Other").
11. Submit -> answers merged back into `interactions/developer.json`, Monday sends `PromptAgent(developerPane, "Your questions have been answered. Read interactions/developer.json and continue.")` + `SendKeys(enter)`, step flips back to `running`.
12. Developer finishes, notifies `DONE` -> reviewer is prompted. Reviewer finishes, notifies `DONE` -> no next step -> `Session.Status = SessionDone`.
13. Progress screen shows "Pipeline complete", no graphify prompt (none was injected), `q` to exit. All three panes remain open in herdr.

### 8.2 Saved pipeline shortcut

Launch screen -> user picks a saved pipeline by name -> straight to Requirement Entry (Agent Picker and Pipeline Ordering are skipped entirely, per analyst resolution #4) -> rest identical to 8.1.

### 8.3 Pane crash mid-step

While developer's step is `running`, the user accidentally closes that pane in herdr. The next crash-check tick's `herdr pane list` no longer contains that pane ID. `PollForCrashes` flips that step to `failed`, `Session.Status = failed`, no further steps are activated (architect's and reviewer's panes, if any were already running, are left exactly as they are). Progress screen renders the failed row in `Red` with a "pane closed unexpectedly" note and stops offering any further Q&A interaction for that session.

### 8.4 Graphify inject + cleanup

Graphify Prompt (Yes) -> user picks "payment-portal" -> `graphify.Service.Inject("payment-portal", session.WorkingDir)` creates `<workingDir>/graphify-out -> ~/.monday/graphs/payment-portal`, recorded on `Session.GraphifyGraph`/`GraphifySymlink`. Pipeline runs as normal; agents that check for the symlink (per the skill, section 9) use `graphify query` while working. On `SessionDone`, Monday asks for confirmation (never automatic, per analyst resolution #8); on "Yes", `RequestUpdate` sends `/graphify update .` into the **last** step's pane (chosen because it is the one guaranteed to already be `claude`, already sitting in the project directory, and already idle/done - there is no documented way for a standalone Go binary to invoke a Claude Code skill directly, so Monday always delegates skill invocation to an already-running `claude` process via the same prompt mechanism used everywhere else, rather than inventing a second mechanism). Monday shows "Press enter once the graph update finishes" (manual confirmation, not another notify type, to keep the protocol to exactly two message kinds), then `Cleanup` removes the symlink after the user confirms.

---

## 9. The `monday` Claude skill

Written verbatim (modulo the exact session-specific values an agent fills in) by `internal/infra/skillwriter` to `~/.claude/skills/monday/SKILL.md`:

```markdown
---
name: monday
description: "Use when running as one step inside a Monday pipeline (you were started with `claude --agent <name>` from a Monday-managed herdr pane). Explains how to notify Monday when you finish or need input, where to save generated documents, the interactions question/answer file format, and how to use an injected graphify-out graph."
---

# monday

You are running as one agent in a Monday pipeline. Monday started you in this pane, gave you a SESSION_ID, PANE_ID, and AGENT_NAME in your first prompt, and is watching for exactly two signals from you: that you are done, or that you need the user to answer something.

## Where your task lives

- `~/.monday/sessions/<SESSION_ID>/docs/requirement.md` - the original user requirement for the whole pipeline.
- `~/.monday/sessions/<SESSION_ID>/history.json` - what earlier steps in the pipeline already did.
- `~/.monday/sessions/<SESSION_ID>/docs/` - save every document you generate here (plans, specs, reports). Do not write project documents anywhere else on disk outside the project itself.

## When you finish your part

Run, in your own pane's shell:

```
monday notify --session <SESSION_ID> --pane <PANE_ID> --type DONE
```

Then stop. Do not keep working, do not poll for anything - Monday will hand off to the next agent in the pipeline, or end the pipeline if you were the last step.

## When you need the user to answer something

1. Write your questions to `~/.monday/sessions/<SESSION_ID>/interactions/<AGENT_NAME>.json` using exactly this shape:

```json
{
  "agent": "<AGENT_NAME>",
  "questions": [
    {
      "id": "short-stable-id",
      "question": "The question, as a short title",
      "description": "Optional extra context for the user",
      "options": ["Option one", "Option two"]
    }
  ]
}
```

   `options` is optional - Monday always lets the user type a free-text answer in addition to any options you provide, so do not add your own "Other" option.

2. Run:

```
monday notify --session <SESSION_ID> --pane <PANE_ID> --type QUES
```

3. Stop and wait. Do not sleep-loop, do not re-read the file yourself on a timer. Monday will show your questions to the user, write each answer back into the same file as an `"answer"` field on the matching question object, and then send you a new prompt telling you the file has been answered.

4. When you receive that prompt, read `~/.monday/sessions/<SESSION_ID>/interactions/<AGENT_NAME>.json` again, use the `answer` field of each question, and continue your work.

## If a `graphify-out` symlink is present

Before you start, check whether `graphify-out` exists in the project root and is a symlink (`ls -la graphify-out` or `test -L graphify-out`). If it is, a knowledge graph for this project has been injected for you by Monday:

- Use `graphify query "<question>"` for broad questions about the codebase instead of grepping around blind.
- Use `graphify path "<A>" "<B>"` for how two things relate, and `graphify explain "<concept>"` for a focused explanation.
- Do not create, move, or delete the `graphify-out` symlink yourself - Monday manages it and removes it after the pipeline finishes and the graph has been updated.
```

This is the only place the skill's content is authored. `skillwriter` embeds this exact markdown as a Go string constant (via `//go:embed skill.md` pointing at a file checked into the repo, so it can be edited without touching Go code) and writes it unmodified - no templating of session-specific values into the installed file itself, since the session-specific `SESSION_ID`/`PANE_ID`/`AGENT_NAME` values are supplied per-run in the initial prompt text (section 5.2), not baked into the skill file.

---

## 10. Challenges and Mitigations

| # | Challenge | Mitigation |
|---|---|---|
| 1 | Discovering Monday's own pane/tab ID - no documented API in `herdr.md` | `HERDR_PANE_ID`/`HERDR_TAB_ID` env var check, then `focused:true` pane from `herdr pane list`, then a clear fatal error. Implemented once in `toolcheck`, reused everywhere. |
| 2 | `herdr pane split` / `agent start` JSON shape undocumented | Direct-parse attempt with a before/after `pane list` diff as the always-available fallback; validated against the real binary in build step 3 before any screen is built on top of it. |
| 3 | Detecting a crashed/closed pane - herdr has no push-based crash event | Polling `herdr pane list` on a slow sub-tick (every 3rd second-tick) of the same `tea.Tick` already driving the elapsed timers, so no extra goroutine or timer is introduced. |
| 4 | Concurrent writers to `session.json`/interactions files (socket goroutine vs. UI loop) | Single-writer rule: the socket only ever pushes events onto a channel; every file mutation happens inside `Update`, which BubbleTea guarantees is single-threaded. |
| 5 | The analyst's interactions filename (`interactions/<agent_name>.json`) collides if the same agent runs twice in one pipeline | Deviation, called out explicitly: suffix repeats as `<agent_name>-2.json`, `<agent_name>-3.json`, computed once at session creation and stored on `PipelineStep.InteractionsFile`; the exact filename is told to the agent in its initial prompt, so it never has to guess. |
| 6 | Shell-escaping a multi-line requirement or agent output into a herdr prompt command | Never inline large text - write it to `docs/requirement.md` once, send only short fixed-template single-line prompts referencing the path. |
| 7 | Stale Unix socket file from a previous crashed run | Each session gets a fresh UUID-namespaced socket path, but still defensively `os.Remove` (ignore `ENOENT`) immediately before `ListenUnix`. |
| 8 | Nothing stops a user from starting two Monday sessions at once, though the NFR says one at a time | On launch, scan `~/.monday/sessions/*/session.json` for `Status: running`; if found, show a blocking warning screen rather than silently allowing a second session (resuming that session is explicitly out of scope for MVP, so this is a hard stop, not a resume prompt). |
| 9 | Malformed `.claude/agents/*.md` frontmatter | Parse failures are collected as warnings and the file is skipped, never a fatal error for the whole discovery pass. |
| 10 | No documented mechanism for a Go binary to invoke a Claude Code skill directly (for the post-pipeline graphify update) | Delegate by sending a normal prompt (`/graphify update .`) into the last step's already-running `claude` pane, then a manual "press enter once it's done" confirmation - reuses the exact same herdr prompting mechanism as every other hand-off, no new integration surface. |
| 11 | Terminal resize / very small terminal | Root model owns `width`/`height` from `tea.WindowSizeMsg` and passes them to the active screen; every screen's `View` is a `lipgloss` layout computed from current size, not a fixed string; below a minimum usable size (e.g. 60x15) every screen falls back to a single "terminal too small" message instead of a garbled render. |

---

## 11. Dependencies and Integration Points

- Go modules: `github.com/charmbracelet/bubbletea`, `github.com/charmbracelet/bubbles` (list, textinput, textarea, spinner), `github.com/charmbracelet/lipgloss`, `github.com/google/uuid`, `gopkg.in/yaml.v3`. No HTTP framework, no CLI framework (stdlib `flag`/`os.Args` is enough for the two entry points) - keeps the dependency surface minimal per the "prioritize simplicity" principle.
- External binaries: `herdr` (all pane/tab/agent orchestration) and `claude` (`claude --agent <name>`, or `claude` started via `herdr agent start ... --kind claude`) - both checked once at startup only, both invoked exclusively through `internal/infra/herdr`.
- OS integration points: `~/.monday` (state), `~/.claude/agents` + `<project>/.claude/agents` (agent discovery, read-only), `~/.claude/skills/monday/SKILL.md` (write), `<project-root>/graphify-out` (symlink, created and removed only, never written into), Unix domain sockets (macOS and Linux both support `net.ListenUnix` identically, which is part of why this design is platform-safe without conditional code).
- Internal integration points: the `monday notify` subcommand is a separate process invocation from the long-running TUI process - its only coupling to the TUI is the socket path and the shared `notifyproto` schema; it must work even if invoked by an agent whose shell has a totally different working directory, so it takes the socket path purely from `--session` (`~/.monday/sessions/<session>/monday.sock`), never from an inherited env var.

---

## 12. Testing Strategy

- **Domain**: table-driven unit tests with no IO - `layout.Plan` against the documented examples (3, 4, 5, 6, 7 agents), `pipeline.Validate`, `agent.Merge`, `session.Service.HandleDone`/`HandleQuestion` against an in-memory fake `Store`/`HerdrClient`/`NotifyServer`.
- **Infra**: `fsstore` tests run against `t.TempDir()`; `socket` tests run a real `net.Listen("unix", ...)` in `t.TempDir()` and a real client dial, asserting both the happy path and a malformed-message path produces `{"ok":false}`; `herdr` client tests use a fake `exec.Command` shim (an interface around command execution, injected in tests) so they do not require the real binary, plus a short manual/integration checklist (run against the real `herdr` binary once, by hand, during build step 3) to lock in the undocumented response shapes.
- **TUI**: `teatest` (bubbletea's test harness) driving each screen's `Update` through scripted key sequences, asserting the resulting `switchScreenMsg` and rendered `View()` snapshots per screen; no end-to-end TUI test attempts to drive real herdr panes - that is validated manually against a real herdr session before considering the feature done.

---

## 13. Ordered Implementation Plan

Each step should land as its own commit/PR, buildable and (where it has domain logic) tested in isolation, before the next step begins. This order deliberately front-loads the two biggest integration unknowns (herdr's undocumented response shapes, and the socket protocol) so that every screen built afterward is wired against infrastructure already proven to work.

1. **Scaffold & shared domain types.** Create the package layout from section 2, `go.mod` deps, every struct from section 3 with `json` tags, `ports.go`. No behavior yet, just types that compile. Exit criteria: `go build ./...` succeeds, `go vet` clean.
2. **Filesystem store.** Implement `infra/fsstore` against the layout in section 5.1, including write-temp-then-rename for every write. Unit tests in `t.TempDir()` covering bootstrap, save/load pipeline, duplicate-name detection, session/history/interactions round-trips.
3. **herdr client + manual validation.** Implement `infra/herdr` per section 5.2, including the pane-split diff fallback. Before building anything on top of it, manually run it by hand inside a real herdr pane to confirm: current-pane detection, `pane split` behavior, `agent start` vs. `pane run` behavior, `agent prompt` + `send-keys` round trip, `agent explain`/`pane list` crash signal. Record findings as code comments next to whichever path won. Exit criteria: a small throwaway `go run` harness can split a pane, start a claude agent in it, and prompt it, observed manually.
4. **Agent discovery.** Implement `infra/claudeagents` (frontmatter parsing, global+project merge) and `domain/agent`. Unit tests with fixture `.md` files including malformed ones.
5. **Pipeline domain service.** `domain/pipeline` validation + `pipeline.Store` wired to `fsstore`. Unit tests.
6. **Pane layout calculator.** `domain/layout.Plan` exactly per section 5.2a, unit tests against the documented examples (3/4/5/6/7 agents) before it is ever called by anything that touches real panes.
7. **Session domain service, no sockets yet.** `domain/session.Service.Create`/`HandleDone`/`HandleQuestion`/`PollForCrashes` wired to the herdr client and fsstore, driven by a throwaway CLI harness (not the TUI yet) to confirm a full pipeline can be spawned, advanced, and polled end to end against real herdr/claude. This is the highest-risk integration point - keep iterating here until a 2-3 agent pipeline reliably spawns, hands off, and detects a manually-closed pane as failed.
8. **Notify socket + `monday notify` subcommand.** `infra/socket` server+client, `domain/notifyproto`, wire `cmd/monday`'s `notify` branch, and connect the socket's event channel into `session.Service` via the `NotifyRouter`. Exit criteria: from a second terminal, running `monday notify --session <id> --pane <id> --type DONE` against the harness from step 7 advances the pipeline.
9. **Interaction service.** `domain/interaction.Service` (load/submit/resume-prompt), wired into the step-7 harness so a manually-written `interactions/<agent>.json` can be answered via a scripted call and the resume prompt observed landing in the real pane.
10. **Graphify service.** `domain/graphify.Service` (list/inject/request-update/cleanup) against `~/.monday/graphs` fixtures in a temp home dir; manual confirmation of the symlink appearing/disappearing in a scratch project directory.
11. **TUI shell.** `tui/theme`, `tui/components`, root `tui/app.Model`/`Screen` interface/`switchScreenMsg` routing, wired in `cmd/monday/main.go` with a single placeholder screen, to prove the router and theme compile and render before any real screen exists.
12. **Tool Check screen**, wired to the real `toolcheck`/`herdr` current-pane detection from step 3.
13. **Launch screen**, wired to `pipeline.Store.List`.
14. **Agent Picker screen**, wired to `agent.Catalog.Discover`.
15. **Pipeline Ordering screen**, including the save-as-name flow wired to `pipeline.Store.Save` and its duplicate-name rejection.
16. **Requirement Entry screen.**
17. **Graphify Prompt screen**, wired to `graphify.Service.ListGraphs`/`Inject`.
18. **Stepped Progress screen**, wired to `session.Service` (steps 7 and 8's harness logic now driven by real UI instead of a throwaway harness) plus the elapsed-timer tick and crash-poll tick.
19. **Q&A overlay**, wired to `interaction.Service`, embedded in the Progress screen.
20. **Claude skill writer.** `infra/skillwriter` with the embedded `skill.md` from section 9, invoked once from `cmd/monday/main.go` bootstrap (before the TUI starts, so the file is always current even if the user never completes a pipeline).
21. **Polish pass.** Empty states (no saved pipelines, no saved graphs, zero agents discovered), minimum-terminal-size fallback, consistent help bars across every screen, the "another Monday session is already running" guard from challenge #8, and a final manual end-to-end run of the full happy path (8.1), the saved-pipeline shortcut (8.2), a forced pane crash (8.3), and a full graphify inject/update/cleanup cycle (8.4).

---

## 14. MVP Scope Recap

Carried verbatim from `docs/analyst-report.md` section 5 - this plan implements exactly that boundary, nothing more:

**In scope**: `~/.monday` bootstrap, `herdr`/`claude` presence + in-pane checks, agent discovery, linear pipeline builder with save/load, row-based pane spawning through herdr, session + interactions persistence, `monday notify` over a Unix socket, graphify injection with a project-root symlink, the `monday` Claude skill file, catppuccin mocha theming throughout.

**Out of scope**: branching/parallel pipelines, editing/deleting a saved pipeline from the UI, resuming a session after a crash, Windows support, running more than one Monday session at a time, live cost/token tracking, automatic retry of a failed step.
