package studentpresencehttp_test

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/moto-ogs/project-phoenix/api"

	authz "github.com/moto-ogs/project-phoenix/auth/authorize/permissions"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-ogs/project-phoenix/api/testutil"
	"github.com/moto-ogs/project-phoenix/modules/careplan"
	"github.com/moto-ogs/project-phoenix/modules/careplan/absencerecords"
	peopleModule "github.com/moto-ogs/project-phoenix/modules/peopledirectory"
	studentpresence "github.com/moto-ogs/project-phoenix/modules/studentpresence"
	"github.com/moto-ogs/project-phoenix/modules/studentpresence/compose/presenceservice"
	studentsAPI "github.com/moto-ogs/project-phoenix/modules/studentpresence/http"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	"github.com/moto-ogs/project-phoenix/tenant"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestStudentStatusDayHandlers_CreateGetDelete(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	resource := newStatusDayTestResource(db)
	student := testpkg.CreateTestStudent(t, db, "StatusHandler", "Student", "SH1")
	router := statusDayTestRouter(resource)

	createReq := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/status-days", student.ID), map[string]any{
		"status": absencerecords.StudentStatusDaySick,
		"dates":  []string{"2026-05-25", "2026-05-26"},
	})
	createRR := executeStatusDayHandler(t, router, createReq, testutil.AdminTestClaims(statusDayAdmin(t, db)), []string{"admin:*"})
	require.Equal(t, http.StatusCreated, createRR.Code)
	assert.Contains(t, createRR.Body.String(), `"status":"sick"`)

	getReq := testutil.NewRequest("GET", fmt.Sprintf("/%d/status-days?from=2026-05-25&to=2026-05-26", student.ID), nil)
	getRR := executeStatusDayHandler(t, router, getReq, testutil.AdminTestClaims(statusDayAdmin(t, db)), []string{"admin:*"})
	require.Equal(t, http.StatusOK, getRR.Code)
	assert.Contains(t, getRR.Body.String(), `"label":"Krank"`)

	rows, err := resource.StudentStatusDayService.FindActiveByStudentAndDateRange(testpkg.Ctx(t), student.ID, calendar.NewDate(2026, 5, 25), calendar.NewDate(2026, 5, 26))
	require.NoError(t, err)
	require.Len(t, rows, 2)

	deleteReq := testutil.NewRequest("DELETE", fmt.Sprintf("/%d/status-days/%d", student.ID, rows[0].ID), nil)
	deleteRR := executeStatusDayHandler(t, router, deleteReq, testutil.AdminTestClaims(statusDayAdmin(t, db)), []string{"admin:*"})
	require.Equal(t, http.StatusOK, deleteRR.Code)
	assert.Contains(t, deleteRR.Body.String(), `"deleted":true`)

	deleteAgainReq := testutil.NewRequest("DELETE", fmt.Sprintf("/%d/status-days/%d", student.ID, rows[0].ID), nil)
	deleteAgainRR := executeStatusDayHandler(t, router, deleteAgainReq, testutil.AdminTestClaims(statusDayAdmin(t, db)), []string{"admin:*"})
	assert.Equal(t, http.StatusNotFound, deleteAgainRR.Code)
}

