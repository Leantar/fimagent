package models

import (
	"encoding/hex"
	"fmt"
	"github.com/zeebo/blake3"
	"io"
	"os"
)

type FsObject struct {
	Path     string
	Hash     string
	Created  int64
	Modified int64
	Uid      uint32
	Gid      uint32
	Mode     uint32
}

func hashFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	hasher := blake3.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", fmt.Errorf("failed to copy file content: %w", err)
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}
