package organizationtenancy

import (
	"context"
	"errors"
	"io"
	"time"
)

// The retrieval of a released return package (Art. 30 Abs. 2 Data Act,
// ADR 0203). An operator creates and checks the package with `tenant
// export`, then releases it: the CLI encrypts it into the store and issues a
// short-lived token for the customer or a target provider the customer
// names. The token is the only credential of the HTTPS retrieval; it names
// exactly one stored archive, so no request can choose another package.

// DataReturnStoreDir holds the encrypted archives, relative to the working
// directory of the server and of the CLI (/app/data-return in the image).
const DataReturnStoreDir = "data-return"

// The recipients of a retrieval token.
const (
	DataReturnCustomer       = "customer"
	DataReturnTargetProvider = "target_provider"
)

// Limits of a grant: a token is short-lived and has few downloads; the
// operator issues a new one while the retrieval period runs.
const (
	DataReturnMaxGrantDays    = 30
	DataReturnMaxDownloads    = 20
	DataReturnEvidenceYears   = 3
	DataReturnMaxRecipientLen = 200
)

var (
	// ErrDataReturnDenied answers every token that does not open a package:
	// unknown, expired, revoked, used up or purged. The caller learns nothing
	// about which.
	ErrDataReturnDenied = errors.New("data return: no package for this token")
	// ErrDataReturnArchiveMissing reports a granted archive missing from the store.
	ErrDataReturnArchiveMissing = errors.New("data return: archive missing from the store")
)

// DataReturnCrypto is the cryptography Security Runtime provides; the root
// binds it. Tokens and archive keys never reach the database in the clear.
type DataReturnCrypto struct {
	NewToken      func() (string, error)
	TokenLookup   func(token string) string
	NewArchiveKey func() ([]byte, error)
	KeyCheck      func(key []byte) string
	KeyMatches    func(key []byte, check string) bool
	SealKey       func(token string, key []byte) ([]byte, error)
	OpenKey       func(token string, sealed []byte) ([]byte, error)
	Encrypt       func(key []byte, dst io.Writer, src io.Reader) error
	Decrypt       func(key []byte, dst io.Writer, src io.Reader) error
	// Fingerprint answers the SHA-256 of what was written to it, in hex.
	Fingerprint func() (io.Writer, func() string)
}

// Complete reports whether every function is bound.
func (c DataReturnCrypto) Complete() bool {
	return c.NewToken != nil && c.TokenLookup != nil && c.NewArchiveKey != nil && c.KeyCheck != nil && c.KeyMatches != nil &&
		c.SealKey != nil && c.OpenKey != nil && c.Encrypt != nil && c.Decrypt != nil && c.Fingerprint != nil
}

// DataReturnPackage is what a token holder learns about its package before
// and while downloading it. No customer data, no subject ID.
type DataReturnPackage struct {
	Package        string    `json:"package"`
	FormatVersion  int       `json:"formatVersion"`
	ManifestSHA256 string    `json:"manifestSHA256"`
	ArchiveSHA256  string    `json:"archiveSHA256"`
	ArchiveBytes   int64     `json:"archiveBytes"`
	Recipient      string    `json:"recipient"`
	ExpiresAt      time.Time `json:"expiresAt"`
	DownloadsLeft  int       `json:"downloadsLeft"`
}

// DataReturn serves a released package to the holder of its token.
type DataReturn interface {
	Package(ctx context.Context, token string) (DataReturnPackage, error)
	// Download counts one download, then calls start with the package and
	// writes the archive to the writer start answers. An error after start
	// means the body is incomplete.
	Download(ctx context.Context, token string, start func(DataReturnPackage) io.Writer) error
}

// DataReturnAudit records each release, grant, revocation and purge in the
// operator audit (#4720). Begin refuses an unknown or inactive operator
// before anything is read or written.
type DataReturnAudit interface {
	BeginDataReturn(ctx context.Context, operatorID int64) error
	RecordDataReturn(ctx context.Context, record DataReturnAuditRecord) error
}

