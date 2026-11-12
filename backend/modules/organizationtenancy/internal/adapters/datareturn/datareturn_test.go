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

	"github.com/moto-ogs/project-phoenix/modules/organizationtenancy"
	testpkg "github.com/moto-ogs/project-phoenix/test"
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

// school creates a school; its grants go before it at cleanup, since the
// root row of a school with an unpurged package refuses to go.
func (f *fixture) school(t *testing.T) int64 {
	t.Helper()
	id, _ := testpkg.CreateTestTenant(t, f.db)
	t.Cleanup(func() {
		_, err := f.db.ExecContext(context.Background(), `DELETE FROM platform.data_return_grants WHERE package = 'school' AND subject_id = ?`, id)
		require.NoError(t, err)
	})
	return id
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
	a, b := f.school(t), f.school(t)
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
	issued := f.releaseSchool(t, f.school(t), "Synthetic", 2)
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
	school := f.school(t)
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
	a := f.releaseSchool(t, f.school(t), "A", 1)
	b := f.releaseSchool(t, f.school(t), "B", 1)

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
	// The web login assumes phoenix_admin for administrative units: that role
	// may neither read nor change a grant or a download row.
	admin := roleSession(t, db, "phoenix_admin")
	for _, statement := range []string{
		`SELECT 1 FROM platform.data_return_grants LIMIT 1`,
		`UPDATE platform.data_return_grants SET max_downloads = 20`,
		`UPDATE platform.data_return_grants SET expires_at = keep_until`,
		`DELETE FROM platform.data_return_grants`,
		`DELETE FROM platform.data_return_downloads`,
		`SELECT nextval('platform.data_return_grants_id_seq')`,
	} {
		_, err := admin.ExecContext(t.Context(), statement)
		require.ErrorContains(t, err, "permission denied", statement)
	}
}

// issueRelease calls data_return_issue for a release of school with the
// given archive name, inside db (a transaction or a pool).
func issueRelease(ctx context.Context, db bun.IDB, operator, school int64, archive string, keepUntil time.Time) error {
	hex64 := strings.Repeat("ab", 32)
	_, err := db.NewRaw(`SELECT platform.data_return_issue(?, 'release', 'school', ?, ?, 10, ?, ?, 1, ?, 'customer', 'Schule', ?, '\x01'::bytea, ?, ?, 1)`,
		operator, school, archive, hex64, hex64, hex64, strings.Repeat("cd", 16)+archive, time.Now().Add(time.Hour), keepUntil).Exec(ctx)
	return err
}

// Scenario A: a release commits while an erasure deletes the school. The
// delete waits for the release's row lock and then finds the grant: the
// school stays, the package is not orphaned. Scenario B: a school that is
// gone gets no release.
func TestReleaseAndErasureOfTheSameSchoolAreSerialized(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	school := f.school(t)
	keep := time.Now().Add(30 * 24 * time.Hour)

	release, err := f.db.BeginTx(t.Context(), nil)
	require.NoError(t, err)
	defer func() { _ = release.Rollback() }()
	require.NoError(t, issueRelease(t.Context(), release, f.operator, school, strings.Repeat("1", 32), keep))

	erase, err := f.db.BeginTx(t.Context(), nil)
	require.NoError(t, err)
	defer func() { _ = erase.Rollback() }()
	var erasePID int
	require.NoError(t, erase.NewRaw(`SELECT pg_backend_pid()`).Scan(t.Context(), &erasePID))
	deleted := make(chan error, 1)
	go func() {
		_, err := erase.ExecContext(context.Background(), `DELETE FROM platform.schools WHERE id = ?`, school)
		deleted <- err
	}()
	require.Eventually(t, func() bool {
		var waiting bool
		err := f.db.NewRaw(`SELECT wait_event_type = 'Lock' FROM pg_stat_activity WHERE pid = ?`, erasePID).Scan(context.Background(), &waiting)
		return err == nil && waiting
	}, 10*time.Second, 20*time.Millisecond, "the delete waits for the release's lock")
	require.NoError(t, release.Commit())
	require.ErrorContains(t, <-deleted, "released return package that is not purged")
	require.NoError(t, erase.Rollback())

	// Scenario B: no release for a school that is gone.
	gone := testpkg.UniqueTestTenantID(t)
	require.ErrorContains(t, issueRelease(t.Context(), f.db, f.operator, gone, strings.Repeat("2", 32), keep), "does not exist or is erased")
	_, err = f.release.Release(t.Context(), organizationtenancy.DataReturnReleaseRequest{Package: writePackage(t, gone, "x", nil),
		SchoolID: gone, OperatorID: f.operator, KeepUntil: keep, DataReturnRecipient: customer(1)})
	require.Error(t, err)
	entries, err := os.ReadDir(f.store)
	require.NoError(t, err)
	require.Empty(t, entries, "a refused release leaves no archive behind")
}

