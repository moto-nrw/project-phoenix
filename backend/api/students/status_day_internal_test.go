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

	usersSvc "github.com/moto-nrw/project-phoenix/services/users"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/active"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	activeService "github.com/moto-nrw/project-phoenix/services/active"
	notificationsService "github.com/moto-nrw/project-phoenix/services/notifications"
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

type recordingAbsenceNotifier struct {
	reports []notificationsService.AbsenceReport
}

func (n *recordingAbsenceNotifier) NotifyAbsenceReported(_ context.Context, report notificationsService.AbsenceReport) {
	n.reports = append(n.reports, report)
}

func TestStaffAbsenceNotificationCallbacks(t *testing.T) {
	t.Parallel()

	tenantID := testpkg.UniqueTestTenantID(t)
	const actorID = 23
	today := timezone.TodayDate()

	t.Run("planned status write keeps the whole submission together", func(t *testing.T) {
		notifier := &recordingAbsenceNotifier{}
		resource := &Resource{ResourceConfig: ResourceConfig{AbsenceNotifier: notifier}}
		ctx := tenant.WithTenantID(context.Background(), tenantID)
		ctx = context.WithValue(ctx, jwt.CtxClaims, jwt.AppClaims{ID: actorID})
		req := httptest.NewRequest(http.MethodPost, "/status-days", nil).WithContext(ctx)

		writeContext := resource.newStatusDayCreateWriteContext(
			req,
			[]string{"users:update"},
			active.StudentStatusDaySick,
			[]timezone.Date{today},
		)
		writeContext.AfterCreateCommit([]int64{41, 42})

		require.Len(t, notifier.reports, 1)
		report := notifier.reports[0]
		assert.Equal(t, tenantID, report.TenantID)
		assert.Equal(t, []int64{41, 42}, report.StudentIDs)
		assert.Equal(t, active.StudentStatusDaySick, report.Status)
		assert.Equal(t, []timezone.Date{today}, report.Dates)
		assert.False(t, report.FromParent)
		assert.Equal(t, int64(actorID), report.ActorAccountID)
	})

	t.Run("manual status change notifies only after commit", func(t *testing.T) {
		notifier := &recordingAbsenceNotifier{}
		resource := &Resource{ResourceConfig: ResourceConfig{AbsenceNotifier: notifier}}
		baseCtx := tenant.WithTenantID(context.Background(), tenantID)
		baseCtx = context.WithValue(baseCtx, jwt.CtxClaims, jwt.AppClaims{ID: actorID})
		ctx, commit := tenant.WithAfterCommitHooksForTest(baseCtx)
		excused := true

		resource.scheduleStudentUpdateWakes(
			ctx,
			tenantID,
			41,
			&UpdateStudentRequest{Excused: &excused},
			false,
			active.StudentStatusDayExcused,
			today,
		)
		assert.Empty(t, notifier.reports)
		commit()

		require.Len(t, notifier.reports, 1)
		assert.Equal(t, active.StudentStatusDayExcused, notifier.reports[0].Status)
		assert.Equal(t, int64(actorID), notifier.reports[0].ActorAccountID)
	})
}

