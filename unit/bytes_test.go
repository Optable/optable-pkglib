// Copyright © 2021 Optable Technologies Inc. All rights reserved.
// See LICENSE for details.
package unit

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestByteUnits(t *testing.T) {
	assert.Equal(t, 1, Byte)

	assert.Equal(t, 1024, Kibibyte)
	assert.Equal(t, 1024*1024, Mebibyte)
	assert.Equal(t, 1024*1024*1024, Gibibyte)
	assert.Equal(t, 1024*1024*1024*1024, Tebibyte)
	assert.Equal(t, 1024*1024*1024*1024*1024, Pebibyte)

	assert.Equal(t, KiB, Kibibyte)
	assert.Equal(t, MiB, Mebibyte)
	assert.Equal(t, GiB, Gibibyte)
	assert.Equal(t, TiB, Tebibyte)
	assert.Equal(t, PiB, Pebibyte)
}
