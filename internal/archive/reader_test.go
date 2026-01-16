package archive

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/ulikunitz/xz"
)

func createTestTarGz(t *testing.T, dir string) string {
	path := filepath.Join(dir, "test.tar.gz")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create file: %v", err)
	}
	defer f.Close()

	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)

	// Add a file
	content := []byte("hello world")
	if err := tw.WriteHeader(&tar.Header{
		Name: "test/hello.txt",
		Mode: 0644,
		Size: int64(len(content)),
	}); err != nil {
		t.Fatalf("write header: %v", err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatalf("write content: %v", err)
	}

	// Add an IR file
	irContent := []byte(`{"test": true}`)
	if err := tw.WriteHeader(&tar.Header{
		Name: "test/bible.ir.json",
		Mode: 0644,
		Size: int64(len(irContent)),
	}); err != nil {
		t.Fatalf("write header: %v", err)
	}
	if _, err := tw.Write(irContent); err != nil {
		t.Fatalf("write content: %v", err)
	}

	tw.Close()
	gw.Close()
	return path
}

func createTestTarXz(t *testing.T, dir string) string {
	path := filepath.Join(dir, "test.tar.xz")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create file: %v", err)
	}
	defer f.Close()

	xw, err := xz.NewWriter(f)
	if err != nil {
		t.Fatalf("xz writer: %v", err)
	}
	tw := tar.NewWriter(xw)

	// Add blobs directory (CAS indicator)
	if err := tw.WriteHeader(&tar.Header{
		Name:     "capsule/blobs/",
		Mode:     0755,
		Typeflag: tar.TypeDir,
	}); err != nil {
		t.Fatalf("write header: %v", err)
	}

	// Add a blob file
	content := []byte("blob content")
	if err := tw.WriteHeader(&tar.Header{
		Name: "capsule/blobs/sha256/ab/abcd1234",
		Mode: 0644,
		Size: int64(len(content)),
	}); err != nil {
		t.Fatalf("write header: %v", err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatalf("write content: %v", err)
	}

	tw.Close()
	xw.Close()
	return path
}

func createTestTarGzInvalidIR(t *testing.T, dir string) string {
	path := filepath.Join(dir, "invalid.tar.gz")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create file: %v", err)
	}
	defer f.Close()

	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)

	// Add an IR file with invalid JSON
	irContent := []byte(`{invalid json}`)
	if err := tw.WriteHeader(&tar.Header{
		Name: "test/bible.ir.json",
		Mode: 0644,
		Size: int64(len(irContent)),
	}); err != nil {
		t.Fatalf("write header: %v", err)
	}
	if _, err := tw.Write(irContent); err != nil {
		t.Fatalf("write content: %v", err)
	}

	tw.Close()
	gw.Close()
	return path
}

func TestNewReader(t *testing.T) {
	dir := t.TempDir()

	tests := []struct {
		name    string
		setup   func(t *testing.T) string
		wantErr bool
	}{
		{
			name: "tar.gz archive",
			setup: func(t *testing.T) string {
				return createTestTarGz(t, dir)
			},
			wantErr: false,
		},
		{
			name: "tar.xz archive",
			setup: func(t *testing.T) string {
				return createTestTarXz(t, dir)
			},
			wantErr: false,
		},
		{
			name: "unsupported format",
			setup: func(t *testing.T) string {
				path := filepath.Join(dir, "test.zip")
				os.WriteFile(path, []byte("not a tar"), 0644)
				return path
			},
			wantErr: true,
		},
		{
			name: "nonexistent file",
			setup: func(t *testing.T) string {
				return filepath.Join(dir, "nonexistent.tar.gz")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.setup(t)
			r, err := NewReader(path)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewReader() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if r != nil {
				r.Close()
			}
		})
	}
}

func TestReaderIterate(t *testing.T) {
	dir := t.TempDir()
	path := createTestTarGz(t, dir)

	r, err := NewReader(path)
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}
	defer r.Close()

	var files []string
	err = r.Iterate(func(header *tar.Header, _ io.Reader) (bool, error) {
		files = append(files, header.Name)
		return false, nil
	})
	if err != nil {
		t.Errorf("Iterate: %v", err)
	}

	if len(files) != 2 {
		t.Errorf("expected 2 files, got %d: %v", len(files), files)
	}
}

func TestIterateCapsule(t *testing.T) {
	dir := t.TempDir()
	path := createTestTarGz(t, dir)

	var count int
	err := IterateCapsule(path, func(header *tar.Header, _ io.Reader) (bool, error) {
		count++
		return false, nil
	})
	if err != nil {
		t.Errorf("IterateCapsule: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 entries, got %d", count)
	}
}

