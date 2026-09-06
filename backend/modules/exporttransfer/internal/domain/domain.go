// Package domain holds the Export Transfer values: what a counterpart is,
// what one attempt looks like, and what may be recorded about it.
package domain

import (
	"errors"
	"strings"
)

// Reason codes for a failed attempt. Stable values, never prose: they are
// read back from the journal years later and mapped to German sentences in
// the UI. A transport's own error text would carry paths, versions and a
// server's wording.
const (
	ReasonNotConfigured = "not_configured"
	ReasonAddressDenied = "address_denied"
	ReasonHostKey       = "host_key_mismatch"
	ReasonAuth          = "authentication_rejected"
	ReasonConnect       = "connection_failed"
	ReasonUpload        = "upload_failed"
	ReasonTooLarge      = "file_too_large"
	ReasonInternal      = "internal_error"
)

// Target is a complete, validated counterpart.
//
// It carries the password, so it must never be logged, serialized into an API
// response, or written to the journal.
type Target struct {
	Host               string
	Port               int
	Username           string
	Password           string
	RemoteDirectory    string
	HostKeyFingerprint string
}

// TargetState is the credential-free configuration view.
type TargetState struct {
	Enabled         bool
	Host            string
	Port            int
	RemoteDirectory string
	// MissingSettings names the setting keys still to fill, in form order.
	MissingSettings []string
}

// Ready reports whether a transfer may be started at all.
func (s TargetState) Ready() bool { return s.Enabled && len(s.MissingSettings) == 0 }

// Request is one transfer: which export, which file, on whose behalf. The
// bytes are produced elsewhere — this capability never builds a file.
type Request struct {
	Kind           string
	Format         string
	Filename       string
	Data           []byte
	ActorAccountID int64
	ActorName      string
}

// Result is the outcome of one attempt. A failure is a value, not an error:
// a counterpart can be down, and that is a normal answer.
type Result struct {
	Success         bool
	Filename        string
	ByteSize        int64
	TargetHost      string
	TargetDirectory string
	// Reason is empty on success, one of the codes above otherwise.
	Reason string
}

// JournalEntry is one recorded attempt.
//
// It holds NO credentials: no username, no password, no host key. Host, port
// and directory are kept because they answer "where did this file go"; the
// credentials answer nothing and would make the trail worth attacking.
type JournalEntry struct {
	ActorAccountID  int64
	ActorName       string
	Kind            string
	Format          string
	Filename        string
	ByteSize        int64
	TargetHost      string
	TargetPort      int
	TargetDirectory string
	Success         bool
	Reason          string
}

// ValidPort reports whether a port number can be dialed at all.
func ValidPort(port int) bool { return port >= 1 && port <= 65535 }

// ValidRemoteDirectory accepts an absolute POSIX directory with no traversal
// segment. A ".." segment would put payroll files outside the agreed
// directory; a name that merely contains dots is legitimate.
func ValidRemoteDirectory(dir string) bool {
	if dir == "" || !strings.HasPrefix(dir, "/") {
		return false
	}
	for _, segment := range strings.Split(dir, "/") {
		if segment == ".." {
			return false
		}
	}
	return true
}

// ValidFingerprint accepts only OpenSSH's SHA256 form: the value
// `ssh-keygen -lf` prints. An empty or malformed fingerprint counts as
// missing — there is no trust-on-first-use path, because the whole point of
// the value is that somebody checked it.
func ValidFingerprint(fingerprint string) bool {
	const prefix = "SHA256:"
	if !strings.HasPrefix(fingerprint, prefix) {
		return false
	}
	// base64 of a SHA-256 digest, unpadded.
	return len(fingerprint) == len(prefix)+43
}

// ErrNotConfigured marks an absent or incomplete counterpart. It is not a
// failed transfer: nothing was attempted, so nothing is recorded.
//
// It lives in the domain rather than with the ports so that a resolver built
// in the composition layer can return it without importing the port package.
var ErrNotConfigured = errors.New("export transfer target not configured")
