// Copyright © 2021 Optable Technologies Inc. All rights reserved.
// See LICENSE for details.
package io

import (
	"bufio"
	"encoding/binary"
	"errors"
	"io"
	"sync"

	"github.com/optable/optable-pkglib/unit"
)

// FrameWriter wraps messages (payload) and takes care of framing them in a
// stream. An example of a popular framing is the new-line delimited file
// format where each message is a single line.
//
// Framing is required for readers to provide an iterator-like interface
// without knowing about the content. The implementer is not required to provide
// any concurrency guarantees.
type FrameWriter interface {
	// Write a single message. Returns the number of bytes required to write
	// the message with framing.
	Write(payload []byte) (int, error)
}

// FrameReader reads messages framed in a stream. A FrameReader is usually the
// opposite of a FrameWriter. The implementer is not required to provide any
// concurrency guarantees. Returns io.EOF when no frames are left.
type FrameReader interface {
	// Read a single message. Returns the payload of the message.
	Read() ([]byte, error)
}

// NewVarLenWriter creates a FrameWriter where each frame is composed of the
// size (uint64) of the message encoded with varlen encoding followed by the
// message itself.
func NewVarLenFrameWriter(w io.Writer) FrameWriter {
	// Buffer used to store the varlen payload.
	var buf [binary.MaxVarintLen32]byte
	return frameWriterFn(func(payload []byte) (int, error) {
		encodedLength := binary.PutUvarint(buf[:], uint64(len(payload)))
		sync, err := w.Write(buf[:encodedLength])
		if err != nil {
			return sync, err
		}

		n, err := w.Write(payload)
		return n + sync, err
	})
}

const varlenFrameReaderBufferSize = 256

// NewVarLenReader creates a FrameReader reading the framing format defined by
// NewVarLenWriter.
func NewVarLenFrameReader(r io.Reader) FrameReader {
	// ReadVarint requires a ReadByte method.
	bufReader := bufio.NewReader(r)
	buf := make([]byte, varlenFrameReaderBufferSize)
	return frameReaderFn(func() ([]byte, error) {
		payloadLen, err := binary.ReadUvarint(bufReader)
		if err != nil {
			// If io.EOF is returned, there's no more frame and we're ok.
			return nil, err
		}

		if payloadLen > uint64(cap(buf)) {
			buf = make([]byte, payloadLen)
		}

		// ReadFull returns `err == nil` IFF len(buf) = number of read bytes.
		_, err = io.ReadFull(bufReader, buf[:payloadLen])
		if errors.Is(err, io.EOF) {
			// If io.EOF is returned, then this is unexpected.
			return nil, io.ErrUnexpectedEOF
		} else if err != nil {
			return nil, err
		}

		return buf[:payloadLen], nil
	})
}

// NewNewlineDelimitedWriter uses the trivial 'delimiter' based framing, i.e.
// it separates messages with a `\n`. It comes with the limitation that the
// payload should not contain a newline, this is the responsibility of the
// caller.
//
// This framing is not robust due to the previous limitation but is provided
// for the ubiquitous json-newline-delimited format.
func NewNewlineDelimitedFrameWriter(w io.Writer) FrameWriter {
	first := true
	newline := []byte{'\n'}
	return frameWriterFn(func(payload []byte) (int, error) {
		if first {
			first = false
			return w.Write(payload)
		}

		written, err := w.Write(newline)
		if err != nil {
			return written, err
		}

		n, err := w.Write(payload)
		return n + written, err
	})
}

// NewNewlineDelimitedReader parses stream separated by newlines. The
// implementation uses bufio.Scanner underneath which has some noted
// peculiarity:
//
// - it will support message delimited by `\r?\n`, i.e. it also supports the
// windows-based newline text format.
// - it is capped to messages of of the buffer size. If a message is larger
// than the default buffer (see `bufio.MaxScanTokenSize`), it will fail with
// `bufio.ErrTooLong`.
//
// Due to the previous points, NewNewlineDelimitedReader is not the exact
// inverse of NewNewlineDelimitedWriter.
func NewNewlineDelimitedFrameReader(r io.Reader, skipEmpty bool) FrameReader {
	scanner := bufio.NewScanner(r)
	buf := make([]byte, unit.Mebibyte)
	scanner.Buffer(buf, 0)

	return frameReaderFn(func() ([]byte, error) {
		for {
			if !scanner.Scan() {
				err := scanner.Err()
				// We reached EOF
				if err == nil {
					err = io.EOF
				}
				return nil, err
			}
			line := scanner.Bytes()
			if skipEmpty && len(line) == 0 {
				continue
			}
			return line, nil
		}
	})
}

type multiFrameReader struct {
	readers []FrameReader
}

func (r *multiFrameReader) Read() ([]byte, error) {
	for len(r.readers) > 0 {
		frame, err := r.readers[0].Read()
		if errors.Is(err, io.EOF) {
			// Allow gc to reclaim FrameReader.
			r.readers[0] = nil
			r.readers = r.readers[1:]
			continue
		} else if err != nil {
			return nil, err
		}
		return frame, nil
	}

	return nil, io.EOF
}

// NewMultiFrameReader returns a FrameReader that concatenates FrameReaders in a
// single virtual FrameReader. This is similar to io.MultiReader.
func MultiFrameReader(readers ...FrameReader) FrameReader {
	r := make([]FrameReader, len(readers))
	copy(r, readers)
	return &multiFrameReader{r}
}

// ReadAllFrames returns all frame exposed by a FrameReader until io.EOF is
// reached. If an error is encountered, it returns said error with an empty slice.
func ReadAllFrames(r FrameReader) ([][]byte, error) {
	frames := make([][]byte, 0, 16)
	for {
		frame, err := r.Read()
		if errors.Is(err, io.EOF) {
			return frames, nil
		} else if err != nil {
			return nil, err
		}

		newFrame := make([]byte, len(frame))
		copy(newFrame, frame)
		frames = append(frames, newFrame)
	}
}

type sliceFrameReader struct {
	frames [][]byte
	pos    int
}

func (s *sliceFrameReader) Read() ([]byte, error) {
	if s.pos == len(s.frames) {
		return nil, io.EOF
	}

	frame := s.frames[s.pos]
	s.pos++

	return frame, nil
}

// SliceFrameReader wraps a slice of frames in a FrameReader.
func SliceFrameReader(frames [][]byte) FrameReader {
	return &sliceFrameReader{frames: frames}
}

// ConcurrentFrameWriter protects a FrameWriter with a mutex.
func ConcurrentFrameWriter(w FrameWriter) FrameWriter {
	var mu sync.Mutex
	return frameWriterFn(func(payload []byte) (int, error) {
		mu.Lock()
		defer mu.Unlock()
		return w.Write(payload)
	})
}

type frameWriterFn func([]byte) (int, error)

func (f frameWriterFn) Write(payload []byte) (int, error) {
	return f(payload)
}

type frameReaderFn func() ([]byte, error)

func (f frameReaderFn) Read() ([]byte, error) {
	return f()
}