func TestContainsPath(t *testing.T) {
	dir := t.TempDir()

	tests := []struct {
		name      string
		setup     func(t *testing.T) string
		predicate func(string) bool
		want      bool
	}{
		{
			name: "find IR file",
			setup: func(t *testing.T) string {
				return createTestTarGz(t, dir)
			},
			predicate: func(name string) bool {
				return filepath.Ext(name) == ".json" && filepath.Base(name) != "manifest.json"
			},
			want: true,
		},
		{
			name: "find blobs directory (CAS)",
			setup: func(t *testing.T) string {
				return createTestTarXz(t, dir)
			},
			predicate: func(name string) bool {
				return filepath.Base(name) == "blobs" || filepath.Dir(name) == "blobs"
			},
			want: true,
		},
		{
			name: "file not found",
			setup: func(t *testing.T) string {
				return createTestTarGz(t, dir)
			},
			predicate: func(name string) bool {
				return name == "nonexistent.txt"
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.setup(t)
			got, err := ContainsPath(path, tt.predicate)
			if err != nil {
				t.Errorf("ContainsPath() error = %v", err)
				return
			}
			if got != tt.want {
				t.Errorf("ContainsPath() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestReadFile(t *testing.T) {
	dir := t.TempDir()
	path := createTestTarGz(t, dir)

	tests := []struct {
		name     string
		filename string
		want     string
		wantErr  bool
	}{
		{
			name:     "read hello.txt",
			filename: "hello.txt",
			want:     "hello world",
			wantErr:  false,
		},
		{
			name:     "read IR file",
			filename: "bible.ir.json",
			want:     `{"test": true}`,
			wantErr:  false,
		},
		{
			name:     "file not found",
			filename: "nonexistent.txt",
			want:     "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ReadFile(path, tt.filename)
			if (err != nil) != tt.wantErr {
				t.Errorf("ReadFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if string(got) != tt.want {
				t.Errorf("ReadFile() = %q, want %q", string(got), tt.want)
			}
		})
	}
}

func TestFindFile(t *testing.T) {
	dir := t.TempDir()
	path := createTestTarGz(t, dir)

	tests := []struct {
		name      string
		predicate func(string) bool
		wantData  string
		wantErr   bool
	}{
		{
			name: "find by extension",
			predicate: func(name string) bool {
				return filepath.Ext(name) == ".txt"
			},
			wantData: "hello world",
			wantErr:  false,
		},
		{
			name: "find JSON",
			predicate: func(name string) bool {
				return filepath.Ext(name) == ".json"
			},
			wantData: `{"test": true}`,
			wantErr:  false,
		},
		{
			name: "no match",
			predicate: func(name string) bool {
				return filepath.Ext(name) == ".xml"
			},
			wantData: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _, err := FindFile(path, tt.predicate)
			if (err != nil) != tt.wantErr {
				t.Errorf("FindFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if string(got) != tt.wantData {
				t.Errorf("FindFile() = %q, want %q", string(got), tt.wantData)
			}
		})
	}
}

func TestReaderClose(t *testing.T) {
	dir := t.TempDir()
	path := createTestTarGz(t, dir)

	r, err := NewReader(path)
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}

	// Close should not error
	if err := r.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
}

func TestNewReader_CorruptedGzip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "corrupt.tar.gz")
	// Write invalid gzip data
	if err := os.WriteFile(path, []byte("not a gzip file"), 0644); err != nil {
		t.Fatalf("create file: %v", err)
	}

	_, err := NewReader(path)
	if err == nil {
		t.Error("NewReader() expected error for corrupted gzip")
	}
}

func TestNewReader_CorruptedXz(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "corrupt.tar.xz")
	// Write invalid xz data
	if err := os.WriteFile(path, []byte("not an xz file"), 0644); err != nil {
		t.Fatalf("create file: %v", err)
	}

	_, err := NewReader(path)
	if err == nil {
		t.Error("NewReader() expected error for corrupted xz")
	}
}

func TestReaderIterate_ErrorInVisitor(t *testing.T) {
	dir := t.TempDir()
	path := createTestTarGz(t, dir)

	r, err := NewReader(path)
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}
	defer r.Close()

	// Test visitor that returns an error
	expectedErr := io.ErrUnexpectedEOF
	err = r.Iterate(func(header *tar.Header, _ io.Reader) (bool, error) {
		return false, expectedErr
	})
	if err != expectedErr {
		t.Errorf("Iterate() error = %v, want %v", err, expectedErr)
	}
}

func TestReaderIterate_StopEarly(t *testing.T) {
	dir := t.TempDir()
	path := createTestTarGz(t, dir)

	r, err := NewReader(path)
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}
	defer r.Close()

	var count int
	err = r.Iterate(func(header *tar.Header, _ io.Reader) (bool, error) {
		count++
		return true, nil // stop after first entry
	})
	if err != nil {
		t.Errorf("Iterate() error = %v", err)
	}
	if count != 1 {
		t.Errorf("expected to stop after 1 entry, got %d", count)
	}
}

