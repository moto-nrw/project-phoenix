package datareturn

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/uptrace/bun"
)

// Service is both sides of the retrieval: the CLI releases, grants,
// revokes and purges; the HTTP route serves a package to its token.
type Service struct {
	store  store
	crypto organizationtenancy.DataReturnCrypto
	audit  organizationtenancy.DataReturnAudit
	root   string
	now    func() time.Time
	logger *slog.Logger
}

// New binds the service. audit is required for the CLI side only.
func New(db bun.IDB, crypto organizationtenancy.DataReturnCrypto, audit organizationtenancy.DataReturnAudit, root string, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{store: store{db: db}, crypto: crypto, audit: audit, root: root, now: time.Now, logger: logger}
}

// The longest retrieval period a release may name: the 7-month alternative
// period of Art. 25 Abs. 4 plus the 30-day retrieval period, with margin.
const maxKeepUntil = 366 * 24 * time.Hour

var archiveNamePattern = regexp.MustCompile(`^[0-9a-f]{32}$`)

func validRecipient(recipient organizationtenancy.DataReturnRecipient) error {
	if recipient.Recipient != organizationtenancy.DataReturnCustomer && recipient.Recipient != organizationtenancy.DataReturnTargetProvider {
		return errors.New("data return: recipient must be customer or target_provider")
	}
	name := strings.TrimSpace(recipient.Name)
	if name == "" || utf8.RuneCountInString(name) > organizationtenancy.DataReturnMaxRecipientLen {
		return errors.New("data return: name the recipient organisation (1 to 200 characters)")
	}
	if recipient.ValidFor < time.Hour || recipient.ValidFor > organizationtenancy.DataReturnMaxGrantDays*24*time.Hour {
		return errors.New("data return: a token is valid for 1 hour to 30 days")
	}
	if recipient.MaxDownloads < 1 || recipient.MaxDownloads > organizationtenancy.DataReturnMaxDownloads {
		return errors.New("data return: a token allows 1 to 20 downloads")
	}
	return nil
}

// Release checks the package, encrypts it into the store and issues the
// first token. The archive key is answered once, for further grants.
func (s *Service) Release(ctx context.Context, request organizationtenancy.DataReturnReleaseRequest) (organizationtenancy.DataReturnIssued, error) {
	var issued organizationtenancy.DataReturnIssued
	now := s.now().UTC()
	if err := validRecipient(request.DataReturnRecipient); err != nil {
		return issued, err
	}
	keepUntil := request.KeepUntil.UTC()
	if keepUntil.Before(now.Add(request.ValidFor)) || keepUntil.After(now.Add(maxKeepUntil)) {
		return issued, errors.New("data return: the retrieval period must outlast the token and end within a year")
	}
	if err := s.audit.BeginDataReturn(ctx, request.OperatorID); err != nil {
		return issued, err
	}
	checked, err := checkPackage(request.Package, request.SchoolID, request.OrganizationID, s.crypto.Fingerprint)
	if err != nil {
		return issued, err
	}
	key, err := s.crypto.NewArchiveKey()
	if err != nil {
		return issued, err
	}
	// The key check is an HMAC of a fresh random key: its head is a random,
	// unique file name that reveals nothing.
	keyCheck := s.crypto.KeyCheck(key)
	name := keyCheck[:32]
	size, digest, err := s.writeArchive(checked, key, name)
	if err != nil {
		return issued, err
	}
	row := grantRow{Package: checked.kind, SubjectID: checked.subjectID, ArchiveName: name, ArchiveBytes: size, ArchiveSHA256: digest,
		ManifestSHA256: checked.manifestSHA256, FormatVersion: checked.formatVersion, KeyCheck: keyCheck, KeepUntil: keepUntil}
	issued, err = s.issue(ctx, row, key, request.OperatorID, request.DataReturnRecipient, "release")
	if err != nil {
		_ = os.Remove(s.archivePath(name))
	}
	return issued, err
}

// Grant issues another token for a released archive, with the archive key
// the release answered.
func (s *Service) Grant(ctx context.Context, request organizationtenancy.DataReturnGrantRequest) (organizationtenancy.DataReturnIssued, error) {
	var issued organizationtenancy.DataReturnIssued
	if err := validRecipient(request.DataReturnRecipient); err != nil {
		return issued, err
	}
	if !archiveNamePattern.MatchString(request.ArchiveName) {
		return issued, errors.New("data return: malformed archive name")
	}
	if err := s.audit.BeginDataReturn(ctx, request.OperatorID); err != nil {
		return issued, err
	}
	row, err := s.store.byArchive(ctx, request.ArchiveName)
	if err != nil {
		return issued, err
	}
	if !s.crypto.KeyMatches(request.ArchiveKey, row.KeyCheck) {
		return issued, errors.New("data return: the archive key does not belong to this archive")
	}
	if s.now().Add(request.ValidFor).After(row.KeepUntil) {
		return issued, errors.New("data return: the token would outlast the retrieval period")
	}
	if _, err := os.Stat(s.archivePath(row.ArchiveName)); err != nil {
		return issued, organizationtenancy.ErrDataReturnArchiveMissing
	}
	return s.issue(ctx, row, request.ArchiveKey, request.OperatorID, request.DataReturnRecipient, "grant")
}