func TestStudentStatusDayHandlers_TodayUpdatesLiveStatusAndClearsOpposite(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	resource := newStatusDayTestResource(db, func() time.Time {
		return calendar.NewDate(2026, 8, 24).BerlinMidnight().Add(12 * time.Hour)
	})
	notifier := &recordingAbsenceNotifier{}
	resource.AbsenceNotifier = notifier
	student := testpkg.CreateTestStudent(t, db, "StatusToday", "Student", "ST1")
	router := statusDayTestRouter(resource)
	today := calendar.NewDate(2026, 8, 24).Format(statusDayDateLayout)

	sickReq := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/status-days", student.ID), map[string]any{
		"status": absencerecords.StudentStatusDaySick,
		"dates":  []string{today},
	})
	sickRR := executeStatusDayHandler(t, router, sickReq, testutil.AdminTestClaims(statusDayAdmin(t, db)), []string{"admin:*"})
	require.Equal(t, http.StatusCreated, sickRR.Code)
	require.Len(t, notifier.reports, 1)

	repeatedSickReq := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/status-days", student.ID), map[string]any{
		"status": absencerecords.StudentStatusDaySick,
		"dates":  []string{today},
	})
	repeatedSickRR := executeStatusDayHandler(t, router, repeatedSickReq, testutil.AdminTestClaims(statusDayAdmin(t, db)), []string{"admin:*"})
	require.Equal(t, http.StatusConflict, repeatedSickRR.Code)
	assert.Contains(t, repeatedSickRR.Body.String(), `"status":409`)
	assert.NotContains(t, repeatedSickRR.Body.String(), `"error"`)
	assert.Contains(t, repeatedSickRR.Body.String(), `"conflicts":[`)
	assert.Contains(t, repeatedSickRR.Body.String(), `"status":"sick"`)
	assert.Contains(t, repeatedSickRR.Body.String(), `"conflict_count":1`)
	assert.Contains(t, repeatedSickRR.Body.String(), `"code":"students.status_day_conflict"`)
	assert.Len(t, notifier.reports, 1, "re-saving the same absence must not notify twice")

	fresh, err := findStudentState(t, resource, student.ID)
	require.NoError(t, err)
	require.NotNil(t, fresh.Sick)
	assert.True(t, *fresh.Sick)
	assert.NotNil(t, fresh.SickSince)

	excusedReq := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/status-days", student.ID), map[string]any{
		"status": absencerecords.StudentStatusDayExcused,
		"dates":  []string{today},
	})
	excusedRR := executeStatusDayHandler(t, router, excusedReq, testutil.AdminTestClaims(statusDayAdmin(t, db)), []string{"admin:*"})
	require.Equal(t, http.StatusConflict, excusedRR.Code)
	require.Len(t, notifier.reports, 1, "a conflict must not notify or overwrite")

	rows, err := resource.StudentStatusDayService.FindActiveByStudentAndDateRange(testpkg.Ctx(t), student.ID, calendar.NewDate(2026, 8, 24), calendar.NewDate(2026, 8, 24))
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, absencerecords.StudentStatusDaySick, rows[0].Status)

	fresh, err = findStudentState(t, resource, student.ID)
	require.NoError(t, err)
	require.NotNil(t, fresh.Sick)
	require.NotNil(t, fresh.Excused)
	assert.True(t, *fresh.Sick)
	assert.False(t, *fresh.Excused)

	deleteSickReq := testutil.NewRequest("DELETE", fmt.Sprintf("/%d/status-days/%d", student.ID, rows[0].ID), nil)
	deleteSickRR := executeStatusDayHandler(t, router, deleteSickReq, testutil.AdminTestClaims(statusDayAdmin(t, db)), []string{"admin:*"})
	require.Equal(t, http.StatusOK, deleteSickRR.Code)

	excusedAfterDeleteReq := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/status-days", student.ID), map[string]any{
		"status": absencerecords.StudentStatusDayExcused,
		"dates":  []string{today},
	})
	excusedAfterDeleteRR := executeStatusDayHandler(t, router, excusedAfterDeleteReq, testutil.AdminTestClaims(statusDayAdmin(t, db)), []string{"admin:*"})
	require.Equal(t, http.StatusCreated, excusedAfterDeleteRR.Code)
	require.Len(t, notifier.reports, 2)
	assert.Equal(t, absencerecords.StudentStatusDayExcused, notifier.reports[1].Status)

	classTripReq := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/status-days", student.ID), map[string]any{
		"status": absencerecords.StudentStatusDayClassTrip,
		"dates":  []string{today},
	})
	classTripRR := executeStatusDayHandler(t, router, classTripReq, testutil.AdminTestClaims(statusDayAdmin(t, db)), []string{"admin:*"})
	require.Equal(t, http.StatusConflict, classTripRR.Code)

	rows, err = resource.StudentStatusDayService.FindActiveByStudentAndDateRange(testpkg.Ctx(t), student.ID, calendar.NewDate(2026, 8, 24), calendar.NewDate(2026, 8, 24))
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, absencerecords.StudentStatusDayExcused, rows[0].Status)

	deleteExcusedReq := testutil.NewRequest("DELETE", fmt.Sprintf("/%d/status-days/%d", student.ID, rows[0].ID), nil)
	deleteExcusedRR := executeStatusDayHandler(t, router, deleteExcusedReq, testutil.AdminTestClaims(statusDayAdmin(t, db)), []string{"admin:*"})
	require.Equal(t, http.StatusOK, deleteExcusedRR.Code)

	classTripAfterDeleteReq := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/status-days", student.ID), map[string]any{
		"status": absencerecords.StudentStatusDayClassTrip,
		"dates":  []string{today},
	})
	classTripAfterDeleteRR := executeStatusDayHandler(t, router, classTripAfterDeleteReq, testutil.AdminTestClaims(statusDayAdmin(t, db)), []string{"admin:*"})
	require.Equal(t, http.StatusCreated, classTripAfterDeleteRR.Code)

	rows, err = resource.StudentStatusDayService.FindActiveByStudentAndDateRange(testpkg.Ctx(t), student.ID, calendar.NewDate(2026, 8, 24), calendar.NewDate(2026, 8, 24))
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, absencerecords.StudentStatusDayClassTrip, rows[0].Status)

	fresh, err = findStudentState(t, resource, student.ID)
	require.NoError(t, err)
	require.NotNil(t, fresh.Sick)
	require.NotNil(t, fresh.Excused)
	assert.False(t, *fresh.Sick)
	assert.False(t, *fresh.Excused)
	assert.Nil(t, fresh.SickSince)
	assert.Nil(t, fresh.ExcusedSince)

	deleteReq := testutil.NewRequest("DELETE", fmt.Sprintf("/%d/status-days/%d", student.ID, rows[0].ID), nil)
	deleteRR := executeStatusDayHandler(t, router, deleteReq, testutil.AdminTestClaims(statusDayAdmin(t, db)), []string{"admin:*"})
	require.Equal(t, http.StatusOK, deleteRR.Code)

	fresh, err = findStudentState(t, resource, student.ID)
	require.NoError(t, err)
	require.NotNil(t, fresh.Sick)
	require.NotNil(t, fresh.Excused)
	assert.False(t, *fresh.Sick)
	assert.False(t, *fresh.Excused)
	assert.Nil(t, fresh.SickSince)
	assert.Nil(t, fresh.ExcusedSince)
}

