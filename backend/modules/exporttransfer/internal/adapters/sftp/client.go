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
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// SFTP client for the manual export transfer (#3050).
//
// moto is a client only; it runs no SFTP server. The package deliberately
// exposes ONE operation — put this byte slice under this name into the
// configured directory. Its internal reads, renames and removals are limited
// to temporary copies of that exact file so a failed audit transaction can be
// compensated; callers cannot operate on arbitrary remote paths.

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

// PendingUpload is a completed remote replacement whose previous state is
// retained until the caller's audit transaction commits or rolls back.
type PendingUpload interface {
	Commit() error
	Rollback() error
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
// New uses this to install the public-only production default. Other production
// callers must not override it: the public-only policy IS the SSRF guard. The
// option exists because the integration test has to reach a loopback server.
func WithAddressPolicy(policy AddressPolicy) Option { return func(c *Client) { c.policy = policy } }

// WithTimeout bounds the whole transfer.
func WithTimeout(timeout time.Duration) Option { return func(c *Client) { c.timeout = timeout } }

// WithMaxBytes caps the transferred size.
func WithMaxBytes(maxBytes int64) Option { return func(c *Client) { c.maxBytes = maxBytes } }

// New builds a Client with the production defaults: public destinations only,
// system resolver, plain TCP.
func New(opts ...Option) *Client {
	client := &Client{
		dial:    (&net.Dialer{}).DialContext,
		resolve: DefaultResolver,
	}
	for _, opt := range []Option{
		WithAddressPolicy(PublicOnlyPolicy{}),
		WithTimeout(DefaultTimeout),
		WithMaxBytes(DefaultMaxBytes),
	} {
		opt(client)
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
	pending, err := c.Prepare(ctx, target, filename, data)
	if err != nil {
		return err
	}
	return pending.Commit()
}

// Prepare uploads and atomically installs the new file while retaining enough
// remote state to restore the previous file if the caller's transaction fails.
func (c *Client) Prepare(ctx context.Context, target Target, filename string, data []byte) (PendingUpload, error) {
	if err := ValidateFilename(filename); err != nil {
		return nil, err
	}
	if int64(len(data)) > c.maxBytes {
		return nil, fmt.Errorf("%w: %d bytes", ErrFileTooLarge, len(data))
	}

	ctx, cancel := context.WithTimeout(ctx, c.timeout)

	addr, err := resolveAllowedAddr(ctx, c.resolve, c.policy, target.Host)
	if err != nil {
		cancel()
		if errors.Is(err, ErrAddressNotAllowed) {
			return nil, err
		}
		return nil, fmt.Errorf("%w: %w", ErrConnect, err)
	}

	conn, err := c.dial(ctx, "tcp", net.JoinHostPort(addr.String(), strconv.Itoa(target.Port)))
	if err != nil {
		cancel()
		return nil, fmt.Errorf("%w: %w", ErrConnect, err)
	}

	// Cancellation is enforced by closing the connection, not by an absolute
	// deadline on it. A deadline would still be armed while the session is
	// being torn down, and the SFTP client's receive loop would then sit in a
	// read until it expired instead of returning at once.
	stop := make(chan struct{})
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
		close(stop)
		cancel()
		_ = conn.Close()
		return nil, err
	}
	var closeOnce sync.Once
	closeConnection := func() {
		closeOnce.Do(func() {
			close(stop)
			cancel()
			// Tear the SSH transport down FIRST. sftp.Client.Close() waits for
			// its receive loop, which only ends when the counterpart closes.
			_ = sshClient.Close()
			_ = client.Close()
			_ = conn.Close()
		})
	}

	// Handshake done: hand the timing back to the context watchdog.
	_ = conn.SetDeadline(time.Time{})

	pending, err := c.writeAtomically(fileOperations(client), target.RemoteDirectory, filename, data)
	if err != nil {
		closeConnection()
		return nil, err
	}
	pending.close = closeConnection
	return pending, nil
}

type remoteFileOperations struct {
	create       func(string) (io.WriteCloser, error)
	open         func(string) (io.ReadCloser, error)
	stat         func(string) (os.FileInfo, error)
	remove       func(string) error
	rename       func(string, string) error
	posixRename  func(string, string) error
	hasExtension func(string) bool
}

func fileOperations(client *sftp.Client) remoteFileOperations {
	return remoteFileOperations{
		create:      func(name string) (io.WriteCloser, error) { return client.Create(name) },
		open:        func(name string) (io.ReadCloser, error) { return client.Open(name) },
		stat:        client.Stat,
		remove:      client.Remove,
		rename:      client.Rename,
		posixRename: client.PosixRename,
		hasExtension: func(name string) bool {
			_, ok := client.HasExtension(name)
			return ok
		},
	}
}

type pendingUpload struct {
	client     remoteFileOperations
	finalPath  string
	backupPath string
	close      func()
	once       sync.Once
	err        error
}

func (u *pendingUpload) Commit() error {
	u.once.Do(func() {
		if u.backupPath != "" {
			u.err = u.client.remove(u.backupPath)
		}
		if u.close != nil {
			u.close()
		}
	})
	return u.err
}

func (u *pendingUpload) Rollback() error {
	u.once.Do(func() {
		if u.backupPath != "" {
			u.err = u.client.posixRename(u.backupPath, u.finalPath)
		} else if err := u.client.remove(u.finalPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			u.err = err
		}
		if u.close != nil {
			u.close()
		}
	})
	return u.err
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
func (c *Client) writeAtomically(client remoteFileOperations, dir, filename string, data []byte) (*pendingUpload, error) {
	tempPath := path.Join(dir, tempName(filename))
	finalPath := path.Join(dir, filename)

	if err := c.writeFile(client, tempPath, data); err != nil {
		// Leave nothing behind. A failed removal is not reported: the
		// transfer already failed, and the leftover is a temporary name that
		// no consumer reads.
		_ = client.remove(tempPath)
		return nil, err
	}

	_, statErr := client.stat(finalPath)
	if errors.Is(statErr, os.ErrNotExist) {
		if err := client.rename(tempPath, finalPath); err != nil {
			_ = client.remove(tempPath)
			return nil, fmt.Errorf("%w: rename: %w", ErrUpload, err)
		}
		return &pendingUpload{client: client, finalPath: finalPath}, nil
	}
	if statErr != nil {
		_ = client.remove(tempPath)
		return nil, fmt.Errorf("%w: inspect existing file: %w", ErrUpload, statErr)
	}

	// Replacing a prior export is allowed only through the POSIX extension:
	// unlike SFTP v3 Rename, it overwrites atomically and never requires the
	// valid old file to be deleted first.
	if !client.hasExtension("posix-rename@openssh.com") {
		_ = client.remove(tempPath)
		return nil, fmt.Errorf("%w: counterpart does not support atomic replacement", ErrUpload)
	}

	backupPath := path.Join(dir, tempName(filename)+".previous")
	if err := copyRemoteFile(client, finalPath, backupPath); err != nil {
		_ = client.remove(tempPath)
		_ = client.remove(backupPath)
		return nil, err
	}
	if err := client.posixRename(tempPath, finalPath); err != nil {
		_ = client.remove(tempPath)
		_ = client.remove(backupPath)
		return nil, fmt.Errorf("%w: atomic rename: %w", ErrUpload, err)
	}
	return &pendingUpload{client: client, finalPath: finalPath, backupPath: backupPath}, nil
}

func copyRemoteFile(client remoteFileOperations, sourcePath, targetPath string) error {
	source, err := client.open(sourcePath)
	if err != nil {
		return fmt.Errorf("%w: open previous file: %w", ErrUpload, err)
	}
	target, err := client.create(targetPath)
	if err != nil {
		_ = source.Close()
		return fmt.Errorf("%w: create rollback copy: %w", ErrUpload, err)
	}
	_, copyErr := io.Copy(target, source)
	targetCloseErr := target.Close()
	sourceCloseErr := source.Close()
	if copyErr != nil {
		return fmt.Errorf("%w: copy previous file: %w", ErrUpload, copyErr)
	}
	if targetCloseErr != nil {
		return fmt.Errorf("%w: close rollback copy: %w", ErrUpload, targetCloseErr)
	}
	if sourceCloseErr != nil {
		return fmt.Errorf("%w: close previous file: %w", ErrUpload, sourceCloseErr)
	}
	return nil
}

func (c *Client) writeFile(client remoteFileOperations, remotePath string, data []byte) error {
	file, err := client.create(remotePath)
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
