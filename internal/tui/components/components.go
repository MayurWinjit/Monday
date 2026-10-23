package components

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/MayurWinjit/monday/internal/domain/session"
	"github.com/MayurWinjit/monday/internal/tui/theme"
)

func Badge(status string) string {
	color, ok := theme.StatusColor[status]
	if !ok {
		color = theme.Subtext0
	}
	return lipgloss.NewStyle().
		Bold(true).
		Foreground(color).
		Render(statusGlyph(status) + " " + status)
}

func statusGlyph(status string) string {
	switch status {
	case "running":
		return "●"
	case "done":
		return "✓"
	case "failed":
		return "✗"
	case "needs_input":
		return "?"
	default:
		return "○"
	}
}

func Card(title, body string, focused bool) string {
	style := theme.BorderStyle
	if focused {
		style = theme.FocusedBorderStyle
	}
	content := body
	if title != "" {
		content = theme.Subheading.Render(title) + "\n\n" + body
	}
	return style.Padding(1, 2).Render(content)
}

func Button(label string, focused bool) string {
	style := lipgloss.NewStyle().Padding(0, 2).Foreground(theme.Text)
	if focused {
		style = style.Background(theme.Mauve).Foreground(theme.Base).Bold(true)
	} else {
		style = style.Foreground(theme.Subtext0)
	}
	return style.Render("[ " + label + " ]")
}

func FormatElapsed(d time.Duration) string {
	d = d.Truncate(time.Second)
	minutes := int(d.Minutes())
	seconds := int(d.Seconds()) % 60
	return fmt.Sprintf("%02d:%02d", minutes, seconds)
}

func ProgressRow(step session.PipelineStep, elapsed time.Duration, selected bool) string {
	marker := "  "
	if selected {
		marker = "▸ "
	}

	elapsedStr := ""
	switch {
	case step.FinishedAt != nil && step.StartedAt != nil:
		elapsedStr = FormatElapsed(step.FinishedAt.Sub(*step.StartedAt))
	case step.StartedAt != nil:
		elapsedStr = FormatElapsed(elapsed)
	}

	name := theme.Body.Render(fmt.Sprintf("%d. %-12s", step.Index+1, step.AgentName))
	badge := Badge(string(step.Status))
	activity := theme.Dim.Render(step.Activity)
	hint := ""
	if step.Status == session.StepNeedsInput {
		hint = lipgloss.NewStyle().Bold(true).Foreground(theme.Yellow).Render(" - press enter to answer")
	}

	row := fmt.Sprintf("%s%s  %s  %s%s", marker, name, badge, activity, hint)
	if elapsedStr != "" {
		row = fmt.Sprintf("%-70s %s", row, theme.Faint.Render(elapsedStr))
	}
	return row
}

func HelpBar(bindings ...string) string {
	return theme.HelpBarStyle.Render(strings.Join(bindings, " · "))
}

func WrapIndent(width int, marker string, style lipgloss.Style, text string) string {
	indentWidth := lipgloss.Width(marker)
	wrapWidth := width - indentWidth
	if wrapWidth < 10 {
		wrapWidth = 10
	}
	rendered := style.Width(wrapWidth).Render(text)
	lines := strings.Split(rendered, "\n")
	pad := strings.Repeat(" ", indentWidth)
	for i, line := range lines {
		if i == 0 {
			lines[i] = marker + line
		} else {
			lines[i] = pad + line
		}
	}
	return strings.Join(lines, "\n")
}

func SpaceBetween(width int, left, right string) string {
	leftWidth := lipgloss.Width(left)
	rightWidth := lipgloss.Width(right)
	gap := width - leftWidth - rightWidth
	if gap < 1 {
		return left + "\n" + right
	}
	return left + strings.Repeat(" ", gap) + right
}