func TestStudentStatusDayHandlers_InvalidRequests(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	resource := newStatusDayTestResource(db)
	student := testpkg.CreateTestStudent(t, db, "StatusInvalid", "Student", "SI1")
	router := statusDayTestRouter(resource)
	overLimitIDs := make([]int64, 501)
	for i := range overLimitIDs {
		overLimitIDs[i] = int64(i + 1)
	}

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
			body:   map[string]any{"status": absencerecords.StudentStatusDaySick, "dates": []string{"broken"}},
		},
		{
			name:   "invalid id",
			method: "DELETE",
			path:   fmt.Sprintf("/%d/status-days/broken", student.ID),
		},
		{
			name:   "bulk student limit",
			method: "POST",
			path:   "/status-days/bulk",
			body: map[string]any{
				"student_ids": overLimitIDs,
				"status":      absencerecords.StudentStatusDayClassTrip,
				"from":        "2026-05-01",
				"to":          "2026-05-02",
			},
		},
		{
			name:   "bulk range too long",
			method: "POST",
			path:   "/status-days/bulk",
			body: map[string]any{
				"student_ids": []int64{student.ID},
				"status":      absencerecords.StudentStatusDayClassTrip,
				"from":        "2026-05-01",
				"to":          "2026-06-01",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := testutil.NewAuthenticatedRequest(t, tc.method, tc.path, tc.body)
			rr := executeStatusDayHandler(t, router, req, testutil.AdminTestClaims(statusDayAdmin(t, db)), []string{"admin:*"})
			assert.Equal(t, http.StatusBadRequest, rr.Code)
		})
	}
}