// issue stores one grant with a fresh token and records it in the operator
// audit; without the audit record the grant does not stay.
func (s *Service) issue(ctx context.Context, base grantRow, key []byte, operatorID int64,
	recipient organizationtenancy.DataReturnRecipient, action string,
) (organizationtenancy.DataReturnIssued, error) {
	var issued organizationtenancy.DataReturnIssued
	token, err := s.crypto.NewToken()
	if err != nil {
		return issued, err
	}
	wrapped, err := s.crypto.SealKey(token, key)
	if err != nil {
		return issued, err
	}
	row := base
	row.ID, row.Recipient, row.RecipientName = 0, recipient.Recipient, strings.TrimSpace(recipient.Name)
	row.TokenLookup, row.WrappedKey, row.IssuedBy, row.MaxDownloads = s.crypto.TokenLookup(token), wrapped, operatorID, recipient.MaxDownloads
	row.ExpiresAt = s.now().UTC().Add(recipient.ValidFor)
	if err := s.store.insert(ctx, &row); err != nil {
		return issued, fmt.Errorf("data return: grant not stored: %w", err)
	}
	expires, keep := row.ExpiresAt, row.KeepUntil
	record := organizationtenancy.DataReturnAuditRecord{OperatorID: operatorID, Action: action, Package: row.Package, SubjectID: row.SubjectID,
		GrantID: row.ID, ArchiveName: row.ArchiveName, ManifestSHA256: row.ManifestSHA256, Recipient: row.Recipient, ExpiresAt: &expires, KeepUntil: &keep}
	if err := s.audit.RecordDataReturn(ctx, record); err != nil {
		_ = s.store.remove(ctx, row.ID)
		return issued, errors.New("data return: operator audit unavailable; the grant was withdrawn")
	}
	return organizationtenancy.DataReturnIssued{GrantID: row.ID, ArchiveName: row.ArchiveName, ArchiveSHA256: row.ArchiveSHA256,
		ArchiveBytes: row.ArchiveBytes, ExpiresAt: row.ExpiresAt, KeepUntil: row.KeepUntil, Token: token, ArchiveKey: key}, nil
}

func (s *Service) archivePath(name string) string {
	return filepath.Join(s.root, name+".mdr")
}

