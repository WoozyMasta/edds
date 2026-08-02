# Workbench corpus

Generate the source image with:

```sh
go run ./testdata/corpus/generate.go -out ./testdata/corpus
```

These files were created by Workbench with mipmaps enabled:

| Output | Workbench format | Expected mip count |
| --- | --- | ---: |
| `mip-grid-256.edds` | default BGRA8 | 9 |
| `mip-grid-256-Alpha.edds` | A8 | 9 |
| `mip-grid-256-DXTCompression.edds` | DXT5 | 9 |
| `mip-grid-256-ColorHQCompression.edds` | BC7 / DX10 | 9 |
| `mip-grid-256-Red.edds` | R8 | 9 |
| `mip-grid-256-RedHQCompression.edds` | BC4 / DX10 | 9 |
| `mip-grid-256-RedGreen.edds` | RG8 | 9 |
| `mip-grid-256-RedGreenHQCompression.edds` | BC5 / DX10 | 9 |

The source contains gradients, grid lines, and non-opaque alpha. The Go unit test
covers chains longer than 11 levels without storing a large binary fixture.
Cube maps, HDR, and swizzle variants are intentionally not included: Workbench
did not create those files through the normal texture conversion workflow.