func TestStudentStatusDayHandlers_RepositoryMissingAndForbidden(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	resource := newStatusDayTestResource(db)
	resource.StudentStatusDayService = nil
	student := testpkg.CreateTestStudent(t, db, "StatusMissing", "Student", "SM1")
	router := statusDayTestRouter(resource)

	getReq := testutil.NewRequest("GET", fmt.Sprintf("/%d/status-days", student.ID), nil)
	getRR := executeStatusDayHandler(t, router, getReq, testutil.AdminTestClaims(statusDayAdmin(t, db)), []string{"admin:*"})
	require.Equal(t, http.StatusOK, getRR.Code)
	assert.Contains(t, getRR.Body.String(), `"data":[]`)

	createReq := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/status-days", student.ID), map[string]any{
		"status": absencerecords.StudentStatusDaySick,
		"dates":  []string{"2026-05-25"},
	})
	createRR := executeStatusDayHandler(t, router, createReq, testutil.AdminTestClaims(statusDayAdmin(t, db)), []string{"admin:*"})
	assert.Equal(t, http.StatusInternalServerError, createRR.Code)

	deleteReq := testutil.NewRequest("DELETE", fmt.Sprintf("/%d/status-days/42", student.ID), nil)
	deleteRR := executeStatusDayHandler(t, router, deleteReq, testutil.AdminTestClaims(statusDayAdmin(t, db)), []string{"admin:*"})
	assert.Equal(t, http.StatusInternalServerError, deleteRR.Code)

	forbiddenReq := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/status-days", student.ID), map[string]any{
		"status": absencerecords.StudentStatusDaySick,
		"dates":  []string{"2026-05-25"},
	})
	forbiddenRR := executeStatusDayHandler(t, statusDayTestRouter(newStatusDayTestResource(db)), forbiddenReq, testutil.AdminTestClaims(statusDayAdmin(t, db)), []string{"users:update"})
	assert.Equal(t, http.StatusForbidden, forbiddenRR.Code)
}

