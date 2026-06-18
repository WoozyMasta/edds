// SPDX-License-Identifier: MIT
// Copyright (c) 2026 WoozyMasta
// Source: github.com/woozymasta/edds

/*
Package edds implements Arma/DayZ EDDS (Enfusion DDS) container read/write
with optional LZ4 chunk-stream compression.

EDDS stores a DDS header followed by a block table and block bodies per mipmap
level (smallest to largest). Blocks may be uncompressed (COPY) or LZ4
compressed using Enfusion chunk-stream format with a rolling 64KB dictionary.

The package focuses on practical workflows: read config, decode the largest
level into RGBA, and write uncompressed RGBA payloads with optional mipmaps.

Package-level Read and Write helpers operate on file paths.
Decode and Encode operate on io.Reader and io.Writer streams.
EncodeFromBlocks writes pre-encoded mipmap payloads
to an io.Writer without re-encoding image pixels.

Encoder and Decoder reuse internal buffers for batch pipelines.
They are not safe for concurrent use; create one per worker goroutine.
Images returned by Decoder share its reusable pixel buffer
and are valid only until the next Decode call on the same Decoder.
*/
package edds
