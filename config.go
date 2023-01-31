package main

import (
	"errors"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v2"
)

type Config struct {
	HostMap       map[string]string `yaml:"hostMap"`
	ListenAddress string            `yaml:"listenAddress"`
	GZip          bool              `yaml:"gzip"`
	Tls           bool              `yaml:"tls"`
	TlsCertFile   string            `yaml:"certFile"`
	TlsKeyFile    string            `yaml:"keyFile"`
	Tunnel        string            `yaml:"tunnel"`
	file          string
}

func (c *Config) reload() error {
	contents, err := os.ReadFile(c.file)
	if err != nil {
		return err
	}

	err = yaml.Unmarshal(contents, c)
	if err == nil {
		if c.Tls {
			c.TlsCertFile = resolvePath(c.TlsCertFile, c.file)
			c.TlsKeyFile = resolvePath(c.TlsKeyFile, c.file)
		}
	}

	return err
}

func LoadConfig(configFile string) (*Config, error) {
	cfg := &Config{file: configFile}
	err := cfg.reload()

	go func(c *Config) {
		for {
			err := c.reload()
			logs.FatalIf(err, "reloading content")
			time.Sleep(time.Second * 10)
		}
	}(cfg)

	return cfg, err
}

func resolvePath(checkPath string, configPath string) string {
	if checkPath == "~" || strings.HasPrefix(checkPath, "~/") {
		usr, _ := user.Current()
		checkPath = path.Join(usr.HomeDir, checkPath[1:])
	}

	if !filepath.IsAbs(checkPath) {
		checkPath, _ = filepath.Abs(path.Join(filepath.Dir(configPath), checkPath))
	}

	if _, err := os.Stat(checkPath); errors.Is(err, os.ErrNotExist) {
		logs.Fatal(checkPath + ": file does not exist")
	}

	return checkPath
}
