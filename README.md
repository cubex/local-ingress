# Local Ingress

A reverse proxy for local development. It routes each request by hostname to a local port or URL,
and can publish itself on the internet through the [cubex.cloud tunnel
server](https://github.com/lucidcube/local-ingress-server), so webhooks and other people can reach
services running on your machine.

```
curl -s https://raw.githubusercontent.com/cubex/local-ingress/master/install.sh | bash
```

This downloads `config.yaml` and `update.sh` into the current directory, then the latest binary.
Run `./update.sh` to update, and `./local-ingress` (or `-c path/to/config.yaml`) to start.

## How it works

Requests arrive on `listenAddress` and are matched against `hostMap` by their `Host` header.

With `tunnelName: tk`, the client also connects to the tunnel server over SSH and publishes itself
as `https://tk.cubex.cloud` and `https://*.tk.cubex.cloud`. Requests to those hosts come back
through the SSH connection to this proxy, which routes them like any other. The server terminates
TLS with wildcard certificates.

The client signs in with an SSH key if the server accepts it, otherwise with Google: your own ID
token plus one for a service account that only `gcp-developers@` and `gcp-testers@` can
impersonate. Each person owns one name; the first to use a name claims it, and choosing a new name
releases the old one.

## Configuration

```yaml
listenAddress: ":8880"
tunnelName: tk
hostMap:
  demo: "8888"                            # demo.tk.cubex.cloud -> 127.0.0.1:8888
  api: "http://api.chive:8822"            # proxy to a URL
  www.cubex-local.com: "8889"             # a full hostname
  "fortifi.*\\.cubex-local\\.com": "9090" # a regular expression
```

| Key | Description |
| --- | --- |
| `listenAddress` | Address to serve on, e.g. `:8880`. |
| `hostMap` | Hostname to destination. A key matches a full hostname, then the prefix of a tunnel host (`demo` for `demo.tk.cubex.cloud`), then as a regular expression. A destination is a port or a URL. |
| `gzip` | Compress responses. |
| `streaming` | Allow WebSocket and server-sent event connections, and serve tunnel connections concurrently. |
| `tls`, `certFile`, `keyFile` | Serve HTTPS locally with the given certificate. |
| `tunnelName` | Name to publish as `<name>.cubex.cloud`; leave empty to run without a tunnel. Overridden by `--name`. |
| `tunnel` | Tunnel server, default `cubex.cloud:2222`. |
| `googleCredentials` | Credential for Google sign-in: an `authorized_user` file from `gcp-dev-login` (default `~/.config/chargehive/devenv-source.json`) or an `impersonated_service_account` file such as `dev-services-cred.json`. |
| `serviceAccount` | Service account to impersonate. Defaults to the one named in an `impersonated_service_account` credential, else `dev-users@dev-services-389814`. |
| `privateKeyPath`, `privateKeyPass` | SSH key for key sign-in; otherwise ssh-agent is used if available. |
| `tunnelHostKey` | The tunnel server's SHA256 host key fingerprint, built in for cubex.cloud. Google tokens are only sent to a server presenting it. |

Flags: `-c` config path, `-n` tunnel name, `-v` verbose logging, `-d` console log format.

The config is reloaded every 10 seconds, except the tunnel settings, which are read at start.

## Releasing

Publish a GitHub release. The Release workflow attaches binaries for each platform, which
`update.sh` downloads from the latest release.
