package capsule

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/FocuswithJustin/JuniperBible/core/ir"
)

// BenchmarkPack benchmarks packing a capsule into a tar.xz archive.
func BenchmarkPack(b *testing.B) {
	// Create temporary directory for the benchmark
	tempDir, err := os.MkdirTemp("", "capsule-bench-pack-*")
	if err != nil {
		b.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a capsule with test data
	capsuleDir := filepath.Join(tempDir, "capsule")
	capsule, err := New(capsuleDir)
	if err != nil {
		b.Fatalf("failed to create capsule: %v", err)
	}

	// Ingest several test files to make the benchmark realistic
	for i := 0; i < 10; i++ {
		testFilePath := filepath.Join(tempDir, "test-file-"+string(rune('0'+i))+".txt")
		testContent := []byte("This is test content for file number " + string(rune('0'+i)) + ". " +
			"It contains some repetitive text to simulate real file content. " +
			"Lorem ipsum dolor sit amet, consectetur adipiscing elit. " +
			"Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua.")
		if err := os.WriteFile(testFilePath, testContent, 0600); err != nil {
			b.Fatalf("failed to write test file: %v", err)
		}

		_, err = capsule.IngestFile(testFilePath)
		if err != nil {
			b.Fatalf("failed to ingest file: %v", err)
		}
	}

	archivePath := filepath.Join(tempDir, "test.capsule.tar.xz")

	// Reset timer after setup
	b.ResetTimer()

	// Run the benchmark
	for i := 0; i < b.N; i++ {
		// Remove archive from previous iteration
		os.Remove(archivePath)

		err := capsule.Pack(archivePath)
		if err != nil {
			b.Fatalf("failed to pack capsule: %v", err)
		}
	}

	b.StopTimer()

	// Report archive size
	info, err := os.Stat(archivePath)
	if err == nil {
		b.ReportMetric(float64(info.Size()), "bytes")
	}
}

// BenchmarkUnpack benchmarks unpacking a capsule from a tar.xz archive.
func BenchmarkUnpack(b *testing.B) {
	// Create temporary directory for the benchmark
	tempDir, err := os.MkdirTemp("", "capsule-bench-unpack-*")
	if err != nil {
		b.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create and pack a capsule to use for unpacking
	capsuleDir := filepath.Join(tempDir, "capsule")
	capsule, err := New(capsuleDir)
	if err != nil {
		b.Fatalf("failed to create capsule: %v", err)
	}

	// Ingest test files
	for i := 0; i < 10; i++ {
		testFilePath := filepath.Join(tempDir, "test-file-"+string(rune('0'+i))+".txt")
		testContent := []byte("This is test content for file number " + string(rune('0'+i)) + ". " +
			"It contains some repetitive text to simulate real file content. " +
			"Lorem ipsum dolor sit amet, consectetur adipiscing elit. " +
			"Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua.")
		if err := os.WriteFile(testFilePath, testContent, 0600); err != nil {
			b.Fatalf("failed to write test file: %v", err)
		}

		_, err = capsule.IngestFile(testFilePath)
		if err != nil {
			b.Fatalf("failed to ingest file: %v", err)
		}
	}

	archivePath := filepath.Join(tempDir, "test.capsule.tar.xz")
	err = capsule.Pack(archivePath)
	if err != nil {
		b.Fatalf("failed to pack capsule: %v", err)
	}

	// Reset timer after setup
	b.ResetTimer()

	// Run the benchmark
	for i := 0; i < b.N; i++ {
		unpackDir := filepath.Join(tempDir, "unpack-"+string(rune('0'+(i%10))))

		// Clean up previous unpack directory
		os.RemoveAll(unpackDir)

		_, err := Unpack(archivePath, unpackDir)
		if err != nil {
			b.Fatalf("failed to unpack capsule: %v", err)
		}

		// Clean up for next iteration
		b.StopTimer()
		os.RemoveAll(unpackDir)
		b.StartTimer()
	}
}

// BenchmarkIngestFile benchmarks ingesting a file into a capsule.
func BenchmarkIngestFile(b *testing.B) {
	// Test with different file sizes
	fileSizes := []struct {
		name string
		size int
	}{
		{"1KB", 1024},
		{"10KB", 10 * 1024},
		{"100KB", 100 * 1024},
		{"1MB", 1024 * 1024},
	}

	for _, fs := range fileSizes {
		b.Run(fs.name, func(b *testing.B) {
			// Create temporary directory
			tempDir, err := os.MkdirTemp("", "capsule-bench-ingest-*")
			if err != nil {
				b.Fatalf("failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(tempDir)

			// Create test file of specified size
			testFilePath := filepath.Join(tempDir, "test-file.bin")
			testContent := make([]byte, fs.size)
			// Fill with some pattern to make it more realistic
			for i := range testContent {
				testContent[i] = byte(i % 256)
			}
			if err := os.WriteFile(testFilePath, testContent, 0600); err != nil {
				b.Fatalf("failed to write test file: %v", err)
			}

			// Create capsule
			capsuleDir := filepath.Join(tempDir, "capsule")
			capsule, err := New(capsuleDir)
			if err != nil {
				b.Fatalf("failed to create capsule: %v", err)
			}

			// Reset timer after setup
			b.ResetTimer()

			// Run the benchmark
			for i := 0; i < b.N; i++ {
				_, err := capsule.IngestFile(testFilePath)
				if err != nil {
					b.Fatalf("failed to ingest file: %v", err)
				}

				// Clean up artifact from manifest to allow re-ingestion
				b.StopTimer()
				// Find and remove the artifact from the manifest
				for id := range capsule.Manifest.Artifacts {
					delete(capsule.Manifest.Artifacts, id)
					break
				}
				b.StartTimer()
			}

			b.StopTimer()
			b.ReportMetric(float64(fs.size), "bytes")
		})
	}
}

// BenchmarkStoreIR benchmarks storing an IR corpus in a capsule.
func BenchmarkStoreIR(b *testing.B) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "capsule-bench-storeir-*")
	if err != nil {
		b.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create capsule
	capsuleDir := filepath.Join(tempDir, "capsule")
	capsule, err := New(capsuleDir)
	if err != nil {
		b.Fatalf("failed to create capsule: %v", err)
	}

	// Create a sample IR corpus
	corpus := &ir.Corpus{
		ID:            "TEST",
		Version:       "1.0.0",
		ModuleType:    ir.ModuleBible,
		Versification: "KJV",
		Language:      "en",
		Title:         "Test Bible",
		Documents: []*ir.Document{
			{
				ID:    "Gen",
				Title: "Genesis",
				Order: 1,
				ContentBlocks: []*ir.ContentBlock{
					{
						ID:       "Gen.1.1",
						Sequence: 0,
						Text:     "In the beginning God created the heaven and the earth.",
					},
					{
						ID:       "Gen.1.2",
						Sequence: 1,
						Text:     "And the earth was without form, and void; and darkness was upon the face of the deep.",
					},
				},
			},
		},
	}

	// Reset timer after setup
	b.ResetTimer()

	// Run the benchmark
	for i := 0; i < b.N; i++ {
		_, err := capsule.StoreIR(corpus, "source-artifact")
		if err != nil {
			b.Fatalf("failed to store IR: %v", err)
		}

		// Clean up for next iteration
		b.StopTimer()
		// Remove the artifacts
		for id := range capsule.Manifest.Artifacts {
			if id != "source-artifact" {
				delete(capsule.Manifest.Artifacts, id)
			}
		}
		// Remove IR extractions
		for id := range capsule.Manifest.IRExtractions {
			delete(capsule.Manifest.IRExtractions, id)
		}
		b.StartTimer()
	}
}

// BenchmarkLoadIR benchmarks loading an IR corpus from a capsule.
func BenchmarkLoadIR(b *testing.B) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "capsule-bench-loadir-*")
	if err != nil {
		b.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create capsule
	capsuleDir := filepath.Join(tempDir, "capsule")
	capsule, err := New(capsuleDir)
	if err != nil {
		b.Fatalf("failed to create capsule: %v", err)
	}

	// Create and store a sample IR corpus
	corpus := &ir.Corpus{
		ID:            "TEST",
		Version:       "1.0.0",
		ModuleType:    ir.ModuleBible,
		Versification: "KJV",
		Language:      "en",
		Title:         "Test Bible",
		Documents: []*ir.Document{
			{
				ID:    "Gen",
				Title: "Genesis",
				Order: 1,
				ContentBlocks: []*ir.ContentBlock{
					{
						ID:       "Gen.1.1",
						Sequence: 0,
						Text:     "In the beginning God created the heaven and the earth.",
					},
				},
			},
		},
	}

	artifact, err := capsule.StoreIR(corpus, "source-artifact")
	if err != nil {
		b.Fatalf("failed to store IR: %v", err)
	}

	// Reset timer after setup
	b.ResetTimer()

	// Run the benchmark
	for i := 0; i < b.N; i++ {
		_, err := capsule.LoadIR(artifact.ID)
		if err != nil {
			b.Fatalf("failed to load IR: %v", err)
		}
	}
}
