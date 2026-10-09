package main

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/NYTimes/gziphandler"
	"github.com/packaged/logger/v2/ld"
	"go.uber.org/zap"
)

type Proxy struct {
	P       *httputil.ReverseProxy
	c       *Config
	handler http.Handler
}

func NewProxy(config *Config) *Proxy {
	p := &Proxy{c: config}
	p.P = &httputil.ReverseProxy{
		Rewrite:       p.Rewriter,
		FlushInterval: -1, // flush immediately for streaming (SSE/WebSocket)
		ErrorHandler: func(w http.ResponseWriter, req *http.Request, err error) {
			logs.Error("error", ld.TrustedString("message", err.Error()), ld.URL(req.Host+req.RequestURI))
		},
		ModifyResponse: func(r *http.Response) error {
			if *verboseLog {
				logs.Info("response", ld.URL(r.Request.Host+r.Request.RequestURI), ld.TrustedString("status", r.Status), ld.TrustedString("code", strconv.Itoa(r.StatusCode)))
			}
			return nil
		},
	}
	p.handler = p.P
	if config.GZip {
		p.handler = gziphandler.GzipHandler(p.handler)
	}
	return p
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if _, hasDestination := p.getDestination(r.Host); hasDestination {
		if p.c.Tls {
			r.Header.Add("X-Forwarded-Proto", "https")
		}

		isWS := strings.EqualFold(r.Header.Get("Upgrade"), "websocket")
		isSSE := strings.Contains(r.Header.Get("Accept"), "text/event-stream")

		if isWS || isSSE {
			if !p.c.Streaming {
				http.Error(w, "Streaming connections are not enabled", http.StatusForbidden)
				return
			}
			// Bypass gzip — gziphandler wraps the ResponseWriter which
			// prevents TCP hijack (WebSocket) and buffered flushing (SSE).
			p.P.ServeHTTP(w, r)
		} else {
			p.handler.ServeHTTP(w, r)
		}
	} else {
		http.Error(w, "The host you are trying to access has not yet been configured", http.StatusNotFound)
	}
}

func (p *Proxy) Rewriter(r *httputil.ProxyRequest) {
	if useDestination, hasDestination := p.getDestination(r.In.Host); hasDestination {
		logs.Info("request", zap.String("from", r.In.Host), zap.String("to", useDestination))
		if strings.ContainsAny(useDestination, ":.") {
			remoteUrl, _ := url.Parse(useDestination)
			remoteUrl.JoinPath(r.In.URL.Path)
			r.SetURL(remoteUrl)
		} else {
			var targetHost string
			if p.c.tunnelName() != "" {
				targetHost = "127.0.0.1:" + useDestination
			} else {
				targetHost = r.In.Host
				if !strings.Contains(targetHost, ":") {
					srv := r.In.Context().Value(http.ServerContextKey).(*http.Server)
					targetHost = targetHost + srv.Addr
				}
				targetHost = strings.Replace(targetHost, p.c.ListenAddress, ":"+useDestination, 1)
			}
			remoteUrl, _ := url.Parse("http://" + targetHost)
			remoteUrl.JoinPath(r.In.URL.Path)
			r.SetURL(remoteUrl)
		}

		// copy inbound first (from SetXForwarded docs)
		r.Out.Header["X-Forwarded-For"] = r.In.Header["X-Forwarded-For"]
		r.SetXForwarded()
	}
}

// getDestination looks up host in hostMap: as a full hostname, then for a
// tunnel host <prefix>.<tunnelName>.<server domain> as just <prefix>, then
// against each key as a regular expression.
func (p *Proxy) getDestination(host string) (string, bool) {
	baseHost := strings.Replace(host, p.c.ListenAddress, "", 1)
	useDestination, hasDestination := p.c.HostMap[baseHost]
	if !hasDestination {
		if prefix, ok := p.c.tunnelPrefix(baseHost); ok {
			if useDestination, hasDestination = p.c.HostMap[prefix]; hasDestination {
				return useDestination, true
			}
		}
	}
	if !hasDestination {
		for tryHost, tryPort := range p.c.HostMap {
			if regexp.MustCompile(tryHost).MatchString(baseHost) {
				return tryPort, true
			}
		}
	}
	return useDestination, hasDestination
}
