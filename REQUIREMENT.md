# Monday - A Multi-agent orchestration tool
Monday is a tui tool that uses `herdr` & `claude` to create multi-agent orchestration for development.

## 1. Technical Requirement
- Language: Go
- Framework: BubbleTea
- Architecture: Multi-Layer (1. Presentation Layer: UI Logic, 2. Domain Layer: Core Logic, 3. Infrastructure Layer: Formats Domain Layer for Presentation Layer)
- Supported OS: MacOS, Linux
- Required CLIs on System: `herdr` & `claude`

## 2. Application Flow
1. User launcher `monday` tui tool from terminal
2. Tool creates `~/.monday` directory to store app data
3. Tool launches full screen interactive tui
4. tool checks for installation of `herdr` & `claude` installed on system or not
5. If tools not found then prompts user to install the required tools before using the tool.
6. After check, tool checks for available agents on `claude` & Shows user the list of available agents. If there are saved pipelines then allow user to directly use the pipeline.
7. Allow user to select & create Multi-agent pipeline. For e.g.
    a. If there are 4 agents. (architect, developer, reviewer, tester)
    b. Allow user to multi-select agents using which orchestration will be created
    c. Allow user to set the orchestration of the agent. Like which agent to start 1st & then pass to which one & then so on till User is satisfied.
    d. Once pipeline setup is done then allow user to Save the pipeline in `~/.monday/piplines` directory.
8. Use a text area to accept requirement from the user & a button to accept the requirement
9. Once button is pressed, initiate a new session for the `monday` tool. The session details are as follows
    a. Session information is stored under `~/.monday/sessions/` directory
    b. Each session has a unique UUID to resume again. & stored as `~/.monday/sessions/<uuid>/`
    c. Each session saves the orchestration guide, pane informations, notification information, Prompt history, generated documents etc.
10. Detect which herdr tab the current tool is running in
11. Based on the number of Agents in the pipeline, spawn/split panes according to the agents. Use `./herdr.md` for the herdr commands usage reference.
12. Once all the panes are available as per pipeline then spawn the agents inside each one.
13. The panes should be symmetric. For e.g. If there are 4 agents then the panes should be of 2x2 layout.
14. Based on the orchestration setup by User, pass the requirement into the 1st agent in pipeline.
15. In addition to the requirement provided by user, pass the requirement that is, If any additional document or files generated then store it under `~/.monday/sessions/<session-id>/docs/`.
16. If there are any questions that requires answers by the user then, provide additional requirement that is store the questions under `~/.monday/sessions/<session-id>/interactions/<agent_name>.json`. The json should be formatted as follows:
```
{
    "agent": "<agent_name>",
    "questions":[
        {
            "id": "<question_id>",
            "question": "<question_title>",
            "description": "<question_description>", //optional
            "options": [
                "<option_1>",
                "<option_2>"
            ]
        }
    ]
}
```
17. Create a mechanism like a command option to notify the `monday` tool if any agent is done with the work or requires user inputs. For e.g. Creating a cli command to notify the main session
```
monday notify --session <session_id> --pane <pane_id> --type QUE

// There should be 2 types
// 1. QUES - if agent requires answers from user & is in pending stage
// 2. DONE - If agent is done with the work
```
18. Once tool is notified & if any agent requires any input from user, then using the json created in 16, Present the questions & options in the tool to answer. For all questions, always provide option for any other answer other than provided options.
19. Once all the questions are answered then, pass the information again back to the agent which requires the answer.
20. Once the task is done by the agent, then the agent should inform the `monday` tool to proceed with remaining process like passing the information to next agent or informing main tool the pipeline is done.

### 2.a Graphify Injection
1. There are 3 graphify outputs in this directory 
    a. Payment Portal : `./payment-portal/graphify-out`
    b. Monorepo: `./monorepo/graphify-out`
    c. Payouts: `./payouts/graphify-out`
2. Ask user if they want to inject the graphify into the directory before initiating pipline,
3. If user says yes then Present user with above 3 options to select which graphify-out directory to inject into the directory.
4. On Selection inject the `graphify-out` directory to the project's root directory using symbolic link 
5. Once development is done then use `/graphify` skill to update the context inside `graphify-out` directory
6. After context update, remove the link from the project's root directory.
7. Save these graphify-out directories inside `~/.monday/graphs/` directory when tool is initiated

## 3. Visual Requirement
1. The Launch screen should show an ASCII with Text `Monday`. Below to that it should show list of available pipelines if there are any. If no available then simply don't show the section.
2. Below to the pipeline section it should show a button that says `Create an orchestration`.
3. Once clicked on the button it should present all the available agents & follow the UI required for the steps 2.5 to 2.9 
4. The TUI should look like a Stepped progress indicator when new pipeline starts. Each step should show agent name, what it's working on, it's status & time required for it to finish the task.
5. Use catppuccin theme for the UI

## 4. Claude Requirement 
1. for ease of access to this tool for claude, create 1 skill named `monday` & store it under `~/.claude/skills/monday/SKILL.md` directory
2. This skill should have clear instructions about how to use this `monday` tool. How to notify the tool, where to save the generated docs & all other information required for claude to access the tool.
