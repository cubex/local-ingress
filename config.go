package main

import (
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"time"
)

type Config struct {
	HostMap       map[string]string `yaml:"hostMap"`
	ListenAddress string            `yaml:"listenAddress"`
	file          string
}

func (c *Config) reload() error {
	contents, err := ioutil.ReadFile(c.file)
	if err != nil {
		return err
	}

	return yaml.Unmarshal(contents, c)
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