func TestStatusDayRangeParsing(t *testing.T) {
	t.Parallel()

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

func TestStatusDayOverviewRangeUsesSingleTodaySnapshot(t *testing.T) {
	t.Parallel()

	today := timezone.NewDate(2026, 5, 25)
	from, to, err := parseStatusDayOverviewRange(
		httptest.NewRequest("GET", "/status-days", nil),
		today,
	)

	require.NoError(t, err)
	assert.Equal(t, today, from)
	assert.Equal(t, timezone.NewDate(2026, 7, 25), to)
}

func TestStatusDayDateHelpers(t *testing.T) {
	t.Parallel()

	dates, err := parseStatusDayDates([]string{"2026-05-25", "2026-05-27"})
	require.NoError(t, err)
	require.Len(t, dates, 2)
	assert.Equal(t, timezone.NewDate(2026, 5, 25), dates[0])
	assert.Equal(t, timezone.NewDate(2026, 5, 27), dates[1])

	_, err = parseStatusDayDates([]string{"2026-05-25", "broken"})
	require.ErrorContains(t, err, "invalid date format")
}

func TestCreateStudentStatusDaysRequestRejectsRangeOverOneYear(t *testing.T) {
	t.Parallel()

	req := &CreateStudentStatusDaysRequest{
		Status: active.StudentStatusDaySick,
		Dates:  []string{"2026-01-01", "2027-01-02"},
	}

	err := req.Bind(nil)

	require.ErrorContains(t, err, "cannot span more than 366 days")
}

func TestApplyAndClearLiveStatusForToday(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 25, 9, 30, 0, 0, time.UTC)
	student := &users.Student{}

	activeService.ApplyLiveStatusForToday(student, active.StudentStatusDaySick, now)
	require.NotNil(t, student.Sick)
	require.NotNil(t, student.SickSince)
	require.NotNil(t, student.Excused)
	assert.True(t, *student.Sick)
	assert.False(t, *student.Excused)
	assert.Equal(t, now, *student.SickSince)
	assert.Nil(t, student.ExcusedSince)

	activeService.ApplyLiveStatusForToday(student, active.StudentStatusDayExcused, now.Add(time.Hour))
	require.NotNil(t, student.Excused)
	require.NotNil(t, student.ExcusedSince)
	require.NotNil(t, student.Sick)
	assert.True(t, *student.Excused)
	assert.False(t, *student.Sick)
	assert.Nil(t, student.SickSince)

	activeService.ClearLiveStatusForToday(student, active.StudentStatusDayExcused)
	require.NotNil(t, student.Excused)
	assert.False(t, *student.Excused)
	assert.Nil(t, student.ExcusedSince)

	activeService.ApplyLiveStatusForToday(student, active.StudentStatusDaySick, now)
	activeService.ClearLiveStatusForToday(student, active.StudentStatusDaySick)
	require.NotNil(t, student.Sick)
	assert.False(t, *student.Sick)
	assert.Nil(t, student.SickSince)

	activeService.ApplyLiveStatusForToday(student, active.StudentStatusDaySick, now)
	activeService.ApplyLiveStatusForToday(student, active.StudentStatusDayClassTrip, now.Add(2*time.Hour))
	require.NotNil(t, student.Sick)
	require.NotNil(t, student.Excused)
	assert.False(t, *student.Sick)
	assert.False(t, *student.Excused)
	assert.Nil(t, student.SickSince)
	assert.Nil(t, student.ExcusedSince)
}

func TestStudentStatusDayResponsesAndEffectiveStatus(t *testing.T) {
	t.Parallel()

	reportedSick := time.Date(2026, 5, 25, 8, 0, 0, 0, time.UTC)
	reportedExcused := reportedSick.Add(time.Hour)
	sick := &active.StudentStatusDay{
		Model:      modelBase.Model{ID: 42},
		StudentID:  90,
		Date:       timezone.NewDate(2026, 5, 25),
		Status:     active.StudentStatusDaySick,
		ReportedAt: reportedSick,
		Source:     active.StudentStatusSourcePlanned,
	}
	excused := &active.StudentStatusDay{
		Model:      modelBase.Model{ID: 43},
		StudentID:  91,
		Date:       timezone.NewDate(2026, 5, 26),
		Status:     active.StudentStatusDayExcused,
		ReportedAt: reportedExcused,
		Source:     active.StudentStatusSourceManual,
	}

	note := "Fieber"
	sick.Note = &note
	responses := newStudentStatusDayResponses([]*active.StudentStatusDay{sick, excused})
	require.Len(t, responses, 2)
	assert.Equal(t, "Krank", responses[0].Label)
	assert.Equal(t, "2026-05-25", responses[0].Date)
	assert.Equal(t, "Entschuldigt", responses[1].Label)
	require.NotNil(t, responses[0].Note)
	assert.Equal(t, "Fieber", *responses[0].Note)

	conflictResponses := newStudentStatusDayConflictResponses([]*active.StudentStatusDay{sick})
	require.Len(t, conflictResponses, 1)
	assert.Equal(t, sick.ID, conflictResponses[0].ID)
	assert.Nil(t, conflictResponses[0].Note, "409 conflict payloads must omit free-text notes")

	studentResponses := []StudentResponse{{ID: 90}, {ID: 91}}
	applyEffectiveStatusDaysToResponses(studentResponses, []*active.StudentStatusDay{excused, sick})
	assert.True(t, studentResponses[0].Sick)
	assert.False(t, studentResponses[0].Excused)
	assert.True(t, studentResponses[1].Excused)

	classTrip := &active.StudentStatusDay{
		Model:      modelBase.Model{ID: 44},
		StudentID:  92,
		Date:       timezone.NewDate(2026, 5, 27),
		Status:     active.StudentStatusDayClassTrip,
		ReportedAt: reportedExcused.Add(time.Hour),
		Source:     active.StudentStatusSourcePlanned,
	}
	classTripResponse := StudentResponse{ID: 92, Excused: true, Sick: true}
	applyEffectiveStatusDays(&classTripResponse, []*active.StudentStatusDay{classTrip})
	assert.True(t, classTripResponse.ClassTrip)
	assert.False(t, classTripResponse.Sick)
	assert.False(t, classTripResponse.Excused)

	alreadySick := StudentResponse{ID: 91, Sick: true}
	applyEffectiveStatusDays(&alreadySick, []*active.StudentStatusDay{excused})
	assert.True(t, alreadySick.Sick)
	assert.False(t, alreadySick.Excused)

	applyEffectiveStatusDays(nil, []*active.StudentStatusDay{sick})
	applyEffectiveStatusDays(&alreadySick, nil)
	applyEffectiveStatusDaysToResponses(nil, []*active.StudentStatusDay{sick})
	applyEffectiveStatusDaysToResponses([]StudentResponse{{ID: 92}}, nil)
}

