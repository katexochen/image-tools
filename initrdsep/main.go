package main

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/klauspost/compress/zstd"
)

func main() {
	if err := run(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	path := os.Args[1]

	f, err := os.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		return err
	}
	defer f.Close()

	var initrdIndex int
	for {
		if err := decompressInitrd(initrdIndex, f); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			fr, err2 := os.OpenFile("remaining", os.O_CREATE|os.O_WRONLY, 0o644)
			if err2 != nil {
				fmt.Printf("creating file for remaining data: %v", err2)
				return err
			}
			defer fr.Close()
			if _, err := io.Copy(fr, f); err != nil {
				fmt.Printf("copying remaining data: %v", err)
			}
			return err
		}
		if _, err := skipPadding(f); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return fmt.Errorf("skipping padding: %w", err)
		}
		initrdIndex++
	}

	return nil
}

func decompressInitrd(index int, f *os.File) error {
	magic, err := peek(f, 2)
	if err != nil {
		return err
	}
	decompressor, err := compressFormat(magic)
	if err != nil {
		return err
	}
	out, err := os.OpenFile(fmt.Sprintf("initrd_%d", index), os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()
	return decompressor(f, out)
}

// compressFormat recognizes the compression format based on the magic bytes.
// Based on https://elixir.bootlin.com/linux/v6.11.2/source/lib/decompress.c#L51-L61
func compressFormat(magic []byte) (decompressor, error) {
	if len(magic) < 2 {
		return nil, fmt.Errorf("minimum magic length is 2")
	}
	switch hex.EncodeToString(magic) {
	// case "1f8b":
	// 	return "gzip", nil
	// case "1f9e":
	// 	return "gzip", nil
	// case "425a":
	// 	return "bzip2", nil
	// case "5d00":
	// 	return "lzma", nil
	// case "fd37":
	// 	return "xz", nil
	// case "894c":
	// 	return "lzo", nil
	// case "0221":
	// 	return "lz4", nil
	case "28b5":
		// https://github.com/facebook/zstd/blob/dev/doc/zstd_compression_format.md
		fmt.Println("detected zstd compressed initrd")
		return func(in io.ReadSeeker, out io.Writer) error {
			frameSize, err := zstdFrameSize(in)
			if err != nil {
				return fmt.Errorf("getting zstd frame size: %w", err)
			}
			limitIn := io.LimitReader(in, int64(frameSize))
			d, err := zstd.NewReader(limitIn)
			if err != nil {
				return err
			}
			defer d.Close()
			if _, err := io.Copy(out, d); err != nil {
				return fmt.Errorf("decompressing zstd: %w", err)
			}
			fmt.Println("successfully decompressed zstd")
			return nil
		}, nil
	case "3037":
		// https://github.com/libyal/dtformats/blob/main/documentation/Copy%20in%20and%20out%20(CPIO)%20archive%20format.asciidoc
		fmt.Println("detected uncompressed initrd in ascii cpio format")
		return func(in io.ReadSeeker, out io.Writer) error {
			size, err := cpioSize(in)
			if err != nil {
				return fmt.Errorf("getting cpio ascii size: %w", err)
			}
			limitIn := io.LimitReader(in, int64(size))
			if _, err := io.Copy(out, limitIn); err != nil {
				return fmt.Errorf("copying cpio ascii: %w", err)
			}
			fmt.Println("successfully copied uncompressed cpio archive")
			return nil
		}, nil
	default:
		return nil, fmt.Errorf("unknown magic bytes %s", hex.EncodeToString(magic))
	}
}

type decompressor = func(io.ReadSeeker, io.Writer) error

func cpioSize(in io.ReadSeeker) (int, error) {
	const cpioHeaderSize = 110
	const cpioTrailer = "TRAILER!!!\x00"

	pos, err := in.Seek(0, io.SeekCurrent)
	if err != nil {
		return 0, fmt.Errorf("getting current position: %w", err)
	}
	defer in.Seek(pos, io.SeekStart)

	var size int
	for {
		magic, err := peek(in, 6)
		if err != nil {
			return 0, fmt.Errorf("reading cpio magic: %w", err)
		}

		switch {
		case string(magic) == "070701" || string(magic) == "070702":
			fileSize, err := peekHexIntAt(in, 54, 8)
			if err != nil {
				return 0, fmt.Errorf("getting cpio entry file size: %w", err)
			}
			pathSize, err := peekHexIntAt(in, 94, 8)
			if err != nil {
				return 0, fmt.Errorf("getting cpio entry path size: %w", err)
			}

			entrySize := cpioHeaderSize
			entrySize += pathSize
			entrySize += ((4 - (entrySize % 4)) % 4) // Path padding
			entrySize += fileSize
			entrySize += ((4 - (entrySize % 4)) % 4) // File padding
			size += entrySize

			if pathSize == len(cpioTrailer) {
				path, err := peekAt(in, cpioHeaderSize, pathSize)
				if err != nil {
					return 0, fmt.Errorf("reading cpio path: %w", err)
				}
				if string(path) == cpioTrailer {
					if _, err := in.Seek(int64(entrySize), io.SeekCurrent); err != nil {
						return 0, fmt.Errorf("seeking next cpio entry: %w", err)
					}
					return size, nil
				}
			}
			if _, err := in.Seek(int64(entrySize), io.SeekCurrent); err != nil {
				return 0, fmt.Errorf("seeking next cpio entry: %w", err)
			}

		case string(magic) == "070707":
			panic("portable ascii cpio not implemented")
		case bytes.Equal(magic[:2], []byte{0x71, 0xC7}):
			panic("binary cpio format not implemented")
		default:
			return 0, fmt.Errorf("unknown cpio magic %s", hex.EncodeToString(magic))
		}
	}
}

func zstdFrameSize(in io.ReadSeeker) (int, error) {
	const zsdtBlockHeaderSize = 3

	pos, err := in.Seek(0, io.SeekCurrent)
	if err != nil {
		return 0, fmt.Errorf("getting current position: %w", err)
	}
	defer in.Seek(pos, io.SeekStart)

	var header zstd.Header
	buf, err := peek(in, 18)
	if err != nil {
		return 0, fmt.Errorf("reading zstd header: %w", err)
	}
	if err := header.Decode(buf); err != nil {
		return 0, fmt.Errorf("decoding zstd header: %w", err)
	}
	size := header.HeaderSize
	in.Seek(int64(header.HeaderSize), io.SeekCurrent)
	if header.HasCheckSum {
		size += 4
	}

	for {
		bhBytes, err := peek(in, zsdtBlockHeaderSize)
		if err != nil {
			return 0, fmt.Errorf("reading zstd block header: %w", err)
		}
		// https://github.com/klauspost/compress/blob/2a46d6bf5d0fb5d9f44b815438ce43470706f73f/zstd/blockdec.go#L139
		blockHeader := uint32(bhBytes[0]) | (uint32(bhBytes[1]) << 8) | (uint32(bhBytes[2]) << 16)
		blockType := zstdBlockType((blockHeader >> 1) & 3)
		blockIsLast := blockHeader&1 != 0
		blockSize := zsdtBlockHeaderSize
		switch blockType {
		case zsdtBlockTypeRLE:
			blockSize += 1
		case zsdtBlockTypeCompressed:
			fallthrough
		case zsdtBlockTypeRaw:
			blockSize += int(blockHeader >> 3)
		default:
			panic("Invalid block type")
		}
		size += blockSize
		in.Seek(int64(blockSize), io.SeekCurrent)
		if blockIsLast {
			break
		}
	}

	return size, nil
}

type zstdBlockType uint8

const (
	zsdtBlockTypeRaw zstdBlockType = iota
	zsdtBlockTypeRLE
	zsdtBlockTypeCompressed
	zsdtBlockTypeReserved
)

func skipPadding(in io.ReadSeeker) (int, error) {
	var skipped int
	for {
		b, err := peek(in, 1)
		if err != nil {
			return 0, fmt.Errorf("reading single padding byte: %w", err)
		}
		if b[0] != 0 {
			return skipped, nil
		}
		if _, err = in.Seek(1, io.SeekCurrent); err != nil {
			return 0, fmt.Errorf("skipping padding: %w", err)
		}
		skipped++
	}
}

func peek(f io.ReadSeeker, n int) ([]byte, error) {
	b := make([]byte, n)
	if _, err := f.Read(b); err != nil {
		return nil, err
	}
	if _, err := f.Seek(-int64(n), io.SeekCurrent); err != nil {
		return nil, err
	}
	return b, nil
}

func peekAt(f io.ReadSeeker, offset, n int) ([]byte, error) {
	pos, err := f.Seek(0, io.SeekCurrent)
	if err != nil {
		return nil, fmt.Errorf("getting current position: %w", err)
	}
	defer f.Seek(pos, io.SeekStart)
	if _, err := f.Seek(int64(offset), io.SeekCurrent); err != nil {
		return nil, fmt.Errorf("seeking to offset %d: %w", offset, err)
	}
	return peek(f, n)
}

func peekHexIntAt(f io.ReadSeeker, offset, size int) (int, error) {
	pos, err := f.Seek(0, io.SeekCurrent)
	if err != nil {
		return 0, fmt.Errorf("getting current position: %w", err)
	}
	defer f.Seek(pos, io.SeekStart)
	if _, err := f.Seek(int64(offset), io.SeekCurrent); err != nil {
		return 0, fmt.Errorf("seeking to offset %d: %w", offset, err)
	}
	buf := make([]byte, size)
	if _, err := f.Read(buf); err != nil {
		return 0, fmt.Errorf("reading %d bytes at offset %d: %w", size, offset, err)
	}
	i, err := strconv.ParseInt(string(buf), 16, 64)
	if err != nil {
		return 0, fmt.Errorf("parsing hex int %s: %w", string(buf), err)
	}
	return int(i), nil
}
