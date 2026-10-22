// Command monday is the entry point for the Monday multi-agent orchestration
// tool. It dispatches to either the full-screen TUI (`monday`) or the
// `monday notify` subcommand used by pipeline agents to signal Monday.
//
// This is a scaffold placeholder (build plan step 1): wiring, the TUI, and
// the notify subcommand are added in later steps.
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "monday: not yet implemented")
	os.Exit(1)
}
