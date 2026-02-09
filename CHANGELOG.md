# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog][],
and this project adheres to [Semantic Versioning][].

<!--
## Unreleased

### Added
### Changed
### Removed
-->

## [0.1.2][] - 2026-02-10

### Added

* Decode with options `ReadWithOptions(path, opts)` and `ReadOptions` struct.

[0.1.2]: https://github.com/WoozyMasta/edds/compare/v0.1.1...v0.1.2

## [0.1.1][] - 2026-02-07

### Added

* New concise full-control writer API `WriteWithOptions`
  with `WriteOptions` struct.
* Table-driven regression tests.

### Changed

* Updated dependency `github.com/woozymasta/bcn` to `v0.1.3`.
* Writer path now supports modern BCn encoder options via
  `WriteOptions.EncodeOptions` (including quality levels and worker settings).
* EDDS write/decode behavior now benefits from BCn-side optimizations
  and parallel workers (`Workers=0` uses `GOMAXPROCS`).

[0.1.1]: https://github.com/WoozyMasta/edds/compare/v0.1.0...v0.1.1

## [0.1.0][] - 2026-02-04

### Added

* First public release

[0.1.0]: https://github.com/WoozyMasta/edds/tree/v0.1.0

<!--links-->
[Keep a Changelog]: https://keepachangelog.com/en/1.1.0/
[Semantic Versioning]: https://semver.org/spec/v2.0.0.html
