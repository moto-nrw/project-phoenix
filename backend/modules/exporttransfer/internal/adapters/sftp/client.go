package sftp

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net"
	"net/netip"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// SFTP client for the manual export transfer (#3050).
//
// moto is a client only; it runs no SFTP server. The package deliberately
// exposes ONE operation — put this byte slice under this name into the
// configured directory. There is no listing, no download, no delete, no
// rename of somebody else's file, because none of those are needed and each
// would be a new way to misuse a school's credentials.

// Failure reasons. They are returned as sentinels so the caller can map each
// to one German sentence and to an audit entry, WITHOUT ever putting the
// underlying transport text (which can carry hostnames, paths, or a server's
// own error prose) in front of a school user.
var (
	// ErrHostKeyMismatch means the counterpart presented a different key than
	// the configured fingerprint. This is the one error that must never be
	// retried away: it is either a changed server or the wrong server.
	ErrHostKeyMismatch = reason{"host key does not match the configured fingerprint", "host_key_mismatch"}
	// ErrAuthFailed covers rejected credentials.
	ErrAuthFailed = reason{"authentication rejected", "authentication_rejected"}
	// ErrConnect covers everything that stopped the connection from opening.
	ErrConnect = reason{"could not connect", "connection_failed"}
	// ErrUpload covers a connection that opened but a file that did not land.
	ErrUpload = reason{"upload failed", "upload_failed"}
	// ErrFileTooLarge means the export exceeded the configured size cap.
	ErrFileTooLarge = reason{"file exceeds the transfer size limit", "file_too_large"}
	// ErrInvalidFilename marks a filename that must never reach a remote path.
	ErrInvalidFilename = reason{"invalid filename", "upload_failed"}
)

// reason is a sentinel that also names WHY the transfer failed, as a stable
// code. Callers read the code through a TransferReason() interface instead of
// importing these sentinels, so the transport stays replaceable — see
// services/active.ReasonedTransferError.
//
// The codes are a contract with audit.export_transfers; they are values, not
// prose, and must not be reworded.
type reason struct {
	text string
	code string
}

func (r reason) Error() string          { return r.text }
func (r reason) TransferReason() string { return r.code }

// Defaults chosen for a payroll file: a DATEV month file is measured in
// kilobytes, so 64 MiB is far above any legitimate export and still small
// enough that a wrong target cannot be used to push bulk data anywhere.
const (
	DefaultTimeout  = 60 * time.Second
	DefaultMaxBytes = 64 << 20
)

// Target is a validated counterpart. The caller (the transfer workflow)
// resolves it from the tenant settings; this package never reads settings.
type Target struct {
	Host               string
	Port               int
	Username           string
	Password           string
	RemoteDirectory    string
	HostKeyFingerprint string
}

// Uploader transfers one file to one counterpart.
type Uploader interface {
	Upload(ctx context.Context, target Target, filename string, data []byte) error
}

// DialFunc opens the TCP connection. Injected so tests can reach an
// in-process server without weakening the address policy.
type DialFunc func(ctx context.Context, network, address string) (net.Conn, error)

// Client is the production Uploader.
type Client struct {
	dial     DialFunc
	resolve  Resolver
	policy   AddressPolicy
	timeout  time.Duration
	maxBytes int64
}

// Option configures a Client.
type Option func(*Client)

// WithAddressPolicy replaces the outbound address policy.
//
// Production must not call this: the public-only policy IS the SSRF guard.
// It exists because the integration test has to reach a loopback server, and
// a guard that a test can only satisfy by being switched off globally is a
// guard that eventually gets switched off globally.
func WithAddressPolicy(policy AddressPolicy) Option { return func(c *Client) { c.policy = policy } }

// WithTimeout bounds the whole transfer.
func WithTimeout(timeout time.Duration) Option { return func(c *Client) { c.timeout = timeout } }

// WithMaxBytes caps the transferred size.
func WithMaxBytes(maxBytes int64) Option { return func(c *Client) { c.maxBytes = maxBytes } }

// New builds a Client with the production defaults: public destinations only,
// system resolver, plain TCP.
func New(opts ...Option) *Client {
	client := &Client{
		dial:     (&net.Dialer{}).DialContext,
		resolve:  DefaultResolver,
		policy:   PublicOnlyPolicy{},
		timeout:  DefaultTimeout,
		maxBytes: DefaultMaxBytes,
	}
	for _, opt := range opts {
		opt(client)
	}
	return client
}

