package config

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	configModel "github.com/moto-nrw/project-phoenix/models/config"
	configSvc "github.com/moto-nrw/project-phoenix/services/config"
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
