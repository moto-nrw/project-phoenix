package datareturn

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/uptrace/bun"
)

// Release is the operator side, run by the CLI: it releases, grants,
// revokes, purges and lists through the functions of migration 1.15.910.
type Release struct {
	store  releaseStore
	crypto organizationtenancy.DataReturnCrypto
	root   string
	now    func() time.Time
}

// NewRelease binds the operator side to a database whose login may assume
// phoenix_data_return, and to the store directory.
func NewRelease(db *bun.DB, crypto organizationtenancy.DataReturnCrypto, root string) *Release {
	return &Release{store: releaseStore{db: db}, crypto: crypto, root: root, now: time.Now}
}

// The longest retrieval period a release may name: the 7-month alternative
// period of Art. 25 Abs. 4 plus the 30-day retrieval period, with margin.
const maxKeepUntil = 366 * 24 * time.Hour

// A temporary release file or an archive without a live grant is swept only
// after these ages, so a release that is still writing or about to store
// its first grant is never touched.
const (
	sweepTemporaryAfter = 24 * time.Hour
	sweepArchiveAfter   = time.Hour
)

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

// expiry is the end of a token issued at now: its lifetime, but never past
// the retrieval period. One now serves validation and the stored row.
func expiry(now time.Time, validFor time.Duration, keepUntil time.Time) time.Time {
	expires := now.Add(validFor)
	if expires.After(keepUntil) {
		return keepUntil
	}
	return expires
}

// Release checks the package, encrypts it into the store and issues the
// first token. The archive key is answered once, for further grants.
func (r *Release) Release(ctx context.Context, request organizationtenancy.DataReturnReleaseRequest) (organizationtenancy.DataReturnIssued, error) {
	var issued organizationtenancy.DataReturnIssued
	now := r.now().UTC()
	if err := validRecipient(request.DataReturnRecipient); err != nil {
		return issued, err
	}
	keepUntil := request.KeepUntil.UTC()
	if keepUntil.Before(now.Add(request.ValidFor)) || keepUntil.After(now.Add(maxKeepUntil)) {
		return issued, errors.New("data return: the retrieval period must outlast the token and end within a year")
	}
	if err := r.store.requireOperator(ctx, request.OperatorID); err != nil {
		return issued, err
	}
	checked, err := checkPackage(request.Package, request.SchoolID, request.OrganizationID, r.crypto.Fingerprint)
	if err != nil {
		return issued, err
	}
	key, err := r.crypto.NewArchiveKey()
	if err != nil {
		return issued, err
	}
	// The key check is an HMAC of a fresh random key: its head is a random,
	// unique file name that reveals nothing.
	keyCheck := r.crypto.KeyCheck(key)
	name := keyCheck[:32]
	size, digest, err := r.writeArchive(checked, key, name)
	if err != nil {
		return issued, err
	}
	row := issueRow{Action: "release", Package: checked.kind, SubjectID: checked.subjectID, ArchiveName: name, ArchiveBytes: size,
		ArchiveSHA256: digest, ManifestSHA256: checked.manifestSHA256, FormatVersion: checked.formatVersion, KeyCheck: keyCheck,
		KeepUntil: keepUntil}
	issued, err = r.issue(ctx, row, key, request.OperatorID, request.DataReturnRecipient, now)
	if err != nil {
		_ = os.Remove(r.archivePath(name))
	}
	return issued, err
}

