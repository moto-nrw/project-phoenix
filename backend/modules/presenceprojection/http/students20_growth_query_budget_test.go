package presenceprojectionhttp_test

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/careplan/absencerecords"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/require"
)

// Resource routes use real owner DB reads and retain their mandatory audit
// writes. This is not the production live-session authentication frame.
func TestStudents20PresenceGrowthQueryBudget(t *testing.T) {
	t.Parallel()
	for _, route := range []string{"day-log", "day-log/export", "status-days"} {
		t.Run(route, func(t *testing.T) {
			testpkg.OwnTenant(t)
			testpkg.SetupIsolatedTestDB(t)
			tc := setupStudentsRoute(t, fixedCalendarClock)
			enableAttendanceLog(t, tc)
			actor := testpkg.CreateTestAccount(t, tc.db, "s20-presence@example.com")
			group := testpkg.CreateTestEducationGroup(t, tc.db, "S20 Group")
			otherGroup := testpkg.CreateTestEducationGroup(t, tc.db, "Other Group")
			excluded := testpkg.CreateTestStudent(t, tc.db, "Other", "Excluded", "S20")
			testpkg.AssignStudentToGroup(t, tc.db, excluded.ID, otherGroup.ID)
			testpkg.CreateTestStudentStatusDay(t, tc.db, excluded.ID, studentsTestToday, absencerecords.StudentStatusDaySick)
			ids := make([]string, 0, 8)
			dayIDs := make([]string, 0, 8)
			reported := make([]string, 0, 8)
			reportedInstants := make([]string, 0, 8)
			counter := testpkg.CaptureQueriesForContext(t, tc.db)
			for _, n := range []int{3, 8} {
				for len(ids) < n {
					name := fmt.Sprintf("Child%02d", len(ids))
					child := testpkg.CreateTestStudent(t, tc.db, name, "Presence", "S20")
					testpkg.AssignStudentToGroup(t, tc.db, child.ID, group.ID)
					day := testpkg.CreateTestStudentStatusDay(t, tc.db, child.ID, studentsTestToday, absencerecords.StudentStatusDaySick)
					ids = append(ids, strconv.FormatInt(child.ID, 10))
					dayIDs = append(dayIDs, strconv.FormatInt(day.ID, 10))
					require.NoError(t, tc.db.NewSelect().Model(day).WherePK().Scan(testpkg.Ctx(t)))
					reported = append(reported, day.ReportedAt.In(calendar.Berlin).Format("02.01. 15:04"))
					reportedInstants = append(reportedInstants, day.ReportedAt.Format(time.RFC3339Nano))
				}
				counter.Reset()
				router := testutil.NewRouter()
				router.Mount("/api/students", tc.router())
				path := fmt.Sprintf("/api/students/%s?group_id=%d&date=%s&from=%s&to=%s&format=xlsx", route, group.ID, studentsTestToday, studentsTestToday, studentsTestToday)
				req := testutil.NewRequest(http.MethodGet, path, nil)
				req = req.WithContext(counter.Context(req.Context()))
				rr := testutil.ExecuteWithAuthPermissions(t, router, req, testutil.AdminTestClaims(int(actor.ID)), []string{"admin:*"})
				require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
				queries := counter.Queries()
				if route == "day-log/export" {
					rows := students20WorkbookRows(t, rr.Body.Bytes())
					require.Equal(t, "Tagesauswertung", rows[0][0])
					require.Equal(t, "Anwesenheit am 24.08.2026", rows[1][0])
					var data [][]string
					for i, row := range rows {
						if len(row) > 0 && row[0] == "Name" {
							data = rows[i+1:]
							break
						}
					}
					require.Len(t, data, n)
					for i := range n {
						require.Equal(t, []string{fmt.Sprintf("Presence, Child%02d", i), "S20", "Krank", "–", "–", "gemeldet " + reported[i]}, appendRowStudents20(data[i], 6))
					}
				} else if route == "day-log" {
					var body dayLogTestBody
					require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
					require.Equal(t, studentsTestToday.String(), body.Data.Date)
					require.Len(t, body.Data.Groups, 1)
					got := body.Data.Groups[0]
					require.Equal(t, strconv.FormatInt(group.ID, 10), got.GroupID)
					require.Equal(t, group.Name, got.Name)
					require.Len(t, got.Students, n)
					for i, row := range got.Students {
						require.Equal(t, ids[i], row.StudentID)
						require.Equal(t, fmt.Sprintf("Child%02d", i), row.FirstName)
						require.Equal(t, "Presence", row.LastName)
						require.Equal(t, "sick", row.Status)
						require.Equal(t, "Krank", row.Label)
						require.Equal(t, "manual", row.Source)
						require.Nil(t, row.CheckInTime)
						require.Nil(t, row.CheckOutTime)
					}
					require.Equal(t, n, got.Counters.Sick)
					require.Equal(t, n, got.Counters.Total)
					require.Zero(t, got.Counters.Present)
					require.Zero(t, got.Counters.Excused)
					require.Zero(t, got.Counters.Absent)
					require.Equal(t, got.Counters, body.Data.Counters)
					var complete struct {
						Data map[string]any `json:"data"`
					}
					require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &complete))
					expectedRows := make([]any, 0, n)
					for i, id := range ids {
						expectedRows = append(expectedRows, map[string]any{"student_id": id, "first_name": fmt.Sprintf("Child%02d", i), "last_name": "Presence", "school_class": "S20", "status": "sick", "label": "Krank", "reported_at": reportedInstants[i], "source": "manual"})
					}
					counts := map[string]any{"present": float64(0), "sick": float64(n), "class_trip": float64(0), "excused": float64(0), "absent": float64(0), "not_scheduled": float64(0), "total": float64(n)}
					require.Equal(t, map[string]any{"date": studentsTestToday.String(), "groups": []any{map[string]any{"group_id": strconv.FormatInt(group.ID, 10), "name": group.Name, "students": expectedRows, "counters": counts}}, "counters": counts}, complete.Data)
				} else {
					var body statusDayOverviewTestBody
					require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
					require.Equal(t, studentsTestToday.String(), body.Data.From)
					require.Equal(t, studentsTestToday.String(), body.Data.To)
					require.Len(t, body.Data.Entries, n)
					require.False(t, body.Data.HasMore)
					require.Equal(t, 50, body.Data.PageSize)
					require.Equal(t, 1, body.Data.Page)
					var complete struct {
						Data map[string]any `json:"data"`
					}
					require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &complete))
					expectedRows := make([]any, 0, n)
					for i, id := range ids {
						expectedRows = append(expectedRows, map[string]any{"id": dayIDs[i], "student_id": id, "first_name": fmt.Sprintf("Child%02d", i), "last_name": "Presence", "school_class": "S20", "group_id": strconv.FormatInt(group.ID, 10), "group_name": group.Name, "date": studentsTestToday.String(), "status": "sick", "label": "Krank", "reported_at": reportedInstants[i], "source": "manual"})
					}
					require.Equal(t, map[string]any{"from": studentsTestToday.String(), "to": studentsTestToday.String(), "groups": []any{map[string]any{"id": strconv.FormatInt(group.ID, 10), "name": group.Name}, map[string]any{"id": strconv.FormatInt(otherGroup.ID, 10), "name": otherGroup.Name}}, "entries": expectedRows, "page": float64(1), "page_size": float64(50), "has_more": false}, complete.Data)
					for i, row := range body.Data.Entries {
						require.Equal(t, ids[i], row.StudentID)
						require.Equal(t, dayIDs[i], row.ID)
						require.Equal(t, fmt.Sprintf("Child%02d", i), row.FirstName)
						require.Equal(t, "Presence", row.LastName)
						require.Equal(t, "S20", row.SchoolClass)
						require.Equal(t, strconv.FormatInt(group.ID, 10), row.GroupID)
						require.Equal(t, group.Name, row.GroupName)
						require.Equal(t, studentsTestToday.String(), row.Date)
						require.Equal(t, "sick", row.Status)
						require.Equal(t, "Krank", row.Label)
						require.Equal(t, "manual", row.Source)
					}
				}
				require.NotContains(t, rr.Body.String(), "Excluded")
				require.EqualValues(t, 1, counter.WriteRows(), "one mandatory audit row per read")
				var auditCount int
				resource := "attendance_day_log"
				if route == "status-days" {
					resource = "student_status_day_overview"
				}
				require.NoError(t, tc.db.NewSelect().TableExpr("audit.data_access_log").ColumnExpr("count(*)").Where("actor_account_id = ? AND resource_type = ? AND tenant_id = ?", actor.ID, resource, testpkg.Tenant(t)).Scan(testpkg.Ctx(t), &auditCount))
				expectedAudits := 1
				if n == 8 {
					expectedAudits = 2
				}
				require.Equal(t, expectedAudits, auditCount)
				assertStudents20PresenceBudget(t, route, n, counter.Measurement(counter.Queries()))
				t.Logf("students20 %s N%d SQL=%d", route, n, len(queries))
			}
		})
	}
}