// A grant takes everything but recipient, token and expiry from the
// archive's own rows; the retrieval period is capped in SQL as well.
func TestIssueTrustsNoCallerValuesForAGrant(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	issued := f.releaseSchool(t, f.school(t), "x", 1)
	var keyCheck string
	require.NoError(t, f.db.NewRaw(`SELECT key_check FROM platform.data_return_grants WHERE id = ?`, issued.GrantID).Scan(t.Context(), &keyCheck))
	bogus := strings.Repeat("ef", 32)
	var id int64
	require.NoError(t, f.db.NewRaw(`SELECT platform.data_return_issue(?, 'grant', 'organization', 1, ?, 999999, ?, ?, 7, ?, 'customer', 'X', ?, '\x01'::bytea, ?, ?, 1)`,
		f.operator, issued.ArchiveName, bogus, bogus, keyCheck, strings.Repeat("9", 64), time.Now().Add(time.Hour), time.Now().AddDate(5, 0, 0)).Scan(t.Context(), &id))
	var same bool
	require.NoError(t, f.db.NewRaw(`SELECT g.package = o.package AND g.subject_id = o.subject_id AND g.archive_bytes = o.archive_bytes
		AND g.archive_sha256 = o.archive_sha256 AND g.manifest_sha256 = o.manifest_sha256 AND g.format_version = o.format_version
		AND g.keep_until = o.keep_until FROM platform.data_return_grants g, platform.data_return_grants o WHERE g.id = ? AND o.id = ?`,
		id, issued.GrantID).Scan(t.Context(), &same))
	require.True(t, same)

	school := f.school(t)
	require.ErrorContains(t, issueRelease(t.Context(), f.db, f.operator, school, strings.Repeat("3", 32), time.Now().AddDate(2, 0, 0)), "within a year")
}

