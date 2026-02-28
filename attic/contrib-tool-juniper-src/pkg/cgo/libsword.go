//go:build cgo && libsword
// +build cgo,libsword

// Package cgo provides CGo bindings to libsword for reference testing.
//
// This package is only compiled when both CGo and libsword are available.
// It serves as the reference implementation for validating the pure Go parsers.
//
// Build requirements:
//   - libsword-dev installed (apt install libsword-dev)
//   - CGo enabled (CGO_ENABLED=1)
//   - Build tag: -tags libsword
//
// Usage:
//
//	go test -tags libsword ./pkg/cgo/...
package cgo

/*
#cgo CFLAGS: -I/usr/include
#cgo LDFLAGS: -lsword

#include <stdlib.h>
#include <string.h>
#include <swmgr.h>
#include <swmodule.h>
#include <versekey.h>

// Helper to create SWMgr
void* create_swmgr(const char* path) {
    return new sword::SWMgr(path, true, NULL, false, false);
}

// Helper to destroy SWMgr
void destroy_swmgr(void* mgr) {
    delete (sword::SWMgr*)mgr;
}

// Get module by name
void* get_module(void* mgr, const char* name) {
    sword::SWMgr* swmgr = (sword::SWMgr*)mgr;
    return swmgr->getModule(name);
}

// Get verse text
const char* get_verse(void* mod, const char* ref) {
    sword::SWModule* swmod = (sword::SWModule*)mod;
    swmod->setKey(ref);
    return swmod->renderText();
}

// Get raw verse text (no rendering)
const char* get_verse_raw(void* mod, const char* ref) {
    sword::SWModule* swmod = (sword::SWModule*)mod;
    swmod->setKey(ref);
    return swmod->getRawEntry();
}

// Get module description
const char* get_description(void* mod) {
    sword::SWModule* swmod = (sword::SWModule*)mod;
    return swmod->getDescription();
}

// Get module type
const char* get_type(void* mod) {
    sword::SWModule* swmod = (sword::SWModule*)mod;
    return swmod->getType();
}

// Get module language
const char* get_language(void* mod) {
    sword::SWModule* swmod = (sword::SWModule*)mod;
    return swmod->getLanguage();
}
*/
import "C"

import (
	"fmt"
	"unsafe"
)

// SwordManager wraps the libsword SWMgr.
type SwordManager struct {
	ptr unsafe.Pointer
}

// SwordModule wraps a libsword SWModule.
type SwordModule struct {
	ptr  unsafe.Pointer
	name string
}

// NewSwordManager creates a new SWORD manager.
func NewSwordManager(configPath string) (*SwordManager, error) {
	cpath := C.CString(configPath)
	defer C.free(unsafe.Pointer(cpath))

	ptr := C.create_swmgr(cpath)
	if ptr == nil {
		return nil, fmt.Errorf("failed to create SWMgr for path: %s", configPath)
	}

	return &SwordManager{ptr: ptr}, nil
}

// Close releases the SWORD manager resources.
func (m *SwordManager) Close() {
	if m.ptr != nil {
		C.destroy_swmgr(m.ptr)
		m.ptr = nil
	}
}

// GetModule retrieves a module by name.
func (m *SwordManager) GetModule(name string) (*SwordModule, error) {
	cname := C.CString(name)
	defer C.free(unsafe.Pointer(cname))

	ptr := C.get_module(m.ptr, cname)
	if ptr == nil {
		return nil, fmt.Errorf("module not found: %s", name)
	}

	return &SwordModule{ptr: ptr, name: name}, nil
}

// GetVerse retrieves rendered verse text.
func (mod *SwordModule) GetVerse(reference string) (string, error) {
	cref := C.CString(reference)
	defer C.free(unsafe.Pointer(cref))

	ctext := C.get_verse(mod.ptr, cref)
	if ctext == nil {
		return "", fmt.Errorf("failed to get verse: %s", reference)
	}

	return C.GoString(ctext), nil
}

// GetVerseRaw retrieves raw verse text without rendering.
func (mod *SwordModule) GetVerseRaw(reference string) (string, error) {
	cref := C.CString(reference)
	defer C.free(unsafe.Pointer(cref))

	ctext := C.get_verse_raw(mod.ptr, cref)
	if ctext == nil {
		return "", fmt.Errorf("failed to get raw verse: %s", reference)
	}

	return C.GoString(ctext), nil
}

// Description returns the module description.
func (mod *SwordModule) Description() string {
	cdesc := C.get_description(mod.ptr)
	return C.GoString(cdesc)
}

// Type returns the module type (Bible, Commentary, etc).
func (mod *SwordModule) Type() string {
	ctype := C.get_type(mod.ptr)
	return C.GoString(ctype)
}

// Language returns the module language.
func (mod *SwordModule) Language() string {
	clang := C.get_language(mod.ptr)
	return C.GoString(clang)
}

// Name returns the module name.
func (mod *SwordModule) Name() string {
	return mod.name
}
