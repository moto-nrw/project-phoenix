package staffshifts

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/services/users/userstest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeSeriesService struct {
	createFn     func(ctx context.Context, series *scheduleModel.StaffShiftSeries) (*scheduleSvc.SeriesResult, error)
	splitFn      func(ctx context.Context, input scheduleSvc.SplitSeriesInput) (*scheduleSvc.SeriesResult, error)
	endFn        func(ctx context.Context, seriesID int64, from timezone.Date) (*scheduleSvc.SeriesResult, error)
	getFn        func(ctx context.Context, seriesID int64) (*scheduleModel.StaffShiftSeries, error)
	updateFn     func(ctx context.Context, input scheduleSvc.UpdateSeriesInput) (*scheduleSvc.SeriesResult, error)
	deviationsFn func(ctx context.Context, seriesID int64) (*scheduleSvc.SeriesDeviations, error)
}

func (f *fakeSeriesService) CreateSeries(ctx context.Context, series *scheduleModel.StaffShiftSeries) (*scheduleSvc.SeriesResult, error) {
	if f.createFn != nil {
		return f.createFn(ctx, series)
	}
	return nil, errors.New("createFn not set")
}

func (f *fakeSeriesService) SplitSeries(ctx context.Context, input scheduleSvc.SplitSeriesInput) (*scheduleSvc.SeriesResult, error) {
	if f.splitFn != nil {
		return f.splitFn(ctx, input)
	}
	return nil, errors.New("splitFn not set")
}

func (f *fakeSeriesService) EndSeries(ctx context.Context, seriesID int64, from timezone.Date) (*scheduleSvc.SeriesResult, error) {
	if f.endFn != nil {
		return f.endFn(ctx, seriesID, from)
	}
	return nil, errors.New("endFn not set")
}

func (f *fakeSeriesService) GetSeries(ctx context.Context, seriesID int64) (*scheduleModel.StaffShiftSeries, error) {
	if f.getFn != nil {
		return f.getFn(ctx, seriesID)
	}
	return nil, errors.New("getFn not set")
}

func (f *fakeSeriesService) UpdateSeries(ctx context.Context, input scheduleSvc.UpdateSeriesInput) (*scheduleSvc.SeriesResult, error) {
	if f.updateFn != nil {
		return f.updateFn(ctx, input)
	}
	return nil, errors.New("updateFn not set")
}

func (f *fakeSeriesService) SeriesDeviations(ctx context.Context, seriesID int64) (*scheduleSvc.SeriesDeviations, error) {
	if f.deviationsFn != nil {
		return f.deviationsFn(ctx, seriesID)
	}
	return nil, errors.New("deviationsFn not set")
}