func TestApplyStatusDaysForDateUsesRequestedDate(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 25, 10, 0, 0, 0, time.UTC)
	repo := &fakeStatusDayRepo{
		findByIDsRows: []*active.StudentStatusDay{
			{
				Model:      modelBase.Model{ID: 42},
				StudentID:  90,
				Date:       timezone.DateFromTime(now),
				Status:     active.StudentStatusDaySick,
				ReportedAt: now,
				Source:     active.StudentStatusSourcePlanned,
			},
		},
	}
	resource := &Resource{ResourceConfig: ResourceConfig{StudentStatusDayService: activeService.NewStudentStatusDayServiceWithPartialAbsences(repo, nil, nil), Logger: slog.Default()}}
	responses := []StudentResponse{{ID: 90}, {ID: 91}}

	require.NoError(t, resource.applyStatusDaysForDate(context.Background(), responses, now))

	assert.Equal(t, timezone.DateFromTime(now), repo.findByIDsDate)
	assert.Equal(t, []int64{90, 91}, repo.findByIDsStudentIDs)
	assert.True(t, responses[0].Sick)
	assert.False(t, responses[0].Excused)
	assert.False(t, responses[1].Sick)
	assert.False(t, responses[1].Excused)
}

func TestResolveDayPlanningStatusDaysOverridePlans(t *testing.T) {
	t.Parallel()

	arrivalTime := time.Date(2026, 5, 25, 8, 0, 0, 0, time.UTC)
	pickupTime := time.Date(2026, 5, 25, 15, 30, 0, 0, time.UTC)
	timetableIDs := map[int64]struct{}{90: {}}

	status, reason, _ := resolveDayPlanningForDate(
		StudentResponse{ID: 90, Sick: true},
		&scheduleService.EffectiveArrivalTime{ArrivalTime: &arrivalTime},
		&scheduleService.EffectivePickupTime{PickupTime: &pickupTime},
		nil,
		timetableIDs,
		true,
	)

	assert.Equal(t, DayPlanningStatusNotComingToday, status)
	assert.Equal(t, dayPlanningReasonSick, reason)
}

func TestResolveDayPlanningActualAttendanceOverridesStatusDays(t *testing.T) {
	t.Parallel()

	checkInTime := time.Date(2026, 5, 25, 8, 0, 0, 0, time.UTC)

	status, reason, _ := resolveDayPlanningForDate(
		StudentResponse{ID: 90, Sick: true},
		nil,
		nil,
		&activeService.AttendanceStatus{Status: "checked_in", CheckInTime: &checkInTime},
		map[int64]struct{}{},
		true,
	)

	assert.Equal(t, DayPlanningStatusComesToday, status)
	assert.Equal(t, dayPlanningReasonUnplanned, reason)
}

