package students

// StudentRepository.Update reconciles the "läuft mit" edges for EVERY caller,
// not just the student PUT. So every flow that rewrites a departure plan can
// end up refusing with one of the two companion sentinels — and each of them
// has to answer with the same actionable status the student PUT does. A 500
// here would tell a supervisor "the server broke" for a situation they can fix
// themselves (give the other child a note first / save again in a moment), and
// would page whoever watches the error rate for it (#1694).

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	"github.com/stretchr/testify/assert"
)

func TestCompanionPlanErrorRenderer(t *testing.T) {
	t.Parallel()

	t.Run("stranded companion is a client-fixable 400", func(t *testing.T) {
		resp := rendererStatus(t, companionPlanErrorRenderer(careplan.ErrCompanionWouldLoseDeparture))

		assert.Equal(t, http.StatusBadRequest, resp.HTTPStatusCode)
		// The German sentinel text is the whole instruction the user gets.
		assert.Equal(t, careplan.ErrCompanionWouldLoseDeparture.Error(), resp.ErrorText)
	})

	t.Run("busy companion lock is a retriable 409 with its code", func(t *testing.T) {
		resp := rendererStatus(t, companionPlanErrorRenderer(careplan.ErrCompanionLockBusy))

		assert.Equal(t, http.StatusConflict, resp.HTTPStatusCode)
		assert.Equal(t, CodeCompanionLockBusy, resp.Code)
	})

	t.Run("classifies through wrapping", func(t *testing.T) {
		// Both review services wrap the repository error with fmt.Errorf("%w"),
		// so equality checks would miss it.
		resp := rendererStatus(t, companionPlanErrorRenderer(
			errors.Join(errors.New("review: update student departure"), userModels.ErrCompanionWouldLoseDeparture),
		))

		assert.Equal(t, http.StatusBadRequest, resp.HTTPStatusCode)
	})

	t.Run("leaves unrelated errors to the caller", func(t *testing.T) {
		assert.Nil(t, companionPlanErrorRenderer(errors.New("boom")))
	})
}

func TestDecideMasterDataChangeRequest_MapsCompanionErrors(t *testing.T) {
	t.Parallel()

	// Approving an allowed_departure_modes change drops the accompanied mode,
	// which is exactly what strands a linked child.
	tests := []struct {
		name string
		err  error
		want int
	}{
		{name: "stranded companion", err: careplan.ErrCompanionWouldLoseDeparture, want: http.StatusBadRequest},
		{name: "busy companion lock", err: careplan.ErrCompanionLockBusy, want: http.StatusConflict},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rs := &Resource{ResourceConfig: ResourceConfig{MasterDataReviewService: &fakeMasterDataReviewService{decideErr: tt.err}}}
			req := staffRequest(http.MethodPost, "/students/master-data-change-requests/100/decide", `{"approve":true}`, "100")
			w := httptest.NewRecorder()

			rs.decideMasterDataChangeRequest(w, req)

			assert.Equal(t, tt.want, w.Code)
			assert.Contains(t, w.Body.String(), "verknüpftes Kind")
		})
	}
}

// companionErrCareRequestService fails every Decide with one companion
// sentinel; MarkDone exists only to satisfy the native decision interface.
type companionErrCareRequestService struct {
	decideErr error
}

func (f *companionErrCareRequestService) MarkDone(context.Context, int64, string, string, int64) error {
	return nil
}

func (f *companionErrCareRequestService) Decide(_ context.Context, input carerequests.DecideInput) (*carerequests.ReviewItem, error) {
	if f.decideErr == nil && input.RequireImpactToken && input.ExpectedImpactToken == nil {
		return nil, carerequests.ErrPickupChangeImpactRequired
	}
	return nil, f.decideErr
}

func TestDecideCareScheduleChangeRequest_MapsCompanionErrors(t *testing.T) {
	t.Parallel()

	// Approving a care-schedule change merges new per-weekday modes onto the
	// stored plan, so it can drop the accompanied day a link depends on.
	tests := []struct {
		name     string
		err      error
		want     int
		wantCode string
	}{
		{name: "stranded companion", err: careplan.ErrCompanionWouldLoseDeparture, want: http.StatusBadRequest},
		{name: "busy companion lock", err: careplan.ErrCompanionLockBusy, want: http.StatusConflict},
		{name: "stale pickup impact", err: carerequests.ErrPickupChangeImpactChanged, want: http.StatusConflict, wantCode: "pickup_change_impact_changed"},
		{name: "care day belongs to booking", err: carerequests.ErrCareDayManagedByBooking, want: http.StatusConflict, wantCode: "care_day_managed_by_booking"},
		{name: "missing pickup impact", want: http.StatusBadRequest},
		{name: "unrelated error stays a server error", err: errors.New("boom"), want: http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rs := &Resource{ResourceConfig: ResourceConfig{CareRequestService: &companionErrCareRequestService{decideErr: tt.err}}}
			req := staffRequest(http.MethodPost, "/students/care-schedule-change-requests/100/decide", `{"approve":true}`, "100")
			w := httptest.NewRecorder()

			rs.decideCareScheduleChangeRequest(w, req)

			assert.Equal(t, tt.want, w.Code)
			if tt.wantCode != "" {
				assert.Contains(t, w.Body.String(), `"code":"`+tt.wantCode+`"`)
			}
		})
	}
}
