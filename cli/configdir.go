// Copyright © 2021 Optable Technologies Inc. All rights reserved.
// See LICENSE for details.
package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type ConfigLoader interface {
	Unmarshal([]byte) (interface{}, error)
	Marshal(interface{}) ([]byte, error)
}

// Simple implementation of a loader marshaling from/into a flat json map
type JSONMapLoader struct{}

func (l *JSONMapLoader) Unmarshal(b []byte) (interface{}, error) {
	object := make(map[string]interface{})
	if err := json.Unmarshal(b, &object); err != nil {
		return nil, err
	}
	return object, nil
}

func (l *JSONMapLoader) Marshal(obj interface{}) ([]byte, error) {
	return json.Marshal(obj)
}

// ConfigDir allows managing a multiple contextual configuration files
type ConfigDir struct {
	path   string
	loader ConfigLoader
}

// We might want to make that configurable, the idea of having a known suffix is to allow
// other programs to write files in the config dir without being picked up by the facility.
// There might be better ways of doing that.
const configExt = ".conf"

func NewConfigDir(path string, loader ConfigLoader) (*ConfigDir, error) {
	stat, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !stat.Mode().IsDir() {
		return nil, fmt.Errorf("%s is not a directory", path)
	}

	return &ConfigDir{path, loader}, nil
}

func (c *ConfigDir) LoadPath(path string) (interface{}, error) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed loading config at %s: %w", path, err)
	}

	return c.loader.Unmarshal(bytes)
}

func (c *ConfigDir) DumpPath(path string, configData interface{}) error {
	bytes, err := c.loader.Marshal(configData)
	if err != nil {
		return fmt.Errorf("failed marshaling config at %s: %w", path, err)
	}

	return os.WriteFile(path, bytes, 0666)
}

func (c *ConfigDir) configPath(name string) string {
	return filepath.Join(c.path, name) + configExt
}

func (c *ConfigDir) Get(name string) (interface{}, error) {
	return c.LoadPath(c.configPath(name))
}

func (c *ConfigDir) Set(name string, configData interface{}) error {
	return c.DumpPath(c.configPath(name), configData)
}

func (c *ConfigDir) Use(name string) error {
	configPath := c.configPath(name)
	linkPath := filepath.Join(c.path, "current")
	return os.Symlink(configPath, linkPath)
}

func configName(path string) string {
	return filepath.Base(strings.TrimSuffix(path, configExt))
}

func (c *ConfigDir) List() ([]string, error) {
	entries, err := os.ReadDir(c.path)
	if err != nil {
		return nil, err
	}

	list := make([]string, 0, len(entries))
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) != configExt || !entry.Type().IsRegular() {
			continue
		}

		list = append(list, configName(entry.Name()))
	}

	return list, nil
}

func (c *ConfigDir) Current() (string, interface{}, error) {
	linkPath := filepath.Join(c.path, "current")
	info, err := os.Lstat(linkPath)
	if err != nil {
		return "", nil, err
	}

	if info.Mode()&os.ModeSymlink == 0 {
		return "", nil, errors.New("invalid current link")
	}

	currentPath, err := os.Readlink(linkPath)
	if err != nil {
		return "", nil, fmt.Errorf("failed loading current link: %w", err)
	}

	config, err := c.LoadPath(currentPath)
	if err != nil {
		return "", nil, fmt.Errorf("failed loading current config: %w", err)
	}
	return configName(currentPath), config, nil
}
