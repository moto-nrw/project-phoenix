package compose_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// dataReturnAudit accepts operator 42 and records every action.
type dataReturnAudit struct {
	mu      sync.Mutex
	records []organizationtenancy.DataReturnAuditRecord
}

func (a *dataReturnAudit) BeginDataReturn(_ context.Context, operatorID int64) error {
	if operatorID != 42 {
		return errors.New("operator is not active")
	}
	return nil
}

func (a *dataReturnAudit) RecordDataReturn(_ context.Context, record organizationtenancy.DataReturnAuditRecord) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.records = append(a.records, record)
	return nil
}

// writeReturnPackage writes a COMPLETE school package of one school: its
// platform.schools row, one person row and one stored file, as tenant
// export writes it.
func writeReturnPackage(t *testing.T, school int64, secret string, mutate func(*organizationtenancy.ExportManifest)) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "package")
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "objects"), 0o700))
	files := map[string]string{
		"platform.schools.jsonl": fmt.Sprintf("{\"id\":%d,\"name\":\"Schule\"}\n", school),
		"users.persons.jsonl":    fmt.Sprintf("{\"id\":1,\"first_name\":%q,\"iban\":\"DE89370400440532013000\"}\n", secret),
		"objects/0":              "%PDF-1.7 " + secret,
		"model.json":             "[]\n",
	}
	manifest := organizationtenancy.ExportManifest{FormatVersion: organizationtenancy.ExportFormatVersion, CatalogVersion: 1, ExportedAt: "2026-11-10T08:00:00Z"}
	for _, name := range []string{"platform.schools.jsonl", "users.persons.jsonl", "objects/0", "model.json"} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, filepath.FromSlash(name)), []byte(files[name]), 0o600))
		manifest.Artifacts = append(manifest.Artifacts, organizationtenancy.ExportArtifact{Path: name,
			SHA256: testpkg.ContentFingerprint([]byte(files[name])), Bytes: int64(len(files[name]))})
	}
	if mutate != nil {
		mutate(&manifest)
	}
	data, err := json.Marshal(manifest)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "manifest.json"), data, 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "COMPLETE"), []byte(testpkg.ContentFingerprint(data)+"\n"), 0o600))
	return dir
}

func dataReturnCrypto() organizationtenancy.DataReturnCrypto {
	return organizationtenancy.DataReturnCrypto{NewToken: testpkg.DataReturnNewToken, TokenLookup: testpkg.DataReturnTokenLookup,
		NewArchiveKey: testpkg.DataReturnNewArchiveKey, KeyCheck: testpkg.DataReturnKeyCheck, KeyMatches: testpkg.DataReturnKeyMatches,
		SealKey: testpkg.DataReturnSealKey, OpenKey: testpkg.DataReturnOpenKey, Encrypt: testpkg.DataReturnEncrypt,
		Decrypt: testpkg.DataReturnDecrypt, Fingerprint: testpkg.DataReturnFingerprint}
}

func customer(downloads int) organizationtenancy.DataReturnRecipient {
	return organizationtenancy.DataReturnRecipient{Recipient: organizationtenancy.DataReturnCustomer, Name: "Schule am Berg",
		ValidFor: 7 * 24 * time.Hour, MaxDownloads: downloads}
}

