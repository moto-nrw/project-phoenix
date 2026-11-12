package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/organizationtenancy"
	organizationCompose "github.com/moto-ogs/project-phoenix/modules/organizationtenancy/compose"
	"github.com/moto-ogs/project-phoenix/roots/owners"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// returnPackage writes a COMPLETE school package of one school.
func returnPackage(t *testing.T, school int64) string {
	t.Helper()
	return subjectReturnPackage(t, "", "platform.schools.jsonl", school)
}

// organizationReturnPackage writes a COMPLETE Träger package of one Träger.
func organizationReturnPackage(t *testing.T, organization int64) string {
	t.Helper()
	return subjectReturnPackage(t, "organization", "platform.organizations.jsonl", organization)
}

func subjectReturnPackage(t *testing.T, kind, subjectTable string, subject int64) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "package")
	require.NoError(t, os.MkdirAll(dir, 0o700))
	files := map[string][]byte{
		subjectTable:          fmt.Appendf(nil, "{\"id\":%d}\n", subject),
		"users.persons.jsonl": []byte("{\"id\":1,\"first_name\":\"Synthetic CLI\"}\n"),
	}
	manifest := organizationtenancy.ExportManifest{Package: kind, FormatVersion: organizationtenancy.ExportFormatVersion, CatalogVersion: 1}
	for name, data := range files {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), data, 0o600))
		manifest.Artifacts = append(manifest.Artifacts, organizationtenancy.ExportArtifact{Path: name, SHA256: testpkg.ContentFingerprint(data), Bytes: int64(len(data))})
	}
	data, err := json.Marshal(manifest)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "manifest.json"), data, 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "COMPLETE"), []byte(testpkg.ContentFingerprint(data)+"\n"), 0o600))
	return dir
}

// The documented operator flow with a real operator: release with the CLI
// as phoenix_data_return, a further grant from the secrets file,
// revoke --all, purge. The HTTP side is TestDataReturnOverHTTP in api.
func TestTenantReturnCLIKeepsSecretsOffStdout(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	operator := testpkg.CreateTestOperator(t, db)
	t.Cleanup(func() {
		_, err := db.ExecContext(context.Background(), `DELETE FROM platform.data_return_grants WHERE issued_by = ?`, operator.ID)
		require.NoError(t, err)
	})
	store := t.TempDir()
	open := func(context.Context) (organizationtenancy.DataReturnRelease, func(), error) {
		release, err := organizationCompose.NewDataReturnRelease(db, owners.NewDataReturnCrypto(), store)
		return release, func() {}, err
	}
	run := func(args ...string) (string, error) {
		command := newTenantReturnCommandWith(open)
		var stdout bytes.Buffer
		command.SetOut(&stdout)
		command.SetErr(io.Discard)
		command.SetArgs(args)
		err := command.ExecuteContext(t.Context())
		return stdout.String(), err
	}
	school, _ := testpkg.CreateTestTenant(t, db)
	t.Cleanup(func() {
		_, err := db.ExecContext(context.Background(), `DELETE FROM platform.data_return_grants WHERE subject_id = ?`, school)
		require.NoError(t, err)
	})
	secrets := t.TempDir()
	require.NoError(t, os.Chmod(secrets, 0o700))
	keepUntil := time.Now().AddDate(0, 0, 40).Format("2006-01-02")
	releaseArgs := func(out string) []string {
		return []string{"release", "--package", returnPackage(t, school), "--school", strconv.FormatInt(school, 10),
			"--operator", strconv.FormatInt(operator.ID, 10), "--keep-until", keepUntil, "--recipient", "customer",
			"--recipient-name", "Schule am Berg", "--secrets-out", out}
	}

	// Never standard output, never an existing file, never a shared directory.
	open0755 := filepath.Join(t.TempDir(), "shared")
	require.NoError(t, os.Mkdir(open0755, 0o755))
	for _, target := range []string{"-", "/dev/stdout", filepath.Join(open0755, "s.json")} {
		_, err := run(releaseArgs(target)...)
		require.Error(t, err, target)
	}

	out, err := run(releaseArgs(filepath.Join(secrets, "release.json"))...)
	require.NoError(t, err)
	var issued issuedSecrets
	data, err := os.ReadFile(filepath.Join(secrets, "release.json"))
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(data, &issued))
	info, err := os.Stat(filepath.Join(secrets, "release.json"))
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
	require.True(t, strings.HasPrefix(issued.Token, "mdr_"))
	require.NotEmpty(t, issued.ArchiveKey)
	require.NotContains(t, out, "mdr_", "no token on stdout, which journald logs")
	require.NotContains(t, out, issued.Token[4:])
	require.NotContains(t, out, issued.ArchiveKey)
	_, err = run(releaseArgs(filepath.Join(secrets, "release.json"))...)
	require.ErrorContains(t, err, "exists")

	retrieval, err := organizationCompose.NewDataReturn(db, owners.NewDataReturnCrypto(), store, nil)
	require.NoError(t, err)
	opens := func(token string) bool {
		_, err := retrieval.Package(t.Context(), token)
		return err == nil
	}
	require.True(t, opens(issued.Token))

	// A further grant takes the archive key from the release's secrets file.
	_, err = run("grant", "--archive", issued.Archive, "--operator", strconv.FormatInt(operator.ID, 10),
		"--archive-key-file", filepath.Join(secrets, "release.json"), "--recipient", "target_provider",
		"--recipient-name", "Neuer Anbieter GmbH", "--secrets-out", filepath.Join(secrets, "grant.json"))
	require.NoError(t, err)
	var granted issuedSecrets
	data, err = os.ReadFile(filepath.Join(secrets, "grant.json"))
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(data, &granted))
	require.Empty(t, granted.ArchiveKey, "a grant does not repeat the archive key")
	require.True(t, opens(granted.Token))

	out, err = run("revoke", "--all", "--operator", strconv.FormatInt(operator.ID, 10))
	require.NoError(t, err)
	require.Contains(t, out, `"revoked"`)
	require.False(t, opens(issued.Token))
	require.False(t, opens(granted.Token))
	out, err = run("purge", "--operator", strconv.FormatInt(operator.ID, 10))
	require.NoError(t, err)
	require.Contains(t, out, `"archives"`)
	require.NoFileExists(t, filepath.Join(store, issued.Archive+".mdr"))

	var actions []string
	require.NoError(t, db.NewRaw(`SELECT action FROM platform.operator_audit_log WHERE operator_id = ? ORDER BY id`, operator.ID).Scan(t.Context(), &actions))
	require.Contains(t, actions, "data_return_release")
	require.Contains(t, actions, "data_return_grant")
	require.Contains(t, actions, "data_return_revoke")
	require.Contains(t, actions, "data_return_purge")
}
