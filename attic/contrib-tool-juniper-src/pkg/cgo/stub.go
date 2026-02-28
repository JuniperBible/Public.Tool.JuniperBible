//go:build !cgo || !libsword
// +build !cgo !libsword

// Package cgo provides stub implementations when CGo/libsword is unavailable.
//
// This package provides placeholder types that return appropriate errors
// when libsword is not available. This allows tests to gracefully skip
// CGo comparison tests.
package cgo

import "errors"

// ErrCGoNotAvailable is returned when CGo bindings are not compiled in.
var ErrCGoNotAvailable = errors.New("CGo libsword bindings not available - build with: go build -tags libsword")

// SwordManager is a stub for when CGo is unavailable.
type SwordManager struct{}

// SwordModule is a stub for when CGo is unavailable.
type SwordModule struct{}

// NewSwordManager returns an error when CGo is unavailable.
func NewSwordManager(configPath string) (*SwordManager, error) {
	return nil, ErrCGoNotAvailable
}

// Close is a no-op for the stub.
func (m *SwordManager) Close() {}

// GetModule returns an error when CGo is unavailable.
func (m *SwordManager) GetModule(name string) (*SwordModule, error) {
	return nil, ErrCGoNotAvailable
}

// GetVerse returns an error when CGo is unavailable.
func (mod *SwordModule) GetVerse(reference string) (string, error) {
	return "", ErrCGoNotAvailable
}

// GetVerseRaw returns an error when CGo is unavailable.
func (mod *SwordModule) GetVerseRaw(reference string) (string, error) {
	return "", ErrCGoNotAvailable
}

// Description returns empty when CGo is unavailable.
func (mod *SwordModule) Description() string {
	return ""
}

// Type returns empty when CGo is unavailable.
func (mod *SwordModule) Type() string {
	return ""
}

// Language returns empty when CGo is unavailable.
func (mod *SwordModule) Language() string {
	return ""
}

// Name returns empty when CGo is unavailable.
func (mod *SwordModule) Name() string {
	return ""
}

// IsCGoAvailable returns false for the stub.
func IsCGoAvailable() bool {
	return false
}
