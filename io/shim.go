// Copyright © 2021 Optable Technologies Inc. All rights reserved.
// See LICENSE for details.
package io

import (
	"bufio"
	"io"
	"sync"
)

// NewBufferWriteCloserSize wraps an io.Writer in a buffer that is both flushed
// and properly closed. If the writer also implements the io.Closer interface,
// it will be properly closed.
func NewBufferWriteCloserSize(w io.Writer, size int) io.WriteCloser {
	if size < 0 {
		size = defaultBufSize
	}

	buf := bufio.NewWriterSize(w, size)

	// First close the buffer
	closers := []io.Closer{CloserFn(buf.Flush)}
	// Then possibly close the writer if required.
	if wc, ok := w.(io.Closer); ok {
		closers = append(closers, wc)
	}

	return NewChainedCloser(buf, closers...)
}

const (
	defaultBufSize = 4096
)

// NewBufferWriteCloser wraps an io.Writer in a buffer that is both flushed
// and properly closed. If the writer also implements the io.Closer interface,
// it will be properly closed.
func NewBufferWriteCloser(w io.Writer) io.WriteCloser {
	return NewBufferWriteCloserSize(w, defaultBufSize)
}

// NewChainedCloser returns a io.WriteCloser that will close the provided
// closer in order. This is often required when transferring ownership of
// composed io.Writer. Some of the chained writers may need to be closed, e.g.
// os.File, net.Conn, gzip.Writer, etc. This wrapper takes care of closing the
// writers in the proper order making the new owner oblivious of all the
// involved hierarchy.
func NewChainedCloser(w io.Writer, cs ...io.Closer) io.WriteCloser {
	return &chainedCloser{Writer: w, cs: cs}
}

type chainedCloser struct {
	io.Writer
	cs []io.Closer
}

func (w *chainedCloser) Close() error {
	for _, c := range w.cs {
		if err := c.Close(); err != nil {
			return err
		}
	}

	return nil
}

type closeOnce struct {
	closer io.Closer
	err    error
	once   sync.Once
}

func (c *closeOnce) Close() error {
	c.once.Do(func() {
		c.err = c.closer.Close()
	})
	return c.err
}

// SafeClose returns a closer that is safe to call concurrently.
func SafeCloser(closer io.Closer) io.Closer {
	return &closeOnce{closer: closer}
}

// MaybeClose closes if the passed object implements io.Closer
// and does nothing otherwise
func MaybeClose(i interface{}) error {
	if closer, ok := i.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

// CloserFn implements the io.Closer interface for closures of the same
// signature.
type CloserFn func() error

func (c CloserFn) Close() error {
	return c()
}
