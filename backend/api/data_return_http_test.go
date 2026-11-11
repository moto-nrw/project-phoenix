package api

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	organizationCompose "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/compose"
	"github.com/moto-nrw/project-phoenix/roots/owners"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// The retrieval as the API mounts it, over a real HTTP server on
// 127.0.0.1: a released package comes back byte for byte to its bearer
// token, and stops answering once the token is revoked (ADR 0203).
func TestDataReturnOverHTTP(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	operator := testpkg.CreateTestOperator(t, db)
	t.Cleanup(func() {
		_, err := db.ExecContext(context.Background(), `DELETE FROM platform.data_return_grants WHERE issued_by = ?`, operator.ID)
		require.NoError(t, err)
	})
	school := testpkg.UniqueTestTenantID(t)
	dir := filepath.Join(t.TempDir(), "package")
	require.NoError(t, os.MkdirAll(dir, 0o700))
	rows := fmt.Appendf(nil, "{\"id\":%d}\n", school)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "platform.schools.jsonl"), rows, 0o600))
	manifest, err := json.Marshal(organizationtenancy.ExportManifest{FormatVersion: organizationtenancy.ExportFormatVersion,
		Artifacts: []organizationtenancy.ExportArtifact{{Path: "platform.schools.jsonl", SHA256: testpkg.ContentFingerprint(rows), Bytes: int64(len(rows))}}})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "manifest.json"), manifest, 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "COMPLETE"), []byte(testpkg.ContentFingerprint(manifest)+"\n"), 0o600))

	store := t.TempDir()
	release, err := organizationCompose.NewDataReturnRelease(db, owners.NewDataReturnCrypto(), store)
	require.NoError(t, err)
	issued, err := release.Release(t.Context(), organizationtenancy.DataReturnReleaseRequest{Package: dir, SchoolID: school, OperatorID: operator.ID,
		KeepUntil: time.Now().AddDate(0, 0, 40), DataReturnRecipient: organizationtenancy.DataReturnRecipient{
			Recipient: organizationtenancy.DataReturnTargetProvider, Name: "Neuer Anbieter GmbH", ValidFor: 24 * time.Hour, MaxDownloads: 2}})
	require.NoError(t, err)
	packages, err := organizationCompose.NewDataReturn(db, owners.NewDataReturnCrypto(), store, nil)
	require.NoError(t, err)
	router := chi.NewRouter()
	mountDataReturn(router, packages, nil)
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)

	get := func(path, token string) (*http.Response, []byte) {
		request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server.URL+"/public/data-return"+path, nil)
		require.NoError(t, err)
		if token != "" {
			request.Header.Set("Authorization", "Bearer "+token)
		}
		response, err := server.Client().Do(request)
		require.NoError(t, err)
		defer func() { _ = response.Body.Close() }()
		body, err := io.ReadAll(response.Body)
		require.NoError(t, err)
		return response, body
	}
	response, _ := get("/register", "")
	require.Equal(t, http.StatusOK, response.StatusCode)
	response, _ = get("/package", "")
	require.Equal(t, http.StatusUnauthorized, response.StatusCode)
	response, body := get("/package", issued.Token)
	require.Equal(t, http.StatusOK, response.StatusCode)
	var described organizationtenancy.DataReturnPackage
	require.NoError(t, json.Unmarshal(body, &described))
	require.Equal(t, "target_provider", described.Recipient)

	response, archive := get("/package/archive", issued.Token)
	require.Equal(t, http.StatusOK, response.StatusCode)
	digest, err := hex.DecodeString(described.ArchiveSHA256)
	require.NoError(t, err)
	require.Equal(t, "sha-256=:"+base64.StdEncoding.EncodeToString(digest)+":", response.Header.Get("Repr-Digest"))
	require.Equal(t, described.ArchiveSHA256, testpkg.ContentFingerprint(archive))
	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	require.NoError(t, err)
	require.Len(t, reader.File, 3)

	_, err = release.Revoke(t.Context(), organizationtenancy.DataReturnRevokeRequest{GrantID: issued.GrantID, OperatorID: operator.ID})
	require.NoError(t, err)
	response, _ = get("/package/archive", issued.Token)
	require.Equal(t, http.StatusUnauthorized, response.StatusCode)
}
