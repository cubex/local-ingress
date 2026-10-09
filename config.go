package main

import (
	"errors"
	"net"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v2"
)

type Config struct {
	HostMap       map[string]string `yaml:"hostMap"`
	ListenAddress string            `yaml:"listenAddress"`
	GZip          bool              `yaml:"gzip"`
	Tls           bool              `yaml:"tls"`
	TlsCertFile   string            `yaml:"certFile"`
	TlsKeyFile    string            `yaml:"keyFile"`
	Tunnel        string            `yaml:"tunnel"`
	TunnelName    string            `yaml:"tunnelName"`
	// TunnelHostKey is the tunnel server's SHA256 host key fingerprint;
	// Google tokens are only sent to a server presenting it.
	TunnelHostKey  string `yaml:"tunnelHostKey"`
	PrivateKeyPath string `yaml:"privateKeyPath"`
	PrivateKeyPass string `yaml:"privateKeyPass"`
	// GoogleCredentials is the credential file used to sign in to the tunnel
	// server: authorized_user (from gcp-dev-login) or impersonated_service_account
	// (dev-services-cred.json, from gcp-dev-cred).
	GoogleCredentials string `yaml:"googleCredentials"`
	// ServiceAccount must be impersonable by the user; the tunnel server
	// only admits users who can mint ID tokens for it. Defaults to the
	// account named in an impersonated_service_account credential.
	ServiceAccount string `yaml:"serviceAccount"`
	Streaming      bool   `yaml:"streaming"`
	file           string
}

func (c *Config) reload() error {
	contents, err := os.ReadFile(c.file)
	if err != nil {
		return err
	}

	err = yaml.Unmarshal(contents, c)
	if err == nil {
		if c.Tls {
			c.TlsCertFile = resolvePath(c.TlsCertFile, c.file)
			c.TlsKeyFile = resolvePath(c.TlsKeyFile, c.file)
		}
	}

	return err
}

const (
	defaultTunnel            = "cubex.cloud:2222"
	cubexCloudHostKey        = "SHA256:y9KiijJT2OuLsFRPmr2sEiBmwfwRXzMoeyG5xRTcBL8"
	defaultGoogleCredentials = "~/.config/chargehive/devenv-source.json"
	defaultServiceAccount    = "dev-users@dev-services-389814.iam.gserviceaccount.com"
)

// legacyTunnelNames maps the publish ports of the old tunnel format,
// <user>@<host>:<sshPort>:<publishPort>, to the names those ports were served as.
// TODO: remove once everyone has moved to tunnelName.
var legacyTunnelNames = map[string]string{
	"6969": "tk", "1985": "bb", "1234": "jh", "1337": "je", "1987": "ou",
	"8008": "ke", "1990": "sb", "10001": "ms", "1992": "ch", "60111": "aj",
	"1066": "am", "5232": "fl", "5346": "ag",
}

type legacyTunnel struct {
	user, address, port, name string
}

// legacyTunnel parses an old-format tunnel, <user>@<host>:<sshPort>:<publishPort>.
func (c *Config) legacyTunnel() (legacyTunnel, bool) {
	user, hostPorts, hasUser := strings.Cut(c.Tunnel, "@")
	i := strings.LastIndex(hostPorts, ":")
	if !hasUser || i < 0 {
		return legacyTunnel{}, false
	}
	port := hostPorts[i+1:]
	return legacyTunnel{user: user, address: hostPorts[:i], port: port, name: legacyTunnelNames[port]}, true
}

// tunnelName is the name to publish under, or "" when tunnelling is off.
func (c *Config) tunnelName() string {
	if *nameFlag != "" {
		return *nameFlag
	}
	if c.TunnelName != "" {
		return c.TunnelName
	}
	legacy, _ := c.legacyTunnel()
	return legacy.name
}

func (c *Config) tunnelAddress() string {
	if legacy, ok := c.legacyTunnel(); ok {
		return legacy.address
	}
	return withDefault(c.Tunnel, defaultTunnel)
}

// tunnelPrefix returns <prefix> for a request to <prefix>.<tunnelName>.<server domain>.
func (c *Config) tunnelPrefix(host string) (string, bool) {
	name := c.tunnelName()
	if name == "" {
		return "", false
	}
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	domain, _, err := net.SplitHostPort(c.tunnelAddress())
	if err != nil {
		return "", false
	}
	return strings.CutSuffix(host, "."+name+"."+domain)
}

// tunnelHostKey is the configured fingerprint, or the cubex.cloud server's
// for cubex.cloud hosts.
func (c *Config) tunnelHostKey(host string) string {
	if c.TunnelHostKey != "" {
		return c.TunnelHostKey
	}
	if host == "cubex.cloud" || strings.HasSuffix(host, ".cubex.cloud") {
		return cubexCloudHostKey
	}
	return ""
}

func (c *Config) googleCredentials() string {
	return expandHome(withDefault(c.GoogleCredentials, defaultGoogleCredentials))
}

func withDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func expandHome(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return path.Join(home, p[1:])
		}
	}
	return p
}

func LoadConfig(configFile string) (*Config, error) {
	cfg := &Config{file: configFile}
	err := cfg.reload()

	go func(c *Config) {
		for {
			err := c.reload()
			logs.FatalIf(err, "reloading content")
			time.Sleep(time.Second * 10)
		}
	}(cfg)

	return cfg, err
}

func resolvePath(checkPath string, configPath string) string {
	if checkPath == "~" || strings.HasPrefix(checkPath, "~/") {
		usr, _ := user.Current()
		checkPath = path.Join(usr.HomeDir, checkPath[1:])
	}

	if !filepath.IsAbs(checkPath) {
		checkPath, _ = filepath.Abs(path.Join(filepath.Dir(configPath), checkPath))
	}

	if _, err := os.Stat(checkPath); errors.Is(err, os.ErrNotExist) {
		logs.Fatal(checkPath + ": file does not exist")
	}

	return checkPath
}