// seriesTestResource wires a Resource whose editorStaffID resolves to staff
// id 42 (the person/staff chain the JWT claims normally drive).
func seriesTestResource(series scheduleSvc.StaffShiftSeriesService) *Resource {
	person := &userModels.Person{}
	person.ID = 41
	staff := &userModels.Staff{}
	staff.ID = 42
	return &Resource{
		SeriesService: series,
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

func seriesRequestCtx(request *http.Request) *http.Request {
	claims := jwt.AppClaims{ID: 7}
	return request.WithContext(context.WithValue(request.Context(), jwt.CtxClaims, claims))
}

func validSeriesBody() string {
	return `{
		"staff_id": 5,
		"weekdays": [1, 3],
		"start_time": "09:00",
		"end_time": "12:00",
		"break_minutes": 15,
		"shift_type_id": null,
		"calendar_period_id": 8,
		"week_pattern": 1,
		"valid_from": "2026-09-01",
		"valid_until": "2026-12-01"
	}`
}

func TestCreateSeriesHandler_MapsPayloadAndSkippedDates(t *testing.T) {
	var got *scheduleModel.StaffShiftSeries
	service := &fakeSeriesService{
		createFn: func(_ context.Context, series *scheduleModel.StaffShiftSeries) (*scheduleSvc.SeriesResult, error) {
			got = series
			return &scheduleSvc.SeriesResult{
				Series:       series,
				Created:      12,
				SkippedDates: []timezone.Date{timezone.NewDate(2026, time.September, 7)},
			}, nil
		},
	}
	resource := seriesTestResource(service)
	recorder := httptest.NewRecorder()
	request := seriesRequestCtx(httptest.NewRequest(http.MethodPost, "/series", strings.NewReader(validSeriesBody())))

	resource.createSeries(recorder, request)
	require.Equal(t, http.StatusCreated, recorder.Code, recorder.Body.String())

	require.NotNil(t, got)
	assert.Equal(t, int64(5), got.StaffID)
	assert.Equal(t, []int16{1, 3}, got.Weekdays)
	assert.Equal(t, 15, got.BreakMinutes)
	assert.Equal(t, int64(8), got.CalendarPeriodID)
	assert.Equal(t, scheduleModel.WeekPatternA, got.WeekPattern)
	assert.Equal(t, timezone.NewDate(2026, time.September, 1), got.ValidFrom)
	require.NotNil(t, got.ValidUntil)
	assert.Equal(t, timezone.NewDate(2026, time.December, 1), *got.ValidUntil)
	assert.Equal(t, int64(42), got.CreatedBy, "created_by must come from the acting admin's staff record")
	assert.Nil(t, got.ShiftTypeID)

	var envelope struct {
		Data SeriesResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	assert.Equal(t, 12, envelope.Data.Created)
	assert.Equal(t, []string{"2026-09-07"}, envelope.Data.SkippedDates)
}

func TestCreateSeriesHandler_RejectsBadInput(t *testing.T) {
	resource := seriesTestResource(&fakeSeriesService{})
	for name, body := range map[string]string{
		"invalid json":      `{`,
		"bad valid_from":    strings.Replace(validSeriesBody(), "2026-09-01", "September", 1),
		"bad valid_until":   strings.Replace(validSeriesBody(), "2026-12-01", "Dezember", 1),
		"bad start_time":    strings.Replace(validSeriesBody(), "09:00", "morgens", 1),
		"bad shift_type_id": strings.Replace(validSeriesBody(), `"shift_type_id": null`, `"shift_type_id": "x"`, 1),
	} {
		t.Run(name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := seriesRequestCtx(httptest.NewRequest(http.MethodPost, "/series", strings.NewReader(body)))
			resource.createSeries(recorder, request)
			assert.Equal(t, http.StatusBadRequest, recorder.Code, recorder.Body.String())
		})
	}
}

func TestCreateSeriesHandler_RequiresIdentity(t *testing.T) {
	resource := seriesTestResource(&fakeSeriesService{})
	recorder := httptest.NewRecorder()
	// No JWT claims in context: editorStaffID must reject.
	request := httptest.NewRequest(http.MethodPost, "/series", strings.NewReader(validSeriesBody()))
	resource.createSeries(recorder, request)
	assert.Equal(t, http.StatusUnauthorized, recorder.Code, recorder.Body.String())
}

func TestSeriesHandlers_MapServiceErrors(t *testing.T) {
	for name, test := range map[string]struct {
		err  error
		want int
	}{
		"not found":     {err: scheduleSvc.ErrSeriesNotFound, want: http.StatusNotFound},
		"invalid":       {err: fmt.Errorf("%w: valid from outside period", scheduleSvc.ErrSeriesInvalid), want: http.StatusBadRequest},
		"inactive type": {err: scheduleSvc.ErrShiftTypeInactive, want: http.StatusBadRequest},
		"internal":      {err: errors.New("boom"), want: http.StatusInternalServerError},
	} {
		t.Run(name, func(t *testing.T) {
			service := &fakeSeriesService{
				createFn: func(context.Context, *scheduleModel.StaffShiftSeries) (*scheduleSvc.SeriesResult, error) {
					return nil, test.err
				},
			}
			resource := seriesTestResource(service)
			recorder := httptest.NewRecorder()
			request := seriesRequestCtx(httptest.NewRequest(http.MethodPost, "/series", strings.NewReader(validSeriesBody())))
			resource.createSeries(recorder, request)
			assert.Equal(t, test.want, recorder.Code, recorder.Body.String())
		})
	}
}

func splitTestRouter(resource *Resource) chi.Router {
	router := chi.NewRouter()
	router.Put("/series/{id}/split", resource.splitSeries)
	router.Delete("/series/{id}", resource.endSeries)
	return router
}

func TestSplitSeriesHandler_InheritsOmittedFields(t *testing.T) {
	var got scheduleSvc.SplitSeriesInput
	service := &fakeSeriesService{
		splitFn: func(_ context.Context, input scheduleSvc.SplitSeriesInput) (*scheduleSvc.SeriesResult, error) {
			got = input
			successor := &scheduleModel.StaffShiftSeries{}
			successor.ID = 99
			return &scheduleSvc.SeriesResult{Series: successor, OldSeriesID: input.SeriesID, Created: 4, Deleted: 3}, nil
		},
	}
	resource := seriesTestResource(service)
	router := splitTestRouter(resource)
	// Only times + effective date: weekdays, week_pattern, shift type, and
	// notes are omitted and must reach the service as "inherit" markers.
	body := `{"effective_date": "2026-10-05", "start_time": "10:00", "end_time": "14:00", "break_minutes": 0}`
	recorder := httptest.NewRecorder()
	request := seriesRequestCtx(httptest.NewRequest(http.MethodPut, "/series/12/split", strings.NewReader(body)))
	router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())

	assert.Equal(t, int64(12), got.SeriesID)
	assert.Equal(t, timezone.NewDate(2026, time.October, 5), got.EffectiveDate)
	assert.Nil(t, got.Weekdays)
	assert.Nil(t, got.WeekPattern)
	assert.False(t, got.ShiftTypeIDSet)
	assert.Nil(t, got.Notes)
	assert.Equal(t, int64(42), got.ActorStaffID)

	var envelope struct {
		Data SeriesResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	assert.Equal(t, int64(99), envelope.Data.SeriesID)
	assert.Equal(t, int64(12), envelope.Data.OldSeriesID)
	assert.Equal(t, int64(3), envelope.Data.Deleted)
	assert.Equal(t, []string{}, envelope.Data.SkippedDates)
}

func TestSplitSeriesHandler_RejectsBadEffectiveDate(t *testing.T) {
	resource := seriesTestResource(&fakeSeriesService{})
	router := splitTestRouter(resource)
	body := `{"effective_date": "Montag", "start_time": "10:00", "end_time": "14:00", "break_minutes": 0}`
	recorder := httptest.NewRecorder()
	request := seriesRequestCtx(httptest.NewRequest(http.MethodPut, "/series/12/split", strings.NewReader(body)))
	router.ServeHTTP(recorder, request)
	assert.Equal(t, http.StatusBadRequest, recorder.Code, recorder.Body.String())
}

func TestEndSeriesHandler(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		var gotID int64
		var gotFrom timezone.Date
		service := &fakeSeriesService{
			endFn: func(_ context.Context, seriesID int64, from timezone.Date) (*scheduleSvc.SeriesResult, error) {
				gotID, gotFrom = seriesID, from
				ended := &scheduleModel.StaffShiftSeries{}
				ended.ID = seriesID
				return &scheduleSvc.SeriesResult{Series: ended, Deleted: 6}, nil
			},
		}
		router := splitTestRouter(seriesTestResource(service))
		recorder := httptest.NewRecorder()
		request := seriesRequestCtx(httptest.NewRequest(http.MethodDelete, "/series/12?from=2026-10-05", nil))
		router.ServeHTTP(recorder, request)
		require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
		assert.Equal(t, int64(12), gotID)
		assert.Equal(t, timezone.NewDate(2026, time.October, 5), gotFrom)
	})

	t.Run("missing from", func(t *testing.T) {
		router := splitTestRouter(seriesTestResource(&fakeSeriesService{}))
		recorder := httptest.NewRecorder()
		request := seriesRequestCtx(httptest.NewRequest(http.MethodDelete, "/series/12", nil))
		router.ServeHTTP(recorder, request)
		assert.Equal(t, http.StatusBadRequest, recorder.Code)
	})

	t.Run("bad from", func(t *testing.T) {
		router := splitTestRouter(seriesTestResource(&fakeSeriesService{}))
		recorder := httptest.NewRecorder()
		request := seriesRequestCtx(httptest.NewRequest(http.MethodDelete, "/series/12?from=Oktober", nil))
		router.ServeHTTP(recorder, request)
		assert.Equal(t, http.StatusBadRequest, recorder.Code)
	})

	t.Run("not found", func(t *testing.T) {
		service := &fakeSeriesService{
			endFn: func(context.Context, int64, timezone.Date) (*scheduleSvc.SeriesResult, error) {
				return nil, scheduleSvc.ErrSeriesNotFound
			},
		}
		router := splitTestRouter(seriesTestResource(service))
		recorder := httptest.NewRecorder()
		request := seriesRequestCtx(httptest.NewRequest(http.MethodDelete, "/series/12?from=2026-10-05", nil))
		router.ServeHTTP(recorder, request)
		assert.Equal(t, http.StatusNotFound, recorder.Code)
	})
}

