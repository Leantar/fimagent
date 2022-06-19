//go:build darwin || linux

package models

import (
	"fmt"
	"golang.org/x/sys/unix"
)

const (
	S_IFMT  = 0o0170000
	S_IFREG = 0o0100000
)

func NewFsObject(path string) (FsObject, error) {
	var stat unix.Stat_t

	err := unix.Lstat(path, &stat)
	if err != nil {
		return FsObject{}, fmt.Errorf("failed to stat path: %w", err)
	}

	created, _ := stat.Ctim.Unix()
	modified, _ := stat.Mtim.Unix()

	obj := FsObject{
		Path:     path,
		Created:  created,
		Modified: modified,
		Uid:      stat.Uid,
		Gid:      stat.Gid,
		Mode:     uint32(stat.Mode),
	}

	// Check if file is regular
	if stat.Mode&S_IFMT == S_IFREG {
		obj.Hash, err = hashFile(path)
		if err != nil {
			return FsObject{}, err
		}
	}

	return obj, nil
}
