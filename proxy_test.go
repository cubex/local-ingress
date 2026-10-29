package main

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/packaged/logger/v2"
)

func TestPreserveHost(t *testing.T) {
	logs = logger.Instance()
	var gotHost, gotForwardedHost string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHost, gotForwardedHost = r.Host, r.Header.Get("X-Forwarded-Host")
	}))
	defer upstream.Close()
	upstreamHost := upstream.Listener.Addr().String()
	_, upstreamPort, _ := net.SplitHostPort(upstreamHost)

	tests := []struct {
		name, inbound, key, destination string
	}{
		{"tunnel port", "tether.tk.cubex.cloud", "tether", upstreamPort},
		{"url", "live.cubex-local.com", "live.cubex-local.com", upstream.URL},
	}
	for _, tt := range tests {
		p := NewProxy(&Config{TunnelName: "tk", HostMap: map[string]string{tt.key: tt.destination}})
		req := httptest.NewRequest(http.MethodGet, "http://"+tt.inbound+"/v1/register", nil)
		gotHost, gotForwardedHost = "", ""

		p.P.ServeHTTP(httptest.NewRecorder(), req)

		if gotHost != tt.inbound {
			t.Errorf("%s: upstream Host = %q, want %q", tt.name, gotHost, tt.inbound)
		}
		if gotForwardedHost != tt.inbound {
			t.Errorf("%s: X-Forwarded-Host = %q, want %q", tt.name, gotForwardedHost, tt.inbound)
		}
	}
}