// Out-of-range weekdays must be a 400, not a silent int16 wrap: 65538 would
// otherwise truncate to 2 and pass both model validation and the DB CHECK.
func TestCreateSeriesHandler_RejectsOutOfRangeWeekdays(t *testing.T) {
	resource := seriesTestResource(&fakeSeriesService{})
	for name, weekdays := range map[string]string{
		"zero":           "[0]",
		"eight":          "[8]",
		"int16 overflow": "[65538]",
		"negative":       "[-1]",
	} {
		t.Run(name, func(t *testing.T) {
			body := strings.Replace(validSeriesBody(), "[1, 3]", weekdays, 1)
			recorder := httptest.NewRecorder()
			request := seriesRequestCtx(httptest.NewRequest(http.MethodPost, "/series", strings.NewReader(body)))
			resource.createSeries(recorder, request)
			assert.Equal(t, http.StatusBadRequest, recorder.Code, recorder.Body.String())
			assert.Contains(t, recorder.Body.String(), "weekdays must be between 1 (Monday) and 7 (Sunday)")
		})
	}
}

func TestSplitSeriesHandler_RejectsOutOfRangeWeekdays(t *testing.T) {
	router := splitTestRouter(seriesTestResource(&fakeSeriesService{}))
	body := `{"effective_date": "2026-10-05", "weekdays": [65538], "start_time": "10:00", "end_time": "14:00", "break_minutes": 0}`
	recorder := httptest.NewRecorder()
	request := seriesRequestCtx(httptest.NewRequest(http.MethodPut, "/series/12/split", strings.NewReader(body)))
	router.ServeHTTP(recorder, request)
	assert.Equal(t, http.StatusBadRequest, recorder.Code, recorder.Body.String())
	assert.Contains(t, recorder.Body.String(), "weekdays must be between 1 (Monday) and 7 (Sunday)")
}