// writeArchive zips the checked package and encrypts it on the way into a
// private file of the store; the plaintext never touches the disk. It
// answers the size and SHA-256 of the plaintext ZIP.
func (s *Service) writeArchive(checked checkedPackage, key []byte, name string) (int64, string, error) {
	if err := os.MkdirAll(s.root, 0o700); err != nil {
		return 0, "", errors.New("data return: store unavailable")
	}
	tmp, err := os.CreateTemp(s.root, ".release-*")
	if err != nil {
		return 0, "", errors.New("data return: store unavailable")
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	reader, writer := io.Pipe()
	go func() { writer.CloseWithError(writeZip(writer, checked)) }()
	hash, digest := s.crypto.Fingerprint()
	counter := &countingWriter{}
	err = s.crypto.Encrypt(key, tmp, io.TeeReader(reader, io.MultiWriter(hash, counter)))
	_ = reader.CloseWithError(err)
	if err == nil {
		err = tmp.Sync()
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return 0, "", fmt.Errorf("data return: archive not written: %w", err)
	}
	if err := os.Rename(tmp.Name(), s.archivePath(name)); err != nil {
		return 0, "", errors.New("data return: archive not stored")
	}
	return counter.n, digest(), nil
}

type countingWriter struct{ n int64 }

func (c *countingWriter) Write(p []byte) (int, error) {
	c.n += int64(len(p))
	return len(p), nil
}

// Revoke ends one grant or every grant of an archive at once.
func (s *Service) Revoke(ctx context.Context, request organizationtenancy.DataReturnRevokeRequest) (int, error) {
	if (request.GrantID > 0) == (request.ArchiveName != "") || request.ArchiveName != "" && !archiveNamePattern.MatchString(request.ArchiveName) {
		return 0, errors.New("data return: name exactly one grant or one archive")
	}
	if err := s.audit.BeginDataReturn(ctx, request.OperatorID); err != nil {
		return 0, err
	}
	rows, err := s.store.revoke(ctx, request.GrantID, request.ArchiveName, s.now().UTC())
	if err != nil {
		return 0, err
	}
	for _, row := range rows {
		if err := s.audit.RecordDataReturn(ctx, organizationtenancy.DataReturnAuditRecord{OperatorID: request.OperatorID, Action: "revoke",
			Package: row.Package, SubjectID: row.SubjectID, GrantID: row.ID, ArchiveName: row.ArchiveName}); err != nil {
			return len(rows), errors.New("data return: revoked, but the operator audit is incomplete")
		}
	}
	return len(rows), nil
}

// Purge removes every archive whose retrieval period ended or whose grants
// are all revoked, and the evidence rows three years after that.
func (s *Service) Purge(ctx context.Context, operatorID int64) (organizationtenancy.DataReturnPurged, error) {
	var purged organizationtenancy.DataReturnPurged
	if err := s.audit.BeginDataReturn(ctx, operatorID); err != nil {
		return purged, err
	}
	now := s.now().UTC()
	names, err := s.store.purgeable(ctx, now)
	if err != nil {
		return purged, err
	}
	for _, name := range names {
		if err := os.Remove(s.archivePath(name)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return purged, errors.New("data return: an archive could not be removed")
		}
		if err := s.store.markPurged(ctx, name, now); err != nil {
			return purged, err
		}
		purged.Archives++
	}
	purged.EvidenceGrants, err = s.store.dropEvidence(ctx, now.AddDate(-organizationtenancy.DataReturnEvidenceYears, 0, 0))
	if err != nil {
		return purged, err
	}
	return purged, s.audit.RecordDataReturn(ctx, organizationtenancy.DataReturnAuditRecord{OperatorID: operatorID, Action: "purge",
		Count: purged.Archives})
}

// Grants answers every grant without tokens or keys, to an active operator.
func (s *Service) Grants(ctx context.Context, operatorID int64) ([]organizationtenancy.DataReturnGrantSummary, error) {
	if err := s.audit.BeginDataReturn(ctx, operatorID); err != nil {
		return nil, err
	}
	rows, err := s.store.list(ctx)
	if err != nil {
		return nil, err
	}
	summaries := make([]organizationtenancy.DataReturnGrantSummary, 0, len(rows))
	for _, row := range rows {
		summaries = append(summaries, row.summary())
	}
	return summaries, nil
}

// Package answers what the token's package is, without spending a download.
func (s *Service) Package(ctx context.Context, token string) (organizationtenancy.DataReturnPackage, error) {
	row, err := s.usableGrant(ctx, token)
	if err != nil {
		return organizationtenancy.DataReturnPackage{}, err
	}
	return row.toPackage(), nil
}

func (s *Service) usableGrant(ctx context.Context, token string) (grantRow, error) {
	if !strings.HasPrefix(token, "mdr_") || len(token) > 128 {
		return grantRow{}, organizationtenancy.ErrDataReturnDenied
	}
	row, err := s.store.byLookup(ctx, s.crypto.TokenLookup(token))
	if err != nil {
		return row, err
	}
	if !row.usable(s.now()) {
		return row, organizationtenancy.ErrDataReturnDenied
	}
	return row, nil
}

// Download spends one download and streams the decrypted archive. The key
// opens only with the token; the archive is the one the grant names.
func (s *Service) Download(ctx context.Context, token string, start func(organizationtenancy.DataReturnPackage) io.Writer) error {
	row, err := s.usableGrant(ctx, token)
	if err != nil {
		return err
	}
	key, err := s.crypto.OpenKey(token, row.WrappedKey)
	if err != nil {
		return organizationtenancy.ErrDataReturnDenied
	}
	file, err := os.Open(s.archivePath(row.ArchiveName))
	if err != nil {
		s.logger.ErrorContext(ctx, "data return archive missing", "grant_id", row.ID)
		return organizationtenancy.ErrDataReturnArchiveMissing
	}
	defer func() { _ = file.Close() }()
	download, downloads, err := s.store.countDownload(ctx, row.ID, s.now())
	if err != nil {
		return err
	}
	row.Downloads = downloads
	counter := &countingWriter{}
	err = s.crypto.Decrypt(key, io.MultiWriter(start(row.toPackage()), counter), file)
	completed := err == nil && counter.n == row.ArchiveBytes
	if finishErr := s.store.finishDownload(context.WithoutCancel(ctx), download, s.now(), completed, counter.n); finishErr != nil {
		s.logger.ErrorContext(ctx, "data return download not recorded",
			"grant_id", row.ID,
			"error", finishErr,
		)
	}
	s.logger.InfoContext(ctx, "data return download",
		"grant_id", row.ID,
		"completed", completed,
		"bytes", counter.n,
	)
	if err == nil && !completed {
		err = errors.New("data return: archive size differs from the release")
	}
	return err
}
