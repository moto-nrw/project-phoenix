package students

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/active"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestStatusDayRangeParsing(t *testing.T) {
	req := httptest.NewRequest("GET", "/status-days?from=2026-05-25&to=2026-05-29", nil)

	from, to, err := parseStatusDayRange(req)
	require.NoError(t, err)
	assert.Equal(t, "2026-05-25", from.Format(dateFormatYYYYMMDD))
	assert.Equal(t, "2026-05-29", to.Format(dateFormatYYYYMMDD))

	_, _, err = parseStatusDayRange(httptest.NewRequest("GET", "/status-days?from=broken&to=2026-05-29", nil))
	require.ErrorContains(t, err, "invalid from date format")

	_, _, err = parseStatusDayRange(httptest.NewRequest("GET", "/status-days?from=2026-05-25&to=broken", nil))
	require.ErrorContains(t, err, "invalid to date format")

	_, _, err = parseStatusDayRange(httptest.NewRequest("GET", "/status-days?from=2026-05-29&to=2026-05-25", nil))
	require.ErrorContains(t, err, "to must be after from")
}

func TestStatusDayDateHelpers(t *testing.T) {
	dates, err := parseStatusDayDates([]string{"2026-05-25", "2026-05-27"})
	require.NoError(t, err)
	require.Len(t, dates, 2)

	assert.Equal(t, "2026-05-25", minDate(dates).Format(dateFormatYYYYMMDD))
	assert.Equal(t, "2026-05-27", maxDate(dates).Format(dateFormatYYYYMMDD))
	assert.True(t, containsDate(dates, timezone.DateOfUTC(time.Date(2026, 5, 25, 15, 0, 0, 0, time.UTC))))
	assert.False(t, containsDate(dates, timezone.DateOfUTC(time.Date(2026, 5, 26, 15, 0, 0, 0, time.UTC))))

	_, err = parseStatusDayDates([]string{"2026-05-25", "broken"})
	require.ErrorContains(t, err, "invalid date format")
}

func TestApplyAndClearLiveStatusForToday(t *testing.T) {
	now := time.Date(2026, 5, 25, 9, 30, 0, 0, time.UTC)
	student := &users.Student{}

	applyLiveStatusForToday(student, active.StudentStatusDaySick, now)
	require.NotNil(t, student.Sick)
	require.NotNil(t, student.SickSince)
	require.NotNil(t, student.Excused)
	assert.True(t, *student.Sick)
	assert.False(t, *student.Excused)
	assert.Equal(t, now, *student.SickSince)
	assert.Nil(t, student.ExcusedSince)

	applyLiveStatusForToday(student, active.StudentStatusDayExcused, now.Add(time.Hour))
	require.NotNil(t, student.Excused)
	require.NotNil(t, student.ExcusedSince)
	require.NotNil(t, student.Sick)
	assert.True(t, *student.Excused)
	assert.False(t, *student.Sick)
	assert.Nil(t, student.SickSince)

	clearLiveStatusForToday(student, active.StudentStatusDayExcused)
	require.NotNil(t, student.Excused)
	assert.False(t, *student.Excused)
	assert.Nil(t, student.ExcusedSince)

	applyLiveStatusForToday(student, active.StudentStatusDaySick, now)
	clearLiveStatusForToday(student, active.StudentStatusDaySick)
	require.NotNil(t, student.Sick)
	assert.False(t, *student.Sick)
	assert.Nil(t, student.SickSince)
}