func appendRowStudents20(row []string, n int) []string {
	for len(row) < n {
		row = append(row, "")
	}
	return row
}

func TestStudents20AttendanceHistoryGrowthQueryBudget(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	tc := setupStudentsRoute(t, fixedCalendarClock)
	enableAttendanceLog(t, tc)
	actor := testpkg.CreateTestAccount(t, tc.db, "s20-history@example.com")
	child := testpkg.CreateTestStudent(t, tc.db, "History", "Subject", "S20")
	other := testpkg.CreateTestStudent(t, tc.db, "History", "Excluded", "S20")
	testpkg.CreateTestStudentStatusDay(t, tc.db, other.ID, studentsTestToday, absencerecords.StudentStatusDayExcused)
	expected := make([]any, 0, 8)
	counter := testpkg.CaptureQueriesForContext(t, tc.db)
	for _, n := range []int{3, 8} {
		for len(expected) < n {
			i := len(expected)
			date := studentsTestToday.AddDays(-i)
			row := testpkg.CreateTestStudentStatusDay(t, tc.db, child.ID, date, absencerecords.StudentStatusDaySick)
			require.NoError(t, tc.db.NewSelect().Model(row).WherePK().Scan(testpkg.Ctx(t)))
			expected = append(expected, map[string]any{"date": date.String(), "attendance": nil, "status_entries": []any{map[string]any{"status": "sick", "label": "Krank", "reported_at": row.ReportedAt.Format(time.RFC3339Nano)}}, "room_detail_available": i < 7, "visits": []any{}, "slots": []any{}})
		}
		counter.Reset()
		router := testutil.NewRouter()
		router.Mount("/api/students", tc.router())
		req := testutil.NewRequest(http.MethodGet, fmt.Sprintf("/api/students/%d/attendance-history?start=%s&end=%s", child.ID, studentsTestToday.AddDays(-7).BerlinMidnight().UTC().Format(time.RFC3339), studentsTestToday.EndOfDay().UTC().Format(time.RFC3339)), nil)
		req = req.WithContext(counter.Context(req.Context()))
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, testutil.AdminTestClaims(int(actor.ID)), []string{"admin:*"})
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		var body struct {
			Data map[string]any `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
		require.Equal(t, map[string]any{"student_id": strconv.FormatInt(child.ID, 10), "days": expected, "range": map[string]any{"start": studentsTestToday.AddDays(-7).BerlinMidnight().UTC().Format(time.RFC3339Nano), "end": studentsTestToday.EndOfDay().UTC().Format(time.RFC3339Nano)}, "clamped": false, "caps": map[string]any{"attendance_days": float64(30), "room_detail_days": float64(7)}}, body.Data)
		require.EqualValues(t, 1, counter.WriteRows())
		assertStudents20PresenceBudget(t, "attendance_history", n, counter.Measurement(counter.Queries()))
		t.Logf("students20 attendance_history N%d SQL=%d", n, counter.Total())
	}
}

func TestStudents20AttendanceExportGrowthQueryBudget(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	tc := setupStudentsRoute(t, fixedCalendarClock)
	enableAttendanceLog(t, tc)
	actor := testpkg.CreateTestAccount(t, tc.db, "s20-history-export@example.com")
	child := testpkg.CreateTestStudent(t, tc.db, "History", "Subject", "S20")
	staff := testpkg.CreateTestStaff(t, tc.db, "History", "Recorder")
	device := testpkg.CreateTestDevice(t, tc.db, "s20-history-export")
	expected := make([][]string, 0, 8)
	counter := testpkg.CaptureQueriesForContext(t, tc.db)
	for _, n := range []int{3, 8} {
		for len(expected) < n {
			date := studentsTestToday.AddDays(-len(expected))
			in := date.BerlinMidnight().Add(8 * time.Hour)
			out := date.BerlinMidnight().Add(15 * time.Hour)
			testpkg.CreateTestAttendanceForDate(t, tc.db, child.ID, staff.ID, device.ID, date, in, &out)
			expected = append([][]string{{date.Format("02.01.2006"), "08:00–15:00", "Anwesend", "08:00", "15:00", "History Recorder", "automatisch"}}, expected...)
		}
		counter.Reset()
		router := testutil.NewRouter()
		router.Mount("/api/students", tc.router())
		req := testutil.NewRequest(http.MethodGet, fmt.Sprintf("/api/students/%d/attendance-history/export?from=%s&to=%s&format=xlsx", child.ID, studentsTestToday.AddDays(-7), studentsTestToday), nil)
		req = req.WithContext(counter.Context(req.Context()))
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, testutil.AdminTestClaims(int(actor.ID)), []string{"admin:*"})
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		rows := students20WorkbookRows(t, rr.Body.Bytes())
		require.Equal(t, []string{"Anwesenheit"}, rows[0])
		require.Equal(t, []string{fmt.Sprintf("Kind-ID %d · 17.08.2026 bis 24.08.2026", child.ID)}, rows[1])
		var data [][]string
		for i, row := range rows {
			if len(row) > 0 && row[0] == "Datum" {
				data = rows[i+1:]
				break
			}
		}
		require.Equal(t, expected, data)
		require.EqualValues(t, 1, counter.WriteRows())
		assertStudents20PresenceBudget(t, "attendance_export", n, counter.Measurement(counter.Queries()))
		t.Logf("students20 attendance_export N%d SQL=%d", n, counter.Total())
	}
}

func assertStudents20PresenceBudget(t *testing.T, route string, n int, queries querybudgets.Measurement) {
	t.Helper()
	switch fmt.Sprintf("%s.n%d", route, n) {
	case "day-log.n3":
		querybudgets.AssertQueryBudget(t, "api.students.students20.day_log.n3", queries)
	case "day-log.n8":
		querybudgets.AssertQueryBudget(t, "api.students.students20.day_log.n8", queries)
	case "day-log/export.n3":
		querybudgets.AssertQueryBudget(t, "api.students.students20.day_log_export.n3", queries)
	case "day-log/export.n8":
		querybudgets.AssertQueryBudget(t, "api.students.students20.day_log_export.n8", queries)
	case "status-days.n3":
		querybudgets.AssertQueryBudget(t, "api.students.students20.status_days.n3", queries)
	case "status-days.n8":
		querybudgets.AssertQueryBudget(t, "api.students.students20.status_days.n8", queries)
	case "attendance_history.n3":
		querybudgets.AssertQueryBudget(t, "api.students.students20.attendance_history.n3", queries)
	case "attendance_history.n8":
		querybudgets.AssertQueryBudget(t, "api.students.students20.attendance_history.n8", queries)
	case "attendance_export.n3":
		querybudgets.AssertQueryBudget(t, "api.students.students20.attendance_export.n3", queries)
	case "attendance_export.n8":
		querybudgets.AssertQueryBudget(t, "api.students.students20.attendance_export.n8", queries)
	default:
		t.Fatalf("unregistered students20 route %s N%d", route, n)
	}
}

func TestStudents20ForeignAttendanceIsolation(t *testing.T) {
	t.Parallel()
	tc := setupStudentsRoute(t, fixedCalendarClock)
	enableAttendanceLog(t, tc)
	foreignTenant, _ := testpkg.CreateTestTenant(t, tc.db)
	foreign := testpkg.CreateTestStudentForTenant(t, tc.db, foreignTenant, "Foreign", "Private", "S20")
	for _, suffix := range []string{"attendance-history", "attendance-history/export?format=xlsx"} {
		rr := authExec(t, tc, testutil.NewRequest(http.MethodGet, fmt.Sprintf("/%d/%s", foreign.ID, suffix), nil), testutil.AdminTestClaims(1), []string{"admin:*"})
		require.Equal(t, http.StatusNotFound, rr.Code, rr.Body.String())
		require.NotContains(t, rr.Body.String(), "Private")
	}
}

// Decode the delivered XLSX with the standard library, like the existing
// pickup-export fixture. The HTTP adapter test does not import rendering code.
func students20WorkbookRows(t *testing.T, data []byte) [][]string {
	t.Helper()
	book, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	require.NoError(t, err)
	decode := func(name string, target any) {
		part, openErr := book.Open(name)
		require.NoError(t, openErr)
		defer func() { require.NoError(t, part.Close()) }()
		require.NoError(t, xml.NewDecoder(part).Decode(target))
	}
	var shared struct {
		Values []string `xml:"si>t"`
	}
	decode("xl/sharedStrings.xml", &shared)
	var sheet struct {
		Rows []struct {
			Cells []struct {
				Type  string `xml:"t,attr"`
				Value string `xml:"v"`
			} `xml:"c"`
		} `xml:"sheetData>row"`
	}
	var sheets []string
	for _, part := range book.File {
		if strings.HasPrefix(part.Name, "xl/worksheets/sheet") && strings.HasSuffix(part.Name, ".xml") {
			sheets = append(sheets, part.Name)
		}
	}
	require.Len(t, sheets, 1)
	decode(sheets[0], &sheet)
	rows := make([][]string, 0, len(sheet.Rows))
	for _, row := range sheet.Rows {
		values := make([]string, 0, len(row.Cells))
		for _, cell := range row.Cells {
			value := cell.Value
			if cell.Type == "s" {
				index, indexErr := strconv.Atoi(value)
				require.NoError(t, indexErr)
				require.GreaterOrEqual(t, index, 0)
				require.Less(t, index, len(shared.Values))
				value = shared.Values[index]
			}
			values = append(values, value)
		}
		for len(values) > 0 && values[len(values)-1] == "" {
			values = values[:len(values)-1]
		}
		rows = append(rows, values)
	}
	return rows
}
