package mxl

import (
	"bufio"
	"errors"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"

	"github.com/samber/lo"
)

var (
	ErrFSNotFound = errors.New("no root filesystem found for domain")
)

type FilesystemReceiver interface {
	AddFilesystem(path string)
	UpdateFilesystem(path string, domains []string)
	RemoveFilesystem(path string)
}

type FilesystemDiscoverer struct {
	mu    sync.Mutex
	roots map[string][]string
	recv  []FilesystemReceiver

	// Resolves the filesystem a domain lives on. Only ever replaced in tests.
	resolve func(domain string) (string, error)
}

func parseMountPaths(r io.Reader) []string {
	out := make([]string, 0)
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		parts := strings.SplitN(scanner.Text(), " ", 3)
		if len(parts) != 3 {
			continue
		}
		out = append(out, parts[1])
	}

	return out
}

func readMountPaths() ([]string, error) {
	fd, err := os.OpenFile("/proc/mounts", os.O_RDONLY, 0000)
	if err != nil {
		return nil, err
	}
	defer func() { _ = fd.Close() }()

	return parseMountPaths(fd), nil
}

// Reports whether domain is located on the filesystem mounted at mountPath.
func isBelowMountPath(domain, mountPath string) bool {
	if domain == mountPath {
		return true
	}

	return strings.HasPrefix(domain, strings.TrimSuffix(mountPath, "/")+"/")
}

// Finds the mount path with the longest match for a domain path.
func longestMatchingMountPath(domain string, mounts []string) (string, error) {
	longestMatch := 0
	path := ""
	for _, mountPath := range mounts {
		if isBelowMountPath(domain, mountPath) {
			if len(mountPath) > longestMatch {
				path = mountPath
				longestMatch = len(mountPath)
			}
		}
	}

	if path == "" {
		return "", ErrFSNotFound
	}

	return path, nil
}

func findDomainFilesystem(domain string) (string, error) {
	mounts, err := readMountPaths()
	if err != nil {
		return "", err
	}

	return longestMatchingMountPath(domain, mounts)
}

func NewFilesystemDiscoverer(recv []FilesystemReceiver) *FilesystemDiscoverer {
	return &FilesystemDiscoverer{
		mu:      sync.Mutex{},
		roots:   map[string][]string{},
		recv:    recv,
		resolve: findDomainFilesystem,
	}
}

func (rd *FilesystemDiscoverer) AddDomain(domain string) {
	rd.mu.Lock()
	defer rd.mu.Unlock()

	path, err := rd.resolve(domain)
	if err != nil {
		slog.Warn("detect domain filesystem", "domain-path", domain, "error", err)
		return
	}

	rd.roots[path] = append(rd.roots[path], domain)
	if len(rd.roots[path]) == 1 {
		for _, recv := range rd.recv {
			recv.AddFilesystem(path)
		}
	}

	for _, recv := range rd.recv {
		recv.UpdateFilesystem(path, rd.roots[path])
	}
}

func (rd *FilesystemDiscoverer) RemoveDomain(domain string) {
	rd.mu.Lock()
	defer rd.mu.Unlock()

	for path, entries := range rd.roots {
		remaining := lo.Without(entries, domain)
		if len(remaining) == len(entries) {
			// The domain was not located on this filesystem
			continue
		}

		if len(remaining) == 0 {
			for _, recv := range rd.recv {
				recv.RemoveFilesystem(path)
			}

			delete(rd.roots, path)
			continue
		}

		rd.roots[path] = remaining
		for _, recv := range rd.recv {
			recv.UpdateFilesystem(path, remaining)
		}
	}
}
