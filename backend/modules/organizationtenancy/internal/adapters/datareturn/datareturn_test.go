package datareturn

import (
	"archive/zip"
	"bytes"
	"context"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"io"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func testCrypto() organizationtenancy.DataReturnCrypto {
	return organizationtenancy.DataReturnCrypto{NewToken: testpkg.DataReturnNewToken, TokenLookup: testpkg.DataReturnTokenLookup,
		NewArchiveKey: testpkg.DataReturnNewArchiveKey, KeyCheck: testpkg.DataReturnKeyCheck, KeyMatches: testpkg.DataReturnKeyMatches,
		SealKey: testpkg.DataReturnSealKey, OpenKey: testpkg.DataReturnOpenKey, Encrypt: testpkg.DataReturnEncrypt,
		Decrypt: testpkg.DataReturnDecrypt, Fingerprint: testpkg.DataReturnFingerprint}
}

// writePackage writes a COMPLETE school package of one school, as tenant
// export writes it: its platform.schools row, a person row and stored files.
func writePackage(t *testing.T, school int64, secret string, extra map[string][]byte) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "package")
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "objects"), 0o700))
	files := map[string][]byte{
		"platform.schools.jsonl": fmt.Appendf(nil, "{\"id\":%d,\"name\":\"Schule\"}\n", school),
		"users.persons.jsonl":    fmt.Appendf(nil, "{\"id\":1,\"first_name\":%q,\"iban\":\"DE89370400440532013000\"}\n", secret),
		"objects/0":              []byte("%PDF-1.7 " + secret),
	}
	for name, data := range extra {
		files[name] = data
	}
	manifest := organizationtenancy.ExportManifest{FormatVersion: organizationtenancy.ExportFormatVersion, CatalogVersion: 1, ExportedAt: "2026-11-10T08:00:00Z"}
	for name, data := range files {
		require.NoError(t, os.WriteFile(filepath.Join(dir, filepath.FromSlash(name)), data, 0o600))
		manifest.Artifacts = append(manifest.Artifacts, organizationtenancy.ExportArtifact{Path: name, SHA256: testpkg.ContentFingerprint(data), Bytes: int64(len(data))})
	}
	data, err := json.Marshal(manifest)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "manifest.json"), data, 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "COMPLETE"), []byte(testpkg.ContentFingerprint(data)+"\n"), 0o600))
	return dir
}

func customer(downloads int) organizationtenancy.DataReturnRecipient {
	return organizationtenancy.DataReturnRecipient{Recipient: organizationtenancy.DataReturnCustomer, Name: "Schule am Berg",
		ValidFor: 7 * 24 * time.Hour, MaxDownloads: downloads}
}

// fixture binds both sides to one test database and store. The release
// side assumes phoenix_data_return by itself; the retrieval runs on a
// session that assumed phoenix_auth, the role of the HTTP pool.
type fixture struct {
	db        *bun.DB
	store     string
	operator  int64
	release   *Release
	retrieval *Retrieval
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	return newFixtureOn(t, testpkg.SetupTestDB(t))
}

func newFixtureOn(t *testing.T, db *bun.DB) *fixture {
	t.Helper()
	operator := testpkg.CreateTestOperator(t, db)
	store := t.TempDir()
	conn := roleSession(t, db, "phoenix_auth")
	f := &fixture{db: db, store: store, operator: operator.ID, release: NewRelease(db, testCrypto(), store),
		retrieval: NewRetrieval(conn, testCrypto(), store, nil)}
	t.Cleanup(func() {
		_, err := db.ExecContext(context.Background(), `DELETE FROM platform.data_return_grants WHERE issued_by = ?`, f.operator)
		require.NoError(t, err)
	})
	return f
}

func (f *fixture) releaseSchool(t *testing.T, school int64, secret string, downloads int) organizationtenancy.DataReturnIssued {
	t.Helper()
	issued, err := f.release.Release(t.Context(), organizationtenancy.DataReturnReleaseRequest{Package: writePackage(t, school, secret, nil),
		SchoolID: school, OperatorID: f.operator, KeepUntil: time.Now().Add(60 * 24 * time.Hour), DataReturnRecipient: customer(downloads)})
	require.NoError(t, err)
	return issued
}

