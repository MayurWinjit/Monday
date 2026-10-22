package toolcheck

import (
	"errors"
	"testing"
)

type fakeHerdr struct {
	err error
}

func (f fakeHerdr) CurrentPaneID() (string, error) {
	return "w1:p1", f.err
}

func lookPathAllFound(string) (string, error) { return "/usr/bin/x", nil }

func lookPathMissing(missing string) func(string) (string, error) {
	return func(name string) (string, error) {
		if name == missing {
			return "", errors.New("not found")
		}
		return "/usr/bin/" + name, nil
	}
}

func TestRun_AllOK(t *testing.T) {
	c := NewWithLookPath(fakeHerdr{}, lookPathAllFound)
	res := c.Run()
	if !res.OK() {
		t.Fatalf("expected OK, got %+v", res)
	}
}

func TestRun_MissingHerdr(t *testing.T) {
	c := NewWithLookPath(fakeHerdr{}, lookPathMissing("herdr"))
	res := c.Run()
	if !res.MissingHerdr {
		t.Fatalf("expected MissingHerdr, got %+v", res)
	}
	if res.OK() {
		t.Fatalf("expected not OK")
	}
}

func TestRun_MissingClaude(t *testing.T) {
	c := NewWithLookPath(fakeHerdr{}, lookPathMissing("claude"))
	res := c.Run()
	if !res.MissingClaude {
		t.Fatalf("expected MissingClaude, got %+v", res)
	}
}

func TestRun_NotInHerdrPane(t *testing.T) {
	c := NewWithLookPath(fakeHerdr{err: errors.New("no current pane")}, lookPathAllFound)
	res := c.Run()
	if !res.NotInHerdrPane {
		t.Fatalf("expected NotInHerdrPane, got %+v", res)
	}
}

func TestRun_MissingHerdrSkipsPaneCheck(t *testing.T) {
	// herdr missing means CurrentPaneID would exec a nonexistent binary;
	// Run should not report NotInHerdrPane as a second, redundant failure.
	c := NewWithLookPath(fakeHerdr{err: errors.New("should not matter")}, lookPathMissing("herdr"))
	res := c.Run()
	if res.NotInHerdrPane {
		t.Fatalf("expected NotInHerdrPane to be skipped when herdr itself is missing, got %+v", res)
	}
}
