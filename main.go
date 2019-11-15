package main

import (
	"log"
	"net/http"
)

// For testing purposes set this to false to disable the JWT check
const EnableAuthCheck = true

func main() {
	cfg, err := LoadConfig("config.yaml")
	if err != nil {
		log.Fatal(err.Error())
	}

	p := NewProxy(cfg)
	httpServer := http.Server{Addr: cfg.ListenAddress, Handler: p}

	log.Printf("Listening on %s", cfg.ListenAddress)
	log.Fatal(httpServer.ListenAndServe())
}
