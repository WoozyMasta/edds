package edds

import "errors"

var (
	// ErrSizeOverflow indicates a size or dimension exceeds supported limits.
	ErrSizeOverflow = errors.New("size overflow")
	// ErrInvalidFormat indicates unsupported format.
	ErrInvalidFormat = errors.New("invalid format")
	// ErrEmptyMipmaps indicates missing mipmap data.
	ErrEmptyMipmaps = errors.New("empty mipmaps")
	// ErrMipmapSizeMismatch indicates mipmap payload size mismatch.
	ErrMipmapSizeMismatch = errors.New("mipmap size mismatch")
	// ErrInputTooLarge indicates input data is too large to encode.
	ErrInputTooLarge = errors.New("input data too large")
	// ErrCompressedDataTooLarge indicates compressed payload exceeds limits.
	ErrCompressedDataTooLarge = errors.New("compressed data too large")
	// ErrChunkTooLarge indicates a compressed chunk exceeds allowed size.
	ErrChunkTooLarge = errors.New("compressed chunk too large")
	// ErrLZ4Compress indicates LZ4 compression failed.
	ErrLZ4Compress = errors.New("LZ4 compression failed")
	// ErrLZ4Decode indicates LZ4 decode failed.
	ErrLZ4Decode = errors.New("LZ4 decode failed")
	// ErrCopySizeMismatch indicates COPY block data size mismatch.
	ErrCopySizeMismatch = errors.New("COPY block size mismatch")
	// ErrUnknownBlockMagic indicates an unknown block magic.
	ErrUnknownBlockMagic = errors.New("unknown block magic")
	// ErrInvalidTargetSize indicates invalid decoded target size.
	ErrInvalidTargetSize = errors.New("invalid target size")
	// ErrChunkStreamTruncated indicates LZ4 chunk stream is truncated.
	ErrChunkStreamTruncated = errors.New("LZ4 chunk-stream truncated")
	// ErrUnknownLZ4Flags indicates unknown LZ4 chunk flags.
	ErrUnknownLZ4Flags = errors.New("unknown LZ4 flags")
	// ErrInvalidChunkSize indicates invalid LZ4 chunk size.
	ErrInvalidChunkSize = errors.New("invalid compressed chunk size")
	// ErrDecodeOverrun indicates decoded data overruns target buffer.
	ErrDecodeOverrun = errors.New("decoded LZ4 overruns target buffer")
	// ErrDecodedSizeMismatch indicates decoded size mismatch.
	ErrDecodedSizeMismatch = errors.New("LZ4 decoded size mismatch")
	// ErrBlockLengthMismatch indicates leftover bytes after decode.
	ErrBlockLengthMismatch = errors.New("LZ4 block length mismatch")
	// ErrBlockTableMagicRead indicates block table magic read failed.
	ErrBlockTableMagicRead = errors.New("reading block table magic failed")
	// ErrBlockTableSizeRead indicates block table size read failed.
	ErrBlockTableSizeRead = errors.New("reading block table size failed")
	// ErrBlockTableUnknownMagic indicates unknown block magic in table.
	ErrBlockTableUnknownMagic = errors.New("unknown block magic in table")
	// ErrBlockTableInvalidSize indicates invalid size in block table.
	ErrBlockTableInvalidSize = errors.New("invalid block size in table")
	// ErrBlockBodyInvalidSize indicates invalid block body size.
	ErrBlockBodyInvalidSize = errors.New("invalid block size")
	// ErrBlockBodyRead indicates block body read failed.
	ErrBlockBodyRead = errors.New("reading block body failed")
	// ErrDDSHeaderRead indicates DDS header read failed.
	ErrDDSHeaderRead = errors.New("reading DDS header failed")
	// ErrDDSDX10Read indicates DDS DX10 header read failed.
	ErrDDSDX10Read = errors.New("reading DDS DX10 header failed")
	// ErrOpenFile indicates EDDS file open failed.
	ErrOpenFile = errors.New("open file failed")
	// ErrDecodeImage indicates image decode failed.
	ErrDecodeImage = errors.New("decode image failed")
	// ErrReadBlockTable indicates block table read failed.
	ErrReadBlockTable = errors.New("read block table failed")
	// ErrSkipBlockBody indicates skipping block body failed.
	ErrSkipBlockBody = errors.New("skip block body failed")
	// ErrReadBlockBody indicates block body read failed.
	ErrReadBlockBody = errors.New("read block body failed")
	// ErrUnknownFormat indicates unsupported DDS/EDDS format.
	ErrUnknownFormat = errors.New("unknown format")
	// ErrDecompressBlock indicates block decompression failed.
	ErrDecompressBlock = errors.New("decompress block failed")
	// ErrLargestMipSizeMismatch indicates mismatch in largest mip size.
	ErrLargestMipSizeMismatch = errors.New("largest mip size mismatch")
	// ErrPickLargestMip indicates failure selecting largest mip.
	ErrPickLargestMip = errors.New("failed to pick largest mip")
	// ErrSeekDataStart indicates seek to data start failed.
	ErrSeekDataStart = errors.New("seek to data start failed")
	// ErrReadRemainingData indicates reading remaining data failed.
	ErrReadRemainingData = errors.New("reading remaining data failed")
	// ErrParseSingleBlock indicates failure parsing legacy single block.
	ErrParseSingleBlock = errors.New("failed to parse single block")
	// ErrCompressMipmap indicates mipmap compression failed.
	ErrCompressMipmap = errors.New("compress mipmap failed")
	// ErrCreateFile indicates file creation failed.
	ErrCreateFile = errors.New("create file failed")
	// ErrWriteDDSMagic indicates DDS magic write failed.
	ErrWriteDDSMagic = errors.New("writing DDS magic failed")
	// ErrWriteDDSHeader indicates DDS header write failed.
	ErrWriteDDSHeader = errors.New("writing DDS header failed")
	// ErrWriteBlockMagic indicates block magic write failed.
	ErrWriteBlockMagic = errors.New("writing block magic failed")
	// ErrWriteBlockSize indicates block size write failed.
	ErrWriteBlockSize = errors.New("writing block size failed")
	// ErrWriteBlockData indicates block data write failed.
	ErrWriteBlockData = errors.New("writing block data failed")
	// ErrWriteUncompressedSize indicates uncompressed size write failed.
	ErrWriteUncompressedSize = errors.New("writing uncompressed size failed")
	// ErrWriteChunkStream indicates chunk stream write failed.
	ErrWriteChunkStream = errors.New("writing chunk stream failed")
	// ErrWriteBlockPayload indicates block payload write failed.
	ErrWriteBlockPayload = errors.New("writing block payload failed")
	// ErrChunkHeaderRead indicates LZ4 chunk header read failed.
	ErrChunkHeaderRead = errors.New("reading chunk header failed")
	// ErrChunkDataRead indicates LZ4 chunk data read failed.
	ErrChunkDataRead = errors.New("reading chunk data failed")
)
