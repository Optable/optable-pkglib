// Copyright © 2021 Optable Technologies Inc. All rights reserved.
// See LICENSE for details.
package io

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func assertChunkReaderRoundTrip(t *testing.T, framer FrameReader, chunker ChunkReader) {
	expected, err := ReadAllFrames(framer)
	assert.NoError(t, err)

	readers, err := ReadAllChunks(chunker)
	assert.NoError(t, err)

	actual, err := ReadAllFrames(MultiFrameReader(readers...))
	assert.NoError(t, err)

	assert.Equal(t, expected, actual)
}

const chunkSize = 128

func assertNewLineDelimitedChunker(t *testing.T, payload string) {
	framer := NewNewlineDelimitedFrameReader(bytes.NewBufferString(payload), true)
	chunker, err := NewNewlineDelimitedChunkReader(bytes.NewBufferString(payload), chunkSize)
	assert.NoError(t, err)
	assertChunkReaderRoundTrip(t, framer, chunker)
}

func TestEmptyNewLineDelimitedChunker(t *testing.T) {
	assertNewLineDelimitedChunker(t, "")
}

func TestOneNewLineDelimitedChunker(t *testing.T) {
	assertNewLineDelimitedChunker(t, "c:bob")
}

func TestExtraNewLineDelimitedChunker(t *testing.T) {
	assertNewLineDelimitedChunker(t, "c:bob\n")
}

func TestNewLineDelimitedChunker(t *testing.T) {
	lines := `
e:538c7f96b164bf1b97bb9f4bb472e89f5b1484f25209c9d9343e92ba09dd9d52
e:dfd79b4d76429b617a0c9f9f0d3ba55b0cc0d6144c888535841acbe0709b0758
e:083f61d375bc02b41df4f91929e18fda9e6f82e54e748e81e79e4bbd6fe34cdc
e:ba843ee8d63e8c4ffe1cebea546d8fac13dd1aac04ce2ea2877c5579cfa2c78e
e:1b0bafae881b82a751108a42ed3c903caa43465a78620616978aed0ce3c6c4f3
e:ae7bc3e0495b5712fefdbe0c102887e100dacd2d885f692cb607da00a11c1c70
e:71e796a2dc2dc25a5b74b2e129705e273f05c92326828e2b056e3817658e1061
e:498947fdf344410ed4c116023fa8e3576b6fed27ff8974bac0cafd9ad05692b1
e:3619e738964dfdc79e8d534373661cfd66d74fec1e1b89491ab7236e4b752162
e:90cf2beb42c3ca27328560f1aac067cea6e8bf46d4ab2b4680402c5fb2820e88
`
	assertNewLineDelimitedChunker(t, lines)
}
