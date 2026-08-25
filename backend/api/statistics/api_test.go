// Package statistics_test drives the production Resource.Router() so the
// full middleware chain (JWT → tenant → permissions → tenant tx) runs as on
// the real server. It pins the permission pair, the range validation, the
// care-day arithmetic with a closing day and a holiday period, the absence
// categories, the room utilization window semantics, and the export audit.
package statistics_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	statisticsAPI "github.com/moto-nrw/project-phoenix/api/statistics"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/services"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

type testContext struct {
	db       *bun.DB
	services *services.Factory
	resource *statisticsAPI.Resource
}

func setupTestContext(t *testing.T) *testContext {
	t.Helper()
	db, svc := testutil.SetupAPITest(t)
	return &testContext{
		db:       db,
		services: svc,
		resource: statisticsAPI.NewResource(svc.Statistics, svc.ListExport, db, slog.Default()),
	}
}

func claimsFor(tb testing.TB, accountID int64) jwt.AppClaims {
	return jwt.AppClaims{
		ID:        int(accountID),
		Sub:       "statistik@example.com",
		Username:  "statistik",
		FirstName: "Statistik",
		LastName:  "Tester",
		Roles:     []string{"admin"},
		TenantID:  testpkg.Tenant(tb),
	}
}

var reportPermissions = []string{permissions.ConfigRead, permissions.UsersRead}

func authExec(t *testing.T, tc *testContext, req *http.Request, claims jwt.AppClaims, perms []string) *httptest.ResponseRecorder {
	t.Helper()
	claims.Permissions = perms
	req.Header.Set("Authorization", "Bearer "+testutil.MintTestJWT(t, claims))
	return testutil.ExecuteRequest(tc.resource.Router(), req)
}

type reportPayload struct {
	Data struct {
		From         string `json:"from"`
		To           string `json:"to"`
		CareDays     int    `json:"care_days"`
		ExcludedDays struct {
			Total          int `json:"total"`
			PublicHolidays int `json:"public_holidays"`
			ClosingDays    int `json:"closing_days"`
			HolidayPeriods int `json:"holiday_periods"`
		} `json:"excluded_days"`
		Totals struct {
			StudentCount   int      `json:"student_count"`
			AttendanceRate *float64 `json:"attendance_rate"`
		} `json:"totals"`
		Students []struct {
			StudentID       string   `json:"student_id"`
			LastName        string   `json:"last_name"`
			GroupName       string   `json:"group_name"`
			PresentDays     int      `json:"present_days"`
			SickDays        int      `json:"sick_days"`
			ExcusedDays     int      `json:"excused_days"`
			UnexplainedDays int      `json:"unexplained_days"`
			AttendanceRate  *float64 `json:"attendance_rate"`
		} `json:"students"`
		Groups []struct {
			GroupID        string   `json:"group_id"`
			Name           string   `json:"name"`
			StudentCount   int      `json:"student_count"`
			AttendanceRate *float64 `json:"attendance_rate"`
		} `json:"groups"`
		Rooms []struct {
			RoomID                 string   `json:"room_id"`
			Name                   string   `json:"name"`
			DaysUsed               int      `json:"days_used"`
			DistinctStudents       int      `json:"distinct_students"`
			StudentMinutes         int      `json:"student_minutes"`
			PeakOccupancy          int      `json:"peak_occupancy"`
			PeakUtilizationPercent *float64 `json:"peak_utilization_percent"`
		} `json:"rooms"`
		RoomDataDays int `json:"room_data_days"`
	} `json:"data"`
}

// Fixed past week without NRW public holidays: Mon 2026-06-08 .. Fri 2026-06-12.
var (
	weekFrom = timezone.NewDate(2026, 6, 8)
	weekTo   = timezone.NewDate(2026, 6, 12)
)

