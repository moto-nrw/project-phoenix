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

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/offeringrequests"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
	requestreviewcompose "github.com/moto-nrw/project-phoenix/modules/requestreview/compose"
)

// fakeOfferingChangeRequestService records the preview and decide calls; the
// embedded interface panics for any other method the routes must not reach.
type fakeOfferingChangeRequestService struct {
	careplan.OfferingChangeRequests
	input                careplan.OfferingChangeDecisionInput
	previewExcluded      []int64
	previewEffectiveFrom *timezone.Date
	preview              *careplan.OfferingChangePreview
}

func (f *fakeOfferingChangeRequestService) PreviewDecision(
	_ context.Context,
	_ int64,
	excluded []int64,
	effectiveFrom *timezone.Date,
) (*careplan.OfferingChangePreview, error) {
	f.previewExcluded = excluded
	f.previewEffectiveFrom = effectiveFrom
	return f.preview, nil
}

func (f *fakeOfferingChangeRequestService) Decide(_ context.Context, input careplan.OfferingChangeDecisionInput) error {
	f.input = input
	return nil
}

func (f *fakeOfferingChangeRequestService) EarliestEffectiveFrom(context.Context) (timezone.Date, error) {
	return timezone.Date(""), nil
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

	svc := &fakeOfferingChangeRequestService{preview: &careplan.OfferingChangePreview{
		Selections: []careplan.OfferingChangePreviewSelection{{
			OfferingID: 11,
			State:      "booked",
			Days:       []string{"mon", "wed"},
		}},
		ManualPlanningConflicts: []careplan.ManualPlanningConflict{{
			ActivityGroupID:   17,
			ActivityGroupName: "Freie Hausaufgaben-Gruppe",
			Days:              []string{"tue"},
			FirstDate:         timezone.NewDate(2027, 2, 2),
			OccurrenceCount:   8,
		}},
		ArrivalExpectationsFollowBookings: true,
	}}
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
	assert.Contains(t, w.Body.String(), `"activity_group_name":"Freie Hausaufgaben-Gruppe"`)
	assert.Contains(t, w.Body.String(), `"days":["tue"]`)
	assert.Contains(t, w.Body.String(), `"first_date":"2027-02-02"`)
	assert.Contains(t, w.Body.String(), `"occurrence_count":8`)
	assert.Contains(t, w.Body.String(), `"arrival_expectations_follow_bookings":true`)
}

func TestToOfferingRequestResponse_IncludesRemainingDaysForOverridePreview(t *testing.T) {
	t.Parallel()

	view := &offeringrequests.ReviewItem{
		Request: &offeringrequests.Request{},
		Diff: []offeringrequests.DiffEntry{{
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

	response := requestreviewcompose.ToOfferingRequestResponse(view)

	require.Len(t, response.Diff, 1)
	assert.Equal(t, "Di", response.Diff[0].RuleDays)
	assert.Equal(t, "Mo, Mi", response.Diff[0].NewWhenExcluded)
	assert.True(t, response.Diff[0].Optoutable)
}

func TestToOfferingRequestResponse_ReportsFullWithdrawalAndUntouchedBookings(t *testing.T) {
	t.Parallel()

	view := &offeringrequests.ReviewItem{
		Request:        &offeringrequests.Request{},
		FullWithdrawal: true,
		Diff: []offeringrequests.DiffEntry{{
			OfferingID: 3,
			Label:      "Regelbetreuung",
			OldState:   "booked",
			OldDays:    []string{"mon", "tue"},
			NewState:   "removed",
		}},
		Unchanged: []offeringrequests.DiffEntry{{
			OfferingID: 4,
			Label:      "Mittagessen",
			OldState:   "booked",
			OldDays:    []string{"mon"},
			NewState:   "booked",
			NewDays:    []string{"mon"},
		}},
	}

	resp := requestreviewcompose.ToOfferingRequestResponse(view)

	assert.True(t, resp.FullWithdrawal)
	assert.Equal(t, "abgemeldet", resp.Diff[0].New)
	require.Len(t, resp.Unchanged, 1)
	assert.Equal(t, "4", resp.Unchanged[0].OfferingID)
	assert.Equal(t, "Mittagessen", resp.Unchanged[0].Label)
	assert.Equal(t, "Mo", resp.Unchanged[0].Days)
}

func TestToOfferingRequestResponse_OmitsFullWithdrawalForAnOrdinaryRequest(t *testing.T) {
	t.Parallel()

	view := &offeringrequests.ReviewItem{
		Request: &offeringrequests.Request{},
		Diff: []offeringrequests.DiffEntry{{
			OfferingID: 3,
			Label:      "Regelbetreuung",
			OldState:   "booked",
			OldDays:    []string{"mon", "tue"},
			NewState:   "booked",
			NewDays:    []string{"mon"},
		}},
	}

	resp := requestreviewcompose.ToOfferingRequestResponse(view)

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

func TestDecideOfferingChangeRequest_PassesCompleteWithdrawalConfirmation(t *testing.T) {
	t.Parallel()

	svc := &fakeOfferingChangeRequestService{}
	rs := &Resource{ResourceConfig: ResourceConfig{OfferingChangeService: svc}}
	req := httptest.NewRequest(http.MethodPost, "/offering-change-requests/100/decide",
		strings.NewReader(`{"approve":true,"effective_from":"2026-09-01","complete_withdrawal_confirmed":true}`))
	req = req.WithContext(context.WithValue(req.Context(), jwt.CtxClaims, jwt.AppClaims{ID: 55, Roles: []string{"staff"}}))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("requestId", "100")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	rs.decideOfferingChangeRequest(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.True(t, svc.input.CompleteWithdrawalConfirmed)
}

func TestRenderOfferingDecisionError_UsesStableCompleteWithdrawalCode(t *testing.T) {
	t.Parallel()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/", nil)

	renderOfferingDecisionError(recorder, request, careplan.ErrCompleteWithdrawalConfirmationRequired)

	assert.Equal(t, http.StatusConflict, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"code":"enrollment.complete_withdrawal_confirmation_required"`)
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

	svc := &fakeOfferingChangeRequestService{preview: &careplan.OfferingChangePreview{}}
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
