package mxl

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func newTestWatcher(t *testing.T, recv *recorder) *Watcher {
	t.Helper()

	ctx, cancel := context.WithCancel(t.Context())
	wg := &sync.WaitGroup{}

	w, err := NewWatcher(ctx, wg, []FlowReceiver{recv})
	require.NoError(t, err)

	t.Cleanup(func() {
		cancel()
		wg.Wait()
	})

	return w
}

// Flows that are already present when a domain is added are reported right
// away, not just on the next change.
func TestWatcherReportsExistingFlows(t *testing.T) {
	domain := t.TempDir()
	first := mkFlowDir(t, domain)
	second := mkFlowDir(t, domain)

	// Directories that are not flows are ignored
	mkDir(t, domain, "not-a-flow")

	recv := &recorder{}
	newTestWatcher(t, recv).AddDomain(domain)

	require.ElementsMatch(t, []string{
		"flow-added " + domain + " " + first,
		"flow-added " + domain + " " + second,
	}, recv.Events())
}

func TestWatcherReportsCreatedAndRemovedFlows(t *testing.T) {
	domain := t.TempDir()
	recv := &recorder{}

	newTestWatcher(t, recv).AddDomain(domain)
	require.Empty(t, recv.Events())

	id := mkFlowDir(t, domain)
	recv.requireEventually(t, "flow-added "+domain+" "+id)

	require.NoError(t, os.RemoveAll(filepath.Join(domain, id+".mxl-flow")))
	recv.requireEventually(t, "flow-removed "+domain+" "+id)
}

func TestWatcherIgnoresUnrelatedEntries(t *testing.T) {
	domain := t.TempDir()
	recv := &recorder{}

	newTestWatcher(t, recv).AddDomain(domain)

	mkDir(t, domain, "not-a-flow")
	mkDir(t, domain, "12345678.mxl-flow")
	require.NoError(t, os.WriteFile(filepath.Join(domain, "a-file"), nil, 0o0644))

	require.Never(t, func() bool { return len(recv.Events()) > 0 }, 250*time.Millisecond, 10*time.Millisecond)
}

// Removing a domain retracts all of its flows.
func TestWatcherRemoveDomain(t *testing.T) {
	domain := t.TempDir()
	id := mkFlowDir(t, domain)

	recv := &recorder{}
	w := newTestWatcher(t, recv)

	w.AddDomain(domain)
	recv.Reset()

	w.RemoveDomain(domain)
	require.Equal(t, []string{"flow-removed " + domain + " " + id}, recv.Events())

	// Flows appearing in a domain that is no longer watched are not reported
	newID := mkFlowDir(t, domain)
	recv.requireNever(t, "flow-added "+domain+" "+newID)
}

// Adding a domain that is already watched does not report its flows twice.
func TestWatcherAddDomainTwice(t *testing.T) {
	domain := t.TempDir()
	id := mkFlowDir(t, domain)

	recv := &recorder{}
	w := newTestWatcher(t, recv)

	w.AddDomain(domain)
	recv.Reset()

	w.AddDomain(domain)
	recv.requireNever(t, "flow-added "+domain+" "+id)
}

// Removing a domain that was never added is a no-op.
func TestWatcherRemoveUnknownDomain(t *testing.T) {
	recv := &recorder{}
	newTestWatcher(t, recv).RemoveDomain(t.TempDir())

	require.Empty(t, recv.Events())
}

// A domain directory that is removed and re-created is a different directory,
// which the inotify watch does not follow. The periodic refresh detects this by
// comparing inode numbers and re-attaches to the new directory.
func TestWatcherHandlesRecreatedDomain(t *testing.T) {
	root := t.TempDir()
	domain := mkDir(t, root, "domain")
	oldID := mkFlowDir(t, domain)

	recv := &recorder{}
	w := newTestWatcher(t, recv)

	w.AddDomain(domain)
	recv.Reset()

	require.NoError(t, os.RemoveAll(domain))
	mkDir(t, root, "domain")
	newID := mkFlowDir(t, domain)

	w.refreshDomains()

	require.Equal(t, []string{
		"flow-removed " + domain + " " + oldID,
		"flow-added " + domain + " " + newID,
	}, recv.Events())

	// The watch now follows the new directory
	recv.Reset()
	anotherID := mkFlowDir(t, domain)
	recv.requireEventually(t, "flow-added "+domain+" "+anotherID)
}

// An unchanged domain must not be torn down and rebuilt by the refresh.
func TestWatcherRefreshIsQuietWhenNothingChanged(t *testing.T) {
	domain := t.TempDir()
	mkFlowDir(t, domain)

	recv := &recorder{}
	w := newTestWatcher(t, recv)

	w.AddDomain(domain)
	recv.Reset()

	w.refreshDomains()
	require.Empty(t, recv.Events())
}

// A domain that disappears without being removed keeps its flows until the
// caller retracts the domain, since the domain may come back.
func TestWatcherRefreshIgnoresVanishedDomain(t *testing.T) {
	root := t.TempDir()
	domain := mkDir(t, root, "domain")
	mkFlowDir(t, domain)

	recv := &recorder{}
	w := newTestWatcher(t, recv)

	w.AddDomain(domain)
	recv.Reset()

	require.NoError(t, os.RemoveAll(domain))
	w.refreshDomains()

	require.Empty(t, recv.Events())
}
