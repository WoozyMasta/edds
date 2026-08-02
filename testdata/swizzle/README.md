# Workbench swizzle corpus

These files were created in Workbench
from `../corpus/mip-grid-256.png` with mipmaps enabled.
The filename suffix is the displayed Workbench profile name.

`SwizzleProfile` implements every deterministic channel mapping in this corpus.
`ColorNoise` is retained as a read fixture only:
Workbench generates a noisy alpha channel,
so it is not a channel swizzle and has no `SwizzleProfile`.
