package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestReadBlockIndex tests reading a .bzs block index file.
func TestReadBlockIndex(t *testing.T) {
	// Create a temporary block index file
	tmpDir, err := os.MkdirTemp("", "ztext-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test data: 2 blocks, 12 bytes each
	// Block 0: offset=0, compSize=100, uncompSize=200
	// Block 1: offset=100, compSize=150, uncompSize=300
	data := []byte{
		0x00, 0x00, 0x00, 0x00, // offset 0
		0x64, 0x00, 0x00, 0x00, // compSize 100
		0xC8, 0x00, 0x00, 0x00, // uncompSize 200
		0x64, 0x00, 0x00, 0x00, // offset 100
		0x96, 0x00, 0x00, 0x00, // compSize 150
		0x2C, 0x01, 0x00, 0x00, // uncompSize 300
	}

	bzsPath := filepath.Join(tmpDir, "test.bzs")
	if err := os.WriteFile(bzsPath, data, 0600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	blocks, err := readBlockIndex(bzsPath)
	if err != nil {
		t.Fatalf("readBlockIndex failed: %v", err)
	}

	if len(blocks) != 2 {
		t.Errorf("expected 2 blocks, got %d", len(blocks))
	}

	// Check first block
	if blocks[0].Offset != 0 {
		t.Errorf("block[0].Offset: expected 0, got %d", blocks[0].Offset)
	}
	if blocks[0].CompressedSize != 100 {
		t.Errorf("block[0].CompressedSize: expected 100, got %d", blocks[0].CompressedSize)
	}
	if blocks[0].UncompSize != 200 {
		t.Errorf("block[0].UncompSize: expected 200, got %d", blocks[0].UncompSize)
	}

	// Check second block
	if blocks[1].Offset != 100 {
		t.Errorf("block[1].Offset: expected 100, got %d", blocks[1].Offset)
	}
	if blocks[1].CompressedSize != 150 {
		t.Errorf("block[1].CompressedSize: expected 150, got %d", blocks[1].CompressedSize)
	}
	if blocks[1].UncompSize != 300 {
		t.Errorf("block[1].UncompSize: expected 300, got %d", blocks[1].UncompSize)
	}
}

// TestReadVerseIndex tests reading a .bzv verse index file.
func TestReadVerseIndex(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "ztext-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test data: 2 verses, 10 bytes each
	// Verse 0: blockNum=0, offset=0, size=50
	// Verse 1: blockNum=0, offset=50, size=75
	data := []byte{
		0x00, 0x00, 0x00, 0x00, // blockNum 0
		0x00, 0x00, 0x00, 0x00, // offset 0
		0x32, 0x00, // size 50
		0x00, 0x00, 0x00, 0x00, // blockNum 0
		0x32, 0x00, 0x00, 0x00, // offset 50
		0x4B, 0x00, // size 75
	}

	bzvPath := filepath.Join(tmpDir, "test.bzv")
	if err := os.WriteFile(bzvPath, data, 0600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	verses, err := readVerseIndex(bzvPath)
	if err != nil {
		t.Fatalf("readVerseIndex failed: %v", err)
	}

	if len(verses) != 2 {
		t.Errorf("expected 2 verses, got %d", len(verses))
	}

	// Check first verse
	if verses[0].BlockNum != 0 {
		t.Errorf("verse[0].BlockNum: expected 0, got %d", verses[0].BlockNum)
	}
	if verses[0].Offset != 0 {
		t.Errorf("verse[0].Offset: expected 0, got %d", verses[0].Offset)
	}
	if verses[0].Size != 50 {
		t.Errorf("verse[0].Size: expected 50, got %d", verses[0].Size)
	}

	// Check second verse
	if verses[1].Offset != 50 {
		t.Errorf("verse[1].Offset: expected 50, got %d", verses[1].Offset)
	}
	if verses[1].Size != 75 {
		t.Errorf("verse[1].Size: expected 75, got %d", verses[1].Size)
	}
}

// TestGetTotalVerses tests verse count lookup via versification.
func TestGetTotalVerses(t *testing.T) {
	v, err := NewVersification(VersKJV)
	if err != nil {
		t.Fatalf("failed to create versification: %v", err)
	}

	tests := []struct {
		book     string
		expected int
	}{
		{"Gen", 1533},
		{"Ps", 2461},
		{"Matt", 1071},
		{"Rev", 404},
		{"Unknown", 0},
	}

	for _, tt := range tests {
		got := v.GetTotalVerses(tt.book)
		if got != tt.expected {
			t.Errorf("GetTotalVerses(%q): expected %d, got %d", tt.book, tt.expected, got)
		}
	}
}

// TestOpenZTextModuleWithRealData tests opening a real SWORD module.
func TestOpenZTextModuleWithRealData(t *testing.T) {
	// Check if sample data exists
	swordPath := "/home/justin/Programming/Workspace/mimicry/contrib/sample-data/kjv"
	confPath := filepath.Join(swordPath, "mods.d", "kjv.conf")

	if _, err := os.Stat(confPath); os.IsNotExist(err) {
		t.Skip("KJV sample data not available")
	}

	conf, err := ParseConfFile(confPath)
	if err != nil {
		t.Fatalf("failed to parse conf: %v", err)
	}

	mod, err := OpenZTextModule(conf, swordPath)
	if err != nil {
		t.Fatalf("failed to open module: %v", err)
	}

	// Check that both testaments are loaded
	if !mod.HasOT() {
		t.Error("expected module to have OT")
	}
	if !mod.HasNT() {
		t.Error("expected module to have NT")
	}

	// Check module info
	info := mod.GetModuleInfo()
	if info.Name != "KJV" {
		t.Errorf("expected module name 'KJV', got %q", info.Name)
	}
	if info.Type != "Bible" {
		t.Errorf("expected module type 'Bible', got %q", info.Type)
	}
}

// TestGetVerseTextWithRealData tests reading actual verse text from KJV module.
func TestGetVerseTextWithRealData(t *testing.T) {
	swordPath := "/home/justin/Programming/Workspace/mimicry/contrib/sample-data/kjv"
	confPath := filepath.Join(swordPath, "mods.d", "kjv.conf")

	if _, err := os.Stat(confPath); os.IsNotExist(err) {
		t.Skip("KJV sample data not available")
	}

	conf, err := ParseConfFile(confPath)
	if err != nil {
		t.Fatalf("failed to parse conf: %v", err)
	}

	mod, err := OpenZTextModule(conf, swordPath)
	if err != nil {
		t.Fatalf("failed to open module: %v", err)
	}

	// Test Genesis 1:1
	ref := &Ref{Book: "Gen", Chapter: 1, Verse: 1}
	text, err := mod.GetVerseText(ref)
	if err != nil {
		t.Fatalf("GetVerseText error: %v", err)
	}

	// Should contain "In the beginning" (with Strong's numbers in KJV)
	if len(text) == 0 {
		t.Fatal("Got empty text for Genesis 1:1")
	}

	// The KJV text contains "beginning" with Strong's markup
	if !strings.Contains(text, "beginning") {
		t.Errorf("Genesis 1:1 should contain 'beginning', got: %s", text[:min(200, len(text))])
	}

	t.Logf("Genesis 1:1: %s...", text[:min(100, len(text))])
}

// TestGetMultipleVerses tests reading multiple verses from different locations.
func TestGetMultipleVerses(t *testing.T) {
	swordPath := "/home/justin/Programming/Workspace/mimicry/contrib/sample-data/kjv"
	confPath := filepath.Join(swordPath, "mods.d", "kjv.conf")

	if _, err := os.Stat(confPath); os.IsNotExist(err) {
		t.Skip("KJV sample data not available")
	}

	conf, err := ParseConfFile(confPath)
	if err != nil {
		t.Fatalf("failed to parse conf: %v", err)
	}

	mod, err := OpenZTextModule(conf, swordPath)
	if err != nil {
		t.Fatalf("failed to open module: %v", err)
	}

	// Test several well-known verses
	tests := []struct {
		ref      *Ref
		contains string
	}{
		{&Ref{Book: "Gen", Chapter: 1, Verse: 1}, "beginning"},
		{&Ref{Book: "Ps", Chapter: 23, Verse: 1}, "shepherd"},
		{&Ref{Book: "Isa", Chapter: 53, Verse: 5}, "wounded"},
	}

	for _, tt := range tests {
		text, err := mod.GetVerseText(tt.ref)
		if err != nil {
			t.Errorf("GetVerseText(%v) error: %v", tt.ref, err)
			continue
		}
		if !strings.Contains(strings.ToLower(text), tt.contains) {
			t.Errorf("GetVerseText(%v) should contain '%s', got: %s", tt.ref, tt.contains, text[:min(100, len(text))])
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