func TestStudentStatusDayResponsesAndEffectiveStatus(t *testing.T) {
	reportedSick := time.Date(2026, 5, 25, 8, 0, 0, 0, time.UTC)
	reportedExcused := reportedSick.Add(time.Hour)
	sick := &active.StudentStatusDay{
		Model:      modelBase.Model{ID: 42},
		StudentID:  90,
		Date:       time.Date(2026, 5, 25, 0, 0, 0, 0, time.UTC),
		Status:     active.StudentStatusDaySick,
		ReportedAt: reportedSick,
		Source:     active.StudentStatusSourcePlanned,
	}
	excused := &active.StudentStatusDay{
		Model:      modelBase.Model{ID: 43},
		StudentID:  91,
		Date:       time.Date(2026, 5, 26, 0, 0, 0, 0, time.UTC),
		Status:     active.StudentStatusDayExcused,
		ReportedAt: reportedExcused,
		Source:     active.StudentStatusSourceManual,
	}

	responses := newStudentStatusDayResponses([]*active.StudentStatusDay{sick, excused})
	require.Len(t, responses, 2)
	assert.Equal(t, "Krank", responses[0].Label)
	assert.Equal(t, "2026-05-25", responses[0].Date)
	assert.Equal(t, "Entschuldigt", responses[1].Label)

	studentResponses := []StudentResponse{{ID: 90}, {ID: 91}}
	applyEffectiveStatusDaysToResponses(studentResponses, []*active.StudentStatusDay{excused, sick})
	assert.True(t, studentResponses[0].Sick)
	assert.False(t, studentResponses[0].Excused)
	assert.True(t, studentResponses[1].Excused)

	alreadySick := StudentResponse{ID: 91, Sick: true}
	applyEffectiveStatusDays(&alreadySick, []*active.StudentStatusDay{excused})
	assert.True(t, alreadySick.Sick)
	assert.False(t, alreadySick.Excused)

	applyEffectiveStatusDays(nil, []*active.StudentStatusDay{sick})
	applyEffectiveStatusDays(&alreadySick, nil)
	applyEffectiveStatusDaysToResponses(nil, []*active.StudentStatusDay{sick})
	applyEffectiveStatusDaysToResponses([]StudentResponse{{ID: 92}}, nil)
}

func TestStudentStatusDayHandlers_CreateGetDelete(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	resource := newStatusDayTestResource(db)
	student := testpkg.CreateTestStudent(t, db, "StatusHandler", "Student", "SH1")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID)
	router := statusDayTestRouter(resource)

	createReq := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/status-days", student.ID), map[string]any{
		"status": active.StudentStatusDaySick,
		"dates":  []string{"2026-05-25", "2026-05-26"},
	})
	createRR := executeStatusDayHandler(router, createReq, testutil.AdminTestClaims(42), []string{"admin:*"})
	require.Equal(t, http.StatusCreated, createRR.Code)
	assert.Contains(t, createRR.Body.String(), `"status":"sick"`)

	getReq := testutil.NewRequest("GET", fmt.Sprintf("/%d/status-days?from=2026-05-25&to=2026-05-26", student.ID), nil)
	getRR := executeStatusDayHandler(router, getReq, testutil.AdminTestClaims(42), []string{"admin:*"})
	require.Equal(t, http.StatusOK, getRR.Code)
	assert.Contains(t, getRR.Body.String(), `"label":"Krank"`)

	rows, err := resource.StudentStatusDayRepo.FindActiveByStudentAndDateRange(testpkg.TenantContext(1), student.ID, time.Date(2026, 5, 25, 0, 0, 0, 0, time.UTC), time.Date(2026, 5, 26, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	require.Len(t, rows, 2)

	deleteReq := testutil.NewRequest("DELETE", fmt.Sprintf("/%d/status-days/%d", student.ID, rows[0].ID), nil)
	deleteRR := executeStatusDayHandler(router, deleteReq, testutil.AdminTestClaims(42), []string{"admin:*"})
	require.Equal(t, http.StatusOK, deleteRR.Code)
	assert.Contains(t, deleteRR.Body.String(), `"deleted":true`)

	deleteAgainReq := testutil.NewRequest("DELETE", fmt.Sprintf("/%d/status-days/%d", student.ID, rows[0].ID), nil)
	deleteAgainRR := executeStatusDayHandler(router, deleteAgainReq, testutil.AdminTestClaims(42), []string{"admin:*"})
	assert.Equal(t, http.StatusNotFound, deleteAgainRR.Code)
}

func TestStudentStatusDayHandlers_TodayUpdatesLiveStatusAndClearsOpposite(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	resource := newStatusDayTestResource(db)
	student := testpkg.CreateTestStudent(t, db, "StatusToday", "Student", "ST1")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID)
	router := statusDayTestRouter(resource)
	today := timezone.DateOfUTC(time.Now()).Format(dateFormatYYYYMMDD)

	sickReq := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/status-days", student.ID), map[string]any{
		"status": active.StudentStatusDaySick,
		"dates":  []string{today},
	})
	sickRR := executeStatusDayHandler(router, sickReq, testutil.AdminTestClaims(42), []string{"admin:*"})
	require.Equal(t, http.StatusCreated, sickRR.Code)

	fresh, err := resource.StudentRepo.FindByID(testpkg.TenantContext(1), student.ID)
	require.NoError(t, err)
	require.NotNil(t, fresh.Sick)
	assert.True(t, *fresh.Sick)
	assert.NotNil(t, fresh.SickSince)

	excusedReq := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/status-days", student.ID), map[string]any{
		"status": active.StudentStatusDayExcused,
		"dates":  []string{today},
	})
	excusedRR := executeStatusDayHandler(router, excusedReq, testutil.AdminTestClaims(42), []string{"admin:*"})
	require.Equal(t, http.StatusCreated, excusedRR.Code)

	rows, err := resource.StudentStatusDayRepo.FindActiveByStudentAndDateRange(testpkg.TenantContext(1), student.ID, timezone.DateOfUTC(time.Now()), timezone.DateOfUTC(time.Now()))
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, active.StudentStatusDayExcused, rows[0].Status)

	fresh, err = resource.StudentRepo.FindByID(testpkg.TenantContext(1), student.ID)
	require.NoError(t, err)
	require.NotNil(t, fresh.Sick)
	require.NotNil(t, fresh.Excused)
	assert.False(t, *fresh.Sick)
	assert.True(t, *fresh.Excused)
	assert.Nil(t, fresh.SickSince)
	assert.NotNil(t, fresh.ExcusedSince)

	deleteReq := testutil.NewRequest("DELETE", fmt.Sprintf("/%d/status-days/%d", student.ID, rows[0].ID), nil)
	deleteRR := executeStatusDayHandler(router, deleteReq, testutil.AdminTestClaims(42), []string{"admin:*"})
	require.Equal(t, http.StatusOK, deleteRR.Code)

	fresh, err = resource.StudentRepo.FindByID(testpkg.TenantContext(1), student.ID)
	require.NoError(t, err)
	require.NotNil(t, fresh.Excused)
	assert.False(t, *fresh.Excused)
	assert.Nil(t, fresh.ExcusedSince)
}