// #2028: whole-series edit, the rule behind a shift, and the deviation preview.

func wholeSeriesRouter(resource *Resource) chi.Router {
	router := chi.NewRouter()
	router.Get("/series/{id}", resource.getSeries)
	router.Get("/series/{id}/deviations", resource.seriesDeviations)
	router.Put("/series/{id}", resource.updateSeries)
	return router
}

func TestGetSeriesHandler_ReturnsStoredRule(t *testing.T) {
	series := &scheduleModel.StaffShiftSeries{
		StaffID:          5,
		Weekdays:         []int16{1, 3},
		StartTime:        mustClock(t, "09:00"),
		EndTime:          mustClock(t, "12:00"),
		BreakMinutes:     15,
		CalendarPeriodID: 8,
		WeekPattern:      scheduleModel.WeekPatternB,
		ValidFrom:        timezone.NewDate(2026, time.September, 1),
	}
	series.ID = 12
	until := timezone.NewDate(2026, time.December, 1)
	series.ValidUntil = &until
	resource := seriesTestResource(&fakeSeriesService{
		getFn: func(_ context.Context, id int64) (*scheduleModel.StaffShiftSeries, error) {
			assert.Equal(t, int64(12), id)
			return series, nil
		},
	})

	recorder := httptest.NewRecorder()
	request := seriesRequestCtx(httptest.NewRequest(http.MethodGet, "/series/12", nil))
	wholeSeriesRouter(resource).ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())

	var envelope struct {
		Data SeriesDetailResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	assert.Equal(t, int64(12), envelope.Data.ID)
	assert.Equal(t, []int{1, 3}, envelope.Data.Weekdays)
	assert.Equal(t, "09:00", envelope.Data.StartTime)
	assert.Equal(t, "12:00", envelope.Data.EndTime)
	assert.Equal(t, scheduleModel.WeekPatternB, envelope.Data.WeekPattern)
	assert.Equal(t, "2026-09-01", envelope.Data.ValidFrom)
	require.NotNil(t, envelope.Data.ValidUntil)
	assert.Equal(t, "2026-12-01", *envelope.Data.ValidUntil)
}

