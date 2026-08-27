# mxl-utils

Go utilities for reading [MXL (Media eXchange Layer)](https://github.com/dmf-mxl/mxl) domains and flows from a Linux host.

These packages are read-only observers: they discover MXL domains on the filesystem, watch them for flows appearing and disappearing, and decode the flow metadata that the MXL runtime writes to shared memory. They do not participate in MXL data transport and do not link against the MXL C library.

```
go get github.com/jonasohland/mxl-utils
```

## Packages

| Package | Contents |
|---|---|
| `pkg/mxl` | Public API: `Flow` (mmap-backed access to a flow's config, runtime info and NMOS flow definition), `Discoverer` (finds domains under search paths), `Watcher` (inotify-based flow add/remove events), `FilesystemDiscoverer` (maps domains onto their backing mount points), and TAI clock/grain-index helpers. |
| `pkg/mxlsys` | Go mirrors of the binary structures in `mxl/flowinfo.h`, laid out to match the on-disk `data` file of a flow. |
| `pkg/testutil` | Builders for synthetic flow directories (`DummyFlow`, video/audio flow definitions), for use in tests of this module and its consumers. |

## Layout on disk

A domain is any directory containing `<flow-id>.mxl-flow` subdirectories:

```
<domain>/
└── ae8b77bd-a796-4aed-90ad-57a7dc388f85.mxl-flow/
    ├── data           # binary mxlFlowInfo (2048 bytes)
    └── flow_def.json  # NMOS flow definition
```

## Usage

```go
flow, err := mxl.Open("/path/to/domain", "ae8b77bd-a796-4aed-90ad-57a7dc388f85")
if err != nil {
    return err
}
defer flow.Close()

config, runtime, err := flow.GetInfo()
if err != nil {
    return err
}

// how far the write head lags behind the current TAI grain index
latency := int64(mxl.CurrentIndex(config.Rate)) - int64(runtime.HeadIndex)
```

Discovery and watching are push-based — implement `DomainReceiver`, `FlowReceiver` or `FilesytemReceiver` and hand them to the respective constructor:

```go
watcher, err := mxl.NewWatcher(ctx, wg, []mxl.FlowReceiver{myReceiver})
if err != nil {
    return err
}

mxl.NewDiscoverer(ctx, wg, []mxl.DomainReceiver{watcher}, searchPaths, staticDomains)
```

## Platform support

Linux only — the packages rely on inotify, `/proc/mounts`, `CLOCK_TAI` and `syscall.Stat_t` inode numbers.

## Build

```bash
make build
make test
```

## License

See [LICENSE.txt](LICENSE.txt).
