// Copyright © 2021 Optable Technologies Inc. All rights reserved.
// See LICENSE for details.
package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/adrg/xdg"
	"github.com/alecthomas/kong"
)

type (
	// ConfigDir allows managing a multiple contextual configuration files. Each
	// configuration is given a context name, e.g. `prod`, `staging`, `devel` and
	// each stores a specific configuration.
	ConfigDir struct {
		path   string
		loader ConfigLoader
	}

	configInfo struct {
		Name string
		Path string
	}

	ConfigDirOption interface {
		apply(opt *ConfigDir) error
	}

	configDirOptionFn func(opt *ConfigDir) error
)

// NewConfigDir creates a ConfigDir at a given path.
func NewConfigDir(path string, opts ...ConfigDirOption) (*ConfigDir, error) {
	cfg := &ConfigDir{path: path, loader: JSONLoader}
	for _, opt := range opts {
		if err := opt.apply(cfg); err != nil {
			return nil, err
		}
	}

	stat, err := os.Stat(cfg.path)
	if err != nil {
		return nil, fmt.Errorf("ConfigDir's '%s' error: %w", cfg.path, err)
	}

	if !stat.Mode().IsDir() {
		return nil, fmt.Errorf("ConfigDir's '%s' is not a directory", cfg.path)
	}

	return cfg, nil
}

func (fn configDirOptionFn) apply(opt *ConfigDir) error {
	return fn(opt)
}

func WithConfigDirLoader(loader ConfigLoader) ConfigDirOption {
	return configDirOptionFn(func(opt *ConfigDir) error {
		opt.loader = loader
		return nil
	})
}

func WithXdgConfigPath(configPath string) ConfigDirOption {
	return configDirOptionFn(func(opt *ConfigDir) error {
		// xdg ensure that the parent directories are automatically created. Thus we
		// ask for a dummy file with the correct parent path..
		configPath, err := xdg.ConfigFile(path.Join(configPath, "dummy"))
		if err != nil {
			return fmt.Errorf("Failed creating configuration file path: %w", err)
		}
		// Remove the dummy part.
		opt.path = path.Dir(configPath)
		return nil
	})
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

type (
	ConfigDirFlag struct {
		Config string `opt:""`
	}

	ConfigUseCmd struct {
		Name string `arg:"" placeholder:"<name>"`
	}

	ConfigListCmd struct {
	}

	ConfigDirCmd struct {
		Use  ConfigUseCmd  `cmd:"use"`
		List ConfigListCmd `cmd:"list"`
	}

	ConfigDirCli struct {
		ConfigDirFlag
		Config ConfigDirCmd `cmd:"config"`

		path      string
		options   []ConfigDirOption
		configDir *ConfigDir
	}
)

func (c *ConfigDirCli) KongInit(path string, options ...ConfigDirOption) kong.Option {
	c.path = path
	c.options = options
	return kong.Bind(c)
}

func (c *ConfigDirCli) Get(cfg interface{}) error {
	configDir := c.configDir
	target := c.ConfigDirFlag.Config

	if target == "" {
		_, err := configDir.Current(cfg)
		return err
	}

	return configDir.Get(target, cfg)
}

func (c *ConfigDirCli) Set(cfg interface{}, setCurrent bool) error {
	target := c.ConfigDirFlag.Config
	if target == "" {
		target = "default"
	}

	if err := c.configDir.Set(target, cfg); err != nil {
		return err
	}

	if setCurrent {
		return c.configDir.Use(target)
	}

	return nil
}

func (c *ConfigDirCli) load() (err error) {
	if c.configDir != nil {
		return nil
	}
	c.configDir, err = NewConfigDir(c.path, c.options...)
	return err
}

func (f *ConfigDirFlag) BeforeResolve(c *ConfigDirCli) (err error) {
	return c.load()
}

func (f *ConfigDirFlag) Help() string {
	return "Name of the configuration to load. See 'config' sub-command help for more information."
}

func (u *ConfigDirCmd) BeforeResolve(c *ConfigDirCli) (err error) {
	return c.load()
}

func (u *ConfigDirCmd) Help() string {
	return `The config sub-command family of commands allow managing parallel
configurations. A configuration contains the information, usually URL and
credentials, that allow connecting to a service. Configurations are referenced by
a unique name, for example 'prod' or 'staging'.
`
}

func (u *ConfigListCmd) BeforeResolve(c *ConfigDirCli) (err error) {
	return c.load()
}

func (u *ConfigListCmd) Run(c *ConfigDirCli) error {
	configs, err := c.configDir.List()
	if err != nil {
		return fmt.Errorf("Failed listing configs: %w", err)
	}

	for _, name := range configs {
		fmt.Println(name)
	}

	return nil
}

func (u *ConfigUseCmd) BeforeResolve(c *ConfigDirCli) (err error) {
	return c.load()
}

func (u *ConfigUseCmd) Run(c *ConfigDirCli) error {
	return c.configDir.Use(u.Name)
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

// Starts with an alphanum and at least 2 characters to avoid "-" config names
// which can be dangerous to work with when interacting with shells.
const allowedConfigNamePattern = "[a-zA-Z0-9][a-zA-Z0-9-_]+"

var allowedConfigNameRegexp = regexp.MustCompile(allowedConfigNamePattern)

func (c *ConfigDir) configInfo(name string, mustExist bool) (*configInfo, error) {
	if !allowedConfigNameRegexp.MatchString(name) {
		return nil, fmt.Errorf("Context name must match: %s", allowedConfigNamePattern)
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

type ConfigLoader interface {
	Unmarshal([]byte, interface{}) error
	Marshal(interface{}) ([]byte, error)
}

// Simple implementation of a loader marshaling from/into a json structure
type jsonLoader struct{}

var JSONLoader = &jsonLoader{}

func (l *jsonLoader) Unmarshal(b []byte, to interface{}) error {
	return json.Unmarshal(b, to)
}

func (l *jsonLoader) Marshal(from interface{}) ([]byte, error) {
	return json.Marshal(from)
}