// ValidateFilename rejects anything that could steer the remote path.
//
// The filename comes from the export service, not from the browser, so this
// is a second line rather than the first. It stays because the remote path is
// assembled by string concatenation, and a name containing a slash or a ".."
// segment would silently write outside the agreed directory.
func ValidateFilename(filename string) error {
	if filename == "" || len(filename) > 255 {
		return fmt.Errorf("%w: length", ErrInvalidFilename)
	}
	if filename == "." || filename == ".." {
		return fmt.Errorf("%w: %q", ErrInvalidFilename, filename)
	}
	if strings.ContainsAny(filename, `/\`) {
		return fmt.Errorf("%w: contains a path separator", ErrInvalidFilename)
	}
	for _, r := range filename {
		if r < 0x20 || r == 0x7f {
			return fmt.Errorf("%w: contains a control character", ErrInvalidFilename)
		}
	}
	return nil
}

// hostKeyCallback pins the counterpart to the configured fingerprint.
//
// There is no fallback branch on purpose. ssh.InsecureIgnoreHostKey and
// trust-on-first-use both turn "we know who receives the payroll data" into
// "somebody received the payroll data".
//
// The comparison is an ordinary one: both sides are public values — a hash of
// a public host key against a fingerprint the school was told to publish — so
// there is no secret whose length or content a timing difference could leak.
// A counterpart usually offers SEVERAL host keys (RSA, ECDSA, ed25519) and the
// negotiated one decides which fingerprint is presented — x/crypto/ssh prefers
// RSA over ed25519, so the key an admin looked up with `ssh-keyscan -t ed25519`
// is often not the key that arrives here. The mismatch therefore names the
// presented fingerprint and its type. That text reaches the LOG only: the
// school reads a fixed sentence, because showing the offered fingerprint in
// the UI would invite pasting whatever answered — which is precisely the
// trust-on-first-use hole the pinning exists to close.
func hostKeyCallback(expected string) ssh.HostKeyCallback {
	return func(_ string, _ net.Addr, key ssh.PublicKey) error {
		actual := ssh.FingerprintSHA256(key)
		if actual != expected {
			return fmt.Errorf("%w: counterpart presented %s (%s), configured %s",
				ErrHostKeyMismatch, actual, key.Type(), expected)
		}
		return nil
	}
}

// tempName is the name the file carries WHILE it is being written. A reader
// on the far side must never find a partial file under the final name, so the
// upload writes here and renames only after a successful close.
//
// The suffix only has to keep two concurrent transfers of the same export
// apart inside a directory this tenant writes to, so an ordinary random
// source is enough; nothing here is a secret or a security boundary.
func tempName(final string) string {
	return fmt.Sprintf(".%s.part-%016x", final, rand.Uint64())
}

func (c *Client) Upload(ctx context.Context, target Target, filename string, data []byte) error {
	if err := ValidateFilename(filename); err != nil {
		return err
	}
	if int64(len(data)) > c.maxBytes {
		return fmt.Errorf("%w: %d bytes", ErrFileTooLarge, len(data))
	}

	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	addr, err := resolveAllowedAddr(ctx, c.resolve, c.policy, target.Host)
	if err != nil {
		if errors.Is(err, ErrAddressNotAllowed) {
			return err
		}
		return fmt.Errorf("%w: %w", ErrConnect, err)
	}

	conn, err := c.dial(ctx, "tcp", net.JoinHostPort(addr.String(), strconv.Itoa(target.Port)))
	if err != nil {
		return fmt.Errorf("%w: %w", ErrConnect, err)
	}
	defer func() { _ = conn.Close() }()

	// Cancellation is enforced by closing the connection, not by an absolute
	// deadline on it. A deadline would still be armed while the session is
	// being torn down, and the SFTP client's receive loop would then sit in a
	// read until it expired instead of returning at once.
	stop := make(chan struct{})
	defer close(stop)
	go func() {
		select {
		case <-ctx.Done():
			_ = conn.Close()
		case <-stop:
		}
	}()

	// The handshake itself does blocking I/O before any of our code runs
	// again, so it gets its own deadline.
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}

	sshClient, client, err := c.newSFTPClient(conn, addr, target)
	if err != nil {
		return err
	}
	// Tear the SSH transport down FIRST. sftp.Client.Close() waits for its
	// receive loop, which only ends when the counterpart closes the channel —
	// a server that never does would otherwise hold the request open until
	// the context expires, long after the file already arrived.
	defer func() {
		_ = sshClient.Close()
		_ = client.Close()
	}()

	// Handshake done: hand the timing back to the context watchdog.
	_ = conn.SetDeadline(time.Time{})

	return c.writeAtomically(client, target.RemoteDirectory, filename, data)
}

// newSFTPClient completes the SSH handshake and opens the SFTP subsystem. It
// returns both clients so the caller can close the SSH connection too —
// closing only the SFTP client leaves the SSH session's goroutines running.
func (c *Client) newSFTPClient(conn net.Conn, addr netip.Addr, target Target) (*ssh.Client, *sftp.Client, error) {
	config := &ssh.ClientConfig{
		User:            target.Username,
		Auth:            []ssh.AuthMethod{ssh.Password(target.Password)},
		HostKeyCallback: hostKeyCallback(target.HostKeyFingerprint),
		Timeout:         c.timeout,
	}

	address := net.JoinHostPort(addr.String(), strconv.Itoa(target.Port))
	sshConn, chans, reqs, err := ssh.NewClientConn(conn, address, config)
	if err != nil {
		// The host key verdict must survive as a distinct reason: it is the
		// difference between "the counterpart said no" and "this is not the
		// counterpart".
		if errors.Is(err, ErrHostKeyMismatch) {
			// Pass the wrapped error on: it names the fingerprint the
			// counterpart actually presented, which is the one thing that
			// makes this failure diagnosable from the log.
			return nil, nil, err
		}
		if isAuthFailure(err) {
			return nil, nil, ErrAuthFailed
		}
		return nil, nil, fmt.Errorf("%w: %w", ErrConnect, err)
	}

	sshClient := ssh.NewClient(sshConn, chans, reqs)
	client, err := sftp.NewClient(sshClient)
	if err != nil {
		_ = sshClient.Close()
		return nil, nil, fmt.Errorf("%w: %w", ErrConnect, err)
	}
	return sshClient, client, nil
}

// isAuthFailure recognizes a rejected login. x/crypto/ssh reports it as a
// plain error string, so this is a text match — kept narrow and used only to
// choose which German sentence the school reads.
func isAuthFailure(err error) bool {
	return strings.Contains(err.Error(), "unable to authenticate")
}

// writeAtomically uploads to a temporary name and renames on success, so the
// final name never appears until the complete file is there.
func (c *Client) writeAtomically(client *sftp.Client, dir, filename string, data []byte) error {
	tempPath := path.Join(dir, tempName(filename))
	finalPath := path.Join(dir, filename)

	if err := c.writeFile(client, tempPath, data); err != nil {
		// Leave nothing behind. A failed removal is not reported: the
		// transfer already failed, and the leftover is a temporary name that
		// no consumer reads.
		_ = client.Remove(tempPath)
		return err
	}

	// Some servers refuse a rename onto an existing name. Removing the
	// previous file first makes re-transferring the same month possible,
	// which is the normal case after a correction.
	if _, statErr := client.Stat(finalPath); statErr == nil {
		_ = client.Remove(finalPath)
	}
	if err := client.Rename(tempPath, finalPath); err != nil {
		_ = client.Remove(tempPath)
		return fmt.Errorf("%w: rename: %w", ErrUpload, err)
	}
	return nil
}

func (c *Client) writeFile(client *sftp.Client, remotePath string, data []byte) error {
	file, err := client.Create(remotePath)
	if err != nil {
		return fmt.Errorf("%w: create: %w", ErrUpload, err)
	}
	written, err := io.Copy(file, bytes.NewReader(data))
	if err != nil {
		_ = file.Close()
		return fmt.Errorf("%w: write: %w", ErrUpload, err)
	}
	// Close is where a buffered write surfaces its error, so it is checked
	// rather than deferred: a silent close would report a truncated file as a
	// successful transfer.
	if err := file.Close(); err != nil {
		return fmt.Errorf("%w: close: %w", ErrUpload, err)
	}
	if written != int64(len(data)) {
		return fmt.Errorf("%w: wrote %d of %d bytes", ErrUpload, written, len(data))
	}
	return nil
}
