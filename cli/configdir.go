// Copyright © 2021 Optable Technologies Inc. All rights reserved.
// See LICENSE for details.
package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type ConfigLoader interface {
	Unmarshal([]byte, interface{}) error
	Marshal(interface{}) ([]byte, error)
}

// Simple implementation of a loader marshaling from/into a json structure
type JSONLoader struct{}

func (l *JSONLoader) Unmarshal(b []byte, to interface{}) error {
	return json.Unmarshal(b, to)
}

func (l *JSONLoader) Marshal(from interface{}) ([]byte, error) {
	return json.Marshal(from)
}

// ConfigDir allows managing a multiple contextual configuration files
type ConfigDir struct {
	path   string
	loader ConfigLoader
}

type configInfo struct {
	Name string
	Path string
}

// We might want to make that configurable, the idea of having a known suffix is to allow
// other programs to write files in the config dir without being picked up by the facility.
// There might be better ways of doing that.
const configExt = ".conf"

// File containing the pointer to current config
const currentName = ".current"

func configName(path string) string {
	return filepath.Base(strings.TrimSuffix(path, configExt))
}

func NewConfigDir(path string, loader ConfigLoader) (*ConfigDir, error) {
	stat, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !stat.Mode().IsDir() {
		return nil, errors.New("not a directory")
	}

	return &ConfigDir{path, loader}, nil
}

func (c *ConfigDir) load(info *configInfo, as interface{}) error {
	bytes, err := os.ReadFile(info.Path)
	if err != nil {
		return err
	}

	return c.loader.Unmarshal(bytes, as)
}

func (c *ConfigDir) dump(info *configInfo, from interface{}) error {
	bytes, err := c.loader.Marshal(from)
	if err != nil {
		return err
	}

	return os.WriteFile(info.Path, bytes, 0666)
}

// At least one alphanum, "-" or "_"
var allowedConfigNameChars = regexp.MustCompile("[a-zA-Z0-9-_]")

func (c *ConfigDir) configInfo(name string, mustExist bool) (*configInfo, error) {
	// Force at least two char to avoid "-" config names which can be dangerous to work with when
	// interacting with shells
	if len(name) < 2 {
		return nil, errors.New("must be at least 2 char long")
	}
	if !allowedConfigNameChars.MatchString(name) {
		return nil, errors.New("only alphanum and dash allowed")
	}

	path := filepath.Join(c.path, name) + configExt
	if mustExist {
		stat, err := os.Stat(path)
		if err != nil {
			return nil, err
		}
		if !stat.Mode().IsRegular() {
			return nil, errors.New("not a regular file")
		}
	}

	return &configInfo{Path: path, Name: name}, nil
}

func errConfigDir(name string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("configdir: %s: %w", name, err)
}

func (c *ConfigDir) Get(name string, as interface{}) error {
	info, err := c.configInfo(name, true)
	if err != nil {
		return errConfigDir(name, fmt.Errorf("get info: %w", err))
	}
	if err := c.load(info, as); err != nil {
		return errConfigDir(name, fmt.Errorf("load: %w", err))
	}
	return nil
}

func (c *ConfigDir) Set(name string, from interface{}) error {
	info, err := c.configInfo(name, false)
	if err != nil {
		return errConfigDir(name, fmt.Errorf("get info: %w", err))
	}
	if err := c.dump(info, from); err != nil {
		return errConfigDir(name, fmt.Errorf("dump: %w", err))
	}
	return nil
}

func (c *ConfigDir) Use(name string) error {
	_, err := c.configInfo(name, true)
	if err != nil {
		return errConfigDir(name, fmt.Errorf("get info: %w", err))
	}

	linkPath := filepath.Join(c.path, currentName)
	file, err := os.Create(linkPath)
	if err != nil {
		return errConfigDir(name, fmt.Errorf("link current: %w", err))
	}

	if _, err := file.Write([]byte(name)); err != nil {
		return errConfigDir(name, fmt.Errorf("write current: %w", err))
	}
	return nil
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

func (c *ConfigDir) Current(as interface{}) (*configInfo, error) {
	linkPath := filepath.Join(c.path, currentName)
	linkStat, err := os.Stat(linkPath)
	if err != nil {
		return nil, err
	}

	if !linkStat.Mode().IsRegular() {
		return nil, errConfigDir(currentName, errors.New("not a regular file"))
	}

	linkContent, err := os.ReadFile(linkPath)
	if err != nil {
		return nil, errConfigDir(currentName, err)
	}

	name := string(linkContent)

	info, err := c.configInfo(name, true)
	if err != nil {
		return nil, errConfigDir(name, err)
	}

	if err := c.load(info, as); err != nil {
		return nil, errConfigDir(name, err)
	}

	return info, nil
}
