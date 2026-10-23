package selfupdate

import (
	"fmt"
	"runtime"
)

type Service struct {
	source    Source
	installer Installer
}

func NewService(source Source, installer Installer) *Service {
	return &Service{source: source, installer: installer}
}

type CheckResult struct {
	Current string
	Latest  Release
	Newer   bool
}

func (s *Service) Check(current string) (CheckResult, error) {
	release, err := s.source.LatestRelease()
	if err != nil {
		return CheckResult{}, fmt.Errorf("selfupdate: check: %w", err)
	}

	newer, err := IsNewer(current, release.Version)
	if err != nil {
		return CheckResult{Current: current, Latest: release, Newer: true}, nil
	}

	return CheckResult{Current: current, Latest: release, Newer: newer}, nil
}

func (s *Service) Apply(release Release) error {
	assetName := AssetName(runtime.GOOS, runtime.GOARCH)
	asset, ok := release.FindAsset(assetName)
	if !ok {
		return fmt.Errorf("selfupdate: no release asset for %s", assetName)
	}

	data, err := s.source.Download(asset.URL)
	if err != nil {
		return fmt.Errorf("selfupdate: download %s: %w", asset.Name, err)
	}

	if checksumAsset, ok := release.FindAsset(ChecksumsAssetName); ok {
		checksums, err := s.source.Download(checksumAsset.URL)
		if err != nil {
			return fmt.Errorf("selfupdate: download checksums: %w", err)
		}
		if err := VerifyChecksum(data, string(checksums), asset.Name); err != nil {
			return err
		}
	}

	if err := s.installer.Install(data); err != nil {
		return fmt.Errorf("selfupdate: install: %w", err)
	}

	return nil
}