func TestStudentStatusDayHandlers_RepositoryErrors(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	student := testpkg.CreateTestStudent(t, db, "StatusErrors", "Student", "SE1")
	lockExceptionDay := composeStudentRows(db).LockExceptionDay
	baseResource := newStatusDayTestResource(db)

	t.Run("get maps repository error to internal server error", func(t *testing.T) {
		baseResource.StudentStatusDayService = presenceservice.NewStatusDays(&fakeStatusDayRepo{findRangeErr: errors.New("find failed")}, nil, nil, lockExceptionDay)
		req := testutil.NewRequest("GET", fmt.Sprintf("/%d/status-days?from=2026-05-25&to=2026-05-26", student.ID), nil)
		rr := executeStatusDayHandler(t, statusDayTestRouter(baseResource), req, testutil.AdminTestClaims(statusDayAdmin(t, db)), []string{"admin:*"})
		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})

	t.Run("create maps clear error to internal server error", func(t *testing.T) {
		resource := newStatusDayTestResource(db)
		resource.StudentStatusDayService = presenceservice.NewStatusDays(&fakeStatusDayRepo{
			clearForDatesErr: errors.New("clear failed"),
			findRangeRows:    []*absencerecords.StudentStatusDay{},
		}, nil, nil, lockExceptionDay)
		req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/status-days", student.ID), map[string]any{
			"status": absencerecords.StudentStatusDaySick,
			"dates":  []string{"2026-05-25"},
		})
		rr := executeStatusDayHandler(t, statusDayTestRouter(resource), req, testutil.AdminTestClaims(statusDayAdmin(t, db)), []string{"admin:*"})
		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})

	t.Run("create maps upsert error to internal server error", func(t *testing.T) {
		resource := newStatusDayTestResource(db)
		resource.StudentStatusDayService = presenceservice.NewStatusDays(&fakeStatusDayRepo{
			upsertErr:     errors.New("upsert failed"),
			findRangeRows: []*absencerecords.StudentStatusDay{},
		}, nil, nil, lockExceptionDay)
		req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/status-days", student.ID), map[string]any{
			"status": absencerecords.StudentStatusDayExcused,
			"dates":  []string{"2026-05-25"},
		})
		rr := executeStatusDayHandler(t, statusDayTestRouter(resource), req, testutil.AdminTestClaims(statusDayAdmin(t, db)), []string{"admin:*"})
		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})

	t.Run("create maps conflict lookup error to internal server error", func(t *testing.T) {
		resource := newStatusDayTestResource(db)
		resource.StudentStatusDayService = presenceservice.NewStatusDays(&fakeStatusDayRepo{findRangeErr: errors.New("fetch failed")}, nil, nil, lockExceptionDay)
		req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/status-days", student.ID), map[string]any{
			"status": absencerecords.StudentStatusDaySick,
			"dates":  []string{"2026-05-25"},
		})
		rr := executeStatusDayHandler(t, statusDayTestRouter(resource), req, testutil.AdminTestClaims(statusDayAdmin(t, db)), []string{"admin:*"})
		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})

	t.Run("delete maps missing or foreign status day to not found", func(t *testing.T) {
		resource := newStatusDayTestResource(db)
		resource.StudentStatusDayService = presenceservice.NewStatusDays(&fakeStatusDayRepo{findByIDErr: errFakeRowNotFound}, nil, nil, lockExceptionDay)
		missingReq := testutil.NewRequest("DELETE", fmt.Sprintf("/%d/status-days/42", student.ID), nil)
		missingRR := executeStatusDayHandler(t, statusDayTestRouter(resource), missingReq, testutil.AdminTestClaims(statusDayAdmin(t, db)), []string{"admin:*"})
		assert.Equal(t, http.StatusNotFound, missingRR.Code)

		resource.StudentStatusDayService = presenceservice.NewStatusDays(&fakeStatusDayRepo{findByIDRow: &absencerecords.StudentStatusDay{StudentID: 99}}, nil, nil, lockExceptionDay)
		foreignReq := testutil.NewRequest("DELETE", fmt.Sprintf("/%d/status-days/42", student.ID), nil)
		foreignRR := executeStatusDayHandler(t, statusDayTestRouter(resource), foreignReq, testutil.AdminTestClaims(statusDayAdmin(t, db)), []string{"admin:*"})
		assert.Equal(t, http.StatusNotFound, foreignRR.Code)
	})

	t.Run("delete maps clear error to internal server error", func(t *testing.T) {
		resource := newStatusDayTestResource(db)
		resource.StudentStatusDayService = presenceservice.NewStatusDays(&fakeStatusDayRepo{
			findByIDRow:  &absencerecords.StudentStatusDay{ID: 42, StudentID: student.ID, Date: calendar.TodayDate(), Status: absencerecords.StudentStatusDaySick},
			clearByIDErr: errors.New("clear failed"),
		}, nil, nil, lockExceptionDay)
		req := testutil.NewRequest("DELETE", fmt.Sprintf("/%d/status-days/42", student.ID), nil)
		rr := executeStatusDayHandler(t, statusDayTestRouter(resource), req, testutil.AdminTestClaims(statusDayAdmin(t, db)), []string{"admin:*"})
		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})
}

