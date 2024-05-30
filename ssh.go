package main

import (
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"golang.org/x/crypto/ssh/agent"
	"io"
	"net"
	"os"
	"strings"

	"go.uber.org/zap"
	"golang.org/x/crypto/ssh"
)

func startSshTunnel(c *Config) {
	if c.Tunnel == "" {
		return
	}

	tunnelSplit := strings.Split(c.Tunnel, "@")
	username, tunnel := tunnelSplit[0], tunnelSplit[1]

	tunnelSplit = strings.Split(tunnel, ":")
	tunnel, publishPort := strings.Join(tunnelSplit[:len(tunnelSplit)-1], ":"), tunnelSplit[len(tunnelSplit)-1]

	sshConfig := &ssh.ClientConfig{
		User:            username,
		Auth:            []ssh.AuthMethod{},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	if c.PrivateKeyPath != "" {
		if auth, err := withKey(c.PrivateKeyPath, c.PrivateKeyPass); err == nil {
			sshConfig.Auth = append(sshConfig.Auth, auth)
		} else {
			logs.FatalIf(err, "loading private key failed")
		}
	} else if os.Getenv("SSH_AUTH_SOCK") != "" {
		// ssh-agent(1) provides a UNIX socket at $SSH_AUTH_SOCK.
		agentConnection, err := net.Dial("unix", os.Getenv("SSH_AUTH_SOCK"))
		logs.FatalIf(err, "opening SSH_AUTH_SOCK")
		sshConfig.Auth = append(sshConfig.Auth, ssh.PublicKeysCallback(agent.NewClient(agentConnection).Signers))
	}

	sshClient, err := ssh.Dial("tcp", tunnel, sshConfig)
	logs.FatalIf(err, "dialing ssh server")
	defer func() { _ = sshClient.Close() }()

	// Listen on remote server port
	listener, err := sshClient.Listen("tcp", fmt.Sprintf("0.0.0.0:%s", publishPort))
	logs.FatalIf(err, "opening port on remote server")
	defer func() { _ = listener.Close() }()

	logs.Info("listening on remote server", zap.String("host", listener.Addr().String()))

	for {
		remote, err := listener.Accept()
		logs.FatalIf(err, "error accepting connection")

		local, err := net.Dial("tcp", c.ListenAddress)
		logs.FatalIf(err, "dialing local service")

		err = handleClient(local, remote)
		logs.ErrorIf(err, "handling transport")

		_ = remote.Close()
		_ = local.Close()
	}
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

// From https://sosedoff.com/2015/05/25/ssh-port-forwarding-with-go.html
// Handle local client connections and tunnel data to the remote server
// Will use io.Copy - http://golang.org/pkg/io/#Copy
func handleClient(local net.Conn, remote net.Conn) error {
	chDone := make(chan error)

	// Start remote -> local data transfer
	go func() {
		_, err := io.Copy(local, remote)
		logs.ErrorIf(err, "error while copy remote->local")
		chDone <- err
	}()

	// Start local -> remote data transfer
	go func() {
		_, err := io.Copy(remote, local)
		logs.ErrorIf(err, "error while copy local->remote")
		chDone <- err
	}()

	return <-chDone
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
