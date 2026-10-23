package selfupdate

import (
	"fmt"
	"os"
	"path/filepath"
)

type BinaryInstaller struct{}

func (BinaryInstaller) Install(data []byte) error {
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("selfupdate: resolve current executable: %w", err)
	}
	execPath, err = filepath.EvalSymlinks(execPath)
	if err != nil {
		return fmt.Errorf("selfupdate: resolve executable symlink: %w", err)
	}
	return ReplaceFile(execPath, data)
}

func ReplaceFile(path string, data []byte) error {
	dir := filepath.Dir(path)

	tmp, err := os.CreateTemp(dir, ".monday-update-*")
	if err != nil {
		return fmt.Errorf("selfupdate: create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("selfupdate: write new binary: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("selfupdate: close temp file: %w", err)
	}
	if err := os.Chmod(tmpPath, 0o755); err != nil {
		return fmt.Errorf("selfupdate: chmod new binary: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("selfupdate: replace executable: %w", err)
	}
	return nil
}
