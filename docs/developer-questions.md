# Developer questions / deviations log

Non-blocking notes raised while implementing against `docs/architect-plan.md`.
Nothing here stops implementation; recorded for the architect/user to confirm
or override later.

## 1. `herdr agent start` needs a trailing `-- --agent <name>` to launch a specific persona

Manually validated against the real `herdr` binary (build step 3, 2026-10-22):

```
herdr agent start <name> --kind claude --pane <pane_id>
```

by itself launches a **bare** `claude` REPL (`argv: ["claude"]`) - the leading
`<name>` positional is only a herdr-side display label, it is not forwarded
to the launched binary. To actually start `claude --agent <name>`, herdr's
`agent start` subcommand needs the agent-binary args appended after `--`:

```
herdr agent start <name> --kind claude --pane <pane_id> -- --agent <name>
```

Confirmed via `argv: ["claude","--agent","architect"]` in the response and a
matching `✳ architect` terminal title, consistent with how the sibling
agents in this very orchestration session (analyst/architect/reviewer) are
titled. `internal/infra/herdr.Client.StartAgent` uses this form. The
`herdr.md` reference doc does not mention the trailing `--` at all - this is
the single most important correction from manual validation.

## 2. Error envelope travels on stderr, not stdout

Every `herdr` subcommand prints `{"id": "...", "result": {...}}` to **stdout**
on success (exit 0), and `{"id": "...", "error": {"code": "...", "message": "..."}}`
to **stderr** on failure (non-zero exit). `herdr.md` does not document the
error shape at all. `internal/infra/herdr` decodes stdout on exit 0 and
stderr on non-zero exit.

## 3. `pane split` response shape

Confirmed: `{"result": {"pane": {...PaneInfo fields..., "pane_id": "wX:pY"}, "type": "pane_info"}}`.
Not `result.pane_id` or `result.new_pane_id` as guessed in architect-plan
section 5.2 - the new pane is nested under `result.pane`. Direct-parse
against this exact shape is implemented as the primary path; the
before/after `pane list` diff described in the plan is kept as a secondary
fallback in case a future herdr version changes the shape, but was not
needed in practice.

## 4. `agent explain --json` is not enveloped

Unlike every other subcommand, `herdr agent explain --json --format json <pane_id>`
prints the detection-state object directly at the top level on stdout (no
`{"id","result"}` wrapper). `internal/infra/herdr` decodes it with its own
dedicated (non-generic-envelope) struct.

## 5. Crash detection does not need `agent explain`

A closed/crashed pane simply disappears from `herdr pane list` (and
`herdr agent list`) entirely - `agent explain` against a gone pane returns
an `agent_not_found` error instead of a "crashed" state. `PollForCrashes`
therefore only needs a presence check against `pane list`, exactly as
section 5.2a / the plan's diff-based fallback already assumed; `ExplainAgent`
is kept on the port for optional richer in-place status display (idle /
working / blocked) of a pane that is still present, not for crash detection.

## 6. `CurrentPane()` dependency direction

Architect-plan section 5.2 says the env-var -> focused-pane -> fatal-error
triple fallback is "implemented as `toolcheck.CurrentPane()` and reused by
`herdr.Client.CurrentPaneID()`" - but `infra/toolcheck` is a step-12 (TUI)
component, while `herdr.Client.CurrentPaneID()` is needed starting at step 3
and is load-bearing for `session.Service.Create` (step 7), long before
step 12 exists. Implemented the triple fallback directly inside
`internal/infra/herdr.Client` (self-contained, no dependency on
`infra/toolcheck`), and `infra/toolcheck` (step 12) instead calls
`herdr.Client.CurrentPaneID()`/`CurrentTabID()` and turns a failure into the
"not in a herdr pane" UI state. Same externally-observable behavior, cleaner
dependency direction (a TUI-readiness checker depending on the herdr client
makes more sense than the reverse).

## 7a. `agent start` requires an agent name that resolves to a real `.claude/agents/*.md` persona

Tried `herdr agent start harness-agent --kind claude --pane <id> -- --agent harness-agent`
with a made-up name (no matching `.claude/agents/harness-agent.md`): `claude`
exits immediately on the unknown `--agent` value, and herdr's own readiness
check then fails with `agent_name_not_found: named agent harness-agent no
longer owns the target terminal`. `agent start` must always be called with a
name from the discovered agent catalog (`internal/domain/agent`), never an
arbitrary label - confirmed by re-running with `architect` (a real persona),
which succeeded end to end (split -> start -> explain idle -> prompt ->
send-keys -> agent responded and burned real context tokens).

## 7b. `pane run` also prints no JSON envelope on success

Same as `pane send-keys` (item above) - `herdr pane run <pane_id> <command...>`
produces empty stdout on success, not `{"id","result"}`. The `StartAgent`
`pane run` fallback path checks the exit code directly rather than attempting
to JSON-decode an empty body (an earlier version of this code tried to decode
it through the generic envelope helper and failed on every real invocation
during harness testing).

## 8. `agent start` can return before herdr's own agent registry is ready for `agent prompt`

Found during build-step-7 harness testing (spawning a real 2-agent pipeline
through `session.Service.Create`): calling `agent start` for one pane,
then `agent start` for a second pane, then immediately `agent prompt` on the
*first* pane's ID, failed with `agent_not_found: agent target <pane> not found`
- even though that first `agent start` call's own response already reported
`interactive_ready:true`. Checking `herdr pane list` moments later showed
both agents present and idle, confirming this is a transient race in
herdr's internal bookkeeping catching up, not a real failure.

Fix: `StartAgent` now follows every successful `agent start` with
`herdr agent wait <pane_id> --until idle --timeout 10000`, which blocks
until herdr's own registry agrees the agent is actually there. This closes
the race before `Create` loops on to the next step or calls `PromptAgent`.
Cheap and correct beats trusting `agent start`'s own response, since
`agent start`'s readiness claim is evidently not the same thing herdr's
later commands check against.

## 7. `pane list` has no `--tab` filter

Only `--workspace <id>`. `HerdrClient.ListPanes(tabID)` passes
`--workspace <workspace-id-parsed-from-tabID-prefix>` to narrow the call
server-side, then filters the result by `tab_id == tabID` client-side.
