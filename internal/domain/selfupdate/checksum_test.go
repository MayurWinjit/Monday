package selfupdate

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestVerifyChecksum(t *testing.T) {
	data := []byte("hello world")
	sum := sha256.Sum256(data)
	hexSum := hex.EncodeToString(sum[:])
	checksums := hexSum + "  monday-linux-amd64\n" + "deadbeef  monday-darwin-arm64\n"

	if err := VerifyChecksum(data, checksums, "monday-linux-amd64"); err != nil {
		t.Fatalf("expected checksum to verify, got %v", err)
	}
}

func TestVerifyChecksum_MissingEntry(t *testing.T) {
	data := []byte("hello world")
	sum := sha256.Sum256(data)
	checksums := hex.EncodeToString(sum[:]) + "  monday-linux-amd64\n"

	if err := VerifyChecksum(data, checksums, "monday-darwin-arm64"); err == nil {
		t.Fatalf("expected an error for a missing checksum entry")
	}
}

func TestVerifyChecksum_Mismatch(t *testing.T) {
	sum := sha256.Sum256([]byte("hello world"))
	checksums := hex.EncodeToString(sum[:]) + "  monday-linux-amd64\n"

	if err := VerifyChecksum([]byte("tampered"), checksums, "monday-linux-amd64"); err == nil {
		t.Fatalf("expected a checksum mismatch error")
	}
}
