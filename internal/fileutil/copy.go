// Package fileutil provides common file system utilities.
package fileutil

import (
	"io"
	"os"
	"path/filepath"

	"github.com/JuniperBible/Public.Tool.JuniperBible/internal/safefile"
)

// CopyDir recursively copies a directory tree from src to dst.
// The destination directory is created if it doesn't exist.
func CopyDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !srcInfo.IsDir() {
		return CopyFile(src, dst)
	}

	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	return copyEntries(entries, src, dst)
}

func copyEntries(entries []os.DirEntry, src, dst string) error {
	for _, entry := range entries {
		if err := copyEntry(entry, src, dst); err != nil {
			return err
		}
	}
	return nil
}

func copyEntry(entry os.DirEntry, src, dst string) error {
	srcPath := filepath.Join(src, entry.Name())
	dstPath := filepath.Join(dst, entry.Name())
	if entry.IsDir() {
		return CopyDir(srcPath, dstPath)
	}
	return CopyFile(srcPath, dstPath)
}

// CopyFile copies a single file from src to dst.
// The destination file is created with the same permissions as the source.
func CopyFile(src, dst string) error {
	srcFile, err := safefile.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return err
	}

	// Create destination directory if needed
	if err := os.MkdirAll(filepath.Dir(dst), 0700); err != nil {
		return err
	}

	dstFile, err := safefile.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}