// StudentRows are what the status-day suites take from the root's student
// graph: the people and companion reads the routes bind, the status-day rows
// and Care Plan's exception-day lock, recording into the test audit store.
type StudentRows struct {
	PeopleDirectory  peopleModule.Capability
	Companions       careplan.StudentCompanions
	StudentStatusDay presenceservice.StudentStatusDayRepository
	LockExceptionDay presenceservice.LockExceptionDay
}

func newStatusDayTestResource(db *bun.DB, clocks ...func() time.Time) *studentsAPI.Resource {
	rows := composeStudentRows(db)
	var clock func() time.Time
	if len(clocks) > 0 {
		clock = clocks[0]
	}
	return studentsAPI.NewResource(studentsAPI.ResourceConfig{

		PeopleDirectory: rows.PeopleDirectory,

		StudentStatusDayService: presenceservice.NewStatusDays(rows.StudentStatusDay, nil, nil, rows.LockExceptionDay, clock),
		// Register requires the tenant settings; the absence edit scope they
		// answer stays the default, so direct reports remain admin-only.
		SettingsService: &settingsDouble{},
		Logger:          slog.Default(),
		Now:             clock,
	})
}

type fakeStatusDayRepo struct {
	upsertErr           error
	clearForDatesErr    error
	clearByIDErr        error
	findByIDRow         *absencerecords.StudentStatusDay
	findByIDErr         error
	findRangeErr        error
	findRangeRows       []*absencerecords.StudentStatusDay
	findByIDsRows       []*absencerecords.StudentStatusDay
	findByIDsDate       calendar.Date
	findByIDsStudentIDs []int64
}

func (r *fakeStatusDayRepo) UpsertReported(_ context.Context, _ *absencerecords.StudentStatusDay) error {
	return r.upsertErr
}

func (r *fakeStatusDayRepo) MarkCleared(_ context.Context, _ int64, _ string, _ calendar.Date, _ time.Time, _ string, _ *int64) error {
	return nil
}

func (r *fakeStatusDayRepo) MarkClearedByID(_ context.Context, _ int64, _ time.Time, _ string, _ *int64) error {
	return r.clearByIDErr
}

func (r *fakeStatusDayRepo) MarkClearedForDates(_ context.Context, _ int64, _ string, _ []calendar.Date, _ time.Time, _ string, _ *int64) error {
	return r.clearForDatesErr
}

func (r *fakeStatusDayRepo) FindActiveByID(_ context.Context, _ int64) (*absencerecords.StudentStatusDay, error) {
	if r.findByIDErr != nil {
		return nil, r.findByIDErr
	}
	if r.findByIDRow != nil {
		return r.findByIDRow, nil
	}
	return nil, errFakeRowNotFound
}

func (r *fakeStatusDayRepo) FindActiveByStudentAndDateRange(_ context.Context, studentID int64, startDate, _ calendar.Date) ([]*absencerecords.StudentStatusDay, error) {
	if r.findRangeErr != nil {
		return nil, r.findRangeErr
	}
	if r.findRangeRows != nil {
		return r.findRangeRows, nil
	}
	return []*absencerecords.StudentStatusDay{
		{
			ID:         42,
			StudentID:  studentID,
			Date:       startDate,
			Status:     absencerecords.StudentStatusDaySick,
			ReportedAt: time.Now(),
			Source:     absencerecords.StudentStatusSourcePlanned,
		},
	}, nil
}

func (r *fakeStatusDayRepo) FindActiveByStudentIDsAndDate(_ context.Context, studentIDs []int64, date calendar.Date) ([]*absencerecords.StudentStatusDay, error) {
	r.findByIDsStudentIDs = append([]int64(nil), studentIDs...)
	r.findByIDsDate = date
	return r.findByIDsRows, nil
}

func (r *fakeStatusDayRepo) FindSignedOffByStudentIDsAndDate(_ context.Context, studentIDs []int64, date calendar.Date) ([]*absencerecords.StudentStatusDay, error) {
	r.findByIDsStudentIDs = append([]int64(nil), studentIDs...)
	r.findByIDsDate = date
	return r.findByIDsRows, nil
}