// An erasure that has begun refuses a release like one that is done: a
// school with an erasure order, a Träger whose order has started. A Träger
// order still in its grace period does not block.
func TestReleaseRefusesASubjectWhoseErasureHasBegun(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	school := f.school(t)
	var organization int64
	require.NoError(t, f.db.NewRaw(`SELECT organization_id FROM platform.schools WHERE id = ?`, school).Scan(t.Context(), &organization))
	keep := time.Now().Add(30 * 24 * time.Hour)
	hex64 := strings.Repeat("ab", 32)
	releaseOrganization := func(archive string) error {
		_, err := f.db.NewRaw(`SELECT platform.data_return_issue(?, 'release', 'organization', ?, ?, 10, ?, ?, 1, ?, 'customer', 'Träger', ?, '\x01'::bytea, ?, ?, 1)`,
			f.operator, organization, archive, hex64, hex64, hex64, strings.Repeat("ef", 16)+archive, time.Now().Add(time.Hour), keep).Exec(t.Context())
		return err
	}
	t.Cleanup(func() {
		for _, statement := range []string{
			`DELETE FROM platform.data_return_grants WHERE package = 'organization' AND subject_id = ?`,
			`DELETE FROM platform.organization_erasure_orders WHERE organization_id = ?`,
		} {
			_, err := f.db.ExecContext(context.Background(), statement, organization)
			require.NoError(t, err)
		}
		_, err := f.db.ExecContext(context.Background(), `DELETE FROM platform.school_erasures WHERE school_id = ?`, school)
		require.NoError(t, err)
	})

	_, err := f.db.ExecContext(t.Context(), `INSERT INTO platform.organization_erasure_orders (organization_id, order_reference, confirmed_on, erase_not_before, recorded_by)
		VALUES (?, 'ORDER-6550', current_date, current_date + 30, ?)`, organization, f.operator)
	require.NoError(t, err)
	require.NoError(t, releaseOrganization(strings.Repeat("4", 32)), "a recorded order in its grace period does not block")
	_, err = f.db.ExecContext(t.Context(), `UPDATE platform.organization_erasure_orders SET started_at = now(), records_notice_shown_at = now(),
		records_choice = 'erasure_chosen' WHERE organization_id = ?`, organization)
	require.NoError(t, err)
	require.ErrorContains(t, releaseOrganization(strings.Repeat("5", 32)), "erasure of the school or Träger has begun")

	_, err = f.db.ExecContext(t.Context(), `INSERT INTO platform.school_erasures (school_id, organization_id, operator_id, order_reference) VALUES (?, ?, ?, 'ORDER-6550')`,
		school, organization, f.operator)
	require.NoError(t, err)
	require.ErrorContains(t, issueRelease(t.Context(), f.db, f.operator, school, strings.Repeat("6", 32), keep), "erasure of the school or Träger has begun")
}

// lockedOut runs statement on tx, which has to wait for a lock that holder
// holds; it returns the statement's result after holder committed.
func lockedOut(t *testing.T, db *bun.DB, tx, holder bun.Tx, statement string, args ...any) error {
	t.Helper()
	var pid int
	require.NoError(t, tx.NewRaw(`SELECT pg_backend_pid()`).Scan(t.Context(), &pid))
	done := make(chan error, 1)
	go func() {
		_, err := tx.ExecContext(context.Background(), statement, args...)
		done <- err
	}()
	require.Eventually(t, func() bool {
		var waiting bool
		err := db.NewRaw(`SELECT wait_event_type = 'Lock' FROM pg_stat_activity WHERE pid = ?`, pid).Scan(context.Background(), &waiting)
		return err == nil && waiting
	}, 10*time.Second, 20*time.Millisecond, "the statement waits for the other transaction")
	require.NoError(t, holder.Commit())
	return <-done
}

