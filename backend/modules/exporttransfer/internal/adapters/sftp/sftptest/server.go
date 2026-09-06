// Package sftptest runs an isolated in-process SFTP server for tests (#3050).
//
// It listens on a loopback port and serves the real filesystem, so a test can
// point the client at a t.TempDir() and then look at the files that actually
// arrived. No Docker, no fixture server, no network.
//
// It lives in its own package so both the transport's own end-to-end tests and
// the capability's integration test drive the SAME server. A second copy would
// drift, and the whole value of these tests is that the far side behaves like a
// real one — including closing the channel when the subsystem ends, which is
// what lets a well-behaved client shut down without waiting for a timeout.
package sftptest

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"net"
	"net/netip"
	"strconv"
	"testing"

	pkgsftp "github.com/pkg/sftp"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

// Credentials the server accepts.
const (
	User     = "lohn-export"
	Password = "s3hr-geheim"
)

// Server describes a running test counterpart.
type Server struct {
	Host string
	Port int
	// Fingerprint is the SHA256 host-key fingerprint, in exactly the form a
	// school pastes into the settings.
	Fingerprint string
}

// Address is the dial target as "host:port".
func (s Server) Address() string {
	return net.JoinHostPort(s.Host, strconv.Itoa(s.Port))
}

var errAuthRejected = errors.New("password rejected")

// Start boots the server and stops it when the test ends.
func Start(t *testing.T) Server {
	t.Helper()

	_, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	signer, err := ssh.NewSignerFromKey(priv)
	require.NoError(t, err)

	config := &ssh.ServerConfig{
		PasswordCallback: func(conn ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
			if conn.User() == User && string(password) == Password {
				return nil, nil
			}
			return nil, errAuthRejected
		},
	}
	config.AddHostKey(signer)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })

	go acceptLoop(listener, config)

	addr, ok := listener.Addr().(*net.TCPAddr)
	require.True(t, ok)
	return Server{
		Host:        "127.0.0.1",
		Port:        addr.Port,
		Fingerprint: ssh.FingerprintSHA256(signer.PublicKey()),
	}
}

// AllowLoopbackPolicy is the ONLY place loopback is permitted. Production
// builds the client without it, so the public-only policy stays in force
// there; see sftp.WithAddressPolicy.
type AllowLoopbackPolicy struct{}

func (AllowLoopbackPolicy) Allow(addr netip.Addr) bool { return addr.IsLoopback() }

func acceptLoop(listener net.Listener, config *ssh.ServerConfig) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			return // listener closed by cleanup
		}
		go serveConn(conn, config)
	}
}

func serveConn(conn net.Conn, config *ssh.ServerConfig) {
	defer func() { _ = conn.Close() }()

	sshConn, chans, reqs, err := ssh.NewServerConn(conn, config)
	if err != nil {
		return // failed handshake: wrong host key expectation or bad password
	}
	defer func() { _ = sshConn.Close() }()
	go ssh.DiscardRequests(reqs)

	for newChannel := range chans {
		if newChannel.ChannelType() != "session" {
			_ = newChannel.Reject(ssh.UnknownChannelType, "only sessions")
			continue
		}
		channel, requests, err := newChannel.Accept()
		if err != nil {
			return
		}
		go answerSubsystemRequests(requests)

		server, err := pkgsftp.NewServer(channel)
		if err != nil {
			return
		}
		// Closing the channel once the subsystem ends is what a real SFTP
		// server does, and the client relies on it: without that EOF its
		// receive loop keeps waiting and Close() blocks.
		go func() {
			_ = server.Serve()
			_ = server.Close()
			_ = channel.Close()
		}()
	}
}

// answerSubsystemRequests accepts exactly the "sftp" subsystem. The payload is
// an SSH string: a 4-byte length followed by the name.
func answerSubsystemRequests(requests <-chan *ssh.Request) {
	for req := range requests {
		ok := req.Type == "subsystem" && len(req.Payload) > 4 && string(req.Payload[4:]) == "sftp"
		if req.WantReply {
			_ = req.Reply(ok, nil)
		}
	}
}
