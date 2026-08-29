package staffshifts

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	auditModel "github.com/moto-nrw/project-phoenix/models/audit"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/services/users/userstest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeShiftService captures the CancelShiftInput the cancellation handler builds
// so the payload-mapping and partial-edit-rejection behaviour can be asserted
// without a database.
type fakeShiftService struct {
	createFn func(ctx context.Context, shift *scheduleModel.StaffShift) (*scheduleModel.StaffShift, error)
	applyFn  func(ctx context.Context, input scheduleSvc.CancelShiftInput) (*scheduleSvc.CancelShiftResult, error)
	moveFn   func(ctx context.Context, input scheduleSvc.MoveShiftInput) (*scheduleModel.StaffShift, error)
}

func (f *fakeShiftService) ListShifts(context.Context, timezone.Date, timezone.Date) ([]*scheduleModel.StaffShift, error) {
	return nil, nil
}

func (f *fakeShiftService) ListShiftsForStaff(context.Context, int64, timezone.Date, timezone.Date) ([]*scheduleModel.StaffShift, error) {
	return nil, nil
}

func (f *fakeShiftService) CreateShift(ctx context.Context, shift *scheduleModel.StaffShift) (*scheduleModel.StaffShift, error) {
	if f.createFn != nil {
		return f.createFn(ctx, shift)
	}
	return nil, nil
}

func (f *fakeShiftService) UpdateShift(context.Context, *scheduleModel.StaffShift) (*scheduleModel.StaffShift, error) {
	return nil, nil
}

func (f *fakeShiftService) UpdateShiftWithOptions(context.Context, *scheduleModel.StaffShift, scheduleSvc.StaffShiftUpdateOptions) (*scheduleModel.StaffShift, error) {
	return nil, nil
}

func (f *fakeShiftService) MoveShift(ctx context.Context, input scheduleSvc.MoveShiftInput) (*scheduleModel.StaffShift, error) {
	if f.moveFn != nil {
		return f.moveFn(ctx, input)
	}
	return &scheduleModel.StaffShift{}, nil
}

func (f *fakeShiftService) DeleteShift(context.Context, int64) error { return nil }

func (f *fakeShiftService) ApplyCancellation(ctx context.Context, input scheduleSvc.CancelShiftInput) (*scheduleSvc.CancelShiftResult, error) {
	if f.applyFn != nil {
		return f.applyFn(ctx, input)
	}
	return &scheduleSvc.CancelShiftResult{Shift: &scheduleModel.StaffShift{}}, nil
}

func (f *fakeShiftService) SetSeriesExceptionRepo(scheduleModel.StaffShiftSeriesExceptionRepository) {
}

func (f *fakeShiftService) SetDeviationEventRepo(auditModel.DeviationEventRepository) {
}

// shiftTestResource wires a Resource whose editorStaffID resolves to staff id 42.
func shiftTestResource(service scheduleSvc.StaffShiftService) *Resource {
	person := &userModels.Person{}
	person.ID = 41
	staff := &userModels.Staff{}
	staff.ID = 42
	return &Resource{
		Service: service,
		PersonService: &userstest.PersonServiceMock{
			FindByAccountIDFn: func(context.Context, int64) (*userModels.Person, error) {
				return person, nil
			},
			GetStaffByPersonIDFn: func(context.Context, int64) (*userModels.Staff, error) {
				return staff, nil
			},
		},
	}
}

func cancellationRouter(resource *Resource) chi.Router {
	router := chi.NewRouter()
	router.Put("/{id}/cancellation", resource.cancellation)
	return router
}

func moveRouter(resource *Resource) chi.Router {
	router := chi.NewRouter()
	router.Put("/{id}/move", resource.move)
	return router
}

func createRouter(resource *Resource) chi.Router {
	router := chi.NewRouter()
	router.Post("/", resource.create)
	return router
}

func TestToShiftResponse_IncludesSeriesOccurrenceDate(t *testing.T) {
	t.Parallel()

	sourceDate := timezone.NewDate(2026, 7, 6)
	seriesID := int64(9)
	shift := &scheduleModel.StaffShift{
		SeriesID:             &seriesID,
		SeriesOccurrenceDate: &sourceDate,
	}

	response := ToShiftResponse(shift)
	require.NotNil(t, response.SeriesOccurrenceDate)
	assert.Equal(t, "2026-07-06", *response.SeriesOccurrenceDate)
}

func TestToShiftResponse_SerializesShiftAndSeriesIDsAsStrings(t *testing.T) {
	t.Parallel()

	seriesID := int64(9223372036854775807)
	originShiftID := int64(9223372036854775805)
	shift := &scheduleModel.StaffShift{SeriesID: &seriesID, OriginShiftID: &originShiftID}
	shift.ID = 9223372036854775806

	encoded, err := json.Marshal(ToShiftResponse(shift))
	require.NoError(t, err)

	var body map[string]any
	require.NoError(t, json.Unmarshal(encoded, &body))
	assert.Equal(t, "9223372036854775806", body["id"])
	assert.Equal(t, "9223372036854775807", body["series_id"])
	assert.Equal(t, "9223372036854775805", body["origin_shift_id"])
}