func download(t *testing.T, packages organizationtenancy.DataReturn, token string) ([]byte, organizationtenancy.DataReturnPackage, error) {
	t.Helper()
	var body bytes.Buffer
	var got organizationtenancy.DataReturnPackage
	err := packages.Download(context.Background(), token, func(pkg organizationtenancy.DataReturnPackage) io.Writer {
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

func TestDataReturnReleasesAPackageOnlyToItsToken(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	store := t.TempDir()
	audit := &dataReturnAudit{}
	release, err := compose.NewDataReturnRelease(db, dataReturnCrypto(), audit, store)
	require.NoError(t, err)
	packages, err := compose.NewDataReturn(db, dataReturnCrypto(), store, nil)
	require.NoError(t, err)
	a, b := testpkg.UniqueTestTenantID(t), testpkg.UniqueTestTenantID(t)
	keep := time.Now().Add(60 * 24 * time.Hour)

	issuedA, err := release.Release(t.Context(), organizationtenancy.DataReturnReleaseRequest{Package: writeReturnPackage(t, a, "Synthetic A", nil),
		SchoolID: a, OperatorID: 42, KeepUntil: keep, DataReturnRecipient: customer(2)})
	require.NoError(t, err)
	issuedB, err := release.Release(t.Context(), organizationtenancy.DataReturnReleaseRequest{Package: writeReturnPackage(t, b, "Synthetic B", nil),
		SchoolID: b, OperatorID: 42, KeepUntil: keep, DataReturnRecipient: customer(2)})
	require.NoError(t, err)
	require.NotEqual(t, issuedA.ArchiveName, issuedB.ArchiveName)
	// The grants are platform rows without a tenant: remove them after the test.
	t.Cleanup(func() {
		_, err := db.NewRaw(`DELETE FROM platform.data_return_grants WHERE archive_name IN (?)`,
			bun.List([]string{issuedA.ArchiveName, issuedB.ArchiveName})).Exec(context.Background())
		require.NoError(t, err)
	})
	require.True(t, strings.HasPrefix(issuedA.Token, "mdr_"))

	// At rest the archive holds no plaintext, and the database no token or key.
	stored, err := os.ReadFile(filepath.Join(store, issuedA.ArchiveName+".mdr"))
	require.NoError(t, err)
	require.NotContains(t, string(stored), "Synthetic A")
	require.NotContains(t, string(stored), "DE89370400440532013000")
	var rows []string
	require.NoError(t, db.NewRaw(`SELECT row_to_json(g)::text FROM platform.data_return_grants AS g WHERE archive_name IN (?)`,
		bun.List([]string{issuedA.ArchiveName, issuedB.ArchiveName})).Scan(t.Context(), &rows))
	require.Len(t, rows, 2)
	for _, row := range rows {
		require.NotContains(t, row, issuedA.Token[4:])
		require.NotContains(t, row, issuedB.Token[4:])
	}
	for _, record := range audit.records {
		data, _ := json.Marshal(record)
		require.NotContains(t, string(data), issuedA.Token)
	}

	described, err := packages.Package(t.Context(), issuedA.Token)
	require.NoError(t, err)
	require.Equal(t, "school", described.Package)
	require.Equal(t, 2, described.DownloadsLeft)

	archive, pkg, err := download(t, packages, issuedA.Token)
	require.NoError(t, err)
	require.Equal(t, int64(len(archive)), pkg.ArchiveBytes)
	require.Equal(t, testpkg.ContentFingerprint(archive), pkg.ArchiveSHA256)
	require.Equal(t, 1, pkg.DownloadsLeft)
	files := zipFiles(t, archive)
	require.Contains(t, files["users.persons.jsonl"], "Synthetic A")
	require.NotContains(t, strings.Join(mapValues(files), ""), "Synthetic B", "one token opens exactly its own package")
	require.Contains(t, files, "COMPLETE")
	require.Contains(t, files, "manifest.json")

	_, _, err = download(t, packages, issuedA.Token)
	require.NoError(t, err)
	_, _, err = download(t, packages, issuedA.Token)
	require.ErrorIs(t, err, organizationtenancy.ErrDataReturnDenied, "the download budget is spent")
	var outcomes []string
	require.NoError(t, db.NewRaw(`SELECT d.outcome FROM platform.data_return_downloads d WHERE d.grant_id = ? ORDER BY d.id`, issuedA.GrantID).Scan(t.Context(), &outcomes))
	require.Equal(t, []string{"completed", "completed"}, outcomes)

	for _, token := range []string{"", "mdr_unknown", issuedA.Token + "x", strings.TrimPrefix(issuedB.Token, "mdr_")} {
		_, err := packages.Package(t.Context(), token)
		require.ErrorIs(t, err, organizationtenancy.ErrDataReturnDenied, "token %q", token)
	}

	// A further grant needs the archive key of that archive.
	_, err = release.Grant(t.Context(), organizationtenancy.DataReturnGrantRequest{ArchiveName: issuedA.ArchiveName, ArchiveKey: issuedB.ArchiveKey,
		OperatorID: 42, DataReturnRecipient: customer(1)})
	require.ErrorContains(t, err, "does not belong")
	target := organizationtenancy.DataReturnRecipient{Recipient: organizationtenancy.DataReturnTargetProvider, Name: "Neuer Anbieter GmbH",
		ValidFor: 24 * time.Hour, MaxDownloads: 1}
	granted, err := release.Grant(t.Context(), organizationtenancy.DataReturnGrantRequest{ArchiveName: issuedA.ArchiveName, ArchiveKey: issuedA.ArchiveKey,
		OperatorID: 42, DataReturnRecipient: target})
	require.NoError(t, err)
	again, pkg, err := download(t, packages, granted.Token)
	require.NoError(t, err)
	require.Equal(t, "target_provider", pkg.Recipient)
	require.Equal(t, archive, again)

	// Revocation and expiry end a token at once.
	count, err := release.Revoke(t.Context(), organizationtenancy.DataReturnRevokeRequest{GrantID: issuedB.GrantID, OperatorID: 42})
	require.NoError(t, err)
	require.Equal(t, 1, count)
	_, err = packages.Package(t.Context(), issuedB.Token)
	require.ErrorIs(t, err, organizationtenancy.ErrDataReturnDenied)
	more, err := release.Grant(t.Context(), organizationtenancy.DataReturnGrantRequest{ArchiveName: issuedA.ArchiveName, ArchiveKey: issuedA.ArchiveKey,
		OperatorID: 42, DataReturnRecipient: customer(3)})
	require.NoError(t, err)
	_, err = db.NewRaw(`UPDATE platform.data_return_grants SET issued_at = now() - interval '2 days', expires_at = now() - interval '1 second' WHERE id = ?`, more.GrantID).Exec(t.Context())
	require.NoError(t, err)
	_, err = packages.Package(t.Context(), more.Token)
	require.ErrorIs(t, err, organizationtenancy.ErrDataReturnDenied)

	// The purge removes the archive whose grants are all revoked and keeps the evidence.
	purged, err := release.Purge(t.Context(), 42)
	require.NoError(t, err)
	require.GreaterOrEqual(t, purged.Archives, 1)
	require.NoFileExists(t, filepath.Join(store, issuedB.ArchiveName+".mdr"))
	require.FileExists(t, filepath.Join(store, issuedA.ArchiveName+".mdr"))
	grants, err := release.Grants(t.Context(), 42)
	require.NoError(t, err)
	var purgedB bool
	for _, grant := range grants {
		purgedB = purgedB || grant.GrantID == issuedB.GrantID && grant.PurgedAt != nil
	}
	require.True(t, purgedB)

	actions := map[string]int{}
	for _, record := range audit.records {
		actions[record.Action]++
	}
	require.Equal(t, 2, actions["release"])
	require.Equal(t, 2, actions["grant"])
	require.Equal(t, 1, actions["revoke"])
	require.Equal(t, 1, actions["purge"])
}

func mapValues(files map[string]string) []string {
	values := make([]string, 0, len(files))
	for _, value := range files {
		values = append(values, value)
	}
	return values
}

func TestDataReturnRefusesAnythingButTheCheckedPackageOfTheNamedSubject(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	store := t.TempDir()
	audit := &dataReturnAudit{}
	release, err := compose.NewDataReturnRelease(db, dataReturnCrypto(), audit, store)
	require.NoError(t, err)
	school := testpkg.UniqueTestTenantID(t)
	keep := time.Now().Add(60 * 24 * time.Hour)
	request := func(dir string, schoolID int64, operator int64) organizationtenancy.DataReturnReleaseRequest {
		return organizationtenancy.DataReturnReleaseRequest{Package: dir, SchoolID: schoolID, OperatorID: operator, KeepUntil: keep, DataReturnRecipient: customer(1)}
	}
	cases := map[string]func() organizationtenancy.DataReturnReleaseRequest{
		"another school": func() organizationtenancy.DataReturnReleaseRequest {
			return request(writeReturnPackage(t, school, "x", nil), school+1, 42)
		},
		"inactive operator": func() organizationtenancy.DataReturnReleaseRequest {
			return request(writeReturnPackage(t, school, "x", nil), school, 7)
		},
		"person review": func() organizationtenancy.DataReturnReleaseRequest {
			return request(writeReturnPackage(t, school, "x", func(m *organizationtenancy.ExportManifest) { m.ReviewRequired = true }), school, 42)
		},
		"Träger package named as school": func() organizationtenancy.DataReturnReleaseRequest {
			return request(writeReturnPackage(t, school, "x", func(m *organizationtenancy.ExportManifest) { m.Package = "organization" }), school, 42)
		},
		"no COMPLETE": func() organizationtenancy.DataReturnReleaseRequest {
			dir := writeReturnPackage(t, school, "x", nil)
			require.NoError(t, os.Remove(filepath.Join(dir, "COMPLETE")))
			return request(dir, school, 42)
		},
		"changed file": func() organizationtenancy.DataReturnReleaseRequest {
			dir := writeReturnPackage(t, school, "x", nil)
			require.NoError(t, os.WriteFile(filepath.Join(dir, "users.persons.jsonl"), []byte("{}\n"), 0o600))
			return request(dir, school, 42)
		},
		"extra file": func() organizationtenancy.DataReturnReleaseRequest {
			dir := writeReturnPackage(t, school, "x", nil)
			require.NoError(t, os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("x"), 0o600))
			return request(dir, school, 42)
		},
		"link": func() organizationtenancy.DataReturnReleaseRequest {
			dir := writeReturnPackage(t, school, "x", nil)
			require.NoError(t, os.Symlink("/etc/hosts", filepath.Join(dir, "objects", "1")))
			return request(dir, school, 42)
		},
		"token outlives the retrieval period": func() organizationtenancy.DataReturnReleaseRequest {
			r := request(writeReturnPackage(t, school, "x", nil), school, 42)
			r.KeepUntil = time.Now().Add(time.Hour)
			return r
		},
	}
	for name, build := range cases {
		_, err := release.Release(t.Context(), build())
		require.Error(t, err, name)
	}
	entries, err := os.ReadDir(store)
	require.NoError(t, err)
	require.Empty(t, entries, "a refused release leaves nothing in the store")
	require.Empty(t, audit.records)
}
