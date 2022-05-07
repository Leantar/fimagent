//go:build windows

package models

import (
	"fmt"
	"os"
	windows "syscall"
	"time"
)

func NewFsObject(path string) (FsObject, error) {
	info, err := os.Stat(path)
	if err != nil {
		return FsObject{}, fmt.Errorf("failed to stat path: %w", err)
	}

	stat := info.Sys().(*windows.Win32FileAttributeData)
	created := time.Unix(0, stat.CreationTime.Nanoseconds()).Unix()
	modified := time.Unix(0, stat.LastWriteTime.Nanoseconds()).Unix()

	obj := FsObject{
		Path:     path,
		Created:  created,
		Modified: modified,
		Uid:      0,
		Gid:      0,
		Mode:     uint32(info.Mode()),
	}

	// Check if file is regular
	if info.Mode().IsRegular() {
		obj.Hash, err = hashFile(path)
		if err != nil {
			return FsObject{}, err
		}
	}

	return obj, nil
}
