package main

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/NYTimes/gziphandler"
	"github.com/packaged/logger/v2/ld"
)

type Proxy struct {
	P       *httputil.ReverseProxy
	c       *Config
	handler http.Handler
}

func NewProxy(config *Config) *Proxy {
	p := &Proxy{c: config}
	p.P = &httputil.ReverseProxy{
		Director: p.Director,
		ErrorHandler: func(w http.ResponseWriter, req *http.Request, err error) {
			logs.Error("request", ld.TrustedString("error", err.Error()), ld.URL(req.Host+req.RequestURI))
		},
		ModifyResponse: func(r *http.Response) error {
			logs.Info("response", ld.URL(r.Request.Host+r.Request.RequestURI), ld.TrustedString("status", r.Status), ld.TrustedString("code", strconv.Itoa(r.StatusCode)))
			return nil
		},
	}
	if config.GZip {
		p.handler = gziphandler.GzipHandler(p.P)
	} else {
		p.handler = p.P
	}
	return p
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if _, hasPort := p.getPort(r.Host); hasPort {
		if p.c.Tls {
			r.Header.Add("X-Forwarded-Proto", "https")
		}
		p.handler.ServeHTTP(w, r)
	} else {
		http.Error(w, "The host you are trying to access has not yet been configured", http.StatusNotFound)
	}
}

func (p *Proxy) Director(r *http.Request) {
	r.URL.Scheme = "http"
	if usePort, hasPort := p.getPort(r.Host); hasPort {
		if strings.ContainsAny(usePort, ":.") {
			remoteUrl, _ := url.Parse(usePort)
			r.URL.Host = remoteUrl.Host
			if remoteUrl.Scheme != "" {
				r.URL.Scheme = remoteUrl.Scheme
			}
		} else {
			targetHost := r.Host
			if !strings.Contains(targetHost, ":") {
				srv := r.Context().Value(http.ServerContextKey).(*http.Server)
				targetHost = targetHost + srv.Addr
			}
			if p.c.Tunnel != "" {
				r.URL.Host = "127.0.0.1:" + usePort
			} else {
				r.URL.Host = strings.Replace(targetHost, p.c.ListenAddress, ":"+usePort, 1)
			}
		}
	} else {
		logs.Info(fmt.Sprintf("%s is not a supported host", r.Host))
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
