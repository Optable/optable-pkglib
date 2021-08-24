// Copyright © 2021 Optable Technologies Inc. All rights reserved.
// See LICENSE for details.
package io

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func basicTestFraming(t *testing.T, w FrameWriter, r FrameReader) {
	data := []byte("0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

	var expected [][]byte
	for i := 0; i < len(data); i++ {
		frame := data[:i]
		_, err := w.Write(frame)
		assert.NoError(t, err)
		expected = append(expected, frame)
	}

	actual, err := ReadAllFrames(r)
	assert.NoError(t, err)
	assert.EqualValues(t, expected, actual)
}

func TestVarLenFraming(t *testing.T) {
	buf := new(bytes.Buffer)
	w := NewVarLenFrameWriter(buf)
	r := NewVarLenFrameReader(buf)
	basicTestFraming(t, w, r)
}

func TestNewlineDelimitedFraming(t *testing.T) {
	buf := new(bytes.Buffer)
	w := NewNewlineDelimitedFrameWriter(buf)
	skipEmpty := false
	r := NewNewlineDelimitedFrameReader(buf, skipEmpty)
	basicTestFraming(t, w, r)
}
