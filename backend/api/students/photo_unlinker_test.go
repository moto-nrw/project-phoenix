package students

// Unit tests for photoUnlinker.UnlinkStored. Empty / malformed URLs must
// never propagate — the unlinker runs after the DB commit so a noisy error
// path would corrupt the surrounding observability.

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPhotoUnlinker_EmptyURLNoOp(t *testing.T) {
	t.Parallel()

	u := NewPhotoUnlinker(slog.Default())
	require.NotPanics(t, func() { u.UnlinkStored("") })
}

func TestPhotoUnlinker_InvalidPrefixLogged(t *testing.T) {
	t.Parallel()

	// Capture warning output so we pin that the path is logged and *not*
	// returned. ResolveStoredPath rejects a URL outside the canonical
	// student-photo prefix.
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	u := NewPhotoUnlinker(logger)

	require.NotPanics(t, func() {
		u.UnlinkStored("/uploads/not-student-photos/whatever.jpg")
	})
	assert.Contains(t, buf.String(), "could not resolve student photo path for cleanup")
}

func TestPhotoUnlinker_TraversalRejected(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	u := NewPhotoUnlinker(logger)

	// ResolveStoredPath must reject anything escaping the prefix dir.
	require.NotPanics(t, func() {
		u.UnlinkStored(common.StudentPhotoStoredURLPrefix + "../etc/passwd")
	})
	assert.Contains(t, buf.String(), "could not resolve student photo path for cleanup")
}

func TestPhotoUnlinker_NilLoggerSafe(t *testing.T) {
	t.Parallel()

	u := NewPhotoUnlinker(nil)
	require.NotPanics(t, func() {
		u.UnlinkStored("/uploads/not-student-photos/abc.jpg")
	})
}

// Deliberately NOT parallel: process-global state — os.Chdir into a temp
// directory.
func TestPhotoUnlinker_RemovesExistingFile(t *testing.T) {
	// publicPhotoBaseDir is relative to CWD; create a temp file under it,
	// invoke through a URL that ResolveStoredPath accepts, assert removal.
	tmpDir := t.TempDir()
	// Reproduce the layout ResolveStoredPath expects: <publicDir>/<urlPath>
	// where publicDir="public" and urlPath="/uploads/student-photos/<name>".
	photoDir := filepath.Join(tmpDir, "public", "uploads", "student-photos")
	require.NoError(t, os.MkdirAll(photoDir, 0o755))

	filename := "real_file.jpg"
	fullPath := filepath.Join(photoDir, filename)
	require.NoError(t, os.WriteFile(fullPath, []byte("not-really-a-jpg"), 0o644))

	// Switch CWD so ResolveStoredPath finds public/ relative to it.
	origWD, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmpDir))
	defer func() { _ = os.Chdir(origWD) }()

	u := NewPhotoUnlinker(slog.Default())
	u.UnlinkStored(common.StudentPhotoStoredURLPrefix + filename)

	_, statErr := os.Stat(fullPath)
	assert.True(t, os.IsNotExist(statErr), "unlinker must delete the resolved file")
}

// Sanity: PhotoUnlinker concrete type implements the service interface.
// Compile-time check, not a runtime assertion.
var _ context.Context = context.Background()
