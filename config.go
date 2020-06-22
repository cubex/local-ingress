package main

import (
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"path"
	"path/filepath"
	"time"
)

type Config struct {
	HostMap       map[string]string `yaml:"hostMap"`
	ListenAddress string            `yaml:"listenAddress"`
	GZip          bool              `yaml:"gzip"`
	Tls           bool              `yaml:"tls"`
	TlsCertFile   string            `yaml:"certFile"`
	TlsKeyFile    string            `yaml:"keyFile"`
	file          string
}

func (c *Config) reload() error {
	contents, err := ioutil.ReadFile(c.file)
	if err != nil {
		return err
	}

	err = yaml.Unmarshal(contents, c)
	if err == nil && c.Tls {
		c.TlsCertFile, _ = filepath.Abs(path.Join(filepath.Dir(c.file), c.TlsCertFile))
		c.TlsKeyFile, _ = filepath.Abs(path.Join(filepath.Dir(c.file), c.TlsKeyFile))
	}
	return err
}

func LoadConfig(configFile string) (*Config, error) {
	cfg := &Config{file: configFile}
	err := cfg.reload()

	go func(c *Config) {
		for {
			err := c.reload()
			if err != nil {
				log.Print(err)
			}
			time.Sleep(time.Second * 10)
		}
	}(cfg)

	return cfg, err
}
