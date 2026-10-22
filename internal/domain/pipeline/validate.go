package pipeline

import (
	"fmt"
	"strings"

	"github.com/MayurWinjit/monday/internal/domain/agent"
)

// Validate checks that p is well-formed: a non-empty, trimmed name, at
// least one agent step, and every agent name resolving against catalog -
// the currently discovered agent catalog. If an agent file was deleted
// since the pipeline was saved, this surfaces a clear "unknown agent: X"
// error rather than silently dropping the step. Uniqueness against other
// saved pipeline names is checked by Store, not here, since that needs a
// directory listing.
func Validate(p Pipeline, catalog []agent.Agent) error {
	if strings.TrimSpace(p.Name) == "" {
		return fmt.Errorf("pipeline name must not be empty")
	}
	if p.Name != strings.TrimSpace(p.Name) {
		return fmt.Errorf("pipeline name must not have leading or trailing whitespace")
	}
	if len(p.Agents) == 0 {
		return fmt.Errorf("pipeline must have at least one agent")
	}

	known := make(map[string]bool, len(catalog))
	for _, a := range catalog {
		known[a.Name] = true
	}
	for _, name := range p.Agents {
		if !known[name] {
			return fmt.Errorf("unknown agent: %s", name)
		}
	}
	return nil
}
