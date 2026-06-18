# edds

Minimal EDDS reader/writer for Arma/DayZ with LZ4 chunk-stream support.

`edds` reads and writes EDDS files (DDS header + block table + block bodies),
supports COPY/LZ4 blocks, and decodes the largest mipmap into RGBA.

## Implemented

* EDDS read (config + decode largest mip)
* EDDS write (RGBA/BGRA and BC1/BC2/BC3, optional mipmaps)
* Stream-oriented encode/decode APIs for `io.Reader` / `io.Writer`
* Reusable `Encoder` / `Decoder` for batch pipelines
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
  Compression: edds.CompressionOptions{
    Mode: edds.CompressionLZ4,
  },
  EncodeOptions: &bcn.EncodeOptions{
    QualityLevel: 8,
    Workers: 0,
  },
})
if err != nil {
  /* handle */
}
```

Use `CompressionNone` for COPY blocks
or `CompressionLZ4HC` with `HCLevel` for slower size-priority compression.

### Write EDDS from pre-encoded blocks

```go
err := edds.WriteFromBlocks("atlas_bc3.edds", bcn.FormatDXT5, width, height, mipPayloads)
if err != nil {
  /* handle */
}
```

### Stream encode/decode

```go
var buf bytes.Buffer

if err := edds.Encode(&buf, img); err != nil {
  /* handle */
}

decoded, err := edds.Decode(bytes.NewReader(buf.Bytes()))
if err != nil {
  /* handle */
}
_ = decoded
```

Pre-encoded mip payloads can be written to any `io.Writer`:

```go
err := edds.EncodeFromBlocks(&buf, bcn.FormatDXT5, width, height, mipPayloads)
if err != nil {
  /* handle */
}
```

### Batch encode/decode

Use one `Encoder` or `Decoder` per worker goroutine to reuse internal buffers:

```go
enc := edds.NewEncoder()
dec := edds.NewDecoder()

_ = enc.Encode(w, img)
decoded, err := dec.Decode(r)
_ = decoded
_ = err
```

`Encoder` and `Decoder` are not safe for concurrent use.
Images returned by `Decoder` share its reusable pixel buffer
and remain valid only until the next decode call on the same decoder.
Copy the image if it must be retained.

## Notes

* Package-level encode/decode supports
  `BGRA8`, `RGBA8`, `DXT1/3/5`, `BC4`, `BC5`.
* `DXT3`, `BC4`, `BC5` may decode in tooling but may not display correctly
  in-game/Workbench.
* Only 2D textures are handled.