func download(retrieval *Retrieval, token string) ([]byte, organizationtenancy.DataReturnPackage, error) {
	var body bytes.Buffer
	var got organizationtenancy.DataReturnPackage
	err := retrieval.Download(context.Background(), token, func(pkg organizationtenancy.DataReturnPackage) io.Writer {
		got = pkg
		return &body
	})
	return body.Bytes(), got, err
}

func zipFiles(t *testing.T, archive []byte) map[string]string {
	t.Helper()
	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	require.NoError(t, err)
	files := map[string]string{}
	for _, file := range reader.File {
		r, err := file.Open()
		require.NoError(t, err)
		data, err := io.ReadAll(r)
		require.NoError(t, err)
		files[file.Name] = string(data)
	}
	return files
}

func TestReleaseServesAPackageOnlyToItsToken(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	a, b := testpkg.UniqueTestTenantID(t), testpkg.UniqueTestTenantID(t)
	issuedA := f.releaseSchool(t, a, "Synthetic A", 2)
	issuedB := f.releaseSchool(t, b, "Synthetic B", 2)
	require.NotEqual(t, issuedA.ArchiveName, issuedB.ArchiveName)
	require.True(t, strings.HasPrefix(issuedA.Token, "mdr_"))

	// At rest the archive holds no plaintext, and the database no token or key.
	stored, err := os.ReadFile(filepath.Join(f.store, issuedA.ArchiveName+".mdr"))
	require.NoError(t, err)
	require.NotContains(t, string(stored), "Synthetic A")
	require.NotContains(t, string(stored), "DE89370400440532013000")
	var rows []string
	require.NoError(t, f.db.NewRaw(`SELECT row_to_json(g)::text FROM platform.data_return_grants AS g WHERE issued_by = ?`, f.operator).Scan(t.Context(), &rows))
	require.Len(t, rows, 2)
	for _, row := range rows {
		require.NotContains(t, row, issuedA.Token[4:])
		require.NotContains(t, row, issuedB.Token[4:])
	}

	described, err := f.retrieval.Package(t.Context(), issuedA.Token)
	require.NoError(t, err)
	require.Equal(t, "school", described.Package)
	require.Equal(t, 2, described.DownloadsLeft)
	archive, pkg, err := download(f.retrieval, issuedA.Token)
	require.NoError(t, err)
	require.Equal(t, int64(len(archive)), pkg.ArchiveBytes)
	require.Equal(t, testpkg.ContentFingerprint(archive), pkg.ArchiveSHA256)
	files := zipFiles(t, archive)
	require.Contains(t, files["users.persons.jsonl"], "Synthetic A")
	for name, content := range files {
		require.NotContains(t, content, "Synthetic B", "one token opens exactly its own package (%s)", name)
	}
	require.Contains(t, files, "COMPLETE")

	for _, token := range []string{"", "mdr_unknown", issuedA.Token + "x", strings.TrimPrefix(issuedB.Token, "mdr_")} {
		_, err := f.retrieval.Package(t.Context(), token)
		require.ErrorIs(t, err, organizationtenancy.ErrDataReturnDenied, "token %q", token)
	}

	// Every action is in the operator audit, without a token or key.
	var audit []string
	require.NoError(t, f.db.NewRaw(`SELECT action || ' ' || changes::text FROM platform.operator_audit_log WHERE operator_id = ? ORDER BY id`, f.operator).Scan(t.Context(), &audit))
	require.Len(t, audit, 2)
	for _, entry := range audit {
		require.True(t, strings.HasPrefix(entry, "data_return_release "), entry)
		require.NotContains(t, entry, issuedA.Token[4:])
	}
}

