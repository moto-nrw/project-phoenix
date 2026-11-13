package compose_test

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/organizationtenancy"
	"github.com/moto-ogs/project-phoenix/modules/organizationtenancy/compose"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func dataReturnCrypto() organizationtenancy.DataReturnCrypto {
	return organizationtenancy.DataReturnCrypto{NewToken: testpkg.DataReturnNewToken, TokenLookup: testpkg.DataReturnTokenLookup,
		NewArchiveKey: testpkg.DataReturnNewArchiveKey, KeyCheck: testpkg.DataReturnKeyCheck, KeyMatches: testpkg.DataReturnKeyMatches,
		SealKey: testpkg.DataReturnSealKey, OpenKey: testpkg.DataReturnOpenKey, Encrypt: testpkg.DataReturnEncrypt,
		Decrypt: testpkg.DataReturnDecrypt, Fingerprint: testpkg.DataReturnFingerprint}
}

// The real output of tenant export goes through release and comes back,
// byte for byte, from the retrieval (ADR 0203).
func TestDataReturnReleasesARealExport(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	school := testpkg.Tenant(t)
	testpkg.CreateTestStudentForTenant(t, db, school, "Synthetic Return", "Fixture", "A")
	operator := testpkg.CreateTestOperator(t, db)
	exporter, err := compose.NewTenantExporter(db, organizationtenancy.ExportFingerprints{Bytes: testpkg.ContentFingerprint, Reader: testpkg.ReaderContentFingerprint},
		openedSealedFields{}, projectedFields{}, &exportAuditFake{})
	require.NoError(t, err)
	out := filepath.Join(t.TempDir(), "export")
	require.NoError(t, exporter.Export(context.Background(), organizationtenancy.TenantExportOptions{SchoolID: school, OperatorID: 42, Output: out, Uploads: t.TempDir()}))

	store := t.TempDir()
	release, err := compose.NewDataReturnRelease(db, dataReturnCrypto(), store)
	require.NoError(t, err)
	retrieval, err := compose.NewDataReturn(db, dataReturnCrypto(), store, nil)
	require.NoError(t, err)
	issued, err := release.Release(t.Context(), organizationtenancy.DataReturnReleaseRequest{Package: out, SchoolID: school, OperatorID: operator.ID,
		KeepUntil: time.Now().Add(40 * 24 * time.Hour), DataReturnRecipient: organizationtenancy.DataReturnRecipient{
			Recipient: organizationtenancy.DataReturnCustomer, Name: "Schule", ValidFor: 24 * time.Hour, MaxDownloads: 1}})
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err := db.ExecContext(context.Background(), `DELETE FROM platform.data_return_grants WHERE archive_name = ?`, issued.ArchiveName)
		require.NoError(t, err)
	})

	var body bytes.Buffer
	require.NoError(t, retrieval.Download(t.Context(), issued.Token, func(organizationtenancy.DataReturnPackage) io.Writer { return &body }))
	reader, err := zip.NewReader(bytes.NewReader(body.Bytes()), int64(body.Len()))
	require.NoError(t, err)
	exported := 0
	require.NoError(t, filepath.WalkDir(out, func(path string, entry os.DirEntry, err error) error {
		if err == nil && !entry.IsDir() {
			exported++
		}
		return err
	}))
	require.Len(t, reader.File, exported, "every file of the export, nothing more")
	for _, file := range reader.File {
		want, err := os.ReadFile(filepath.Join(out, filepath.FromSlash(file.Name)))
		require.NoError(t, err)
		r, err := file.Open()
		require.NoError(t, err)
		got, err := io.ReadAll(r)
		require.NoError(t, err)
		require.Equal(t, want, got, file.Name)
	}
}
