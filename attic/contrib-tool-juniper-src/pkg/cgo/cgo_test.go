// Package cgo provides tests that work regardless of CGo availability.
package cgo

import (
	"errors"
	"testing"
)

func TestIsCGoAvailable(t *testing.T) {
	// This test verifies the function returns a valid boolean
	// The actual value depends on build tags
	available := IsCGoAvailable()

	// Type assertion - should be a bool
	_ = available
	t.Logf("CGo available: %v", available)
}

func TestSwordManager_Interface(t *testing.T) {
	// Test that SwordManager has the expected interface
	var mgr *SwordManager

	// Create a new manager (may fail without libsword, but tests interface)
	mgr, err := NewSwordManager("/nonexistent/path")

	if IsCGoAvailable() {
		// With CGo, we expect an error for invalid path
		if err == nil {
			t.Logf("Manager created (unexpected for nonexistent path)")
			mgr.Close()
		}
	} else {
		// Without CGo, we expect ErrCGoNotAvailable
		if !errors.Is(err, ErrCGoNotAvailable) {
			t.Errorf("Expected ErrCGoNotAvailable, got %v", err)
		}
	}
}

func TestSwordManager_Close(t *testing.T) {
	// Test that Close is safe to call on nil/stub manager
	var mgr SwordManager
	mgr.Close() // Should not panic
}

func TestSwordModule_StubMethods(t *testing.T) {
	if IsCGoAvailable() {
		t.Skip("Skipping stub test when CGo is available")
	}

	var mod SwordModule

	// All methods should return empty/error for stub
	if desc := mod.Description(); desc != "" {
		t.Errorf("Expected empty Description, got %s", desc)
	}
	if typ := mod.Type(); typ != "" {
		t.Errorf("Expected empty Type, got %s", typ)
	}
	if lang := mod.Language(); lang != "" {
		t.Errorf("Expected empty Language, got %s", lang)
	}
	if name := mod.Name(); name != "" {
		t.Errorf("Expected empty Name, got %s", name)
	}

	// GetVerse should return error
	_, err := mod.GetVerse("Gen.1.1")
	if !errors.Is(err, ErrCGoNotAvailable) {
		t.Errorf("Expected ErrCGoNotAvailable, got %v", err)
	}

	// GetVerseRaw should return error
	_, err = mod.GetVerseRaw("Gen.1.1")
	if !errors.Is(err, ErrCGoNotAvailable) {
		t.Errorf("Expected ErrCGoNotAvailable, got %v", err)
	}
}

func TestErrCGoNotAvailable(t *testing.T) {
	// Test that the error is properly defined
	if ErrCGoNotAvailable == nil {
		t.Error("ErrCGoNotAvailable should not be nil")
	}

	msg := ErrCGoNotAvailable.Error()
	if msg == "" {
		t.Error("Error message should not be empty")
	}
	t.Logf("Error message: %s", msg)
}

func TestSwordManager_GetModule_Stub(t *testing.T) {
	if IsCGoAvailable() {
		t.Skip("Skipping stub test when CGo is available")
	}

	var mgr SwordManager
	_, err := mgr.GetModule("KJV")
	if !errors.Is(err, ErrCGoNotAvailable) {
		t.Errorf("Expected ErrCGoNotAvailable, got %v", err)
	}
}

// BenchmarkIsCGoAvailable measures the overhead of checking CGo availability
func BenchmarkIsCGoAvailable(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = IsCGoAvailable()
	}
}
