package selfupdate

import (
	"crypto/sha256"
	"encoding/hex"
	"runtime"
	"testing"
)

func checksumLine(data []byte, assetName string) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]) + "  " + assetName + "\n"
}

type fakeSource struct {
	release     Release
	releaseErr  error
	downloads   map[string][]byte
	downloadErr error
}

func (f *fakeSource) LatestRelease() (Release, error) {
	return f.release, f.releaseErr
}

func (f *fakeSource) Download(url string) ([]byte, error) {
	if f.downloadErr != nil {
		return nil, f.downloadErr
	}
	return f.downloads[url], nil
}

type fakeInstaller struct {
	installed []byte
	err       error
}

func (f *fakeInstaller) Install(data []byte) error {
	if f.err != nil {
		return f.err
	}
	f.installed = data
	return nil
}

func TestService_Check_NewerAvailable(t *testing.T) {
	source := &fakeSource{release: Release{Version: "v1.1.0"}}
	service := NewService(source, &fakeInstaller{})

	result, err := service.Check("v1.0.0")
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !result.Newer {
		t.Fatalf("expected Newer to be true")
	}
	if result.Latest.Version != "v1.1.0" {
		t.Fatalf("Latest.Version = %q", result.Latest.Version)
	}
}

func TestService_Check_AlreadyUpToDate(t *testing.T) {
	source := &fakeSource{release: Release{Version: "v1.0.0"}}
	service := NewService(source, &fakeInstaller{})

	result, err := service.Check("v1.0.0")
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if result.Newer {
		t.Fatalf("expected Newer to be false")
	}
}

func TestService_Check_UnparsableCurrentVersionAlwaysOffersUpdate(t *testing.T) {
	source := &fakeSource{release: Release{Version: "v1.0.0"}}
	service := NewService(source, &fakeInstaller{})

	result, err := service.Check("dev")
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if !result.Newer {
		t.Fatalf("expected a dev build to always be offered the latest release")
	}
}

func TestService_Check_SourceError(t *testing.T) {
	source := &fakeSource{releaseErr: errBoom{}}
	service := NewService(source, &fakeInstaller{})

	if _, err := service.Check("v1.0.0"); err == nil {
		t.Fatalf("expected an error when the source fails")
	}
}

func TestService_Apply_DownloadsVerifiesAndInstalls(t *testing.T) {
	binary := []byte("new-binary-contents")
	checksums := checksumLine(binary, AssetName(runtime.GOOS, runtime.GOARCH))

	source := &fakeSource{
		downloads: map[string][]byte{
			"https://example.com/binary":    binary,
			"https://example.com/checksums": []byte(checksums),
		},
	}
	installer := &fakeInstaller{}
	service := NewService(source, installer)

	release := Release{
		Version: "v1.1.0",
		Assets: []Asset{
			{Name: AssetName(runtime.GOOS, runtime.GOARCH), URL: "https://example.com/binary"},
			{Name: ChecksumsAssetName, URL: "https://example.com/checksums"},
		},
	}

	if err := service.Apply(release); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if string(installer.installed) != string(binary) {
		t.Fatalf("installed = %q, want %q", installer.installed, binary)
	}
}

func TestService_Apply_MissingAsset(t *testing.T) {
	source := &fakeSource{}
	service := NewService(source, &fakeInstaller{})

	release := Release{Version: "v1.1.0"}
	if err := service.Apply(release); err == nil {
		t.Fatalf("expected an error when no matching asset exists")
	}
}

func TestService_Apply_ChecksumMismatchAborts(t *testing.T) {
	binary := []byte("new-binary-contents")
	wrongChecksums := checksumLine([]byte("different"), AssetName(runtime.GOOS, runtime.GOARCH))

	source := &fakeSource{
		downloads: map[string][]byte{
			"https://example.com/binary":    binary,
			"https://example.com/checksums": []byte(wrongChecksums),
		},
	}
	installer := &fakeInstaller{}
	service := NewService(source, installer)

	release := Release{
		Version: "v1.1.0",
		Assets: []Asset{
			{Name: AssetName(runtime.GOOS, runtime.GOARCH), URL: "https://example.com/binary"},
			{Name: ChecksumsAssetName, URL: "https://example.com/checksums"},
		},
	}

	if err := service.Apply(release); err == nil {
		t.Fatalf("expected a checksum mismatch error")
	}
	if installer.installed != nil {
		t.Fatalf("expected Install to not be called on checksum mismatch")
	}
}

type errBoom struct{}

func (errBoom) Error() string { return "boom" }
