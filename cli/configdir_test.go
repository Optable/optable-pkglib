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
	configDir, err := NewConfigDir(file.Name(), &JSONLoader{})
	assert.Nil(t, configDir)
	assert.Error(t, err)
}

func TestOpenConfigDirDoesntFailOnDir(t *testing.T) {
	dir := requireTempDir(t)
	defer os.RemoveAll(dir)
	configDir, err := NewConfigDir(dir, &JSONLoader{})
	assert.NotNil(t, configDir)
	assert.NoError(t, err)
}

func TestConfigDirCurrentFailsOnAbsentLink(t *testing.T) {
	dir := requireTempDir(t)
	defer os.RemoveAll(dir)

	configDir, err := NewConfigDir(dir, &JSONLoader{})
	require.NoError(t, err)

	dummy := struct{}{}
	config, err := configDir.Current(&dummy)
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

	configDir, err := NewConfigDir(dir, &JSONLoader{})
	require.NoError(t, err)
	list, err := configDir.List()
	require.NoError(t, err)
	assert.Len(t, list, 1)
	assert.True(t, strings.HasPrefix(list[0], "yes-"))
}

func TestConfigDirValidatesName(t *testing.T) {
	type someConfig struct{}

	dir := requireTempDir(t)
	defer os.RemoveAll(dir)
	loader := &JSONLoader{}
	configDir, err := NewConfigDir(dir, loader)
	require.NoError(t, err)

	invalids := []string{
		"/etc/passwd",
		"/",
		"",
		" ",
		"-",
		".",
		"..",
	}
	conf := &someConfig{}
	for _, invalid := range invalids {
		err = configDir.Set(invalid, conf)
		assert.Error(t, err)
	}

	err = configDir.Set("valid-name", conf)
	assert.NoError(t, err)
}

func TestConfigDirSetDumpsAndLoadConfig(t *testing.T) {
	type someConfig struct {
		Name  string
		Count int
		Odd   bool
	}

	dir := requireTempDir(t)
	defer os.RemoveAll(dir)
	loader := &JSONLoader{}

	configDir, err := NewConfigDir(dir, loader)
	require.NoError(t, err)

	fortyTwoConfig := &someConfig{
		Name:  "forty two",
		Count: 42,
		Odd:   false,
	}

	twentyOne := &someConfig{
		Name:  "twenty one",
		Count: 21,
		Odd:   true,
	}

	err = configDir.Set("fortytwo", &fortyTwoConfig)
	require.NoError(t, err)

	err = configDir.Set("twentyone", &twentyOne)
	require.NoError(t, err)

	err = configDir.Use("twentyone")
	require.NoError(t, err)

	// Recreating a config dir to show state is loaded from disk
	configDir, err = NewConfigDir(dir, loader)
	require.NoError(t, err)

	configs, err := configDir.List()
	require.NoError(t, err)

	assert.Len(t, configs, 2)

	current := &someConfig{}
	info, err := configDir.Current(current)

	require.NoError(t, err)
	require.NotNil(t, info)
	assert.Equal(t, info.Name, "twentyone")
	assert.Equal(t, info.Path, dir+"/twentyone.conf")

	assert.Equal(t, "twenty one", current.Name)
	assert.Equal(t, 21, current.Count)
	assert.Equal(t, true, current.Odd)
}
