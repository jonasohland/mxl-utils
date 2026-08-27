package mxl

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const procMounts = `proc /proc proc rw,nosuid,nodev,noexec,relatime 0 0
/dev/nvme0n1p2 / ext4 rw,relatime 0 0
/dev/nvme0n1p1 /boot vfat rw,relatime 0 0
tmpfs /dev/shm tmpfs rw,nosuid,nodev,inode64 0 0
/dev/sda1 /mnt/data ext4 rw,relatime 0 0
`

func TestParseMountPaths(t *testing.T) {
	require.Equal(t,
		[]string{"/proc", "/", "/boot", "/dev/shm", "/mnt/data"},
		parseMountPaths(strings.NewReader(procMounts)))
}

func TestParseMountPathsSkipsMalformedLines(t *testing.T) {
	require.Equal(t,
		[]string{"/", "/boot"},
		parseMountPaths(strings.NewReader("\ngarbage\n/dev/sda1 /\n/dev/nvme0n1p2 / ext4 rw 0 0\n/dev/nvme0n1p1 /boot vfat rw 0 0\n")))
}

func TestParseMountPathsEmpty(t *testing.T) {
	require.Empty(t, parseMountPaths(strings.NewReader("")))
}

func TestLongestMatchingMountPath(t *testing.T) {
	mounts := parseMountPaths(strings.NewReader(procMounts))

	for _, tc := range []struct {
		name   string
		domain string
		mount  string
		err    error
	}{
		{name: "domain on the root filesystem", domain: "/srv/domain", mount: "/"},
		{name: "the root filesystem itself", domain: "/", mount: "/"},
		{name: "longest match wins", domain: "/mnt/data/domain", mount: "/mnt/data"},
		{name: "mount path itself", domain: "/mnt/data", mount: "/mnt/data"},
		{name: "shared memory", domain: "/dev/shm/domain", mount: "/dev/shm"},
		// A mount path is only a match when it is a path prefix, sharing a
		// string prefix with a mount path is not enough
		{name: "sibling of a mount path", domain: "/mnt/database/domain", mount: "/"},
		{name: "sibling of a nested mount path", domain: "/dev/shmem/domain", mount: "/"},
		{name: "relative path", domain: "srv/domain", err: ErrFSNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mount, err := longestMatchingMountPath(tc.domain, mounts)

			if tc.err != nil {
				require.ErrorIs(t, err, tc.err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tc.mount, mount)
		})
	}
}

func TestLongestMatchingMountPathWithoutRoot(t *testing.T) {
	_, err := longestMatchingMountPath("/srv/domain", []string{"/boot", "/mnt/data"})
	require.ErrorIs(t, err, ErrFSNotFound)
}

// The mount table of the host always has a filesystem that holds the temporary
// directory, whichever one that happens to be.
func TestFindDomainFilesystem(t *testing.T) {
	path, err := findDomainFilesystem(t.TempDir())
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(path, "/"), "expected an absolute mount path, got %q", path)
}

// Builds a filesystem discoverer that resolves domains through a fixed table
// instead of the mount table of the host.
func newTestFilesystemDiscoverer(recv *recorder, resolve map[string]string) *FilesystemDiscoverer {
	fd := NewFilesystemDiscoverer([]FilesytemReceiver{recv})
	fd.resolve = func(domain string) (string, error) {
		path, ok := resolve[domain]
		if !ok {
			return "", ErrFSNotFound
		}

		return path, nil
	}

	return fd
}

func TestFilesystemDiscoverer(t *testing.T) {
	recv := &recorder{}
	fd := newTestFilesystemDiscoverer(recv, map[string]string{
		"/mnt/data/one": "/mnt/data",
		"/mnt/data/two": "/mnt/data",
		"/srv/three":    "/",
	})

	// The first domain on a filesystem brings the filesystem itself into
	// existence
	fd.AddDomain("/mnt/data/one")
	require.Equal(t, []string{
		"fs-added /mnt/data",
		"fs-updated /mnt/data [/mnt/data/one]",
	}, recv.Events())

	// Further domains on the same filesystem only update its domain list
	recv.Reset()
	fd.AddDomain("/mnt/data/two")
	require.Equal(t, []string{
		"fs-updated /mnt/data [/mnt/data/one /mnt/data/two]",
	}, recv.Events())

	// A domain on a different filesystem adds that filesystem
	recv.Reset()
	fd.AddDomain("/srv/three")
	require.Equal(t, []string{
		"fs-added /",
		"fs-updated / [/srv/three]",
	}, recv.Events())

	// Removing one of several domains keeps the filesystem alive
	recv.Reset()
	fd.RemoveDomain("/mnt/data/one")
	require.Equal(t, []string{
		"fs-updated /mnt/data [/mnt/data/two]",
	}, recv.Events())

	// Removing the last domain retires the filesystem
	recv.Reset()
	fd.RemoveDomain("/mnt/data/two")
	require.Equal(t, []string{
		"fs-removed /mnt/data",
	}, recv.Events())

	// The other filesystem is untouched by all of this
	recv.Reset()
	fd.RemoveDomain("/srv/three")
	require.Equal(t, []string{
		"fs-removed /",
	}, recv.Events())
}

// Domains whose filesystem cannot be determined are skipped, not reported.
func TestFilesystemDiscovererUnresolvableDomain(t *testing.T) {
	recv := &recorder{}
	fd := newTestFilesystemDiscoverer(recv, nil)

	fd.AddDomain("/srv/domain")
	require.Empty(t, recv.Events())

	fd.RemoveDomain("/srv/domain")
	require.Empty(t, recv.Events())
}

// Removing a domain that is not known must not disturb the filesystems that
// are.
func TestFilesystemDiscovererRemoveUnknownDomain(t *testing.T) {
	recv := &recorder{}
	fd := newTestFilesystemDiscoverer(recv, map[string]string{
		"/mnt/data/one": "/mnt/data",
		"/srv/other":    "/",
	})

	fd.AddDomain("/mnt/data/one")
	recv.Reset()

	fd.RemoveDomain("/srv/other")
	require.Empty(t, recv.Events())
}

func TestReadMountPaths(t *testing.T) {
	mounts, err := readMountPaths()
	require.NoError(t, err)
	require.NotEmpty(t, mounts)
	require.Contains(t, mounts, "/")
}