func insertAttendance(t *testing.T, db *bun.DB, tenantID, studentID, deviceID int64, date timezone.Date) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	checkIn := date.BerlinMidnight().Add(8 * time.Hour)
	checkOut := checkIn.Add(6 * time.Hour)
	row := &activeModels.Attendance{
		StudentID:    studentID,
		Date:         date,
		CheckInTime:  checkIn,
		CheckOutTime: &checkOut,
		DeviceID:     deviceID,
	}
	row.SetTenantID(tenantID)
	_, err := db.NewInsert().Model(row).ModelTableExpr(`active.attendance`).Exec(ctx)
	require.NoError(t, err)
}

func insertStatusDay(t *testing.T, db *bun.DB, tenantID, studentID int64, date timezone.Date, status string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	row := &activeModels.StudentStatusDay{
		StudentID:  studentID,
		Date:       date,
		Status:     status,
		ReportedAt: date.BerlinMidnight().Add(7 * time.Hour),
		Source:     activeModels.StudentStatusSourceManual,
	}
	row.SetTenantID(tenantID)
	_, err := db.NewInsert().Model(row).ModelTableExpr(`active.student_status_days`).Exec(ctx)
	require.NoError(t, err)
}

func insertAcceptedPrivacyConsent(t *testing.T, db *bun.DB, tenantID, studentID int64, retentionDays int) {
	t.Helper()
	acceptedAt := time.Now()
	consent := &userModels.PrivacyConsent{
		StudentID:         studentID,
		PolicyVersion:     "statistics-test",
		Accepted:          true,
		AcceptedAt:        &acceptedAt,
		DataRetentionDays: retentionDays,
	}
	consent.SetTenantID(tenantID)
	_, err := db.NewInsert().Model(consent).ModelTableExpr(`users.privacy_consents`).Exec(t.Context())
	require.NoError(t, err)
}

func insertHolidayPeriod(t *testing.T, db *bun.DB, tenantID int64, name string, from, to timezone.Date) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	row := &scheduleModels.CalendarPeriod{
		Name:            name,
		PeriodType:      scheduleModels.PeriodTypeHoliday,
		StartDate:       from,
		EndDate:         to,
		WeekCycleLength: 1,
		IsActive:        true,
	}
	row.SetTenantID(tenantID)
	_, err := db.NewInsert().Model(row).ModelTableExpr(`schedule.calendar_periods`).Exec(ctx)
	require.NoError(t, err)
}

func TestStatisticsReport_RequiresBothPermissions(t *testing.T) {
	t.Parallel()
	tc := setupTestContext(t)
	_, account := testpkg.CreateTestStaffWithAccountForTenant(t, tc.db, testpkg.Tenant(t), "Nur", "Lesen")
	claims := claimsFor(t, account.ID)

	for _, perms := range [][]string{
		{permissions.ConfigRead},
		{permissions.UsersRead},
		{},
	} {
		req := httptest.NewRequest(http.MethodGet, "/report?from=2026-06-08&to=2026-06-12", nil)
		rec := authExec(t, tc, req, claims, perms)
		assert.Equal(t, http.StatusForbidden, rec.Code, "permissions %v must not open the report", perms)
	}
}

func TestStatisticsReport_RejectsInvalidRanges(t *testing.T) {
	t.Parallel()
	tc := setupTestContext(t)
	_, account := testpkg.CreateTestStaffWithAccountForTenant(t, tc.db, testpkg.Tenant(t), "Range", "Tester")
	claims := claimsFor(t, account.ID)

	future := timezone.TodayDate().AddDays(1)
	for name, query := range map[string]string{
		"missing":   "",
		"reversed":  "from=2026-06-12&to=2026-06-08",
		"future":    "from=" + future.AddDays(-3).String() + "&to=" + future.String(),
		"too_long":  "from=2025-01-01&to=2026-01-02",
		"bad_group": "from=2026-06-08&to=2026-06-12&group_id=abc",
	} {
		req := httptest.NewRequest(http.MethodGet, "/report?"+query, nil)
		rec := authExec(t, tc, req, claims, reportPermissions)
		assert.Equal(t, http.StatusBadRequest, rec.Code, name)
	}
}

