package students

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	userService "github.com/moto-nrw/project-phoenix/services/users"
)

type fakeOfferingChangeRequestService struct {
	input                enrollmentService.DecideOfferingChangeInput
	previewExcluded      []int64
	previewEffectiveFrom *timezone.Date
	preview              []enrollmentService.OfferingChangePreviewSelection
}

func (f *fakeOfferingChangeRequestService) Catalog(context.Context, int64) (*enrollmentService.OfferingChangeCatalog, error) {
	return nil, nil
}

func (f *fakeOfferingChangeRequestService) CatalogAt(context.Context, int64, timezone.Date) (*enrollmentService.OfferingChangeCatalog, error) {
	return nil, nil
}

func (f *fakeOfferingChangeRequestService) GetForStudent(context.Context, int64) (*enrollmentService.OfferingChangeView, error) {
	return nil, nil
}

func (f *fakeOfferingChangeRequestService) Create(context.Context, enrollmentService.CreateOfferingChangeInput) (*enrollmentModels.OfferingChangeRequest, error) {
	return nil, nil
}

func (f *fakeOfferingChangeRequestService) Withdraw(context.Context, int64, int64, int64) error {
	return nil
}

func (f *fakeOfferingChangeRequestService) ListHistory(context.Context, modelBase.RequestQueueFilters) ([]*enrollmentService.OfferingChangeHistoryItem, *userService.HistoryCursor, error) {
	return nil, nil, nil
}

func (f *fakeOfferingChangeRequestService) ListDirectCorrections(context.Context, modelBase.RequestQueueFilters) ([]*enrollmentService.DirectCorrectionItem, *userService.HistoryCursor, error) {
	return nil, nil, nil
}

func (f *fakeOfferingChangeRequestService) ListPending(context.Context, modelBase.RequestQueueFilters) ([]*enrollmentService.OfferingChangeView, *userService.HistoryCursor, error) {
	return nil, nil, nil
}

func (f *fakeOfferingChangeRequestService) PendingCount(context.Context) (int, error) {
	return 0, nil
}

func (f *fakeOfferingChangeRequestService) PreviewDecision(
	_ context.Context,
	_ int64,
	excluded []int64,
	effectiveFrom *timezone.Date,
) ([]enrollmentService.OfferingChangePreviewSelection, error) {
	f.previewExcluded = excluded
	f.previewEffectiveFrom = effectiveFrom
	return f.preview, nil
}

func (f *fakeOfferingChangeRequestService) Decide(_ context.Context, input enrollmentService.DecideOfferingChangeInput) error {
	f.input = input
	return nil
}

func (f *fakeOfferingChangeRequestService) EarliestEffectiveFrom(context.Context) (timezone.Date, error) {
	return timezone.Date{}, nil
}

func TestDecideOfferingChangeRequest_UsesReviewerRolesForAudit(t *testing.T) {
	t.Parallel()

	svc := &fakeOfferingChangeRequestService{}
	rs := &Resource{ResourceConfig: ResourceConfig{OfferingChangeService: svc}}
	req := httptest.NewRequest(http.MethodPost, "/offering-change-requests/100/decide", strings.NewReader(`{"approve":true,"effective_from":"2026-09-01"}`))
	claims := jwt.AppClaims{ID: 55, Roles: []string{"group_supervisor", "staff"}}
	req = req.WithContext(context.WithValue(req.Context(), jwt.CtxClaims, claims))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("requestId", "100")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	rs.decideOfferingChangeRequest(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, int64(55), svc.input.ReviewedBy)
	assert.Equal(t, "group_supervisor,staff", svc.input.ActorRole)
}

func TestPreviewOfferingChangeRequest_ReturnsMaterializedDays(t *testing.T) {
	t.Parallel()

	svc := &fakeOfferingChangeRequestService{preview: []enrollmentService.OfferingChangePreviewSelection{{
		OfferingID: 11,
		State:      "booked",
		Days:       []string{"mon", "wed"},
	}}}
	rs := &Resource{ResourceConfig: ResourceConfig{OfferingChangeService: svc}}
	req := httptest.NewRequest(http.MethodPost, "/offering-change-requests/100/preview",
		strings.NewReader(`{"excluded_offering_ids":["9"]}`))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("requestId", "100")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	rs.previewOfferingChangeRequest(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, []int64{9}, svc.previewExcluded)
	assert.Contains(t, w.Body.String(), `"offering_id":"11"`)
	assert.Contains(t, w.Body.String(), `"new":"Mo, Mi"`)
}

func TestToOfferingRequestResponse_IncludesRemainingDaysForOverridePreview(t *testing.T) {
	t.Parallel()

	view := &enrollmentService.OfferingChangeView{
		Request: &enrollmentModels.OfferingChangeRequest{},
		Diff: []enrollmentService.OfferingChangeDiffEntry{{
			OfferingID:          9,
			Label:               "Mittagessen",
			NewState:            "booked",
			NewDays:             []string{"mon", "tue", "wed"},
			NewAutomaticDays:    []string{"tue", "wed"},
			NewRuleDays:         []string{"tue"},
			NewDaysWithoutRules: []string{"mon", "wed"},
			AutoTriggerIDs:      []int64{5},
			AutoTriggerNames:    []string{"Randstunde"},
		}},
	}

	response := toOfferingRequestResponse(view)

	require.Len(t, response.Diff, 1)
	assert.Equal(t, "Di", response.Diff[0].RuleDays)
	assert.Equal(t, "Mo, Mi", response.Diff[0].NewWhenExcluded)
	assert.True(t, response.Diff[0].Optoutable)
}

