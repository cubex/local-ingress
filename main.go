package main

import (
	cli "gopkg.in/alecthomas/kingpin.v2"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
)

var (
	configPath = cli.Flag("config", "Path to the config yaml").Short('c').String()
)

// For testing purposes set this to false to disable the JWT check
const EnableAuthCheck = true

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
			if err != nil {
				log.Fatal(err.Error())
			}
			break
		}
	}

	if cfg == nil {
		log.Fatal("Config file not found")
	}

	p := NewProxy(cfg)
	httpServer := http.Server{Addr: cfg.ListenAddress, Handler: p}

	log.Printf("Listening on %s", cfg.ListenAddress)
	if cfg.Tls {
		log.Println("Serving with TLS")
	}
	if cfg.Tls {
		log.Fatal(httpServer.ListenAndServeTLS(cfg.TlsCertFile, cfg.TlsKeyFile))
	}
	log.Fatal(httpServer.ListenAndServe())
}
