package main

import "testing"

func TestTunnelConfig(t *testing.T) {
	tests := []struct {
		tunnel, tunnelName string
		wantAddr, wantName string
	}{
		{"", "", defaultTunnel, ""},
		{"", "tk", defaultTunnel, "tk"},
		{"localhost:2222", "tk", "localhost:2222", "tk"},
		{"root@cubex.cloud:2222:6969", "", "cubex.cloud:2222", "tk"},
		{"root@cubex.cloud:2222:6969", "other", "cubex.cloud:2222", "other"},
		{"root@cubex.cloud:2222:4444", "", "cubex.cloud:2222", ""},
	}
	for _, tt := range tests {
		c := &Config{Tunnel: tt.tunnel, TunnelName: tt.tunnelName}
		if got := c.tunnelAddress(); got != tt.wantAddr {
			t.Errorf("%q/%q: address = %q, want %q", tt.tunnel, tt.tunnelName, got, tt.wantAddr)
		}
		if got := c.tunnelName(); got != tt.wantName {
			t.Errorf("%q/%q: name = %q, want %q", tt.tunnel, tt.tunnelName, got, tt.wantName)
		}
	}
}

func TestTunnelPrefix(t *testing.T) {
	c := &Config{Tunnel: "next.cubex.cloud:2222", TunnelName: "tk"}
	tests := map[string]string{
		"demo.tk.next.cubex.cloud":        "demo",
		"demo.tk.next.cubex.cloud:443":    "demo",
		"api.ch.demo.tk.next.cubex.cloud": "api.ch.demo",
	}
	for host, want := range tests {
		if got, ok := c.tunnelPrefix(host); !ok || got != want {
			t.Errorf("%s: prefix = %q, %v; want %q", host, got, ok, want)
		}
	}
	for _, host := range []string{"demo.jh.next.cubex.cloud", "demo.tk.cubex.cloud", "tk.next.cubex.cloud", "demo.cubex-local.com"} {
		if got, ok := c.tunnelPrefix(host); ok {
			t.Errorf("%s: matched prefix %q", host, got)
		}
	}

	p := &Proxy{c: &Config{Tunnel: "next.cubex.cloud:2222", TunnelName: "tk", HostMap: map[string]string{
		"demo":                     "http://frontend",
		"full.tk.next.cubex.cloud": "8080",
	}}}
	if d, ok := p.getDestination("demo.tk.next.cubex.cloud"); !ok || d != "http://frontend" {
		t.Errorf("prefix lookup = %q, %v", d, ok)
	}
	if d, ok := p.getDestination("full.tk.next.cubex.cloud"); !ok || d != "8080" {
		t.Errorf("full hostname lookup = %q, %v", d, ok)
	}
}