func TestDownloadBudgetHoldsUnderConcurrentDownloads(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	issued := f.releaseSchool(t, testpkg.UniqueTestTenantID(t), "Synthetic", 2)
	// The pool, not the single phoenix_auth session: downloads run in parallel.
	pool := NewRetrieval(f.db, testCrypto(), f.store, nil)
	var wg sync.WaitGroup
	results := make(chan error, 6)
	for range 6 {
		wg.Go(func() {
			_, _, err := download(pool, issued.Token)
			results <- err
		})
	}
	wg.Wait()
	close(results)
	succeeded := 0
	for err := range results {
		if err == nil {
			succeeded++
		} else {
			require.ErrorIs(t, err, organizationtenancy.ErrDataReturnDenied)
		}
	}
	require.Equal(t, 2, succeeded, "the budget of two downloads holds")
}

func TestRevocationStopsADownloadInFlight(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	school := testpkg.UniqueTestTenantID(t)
	// 1 MiB that deflate cannot shrink, so the archive spans many checks.
	big := make([]byte, 1<<20)
	random := rand.New(rand.NewPCG(1, 2))
	for i := range big {
		big[i] = byte(random.Uint32())
	}
	issued, err := f.release.Release(t.Context(), organizationtenancy.DataReturnReleaseRequest{
		Package: writePackage(t, school, "x", map[string][]byte{"objects/1": big}), SchoolID: school, OperatorID: f.operator,
		KeepUntil: time.Now().Add(60 * 24 * time.Hour), DataReturnRecipient: customer(3)})
	require.NoError(t, err)
	f.retrieval.recheckBytes = 64 << 10
	var body bytes.Buffer
	revoked := false
	err = f.retrieval.Download(t.Context(), issued.Token, func(organizationtenancy.DataReturnPackage) io.Writer {
		return writerFunc(func(p []byte) (int, error) {
			if !revoked && body.Len() > 0 {
				revoked = true
				count, err := f.release.Revoke(context.Background(), organizationtenancy.DataReturnRevokeRequest{GrantID: issued.GrantID, OperatorID: f.operator})
				require.NoError(t, err)
				require.Equal(t, 1, count)
			}
			return body.Write(p)
		})
	})
	require.ErrorIs(t, err, organizationtenancy.ErrDataReturnDenied)
	require.Less(t, int64(body.Len()), issued.ArchiveBytes)
	var outcome string
	require.NoError(t, f.db.NewRaw(`SELECT outcome FROM platform.data_return_downloads WHERE grant_id = ?`, issued.GrantID).Scan(t.Context(), &outcome))
	require.Equal(t, "failed", outcome)
}

// roleSession answers a session that assumed role. It is discarded at the
// end, so the role never returns to the pool of the package database.
func roleSession(t *testing.T, db *bun.DB, role string) bun.Conn {
	t.Helper()
	conn, err := db.Conn(t.Context())
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = conn.Raw(func(any) error { return driver.ErrBadConn })
		_ = conn.Close()
	})
	_, err = conn.ExecContext(t.Context(), "SET ROLE "+role)
	require.NoError(t, err)
	return conn
}

type writerFunc func([]byte) (int, error)

func (w writerFunc) Write(p []byte) (int, error) { return w(p) }

