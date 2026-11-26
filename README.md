# mxl-utils

Go packages for reading [MXL](https://github.com/dmf-mxl/mxl) (Media eXchange Layer) domains and flows directly from the filesystem. The packages do not link against the MXL C library. They read the flow files with `mmap` and parse the binary layout in Go.

Linux only. The code uses `CLOCK_TAI`, `/proc/mounts` and inotify.

## Packages

- `pkg/mxl`: open flows and read their info and definition, find domains below a set of directories, watch domains for added and removed flows, and convert between timestamps and grain/sample indices.
- `pkg/mxlsys`: Go structs that match the binary layout of the MXL flow info.
- `pkg/testutil`: create dummy flows on disk for tests.

## Usage

```
go get github.com/jonasohland/mxl-utils
```

Read the info of a single flow:

```go
flow, err := mxl.Open("/dev/shm/mxl", "5fbec3b1-1b0f-417d-9059-8b94a47197ed")
if err != nil {
	return err
}
defer flow.Close()

config, runtime, err := flow.GetInfo()
if err != nil {
	return err
}
fmt.Println(config.Format, config.Rate, runtime.HeadIndex)
```

To follow flows as they appear and disappear, create a `Watcher` with one or more `FlowReceiver` implementations and pass it to a `Discoverer` as a `DomainReceiver`. The `Discoverer` scans the given directories every few seconds and reports the domains it finds. The `Watcher` then reports the flows in each domain.

## Development

```
make build
make vet
make test
```

## License

Apache License 2.0. See [LICENSE.txt](LICENSE.txt).