// The release of a Träger package and the start of the Träger's erasure
// exclude each other in both orders (#6665): whichever commits first, the
// other waits for it and is refused. An erasure never starts with a
// retrievable package, and a started erasure never gets one.
func TestReleaseAndStartOfTheTraegerErasureAreSerialized(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	const begin = `SELECT platform.begin_organization_erasure(?, ?, 'ORDER-6665')`
	const release = `SELECT platform.data_return_issue(?, 'release', 'organization', ?, ?, 10, ?, ?, 1, ?, 'customer', 'Träger', ?, '\x01'::bytea, ?, ?, 1)`
	hex64 := strings.Repeat("ab", 32)
	// dueOrder deactivates a new Träger and records its due erasure order.
	dueOrder := func(t *testing.T) (organization int64, releaseArgs []any) {
		t.Helper()
		require.NoError(t, f.db.NewRaw(`SELECT organization_id FROM platform.schools WHERE id = ?`, f.school(t)).Scan(t.Context(), &organization))
		t.Cleanup(func() {
			for _, statement := range []string{
				`DELETE FROM platform.data_return_grants WHERE package = 'organization' AND subject_id = ?`,
				`DELETE FROM platform.organization_erasure_orders WHERE organization_id = ?`,
			} {
				_, err := f.db.ExecContext(context.Background(), statement, organization)
				require.NoError(t, err)
			}
		})
		_, err := f.db.ExecContext(t.Context(), `UPDATE platform.organizations SET active = false WHERE id = ?`, organization)
		require.NoError(t, err)
		_, err = f.db.ExecContext(t.Context(), `INSERT INTO platform.organization_erasure_orders (organization_id, order_reference, confirmed_on,
			erase_not_before, recorded_by, records_notice_shown_at, records_choice)
			VALUES (?, 'ORDER-6665', '2026-01-02', '2026-01-03', ?, now(), 'erasure_chosen')`, organization, f.operator)
		require.NoError(t, err)
		archive := fmt.Sprintf("%032x", organization)
		return organization, []any{f.operator, organization, archive, hex64, hex64, hex64, strings.Repeat("ef", 16) + archive,
			time.Now().Add(time.Hour), time.Now().Add(30 * 24 * time.Hour)}
	}
	state := func(t *testing.T, organization int64) (started bool, packages int) {
		t.Helper()
		require.NoError(t, f.db.NewRaw(`SELECT started_at IS NOT NULL, platform.data_return_open_for('organization', organization_id)
			FROM platform.organization_erasure_orders WHERE organization_id = ?`, organization).Scan(t.Context(), &started, &packages))
		return started, packages
	}
	transactions := func(t *testing.T) (first, second bun.Tx) {
		t.Helper()
		first, err := f.db.BeginTx(t.Context(), nil)
		require.NoError(t, err)
		second, err = f.db.BeginTx(t.Context(), nil)
		require.NoError(t, err)
		t.Cleanup(func() { _, _ = first.Rollback(), second.Rollback() })
		return first, second
	}

	t.Run("the release commits first", func(t *testing.T) {
		t.Parallel()
		organization, releaseArgs := dueOrder(t)
		releasing, beginning := transactions(t)
		_, err := releasing.ExecContext(t.Context(), release, releaseArgs...)
		require.NoError(t, err, "an order that has not started does not block the release")
		err = lockedOut(t, f.db, beginning, releasing, begin, organization, f.operator)
		require.ErrorContains(t, err, "released return package that is not purged")
		require.NoError(t, beginning.Rollback())
		started, packages := state(t, organization)
		require.False(t, started, "the erasure does not start while the package can be retrieved")
		require.Equal(t, 1, packages)
	})

	t.Run("the erasure starts first", func(t *testing.T) {
		t.Parallel()
		organization, releaseArgs := dueOrder(t)
		beginning, releasing := transactions(t)
		_, err := beginning.ExecContext(t.Context(), begin, organization, f.operator)
		require.NoError(t, err)
		err = lockedOut(t, f.db, releasing, beginning, release, releaseArgs...)
		require.ErrorContains(t, err, "erasure of the school or Träger has begun")
		require.NoError(t, releasing.Rollback())
		started, packages := state(t, organization)
		require.True(t, started)
		require.Zero(t, packages, "a started erasure gets no package")
	})
}

// A revocation records who ended the token: here an operator.
func TestRevocationRecordsItsOrigin(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	issued := f.releaseSchool(t, f.school(t), "x", 1)
	released, err := f.release.store.released(t.Context(), f.operator, issued.ArchiveName)
	require.NoError(t, err)
	require.True(t, released)
	released, err = f.release.store.released(t.Context(), f.operator, strings.Repeat("7", 32))
	require.NoError(t, err)
	require.False(t, released)
	_, err = f.release.Revoke(t.Context(), organizationtenancy.DataReturnRevokeRequest{GrantID: issued.GrantID, OperatorID: f.operator})
	require.NoError(t, err)
	grants, err := f.release.Grants(t.Context(), f.operator)
	require.NoError(t, err)
	for _, grant := range grants {
		if grant.GrantID == issued.GrantID {
			require.NotNil(t, grant.RevokedBy)
			require.Equal(t, "operator", *grant.RevokedBy)
			return
		}
	}
	t.Fatal("revoked grant not listed")
}