func TestStatisticsReport_ComputesQuotasAndRooms(t *testing.T) {
	t.Parallel()
	tc := setupTestContext(t)
	tenantID := testpkg.Tenant(t)
	ctx := testpkg.Ctx(t)
	_, account := testpkg.CreateTestStaffWithAccountForTenant(t, tc.db, tenantID, "Quote", "Tester")
	claims := claimsFor(t, account.ID)

	group := testpkg.CreateTestEducationGroup(t, tc.db, "Sonnen")
	anna := testpkg.CreateTestStudent(t, tc.db, "Anna", "Anwesend", "1a")
	bert := testpkg.CreateTestStudent(t, tc.db, "Bert", "Bettruhe", "1a")
	insertAcceptedPrivacyConsent(t, tc.db, tenantID, anna.ID, 30)
	insertAcceptedPrivacyConsent(t, tc.db, tenantID, bert.ID, 30)
	for _, st := range []int64{anna.ID, bert.ID} {
		_, err := tc.db.NewUpdate().TableExpr("users.students").Set("group_id = ?", group.ID).Where("id = ?", st).Exec(ctx)
		require.NoError(t, err)
	}
	device := testpkg.CreateTestDevice(t, tc.db, "stat-device")

	// Tue 09.06. is a closing day, Fri 12.06. lies in a holiday period.
	// Care days: Mon, Wed, Thu = 3.
	require.NoError(t, tc.services.ClosingDays.Create(ctx, &scheduleModels.ClosingDay{
		StartDate: timezone.NewDate(2026, 6, 9),
		EndDate:   timezone.NewDate(2026, 6, 9),
		Reason:    "Pädagogischer Tag",
	}))
	insertHolidayPeriod(t, tc.db, tenantID, "Pfingstferien", timezone.NewDate(2026, 6, 12), timezone.NewDate(2026, 6, 14))

	// Anna: present Mon + Wed, unexplained Thu. Attendance on the closing
	// day must not count anywhere.
	insertAttendance(t, tc.db, tenantID, anna.ID, device.ID, timezone.NewDate(2026, 6, 8))
	insertAttendance(t, tc.db, tenantID, anna.ID, device.ID, timezone.NewDate(2026, 6, 9))
	insertAttendance(t, tc.db, tenantID, anna.ID, device.ID, timezone.NewDate(2026, 6, 10))
	// Bert: sick Mon (sick beats an excused row on the same day), class trip
	// Wed (counts as excused), present Thu.
	insertStatusDay(t, tc.db, tenantID, bert.ID, timezone.NewDate(2026, 6, 8), activeModels.StudentStatusDaySick)
	insertStatusDay(t, tc.db, tenantID, bert.ID, timezone.NewDate(2026, 6, 8), activeModels.StudentStatusDayExcused)
	insertStatusDay(t, tc.db, tenantID, bert.ID, timezone.NewDate(2026, 6, 10), activeModels.StudentStatusDayClassTrip)
	insertAttendance(t, tc.db, tenantID, bert.ID, device.ID, timezone.NewDate(2026, 6, 11))

	// Room: two overlapping visits on Monday (peak 2), one visit that
	// started the evening before the window and ran into it (clamped).
	room := testpkg.CreateTestRoom(t, tc.db, "Bauraum")
	activity := testpkg.CreateTestActivityGroup(t, tc.db, "Bauen")
	session := testpkg.CreateTestActiveGroup(t, tc.db, activity.ID, room.ID)
	monday := timezone.NewDate(2026, 6, 8).BerlinMidnight()
	end := func(t time.Time) *time.Time { return &t }
	testpkg.CreateTestVisit(t, tc.db, anna.ID, session.ID, monday.Add(14*time.Hour), end(monday.Add(15*time.Hour)))
	testpkg.CreateTestVisit(t, tc.db, bert.ID, session.ID, monday.Add(14*time.Hour+30*time.Minute), end(monday.Add(15*time.Hour+30*time.Minute)))
	testpkg.CreateTestVisit(t, tc.db, anna.ID, session.ID, monday.Add(-2*time.Hour), end(monday.Add(30*time.Minute)))
	thursday := monday.AddDate(0, 0, 3)
	testpkg.CreateTestVisit(t, tc.db, anna.ID, session.ID, thursday.Add(23*time.Hour), end(thursday.Add(25*time.Hour)))
	expiredVisit := testpkg.CreateTestVisit(t, tc.db, anna.ID, session.ID, monday.Add(15*time.Hour), end(monday.Add(16*time.Hour)))
	_, err := tc.db.NewUpdate().TableExpr(`active.visits`).Set(`created_at = ?`, time.Now().AddDate(0, 0, -31)).Where(`id = ?`, expiredVisit.ID).Exec(ctx)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/report?from="+weekFrom.String()+"&to="+weekTo.String(), nil)
	rec := authExec(t, tc, req, claims, reportPermissions)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var payload reportPayload
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	data := payload.Data

	assert.Equal(t, 3, data.CareDays)
	assert.Equal(t, 2, data.ExcludedDays.Total)
	assert.Equal(t, 1, data.ExcludedDays.ClosingDays)
	assert.Equal(t, 1, data.ExcludedDays.HolidayPeriods)
	assert.Equal(t, 0, data.ExcludedDays.PublicHolidays)

	require.Len(t, data.Students, 2)
	byName := map[string]int{}
	for i, st := range data.Students {
		byName[st.LastName] = i
	}
	annaRow := data.Students[byName["Anwesend"]]
	assert.Equal(t, 2, annaRow.PresentDays)
	assert.Equal(t, 0, annaRow.SickDays)
	assert.Equal(t, 0, annaRow.ExcusedDays)
	assert.Equal(t, 1, annaRow.UnexplainedDays)
	require.NotNil(t, annaRow.AttendanceRate)
	assert.InDelta(t, 66.7, *annaRow.AttendanceRate, 0.01)
	assert.Equal(t, group.Name, annaRow.GroupName)

	bertRow := data.Students[byName["Bettruhe"]]
	assert.Equal(t, 1, bertRow.PresentDays)
	assert.Equal(t, 1, bertRow.SickDays)
	assert.Equal(t, 1, bertRow.ExcusedDays)
	assert.Equal(t, 0, bertRow.UnexplainedDays)
	require.NotNil(t, bertRow.AttendanceRate)
	assert.InDelta(t, 33.3, *bertRow.AttendanceRate, 0.01)

	require.Len(t, data.Groups, 1)
	assert.Equal(t, group.Name, data.Groups[0].Name)
	assert.Equal(t, 2, data.Groups[0].StudentCount)
	require.NotNil(t, data.Groups[0].AttendanceRate)
	assert.InDelta(t, 50.0, *data.Groups[0].AttendanceRate, 0.01)
	assert.Equal(t, 2, data.Totals.StudentCount)

	require.NotEmpty(t, data.Rooms)
	var found bool
	for _, r := range data.Rooms {
		if r.RoomID != strconv.FormatInt(room.ID, 10) {
			continue
		}
		found = true
		assert.Equal(t, 3, r.DaysUsed)
		assert.Equal(t, 2, r.DistinctStudents)
		assert.Equal(t, 2, r.PeakOccupancy)
		// 60 + 60 + 30 clamped minutes, plus 120 across midnight. The
		// older 60-minute visit is excluded by the student's retention.
		assert.Equal(t, 270, r.StudentMinutes)
		require.NotNil(t, r.PeakUtilizationPercent, "fixture room has capacity 30")
		assert.InDelta(t, 6.7, *r.PeakUtilizationPercent, 0.01)
	}
	assert.True(t, found, "room row missing")
	assert.Greater(t, data.RoomDataDays, 0)

	// Group filter narrows the child list; an unknown group yields none.
	req = httptest.NewRequest(http.MethodGet, "/report?from="+weekFrom.String()+"&to="+weekTo.String()+"&group_id="+data.Groups[0].GroupID, nil)
	rec = authExec(t, tc, req, claims, reportPermissions)
	require.Equal(t, http.StatusOK, rec.Code)
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload))
	assert.Len(t, payload.Data.Students, 2)
}

