package objects

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTenantErasureRejectsSymlinksAndMissingRoot(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	outside := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(outside, "keep"), []byte("synthetic"), 0600))
	require.NoError(t, os.Symlink(outside, filepath.Join(root, "files")))
	// IDs are opaque namespace keys here, not database fixtures.
	var namespaceID int64 = 73
	_, err := NewLocal(root).TenantObjects(t.Context(), "files", "tenantDir", namespaceID, true)
	require.Error(t, err)
	data, err := os.ReadFile(filepath.Join(outside, "keep"))
	require.NoError(t, err)
	require.Equal(t, "synthetic", string(data))
	_, err = NewLocal(filepath.Join(root, "missing")).TenantObjects(t.Context(), "files", "tenantDir", namespaceID, true)
	require.Error(t, err)
	require.NoError(t, os.MkdirAll(filepath.Join(root, "student-photos"), 0700))
	require.NoError(t, os.Symlink(filepath.Join(outside, "keep"), filepath.Join(root, "student-photos", fmt.Sprintf("%d_link", namespaceID))))
	_, err = NewLocal(root).TenantObjects(t.Context(), "student-photos", "tenantPrefix", namespaceID, false)
	require.Error(t, err)
}

func TestTenantErasureRemovesTheEmptiedTenantDirectory(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// IDs are opaque namespace keys here, not database fixtures.
	var namespaceID int64 = 74
	tenantDir := filepath.Join(root, "files", fmt.Sprint(namespaceID))
	require.NoError(t, os.MkdirAll(filepath.Join(tenantDir, "nested"), 0700))
	require.NoError(t, os.WriteFile(filepath.Join(tenantDir, "nested", "object"), []byte("synthetic"), 0600))
	neighbour := filepath.Join(root, "files", fmt.Sprintf("%d0", namespaceID), "object")
	require.NoError(t, os.MkdirAll(filepath.Dir(neighbour), 0700))
	require.NoError(t, os.WriteFile(neighbour, []byte("synthetic"), 0600))

	count, err := NewLocal(root).TenantObjects(t.Context(), "files", "tenantDir", namespaceID, false)
	require.NoError(t, err)
	require.Equal(t, int64(1), count)
	require.DirExists(t, tenantDir, "counting removes nothing")

	count, err = NewLocal(root).TenantObjects(t.Context(), "files", "tenantDir", namespaceID, true)
	require.NoError(t, err)
	require.Equal(t, int64(1), count)
	require.NoDirExists(t, tenantDir)
	require.FileExists(t, neighbour)

	count, err = NewLocal(root).TenantObjects(t.Context(), "files", "tenantDir", namespaceID, true)
	require.NoError(t, err)
	require.Zero(t, count, "a repeated removal finds nothing")
}