// Grant issues another token for a live archive, with the archive key the
// release answered.
func (r *Release) Grant(ctx context.Context, request organizationtenancy.DataReturnGrantRequest) (organizationtenancy.DataReturnIssued, error) {
	var issued organizationtenancy.DataReturnIssued
	now := r.now().UTC()
	if err := validRecipient(request.DataReturnRecipient); err != nil {
		return issued, err
	}
	if !archiveNamePattern.MatchString(request.ArchiveName) {
		return issued, errors.New("data return: malformed archive name")
	}
	head, live, err := r.store.archive(ctx, request.OperatorID, request.ArchiveName)
	if err != nil {
		return issued, err
	}
	if !live || !head.KeepUntil.After(now) {
		return issued, errors.New("data return: no live archive with this name; release the package again")
	}
	if !r.crypto.KeyMatches(request.ArchiveKey, head.KeyCheck) {
		return issued, errors.New("data return: the archive key does not belong to this archive")
	}
	if _, err := os.Stat(r.archivePath(head.ArchiveName)); err != nil {
		return issued, organizationtenancy.ErrDataReturnArchiveMissing
	}
	row := issueRow{Action: "grant", Package: head.Package, SubjectID: head.SubjectID, ArchiveName: head.ArchiveName,
		ArchiveBytes: head.ArchiveBytes, ArchiveSHA256: head.ArchiveSHA256, ManifestSHA256: head.ManifestSHA256,
		FormatVersion: head.FormatVersion, KeyCheck: head.KeyCheck, KeepUntil: head.KeepUntil.UTC()}
	return r.issue(ctx, row, request.ArchiveKey, request.OperatorID, request.DataReturnRecipient, now)
}

// issue stores one grant with a fresh token. The function checks the
// operator and appends the operator audit in the same transaction.
func (r *Release) issue(ctx context.Context, row issueRow, key []byte, operatorID int64,
	recipient organizationtenancy.DataReturnRecipient, now time.Time,
) (organizationtenancy.DataReturnIssued, error) {
	var issued organizationtenancy.DataReturnIssued
	token, err := r.crypto.NewToken()
	if err != nil {
		return issued, err
	}
	row.WrappedKey, err = r.crypto.SealKey(token, key)
	if err != nil {
		return issued, err
	}
	row.OperatorID, row.Recipient, row.RecipientName = operatorID, recipient.Recipient, strings.TrimSpace(recipient.Name)
	row.TokenLookup, row.MaxDownloads = r.crypto.TokenLookup(token), recipient.MaxDownloads
	row.ExpiresAt = expiry(now, recipient.ValidFor, row.KeepUntil)
	id, err := r.store.issue(ctx, row)
	if err != nil {
		return issued, err
	}
	return organizationtenancy.DataReturnIssued{GrantID: id, ArchiveName: row.ArchiveName, ArchiveSHA256: row.ArchiveSHA256,
		ArchiveBytes: row.ArchiveBytes, ExpiresAt: row.ExpiresAt, KeepUntil: row.KeepUntil, Token: token, ArchiveKey: key}, nil
}

func (r *Release) archivePath(name string) string {
	return filepath.Join(r.root, name+".mdr")
}

