package main

import (
	"net/http"
	"os"
	"path"
	"path/filepath"

	"github.com/packaged/logger/v2"
	"github.com/packaged/logger/v2/ld"
	cli "gopkg.in/alecthomas/kingpin.v2"
)

var (
	configPath = cli.Flag("config", "Path to the config yaml").Short('c').String()
)

// For testing purposes set this to false to disable the JWT check
const EnableAuthCheck = true

var logs = logger.DevelopmentInstance()

func main() {
	cli.Parse()

	var configPaths []string
	if *configPath != "" {
		// config file specified
		if filepath.IsAbs(*configPath) {
			// absolute path
			configPaths = append(configPaths, *configPath)
		} else if cwd, err := os.Getwd(); err == nil {
			// relative path
			configPaths = append(configPaths, path.Join(cwd, *configPath))
		}
	} else {
		if cwd, err := os.Getwd(); err == nil {
			// no config specified, search in current directory
			configPaths = append(configPaths, path.Join(cwd, "config.yaml"))
		}

		if binPath, err := filepath.Abs(filepath.Dir(os.Args[0])); err == nil {
			// search in binary directory
			configPaths = append(configPaths, path.Join(binPath, "config.yaml"))
		}
	}

	var cfg *Config
	for _, configFile := range configPaths {
		info, err := os.Stat(configFile)
		if !os.IsNotExist(err) && !info.IsDir() {
			cfg, err = LoadConfig(configFile)
			logs.FatalIf(err, "loading config")
			break
		}
	}

	if cfg == nil {
		logs.Fatal("Config file not found")
	}

	go startSshTunnel(cfg)

	p := NewProxy(cfg)
	httpServer := http.Server{Addr: cfg.ListenAddress, Handler: p}

	logs.Debug("Listening", ld.TrustedString("host", cfg.ListenAddress))
	if cfg.Tls {
		logs.Debug("Serving with TLS")
	}
	if cfg.Tls {
		logs.FatalIf(httpServer.ListenAndServeTLS(cfg.TlsCertFile, cfg.TlsKeyFile), "serve")
	}
	logs.FatalIf(httpServer.ListenAndServe(), "serve")
}