func TestCreateHandler_AcceptsStringOriginShiftIDWithoutPrecisionLoss(t *testing.T) {
	t.Parallel()

	const originShiftID = int64(9223372036854775807)
	var got *scheduleModel.StaffShift
	service := &fakeShiftService{
		createFn: func(_ context.Context, shift *scheduleModel.StaffShift) (*scheduleModel.StaffShift, error) {
			got = shift
			return shift, nil
		},
	}
	body := `{"staff_id":7,"date":"2026-07-07","start_time":"09:00","end_time":"15:00","break_minutes":20,"shift_type_id":null,"origin_shift_id":"9223372036854775807"}`
	recorder := httptest.NewRecorder()
	request := seriesRequestCtx(httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body)))
	createRouter(shiftTestResource(service)).ServeHTTP(recorder, request)
	require.Equal(t, http.StatusCreated, recorder.Code, recorder.Body.String())
	require.NotNil(t, got)
	require.NotNil(t, got.OriginShiftID)
	assert.Equal(t, originShiftID, *got.OriginShiftID)
}

func TestCreateHandler_AcceptsLegacyNumericOriginShiftID(t *testing.T) {
	t.Parallel()

	var got *scheduleModel.StaffShift
	service := &fakeShiftService{
		createFn: func(_ context.Context, shift *scheduleModel.StaffShift) (*scheduleModel.StaffShift, error) {
			got = shift
			return shift, nil
		},
	}
	body := `{"staff_id":7,"date":"2026-07-07","start_time":"09:00","end_time":"15:00","break_minutes":20,"shift_type_id":null,"origin_shift_id":42}`
	recorder := httptest.NewRecorder()
	request := seriesRequestCtx(httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body)))
	createRouter(shiftTestResource(service)).ServeHTTP(recorder, request)
	require.Equal(t, http.StatusCreated, recorder.Code, recorder.Body.String())
	require.NotNil(t, got)
	require.NotNil(t, got.OriginShiftID)
	assert.Equal(t, int64(42), *got.OriginShiftID)
}

func TestMoveHandler_MapsAtomicMovePayload(t *testing.T) {
	t.Parallel()

	var got scheduleSvc.MoveShiftInput
	service := &fakeShiftService{
		moveFn: func(_ context.Context, input scheduleSvc.MoveShiftInput) (*scheduleModel.StaffShift, error) {
			got = input
			return &scheduleModel.StaffShift{}, nil
		},
	}
	body := `{"source_staff_id":7,"target_staff_id":8,"date":"2026-07-07",` +
		`"start_time":"09:00","end_time":"15:00","break_minutes":20,"shift_type_id":5}`
	recorder := httptest.NewRecorder()
	request := seriesRequestCtx(httptest.NewRequest(http.MethodPut, "/9/move", strings.NewReader(body)))
	moveRouter(shiftTestResource(service)).ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())

	assert.Equal(t, int64(9), got.ShiftID)
	assert.Equal(t, int64(7), got.SourceStaffID)
	assert.Equal(t, int64(8), got.TargetStaffID)
	assert.Equal(t, "2026-07-07", got.Date.String())
	assert.Equal(t, "09:00", timezone.NormalizeWallClock(got.StartTime).Format("15:04"))
	assert.Equal(t, "15:00", timezone.NormalizeWallClock(got.EndTime).Format("15:04"))
	assert.Equal(t, 20, got.BreakMinutes)
	require.NotNil(t, got.ShiftTypeID)
	assert.Equal(t, int64(5), *got.ShiftTypeID)
	assert.Equal(t, int64(42), got.ActorStaffID)
}

func TestMoveHandler_RequiresExplicitShiftType(t *testing.T) {
	t.Parallel()

	called := false
	service := &fakeShiftService{
		moveFn: func(context.Context, scheduleSvc.MoveShiftInput) (*scheduleModel.StaffShift, error) {
			called = true
			return &scheduleModel.StaffShift{}, nil
		},
	}
	body := `{"source_staff_id":7,"target_staff_id":8,"date":"2026-07-07",` +
		`"start_time":"09:00","end_time":"15:00","break_minutes":20}`
	recorder := httptest.NewRecorder()
	request := seriesRequestCtx(httptest.NewRequest(http.MethodPut, "/9/move", strings.NewReader(body)))
	moveRouter(shiftTestResource(service)).ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusBadRequest, recorder.Code, recorder.Body.String())
	assert.False(t, called)
}

