package selfupdate

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

func VerifyChecksum(data []byte, checksumsFile, assetName string) error {
	sum := sha256.Sum256(data)
	got := hex.EncodeToString(sum[:])

	for _, line := range strings.Split(checksumsFile, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		want, name := fields[0], strings.TrimPrefix(fields[1], "*")
		if name != assetName {
			continue
		}
		if !strings.EqualFold(want, got) {
			return fmt.Errorf("selfupdate: checksum mismatch for %s: want %s, got %s", assetName, want, got)
		}
		return nil
	}

	return fmt.Errorf("selfupdate: no checksum entry found for %s", assetName)
}