func TestStudentStatusDayHandlers_CreateGetDelete(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	resource := newStatusDayTestResource(db)
	student := testpkg.CreateTestStudent(t, db, "StatusHandler", "Student", "SH1")
	router := statusDayTestRouter(resource)

	createReq := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/status-days", student.ID), map[string]any{
		"status": active.StudentStatusDaySick,
		"dates":  []string{"2026-05-25", "2026-05-26"},
	})
	createRR := executeStatusDayHandler(t, router, createReq, testutil.AdminTestClaims(42), []string{"admin:*"})
	require.Equal(t, http.StatusCreated, createRR.Code)
	assert.Contains(t, createRR.Body.String(), `"status":"sick"`)

	getReq := testutil.NewRequest("GET", fmt.Sprintf("/%d/status-days?from=2026-05-25&to=2026-05-26", student.ID), nil)
	getRR := executeStatusDayHandler(t, router, getReq, testutil.AdminTestClaims(42), []string{"admin:*"})
	require.Equal(t, http.StatusOK, getRR.Code)
	assert.Contains(t, getRR.Body.String(), `"label":"Krank"`)

	rows, err := resource.StudentStatusDayService.GetActiveByStudentAndDateRange(testpkg.Ctx(t), student.ID, timezone.NewDate(2026, 5, 25), timezone.NewDate(2026, 5, 26))
	require.NoError(t, err)
	require.Len(t, rows, 2)

	deleteReq := testutil.NewRequest("DELETE", fmt.Sprintf("/%d/status-days/%d", student.ID, rows[0].ID), nil)
	deleteRR := executeStatusDayHandler(t, router, deleteReq, testutil.AdminTestClaims(42), []string{"admin:*"})
	require.Equal(t, http.StatusOK, deleteRR.Code)
	assert.Contains(t, deleteRR.Body.String(), `"deleted":true`)

	deleteAgainReq := testutil.NewRequest("DELETE", fmt.Sprintf("/%d/status-days/%d", student.ID, rows[0].ID), nil)
	deleteAgainRR := executeStatusDayHandler(t, router, deleteAgainReq, testutil.AdminTestClaims(42), []string{"admin:*"})
	assert.Equal(t, http.StatusNotFound, deleteAgainRR.Code)
}

