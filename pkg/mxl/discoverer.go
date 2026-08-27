package mxl

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/samber/lo"
)

type domainPathCache map[string]struct{}

type Discoverer struct {
	paths  []string
	static []string
	cached domainPathCache
	recv   []DomainReceiver
}

type DomainReceiver interface {
	AddDomain(path string)
	RemoveDomain(path string)
}

func NewDiscoverer(ctx context.Context, wg *sync.WaitGroup, recv []DomainReceiver, paths, static []string) *Discoverer {
	d := &Discoverer{
		paths:  paths,
		static: static,
		cached: map[string]struct{}{},
		recv:   recv,
	}

	for _, path := range static {
		for _, recv := range d.recv {
			recv.AddDomain(path)
		}
	}

	wg.Go(func() { d.run(ctx) })
	return d
}

func (d *Discoverer) added(path string) {
	if lo.Contains(d.static, path) {
		return
	}
	for _, recv := range d.recv {
		recv.AddDomain(path)
	}
}

func (d *Discoverer) removed(path string) {
	if lo.Contains(d.static, path) {
		return
	}
	for _, recv := range d.recv {
		recv.RemoveDomain(path)
	}
}

func (d *Discoverer) reload() {
	discovered := domainPathCache{}
	for _, path := range d.paths {
		d.reloadAt(path, discovered)
	}

	for path := range d.cached {
		_, ok := discovered[path]
		if !ok {
			d.removed(path)
		}
	}
	for path := range discovered {
		_, ok := d.cached[path]
		if !ok {
			d.added(path)
		}
	}

	d.cached = discovered
}

func (d *Discoverer) reloadAt(path string, discovered domainPathCache) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return
	}

	isDomain := false
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		if IsFlowDir(entry.Name()) {
			isDomain = true
		} else {
			d.reloadAt(filepath.Join(path, entry.Name()), discovered)
		}
	}

	if isDomain {
		discovered[path] = struct{}{}
	}
}

func (d *Discoverer) run(ctx context.Context) {
	defer func() {
		for _, path := range d.static {
			for _, recv := range d.recv {
				recv.RemoveDomain(path)
			}
		}

		for path := range d.cached {
			d.removed(path)
		}
	}()
	for {
		d.reload()
		select {
		case <-ctx.Done():
			return
		case <-time.After(7 * time.Second):
		}
	}
}
