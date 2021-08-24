// Copyright © 2021 Optable Technologies Inc. All rights reserved.
// See LICENSE for details.
package io

import (
	"bytes"
	"errors"
	"io"
)

type (
	// ChunkReader breaks a stream into chunks amenable to parallel parsing.
	ChunkReader interface {
		// NextChunk returns a FrameReader.
		NextChunk() (FrameReader, error)
	}
)

var InvalidArgErr = errors.New("Invalid argument")

// NewNewlineDelimitedChunkReader returns a ChunkReader that breaks chunks of
// frames delimited by newlines. It is expected that the caller provides a
// chunkSize large enough to include a full frame. This is a limitation similar
// to bufio.Scanner. We recommend that the chunkSize should contain a handful
// of frames. Otherwise use a FrameReader directly.
//
// The chunker will not look for `\r` rune like bufio.Scanner (and
// NewlineDelimitedFrameReader) does.
func NewNewlineDelimitedChunkReader(reader io.Reader, chunkSize int) (ChunkReader, error) {
	if chunkSize < 0 {
		return nil, InvalidArgErr
	}

	if reader == nil {
		return nil, InvalidArgErr
	}

	return &delimitedChunker{
		r:         reader,
		delimiter: '\n',
		chunkSize: chunkSize,
	}, nil
}

type delimitedChunker struct {
	r         io.Reader
	delimiter byte
	chunkSize int

	prev []byte
}

var NoFrameFoundErr = errors.New("No frame found in chunk")

func (c *delimitedChunker) NextChunk() (FrameReader, error) {
	if c.r == nil {
		return nil, io.EOF
	}

	buf := make([]byte, c.chunkSize)
	n, err := io.ReadFull(c.r, buf)
	if errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF) {
		// ReadFull returns ErrUnexpectedEOF if it couldn't read the full
		// buffer. We use this signal as equivalent to EOF and only the last
		// chunk can have less than chunkSize.
		c.r = nil
		buf = buf[:n]
	} else if err != nil {
		return nil, err
	}

	var buffers []io.Reader
	if len(c.prev) > 0 {
		buffers, c.prev = append(buffers, bytes.NewReader(c.prev)), nil
	}

	pos := bytes.LastIndexByte(buf, c.delimiter)
	if pos == -1 {
		// We got a chunk and found no frame, that's unexpected
		if len(buf) == c.chunkSize {
			return nil, NoFrameFoundErr
		}
		buffers = append(buffers, bytes.NewReader(buf))
	} else {
		buffers, c.prev = append(buffers, bytes.NewReader(buf[0:pos])), buf[pos:]
	}

	reader := io.MultiReader(buffers...)
	return NewNewlineDelimitedFrameReader(reader, true), nil
}

// ReadAllChunks consumes all FrameReader from the chunker and returns them in
// a slice. If an error is encountered (except io.EOF) returns it immediately
// with a nil slice.
//
// Carefully use this function as it may hold the entire io.Reader in memory and
// will never return with an infinite stream. This utility function is used
// mostly for testing.
func ReadAllChunks(chunker ChunkReader) (readers []FrameReader, err error) {
	for {
		reader, err := chunker.NextChunk()
		if errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return nil, err
		}

		readers = append(readers, reader)
	}

	return readers, nil
}
