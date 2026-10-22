// Command monday is the entry point for the Monday multi-agent orchestration
// tool. It dispatches to either the full-screen TUI (`monday`) or the
// `monday notify` subcommand used by pipeline agents to signal Monday.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/MayurWinjit/monday/internal/app"
	"github.com/MayurWinjit/monday/internal/domain/notifyproto"
	"github.com/MayurWinjit/monday/internal/infra/socket"
	tuiapp "github.com/MayurWinjit/monday/internal/tui/app"
	"github.com/MayurWinjit/monday/internal/tui/app/screens/placeholder"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "notify" {
		os.Exit(runNotify(os.Args[2:]))
	}
	os.Exit(runTUI())
}

// runTUI wires internal/app.Services and launches the full-screen TUI. The
// screen factory below only knows about the placeholder screen for now
// (build step 11) - later steps replace the initial screen and grow the
// factory's switch as each real screen is built, without ever needing
// internal/tui/app itself to import a concrete screen package (see
// internal/tui/app/model.go's package comment).
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

	factory := func(id tuiapp.ScreenID, payload any, services *app.Services) tuiapp.Screen {
		return placeholder.New()
	}

	model := tuiapp.New(services, factory, tuiapp.ScreenToolCheck, placeholder.New())
	if _, err := tea.NewProgram(model, tea.WithAltScreen()).Run(); err != nil {
		fmt.Fprintln(os.Stderr, "monday:", err)
		return 1
	}
	return 0
}

// runNotify implements `monday notify --session <id> --pane <pane_id> --type QUES|DONE`
// (REQUIREMENT.md step 17, architect-plan section 5.3): a thin CLI entry
// point that dials the session's Unix socket and sends one NotifyMessage.
// It takes the socket path purely from --session, never from an inherited
// environment variable, since it may be invoked by an agent whose shell has
// a completely different working directory than Monday's own process.
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