func TestStudentStatusDayHandlers_TodayUpdatesLiveStatusAndClearsOpposite(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	resource := newStatusDayTestResource(db)
	notifier := &recordingAbsenceNotifier{}
	resource.AbsenceNotifier = notifier
	student := testpkg.CreateTestStudent(t, db, "StatusToday", "Student", "ST1")
	router := statusDayTestRouter(resource)
	today := timezone.TodayDate().Format(dateFormatYYYYMMDD)

	sickReq := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/status-days", student.ID), map[string]any{
		"status": active.StudentStatusDaySick,
		"dates":  []string{today},
	})
	sickRR := executeStatusDayHandler(t, router, sickReq, testutil.AdminTestClaims(42), []string{"admin:*"})
	require.Equal(t, http.StatusCreated, sickRR.Code)
	require.Len(t, notifier.reports, 1)

	repeatedSickReq := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/status-days", student.ID), map[string]any{
		"status": active.StudentStatusDaySick,
		"dates":  []string{today},
	})
	repeatedSickRR := executeStatusDayHandler(t, router, repeatedSickReq, testutil.AdminTestClaims(42), []string{"admin:*"})
	require.Equal(t, http.StatusConflict, repeatedSickRR.Code)
	assert.Contains(t, repeatedSickRR.Body.String(), `"status":"error"`)
	assert.Contains(t, repeatedSickRR.Body.String(), `"conflicts":[`)
	assert.Contains(t, repeatedSickRR.Body.String(), `"status":"sick"`)
	assert.Contains(t, repeatedSickRR.Body.String(), `"conflict_count":1`)
	assert.Len(t, notifier.reports, 1, "re-saving the same absence must not notify twice")

	fresh, err := resource.PersonService.GetStudentByID(testpkg.Ctx(t), student.ID)
	require.NoError(t, err)
	require.NotNil(t, fresh.Sick)
	assert.True(t, *fresh.Sick)
	assert.NotNil(t, fresh.SickSince)

	excusedReq := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/status-days", student.ID), map[string]any{
		"status": active.StudentStatusDayExcused,
		"dates":  []string{today},
	})
	excusedRR := executeStatusDayHandler(t, router, excusedReq, testutil.AdminTestClaims(42), []string{"admin:*"})
	require.Equal(t, http.StatusConflict, excusedRR.Code)
	require.Len(t, notifier.reports, 1, "a conflict must not notify or overwrite")

	rows, err := resource.StudentStatusDayService.GetActiveByStudentAndDateRange(testpkg.Ctx(t), student.ID, timezone.TodayDate(), timezone.TodayDate())
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, active.StudentStatusDaySick, rows[0].Status)

	fresh, err = resource.PersonService.GetStudentByID(testpkg.Ctx(t), student.ID)
	require.NoError(t, err)
	require.NotNil(t, fresh.Sick)
	require.NotNil(t, fresh.Excused)
	assert.True(t, *fresh.Sick)
	assert.False(t, *fresh.Excused)

	deleteSickReq := testutil.NewRequest("DELETE", fmt.Sprintf("/%d/status-days/%d", student.ID, rows[0].ID), nil)
	deleteSickRR := executeStatusDayHandler(t, router, deleteSickReq, testutil.AdminTestClaims(42), []string{"admin:*"})
	require.Equal(t, http.StatusOK, deleteSickRR.Code)

	excusedAfterDeleteReq := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/status-days", student.ID), map[string]any{
		"status": active.StudentStatusDayExcused,
		"dates":  []string{today},
	})
	excusedAfterDeleteRR := executeStatusDayHandler(t, router, excusedAfterDeleteReq, testutil.AdminTestClaims(42), []string{"admin:*"})
	require.Equal(t, http.StatusCreated, excusedAfterDeleteRR.Code)
	require.Len(t, notifier.reports, 2)
	assert.Equal(t, active.StudentStatusDayExcused, notifier.reports[1].Status)

	classTripReq := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/status-days", student.ID), map[string]any{
		"status": active.StudentStatusDayClassTrip,
		"dates":  []string{today},
	})
	classTripRR := executeStatusDayHandler(t, router, classTripReq, testutil.AdminTestClaims(42), []string{"admin:*"})
	require.Equal(t, http.StatusConflict, classTripRR.Code)

	rows, err = resource.StudentStatusDayService.GetActiveByStudentAndDateRange(testpkg.Ctx(t), student.ID, timezone.TodayDate(), timezone.TodayDate())
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, active.StudentStatusDayExcused, rows[0].Status)

	deleteExcusedReq := testutil.NewRequest("DELETE", fmt.Sprintf("/%d/status-days/%d", student.ID, rows[0].ID), nil)
	deleteExcusedRR := executeStatusDayHandler(t, router, deleteExcusedReq, testutil.AdminTestClaims(42), []string{"admin:*"})
	require.Equal(t, http.StatusOK, deleteExcusedRR.Code)

	classTripAfterDeleteReq := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/status-days", student.ID), map[string]any{
		"status": active.StudentStatusDayClassTrip,
		"dates":  []string{today},
	})
	classTripAfterDeleteRR := executeStatusDayHandler(t, router, classTripAfterDeleteReq, testutil.AdminTestClaims(42), []string{"admin:*"})
	require.Equal(t, http.StatusCreated, classTripAfterDeleteRR.Code)

	rows, err = resource.StudentStatusDayService.GetActiveByStudentAndDateRange(testpkg.Ctx(t), student.ID, timezone.TodayDate(), timezone.TodayDate())
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, active.StudentStatusDayClassTrip, rows[0].Status)

	fresh, err = resource.PersonService.GetStudentByID(testpkg.Ctx(t), student.ID)
	require.NoError(t, err)
	require.NotNil(t, fresh.Sick)
	require.NotNil(t, fresh.Excused)
	assert.False(t, *fresh.Sick)
	assert.False(t, *fresh.Excused)
	assert.Nil(t, fresh.SickSince)
	assert.Nil(t, fresh.ExcusedSince)

	deleteReq := testutil.NewRequest("DELETE", fmt.Sprintf("/%d/status-days/%d", student.ID, rows[0].ID), nil)
	deleteRR := executeStatusDayHandler(t, router, deleteReq, testutil.AdminTestClaims(42), []string{"admin:*"})
	require.Equal(t, http.StatusOK, deleteRR.Code)

	fresh, err = resource.PersonService.GetStudentByID(testpkg.Ctx(t), student.ID)
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
			body:   map[string]any{"status": active.StudentStatusDaySick, "dates": []string{"broken"}},
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
				"status":      active.StudentStatusDayClassTrip,
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
				"status":      active.StudentStatusDayClassTrip,
				"from":        "2026-05-01",
				"to":          "2026-06-01",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := testutil.NewAuthenticatedRequest(t, tc.method, tc.path, tc.body)
			rr := executeStatusDayHandler(t, router, req, testutil.AdminTestClaims(42), []string{"admin:*"})
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
	getRR := executeStatusDayHandler(t, router, getReq, testutil.AdminTestClaims(42), []string{"admin:*"})
	require.Equal(t, http.StatusOK, getRR.Code)
	assert.Contains(t, getRR.Body.String(), `"data":[]`)

	createReq := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/status-days", student.ID), map[string]any{
		"status": active.StudentStatusDaySick,
		"dates":  []string{"2026-05-25"},
	})
	createRR := executeStatusDayHandler(t, router, createReq, testutil.AdminTestClaims(42), []string{"admin:*"})
	assert.Equal(t, http.StatusInternalServerError, createRR.Code)

	deleteReq := testutil.NewRequest("DELETE", fmt.Sprintf("/%d/status-days/42", student.ID), nil)
	deleteRR := executeStatusDayHandler(t, router, deleteReq, testutil.AdminTestClaims(42), []string{"admin:*"})
	assert.Equal(t, http.StatusInternalServerError, deleteRR.Code)

	forbiddenReq := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/status-days", student.ID), map[string]any{
		"status": active.StudentStatusDaySick,
		"dates":  []string{"2026-05-25"},
	})
	forbiddenRR := executeStatusDayHandler(t, statusDayTestRouter(newStatusDayTestResource(db)), forbiddenReq, testutil.AdminTestClaims(42), []string{"users:update"})
	assert.Equal(t, http.StatusForbidden, forbiddenRR.Code)
}

