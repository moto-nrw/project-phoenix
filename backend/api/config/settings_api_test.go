package config

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPrepareEnrollmentLegalAGBDocumentCleanup_ReplacementDeletesUnreferencedAfterCommit(t *testing.T) {
	t.Parallel()

	oldURL := legalAGBDocumentPrefix + "1_old.pdf"
	newURL := legalAGBDocumentPrefix + "1_new.pdf"
	var removedPath string

	cleanup, err := prepareEnrollmentLegalAGBDocumentCleanup(
		context.Background(),
		1,
		oldURL,
		newURL,
		func(_ context.Context, gotURL string) (bool, error) {
			assert.Equal(t, oldURL, gotURL)
			return false, nil
		},
		func(publicDir, urlPath, requiredPrefix string) (string, error) {
			assert.Equal(t, "public", publicDir)
			assert.Equal(t, oldURL, urlPath)
			assert.Equal(t, legalAGBDocumentPrefix, requiredPrefix)
			return "/tmp/tenant-1-old.pdf", nil
		},
		func(path string) {
			removedPath = path
		},
	)

	require.NoError(t, err)
	require.NotNil(t, cleanup)
	assert.Empty(t, removedPath)
	cleanup()

	assert.Equal(t, "/tmp/tenant-1-old.pdf", removedPath)
}

func TestPrepareEnrollmentLegalAGBDocumentCleanup_DeleteDeletesUnreferencedAfterCommit(t *testing.T) {
	t.Parallel()

	oldURL := legalAGBDocumentPrefix + "1_old.pdf"
	removed := false

	cleanup, err := prepareEnrollmentLegalAGBDocumentCleanup(
		context.Background(),
		1,
		oldURL,
		"",
		func(context.Context, string) (bool, error) { return false, nil },
		func(string, string, string) (string, error) { return "/tmp/tenant-1-old.pdf", nil },
		func(string) { removed = true },
	)

	require.NoError(t, err)
	require.NotNil(t, cleanup)
	cleanup()

	assert.True(t, removed)
}

func TestPrepareEnrollmentLegalAGBDocumentCleanup_KeepsReferencedDocument(t *testing.T) {
	t.Parallel()

	oldURL := legalAGBDocumentPrefix + "1_old.pdf"
	removeCalled := false

	cleanup, err := prepareEnrollmentLegalAGBDocumentCleanup(
		context.Background(),
		1,
		oldURL,
		legalAGBDocumentPrefix+"1_new.pdf",
		func(context.Context, string) (bool, error) { return true, nil },
		func(string, string, string) (string, error) { return "/tmp/tenant-1-old.pdf", nil },
		func(string) { removeCalled = true },
	)

	require.NoError(t, err)
	assert.Nil(t, cleanup)
	assert.False(t, removeCalled)
}

func TestPrepareEnrollmentLegalAGBDocumentCleanup_RollbackDoesNotRemoveDocument(t *testing.T) {
	t.Parallel()

	oldURL := legalAGBDocumentPrefix + "1_old.pdf"
	removeCalled := false

	cleanup, err := prepareEnrollmentLegalAGBDocumentCleanup(
		context.Background(),
		1,
		oldURL,
		legalAGBDocumentPrefix+"1_new.pdf",
		func(context.Context, string) (bool, error) { return false, nil },
		func(string, string, string) (string, error) { return "/tmp/tenant-1-old.pdf", nil },
		func(string) { removeCalled = true },
	)

	require.NoError(t, err)
	require.NotNil(t, cleanup)
	assert.False(t, removeCalled)
}

func TestPrepareEnrollmentLegalAGBDocumentCleanup_IgnoresInvalidStoredURL(t *testing.T) {
	t.Parallel()

	referenceChecked := false

	cleanup, err := prepareEnrollmentLegalAGBDocumentCleanup(
		context.Background(),
		1,
		"/uploads/not-legal/tenant-1-old.pdf",
		"",
		func(context.Context, string) (bool, error) {
			referenceChecked = true
			return false, nil
		},
		func(string, string, string) (string, error) { return "", errors.New("invalid path") },
		func(string) { t.Fatal("remove must not be called") },
	)

	require.NoError(t, err)
	assert.Nil(t, cleanup)
	assert.False(t, referenceChecked)
}

func TestPrepareEnrollmentLegalAGBDocumentCleanup_IgnoresOtherTenantDocument(t *testing.T) {
	t.Parallel()

	referenceChecked := false

	cleanup, err := prepareEnrollmentLegalAGBDocumentCleanup(
		context.Background(),
		1,
		legalAGBDocumentPrefix+"2_old.pdf",
		"",
		func(context.Context, string) (bool, error) {
			referenceChecked = true
			return false, nil
		},
		func(string, string, string) (string, error) {
			t.Fatal("resolve must not be called for another tenant document")
			return "", nil
		},
		func(string) { t.Fatal("remove must not be called") },
	)

	require.NoError(t, err)
	assert.Nil(t, cleanup)
	assert.False(t, referenceChecked)
}

func TestPrepareEnrollmentLegalAGBDocumentCleanup_PropagatesReferenceCheckError(t *testing.T) {
	t.Parallel()

	expected := errors.New("db failed")

	cleanup, err := prepareEnrollmentLegalAGBDocumentCleanup(
		context.Background(),
		1,
		legalAGBDocumentPrefix+"1_old.pdf",
		"",
		func(context.Context, string) (bool, error) { return false, expected },
		func(string, string, string) (string, error) { return "/tmp/tenant-1-old.pdf", nil },
		func(string) { t.Fatal("remove must not be called") },
	)

	require.ErrorIs(t, err, expected)
	assert.Nil(t, cleanup)
}