// revoke --all and the purge act on every grant of a database: this test
// has a database of its own.
func TestGrantRevokeAllAndPurge(t *testing.T) {
	t.Parallel()
	f := newFixtureOn(t, testpkg.SetupIsolatedTestDB(t))
	a := f.releaseSchool(t, testpkg.UniqueTestTenantID(t), "A", 1)
	b := f.releaseSchool(t, testpkg.UniqueTestTenantID(t), "B", 1)

	_, err := f.release.Grant(t.Context(), organizationtenancy.DataReturnGrantRequest{ArchiveName: a.ArchiveName, ArchiveKey: b.ArchiveKey,
		OperatorID: f.operator, DataReturnRecipient: customer(1)})
	require.ErrorContains(t, err, "does not belong")
	target := organizationtenancy.DataReturnRecipient{Recipient: organizationtenancy.DataReturnTargetProvider, Name: "Neuer Anbieter GmbH",
		ValidFor: 24 * time.Hour, MaxDownloads: 1}
	granted, err := f.release.Grant(t.Context(), organizationtenancy.DataReturnGrantRequest{ArchiveName: a.ArchiveName, ArchiveKey: a.ArchiveKey,
		OperatorID: f.operator, DataReturnRecipient: target})
	require.NoError(t, err)
	_, pkg, err := download(f.retrieval, granted.Token)
	require.NoError(t, err)
	require.Equal(t, "target_provider", pkg.Recipient)

	// After a database restore, every unpurged token is revoked at once.
	count, err := f.release.Revoke(t.Context(), organizationtenancy.DataReturnRevokeRequest{All: true, OperatorID: f.operator})
	require.NoError(t, err)
	require.GreaterOrEqual(t, count, 3)
	for _, token := range []string{a.Token, b.Token, granted.Token} {
		_, err := f.retrieval.Package(t.Context(), token)
		require.ErrorIs(t, err, organizationtenancy.ErrDataReturnDenied)
	}
	_, err = f.release.Grant(t.Context(), organizationtenancy.DataReturnGrantRequest{ArchiveName: a.ArchiveName, ArchiveKey: a.ArchiveKey,
		OperatorID: f.operator, DataReturnRecipient: customer(1)})
	require.NoError(t, err, "a revoked archive gets a new token until it is purged")

	// An orphan: a release that died between writing and storing its grant.
	orphan := filepath.Join(f.store, strings.Repeat("ab", 16)+".mdr")
	require.NoError(t, os.WriteFile(orphan, []byte("x"), 0o600))
	old := time.Now().Add(-2 * time.Hour)
	require.NoError(t, os.Chtimes(orphan, old, old))

	purged, err := f.release.Purge(t.Context(), f.operator)
	require.NoError(t, err)
	require.GreaterOrEqual(t, purged.Archives, 1)
	require.GreaterOrEqual(t, purged.Orphans, 1)
	require.NoFileExists(t, filepath.Join(f.store, b.ArchiveName+".mdr"))
	require.FileExists(t, filepath.Join(f.store, a.ArchiveName+".mdr"), "the fresh grant keeps archive A")
	require.NoFileExists(t, orphan)

	grants, err := f.release.Grants(t.Context(), f.operator)
	require.NoError(t, err)
	var sawA bool
	for _, grant := range grants {
		if grant.ArchiveName == a.ArchiveName && grant.PurgedAt == nil {
			sawA = true
			require.NotNil(t, grant.ArchiveStored)
			require.True(t, *grant.ArchiveStored)
		}
	}
	require.True(t, sawA)
	var actions []string
	require.NoError(t, f.db.NewRaw(`SELECT DISTINCT action FROM platform.operator_audit_log WHERE operator_id = ? ORDER BY action`, f.operator).Scan(t.Context(), &actions))
	require.Equal(t, []string{"data_return_grant", "data_return_purge", "data_return_release", "data_return_revoke"}, actions)
}