func TestStudentStatusDayHandlers_RepositoryErrors(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	student := testpkg.CreateTestStudent(t, db, "StatusErrors", "Student", "SE1")
	baseResource := newStatusDayTestResource(db)

	t.Run("get maps repository error to internal server error", func(t *testing.T) {
		baseResource.StudentStatusDayService = activeService.NewStudentStatusDayServiceWithPartialAbsences(&fakeStatusDayRepo{findRangeErr: errors.New("find failed")}, nil, nil)
		req := testutil.NewRequest("GET", fmt.Sprintf("/%d/status-days?from=2026-05-25&to=2026-05-26", student.ID), nil)
		rr := executeStatusDayHandler(t, statusDayTestRouter(baseResource), req, testutil.AdminTestClaims(42), []string{"admin:*"})
		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})

	t.Run("create maps clear error to internal server error", func(t *testing.T) {
		resource := newStatusDayTestResource(db)
		resource.StudentStatusDayService = activeService.NewStudentStatusDayServiceWithPartialAbsences(&fakeStatusDayRepo{
			clearForDatesErr: errors.New("clear failed"),
			findRangeRows:    []*active.StudentStatusDay{},
		}, nil, nil)
		req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/status-days", student.ID), map[string]any{
			"status": active.StudentStatusDaySick,
			"dates":  []string{"2026-05-25"},
		})
		rr := executeStatusDayHandler(t, statusDayTestRouter(resource), req, testutil.AdminTestClaims(42), []string{"admin:*"})
		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})

	t.Run("create maps upsert error to internal server error", func(t *testing.T) {
		resource := newStatusDayTestResource(db)
		resource.StudentStatusDayService = activeService.NewStudentStatusDayServiceWithPartialAbsences(&fakeStatusDayRepo{
			upsertErr:     errors.New("upsert failed"),
			findRangeRows: []*active.StudentStatusDay{},
		}, nil, nil)
		req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/status-days", student.ID), map[string]any{
			"status": active.StudentStatusDayExcused,
			"dates":  []string{"2026-05-25"},
		})
		rr := executeStatusDayHandler(t, statusDayTestRouter(resource), req, testutil.AdminTestClaims(42), []string{"admin:*"})
		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})

	t.Run("create maps conflict lookup error to internal server error", func(t *testing.T) {
		resource := newStatusDayTestResource(db)
		resource.StudentStatusDayService = activeService.NewStudentStatusDayServiceWithPartialAbsences(&fakeStatusDayRepo{findRangeErr: errors.New("fetch failed")}, nil, nil)
		req := testutil.NewAuthenticatedRequest(t, "POST", fmt.Sprintf("/%d/status-days", student.ID), map[string]any{
			"status": active.StudentStatusDaySick,
			"dates":  []string{"2026-05-25"},
		})
		rr := executeStatusDayHandler(t, statusDayTestRouter(resource), req, testutil.AdminTestClaims(42), []string{"admin:*"})
		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})

	t.Run("delete maps missing or foreign status day to not found", func(t *testing.T) {
		resource := newStatusDayTestResource(db)
		resource.StudentStatusDayService = activeService.NewStudentStatusDayServiceWithPartialAbsences(&fakeStatusDayRepo{findByIDErr: sql.ErrNoRows}, nil, nil)
		missingReq := testutil.NewRequest("DELETE", fmt.Sprintf("/%d/status-days/42", student.ID), nil)
		missingRR := executeStatusDayHandler(t, statusDayTestRouter(resource), missingReq, testutil.AdminTestClaims(42), []string{"admin:*"})
		assert.Equal(t, http.StatusNotFound, missingRR.Code)

		resource.StudentStatusDayService = activeService.NewStudentStatusDayServiceWithPartialAbsences(&fakeStatusDayRepo{findByIDRow: &active.StudentStatusDay{StudentID: 99}}, nil, nil)
		foreignReq := testutil.NewRequest("DELETE", fmt.Sprintf("/%d/status-days/42", student.ID), nil)
		foreignRR := executeStatusDayHandler(t, statusDayTestRouter(resource), foreignReq, testutil.AdminTestClaims(42), []string{"admin:*"})
		assert.Equal(t, http.StatusNotFound, foreignRR.Code)
	})

	t.Run("delete maps clear error to internal server error", func(t *testing.T) {
		resource := newStatusDayTestResource(db)
		resource.StudentStatusDayService = activeService.NewStudentStatusDayServiceWithPartialAbsences(&fakeStatusDayRepo{
			findByIDRow:  &active.StudentStatusDay{Model: modelBase.Model{ID: 42}, StudentID: student.ID, Date: timezone.TodayDate(), Status: active.StudentStatusDaySick},
			clearByIDErr: errors.New("clear failed"),
		}, nil, nil)
		req := testutil.NewRequest("DELETE", fmt.Sprintf("/%d/status-days/42", student.ID), nil)
		rr := executeStatusDayHandler(t, statusDayTestRouter(resource), req, testutil.AdminTestClaims(42), []string{"admin:*"})
		assert.Equal(t, http.StatusInternalServerError, rr.Code)
	})
}