// DataReturnAuditRecord names an operator action without personal data,
// tokens or keys.
type DataReturnAuditRecord struct {
	OperatorID     int64      `json:"operator_id"`
	Action         string     `json:"action"`
	Package        string     `json:"package,omitempty"`
	SubjectID      int64      `json:"subject_id,omitempty"`
	GrantID        int64      `json:"grant_id,omitempty"`
	ArchiveName    string     `json:"archive_name,omitempty"`
	ManifestSHA256 string     `json:"manifest_sha256,omitempty"`
	Recipient      string     `json:"recipient,omitempty"`
	ExpiresAt      *time.Time `json:"expires_at,omitempty"`
	KeepUntil      *time.Time `json:"keep_until,omitempty"`
	Count          int        `json:"count,omitempty"`
}

// DataReturnRelease is the operator side of the retrieval, run by the CLI.
type DataReturnRelease interface {
	Release(ctx context.Context, request DataReturnReleaseRequest) (DataReturnIssued, error)
	Grant(ctx context.Context, request DataReturnGrantRequest) (DataReturnIssued, error)
	Revoke(ctx context.Context, request DataReturnRevokeRequest) (int, error)
	Purge(ctx context.Context, operatorID int64) (DataReturnPurged, error)
	Grants(ctx context.Context, operatorID int64) ([]DataReturnGrantSummary, error)
}

// DataReturnRecipient names who may retrieve with a token and how long.
type DataReturnRecipient struct {
	Recipient    string
	Name         string
	ValidFor     time.Duration
	MaxDownloads int
}

// DataReturnReleaseRequest releases a checked, COMPLETE package directory.
// SchoolID or OrganizationID, exactly one, must be the subject the package
// itself names. KeepUntil is the end of the retrieval period.
type DataReturnReleaseRequest struct {
	Package        string
	SchoolID       int64
	OrganizationID int64
	OperatorID     int64
	KeepUntil      time.Time
	DataReturnRecipient
}

// DataReturnGrantRequest issues another token for a released archive. The
// operator supplies the archive key the release answered.
type DataReturnGrantRequest struct {
	ArchiveName string
	ArchiveKey  []byte
	OperatorID  int64
	DataReturnRecipient
}

// DataReturnRevokeRequest ends one grant or every grant of an archive.
type DataReturnRevokeRequest struct {
	GrantID     int64
	ArchiveName string
	OperatorID  int64
}

// DataReturnIssued is shown to the operator once. Token and ArchiveKey are
// secrets: hand the token to the recipient over a confirmed channel, keep
// the key in the protected order until the retrieval period ends.
type DataReturnIssued struct {
	GrantID       int64
	ArchiveName   string
	ArchiveSHA256 string
	ArchiveBytes  int64
	ExpiresAt     time.Time
	KeepUntil     time.Time
	Token         string
	ArchiveKey    []byte
}

// DataReturnPurged counts what a purge removed.
type DataReturnPurged struct {
	Archives       int `json:"archives"`
	EvidenceGrants int `json:"evidence_grants"`
}

// DataReturnGrantSummary lists one grant without its token or key.
type DataReturnGrantSummary struct {
	GrantID       int64      `json:"grant_id"`
	Package       string     `json:"package"`
	SubjectID     int64      `json:"subject_id"`
	ArchiveName   string     `json:"archive_name"`
	Recipient     string     `json:"recipient"`
	RecipientName string     `json:"recipient_name"`
	IssuedAt      time.Time  `json:"issued_at"`
	ExpiresAt     time.Time  `json:"expires_at"`
	KeepUntil     time.Time  `json:"keep_until"`
	RevokedAt     *time.Time `json:"revoked_at,omitempty"`
	PurgedAt      *time.Time `json:"purged_at,omitempty"`
	Downloads     int        `json:"downloads"`
	MaxDownloads  int        `json:"max_downloads"`
}