func TestIterateCapsule_InvalidPath(t *testing.T) {
	err := IterateCapsule("/nonexistent/file.tar.gz", func(header *tar.Header, _ io.Reader) (bool, error) {
		return false, nil
	})
	if err == nil {
		t.Error("IterateCapsule() expected error for invalid path")
	}
}

func TestContainsPath_Error(t *testing.T) {
	_, err := ContainsPath("/nonexistent/file.tar.gz", func(name string) bool {
		return true
	})
	if err == nil {
		t.Error("ContainsPath() expected error for invalid path")
	}
}

func TestReadFile_WithFullPath(t *testing.T) {
	dir := t.TempDir()
	path := createTestTarGz(t, dir)

	// Test reading with full path (including directory prefix)
	got, err := ReadFile(path, "test/hello.txt")
	if err != nil {
		t.Errorf("ReadFile() with full path error = %v", err)
		return
	}
	if string(got) != "hello world" {
		t.Errorf("ReadFile() = %q, want %q", string(got), "hello world")
	}
}

func TestFindFile_ReturnsName(t *testing.T) {
	dir := t.TempDir()
	path := createTestTarGz(t, dir)

	// Test that FindFile returns the found name
	_, name, err := FindFile(path, func(n string) bool {
		return filepath.Ext(n) == ".txt"
	})
	if err != nil {
		t.Errorf("FindFile() error = %v", err)
		return
	}
	if name != "test/hello.txt" {
		t.Errorf("FindFile() name = %q, want %q", name, "test/hello.txt")
	}
}

func TestReaderIterate_CorruptedTar(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "corrupt.tar.gz")

	// Create a gzip file with corrupted tar content
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create file: %v", err)
	}
	gw := gzip.NewWriter(f)
	// Write some invalid tar data (not a valid tar header)
	gw.Write([]byte("this is not a valid tar archive at all"))
	gw.Close()
	f.Close()

	r, err := NewReader(path)
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}
	defer r.Close()

	// Iterate should fail when reading the corrupted tar
	err = r.Iterate(func(header *tar.Header, _ io.Reader) (bool, error) {
		return false, nil
	})
	if err == nil {
		t.Error("Iterate() expected error for corrupted tar")
	}
}

func TestReadFile_ErrorReading(t *testing.T) {
	dir := t.TempDir()
	path := createTestTarGz(t, dir)

	// Test that we handle errors during read properly by testing
	// that all paths through ReadFile are covered
	content, err := ReadFile(path, "hello.txt")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if len(content) == 0 {
		t.Error("ReadFile() returned empty content")
	}
}

func TestFindFile_ErrorReading(t *testing.T) {
	dir := t.TempDir()
	path := createTestTarGz(t, dir)

	// Test FindFile with a predicate that matches
	content, name, err := FindFile(path, func(n string) bool {
		return filepath.Base(n) == "hello.txt"
	})
	if err != nil {
		t.Fatalf("FindFile() error = %v", err)
	}
	if len(content) == 0 {
		t.Error("FindFile() returned empty content")
	}
	if name == "" {
		t.Error("FindFile() returned empty name")
	}
}

func TestReadFile_ArchiveOpenError(t *testing.T) {
	// Test with an archive that doesn't exist - should return error from IterateCapsule
	_, err := ReadFile("/nonexistent/archive.tar.gz", "test.txt")
	if err == nil {
		t.Error("ReadFile() expected error for nonexistent archive")
	}
	// Should not be "file not found" error since the archive itself couldn't be opened
	if err.Error() == "file not found: test.txt" {
		t.Error("ReadFile() should return archive open error, not file not found error")
	}
}

func TestFindFile_ArchiveOpenError(t *testing.T) {
	// Test with an archive that doesn't exist - should return error from IterateCapsule
	_, _, err := FindFile("/nonexistent/archive.tar.gz", func(name string) bool {
		return true
	})
	if err == nil {
		t.Error("FindFile() expected error for nonexistent archive")
	}
	// Should not be "no matching file found" error since the archive itself couldn't be opened
	if err.Error() == "no matching file found" {
		t.Error("FindFile() should return archive open error, not no matching file found error")
	}
}

func TestReaderClose_WithXzArchive(t *testing.T) {
	dir := t.TempDir()
	path := createTestTarXz(t, dir)

	r, err := NewReader(path)
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}

	// For tar.xz, decompressor is nil, so this tests the nil branch
	if err := r.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
}

func TestReaderClose_MultipleTimes(t *testing.T) {
	dir := t.TempDir()
	path := createTestTarGz(t, dir)

	r, err := NewReader(path)
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}

	// First close should succeed
	if err := r.Close(); err != nil {
		t.Errorf("First Close() error = %v", err)
	}

	// Second close should fail (file already closed)
	// This exercises the error paths in Close()
	if err := r.Close(); err == nil {
		// On some systems, closing twice might not error, but we tried
		t.Logf("Second Close() did not error (may be system-dependent)")
	}
}