func newStatusDayTestResource(db *bun.DB) *Resource {
	repoFactory := repositories.NewFactory(db)
	return NewResource(ResourceConfig{
		PersonService:           usersSvc.NewPersonService(usersSvc.PersonServiceDependencies{StudentRepo: repoFactory.Student}),
		StudentService:          usersSvc.NewStudentService(repoFactory.Student, repoFactory.PrivacyConsent, repoFactory.StudentCompanion, nil),
		StudentStatusDayService: activeService.NewStudentStatusDayServiceWithPartialAbsences(repoFactory.StudentStatusDay, nil, nil),
		Logger:                  slog.Default(),
		DB:                      db,
	})
}

type fakeStatusDayRepo struct {
	upsertErr           error
	clearForDatesErr    error
	clearByIDErr        error
	findByIDRow         *active.StudentStatusDay
	findByIDErr         error
	findRangeErr        error
	findRangeRows       []*active.StudentStatusDay
	findByIDsRows       []*active.StudentStatusDay
	findByIDsDate       timezone.Date
	findByIDsStudentIDs []int64
}

func (r *fakeStatusDayRepo) UpsertReported(_ context.Context, _ *active.StudentStatusDay) error {
	return r.upsertErr
}

func (r *fakeStatusDayRepo) MarkCleared(_ context.Context, _ int64, _ string, _ timezone.Date, _ time.Time, _ string) error {
	return nil
}