func TestStudentStatusDayHandlers_InvalidRequests(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	resource := newStatusDayTestResource(db)
	student := testpkg.CreateTestStudent(t, db, "StatusInvalid", "Student", "SI1")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID)
	router := statusDayTestRouter(resource)

	cases := []struct {
		name   string
		method string
		path   string
		body   map[string]any
	}{
		{
			name:   "invalid range",
			method: "GET",
			path:   fmt.Sprintf("/%d/status-days?from=2026-05-30&to=2026-05-25", student.ID),
		},
		{
			name:   "invalid status",
			method: "POST",
			path:   fmt.Sprintf("/%d/status-days", student.ID),
			body:   map[string]any{"status": "vacation", "dates": []string{"2026-05-25"}},
		},
		{
			name:   "invalid date",
			method: "POST",
			path:   fmt.Sprintf("/%d/status-days", student.ID),
			body:   map[string]any{"status": active.StudentStatusDaySick, "dates": []string{"broken"}},
		},
		{
			name:   "invalid id",
			method: "DELETE",
			path:   fmt.Sprintf("/%d/status-days/broken", student.ID),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := testutil.NewAuthenticatedRequest(t, tc.method, tc.path, tc.body)
			rr := executeStatusDayHandler(router, req, testutil.AdminTestClaims(42), []string{"admin:*"})
			assert.Equal(t, http.StatusBadRequest, rr.Code)
		})
	}
}

