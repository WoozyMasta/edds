<!-- markdownlint-disable MD024 -->
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

## Unreleased

### Added

* Read support for Workbench legacy A8, R8, and RG8 textures, plus DX10 BC7,
  signed BC4/BC5, BGRX8, RGB10A2, R8S, RG8S, RGB565, RGBA5551, and RGBA4444.
* Workbench-generated 256x256 read corpus covering
  BGRA8, A8, R8, RG8, DXT5, BC4, BC5, and BC7.
* Configurable EDDS read limits for block tables, block bodies,
  decoded payloads, output images, and buffered input.

### Changed

* Default EDDS reads now enforce limits that accept 16K RGBA textures.
* Path-based write APIs now replace output files only after a successful encode.
* `Decode` now reads current EDDS block streams sequentially
  from non-seekable inputs.
* Current EDDS block-table errors no longer fall back to legacy parsing.
* Reusable LZ4 decoding now passes chunk views directly to the decompressor,
  avoiding per-chunk allocations and copies.

### Fixed

* `MaxMipMaps: 0` now writes the complete mip chain
  instead of capping it at 11 levels.
* Reject DX10 arrays, cubemaps, and volume textures,
  which cannot be represented by the single-image read API.

## [0.3.0][] - 2026-06-18

### Added

* Stream-oriented `Encode`/`Decode` APIs
  for working with `io.Writer` and `io.Reader`.
* Reusable `Encoder` and `Decoder` types
  for batch pipelines with lower per-image allocations.
* Stream-oriented `EncodeFromBlocks` APIs
  for writing pre-encoded mip payloads to `io.Writer`.

### Changed

* EDDS encode/decode paths now reuse BCn and container buffers where possible.

[0.3.0]: https://github.com/WoozyMasta/edds/compare/v0.2.0...v0.3.0

## [0.2.0][] - 2026-06-18

### Added

* Configurable EDDS block compression via `CompressionOptions`.
* `WriteFromBlocksWithCompressionOptions`
  for writing pre-encoded payloads with explicit compression settings.

### Changed

* `WriteOptions.Compress` is now deprecated in favor of
  `WriteOptions.Compression.Mode`.
* Default write compression now uses fast LZ4 instead of LZ4 HC,
  keeping LZ4 size savings while avoiding previous high-compression CPU cost;
  `BenchmarkMainFlowWriteDXT5`: about 77% faster,
  `BenchmarkContainerWriteFromBlocksDXT5/LZ4`: about 65% faster.
* Updated dependency `github.com/woozymasta/bcn` to `v0.4.0`
  and switched mipmap generation to `GenerateMipmapsN`.

[0.2.0]: https://github.com/WoozyMasta/edds/compare/v0.1.3...v0.2.0

## [0.1.3][] - 2026-02-17

### Added

* Baseline benchmarks for main IO flows:
  `BenchmarkMainFlowWriteDXT5`, `BenchmarkMainFlowReadDXT5`,
  `BenchmarkMainFlowWriteBGRA8`, `BenchmarkMainFlowReadBGRA8`.
* Container-only write benchmark `BenchmarkContainerWriteFromBlocksDXT5`
  with `COPY`/`LZ4` sub-benchmarks to separate EDDS container cost
  from BCn encoding cost.

### Changed

* Updated dependency `github.com/woozymasta/bcn` to `v0.1.5`.

[0.1.3]: https://github.com/WoozyMasta/edds/compare/v0.1.2...v0.1.3

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
