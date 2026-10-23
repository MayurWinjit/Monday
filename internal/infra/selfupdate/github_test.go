package selfupdate

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGitHubSource_LatestRelease(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/acme/widget/releases/latest" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
			"tag_name": "v1.2.3",
			"assets": [
				{"name": "monday-linux-amd64", "browser_download_url": "https://example.com/monday-linux-amd64"},
				{"name": "checksums.txt", "browser_download_url": "https://example.com/checksums.txt"}
			]
		}`)
	}))
	defer server.Close()

	source := NewGitHubSource("acme", "widget")
	source.APIBase = server.URL

	release, err := source.LatestRelease()
	if err != nil {
		t.Fatalf("LatestRelease: %v", err)
	}
	if release.Version != "v1.2.3" {
		t.Fatalf("Version = %q, want %q", release.Version, "v1.2.3")
	}
	asset, ok := release.FindAsset("monday-linux-amd64")
	if !ok {
		t.Fatalf("expected a monday-linux-amd64 asset")
	}
	if asset.URL != "https://example.com/monday-linux-amd64" {
		t.Fatalf("asset URL = %q", asset.URL)
	}
}

func TestGitHubSource_LatestRelease_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	source := NewGitHubSource("acme", "widget")
	source.APIBase = server.URL

	if _, err := source.LatestRelease(); err == nil {
		t.Fatalf("expected an error for a 404 response")
	}
}

func TestGitHubSource_Download(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("binary-contents"))
	}))
	defer server.Close()

	source := NewGitHubSource("acme", "widget")
	data, err := source.Download(server.URL + "/asset")
	if err != nil {
		t.Fatalf("Download: %v", err)
	}
	if string(data) != "binary-contents" {
		t.Fatalf("Download data = %q", data)
	}
}

func TestGitHubSource_Download_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	source := NewGitHubSource("acme", "widget")
	if _, err := source.Download(server.URL + "/asset"); err == nil {
		t.Fatalf("expected an error for a 500 response")
	}
}
