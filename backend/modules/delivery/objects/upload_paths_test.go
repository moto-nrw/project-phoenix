package objects

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveStoredPath_Valid(t *testing.T) {
	t.Parallel()

	path, err := ResolveStoredPath("public", "/uploads/avatars/global/test.jpg", "/uploads/avatars/")
	assert.NoError(t, err)
	assert.NotEmpty(t, path)
}

func TestResolveStoredPath_InvalidPrefix(t *testing.T) {
	t.Parallel()

	_, err := ResolveStoredPath("public", "/uploads/not-avatars/test.jpg", "/uploads/avatars/")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid path")
}

func TestResolveStoredPath_Traversal(t *testing.T) {
	t.Parallel()

	_, err := ResolveStoredPath("public", "/uploads/avatars/../../etc/passwd", "/uploads/avatars/")
	assert.Error(t, err)
}

func TestRemoveImage_ExistingFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	filePath := filepath.Join(dir, "to_delete.jpg")
	require.NoError(t, os.WriteFile(filePath, []byte("data"), 0644))

	(UploadFiles{}).RemoveUpload(filePath)

	_, err := os.Stat(filePath)
	assert.True(t, os.IsNotExist(err), "file should be deleted")
}

func TestRemoveImage_NonExistent(t *testing.T) {
	t.Parallel()

	// Should not panic when file does not exist
	assert.NotPanics(t, func() {
		(UploadFiles{}).RemoveUpload("/tmp/nonexistent_file_abc123.jpg")
	})
}

func TestRemoveImage_EmptyPath(t *testing.T) {
	t.Parallel()

	// Should be a no-op, no panic
	assert.NotPanics(t, func() {
		(UploadFiles{}).RemoveUpload("")
	})
}

func TestPublicDirCandidates(t *testing.T) {
	t.Parallel()

	base := "/some/project/backend"
	candidates := publicDirCandidates(base)

	assert.NotEmpty(t, candidates)

	// Should contain "public" and "backend/public" relative to base and parents
	assert.Contains(t, candidates, filepath.Join(base, "public"))
	assert.Contains(t, candidates, filepath.Join(base, "backend", "public"))

	// Should also walk up to parent
	parent := filepath.Dir(base)
	assert.Contains(t, candidates, filepath.Join(parent, "public"))
	assert.Contains(t, candidates, filepath.Join(parent, "backend", "public"))
}
