package presenceprojectionhttp_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

type attendanceActorBody struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
}

// recordedAttendance is one child's school stay today and who booked it.
type recordedAttendance struct {
	studentID int64
	wantIn    attendanceActorBody
	wantOut   *attendanceActorBody
}

// recordAttendanceActorCases books one stay per case (#4761): a web booking
// by a current staff member, a kiosk booking without a person, a booking by
// a staff member who left since, one whose personal data the automatic
// deletion of staff erased, and a checkout moto closed itself.
func recordAttendanceActorCases(t *testing.T, tc *testContext) []recordedAttendance {
	t.Helper()
	ctx := testpkg.Ctx(t)
	checkIn := calendar.Today().Add(8 * time.Hour)
	checkOut := calendar.Today().Add(15 * time.Hour)
	today := calendar.TodayDate().String()
	device := testpkg.CreateTestDevice(t, tc.db, fmt.Sprintf("actor-dev-%d", time.Now().UnixNano()))

	web := testpkg.CreateTestStaff(t, tc.db, "Wiebke", "Webbuchung")
	webOut := testpkg.CreateTestStaff(t, tc.db, "Paul", "Abholung")
	webChild := testpkg.CreateTestStudent(t, tc.db, "Ben", "Web", "1a")
	testpkg.CreateTestAttendance(t, tc.db, webChild.ID, web.ID, device.ID, checkIn, nil)
	testpkg.CheckOutTestAttendanceByStaff(t, tc.db, webChild.ID, today, checkOut, webOut.ID)

	deviceChild := testpkg.CreateTestStudent(t, tc.db, "Mia", "Geraet", "1a")
	testpkg.CreateTestAttendance(t, tc.db, deviceChild.ID, 0, device.ID, checkIn, nil)
	_, err := tc.db.ExecContext(ctx, `UPDATE active.attendance SET check_out_time = ?, checked_out_device_id = ?
		WHERE student_id = ? AND date = ?`, checkOut, device.ID, deviceChild.ID, today)
	require.NoError(t, err)

	retired := testpkg.CreateTestStaff(t, tc.db, "Rita", "Ausgeschieden")
	retiredChild := testpkg.CreateTestStudent(t, tc.db, "Lea", "Frueher", "1a")
	testpkg.CreateTestAttendance(t, tc.db, retiredChild.ID, retired.ID, device.ID, checkIn, nil)
	_, err = tc.db.ExecContext(ctx, `UPDATE users.staff_school_memberships SET deleted_at = NOW() WHERE id = ?`, retired.ID)
	require.NoError(t, err)

	erased := testpkg.CreateTestStaff(t, tc.db, "Eva", "Geloescht")
	erasedChild := testpkg.CreateTestStudent(t, tc.db, "Tom", "Ehemals", "1a")
	testpkg.CreateTestAttendance(t, tc.db, erasedChild.ID, erased.ID, device.ID, checkIn, &checkOut)
	// As the automatic deletion of staff leaves it (#3963): an erased
	// tombstone whose person is anonymized and soft-deleted.
	_, err = tc.db.ExecContext(ctx, `UPDATE users.staff_school_memberships
		SET deleted_at = NOW() - INTERVAL '1 day', personal_data_erased_at = NOW() WHERE id = ?`, erased.ID)
	require.NoError(t, err)
	_, err = tc.db.ExecContext(ctx, `UPDATE users.persons
		SET first_name = 'Gelöscht', last_name = 'Benutzer', deleted_at = NOW() WHERE id = ?`, erased.PersonID)
	require.NoError(t, err)

	return []recordedAttendance{
		{
			studentID: webChild.ID,
			wantIn:    attendanceActorBody{Kind: "staff", Name: "Wiebke Webbuchung"},
			wantOut:   &attendanceActorBody{Kind: "staff", Name: "Paul Abholung"},
		},
		{studentID: deviceChild.ID, wantIn: attendanceActorBody{Kind: "device"}, wantOut: &attendanceActorBody{Kind: "device"}},
		{studentID: retiredChild.ID, wantIn: attendanceActorBody{Kind: "staff", Name: "Rita Ausgeschieden"}},
		{studentID: erasedChild.ID, wantIn: attendanceActorBody{Kind: "former_staff"}, wantOut: &attendanceActorBody{Kind: "automatic"}},
	}
}

func TestStudentDetailNamesWhoRecordedTodaysAttendance(t *testing.T) {
	t.Parallel()
	tc := setupStudentsRoute(t)
	for _, tt := range recordAttendanceActorCases(t, tc) {
		req := testutil.NewRequest("GET", fmt.Sprintf("/%d", tt.studentID), nil)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})
		require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())

		var body struct {
			Data struct {
				ActualArrivalBy *attendanceActorBody `json:"actual_arrival_by"`
				ActualPickupBy  *attendanceActorBody `json:"actual_pickup_by"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
		require.NotNil(t, body.Data.ActualArrivalBy, "student %d", tt.studentID)
		assert.Equal(t, tt.wantIn, *body.Data.ActualArrivalBy, "student %d", tt.studentID)
		assert.Equal(t, tt.wantOut, body.Data.ActualPickupBy, "student %d", tt.studentID)
	}
}

func TestAttendanceHistoryNamesWhoRecordedEachDay(t *testing.T) {
	t.Parallel()
	tc := setupStudentsRoute(t)
	enableAttendanceLog(t, tc)
	for _, tt := range recordAttendanceActorCases(t, tc) {
		req := testutil.NewRequest("GET", fmt.Sprintf("/%d/attendance-history", tt.studentID), nil)
		rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})
		require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())

		var body struct {
			Data struct {
				Days []struct {
					Attendance *struct {
						CheckedInActor  *attendanceActorBody `json:"checked_in_actor"`
						CheckedOutActor *attendanceActorBody `json:"checked_out_actor"`
					} `json:"attendance"`
				} `json:"days"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
		require.Len(t, body.Data.Days, 1, "student %d", tt.studentID)
		attendance := body.Data.Days[0].Attendance
		require.NotNil(t, attendance)
		require.NotNil(t, attendance.CheckedInActor, "student %d", tt.studentID)
		assert.Equal(t, tt.wantIn, *attendance.CheckedInActor, "student %d", tt.studentID)
		assert.Equal(t, tt.wantOut, attendance.CheckedOutActor, "student %d", tt.studentID)
	}
}
