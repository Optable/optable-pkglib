// Copyright © 2021 Optable Technologies Inc. All rights reserved.
// See LICENSE for details.
package cli

import (
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func requireTempDir(t *testing.T) string {
	dir, err := ioutil.TempDir("", "pkglib-test-*")
	require.NoError(t, err)
	return dir
}

func TestOpenConfigDirFailsOnFiles(t *testing.T) {
	file, err := ioutil.TempFile("", "pkglib-test-*")
	require.NoError(t, err)
	defer os.Remove(file.Name())
	configDir, err := NewConfigDir(file.Name(), &JSONMapLoader{})
	assert.Nil(t, configDir)
	assert.Error(t, err)
}

func TestOpenConfigDirDoesntFailOnDir(t *testing.T) {
	dir := requireTempDir(t)
	defer os.RemoveAll(dir)
	configDir, err := NewConfigDir(dir, &JSONMapLoader{})
	assert.NotNil(t, configDir)
	assert.NoError(t, err)
}

func TestConfigDirCurrentFailsOnAbsentLink(t *testing.T) {
	dir := requireTempDir(t)
	defer os.RemoveAll(dir)

	configDir, err := NewConfigDir(dir, &JSONMapLoader{})
	require.NoError(t, err)

	current, config, err := configDir.Current()
	assert.Empty(t, current)
	assert.Nil(t, config)
	assert.Error(t, err)
}

func TestConfigDirOnlyListRecognizedFiles(t *testing.T) {
	dir := requireTempDir(t)
	defer os.RemoveAll(dir)
	_, err := os.CreateTemp(dir, "nope-*.chose")
	require.NoError(t, err)
	_, err = os.CreateTemp(dir, "yes-*"+configExt)
	require.NoError(t, err)

	configDir, err := NewConfigDir(dir, &JSONMapLoader{})
	require.NoError(t, err)
	list, err := configDir.List()
	require.NoError(t, err)
	assert.Len(t, list, 1)
	assert.True(t, strings.HasPrefix(list[0], "yes-"))
}

func TestConfigDirSetDumpsAndLoadConfig(t *testing.T) {
	dir := requireTempDir(t)
	defer os.RemoveAll(dir)

	configDir, err := NewConfigDir(dir, &JSONMapLoader{})
	require.NoError(t, err)

	fortyTwoConfig := map[string]interface{}{
		"name":  "forty two",
		"count": 42,
		"odd":   false,
	}

	twentyOne := map[string]interface{}{
		"name":  "twenty one",
		"count": 21,
		"odd":   true,
	}

	err = configDir.Set("fortytwo", &fortyTwoConfig)
	require.NoError(t, err)

	err = configDir.Set("twentyone", &twentyOne)
	require.NoError(t, err)

	err = configDir.Use("twentyone")
	require.NoError(t, err)

	// Recreating a config dir to show state is loaded from disk
	configDir, err = NewConfigDir(dir, &JSONMapLoader{})
	require.NoError(t, err)

	configs, err := configDir.List()
	require.NoError(t, err)

	assert.Len(t, configs, 2)

	current, config, err := configDir.Current()
	require.NoError(t, err)
	assert.Equal(t, current, "twentyone")

	currentMap := config.(map[string]interface{})
	assert.Equal(t, "twenty one", currentMap["name"])
	assert.Equal(t, float64(21), currentMap["count"])
	assert.Equal(t, true, currentMap["odd"])
}
