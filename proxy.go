package main

import (
	"log"
	"net/http"
	"net/http/httputil"
	"regexp"
	"strings"
)

type Proxy struct {
	P *httputil.ReverseProxy
	c *Config
}

func NewProxy(config *Config) *Proxy {
	p := &Proxy{c: config}
	p.P = &httputil.ReverseProxy{Director: p.Director}
	return p
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if _, hasPort := p.getPort(r.Host); hasPort {
		p.P.ServeHTTP(w, r)
	} else {
		http.Error(w, "The host you are trying to access has not yet been configured", http.StatusNotFound)
	}
}

func (p *Proxy) Director(r *http.Request) {
	r.URL.Scheme = "http"
	if usePort, hasPort := p.getPort(r.Host); hasPort {
		r.URL.Host = strings.Replace(r.Host, p.c.ListenAddress, ":"+usePort, 1)
	} else {
		log.Print(r.Host, " is not a supported host")
	}
}

func (p *Proxy) getPort(host string) (string, bool) {
	baseHost := strings.Replace(host, p.c.ListenAddress, "", 1)
	usePort, hasPort := p.c.HostMap[baseHost]
	if !hasPort {
		for tryHost, tryPort := range p.c.HostMap {
			if regexp.MustCompile(tryHost).MatchString(baseHost) {
				return tryPort, true
			}
		}
	}
	return usePort, hasPort
}