func TestStudentStatusDayHandlers_RepositoryMissingAndForbidden(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	resource := newStatusDayTestResource(db)
	resource.StudentStatusDayRepo = nil
	student := testpkg.CreateTestStudent(t, db, "StatusMissing", "Student", "SM1")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID)
	router := statusDayTestRouter(resource)

	getReq := testutil.NewRequest("GET", fmt.Sprintf("/%d/status-days", student.ID), nil)
	getRR := executeStatusDayHandler(router, getReq, testutil.AdminTestClaims(42), []string{"admin:*"})
	require.Equal(t, http.StatusOK, getRR.Code)
	assert.Contains(t, getRR.Body.String(), `"data":[]`)

	createReq := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/status-days", student.ID), map[string]any{
		"status": active.StudentStatusDaySick,
		"dates":  []string{"2026-05-25"},
	})
	createRR := executeStatusDayHandler(router, createReq, testutil.AdminTestClaims(42), []string{"admin:*"})
	assert.Equal(t, http.StatusInternalServerError, createRR.Code)

	deleteReq := testutil.NewRequest("DELETE", fmt.Sprintf("/%d/status-days/42", student.ID), nil)
	deleteRR := executeStatusDayHandler(router, deleteReq, testutil.AdminTestClaims(42), []string{"admin:*"})
	assert.Equal(t, http.StatusInternalServerError, deleteRR.Code)

	forbiddenReq := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/status-days", student.ID), map[string]any{
		"status": active.StudentStatusDaySick,
		"dates":  []string{"2026-05-25"},
	})
	forbiddenRR := executeStatusDayHandler(statusDayTestRouter(newStatusDayTestResource(db)), forbiddenReq, testutil.AdminTestClaims(42), []string{"users:update"})
	assert.Equal(t, http.StatusForbidden, forbiddenRR.Code)
}

func TestStudentStatusDayHandlers_RepositoryErrors(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	student := testpkg.CreateTestStudent(t, db, "StatusErrors", "Student", "SE1")
	defer testpkg.CleanupActivityFixtures(t, db, student.ID)
	baseResource := newStatusDayTestResource(db)

	t.Run("get maps repository error to internal server error", func(t *testing.T) {
		baseResource.StudentStatusDayRepo = &fakeStatusDayRepo{findRangeErr: errors.New("find failed")}
		req := testutil.NewRequest("GET", fmt.Sprintf("/%d/status-days?from=2026-05-25&to=2026-05-26", student.ID), nil)
		rr := executeStatusDayHandler(statusDayTestRouter(baseResource), req, testutil.AdminTestClaims(42), []string{"admin:*"})
		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})

	t.Run("create maps clear error to internal server error", func(t *testing.T) {
		resource := newStatusDayTestResource(db)
		resource.StudentStatusDayRepo = &fakeStatusDayRepo{clearForDatesErr: errors.New("clear failed")}
		req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/status-days", student.ID), map[string]any{
			"status": active.StudentStatusDaySick,
			"dates":  []string{"2026-05-25"},
		})
		rr := executeStatusDayHandler(statusDayTestRouter(resource), req, testutil.AdminTestClaims(42), []string{"admin:*"})
		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})

	t.Run("create maps upsert error to internal server error", func(t *testing.T) {
		resource := newStatusDayTestResource(db)
		resource.StudentStatusDayRepo = &fakeStatusDayRepo{upsertErr: errors.New("upsert failed")}
		req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/status-days", student.ID), map[string]any{
			"status": active.StudentStatusDayExcused,
			"dates":  []string{"2026-05-25"},
		})
		rr := executeStatusDayHandler(statusDayTestRouter(resource), req, testutil.AdminTestClaims(42), []string{"admin:*"})
		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})

	t.Run("create maps response fetch error to internal server error", func(t *testing.T) {
		resource := newStatusDayTestResource(db)
		resource.StudentStatusDayRepo = &fakeStatusDayRepo{findRangeErr: errors.New("fetch failed")}
		req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/status-days", student.ID), map[string]any{
			"status": active.StudentStatusDaySick,
			"dates":  []string{"2026-05-25"},
		})
		rr := executeStatusDayHandler(statusDayTestRouter(resource), req, testutil.AdminTestClaims(42), []string{"admin:*"})
		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})

	t.Run("delete maps missing or foreign status day to not found", func(t *testing.T) {
		resource := newStatusDayTestResource(db)
		resource.StudentStatusDayRepo = &fakeStatusDayRepo{findByIDErr: sql.ErrNoRows}
		missingReq := testutil.NewRequest("DELETE", fmt.Sprintf("/%d/status-days/42", student.ID), nil)
		missingRR := executeStatusDayHandler(statusDayTestRouter(resource), missingReq, testutil.AdminTestClaims(42), []string{"admin:*"})
		assert.Equal(t, http.StatusNotFound, missingRR.Code)

		resource.StudentStatusDayRepo = &fakeStatusDayRepo{findByIDRow: &active.StudentStatusDay{StudentID: 99}}
		foreignReq := testutil.NewRequest("DELETE", fmt.Sprintf("/%d/status-days/42", student.ID), nil)
		foreignRR := executeStatusDayHandler(statusDayTestRouter(resource), foreignReq, testutil.AdminTestClaims(42), []string{"admin:*"})
		assert.Equal(t, http.StatusNotFound, foreignRR.Code)
	})

	t.Run("delete maps clear error to internal server error", func(t *testing.T) {
		resource := newStatusDayTestResource(db)
		resource.StudentStatusDayRepo = &fakeStatusDayRepo{
			findByIDRow:  &active.StudentStatusDay{Model: modelBase.Model{ID: 42}, StudentID: student.ID, Date: time.Now(), Status: active.StudentStatusDaySick},
			clearByIDErr: errors.New("clear failed"),
		}
		req := testutil.NewRequest("DELETE", fmt.Sprintf("/%d/status-days/42", student.ID), nil)
		rr := executeStatusDayHandler(statusDayTestRouter(resource), req, testutil.AdminTestClaims(42), []string{"admin:*"})
		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})
}

