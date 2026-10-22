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