func TestToOfferingRequestResponse_ReportsFullWithdrawalAndUntouchedBookings(t *testing.T) {
	t.Parallel()

	view := &enrollmentService.OfferingChangeView{
		Request:        &enrollmentModels.OfferingChangeRequest{},
		FullWithdrawal: true,
		Diff: []enrollmentService.OfferingChangeDiffEntry{{
			OfferingID: 3,
			Label:      "Regelbetreuung",
			OldState:   "booked",
			OldDays:    []string{"mon", "tue"},
			NewState:   "removed",
		}},
		Unchanged: []enrollmentService.OfferingChangeDiffEntry{{
			OfferingID: 4,
			Label:      "Mittagessen",
			OldState:   "booked",
			OldDays:    []string{"mon"},
			NewState:   "booked",
			NewDays:    []string{"mon"},
		}},
	}

	resp := toOfferingRequestResponse(view)

	assert.True(t, resp.FullWithdrawal)
	assert.Equal(t, "abgemeldet", resp.Diff[0].New)
	require.Len(t, resp.Unchanged, 1)
	assert.Equal(t, "4", resp.Unchanged[0].OfferingID)
	assert.Equal(t, "Mittagessen", resp.Unchanged[0].Label)
	assert.Equal(t, "Mo", resp.Unchanged[0].Days)
}

func TestToOfferingRequestResponse_OmitsFullWithdrawalForAnOrdinaryRequest(t *testing.T) {
	t.Parallel()

	view := &enrollmentService.OfferingChangeView{
		Request: &enrollmentModels.OfferingChangeRequest{},
		Diff: []enrollmentService.OfferingChangeDiffEntry{{
			OfferingID: 3,
			Label:      "Regelbetreuung",
			OldState:   "booked",
			OldDays:    []string{"mon", "tue"},
			NewState:   "booked",
			NewDays:    []string{"mon"},
		}},
	}

	resp := toOfferingRequestResponse(view)

	assert.False(t, resp.FullWithdrawal)
	assert.Empty(t, resp.Unchanged)
}

// The office confirms the date the switch applies on, and the endpoint has to
// hand exactly that date to the decision (#2484).
func TestDecideOfferingChangeRequest_PassesTheConfirmedDate(t *testing.T) {
	t.Parallel()

	svc := &fakeOfferingChangeRequestService{}
	rs := &Resource{ResourceConfig: ResourceConfig{OfferingChangeService: svc}}
	req := httptest.NewRequest(http.MethodPost, "/offering-change-requests/100/decide",
		strings.NewReader(`{"approve":true,"effective_from":"2026-09-01"}`))
	claims := jwt.AppClaims{ID: 55, Roles: []string{"staff"}}
	req = req.WithContext(context.WithValue(req.Context(), jwt.CtxClaims, claims))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("requestId", "100")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	rs.decideOfferingChangeRequest(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.NotNil(t, svc.input.EffectiveFrom)
	assert.Equal(t, timezone.NewDate(2026, 9, 1), *svc.input.EffectiveFrom)
}

// No date, no approval: the switch would otherwise apply on a day nobody chose.
func TestDecideOfferingChangeRequest_RefusesApprovalWithoutADate(t *testing.T) {
	t.Parallel()

	svc := &fakeOfferingChangeRequestService{}
	rs := &Resource{ResourceConfig: ResourceConfig{OfferingChangeService: svc}}
	req := httptest.NewRequest(http.MethodPost, "/offering-change-requests/100/decide",
		strings.NewReader(`{"approve":true}`))
	claims := jwt.AppClaims{ID: 55, Roles: []string{"staff"}}
	req = req.WithContext(context.WithValue(req.Context(), jwt.CtxClaims, claims))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("requestId", "100")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	rs.decideOfferingChangeRequest(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Zero(t, svc.input.RequestID, "no decision may reach the service")
}

// A rejection needs no date, so the endpoint must not start demanding one.
func TestDecideOfferingChangeRequest_RejectionNeedsNoDate(t *testing.T) {
	t.Parallel()

	svc := &fakeOfferingChangeRequestService{}
	rs := &Resource{ResourceConfig: ResourceConfig{OfferingChangeService: svc}}
	req := httptest.NewRequest(http.MethodPost, "/offering-change-requests/100/decide",
		strings.NewReader(`{"approve":false,"reason":"Kein Platz"}`))
	claims := jwt.AppClaims{ID: 55, Roles: []string{"staff"}}
	req = req.WithContext(context.WithValue(req.Context(), jwt.CtxClaims, claims))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("requestId", "100")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	rs.decideOfferingChangeRequest(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Nil(t, svc.input.EffectiveFrom)
}

// The card previews the decision for the date currently chosen in it.
func TestPreviewOfferingChangeRequest_PassesTheChosenDate(t *testing.T) {
	t.Parallel()

	svc := &fakeOfferingChangeRequestService{}
	rs := &Resource{ResourceConfig: ResourceConfig{OfferingChangeService: svc}}
	req := httptest.NewRequest(http.MethodPost, "/offering-change-requests/100/preview",
		strings.NewReader(`{"excluded_offering_ids":[],"effective_from":"2026-09-01"}`))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("requestId", "100")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	rs.previewOfferingChangeRequest(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.NotNil(t, svc.previewEffectiveFrom)
	assert.Equal(t, timezone.NewDate(2026, 9, 1), *svc.previewEffectiveFrom)
}
