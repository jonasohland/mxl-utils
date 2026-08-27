package mxl

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// Creates a directory that looks like a flow to the discovery code. The
// contents of a flow are irrelevant here, discovery only ever looks at
// directory names.
func mkFlowDir(t *testing.T, domain string) string {
	t.Helper()

	id := uuid.NewString()
	require.NoError(t, os.MkdirAll(filepath.Join(domain, id+".mxl-flow"), 0o0755))

	return id
}

func mkDir(t *testing.T, elem ...string) string {
	t.Helper()

	path := filepath.Join(elem...)
	require.NoError(t, os.MkdirAll(path, 0o0755))

	return path
}

// Records everything the discovery components report, so that tests can make
// assertions about the events themselves and about their order.
type recorder struct {
	mu     sync.Mutex
	events []string
}

func (r *recorder) record(format string, args ...any) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.events = append(r.events, fmt.Sprintf(format, args...))
}

func (r *recorder) Events() []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	return slices.Clone(r.events)
}

func (r *recorder) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.events = nil
}

func (r *recorder) Contains(event string) bool {
	return slices.Contains(r.Events(), event)
}

func (r *recorder) AddDomain(path string)    { r.record("domain-added %s", path) }
func (r *recorder) RemoveDomain(path string) { r.record("domain-removed %s", path) }

func (r *recorder) AddFlow(domain, id string)    { r.record("flow-added %s %s", domain, id) }
func (r *recorder) RemoveFlow(domain, id string) { r.record("flow-removed %s %s", domain, id) }

func (r *recorder) AddFilesystem(path string)    { r.record("fs-added %s", path) }
func (r *recorder) RemoveFilesystem(path string) { r.record("fs-removed %s", path) }
func (r *recorder) UpdateFilesystem(path string, domains []string) {
	r.record("fs-updated %s %v", path, domains)
}

func (r *recorder) requireEventually(t *testing.T, event string) {
	t.Helper()

	require.Eventuallyf(t, func() bool { return r.Contains(event) },
		5*time.Second, 10*time.Millisecond,
		"expected event %q, recorded: %v", event, r.Events())
}

func (r *recorder) requireNever(t *testing.T, event string) {
	t.Helper()

	require.Neverf(t, func() bool { return r.Contains(event) },
		250*time.Millisecond, 10*time.Millisecond,
		"unexpected event %q, recorded: %v", event, r.Events())
}
