package main

import (
	"bytes"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/MayurWinjit/monday/internal/buildinfo"
	"github.com/MayurWinjit/monday/internal/domain/notifyproto"
	domainselfupdate "github.com/MayurWinjit/monday/internal/domain/selfupdate"
	"github.com/MayurWinjit/monday/internal/infra/socket"
)

func shortTempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "mnd")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

func TestRunNotify_HappyPath(t *testing.T) {
	home := shortTempDir(t)
	t.Setenv("HOME", home)

	server := socket.NewServer(home + "/.monday")
	events, closeFn, err := server.Listen("session-1")
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer closeFn()

	code := runNotify([]string{"--session", "session-1", "--pane", "w1:p2", "--type", "DONE"})
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d", code)
	}

	select {
	case msg := <-events:
		if msg.Type != notifyproto.NotifyDone || msg.SessionID != "session-1" || msg.PaneID != "w1:p2" {
			t.Fatalf("unexpected message: %+v", msg)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for event")
	}
}

func TestRunNotify_MissingFlags(t *testing.T) {
	if code := runNotify([]string{"--session", "x"}); code != 2 {
		t.Fatalf("expected exit code 2 for missing flags, got %d", code)
	}
}

func TestRunNotify_InvalidType(t *testing.T) {
	if code := runNotify([]string{"--session", "x", "--pane", "y", "--type", "NOPE"}); code != 2 {
		t.Fatalf("expected exit code 2 for an invalid --type, got %d", code)
	}
}

func TestRunNotify_NoSocketListening(t *testing.T) {
	home := shortTempDir(t)
	t.Setenv("HOME", home)

	code := runNotify([]string{"--session", "no-such-session", "--pane", "w1:p2", "--type", "DONE"})
	if code != 1 {
		t.Fatalf("expected exit code 1 when no server is listening, got %d", code)
	}
}

type fakeUpdateSource struct {
	release domainselfupdate.Release
	err     error
}

func (f *fakeUpdateSource) LatestRelease() (domainselfupdate.Release, error) {
	return f.release, f.err
}

func (f *fakeUpdateSource) Download(url string) ([]byte, error) {
	return []byte("binary"), nil
}

type fakeUpdateInstaller struct {
	installed bool
	err       error
}

func (f *fakeUpdateInstaller) Install(data []byte) error {
	if f.err != nil {
		return f.err
	}
	f.installed = true
	return nil
}

func TestRunUpdate_AlreadyUpToDate(t *testing.T) {
	original := buildinfo.Version
	buildinfo.Version = "v1.0.0"
	t.Cleanup(func() { buildinfo.Version = original })

	source := &fakeUpdateSource{release: domainselfupdate.Release{Version: "v1.0.0"}}
	installer := &fakeUpdateInstaller{}
	service := domainselfupdate.NewService(source, installer)

	var stdout, stderr bytes.Buffer
	code := runUpdate(nil, service, &stdout, &stderr, strings.NewReader(""))
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d (stderr: %s)", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "already up to date") {
		t.Fatalf("expected an up-to-date message, got %q", stdout.String())
	}
	if installer.installed {
		t.Fatalf("expected Install not to be called")
	}
}

func TestRunUpdate_CheckOnlyDoesNotInstall(t *testing.T) {
	source := &fakeUpdateSource{release: domainselfupdate.Release{Version: "v99.0.0"}}
	installer := &fakeUpdateInstaller{}
	service := domainselfupdate.NewService(source, installer)

	var stdout, stderr bytes.Buffer
	code := runUpdate([]string{"--check"}, service, &stdout, &stderr, strings.NewReader(""))
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d (stderr: %s)", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "v99.0.0") {
		t.Fatalf("expected the new version to be mentioned, got %q", stdout.String())
	}
	if installer.installed {
		t.Fatalf("expected --check to not install anything")
	}
}

func TestRunUpdate_DeclinedConfirmationDoesNotInstall(t *testing.T) {
	source := &fakeUpdateSource{release: domainselfupdate.Release{Version: "v99.0.0"}}
	installer := &fakeUpdateInstaller{}
	service := domainselfupdate.NewService(source, installer)

	var stdout, stderr bytes.Buffer
	code := runUpdate(nil, service, &stdout, &stderr, strings.NewReader("n\n"))
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d (stderr: %s)", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "cancelled") {
		t.Fatalf("expected a cancellation message, got %q", stdout.String())
	}
	if installer.installed {
		t.Fatalf("expected a declined confirmation to not install anything")
	}
}

func TestRunUpdate_ConfirmedInstallsAndSucceeds(t *testing.T) {
	source := &fakeUpdateSource{release: domainselfupdate.Release{
		Version: "v99.0.0",
		Assets: []domainselfupdate.Asset{
			{Name: domainselfupdate.AssetName(runtime.GOOS, runtime.GOARCH), URL: "https://example.com/binary"},
		},
	}}
	installer := &fakeUpdateInstaller{}
	service := domainselfupdate.NewService(source, installer)

	var stdout, stderr bytes.Buffer
	code := runUpdate(nil, service, &stdout, &stderr, strings.NewReader("y\n"))
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d (stderr: %s)", code, stderr.String())
	}
	if !installer.installed {
		t.Fatalf("expected Install to be called")
	}
	if !strings.Contains(stdout.String(), "updated to v99.0.0") {
		t.Fatalf("expected a success message, got %q", stdout.String())
	}
}

func TestRunUpdate_YesFlagSkipsConfirmation(t *testing.T) {
	source := &fakeUpdateSource{release: domainselfupdate.Release{
		Version: "v99.0.0",
		Assets: []domainselfupdate.Asset{
			{Name: domainselfupdate.AssetName(runtime.GOOS, runtime.GOARCH), URL: "https://example.com/binary"},
		},
	}}
	installer := &fakeUpdateInstaller{}
	service := domainselfupdate.NewService(source, installer)

	var stdout, stderr bytes.Buffer
	code := runUpdate([]string{"--yes"}, service, &stdout, &stderr, strings.NewReader(""))
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d (stderr: %s)", code, stderr.String())
	}
	if !installer.installed {
		t.Fatalf("expected --yes to install without a prompt")
	}
}

func TestRunUpdate_SourceErrorReturnsExitCode1(t *testing.T) {
	source := &fakeUpdateSource{err: errUpdateBoom{}}
	installer := &fakeUpdateInstaller{}
	service := domainselfupdate.NewService(source, installer)

	var stdout, stderr bytes.Buffer
	code := runUpdate(nil, service, &stdout, &stderr, strings.NewReader(""))
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
}

type errUpdateBoom struct{}

func (errUpdateBoom) Error() string { return "boom" }