// A full origin payload (the admin modal always sends it) maps every field onto
// ApplyOriginEdits without loss.
func TestCancellationHandler_MapsFullOriginPayload(t *testing.T) {
	t.Parallel()

	var got scheduleSvc.CancelShiftInput
	service := &fakeShiftService{
		applyFn: func(_ context.Context, input scheduleSvc.CancelShiftInput) (*scheduleSvc.CancelShiftResult, error) {
			got = input
			return &scheduleSvc.CancelShiftResult{Shift: &scheduleModel.StaffShift{}}, nil
		},
	}
	router := cancellationRouter(shiftTestResource(service))
	body := `{"cancelled": true, "change_reason": "krank",
		"start_time": "09:00", "end_time": "14:00", "break_minutes": 30, "shift_type_id": 7}`
	recorder := httptest.NewRecorder()
	request := seriesRequestCtx(httptest.NewRequest(http.MethodPut, "/5/cancellation", strings.NewReader(body)))
	router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())

	assert.True(t, got.ApplyOriginEdits)
	assert.Equal(t, 30, got.BreakMinutes)
	require.NotNil(t, got.ShiftTypeID)
	assert.Equal(t, int64(7), *got.ShiftTypeID)
	assert.Equal(t, int64(42), got.ActorStaffID, "actor must come from the acting admin's staff record")
}

// Omitting all four origin fields preserves the stored origin values: the handler
// must not flag an origin edit.
func TestCancellationHandler_OmittedOriginFieldsPreserveStoredValues(t *testing.T) {
	t.Parallel()

	var got scheduleSvc.CancelShiftInput
	service := &fakeShiftService{
		applyFn: func(_ context.Context, input scheduleSvc.CancelShiftInput) (*scheduleSvc.CancelShiftResult, error) {
			got = input
			return &scheduleSvc.CancelShiftResult{Shift: &scheduleModel.StaffShift{}}, nil
		},
	}
	router := cancellationRouter(shiftTestResource(service))
	body := `{"cancelled": true, "change_reason": "krank"}`
	recorder := httptest.NewRecorder()
	request := seriesRequestCtx(httptest.NewRequest(http.MethodPut, "/5/cancellation", strings.NewReader(body)))
	router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	assert.False(t, got.ApplyOriginEdits, "no origin field present must preserve the stored window/type/break")
}

// A partial origin payload must be rejected rather than silently zeroing the
// omitted fields on the origin shift (#1841).
func TestCancellationHandler_RejectsPartialOriginPayload(t *testing.T) {
	t.Parallel()

	called := false
	service := &fakeShiftService{
		applyFn: func(context.Context, scheduleSvc.CancelShiftInput) (*scheduleSvc.CancelShiftResult, error) {
			called = true
			return &scheduleSvc.CancelShiftResult{Shift: &scheduleModel.StaffShift{}}, nil
		},
	}
	// Each body edits the origin window but omits one of the fields the edit would
	// overwrite, so applying it would reset that field to 0/null.
	for name, body := range map[string]string{
		"missing break_minutes": `{"cancelled": true, "start_time": "09:00", "end_time": "14:00", "shift_type_id": 7}`,
		"missing shift_type_id": `{"cancelled": true, "start_time": "09:00", "end_time": "14:00", "break_minutes": 30}`,
		"missing end_time":      `{"cancelled": true, "start_time": "09:00", "break_minutes": 30, "shift_type_id": 7}`,
		"only break_minutes":    `{"cancelled": true, "break_minutes": 30}`,
		"only shift_type_id":    `{"cancelled": true, "shift_type_id": 7}`,
		"null break_minutes":    `{"cancelled": true, "start_time": "09:00", "end_time": "14:00", "break_minutes": null, "shift_type_id": 7}`,
	} {
		t.Run(name, func(t *testing.T) {
			called = false
			router := cancellationRouter(shiftTestResource(service))
			recorder := httptest.NewRecorder()
			request := seriesRequestCtx(httptest.NewRequest(http.MethodPut, "/5/cancellation", strings.NewReader(body)))
			router.ServeHTTP(recorder, request)
			assert.Equal(t, http.StatusBadRequest, recorder.Code, recorder.Body.String())
			assert.False(t, called, "the service must not be reached on a partial origin payload")
		})
	}
}

// An explicit null shift_type_id alongside the full window is a legitimate
// "clear the type" edit, not a partial payload.
func TestCancellationHandler_AcceptsExplicitNullShiftType(t *testing.T) {
	t.Parallel()

	var got scheduleSvc.CancelShiftInput
	service := &fakeShiftService{
		applyFn: func(_ context.Context, input scheduleSvc.CancelShiftInput) (*scheduleSvc.CancelShiftResult, error) {
			got = input
			return &scheduleSvc.CancelShiftResult{Shift: &scheduleModel.StaffShift{}}, nil
		},
	}
	router := cancellationRouter(shiftTestResource(service))
	body := `{"cancelled": true, "start_time": "09:00", "end_time": "14:00", "break_minutes": 0, "shift_type_id": null}`
	recorder := httptest.NewRecorder()
	request := seriesRequestCtx(httptest.NewRequest(http.MethodPut, "/5/cancellation", strings.NewReader(body)))
	router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	assert.True(t, got.ApplyOriginEdits)
	assert.Nil(t, got.ShiftTypeID, "an explicit null clears the origin's type")
	assert.Equal(t, 0, got.BreakMinutes)
}