// writeArchive zips the checked package and encrypts it on the way into a
// private file of the store; the plaintext never touches the disk. It
// answers the size and SHA-256 of the plaintext ZIP.
func (r *Release) writeArchive(checked checkedPackage, key []byte, name string) (int64, string, error) {
	if err := os.MkdirAll(r.root, 0o700); err != nil {
		return 0, "", errors.New("data return: store unavailable")
	}
	tmp, err := os.CreateTemp(r.root, ".release-*")
	if err != nil {
		return 0, "", errors.New("data return: store unavailable")
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	reader, writer := io.Pipe()
	go func() { writer.CloseWithError(writeZip(writer, checked, r.crypto.Fingerprint)) }()
	hash, digest := r.crypto.Fingerprint()
	counter := &countingWriter{}
	err = r.crypto.Encrypt(key, tmp, io.TeeReader(reader, io.MultiWriter(hash, counter)))
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
	if err := os.Rename(tmp.Name(), r.archivePath(name)); err != nil {
		return 0, "", errors.New("data return: archive not stored")
	}
	return counter.n, digest(), nil
}

type countingWriter struct{ n int64 }

func (c *countingWriter) Write(p []byte) (int, error) {
	c.n += int64(len(p))
	return len(p), nil
}

// Revoke ends one grant, every grant of an archive, or every grant at once.
// It maps the request onto the selectors of the database function: exactly
// one is set, the others travel as NULL.
func (r *Release) Revoke(ctx context.Context, request organizationtenancy.DataReturnRevokeRequest) (int, error) {
	var grant *int64
	var archive *string
	named := 0
	if request.GrantID > 0 {
		grant, named = &request.GrantID, named+1
	}
	if request.ArchiveName != "" {
		if !archiveNamePattern.MatchString(request.ArchiveName) {
			return 0, errors.New("data return: malformed archive name")
		}
		archive, named = &request.ArchiveName, named+1
	}
	if request.All {
		named++
	}
	if named != 1 {
		return 0, errors.New("data return: name exactly one grant, one archive or all")
	}
	return r.store.revoke(ctx, request.OperatorID, grant, archive, request.All)
}

// Purge marks every archive whose retrieval period ended or whose grants are
// all revoked as purged, one by one under its lock and each in the operator
// audit, and only then removes its file. It then sweeps files no live grant
// names and drops evidence rows three years after their period.
func (r *Release) Purge(ctx context.Context, operatorID int64) (organizationtenancy.DataReturnPurged, error) {
	var purged organizationtenancy.DataReturnPurged
	names, err := r.store.names(ctx, "data_return_purgeable", operatorID)
	if err != nil {
		return purged, err
	}
	for _, name := range names {
		done, err := r.store.purgeArchive(ctx, operatorID, name)
		if err != nil {
			return purged, err
		}
		if !done {
			continue // a grant issued since the listing keeps the archive
		}
		if err := os.Remove(r.archivePath(name)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return purged, errors.New("data return: an archive could not be removed; run the purge again")
		}
		purged.Archives++
	}
	if purged.Orphans, err = r.sweep(ctx, operatorID); err != nil {
		return purged, err
	}
	purged.EvidenceGrants, err = r.store.dropEvidence(ctx, operatorID)
	return purged, err
}

// sweep removes what no live grant names: a release that died before its
// grant was stored, or a file whose purge was recorded but not removed.
func (r *Release) sweep(ctx context.Context, operatorID int64) (int, error) {
	entries, err := os.ReadDir(r.root)
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, errors.New("data return: store unreadable")
	}
	live, err := r.store.names(ctx, "data_return_live_archives", operatorID)
	if err != nil {
		return 0, err
	}
	keep := make(map[string]bool, len(live))
	for _, name := range live {
		keep[name+".mdr"] = true
	}
	now, removed := r.now(), 0
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() || keep[entry.Name()] || !sweepable(entry.Name(), now.Sub(info.ModTime())) {
			continue
		}
		if os.Remove(filepath.Join(r.root, entry.Name())) == nil {
			removed++
		}
	}
	return removed, nil
}

func sweepable(name string, age time.Duration) bool {
	switch {
	case strings.HasPrefix(name, ".release-"):
		return age > sweepTemporaryAfter
	case strings.HasSuffix(name, ".mdr") && archiveNamePattern.MatchString(strings.TrimSuffix(name, ".mdr")):
		return age > sweepArchiveAfter
	default:
		return false
	}
}

// Grants answers every grant without tokens or keys, to an active operator,
// and whether the archive of a live grant is in the store: after a restore
// of the database without the store, a grant can name a missing archive.
func (r *Release) Grants(ctx context.Context, operatorID int64) ([]organizationtenancy.DataReturnGrantSummary, error) {
	grants, err := r.store.list(ctx, operatorID)
	if err != nil {
		return nil, err
	}
	for i := range grants {
		if grants[i].PurgedAt != nil {
			continue
		}
		_, statErr := os.Stat(r.archivePath(grants[i].ArchiveName))
		stored := statErr == nil
		grants[i].ArchiveStored = &stored
	}
	return grants, nil
}
