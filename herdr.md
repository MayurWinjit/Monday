# Herdr - A Terminal Multiplexer for Multiple AI Agent Session

## Fetching current scene

### Fetching Herdr workspaces 
```
herdr workspace list
```
It'll give you a json in which get the data for variable `workspaces` in dictionary for variable `result`.
Example response:
```
{
  "id": "cli:workspace:list",
  "result": {
    "type": "workspace_list",
    "workspaces": [
      {
        "active_tab_id": "w1E:t2",
        "agent_status": "idle",
        "focused": false,
        "label": "EPOS-1621",
        "number": 1,
        "pane_count": 4,
        "tab_count": 3,
        "workspace_id": "w1E"
      },
      {
        "active_tab_id": "w1H:t1",
        "agent_status": "unknown",
        "focused": true,
        "label": "dotfiles",
        "number": 2,
        "pane_count": 2,
        "tab_count": 2,
        "workspace_id": "w1H"
      }
    ]
  }
}
```
The workspace_id can be used to switch to particular worspace. Workspaces can be identified with label as well.

### Fetching Herdr Tabs 
```
herdr tab list
```
Example response:
```
{
  "id": "cli:tab:list",
  "result": {
    "tabs": [
      {
        "agent_status": "unknown",
        "focused": false,
        "label": "nvim",
        "number": 1,
        "pane_count": 1,
        "tab_id": "w1E:t1",
        "workspace_id": "w1E"
      },
      {
        "agent_status": "idle",
        "focused": false,
        "label": "claude",
        "number": 2,
        "pane_count": 2,
        "tab_id": "w1E:t2",
        "workspace_id": "w1E"
      }
    ],
    "type": "tab_list"
  }
}
```
The tabs shows created `tabs` in herdr plexer. The `tab_id` variable can be used to switch to particular

### Fetching agent list
```
herdr agent list
```
Example response:
```
{
  "id": "cli:agent:list",
  "result": {
    "agents": [
      {
        "agent": "claude",
        "agent_session": {
          "agent": "claude",
          "kind": "id",
          "source": "herdr:claude",
          "value": "53fbb527-8a9e-4871-89c5-e37104af9d15"
        },
        "agent_status": "idle",
        "cwd": "/Users/mayurm/Development/02_Ezeebit/01_web/02_web_portal_monorepo/worktrees/EPOS-1621",
        "focused": false,
        "foreground_cwd": "/Users/mayurm/Development/02_Ezeebit/01_web/02_web_portal_monorepo/worktrees/EPOS-1621",
        "pane_id": "w1E:p4",
        "revision": 3,
        "state_change_seq": 219,
        "tab_id": "w1E:t2",
        "terminal_id": "term_658e92c8ac5be42",
        "terminal_title": "✳ Claude Code",
        "terminal_title_stripped": "Claude Code",
        "workspace_id": "w1E"
      }
    ],
    "type": "agent_list"
  }
}
```
The `agents` & session values can be used to fetch session context & share the context between two sessions.

### Fetching Herdr Panes
```
herdr pane list
```
Example response:
```
{
"id": "cli:pane:list",
"result": {
  "panes": [
    {
      "agent_status": "unknown",
      "cwd": "/Users/mayurm/Development/02_Ezeebit/01_web/02_web_portal_monorepo/worktrees/EPOS-1621",
      "focused": false,
      "foreground_cwd": "/Users/mayurm/Development/02_Ezeebit/01_web/02_web_portal_monorepo/worktrees/EPOS-1621",
      "pane_id": "w1E:p1",
      "revision": 4,
      "scroll": {
        "max_offset_from_bottom": 0,
        "offset_from_bottom": 0,
        "viewport_rows": 100
      },
      "tab_id": "w1E:t1",
      "terminal_id": "term_658e8207076643d",
      "terminal_title": "nvim ~/D/0/0/0/w/EPOS-1621",
      "terminal_title_stripped": "nvim ~/D/0/0/0/w/EPOS-1621",
      "workspace_id": "w1E"
    }
  ],
  "type": "pane_list"
}
}
```
Panes are sections created inside a tab. Each pane can be identified by `pane_id`

## Switching focus
For switching workspace:
```
herdr workspace focus <workspace_id>
```
For switching tab:
```
herdr tab focus <tab_id>
```
For switching to an agent:
```
herdr agent focus <pane_id>
```
For switching to a pane:
```
herdr pane focus <pane_id>
```

## Managing agents
To read agent terminal output:
```
herdr agent read <pane_id>
```
To rename agent: 
```
herdr agent rename <pane_id> <name>
```
To send a prompt to an agent
```
herdr agent prompt <pane_id> <prompt_text>
```
After sending prompt, to execute the promot do this:
```
herdr pane send-keys <pane_id> enter
```
To Start a supported interactive agent in an existing pane
```
herdr agent start <name> --kind claude --pane <pane_id>
```
To explain agent detection state
```
herdr agent explain --json --format json <pane_id>
```
To send keys to an agent
```
herdr agent send-keys <pane_id> <key>
```

### Managing Claude Agents

To spawn a developer agent:
    1. Step 1 (Creating pane for the agent):
        `herdr pane split <pane_id> --direction right|down`
    2. Step 2 (spawning developer agent):
        `herdr pane run <pane_id> "claude --agent developer <prompt>"`

To spawn a reviewer agent:
    1. Step 1 (Creating pane for the agent):
        `herdr pane split <pane_id> --direction right|down`
    2. Step 2 (spawning developer agent):
        `herdr pane run <pane_id> "claude --agent reviewer <prompt>"`

To spawn a architect agent:
    1. Step 1 (Creating pane for the agent):
        `herdr pane split <pane_id> --direction right|down`
    2. Step 2 (spawning developer agent):
        `herdr pane run <pane_id> "claude --agent architect <prompt>"`


## Managing Panes
To split pane
```
herdr pane split <pane_id> --direction right|down
```
To close pane
```
herdr pane close <pane_id>
```
To release agent lifecycle authority
```
herdr pane release-agent --agent <label> <pane_id>
```
To read pane output
```
herdr pane read <pane_id>
```
