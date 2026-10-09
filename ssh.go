package main

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"regexp"
	"time"

	"go.uber.org/zap"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// refreshRequest carries fresh tokens before the current ones expire; the
// server disconnects clients whose tokens lapse.
const refreshRequest = "refresh-tokens@cubex.cloud"

var tunnelNamePattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`)

// startSshTunnel signs in with an SSH key if one is configured and accepted,
// otherwise with Google tokens, and publishes this proxy as <name>.<host>.
//
// TODO: remove the legacy path once the old OpenSSH server is retired. A
// legacy config (<user>@<host>:<sshPort>:<publishPort>) may reach that server,
// whose host key is not pinned; it is only offered key auth and gets a TCP
// port forward rather than a named tunnel.
func startSshTunnel(c *Config) {
	name := c.tunnelName()
	legacy, isLegacy := c.legacyTunnel()
	if isLegacy {
		if name == "" {
			logs.Fatal("tunnel " + c.Tunnel + " uses the old format; replace it with tunnelName: <name>")
		}
		logs.Warn("tunnel uses the old format; replace it with `tunnelName: " + name + "`")
	}
	if name == "" {
		return
	}
	if !tunnelNamePattern.MatchString(name) {
		logs.Fatal("tunnel name must be lowercase letters, digits and hyphens: " + name)
	}

	address := c.tunnelAddress()
	host, _, err := net.SplitHostPort(address)
	logs.FatalIf(err, "parsing tunnel address")
	expectedHostKey := c.tunnelHostKey(host)

	ctx := context.Background()
	verified := false
	email := ""

	user := "tunnel"
	if isLegacy {
		user = legacy.user
	}
	var auth []ssh.AuthMethod
	if keyAuth := sshKeyAuth(c); keyAuth != nil {
		auth = append(auth, keyAuth)
	}
	// Last, because an error from this callback ends authentication.
	auth = append(auth, ssh.PasswordCallback(func() (string, error) {
		if !verified {
			return "", errors.New("not sending Google tokens to a server whose host key is not pinned; set tunnelHostKey")
		}
		password, e, err := tunnelCredentials(ctx, c, host)
		email = e
		return password, err
	}))

	sshClient, err := ssh.Dial("tcp", address, &ssh.ClientConfig{
		User: user,
		Auth: auth,
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			fp := ssh.FingerprintSHA256(key)
			switch {
			case expectedHostKey != "" && fp == expectedHostKey:
				verified = true
				return nil
			case isLegacy:
				return nil
			default:
				return fmt.Errorf("host key %s for %s does not match tunnelHostKey %q", fp, hostname, expectedHostKey)
			}
		},
	})
	logs.FatalIf(err, "connecting to tunnel server")
	defer func() { _ = sshClient.Close() }()

	var listener net.Listener
	if verified {
		listener, err = sshClient.ListenUnix(name)
		if err != nil {
			logs.Fatal("tunnel name " + name + " was refused: it belongs to someone else")
		}
	} else {
		listener, err = sshClient.Listen("tcp", "0.0.0.0:"+legacy.port)
		logs.FatalIf(err, "opening port on remote server")
	}
	defer func() { _ = listener.Close() }()

	logs.Info("tunnel open", zap.String("url", "https://"+name+"."+host), zap.String("email", email))

	if email != "" {
		go func() {
			for range time.Tick(45 * time.Minute) {
				password, _, err := tunnelCredentials(ctx, c, host)
				if err == nil {
					payload := ssh.Marshal(&struct{ Credentials string }{password})
					var ok bool
					if ok, _, err = sshClient.SendRequest(refreshRequest, true, payload); err == nil && !ok {
						err = errors.New("server refused the new tokens")
					}
				}
				logs.ErrorIf(err, "refreshing tunnel tokens")
			}
		}()
	}

	for {
		remote, err := listener.Accept()
		logs.FatalIf(err, "error accepting connection")

		local, err := net.Dial("tcp", c.ListenAddress)
		logs.FatalIf(err, "dialing local service")

		if c.Streaming {
			go func() {
				err := handleClient(local, remote)
				logs.ErrorIf(err, "handling transport")
			}()
		} else {
			err = handleClient(local, remote)
			logs.ErrorIf(err, "handling transport")
		}
	}
}

// sshKeyAuth returns key authentication from privateKeyPath or ssh-agent, or
// nil when neither is available.
func sshKeyAuth(c *Config) ssh.AuthMethod {
	if c.PrivateKeyPath != "" {
		auth, err := withKey(expandHome(c.PrivateKeyPath), c.PrivateKeyPass)
		logs.FatalIf(err, "loading private key failed")
		return auth
	}
	if sock := os.Getenv("SSH_AUTH_SOCK"); sock != "" {
		// ssh-agent(1) provides a UNIX socket at $SSH_AUTH_SOCK.
		agentConnection, err := net.Dial("unix", sock)
		if err != nil {
			logs.ErrorIf(err, "opening SSH_AUTH_SOCK")
			return nil
		}
		return ssh.PublicKeysCallback(agent.NewClient(agentConnection).Signers)
	}
	return nil
}

func withKey(privateKeyPath, privateKeyPassword string) (ssh.AuthMethod, error) {
	// read private key file
	pemBytes, err := os.ReadFile(privateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("Reading private key file failed %v", err)
	}
	// create signer
	signer, err := signerFromPem(pemBytes, []byte(privateKeyPassword))
	if err != nil {
		return nil, err
	}

	return ssh.PublicKeys(signer), nil
}

// Handle local client connections and tunnel data to the remote server
// Will use io.Copy - http://golang.org/pkg/io/#Copy
func handleClient(local net.Conn, remote net.Conn) error {
	chDone := make(chan error, 2)

	// Start remote -> local data transfer
	go func() {
		_, err := io.Copy(local, remote)
		chDone <- err
	}()

	// Start local -> remote data transfer
	go func() {
		_, err := io.Copy(remote, local)
		chDone <- err
	}()

	// Wait for the first copy to finish, then close both connections
	// to unblock the other goroutine
	err := <-chDone
	_ = local.Close()
	_ = remote.Close()

	// Wait for the second goroutine to finish
	<-chDone

	return err
}

func signerFromPem(pemBytes []byte, password []byte) (ssh.Signer, error) {

	// read pem block
	err := errors.New("Pem decode failed, no key found")
	pemBlock, _ := pem.Decode(pemBytes)
	if pemBlock == nil {
		return nil, err
	}

	// handle encrypted key
	if x509.IsEncryptedPEMBlock(pemBlock) {
		// decrypt PEM
		pemBlock.Bytes, err = x509.DecryptPEMBlock(pemBlock, []byte(password))
		if err != nil {
			return nil, fmt.Errorf("Decrypting PEM block failed %v", err)
		}

		// get RSA, EC or DSA key
		key, err := parsePemBlock(pemBlock)
		if err != nil {
			return nil, err
		}

		// generate signer instance from key
		signer, err := ssh.NewSignerFromKey(key)
		if err != nil {
			return nil, fmt.Errorf("Creating signer from encrypted key failed %v", err)
		}

		return signer, nil
	} else {
		// generate signer instance from plain key
		signer, err := ssh.ParsePrivateKey(pemBytes)
		if err != nil {
			return nil, fmt.Errorf("Parsing plain private key failed %v", err)
		}

		return signer, nil
	}
}

func parsePemBlock(block *pem.Block) (interface{}, error) {
	switch block.Type {
	case "RSA PRIVATE KEY":
		key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("Parsing PKCS private key failed %v", err)
		} else {
			return key, nil
		}
	case "EC PRIVATE KEY":
		key, err := x509.ParseECPrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("Parsing EC private key failed %v", err)
		} else {
			return key, nil
		}
	case "DSA PRIVATE KEY":
		key, err := ssh.ParseDSAPrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("Parsing DSA private key failed %v", err)
		} else {
			return key, nil
		}
	default:
		return nil, fmt.Errorf("Parsing private key failed, unsupported key type %q", block.Type)
	}
}
