package common_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type quotaRejection struct{}

func (quotaRejection) Error() string     { return "quota reached" }
func (quotaRejection) ErrorCode() string { return "students.child_quota_reached" }
func (quotaRejection) ErrorDetails() any {
	return struct {
		Booked    int `json:"booked_places"`
		Occupied  int `json:"occupied_places"`
		Requested int `json:"requested_places"`
	}{Booked: 50, Occupied: 50, Requested: 1}
}

// TestBusinessRejectionAnswers409WithCodeAndDetails pins the Fehlerklasse
// "Fachliche Ablehnung": a module error anywhere in the chain becomes 409 with
// its stable code and the values its message names.
func TestBusinessRejectionAnswers409WithCodeAndDetails(t *testing.T) {
	t.Parallel()
	err := fmt.Errorf("create student: %w", quotaRejection{})
	require.True(t, common.IsBusinessRejection(err))

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/", nil)
	common.RenderError(w, r, common.ErrorBusinessRejection(err))

	require.Equal(t, http.StatusConflict, w.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "students.child_quota_reached", body["code"])
	assert.Equal(t, map[string]any{"booked_places": float64(50), "occupied_places": float64(50), "requested_places": float64(1)}, body["details"])
}

// TestUnclassifiedBusinessRejectionIsNotAServerError pins the fallback: a
// handler that wraps a rejection into a 500 still answers 409 with its code.
func TestUnclassifiedBusinessRejectionIsNotAServerError(t *testing.T) {
	t.Parallel()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/", nil)
	common.RenderError(w, r, common.ErrorInternalServer(fmt.Errorf("import: %w", quotaRejection{})))

	require.Equal(t, http.StatusConflict, w.Code)
	assert.Contains(t, w.Body.String(), `"code":"students.child_quota_reached"`)
}

// TestBusinessRejectionRollsBackTheRequest pins that a 409 rejection never
// commits the rows its operation wrote before it was refused.
func TestBusinessRejectionRollsBackTheRequest(t *testing.T) {
	t.Parallel()
	ctx := tenant.WithRollbackMarker(context.Background())
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/", nil).WithContext(ctx)
	common.RenderError(w, r, common.ErrorBusinessRejection(quotaRejection{}))
	require.Equal(t, http.StatusConflict, w.Code)
	assert.True(t, tenant.RollbackRequested(ctx))

	other := tenant.WithRollbackMarker(context.Background())
	common.RenderError(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/", nil).WithContext(other),
		common.ErrorConflict(errors.New("plain conflict")))
	assert.False(t, tenant.RollbackRequested(other), "other conflicts keep their commit behaviour")
}

func TestBusinessRejectionLeavesOtherErrorsAlone(t *testing.T) {
	t.Parallel()
	err := errors.New("database down")
	require.False(t, common.IsBusinessRejection(err))

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/", nil)
	common.RenderError(w, r, common.ErrorBusinessRejection(err))
	require.Equal(t, http.StatusInternalServerError, w.Code)
}
