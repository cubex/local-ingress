package main

import (
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
)

// For testing purposes set this to false to disable the JWT check
const EnableAuthCheck = true

func main() {
	configFile := "config.yaml"

	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err == nil {
		configFile = path.Join(dir, configFile)
	}

	cfg, err := LoadConfig(configFile)
	if err != nil {
		log.Fatal(err.Error())
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
