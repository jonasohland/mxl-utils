package mxl

import (
	"bufio"
	"errors"
	"log/slog"
	"os"
	"strings"
	"sync"

	"github.com/samber/lo"
)

var (
	ErrFSNotFound = errors.New("no root filesystem found for domain")
)

type FilesytemReceiver interface {
	AddFilesystem(path string)
	UpdateFilesystem(path string, domains []string)
	RemoveFilesystem(path string)
}

type FilesystemDiscoverer struct {
	mu    sync.Mutex
	roots map[string][]string
	recv  []FilesytemReceiver
}

func readMountPaths() ([]string, error) {
	fd, err := os.OpenFile("/proc/mounts", os.O_RDONLY, 0000)
	if err != nil {
		return nil, err
	}
	defer func() { _ = fd.Close() }()

	out := make([]string, 0)
	scanner := bufio.NewScanner(fd)
	for scanner.Scan() {
		parts := strings.SplitN(scanner.Text(), " ", 3)
		if len(parts) != 3 {
			continue
		}
		out = append(out, parts[1])
	}

	return out, nil
}

func findDomainFilesystem(domain string) (string, error) {
	mounts, err := readMountPaths()
	if err != nil {
		return "", err
	}

	// find mount path with longest prefix match
	longestMatch := 0
	path := ""
	for _, mountPath := range mounts {
		if strings.HasPrefix(domain, mountPath) {
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

func NewFilesystemDiscoverer(recv []FilesytemReceiver) *FilesystemDiscoverer {
	return &FilesystemDiscoverer{mu: sync.Mutex{}, roots: map[string][]string{}, recv: recv}
}

func (rd *FilesystemDiscoverer) AddDomain(domain string) {
	rd.mu.Lock()
	defer rd.mu.Unlock()

	path, err := findDomainFilesystem(domain)
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
		rd.roots[path] = lo.Without(entries, domain)
		if len(rd.roots[path]) == 0 {
			for _, recv := range rd.recv {
				recv.RemoveFilesystem(path)
			}

			delete(rd.roots, path)
		} else {
			for _, recv := range rd.recv {
				recv.UpdateFilesystem(path, rd.roots[path])
			}
		}
	}
}