func (r *fakeStatusDayRepo) MarkClearedByID(_ context.Context, _ int64, _ time.Time, _ string) error {
	return r.clearByIDErr
}

func (r *fakeStatusDayRepo) MarkClearedForDates(_ context.Context, _ int64, _ string, _ []timezone.Date, _ time.Time, _ string) error {
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

func (r *fakeStatusDayRepo) FindActiveByStudentAndDateRange(_ context.Context, studentID int64, startDate, _ timezone.Date) ([]*active.StudentStatusDay, error) {
	if r.findRangeErr != nil {
		return nil, r.findRangeErr
	}
	if r.findRangeRows != nil {
		return r.findRangeRows, nil
	}
	return []*active.StudentStatusDay{
		{
			Model:      modelBase.Model{ID: 42},
			StudentID:  studentID,
			Date:       startDate,
			Status:     active.StudentStatusDaySick,
			ReportedAt: time.Now(),
			Source:     active.StudentStatusSourcePlanned,
		},
	}, nil
}

func (r *fakeStatusDayRepo) FindActiveByStudentIDsAndDate(_ context.Context, studentIDs []int64, date timezone.Date) ([]*active.StudentStatusDay, error) {
	r.findByIDsStudentIDs = append([]int64(nil), studentIDs...)
	r.findByIDsDate = date
	return r.findByIDsRows, nil
}

func (r *fakeStatusDayRepo) FindSignedOffByStudentIDsAndDate(_ context.Context, studentIDs []int64, date timezone.Date) ([]*active.StudentStatusDay, error) {
	r.findByIDsStudentIDs = append([]int64(nil), studentIDs...)
	r.findByIDsDate = date
	return r.findByIDsRows, nil
}

func (r *fakeStatusDayRepo) FindByStudentAndDateRange(_ context.Context, _ int64, _, _ timezone.Date) ([]*active.StudentStatusDay, error) {
	return nil, nil
}

func statusDayTestRouter(resource *Resource) chi.Router {
	router := chi.NewRouter()
	router.Use(render.SetContentType(render.ContentTypeJSON))
	router.Get("/{id}/status-days", resource.getStudentStatusDays)
	router.Post("/{id}/status-days", resource.createStudentStatusDays)
	router.Post("/status-days/bulk", resource.bulkCreateStudentStatusDays)
	router.Delete("/{id}/status-days/{statusDayId}", resource.deleteStudentStatusDay)
	return router
}

func executeStatusDayHandler(tb testing.TB, router chi.Router, req *http.Request, claims jwt.AppClaims, permissions []string) *httptest.ResponseRecorder {
	tb.Helper()
	// Claims carrying the bootstrap tenant follow the test into its own
	// tenant (#2419), mirroring testutil.WithClaims.
	claims.TenantID = testpkg.RebaseTenantID(tb, claims.TenantID)
	ctx := context.WithValue(req.Context(), jwt.CtxClaims, claims)
	ctx = context.WithValue(ctx, jwt.CtxPermissions, permissions)
	ctx = tenant.WithTenantID(ctx, claims.TenantID)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	return rr
}

// Stubs for the issue #585 refactor interface additions — unused here.
func (r *fakeStatusDayRepo) ArchiveAndClearStatusFlag(context.Context, string, string, string, timezone.Date, time.Time, string) (int64, error) {
	return 0, nil
}

func (r *fakeStatusDayRepo) CountEffectiveDashboardAbsences(context.Context, timezone.Date) (*active.StudentStatusCounts, error) {
	return &active.StudentStatusCounts{}, nil
}
