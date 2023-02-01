package main

import (
	"fmt"
	"io"
	"net"
	"os"
	"strings"

	"go.uber.org/zap"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

func startSshTunnel(c *Config) {
	if c.Tunnel == "" {
		return
	}

	tunnelSplit := strings.Split(c.Tunnel, "@")
	username, tunnel := tunnelSplit[0], tunnelSplit[1]

	tunnelSplit = strings.Split(tunnel, ":")
	tunnel, publishPort := strings.Join(tunnelSplit[:len(tunnelSplit)-1], ":"), tunnelSplit[len(tunnelSplit)-1]

	// ssh-agent(1) provides a UNIX socket at $SSH_AUTH_SOCK.
	agentConnection, err := net.Dial("unix", os.Getenv("SSH_AUTH_SOCK"))
	logs.FatalIf(err, "opening SSH_AUTH_SOCK")

	sshConfig := &ssh.ClientConfig{
		User:            username,
		Auth:            []ssh.AuthMethod{ssh.PublicKeysCallback(agent.NewClient(agentConnection).Signers)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	sshClient, err := ssh.Dial("tcp", tunnel, sshConfig)
	logs.FatalIf(err, "dialing ssh server")
	defer func() { _ = sshClient.Close() }()

	// Listen on remote server port
	listener, err := sshClient.Listen("tcp", fmt.Sprintf("0.0.0.0:%s", publishPort))
	logs.FatalIf(err, "opening port on remote server")
	defer func() { _ = listener.Close() }()

	logs.Info("listening on remote server", zap.String("host", listener.Addr().String()))

	var local net.Conn
	localConnected := false
	for {
		remote, err := listener.Accept()
		logs.FatalIf(err, "error accepting connection")

		if localConnected == false {
			local, err = net.Dial("tcp", c.ListenAddress)
			logs.FatalIf(err, "dialing local service")
			localConnected = true
		}

		if err = handleClient(local, remote); err != nil {
			_ = local.Close()
			localConnected = false
			logs.ErrorIf(err, "dialing local service")
		}

		_ = remote.Close()
	}
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
