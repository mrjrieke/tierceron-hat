## License
[LICENSE](LICENSE)

# Tierceron-hat

[![GitHub release](https://img.shields.io/github/release/trimble-oss/tierceron-hat.svg?style=flat-square)](https://github.com/trimble-oss/tierceron-hat/releases/latest)
[![Go Report Card](https://goreportcard.com/badge/github.com/trimble-oss/tierceron-hat)](https://goreportcard.com/report/github.com/trimble-oss/tierceron-hat)
[![PkgGoDev](https://img.shields.io/badge/go.dev-docs-007d9c?logo=go&logoColor=white&style=flat-square)](https://pkg.go.dev/github.com/trimble-oss/tierceron-hat)

## What is it?
Tierceron-hat collects the small executables and support packages used by the Tierceron "wanderer" tooling. The repository contains multiple CLI entrypoints plus the shared protobuf and tap code they build on.

## What is in this repo?
- `tiara`, `brim`, `brimfeather`, `captap`, `captip`, `captiptwo`, and `crown`: standalone commands built into `bin/`.
- `cap`: shared protobuf and gRPC definitions plus shared command support code.
- `cap/tap`: OS-specific tap implementations for Linux, macOS, and Windows.
- `captip/captiplib` and `captip/captiplibjs`: reusable library code for native and JS or wasm builds.
- `capfull`: a wasm build target generated as `bin/capfull.wasm`.

## Key Features
- Multiple runnable command targets managed from a single Makefile.
- Shared protobuf and gRPC definitions for the `cap` transport layer.
- Cross-platform tap support with platform-specific implementations.
- Native and wasm build outputs for different deployment targets.

## Getting started
Build one or more commands from the repository root:

- `make tiara`
- `make brim`
- `make brimfeather`
- `make captap`
- `make captip`
- `make captiptwo`
- `make crown`
- `make capfull`
- `make all`

If you need to regenerate gRPC stubs for `cap`, run `make capgrpc`.

Built binaries and wasm artifacts are written under `./bin/`.

## Trusted Committers
- [Joel Rieke](mailto:joel_rieke@trimble.com)
- [David Mkrtychyan](mailto:david_mkrtychyan@trimble.com)
- [Karnveer Gill](mailto:karnveer_gill@trimble.com)
- [Meghan Bailey](mailto:meghan_bailey@trimble.com)

## Code of Conduct
Please read [CODE_OF_CONDUCT.MD](CODE_OF_CONDUCT.MD) before contributing or opening issues.

## Security
Please review [SECURITY.md](SECURITY.md) for vulnerability reporting guidance.
