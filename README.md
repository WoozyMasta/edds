# edds

Minimal EDDS reader/writer for Arma/DayZ with LZ4 chunk-stream support.

`edds` reads and writes EDDS files (DDS header + block table + block bodies),
supports COPY/LZ4 blocks, and decodes the largest mipmap into RGBA.

## Implemented

* EDDS read (config + decode largest mip)
* EDDS write (RGBA/BGRA and BC1/BC2/BC3, optional mipmaps)
* Optional passthrough of `bcn.EncodeOptions` (quality/workers/etc.)
* LZ4 Enfusion chunk-stream compress/decompress (COPY/LZ4 blocks)
* DDS header interop via `github.com/woozymasta/bcn`

## Usage

### Read EDDS

```go
img, err := edds.Read("atlas.edds")
if err != nil {
  /* handle */
}
_ = img
```

### Read config only

```go
cfg, err := edds.ReadConfig("atlas.edds")
if err != nil {
  /* handle */
}
_ = cfg
```

### Write EDDS with mipmaps (BGRA8)

```go
err := edds.WriteWithMipmaps(img, "atlas.edds", 0) // 0 = full chain
if err != nil {
  /* handle */
}
```

### Write EDDS with format

```go
err := edds.WriteWithFormat(img, "atlas_bc5.edds", bcn.FormatBC5, 0)
if err != nil {
  /* handle */
}
```

### Write EDDS with full options

```go
err := edds.WriteWithOptions(img, "atlas_dxt5.edds", &edds.WriteOptions{
  Format:     bcn.FormatDXT5,
  MaxMipMaps: 0,
  Compress:   true,
  EncodeOptions: &bcn.EncodeOptions{
    QualityLevel: 8,
    Workers: 0,
  },
})
if err != nil {
  /* handle */
}
```

### Write EDDS from pre-encoded blocks

```go
err := edds.WriteFromBlocks("atlas_bc3.edds", bcn.FormatDXT5, width, height, mipPayloads)
if err != nil {
  /* handle */
}
```

## Notes

* Output is uncompressed RGBA payload stored as BGRA in EDDS blocks.
* Only 2D textures are handled.
* BC7 output is untested.
* BC4/BC5 do not show in-game (likely requires additional header fields).