func TestReleaseRefusesAnythingButTheCheckedPackageOfTheNamedSubject(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	school := testpkg.UniqueTestTenantID(t)
	keep := time.Now().Add(60 * 24 * time.Hour)
	request := func(dir string, schoolID, operator int64) organizationtenancy.DataReturnReleaseRequest {
		return organizationtenancy.DataReturnReleaseRequest{Package: dir, SchoolID: schoolID, OperatorID: operator, KeepUntil: keep, DataReturnRecipient: customer(1)}
	}
	cases := map[string]func() organizationtenancy.DataReturnReleaseRequest{
		"another school": func() organizationtenancy.DataReturnReleaseRequest {
			return request(writePackage(t, school, "x", nil), school+1, f.operator)
		},
		"unknown operator": func() organizationtenancy.DataReturnReleaseRequest {
			return request(writePackage(t, school, "x", nil), school, f.operator+1_000_000)
		},
		"no COMPLETE": func() organizationtenancy.DataReturnReleaseRequest {
			dir := writePackage(t, school, "x", nil)
			require.NoError(t, os.Remove(filepath.Join(dir, "COMPLETE")))
			return request(dir, school, f.operator)
		},
		"changed file": func() organizationtenancy.DataReturnReleaseRequest {
			dir := writePackage(t, school, "x", nil)
			require.NoError(t, os.WriteFile(filepath.Join(dir, "users.persons.jsonl"), []byte("{}\n"), 0o600))
			return request(dir, school, f.operator)
		},
		"extra file": func() organizationtenancy.DataReturnReleaseRequest {
			dir := writePackage(t, school, "x", nil)
			require.NoError(t, os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("x"), 0o600))
			return request(dir, school, f.operator)
		},
		"link": func() organizationtenancy.DataReturnReleaseRequest {
			dir := writePackage(t, school, "x", nil)
			require.NoError(t, os.Symlink("/etc/hosts", filepath.Join(dir, "objects", "1")))
			return request(dir, school, f.operator)
		},
		"token outlives the retrieval period": func() organizationtenancy.DataReturnReleaseRequest {
			r := request(writePackage(t, school, "x", nil), school, f.operator)
			r.KeepUntil = time.Now().Add(time.Hour)
			return r
		},
	}
	for name, build := range cases {
		_, err := f.release.Release(t.Context(), build())
		require.Error(t, err, name)
	}
	entries, err := os.ReadDir(f.store)
	require.NoError(t, err)
	require.Empty(t, entries, "a refused release leaves nothing in the store")
}

// A file swapped or turned into a link between the check and the ZIP never
// leaves: writeZip compares every file with what the check verified.
func TestZipRefusesAFileChangedAfterTheCheck(t *testing.T) {
	t.Parallel()
	school := testpkg.UniqueTestTenantID(t)
	for name, change := range map[string]func(string){
		"content": func(dir string) {
			require.NoError(t, os.WriteFile(filepath.Join(dir, "objects", "0"), []byte("%PDF-1.7 other"), 0o600))
		},
		"link": func(dir string) {
			target := filepath.Join(dir, "objects", "0")
			copied := filepath.Join(t.TempDir(), "copy")
			data, err := os.ReadFile(target)
			require.NoError(t, err)
			require.NoError(t, os.WriteFile(copied, data, 0o600))
			require.NoError(t, os.Remove(target))
			require.NoError(t, os.Symlink(copied, target))
		},
	} {
		dir := writePackage(t, school, "x", nil)
		checked, err := checkPackage(dir, school, 0, testpkg.DataReturnFingerprint)
		require.NoError(t, err)
		change(dir)
		require.ErrorContains(t, writeZip(io.Discard, checked, testpkg.DataReturnFingerprint), "changed after the check", name)
	}
}

// The token never outlives the retrieval period, whatever the lifetime asks
// and whenever the issue happens: one now, clamped.
func TestExpiryIsClampedToTheRetrievalPeriod(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	keep := now.Add(48 * time.Hour)
	require.Equal(t, now.Add(24*time.Hour), expiry(now, 24*time.Hour, keep))
	require.Equal(t, keep, expiry(now, 7*24*time.Hour, keep))
}

func TestRolesHoldExactlyTheirRights(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	conn := roleSession(t, db, "phoenix_data_return")
	_, err := conn.ExecContext(t.Context(), `SELECT 1 FROM platform.data_return_grants LIMIT 1`)
	require.ErrorContains(t, err, "permission denied", "the release role reaches the grants only through the functions")
	_, err = conn.ExecContext(t.Context(), `SELECT 1 FROM platform.operator_audit_log LIMIT 1`)
	require.ErrorContains(t, err, "permission denied")
	auth := roleSession(t, db, "phoenix_auth")
	_, err = auth.ExecContext(t.Context(), `SELECT platform.data_return_list(1)`)
	require.ErrorContains(t, err, "permission denied", "the HTTP role cannot run release functions")
}
