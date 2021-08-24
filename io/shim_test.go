// Copyright © 2021 Optable Technologies Inc. All rights reserved.
// See LICENSE for details.
package io

import (
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBufferWriterCloser(t *testing.T) {
	buf := new(bytes.Buffer)

	bufSize := 32
	writeCloser := NewBufferWriteCloserSize(buf, bufSize)

	payload := []byte("helloworld")
	assert.Less(t, len(payload), bufSize)

	n, err := writeCloser.Write(payload)
	assert.NoError(t, err)
	assert.Equal(t, len(payload), n)

	// Since the payload is smaller than the buffer, nothing is flushed.
	assert.Empty(t, buf.Bytes())

	assert.NoError(t, writeCloser.Close())
	assert.Equal(t, payload, buf.Bytes())
}

func TestChainedCloser(t *testing.T) {
	var seq int
	isClosed := func(barrier *int) io.Closer {
		return CloserFn(func() error {
			seq++
			*barrier = seq
			return nil
		})
	}

	w := new(bytes.Buffer)
	var a, b, c int
	wc := NewChainedCloser(w, isClosed(&a), isClosed(&b), isClosed(&c))

	payload := []byte("helloworld")

	n, err := wc.Write(payload)
	assert.NoError(t, err)
	assert.Equal(t, len(payload), n)

	// Not closed yet.
	assert.Equal(t, 0, a)
	assert.Equal(t, 0, b)
	assert.Equal(t, 0, c)

	assert.NoError(t, wc.Close())
	// Closed in order of chaining.
	assert.Equal(t, 1, a)
	assert.Equal(t, 2, b)
	assert.Equal(t, 3, c)
}