func (r *fakeStatusDayRepo) FindByStudentAndDateRange(_ context.Context, _ int64, _, _ calendar.Date) ([]*absencerecords.StudentStatusDay, error) {
	return nil, nil
}

// statusDayTestRouter mounts the projection's status-day read next to the
// resource's registered write routes, without the tenant transaction the root
// mounts around them.
func statusDayTestRouter(resource *studentsAPI.Resource) chi.Router {
	router := chi.NewRouter()
	router.Use(render.SetContentType(render.ContentTypeJSON))
	router.Get("/{id}/status-days", api.NewStudentStatusDayReadHTTPForTests(resource))
	resource.Register(router, func(next http.Handler) http.Handler { return next })
	return router
}

// statusDayDateLayout is the YYYY-MM-DD wire layout of status-day dates.
const statusDayDateLayout = "2006-01-02"

// findStudentState reads the student the routes act on through the People
// Directory the resource is bound to.
func findStudentState(t *testing.T, resource *studentsAPI.Resource, id int64) (*peopleModule.StudentState, error) {
	t.Helper()
	record, err := resource.PeopleDirectory.FindStudentRecord(testpkg.Ctx(t), id)
	if err != nil {
		return nil, err
	}
	return peopleModule.StudentStateFromRecord(record), nil
}

// recordingAbsenceNotifier records the absence reports the routes hand to
// Delivery.
type recordingAbsenceNotifier struct {
	reports []studentpresence.AbsenceReport
}

func (n *recordingAbsenceNotifier) NotifyAbsenceReported(_ context.Context, report studentpresence.AbsenceReport) error {
	n.reports = append(n.reports, report)
	return nil
}

func executeStatusDayHandler(tb testing.TB, router chi.Router, req *http.Request, claims testpkg.Claims, permissions []string) *httptest.ResponseRecorder {
	tb.Helper()
	// Claims carrying the bootstrap tenant follow the test into its own
	// tenant (#2419), mirroring testutil.WithClaims.
	claims.TenantID = testpkg.RebaseTenantID(tb, claims.TenantID)
	ctx := testpkg.WithPackageTenantRuntime(req.Context())
	ctx = testpkg.WithAuthenticatedContext(ctx, claims, permissions)
	ctx = tenant.WithTenantID(ctx, claims.TenantID)
	principal, err := authz.NewPrincipal(authz.PrincipalInput{
		AccountID: int64(claims.ID), TenantID: claims.TenantID, Roles: claims.Roles, Permissions: permissions, Admin: claims.IsAdmin,
	})
	require.NoError(tb, err)
	ctx = authz.WithPrincipal(ctx, principal)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	return rr
}

// Stubs for the issue #585 refactor interface additions — unused here.
func (r *fakeStatusDayRepo) ArchiveAndClearStatusFlag(context.Context, string, string, string, calendar.Date, time.Time, string) (int64, error) {
	return 0, nil
}

func (r *fakeStatusDayRepo) CountEffectiveDashboardAbsences(context.Context, calendar.Date) (*absencerecords.StudentStatusCounts, error) {
	return &absencerecords.StudentStatusCounts{}, nil
}

// errFakeRowNotFound is a repository not-found in the shape the routes and
// services recognise (the RepositoryNotFound marker of models/base).
var errFakeRowNotFound error = fakeRowNotFoundError{}

type fakeRowNotFoundError struct{}

func (fakeRowNotFoundError) Error() string { return "record not found" }

// RepositoryNotFound marks the repository not-found sentinel.
func (fakeRowNotFoundError) RepositoryNotFound() {}

// statusDayAdmin is a real account to sign the request in with: status-day
// writes record the reporting and clearing account (#6842).
func statusDayAdmin(t *testing.T, db *bun.DB) int {
	t.Helper()
	return int(testpkg.CreateTestAccount(t, db, "status-day-admin").ID)
}
