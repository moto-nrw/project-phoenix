package config

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	configModel "github.com/moto-nrw/project-phoenix/models/config"
	configSvc "github.com/moto-nrw/project-phoenix/services/config"
	enrollmentSvc "github.com/moto-nrw/project-phoenix/services/enrollment"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderSettingsError_NotFound(t *testing.T) {
	err := &configSvc.SettingsError{
		Op:  "resolve",
		Err: &configSvc.DefinitionNotFoundError{Key: "bad.key"},
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/test", nil)

	renderSettingsError(w, r, err)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestRenderSettingsError_InvalidValue(t *testing.T) {
	err := &configSvc.SettingsError{
		Op:  "set_value",
		Err: &configSvc.InvalidValueError{Key: "test", Reason: "too small"},
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPut, "/test", nil)

	renderSettingsError(w, r, err)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRenderSettingsError_GenericSettingsError(t *testing.T) {
	err := &configSvc.SettingsError{
		Op:  "set_value",
		Err: assert.AnError,
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPut, "/test", nil)

	renderSettingsError(w, r, err)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestRenderSettingsError_NonSettingsError(t *testing.T) {
	err := assert.AnError

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/test", nil)

	renderSettingsError(w, r, err)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestRenderSettingsError_PermissionDenied(t *testing.T) {
	err := &configSvc.SettingsError{
		Op:  "set_value",
		Err: &configSvc.PermissionDeniedError{Key: "admin.setting", RequiredPermission: "config:manage"},
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPut, "/test", nil)

	renderSettingsError(w, r, err)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestNewSettingsResource(t *testing.T) {
	res := NewSettingsResource(nil, nil, nil)
	assert.NotNil(t, res)
}

func TestSettingsRouter_ReturnsRouter(t *testing.T) {
	res := NewSettingsResource(nil, nil, nil)
	router := res.SettingsRouter()
	assert.NotNil(t, router)
}

func TestGuardDirectManagedSettingWrite_BlocksAGBDocumentURL(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPut, "/test", nil)

	blocked := guardDirectManagedSettingWrite(w, r, configModel.KeyEnrollmentLegalAGBDocumentURL)

	assert.True(t, blocked)
	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestGuardDirectManagedSettingWrite_AllowsRegularSettings(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPut, "/test", nil)

	blocked := guardDirectManagedSettingWrite(w, r, configModel.KeyEnrollmentLegalAGBText)

	assert.False(t, blocked)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestCanDeleteEnrollmentLegalAGBDocument_BlocksActivePDFMode(t *testing.T) {
	settings := legalAGBDeleteSettingsStub{
		bools: map[string]bool{
			configModel.KeyEnrollmentLegalTermsEnabled: true,
		},
		strings: map[string]string{
			configModel.KeyEnrollmentLegalAGBDisplayMode: configModel.EnrollmentLegalAGBDisplayModePDF,
		},
	}

	err := canDeleteEnrollmentLegalAGBDocument(context.Background(), settings)

	require.Error(t, err)
	assert.True(t, errors.Is(err, errCannotDeleteActiveLegalAGBPDF))
}

func TestCanDeleteEnrollmentLegalAGBDocument_AllowsInactiveTerms(t *testing.T) {
	settings := legalAGBDeleteSettingsStub{
		bools: map[string]bool{
			configModel.KeyEnrollmentLegalTermsEnabled: false,
		},
		strings: map[string]string{
			configModel.KeyEnrollmentLegalAGBDisplayMode: configModel.EnrollmentLegalAGBDisplayModePDF,
		},
	}

	err := canDeleteEnrollmentLegalAGBDocument(context.Background(), settings)

	require.NoError(t, err)
}

func TestCanDeleteEnrollmentLegalAGBDocument_AllowsTextMode(t *testing.T) {
	settings := legalAGBDeleteSettingsStub{
		bools: map[string]bool{
			configModel.KeyEnrollmentLegalTermsEnabled: true,
		},
		strings: map[string]string{
			configModel.KeyEnrollmentLegalAGBDisplayMode: configModel.EnrollmentLegalAGBDisplayModeText,
		},
	}

	err := canDeleteEnrollmentLegalAGBDocument(context.Background(), settings)

	require.NoError(t, err)
}

func TestPrepareEnrollmentLegalAGBDocumentCleanup_ReplacementDeletesUnreferencedAfterCommit(t *testing.T) {
	ctx, drain := tenant.WithAfterCommitHooksForTest(context.Background())
	oldURL := legalAGBDocumentPrefix + "1_old.pdf"
	newURL := legalAGBDocumentPrefix + "1_new.pdf"
	var removedPath string

	cleanup, err := prepareEnrollmentLegalAGBDocumentCleanup(
		ctx,
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
	tenant.RegisterAfterCommit(ctx, cleanup)
	assert.Empty(t, removedPath)

	drain()

	assert.Equal(t, "/tmp/tenant-1-old.pdf", removedPath)
}

func TestPrepareEnrollmentLegalAGBDocumentCleanup_DeleteDeletesUnreferencedAfterCommit(t *testing.T) {
	ctx, drain := tenant.WithAfterCommitHooksForTest(context.Background())
	oldURL := legalAGBDocumentPrefix + "1_old.pdf"
	removed := false

	cleanup, err := prepareEnrollmentLegalAGBDocumentCleanup(
		ctx,
		1,
		oldURL,
		"",
		func(context.Context, string) (bool, error) { return false, nil },
		func(string, string, string) (string, error) { return "/tmp/tenant-1-old.pdf", nil },
		func(string) { removed = true },
	)

	require.NoError(t, err)
	require.NotNil(t, cleanup)
	tenant.RegisterAfterCommit(ctx, cleanup)

	drain()

	assert.True(t, removed)
}

func TestPrepareEnrollmentLegalAGBDocumentCleanup_KeepsReferencedDocument(t *testing.T) {
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
	ctx, _ := tenant.WithAfterCommitHooksForTest(context.Background())
	oldURL := legalAGBDocumentPrefix + "1_old.pdf"
	removeCalled := false

	cleanup, err := prepareEnrollmentLegalAGBDocumentCleanup(
		ctx,
		1,
		oldURL,
		legalAGBDocumentPrefix+"1_new.pdf",
		func(context.Context, string) (bool, error) { return false, nil },
		func(string, string, string) (string, error) { return "/tmp/tenant-1-old.pdf", nil },
		func(string) { removeCalled = true },
	)

	require.NoError(t, err)
	require.NotNil(t, cleanup)
	tenant.RegisterAfterCommit(ctx, cleanup)

	assert.False(t, removeCalled)
}

func TestPrepareEnrollmentLegalAGBDocumentCleanup_IgnoresInvalidStoredURL(t *testing.T) {
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

func TestPublicEnrollmentLegalDocumentURL_RewritesStoredUploadURL(t *testing.T) {
	got := enrollmentSvc.PublicEnrollmentLegalDocumentURL(legalAGBDocumentPrefix + "tenant-1.pdf")

	assert.Equal(t, "/api/public/enrollment-legal-documents/tenant-1.pdf", got)
}

type legalAGBDeleteSettingsStub struct {
	bools   map[string]bool
	strings map[string]string
}

func (s legalAGBDeleteSettingsStub) ResolveBool(_ context.Context, key string) (bool, error) {
	return s.bools[key], nil
}

func (s legalAGBDeleteSettingsStub) ResolveString(_ context.Context, key string) (string, error) {
	return s.strings[key], nil
}