func TestStatisticsExport_RendersAndAudits(t *testing.T) {
	t.Parallel()
	tc := setupTestContext(t)
	tenantID := testpkg.Tenant(t)
	ctx := testpkg.Ctx(t)
	_, account := testpkg.CreateTestStaffWithAccountForTenant(t, tc.db, tenantID, "Export", "Tester")
	claims := claimsFor(t, account.ID)
	student := testpkg.CreateTestStudent(t, tc.db, "Export", "Kind", "2b")
	device := testpkg.CreateTestDevice(t, tc.db, "stat-export-device")
	insertAttendance(t, tc.db, tenantID, student.ID, device.ID, timezone.NewDate(2026, 6, 8))

	for _, format := range []string{"pdf", "xlsx", "docx"} {
		req := httptest.NewRequest(http.MethodGet, "/export?from="+weekFrom.String()+"&to="+weekTo.String()+"&format="+format, nil)
		rec := authExec(t, tc, req, claims, reportPermissions)
		require.Equal(t, http.StatusOK, rec.Code, format)
		assert.Contains(t, rec.Header().Get("Content-Disposition"), "statistik-2026-06-08-2026-06-12."+format)
		assert.NotEmpty(t, rec.Body.Bytes())
	}

	req := httptest.NewRequest(http.MethodGet, "/export?from="+weekFrom.String()+"&to="+weekTo.String()+"&format=csv", nil)
	rec := authExec(t, tc, req, claims, reportPermissions)
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	req = httptest.NewRequest(http.MethodGet, "/export?from="+weekFrom.String()+"&to="+weekTo.String()+"&format=xlsx&section=rooms", nil)
	rec = authExec(t, tc, req, claims, reportPermissions)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Disposition"), "raumauslastung-2026-06-08-2026-06-12.xlsx")

	req = httptest.NewRequest(http.MethodGet, "/export?from="+weekFrom.String()+"&to="+weekTo.String()+"&section=nope", nil)
	rec = authExec(t, tc, req, claims, reportPermissions)
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	// One audit row per export; the JSON view is deduplicated per window.
	for range 2 {
		req := httptest.NewRequest(http.MethodGet, "/report?from="+weekFrom.String()+"&to="+weekTo.String(), nil)
		rec := authExec(t, tc, req, claims, reportPermissions)
		require.Equal(t, http.StatusOK, rec.Code)
	}
	var exportRows, viewRows int
	require.NoError(t, tc.db.NewSelect().TableExpr("audit.data_access_log").
		ColumnExpr("count(*)").
		Where("tenant_id = ? AND actor_account_id = ? AND resource_type = ? AND metadata->>'action' = 'export'", tenantID, account.ID, "attendance_statistics").
		Scan(ctx, &exportRows))
	require.NoError(t, tc.db.NewSelect().TableExpr("audit.data_access_log").
		ColumnExpr("count(*)").
		Where("tenant_id = ? AND actor_account_id = ? AND resource_type = ? AND metadata->>'action' = 'view'", tenantID, account.ID, "attendance_statistics").
		Scan(ctx, &viewRows))
	assert.Equal(t, 4, exportRows, "every successful export writes its own audit row")
	assert.Equal(t, 1, viewRows, "repeated views of the same window collapse into one row")
}
