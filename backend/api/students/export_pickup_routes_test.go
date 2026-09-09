package students_test

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// Read the delivered workbook, not the export handler's filtering helpers.
func pickupExportRows(t *testing.T, tc *testContext, claims jwt.AppClaims, filters map[string]string) [][]string {
	t.Helper()
	req := testutil.NewAuthenticatedRequest(t, http.MethodPost, "/export", map[string]any{
		"format": "xlsx", "preset": "pickup_list",
		"columns": []string{"name", "planned_pickup"}, "filters": filters,
	})
	rr := authExec(t, tc, req, claims, []string{"users:read"})
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	book, err := zip.NewReader(bytes.NewReader(rr.Body.Bytes()), int64(rr.Body.Len()))
	require.NoError(t, err)
	decodePart := func(name string, target any) {
		part, openErr := book.Open(name)
		require.NoError(t, openErr)
		defer func() { require.NoError(t, part.Close()) }()
		require.NoError(t, xml.NewDecoder(part).Decode(target))
	}
	var shared struct {
		Values []string `xml:"si>t"`
	}
	decodePart("xl/sharedStrings.xml", &shared)
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
	decodePart(sheets[0], &sheet)
	var rows [][]string
	for _, row := range sheet.Rows {
		var values []string
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
	for i, row := range rows {
		if len(row) > 0 && row[0] == "Name" {
			return rows[i+1:]
		}
	}
	t.Fatal("export workbook has no Name header")
	return nil
}

func TestPickupExportCombinesTimes(t *testing.T) {
	t.Parallel()
	tc := setupStudentsRoute(t, fixedCalendarClock)
	staff, account := testpkg.CreateTestStaffWithAccount(t, tc.db, "Export", "Teacher")
	group := testpkg.CreateTestEducationGroup(t, tc.db, "Pickup cohort")
	for _, child := range []struct{ name, pickup string }{
		{"Early", "13:00"}, {"Middle", "14:30"}, {"Late", "16:00"},
	} {
		student := testpkg.CreateTestStudent(t, tc.db, child.name, "Child", "1a")
		testpkg.CreateTestArrivalSchedule(t, tc.db, student.ID, 1, staff.ID, "08:00")
		testpkg.CreateTestPickupSchedule(t, tc.db, student.ID, 1, staff.ID, child.pickup)
		if child.name == "Middle" {
			testpkg.AssignStudentToGroup(t, tc.db, student.ID, group.ID)
		}
	}
	otherTenant, _ := testpkg.CreateTestTenant(t, tc.db)
	testpkg.CreateTestStudentForTenant(t, tc.db, otherTenant, "Foreign", "Child", "1a")
	claims := testutil.AdminTestClaims(int(account.ID))
	for _, scenario := range []struct {
		name    string
		filters map[string]string
		want    [][]string
	}{
		{"OR", map[string]string{"pickup_time": "14:30,16:00"}, [][]string{{"Middle Child", "14:30"}, {"Late Child", "16:00"}}},
		{"duplicates", map[string]string{"pickup_time": "14:30,16:00,14:30"}, [][]string{{"Middle Child", "14:30"}, {"Late Child", "16:00"}}},
		{"legacy single", map[string]string{"pickup_time": "14:30"}, [][]string{{"Middle Child", "14:30"}}},
		{"no restriction", map[string]string{"pickup_time": ""}, [][]string{{"Early Child", "13:00"}, {"Middle Child", "14:30"}, {"Late Child", "16:00"}}},
		{"legacy all", map[string]string{"pickup_time": "all"}, [][]string{{"Early Child", "13:00"}, {"Middle Child", "14:30"}, {"Late Child", "16:00"}}},
		{"group and class", map[string]string{"pickup_time": "14:30,16:00", "group_id": strconv.FormatInt(group.ID, 10), "school_class": "1a"}, [][]string{{"Middle Child", "14:30"}}},
		{"different class", map[string]string{"pickup_time": "14:30,16:00", "school_class": "2b"}, nil},
		{"arrival", map[string]string{"pickup_time": "14:30,16:00", "arrival_time": "09:00"}, nil},
		{"no schoolyard visits", map[string]string{"pickup_time": "14:30,16:00", "status": "schulhof"}, nil},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			require.ElementsMatch(t, scenario.want, pickupExportRows(t, tc, claims, scenario.filters))
		})
	}
}

func TestPickupExportMissingTimesAndDayExceptions(t *testing.T) {
	t.Parallel()
	tc := setupStudentsRoute(t, fixedCalendarClock)
	staff, account := testpkg.CreateTestStaffWithAccount(t, tc.db, "Pickup", "Planner")
	missing := testpkg.CreateTestStudent(t, tc.db, "Missing", "Child", "1a")
	absent := testpkg.CreateTestStudent(t, tc.db, "Absent", "Child", "1a")
	changed := testpkg.CreateTestStudent(t, tc.db, "Changed", "Child", "1a")
	for _, id := range []int64{missing.ID, absent.ID, changed.ID} {
		for _, weekday := range []int{1, 2} {
			testpkg.CreateTestArrivalSchedule(t, tc.db, id, weekday, staff.ID, "08:00")
		}
	}
	testpkg.CreateTestPickupSchedule(t, tc.db, changed.ID, 1, staff.ID, "13:00")
	testpkg.CreateTestPickupSchedule(t, tc.db, changed.ID, 2, staff.ID, "16:00")
	testpkg.CreateTestPickupException(t, tc.db, absent.ID, studentsTestToday, staff.ID, "", "Absent")
	testpkg.CreateTestPickupException(t, tc.db, changed.ID, studentsTestToday, staff.ID, "14:30", "Earlier today")
	claims := testutil.AdminTestClaims(int(account.ID))
	for _, scenario := range []struct {
		name    string
		filters map[string]string
		want    [][]string
	}{
		{"none excludes absence exception", map[string]string{"pickup_time": "none"}, [][]string{{"Missing Child"}}},
		{"none OR effective exception time", map[string]string{"pickup_time": "none,14:30"}, [][]string{{"Missing Child"}, {"Changed Child", "14:30"}}},
		{"regular time is overridden", map[string]string{"pickup_time": "13:00,16:00"}, nil},
		{"selected day uses its own plan", map[string]string{"pickup_time": "14:30,16:00", "date": timezone.NewDate(2026, 8, 25).String()}, [][]string{{"Changed Child", "16:00"}}},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			require.ElementsMatch(t, scenario.want, pickupExportRows(t, tc, claims, scenario.filters))
		})
	}
	// An account without staff membership may read the redacted list, but
	// unknown pickup times must not be interpreted as "none" during export.
	guest := testpkg.CreateTestAccount(t, tc.db, "pickup-guest@example.test")
	guestClaims := testutil.TeacherTestClaims(int(guest.ID))
	require.Len(t, pickupExportRows(t, tc, guestClaims, map[string]string{}), 3)
	require.Empty(t, pickupExportRows(t, tc, guestClaims, map[string]string{"pickup_time": "none,14:30"}))
}
