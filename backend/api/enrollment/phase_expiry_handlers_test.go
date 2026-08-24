package enrollment

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
)

type phaseExpiryServiceStub struct {
	warnings []*enrollmentService.PhaseExpiryWarning
}

func (s *phaseExpiryServiceStub) ListWarnings(
	_ context.Context,
	_ timezone.Date,
) ([]*enrollmentService.PhaseExpiryWarning, error) {
	return s.warnings, nil
}

func TestListPhaseExpiryWarnings_StringifiesIDsAndDates(t *testing.T) {
	t.Parallel()

	successorID := int64(22)
	resource := &Resource{PhaseExpiryService: &phaseExpiryServiceStub{warnings: []*enrollmentService.PhaseExpiryWarning{
		{
			SourcePhaseID:      3,
			SourcePhaseName:    "1. Halbjahr",
			SuccessorPhaseID:   &successorID,
			FirstAffectedDate:  timezone.NewDate(2027, 2, 1),
			AffectedChildren:   204,
			UnresolvedChildren: 17,
			State:              enrollmentService.PhaseExpiryStateIncomplete,
			Overdue:            false,
		},
	}}}
	router := chi.NewRouter()
	router.Use(render.SetContentType(render.ContentTypeJSON))
	router.Get("/enrollment/phases/expiry-warnings", resource.listPhaseExpiryWarnings)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/enrollment/phases/expiry-warnings", nil))
	require.Equal(t, http.StatusOK, recorder.Code)

	var response struct {
		Data []PhaseExpiryWarningResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Len(t, response.Data, 1)
	assert.Equal(t, "3", response.Data[0].SourcePhaseID)
	require.NotNil(t, response.Data[0].SuccessorPhaseID)
	assert.Equal(t, "22", *response.Data[0].SuccessorPhaseID)
	assert.Equal(t, "2027-02-01", response.Data[0].FirstAffectedDate)
	assert.Equal(t, 17, response.Data[0].UnresolvedChildren)
}

func TestListPhaseExpiryWarnings_RequiresAdminScope(t *testing.T) {
	t.Parallel()

	resource := &Resource{PhaseExpiryService: &phaseExpiryServiceStub{}}
	router := chi.NewRouter()
	router.Use(render.SetContentType(render.ContentTypeJSON))
	router.With(authorize.RequiresPermission("admin:*")).Get(
		"/enrollment/phases/expiry-warnings",
		resource.listPhaseExpiryWarnings,
	)

	readOnly := executeAdminJSONWithPermissions(
		t,
		router,
		http.MethodGet,
		"/enrollment/phases/expiry-warnings",
		[]string{"config:read"},
	)
	assert.Equal(t, http.StatusForbidden, readOnly.Code)

	admin := executeAdminJSONWithPermissions(
		t,
		router,
		http.MethodGet,
		"/enrollment/phases/expiry-warnings",
		[]string{"admin:*"},
	)
	assert.Equal(t, http.StatusOK, admin.Code)
}
