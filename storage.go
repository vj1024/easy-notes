package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

var errSymlinkPath = errors.New("symbolic links are not allowed")

// resolveSafePath resolves a user-controlled relative path below root. In
// addition to blocking "..", it rejects every existing symbolic-link component
// so file operations cannot escape the storage directory through a symlink.
func resolveSafePath(root, requestPath string) (string, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve storage directory: %w", err)
	}

	cleanPath := filepath.Clean(strings.TrimPrefix(requestPath, "/"))
	if cleanPath == "." {
		return absRoot, nil
	}
	if filepath.IsAbs(cleanPath) {
		return "", errors.New("absolute paths are not allowed")
	}

	fullPath := filepath.Join(absRoot, cleanPath)
	relPath, err := filepath.Rel(absRoot, fullPath)
	if err != nil || relPath == ".." || strings.HasPrefix(relPath, ".."+string(os.PathSeparator)) {
		return "", errors.New("path traversal detected")
	}

	current := absRoot
	for _, part := range strings.Split(relPath, string(os.PathSeparator)) {
		current = filepath.Join(current, part)
		info, statErr := os.Lstat(current)
		if statErr != nil {
			if os.IsNotExist(statErr) {
				continue
			}
			return "", fmt.Errorf("inspect path: %w", statErr)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("%w: %s", errSymlinkPath, part)
		}
	}

	return fullPath, nil
}

func safePath(requestPath string) (string, error) {
	return resolveSafePath(baseDir, requestPath)
}

// atomicWrite writes to a temporary file in the destination directory and only
// replaces the destination after the complete payload has been flushed.
func atomicWrite(path string, source io.Reader, mode os.FileMode) (retErr error) {
	dir := filepath.Dir(path)
	temp, err := os.CreateTemp(dir, ".easy-notes-*")
	if err != nil {
		return fmt.Errorf("create temporary file: %w", err)
	}
	tempPath := temp.Name()
	defer func() {
		_ = temp.Close()
		if retErr != nil {
			_ = os.Remove(tempPath)
		}
	}()

	if err := temp.Chmod(mode); err != nil {
		return fmt.Errorf("set temporary file permissions: %w", err)
	}
	if _, err := io.Copy(temp, source); err != nil {
		return fmt.Errorf("write temporary file: %w", err)
	}
	if err := temp.Sync(); err != nil {
		return fmt.Errorf("flush temporary file: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close temporary file: %w", err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("replace destination file: %w", err)
	}
	return nil
}
