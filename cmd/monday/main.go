package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/MayurWinjit/monday/internal/app"
	"github.com/MayurWinjit/monday/internal/buildinfo"
	"github.com/MayurWinjit/monday/internal/domain/notifyproto"
	domainselfupdate "github.com/MayurWinjit/monday/internal/domain/selfupdate"
	infraselfupdate "github.com/MayurWinjit/monday/internal/infra/selfupdate"
	"github.com/MayurWinjit/monday/internal/infra/socket"
	tuiapp "github.com/MayurWinjit/monday/internal/tui/app"
	"github.com/MayurWinjit/monday/internal/tui/app/screens/agentpicker"
	"github.com/MayurWinjit/monday/internal/tui/app/screens/graphifyprompt"
	"github.com/MayurWinjit/monday/internal/tui/app/screens/launch"
	"github.com/MayurWinjit/monday/internal/tui/app/screens/pipelineorder"
	"github.com/MayurWinjit/monday/internal/tui/app/screens/progress"
	"github.com/MayurWinjit/monday/internal/tui/app/screens/requirement"
	"github.com/MayurWinjit/monday/internal/tui/app/screens/toolcheck"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "notify":
			os.Exit(runNotify(os.Args[2:]))
		case "update":
			os.Exit(runUpdate(os.Args[2:], buildUpdateService(), os.Stdout, os.Stderr, os.Stdin))
		case "version", "--version", "-v":
			fmt.Println(buildinfo.Version)
			os.Exit(0)
		}
	}
	os.Exit(runTUI())
}

func runTUI() int {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, "monday:", err)
		return 1
	}

	services, err := app.Build(homeDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "monday:", err)
		return 1
	}

	if err := services.SkillWriter.Install(); err != nil {
		fmt.Fprintln(os.Stderr, "monday: could not install the monday Claude skill:", err)
	}

	factory := func(id tuiapp.ScreenID, payload any, services *app.Services) tuiapp.Screen {
		switch id {
		case tuiapp.ScreenToolCheck:
			return toolcheck.New(services.ToolCheck)
		case tuiapp.ScreenLaunch:
			p, _ := payload.(tuiapp.LaunchPayload)
			return launch.New(services, p.AlreadyRunningID)
		case tuiapp.ScreenAgentPicker:
			p, _ := payload.(tuiapp.AgentPickerPayload)
			return agentpicker.New(p.Agents)
		case tuiapp.ScreenPipelineOrder:
			p, _ := payload.(tuiapp.PipelineOrderPayload)
			return pipelineorder.New(services, p.Available, p.Sequence)
		case tuiapp.ScreenRequirement:
			p, _ := payload.(tuiapp.RequirementPayload)
			return requirement.New(p.Pipeline)
		case tuiapp.ScreenGraphifyPrompt:
			p, _ := payload.(tuiapp.GraphifyPromptPayload)
			return graphifyprompt.New(services, p.Pipeline, p.Requirement)
		case tuiapp.ScreenProgress:
			p, _ := payload.(tuiapp.ProgressPayload)
			return progress.New(services, p)
		default:
			return toolcheck.New(services.ToolCheck)
		}
	}

	model := tuiapp.New(services, factory, tuiapp.ScreenToolCheck, toolcheck.New(services.ToolCheck))
	if _, err := tea.NewProgram(model, tea.WithAltScreen()).Run(); err != nil {
		fmt.Fprintln(os.Stderr, "monday:", err)
		return 1
	}
	return 0
}

func runNotify(args []string) int {
	fs := flag.NewFlagSet("monday notify", flag.ContinueOnError)
	sessionID := fs.String("session", "", "session UUID (required)")
	paneID := fs.String("pane", "", "pane ID this notification is from (required)")
	typeFlag := fs.String("type", "", "QUES or DONE (required)")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: monday notify --session <id> --pane <pane_id> --type QUES|DONE")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}

	if *sessionID == "" || *paneID == "" || *typeFlag == "" {
		fs.Usage()
		return 2
	}

	var notifyType notifyproto.NotifyType
	switch notifyproto.NotifyType(*typeFlag) {
	case notifyproto.NotifyQuestion:
		notifyType = notifyproto.NotifyQuestion
	case notifyproto.NotifyDone:
		notifyType = notifyproto.NotifyDone
	default:
		fmt.Fprintf(os.Stderr, "monday notify: --type must be QUES or DONE, got %q\n", *typeFlag)
		return 2
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, "monday notify:", err)
		return 1
	}
	root := filepath.Join(homeDir, ".monday")
	sockPath := socket.Path(root, *sessionID)

	client := socket.NewClient()
	ack, err := client.Send(sockPath, notifyproto.NotifyMessage{
		Type:      notifyType,
		SessionID: *sessionID,
		PaneID:    *paneID,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "monday notify:", err)
		return 1
	}
	if !ack.OK {
		fmt.Fprintln(os.Stderr, "monday notify: rejected:", ack.Error)
		return 1
	}
	return 0
}

func buildUpdateService() *domainselfupdate.Service {
	source := infraselfupdate.NewGitHubSource(buildinfo.RepoOwner, buildinfo.RepoName)
	return domainselfupdate.NewService(source, infraselfupdate.BinaryInstaller{})
}

func runUpdate(args []string, service *domainselfupdate.Service, stdout, stderr io.Writer, stdin io.Reader) int {
	fs := flag.NewFlagSet("monday update", flag.ContinueOnError)
	fs.SetOutput(stderr)
	checkOnly := fs.Bool("check", false, "only check whether an update is available, do not install it")
	yes := fs.Bool("yes", false, "install the update without asking for confirmation")
	fs.Usage = func() {
		_, _ = fmt.Fprintln(stderr, "usage: monday update [--check] [--yes]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}

	result, err := service.Check(buildinfo.Version)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "monday update:", err)
		return 1
	}

	if !result.Newer {
		_, _ = fmt.Fprintf(stdout, "monday is already up to date (%s)\n", result.Current)
		return 0
	}
	_, _ = fmt.Fprintf(stdout, "a new version is available: %s -> %s\n", result.Current, result.Latest.Version)

	if *checkOnly {
		return 0
	}

	if !*yes {
		_, _ = fmt.Fprint(stdout, "install this update now? [y/N] ")
		reader := bufio.NewReader(stdin)
		answer, _ := reader.ReadString('\n')
		if strings.TrimSpace(strings.ToLower(answer)) != "y" {
			_, _ = fmt.Fprintln(stdout, "update cancelled")
			return 0
		}
	}

	_, _ = fmt.Fprintf(stdout, "downloading %s...\n", result.Latest.Version)
	if err := service.Apply(result.Latest); err != nil {
		_, _ = fmt.Fprintln(stderr, "monday update:", err)
		return 1
	}

	_, _ = fmt.Fprintf(stdout, "updated to %s - restart monday to use the new version\n", result.Latest.Version)
	return 0
}
