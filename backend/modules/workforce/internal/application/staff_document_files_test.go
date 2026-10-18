package application

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type publicDirUploads struct{ dir string }

func (u publicDirUploads) PublicDir() (string, error) { return u.dir, nil }

type noStaffDocuments struct{ workforce.StaffDocuments }

type noDocumentCleanupQueue struct{ workforce.DocumentCleanupQueue }

func TestNewStaffDocumentFileRecoveryRequiresEveryDependency(t *testing.T) {
	t.Parallel()
	documents, queue, uploads := noStaffDocuments{}, noDocumentCleanupQueue{}, publicDirUploads{}
	for name, build := range map[string]func() (workforce.StaffDocumentFileRecovery, error){
		"documents": func() (workforce.StaffDocumentFileRecovery, error) {
			return NewStaffDocumentFileRecovery(nil, queue, uploads, slog.Default())
		},
		"queue": func() (workforce.StaffDocumentFileRecovery, error) {
			return NewStaffDocumentFileRecovery(documents, nil, uploads, slog.Default())
		},
		"uploads": func() (workforce.StaffDocumentFileRecovery, error) {
			return NewStaffDocumentFileRecovery(documents, queue, nil, slog.Default())
		},
		"logger": func() (workforce.StaffDocumentFileRecovery, error) {
			return NewStaffDocumentFileRecovery(documents, queue, uploads, nil)
		},
	} {
		recovery, err := build()
		assert.Nil(t, recovery, name)
		assert.EqualError(t, err, "workforce staff document files: all dependencies are required", name)
	}
}

func TestStaffDocumentFileRecoveryRemovesOnlyPlainStoredNames(t *testing.T) {
	t.Parallel()
	publicDir := t.TempDir()
	recovery, err := NewStaffDocumentFileRecovery(noStaffDocuments{}, noDocumentCleanupQueue{}, publicDirUploads{dir: publicDir}, slog.Default())
	require.NoError(t, err)

	dir := filepath.Join(publicDir, "uploads", "staff-documents", "42")
	assert.Equal(t, dir, workforce.StaffDocumentsDir(42, publicDirUploads{dir: publicDir}))
	require.NoError(t, os.MkdirAll(dir, 0o755))
	stored := filepath.Join(dir, "document.pdf")
	require.NoError(t, os.WriteFile(stored, []byte("pdf"), 0o600))

	require.NoError(t, recovery.RemoveStoredDocument(42, "document.pdf"))
	_, statErr := os.Stat(stored)
	assert.True(t, os.IsNotExist(statErr), "the stored file is removed")
	require.NoError(t, recovery.RemoveStoredDocument(42, "document.pdf"), "a missing file is no error")

	for _, name := range []string{"", "../document.pdf", "nested/document.pdf"} {
		assert.EqualError(t, recovery.RemoveStoredDocument(42, name), "stored document filename failed validation on delete", name)
	}
}
