package selfupdate

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/MayurWinjit/monday/internal/domain/selfupdate"
)

const defaultAPIBase = "https://api.github.com"

type GitHubSource struct {
	Owner   string
	Repo    string
	APIBase string
	HTTP    *http.Client
}

func NewGitHubSource(owner, repo string) *GitHubSource {
	return &GitHubSource{
		Owner:   owner,
		Repo:    repo,
		APIBase: defaultAPIBase,
		HTTP:    &http.Client{Timeout: 20 * time.Second},
	}
}

type githubRelease struct {
	TagName string        `json:"tag_name"`
	Assets  []githubAsset `json:"assets"`
}

type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

func (g *GitHubSource) LatestRelease() (selfupdate.Release, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/releases/latest", g.APIBase, g.Owner, g.Repo)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return selfupdate.Release{}, fmt.Errorf("selfupdate: build request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := g.HTTP.Do(req)
	if err != nil {
		return selfupdate.Release{}, fmt.Errorf("selfupdate: fetch latest release: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return selfupdate.Release{}, fmt.Errorf("selfupdate: fetch latest release: unexpected status %s", resp.Status)
	}

	var parsed githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return selfupdate.Release{}, fmt.Errorf("selfupdate: decode release: %w", err)
	}

	release := selfupdate.Release{Version: parsed.TagName}
	for _, a := range parsed.Assets {
		release.Assets = append(release.Assets, selfupdate.Asset{Name: a.Name, URL: a.BrowserDownloadURL})
	}
	return release, nil
}

func (g *GitHubSource) Download(url string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("selfupdate: build request: %w", err)
	}

	resp, err := g.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("selfupdate: download: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("selfupdate: download: unexpected status %s", resp.Status)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("selfupdate: read download: %w", err)
	}
	return data, nil
}
