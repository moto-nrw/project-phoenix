package enrollment

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
)

// responseOverviewPhaseService answers only the response overview; every
// other call falls through to the embedded mock.
type responseOverviewPhaseService struct {
	*mockPhaseService
	id     int64
	result *enrollmentService.PhaseResponseOverview
	err    error
}

func (m *responseOverviewPhaseService) ResponseOverview(_ context.Context, id int64) (*enrollmentService.PhaseResponseOverview, error) {
	m.id = id
	return m.result, m.err
}

// ResponseOverview keeps the shared mock a full PhaseService.
func (m *mockPhaseService) ResponseOverview(context.Context, int64) (*enrollmentService.PhaseResponseOverview, error) {
	return nil, errors.New("response overview not stubbed")
}

func buildPhaseResponseRouter(svc enrollmentService.PhaseService) chi.Router {
	rs := &Resource{PhaseService: svc}
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	r.Get("/enrollment/phases/{id}/responses", rs.getPhaseResponseOverview)
	return r
}

func TestPhaseResponseOverviewHandler_NilServiceReturns500(t *testing.T) {
	t.Parallel()

	w := executePhaseJSON(t, buildPhaseResponseRouter(nil), http.MethodGet, "/enrollment/phases/12/responses", nil)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestPhaseResponseOverviewHandler_InvalidIDRejected(t *testing.T) {
	t.Parallel()

	svc := &responseOverviewPhaseService{mockPhaseService: &mockPhaseService{}}
	w := executePhaseJSON(t, buildPhaseResponseRouter(svc), http.MethodGet, "/enrollment/phases/abc/responses", nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestPhaseResponseOverviewHandler_NotFoundReturns404(t *testing.T) {
	t.Parallel()

	svc := &responseOverviewPhaseService{mockPhaseService: &mockPhaseService{}, err: enrollmentService.ErrPhaseNotFound}
	w := executePhaseJSON(t, buildPhaseResponseRouter(svc), http.MethodGet, "/enrollment/phases/12/responses", nil)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestPhaseResponseOverviewHandler_GenericErrorReturns500(t *testing.T) {
	t.Parallel()

	svc := &responseOverviewPhaseService{mockPhaseService: &mockPhaseService{}, err: errors.New("synthetic boom")}
	w := executePhaseJSON(t, buildPhaseResponseRouter(svc), http.MethodGet, "/enrollment/phases/12/responses", nil)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestPhaseResponseOverviewHandler_RendersRowsAndExclusions(t *testing.T) {
	t.Parallel()

	requestID, pendingID := int64(900), int64(901)
	svc := &responseOverviewPhaseService{
		mockPhaseService: &mockPhaseService{},
		result: &enrollmentService.PhaseResponseOverview{
			Applicable: true, Expected: 2, Responded: 1,
			Rows: []enrollmentService.PhaseResponseRow{
				{StudentID: 7, FirstName: "Mia", LastName: "Arslan", SchoolClass: "2a", PendingRequestID: &pendingID, ChildStatus: "pending_renewal"},
				{StudentID: 8, FirstName: "Ben", LastName: "Yilmaz", SchoolClass: "1b", HasParentApp: true, Responded: true, RequestID: &requestID, ChildStatus: "submitted"},
			},
			Excluded: []enrollmentService.PhaseResponseExclusion{{Reason: enrollmentService.PhaseResponseExcludedGraduating, Count: 3}},
		},
	}
	w := executePhaseJSON(t, buildPhaseResponseRouter(svc), http.MethodGet, "/enrollment/phases/12/responses", nil)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, int64(12), svc.id)

	var body struct {
		Data PhaseResponseOverviewResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.True(t, body.Data.Applicable)
	assert.Equal(t, 2, body.Data.Expected)
	assert.Equal(t, 1, body.Data.Responded)
	require.Len(t, body.Data.Children, 2)
	assert.False(t, body.Data.Children[0].Responded)
	assert.Nil(t, body.Data.Children[0].RequestID)
	require.NotNil(t, body.Data.Children[0].PendingRequestID)
	assert.Equal(t, pendingID, *body.Data.Children[0].PendingRequestID)
	assert.True(t, body.Data.Children[1].HasParentApp)
	require.NotNil(t, body.Data.Children[1].RequestID)
	assert.Equal(t, requestID, *body.Data.Children[1].RequestID)
	assert.Equal(t, []PhaseResponseExclusionResponse{{Reason: "graduating", Count: 3}}, body.Data.Excluded)
}

func TestPhaseResponseOverviewHandler_NotApplicableRendersEmptyArrays(t *testing.T) {
	t.Parallel()

	svc := &responseOverviewPhaseService{
		mockPhaseService: &mockPhaseService{},
		result: &enrollmentService.PhaseResponseOverview{
			Rows: []enrollmentService.PhaseResponseRow{}, Excluded: []enrollmentService.PhaseResponseExclusion{},
		},
	}
	w := executePhaseJSON(t, buildPhaseResponseRouter(svc), http.MethodGet, "/enrollment/phases/12/responses", nil)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"applicable":false`)
	assert.Contains(t, w.Body.String(), `"children":[]`)
	assert.Contains(t, w.Body.String(), `"excluded":[]`)
}