func newStatusDayTestResource(db *bun.DB) *Resource {
	repoFactory := repositories.NewFactory(db)
	return NewResource(ResourceConfig{
		StudentRepo:          repoFactory.Student,
		StudentStatusDayRepo: repoFactory.StudentStatusDay,
		Logger:               slog.Default(),
		DB:                   db,
	})
}

type fakeStatusDayRepo struct {
	upsertErr        error
	clearForDatesErr error
	clearByIDErr     error
	findByIDRow      *active.StudentStatusDay
	findByIDErr      error
	findRangeErr     error
}

func (r *fakeStatusDayRepo) UpsertReported(_ context.Context, _ *active.StudentStatusDay) error {
	return r.upsertErr
}

func (r *fakeStatusDayRepo) MarkCleared(_ context.Context, _ int64, _ string, _ time.Time, _ time.Time, _ string) error {
	return nil
}

func (r *fakeStatusDayRepo) MarkClearedByID(_ context.Context, _ int64, _ time.Time, _ string) error {
	return r.clearByIDErr
}

func (r *fakeStatusDayRepo) MarkClearedForDates(_ context.Context, _ int64, _ string, _ []time.Time, _ time.Time, _ string) error {
	return r.clearForDatesErr
}

func (r *fakeStatusDayRepo) FindActiveByID(_ context.Context, _ int64) (*active.StudentStatusDay, error) {
	if r.findByIDErr != nil {
		return nil, r.findByIDErr
	}
	if r.findByIDRow != nil {
		return r.findByIDRow, nil
	}
	return nil, sql.ErrNoRows
}

func (r *fakeStatusDayRepo) FindActiveByStudentAndDateRange(_ context.Context, studentID int64, startDate, _ time.Time) ([]*active.StudentStatusDay, error) {
	if r.findRangeErr != nil {
		return nil, r.findRangeErr
	}
	return []*active.StudentStatusDay{
		{
			Model:      modelBase.Model{ID: 42},
			StudentID:  studentID,
			Date:       timezone.DateOfUTC(startDate),
			Status:     active.StudentStatusDaySick,
			ReportedAt: time.Now(),
			Source:     active.StudentStatusSourcePlanned,
		},
	}, nil
}

func (r *fakeStatusDayRepo) FindActiveByStudentIDsAndDate(_ context.Context, _ []int64, _ time.Time) ([]*active.StudentStatusDay, error) {
	return nil, nil
}

func (r *fakeStatusDayRepo) FindByStudentAndDateRange(_ context.Context, _ int64, _, _ time.Time) ([]*active.StudentStatusDay, error) {
	return nil, nil
}

func statusDayTestRouter(resource *Resource) chi.Router {
	router := chi.NewRouter()
	router.Use(render.SetContentType(render.ContentTypeJSON))
	router.Get("/{id}/status-days", resource.getStudentStatusDays)
	router.Post("/{id}/status-days", resource.createStudentStatusDays)
	router.Delete("/{id}/status-days/{statusDayId}", resource.deleteStudentStatusDay)
	return router
}

func executeStatusDayHandler(router chi.Router, req *http.Request, claims jwt.AppClaims, permissions []string) *httptest.ResponseRecorder {
	ctx := context.WithValue(req.Context(), jwt.CtxClaims, claims)
	ctx = context.WithValue(ctx, jwt.CtxPermissions, permissions)
	ctx = tenant.WithTenantID(ctx, claims.TenantID)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	return rr
}
