/*
Package edds implements Arma/DayZ EDDS (Enfusion DDS) container read/write
with optional LZ4 chunk-stream compression.

EDDS stores a DDS header followed by a block table and block bodies per mipmap
level (smallest to largest). Blocks may be uncompressed (COPY) or LZ4
compressed using Enfusion chunk-stream format with a rolling 64KB dictionary.

The package focuses on practical workflows: read config, decode the largest
level into RGBA, and write uncompressed RGBA payloads with optional mipmaps.
*/
package edds
