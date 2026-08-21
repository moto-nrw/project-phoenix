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
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
	userService "github.com/moto-nrw/project-phoenix/services/users"
)

func TestCompanionPlanErrorRenderer(t *testing.T) {
	t.Parallel()

	t.Run("stranded companion is a client-fixable 400", func(t *testing.T) {
		resp := rendererStatus(t, companionPlanErrorRenderer(userService.ErrCompanionWouldLoseDeparture))

		assert.Equal(t, http.StatusBadRequest, resp.HTTPStatusCode)
		// The German sentinel text is the whole instruction the user gets.
		assert.Equal(t, userService.ErrCompanionWouldLoseDeparture.Error(), resp.ErrorText)
	})

	t.Run("busy companion lock is a retriable 409 with its code", func(t *testing.T) {
		resp := rendererStatus(t, companionPlanErrorRenderer(userService.ErrCompanionLockBusy))

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
		{name: "stranded companion", err: userService.ErrCompanionWouldLoseDeparture, want: http.StatusBadRequest},
		{name: "busy companion lock", err: userService.ErrCompanionLockBusy, want: http.StatusConflict},
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
// sentinel; the other methods exist only to satisfy the interface.
type companionErrCareRequestService struct {
	decideErr error
}

func (f *companionErrCareRequestService) CreateRequest(context.Context, int64, int64, map[string]any) (*scheduleModels.CareScheduleChangeRequest, error) {
	return nil, nil
}

func (f *companionErrCareRequestService) WithdrawRequest(context.Context, int64, int64, int64) (*scheduleModels.CareScheduleChangeRequest, error) {
	return nil, nil
}

func (f *companionErrCareRequestService) WithdrawPickupChangeRequest(context.Context, int64, int64, int64) (*scheduleModels.CareScheduleChangeRequest, error) {
	return nil, nil
}

func (f *companionErrCareRequestService) GetPendingForStudent(context.Context, int64) (*scheduleModels.CareScheduleChangeRequest, []scheduleService.RequestDiffEntry, error) {
	return nil, nil, nil
}

func (f *companionErrCareRequestService) ListHistory(context.Context, modelBase.RequestQueueFilters) ([]*scheduleService.CareRequestHistoryItem, *userService.HistoryCursor, error) {
	return nil, nil, nil
}

func (f *companionErrCareRequestService) ListPending(context.Context, modelBase.RequestQueueFilters) ([]*scheduleService.CareRequestReviewItem, *userService.HistoryCursor, error) {
	return nil, nil, nil
}

func (f *companionErrCareRequestService) CreatePickupChangeRequest(context.Context, int64, int64, timezone.Date, time.Time, string) (*scheduleModels.CareScheduleChangeRequest, error) {
	return nil, nil
}

func (f *companionErrCareRequestService) ListPendingPickupChanges(context.Context) ([]*scheduleService.CareRequestReviewItem, error) {
	return nil, nil
}

func (f *companionErrCareRequestService) ListPickupChangeRequests(context.Context, int64, time.Time) ([]*scheduleModels.CareScheduleChangeRequest, error) {
	return nil, nil
}

func (f *companionErrCareRequestService) Decide(_ context.Context, input scheduleService.CareRequestDecideInput) (*scheduleService.CareRequestReviewItem, error) {
	if f.decideErr == nil && input.RequireImpactToken && input.ExpectedImpactToken == nil {
		return nil, scheduleService.ErrPickupChangeImpactRequired
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
		{name: "stranded companion", err: userService.ErrCompanionWouldLoseDeparture, want: http.StatusBadRequest},
		{name: "busy companion lock", err: userService.ErrCompanionLockBusy, want: http.StatusConflict},
		{name: "stale pickup impact", err: scheduleService.ErrPickupChangeImpactChanged, want: http.StatusConflict, wantCode: "pickup_change_impact_changed"},
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
