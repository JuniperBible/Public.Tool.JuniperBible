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

func ensureParentDir(dstPath string) error {
	if err := os.MkdirAll(filepath.Dir(dstPath), 0700); err != nil {
		return fmt.Errorf("failed to create parent directory: %w", err)
	}
	return nil
}

func buildTarHeader(info os.FileInfo, name string, now time.Time) (*tar.Header, error) {
	header, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return nil, err
	}
	header.Name = name
	if info.IsDir() {
		header.Name += "/"
	}
	header.ModTime = now
	return header, nil
}

func writeFileContent(tw *tar.Writer, path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = io.Copy(tw, file)
	return err
}

func addToArchive(tw *tar.Writer, srcDir, baseDir string, now time.Time) filepath.WalkFunc {
	return func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		if relPath == "." {
			return nil
		}
		header, err := buildTarHeader(info, baseDir+"/"+relPath, now)
		if err != nil {
			return err
		}
		if err := tw.WriteHeader(header); err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		return writeFileContent(tw, path)
	}
}

func CreateTarGz(srcDir, dstPath, baseDir string, createParentDir bool) error {
	if createParentDir {
		if err := ensureParentDir(dstPath); err != nil {
			return err
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

	if err := filepath.Walk(srcDir, addToArchive(tw, srcDir, baseDir, time.Now())); err != nil {
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
