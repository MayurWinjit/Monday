package components

import (
	"testing"
	"time"
)

func TestFormatElapsed(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{0, "00:00"},
		{5 * time.Second, "00:05"},
		{65 * time.Second, "01:05"},
		{90*time.Second + 400*time.Millisecond, "01:30"},
		{3661 * time.Second, "61:01"},
	}
	for _, tt := range tests {
		if got := FormatElapsed(tt.d); got != tt.want {
			t.Errorf("FormatElapsed(%v) = %q, want %q", tt.d, got, tt.want)
		}
	}
}

func TestBadge_KnownAndUnknownStatus(t *testing.T) {
	for _, status := range []string{"waiting", "running", "needs_input", "done", "failed", "something-unrecognized"} {
		got := Badge(status)
		if got == "" {
			t.Errorf("Badge(%q) returned empty string", status)
		}
	}
}

func TestHelpBar_JoinsWithSeparator(t *testing.T) {
	got := HelpBar("a", "b", "c")
	if !contains(got, "a") || !contains(got, "b") || !contains(got, "c") || !contains(got, "·") {
		t.Errorf("HelpBar() = %q, expected it to join bindings with a separator", got)
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (func() bool {
		for i := 0; i+len(needle) <= len(haystack); i++ {
			if haystack[i:i+len(needle)] == needle {
				return true
			}
		}
		return false
	})()
}