func TestUpdateSeriesHandler_MapsPayloadAndValidUntilPresence(t *testing.T) {
	var got scheduleSvc.UpdateSeriesInput
	service := &fakeSeriesService{
		updateFn: func(_ context.Context, input scheduleSvc.UpdateSeriesInput) (*scheduleSvc.SeriesResult, error) {
			got = input
			updated := &scheduleModel.StaffShiftSeries{}
			updated.ID = input.SeriesID
			return &scheduleSvc.SeriesResult{Series: updated, Created: 6, Deleted: 6}, nil
		},
	}
	router := wholeSeriesRouter(seriesTestResource(service))

	body := `{"weekdays": [1], "start_time": "14:00", "end_time": "17:00", "break_minutes": 15, "week_pattern": 0, "valid_until": null}`
	recorder := httptest.NewRecorder()
	request := seriesRequestCtx(httptest.NewRequest(http.MethodPut, "/series/12", strings.NewReader(body)))
	router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())

	assert.Equal(t, int64(12), got.SeriesID)
	assert.Equal(t, []int16{1}, got.Weekdays)
	assert.Equal(t, 15, got.BreakMinutes)
	assert.Equal(t, int64(42), got.ActorStaffID, "updated_by must come from the acting admin's staff record")
	// An explicit null clears the end date; an omitted key must not.
	assert.True(t, got.ValidUntilSet)
	assert.Nil(t, got.ValidUntil)

	recorder = httptest.NewRecorder()
	request = seriesRequestCtx(httptest.NewRequest(http.MethodPut, "/series/12",
		strings.NewReader(`{"start_time": "14:00", "end_time": "17:00", "break_minutes": 0}`)))
	router.ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	assert.False(t, got.ValidUntilSet, "an omitted valid_until keeps the stored end date")
	assert.Nil(t, got.Weekdays, "omitted weekdays keep the stored rule")
	assert.Nil(t, got.WeekPattern)
}

func TestUpdateSeriesHandler_RejectsBadInput(t *testing.T) {
	router := wholeSeriesRouter(seriesTestResource(&fakeSeriesService{}))
	for name, body := range map[string]string{
		"invalid json":    `{`,
		"bad start_time":  `{"start_time": "morgens", "end_time": "17:00", "break_minutes": 0}`,
		"bad weekday":     `{"weekdays": [9], "start_time": "14:00", "end_time": "17:00", "break_minutes": 0}`,
		"bad valid_until": `{"start_time": "14:00", "end_time": "17:00", "break_minutes": 0, "valid_until": "Dezember"}`,
	} {
		t.Run(name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			request := seriesRequestCtx(httptest.NewRequest(http.MethodPut, "/series/12", strings.NewReader(body)))
			router.ServeHTTP(recorder, request)
			assert.Equal(t, http.StatusBadRequest, recorder.Code, recorder.Body.String())
		})
	}
}

func TestSeriesDeviationsHandler_SplitsOverwrittenAndPreserved(t *testing.T) {
	resource := seriesTestResource(&fakeSeriesService{
		deviationsFn: func(_ context.Context, id int64) (*scheduleSvc.SeriesDeviations, error) {
			assert.Equal(t, int64(12), id)
			return &scheduleSvc.SeriesDeviations{
				From: timezone.NewDate(2026, time.September, 8),
				Overwritten: []scheduleSvc.SeriesDeviation{{
					Date:      timezone.NewDate(2026, time.September, 14),
					Kind:      scheduleSvc.SeriesDeviationTimeEdit,
					StartTime: mustClock(t, "11:00"),
					EndTime:   mustClock(t, "13:00"),
				}},
				Preserved: []scheduleSvc.SeriesDeviation{{
					Date: timezone.NewDate(2026, time.September, 21),
					Kind: scheduleSvc.SeriesDeviationRemoved,
				}},
			}, nil
		},
	})

	recorder := httptest.NewRecorder()
	request := seriesRequestCtx(httptest.NewRequest(http.MethodGet, "/series/12/deviations", nil))
	wholeSeriesRouter(resource).ServeHTTP(recorder, request)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())

	var envelope struct {
		Data SeriesDeviationsResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &envelope))
	assert.Equal(t, "2026-09-08", envelope.Data.From)
	require.Len(t, envelope.Data.Overwritten, 1)
	assert.Equal(t, "2026-09-14", envelope.Data.Overwritten[0].Date)
	assert.Equal(t, "time_edit", envelope.Data.Overwritten[0].Kind)
	assert.Equal(t, "11:00", envelope.Data.Overwritten[0].StartTime)
	require.Len(t, envelope.Data.Preserved, 1)
	assert.Equal(t, "removed", envelope.Data.Preserved[0].Kind)
	// A removed occurrence has no row, so it carries no window.
	assert.Empty(t, envelope.Data.Preserved[0].StartTime)
}

func mustClock(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse("15:04", value)
	require.NoError(t, err)
	return parsed
}
