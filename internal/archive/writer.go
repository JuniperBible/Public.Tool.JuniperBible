package archive

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// CreateTarGz creates a tar.gz archive from a source directory.
// The baseDir parameter specifies the directory name inside the archive.
// If createParentDir is true, parent directories of dstPath are created.
func CreateTarGz(srcDir, dstPath, baseDir string, createParentDir bool) error {
	if createParentDir {
		if err := os.MkdirAll(filepath.Dir(dstPath), 0700); err != nil {
			return fmt.Errorf("failed to create parent directory: %w", err)
		}
	}

	outFile, err := os.Create(dstPath)
	if err != nil {
		return fmt.Errorf("failed to create archive file: %w", err)
	}
	defer outFile.Close()

	gw := gzip.NewWriter(outFile)
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	now := time.Now()

	err = filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}

		// Skip root directory
		if relPath == "." {
			return nil
		}

		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}

		// Set the name with the base directory prefix
		header.Name = baseDir + "/" + relPath
		if info.IsDir() {
			header.Name += "/"
		}

		// Normalize timestamps for reproducibility
		header.ModTime = now

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		if !info.IsDir() {
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()

			if _, err := io.Copy(tw, file); err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to create archive: %w", err)
	}

	return nil
}

// CreateCapsuleTarGz creates a capsule archive from a directory.
// This is a convenience function that derives the base dir name from srcDir.
func CreateCapsuleTarGz(srcDir, dstPath string) error {
	baseDir := filepath.Base(srcDir)
	return CreateTarGz(srcDir, dstPath, baseDir, true)
}

// CreateCapsuleTarGzFromPath creates a capsule archive deriving the base dir from dstPath.
// This derives the base directory name by removing .tar.gz and .capsule suffixes from dstPath.
func CreateCapsuleTarGzFromPath(srcDir, dstPath string) error {
	baseDir := filepath.Base(strings.TrimSuffix(strings.TrimSuffix(dstPath, ".tar.gz"), ".capsule"))
	return CreateTarGz(srcDir, dstPath, baseDir, false)
}
