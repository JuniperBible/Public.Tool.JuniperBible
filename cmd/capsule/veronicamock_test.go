package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"

	"github.com/JuniperBible/Public.Tool.JuniperBible/core/capsule"
	"github.com/JuniperBible/Public.Tool.JuniperBible/core/cas"
)

// testMockVeronicaCAS is a test double for cas.VeronicaCAS.
type testMockVeronicaCAS struct {
	mu    sync.Mutex
	blobs map[string][]byte
}

func newTestMockCAS() *testMockVeronicaCAS {
	return &testMockVeronicaCAS{blobs: make(map[string][]byte)}
}

func (m *testMockVeronicaCAS) Put(_ context.Context, data []byte) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	h := sha256.Sum256(data)
	digest := "sha256:" + hex.EncodeToString(h[:])
	m.blobs[digest] = append([]byte(nil), data...)
	return digest, nil
}

func (m *testMockVeronicaCAS) Get(_ context.Context, digest string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	data, ok := m.blobs[digest]
	if !ok {
		return nil, fmt.Errorf("not found: %s", digest)
	}
	return data, nil
}

// Ensure testMockVeronicaCAS implements cas.VeronicaCAS.
var _ cas.VeronicaCAS = (*testMockVeronicaCAS)(nil)

// globalTestMock is a single shared mock CAS used by all tests.
var globalTestMock = newTestMockCAS()

// testVOpt returns a CapsuleOption that uses the global shared mock CAS.
func testVOpt() capsule.CapsuleOption {
	return capsule.WithVeronicaClient(globalTestMock)
}
