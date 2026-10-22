// Command monday is the entry point for the Monday multi-agent orchestration
// tool. It dispatches to either the full-screen TUI (`monday`) or the
// `monday notify` subcommand used by pipeline agents to signal Monday.
//
// The TUI itself is wired in a later build step (section 13 step 11+);
// until then, running `monday` with no subcommand prints a placeholder.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/MayurWinjit/monday/internal/domain/notifyproto"
	"github.com/MayurWinjit/monday/internal/infra/socket"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "notify" {
		os.Exit(runNotify(os.Args[2:]))
	}

	fmt.Fprintln(os.Stderr, "monday: the TUI is not yet implemented - run `monday notify --help` for the notify subcommand")
	os.Exit(1)
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
