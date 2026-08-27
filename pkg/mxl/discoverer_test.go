package mxl

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

// Builds a discoverer without starting its reload loop, so that tests can step
// through discovery cycles one at a time.
func newTestDiscoverer(recv *recorder, paths, static []string) *Discoverer {
	return &Discoverer{
		paths:  paths,
		static: static,
		cached: domainPathCache{},
		recv:   []DomainReceiver{recv},
	}
}

func TestDiscovererFindsDomains(t *testing.T) {
	root := t.TempDir()
	recv := &recorder{}

	// A domain directly below the search path, and one nested a few levels
	// deep. The intermediate directories are not domains themselves.
	shallow := mkDir(t, root, "shallow")
	mkFlowDir(t, shallow)

	deep := mkDir(t, root, "some", "nested", "path")
	mkFlowDir(t, deep)
	mkFlowDir(t, deep)

	// Neither an empty directory nor one holding only files is a domain
	mkDir(t, root, "empty")
	other := mkDir(t, root, "other")
	require.NoError(t, os.WriteFile(filepath.Join(other, "a-file"), nil, 0o0644))

	newTestDiscoverer(recv, []string{root}, nil).reload()

	require.ElementsMatch(t, []string{
		"domain-added " + shallow,
		"domain-added " + deep,
	}, recv.Events())
}

// A domain nested inside another domain is reported as a domain of its own.
func TestDiscovererFindsNestedDomains(t *testing.T) {
	root := t.TempDir()
	recv := &recorder{}

	outer := mkDir(t, root, "outer")
	mkFlowDir(t, outer)

	inner := mkDir(t, outer, "inner")
	mkFlowDir(t, inner)

	newTestDiscoverer(recv, []string{root}, nil).reload()

	require.ElementsMatch(t, []string{
		"domain-added " + outer,
		"domain-added " + inner,
	}, recv.Events())
}

func TestDiscovererReportsChangesOnly(t *testing.T) {
	root := t.TempDir()
	recv := &recorder{}

	first := mkDir(t, root, "first")
	mkFlowDir(t, first)

	d := newTestDiscoverer(recv, []string{root}, nil)

	d.reload()
	require.Equal(t, []string{"domain-added " + first}, recv.Events())

	// A domain that is still there is not reported again
	recv.Reset()
	d.reload()
	require.Empty(t, recv.Events())

	// Only the newly appeared domain is reported
	recv.Reset()
	second := mkDir(t, root, "second")
	mkFlowDir(t, second)
	d.reload()
	require.Equal(t, []string{"domain-added " + second}, recv.Events())

	// Removing the last flow of a domain removes the domain
	recv.Reset()
	require.NoError(t, os.RemoveAll(second))
	d.reload()
	require.Equal(t, []string{"domain-removed " + second}, recv.Events())
}

// Search paths that do not exist are ignored rather than reported as an error.
func TestDiscovererIgnoresMissingSearchPaths(t *testing.T) {
	root := t.TempDir()
	recv := &recorder{}

	domain := mkDir(t, root, "domain")
	mkFlowDir(t, domain)

	newTestDiscoverer(recv, []string{filepath.Join(root, "does-not-exist"), root}, nil).reload()

	require.Equal(t, []string{"domain-added " + domain}, recv.Events())
}

// Statically configured domains are owned by the caller, so discovery must not
// add or remove them, even when they show up below a search path.
func TestDiscovererIgnoresStaticDomains(t *testing.T) {
	root := t.TempDir()
	recv := &recorder{}

	static := mkDir(t, root, "static")
	mkFlowDir(t, static)

	d := newTestDiscoverer(recv, []string{root}, []string{static})

	d.reload()
	require.Empty(t, recv.Events())

	require.NoError(t, os.RemoveAll(static))
	d.reload()
	require.Empty(t, recv.Events())
}

func TestDiscovererLifecycle(t *testing.T) {
	root := t.TempDir()
	recv := &recorder{}

	discovered := mkDir(t, root, "discovered")
	mkFlowDir(t, discovered)
	static := mkDir(t, root, "static")

	ctx, cancel := context.WithCancel(t.Context())
	wg := &sync.WaitGroup{}

	NewDiscoverer(ctx, wg, []DomainReceiver{recv}, []string{root}, []string{static})

	// Static domains are reported immediately, discovered ones after the first
	// reload cycle has run
	require.Contains(t, recv.Events(), "domain-added "+static)
	recv.requireEventually(t, "domain-added "+discovered)

	// Shutting down retracts every domain that was reported
	cancel()
	wg.Wait()

	require.Contains(t, recv.Events(), "domain-removed "+static)
	require.Contains(t, recv.Events(), "domain-removed "+discovered)
}
