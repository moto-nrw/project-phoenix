package presenceprojectionhttp_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStudentResponse_FullAccess(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	t.Run("staff_outside_the_group_sees_full_access_fields", func(t *testing.T) {
		// #2329: read access is tenant-wide for staff, so the address block —
		// gated on full access — reaches a staff member who supervises nothing.
		student := testpkg.CreateTestStudent(t, tc.db, "Limited", "Access", "LA1")
		_, account := testpkg.CreateTestStaffWithAccount(t, tc.db, "Limited", "Staff")

		ctx := testpkg.Ctx(t)
		_, err := tc.db.ExecContext(ctx,
			"UPDATE users.student_profiles SET address_street = ? WHERE id = ?",
			"Vollzugriffweg 1", student.ID)
		require.NoError(t, err)

		req := testutil.NewRequest("GET", fmt.Sprintf("/%d", student.ID), nil)

		claims := testutil.TeacherTestClaims(int(account.ID))
		rr := authExec(t, tc, req, claims, []string{"users:read"})

		require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())

		var resp struct {
			Data struct {
				HasFullAccess bool `json:"has_full_access"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
		assert.True(t, resp.Data.HasFullAccess, "any verified staff member has full read access")
		assert.Contains(t, rr.Body.String(), "Vollzugriffweg 1")
	})

	t.Run("account_without_staff_record_stays_redacted", func(t *testing.T) {
		// Guest/guardian accounts hold users:read but no staff record, so the
		// full-access fields (address, RFID tag, photo URL) stay out.
		student := testpkg.CreateTestStudent(t, tc.db, "Redacted", "Access", "RA1")
		guest := testpkg.CreateTestAccount(t, tc.db, "students-redacted-guest@example.com")

		ctx := testpkg.Ctx(t)
		_, err := tc.db.ExecContext(ctx,
			"UPDATE users.student_profiles SET address_street = ? WHERE id = ?",
			"Geheimstrasse 9", student.ID)
		require.NoError(t, err)

		req := testutil.NewRequest("GET", fmt.Sprintf("/%d", student.ID), nil)

		claims := testutil.TeacherTestClaims(int(guest.ID))
		rr := authExec(t, tc, req, claims, []string{"users:read"})

		require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())

		var resp struct {
			Data struct {
				HasFullAccess  bool `json:"has_full_access"`
				HasWriteAccess bool `json:"has_write_access"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
		assert.False(t, resp.Data.HasFullAccess)
		assert.False(t, resp.Data.HasWriteAccess)
		assert.NotContains(t, rr.Body.String(), "Geheimstrasse 9",
			"the address block must stay out of a redacted response")
	})
}

func TestGuestStudentResponsesHideSensitiveData(t *testing.T) {
	t.Parallel()
	tc := setupStudentsRoute(t, fixedCalendarClock)
	student := testpkg.CreateTestStudent(t, tc.db, "Mila", "Gastansicht", "GS1")
	testpkg.FillWideStudentFields(t, tc.db, student.ID, student.PersonID)
	staff, account := testpkg.CreateTestStaffWithAccount(t, tc.db, "Gast", "Konto")
	teacher, _ := testpkg.CreateTestTeacherWithAccount(t, tc.db, "OGS", "Kontakt")
	group := testpkg.CreateTestEducationGroup(t, tc.db, "Gastgruppe")
	testpkg.CreateTestGroupTeacher(t, tc.db, group.ID, teacher.ID)
	testpkg.AssignStudentToGroup(t, tc.db, student.ID, group.ID)
	testpkg.CreateTestPickupScheduleWithNote(t, tc.db, student.ID, 1, staff.ID, "15:30", "Abholplan vertraulich")
	testpkg.CreateTestArrivalException(t, tc.db, student.ID, studentsTestToday, staff.ID, "13:00", "Ankunftsplan vertraulich")
	_, err := tc.db.NewRaw(`UPDATE users.student_care_profiles SET departure_companion_note = ?, pickup_status = ?
		WHERE membership_id = (SELECT id FROM users.student_school_memberships WHERE student_profile_id = ? AND deleted_at IS NULL)`,
		"Abholperson vertraulich", "Rücksprache vertraulich", student.ID).Exec(testpkg.Ctx(t))
	require.NoError(t, err)

	for _, granted := range [][]string{{"students:read"}, {"users:read"}, {"admin:*"}} {
		for _, path := range []string{
			fmt.Sprintf("/%d", student.ID),
			"/?include_pickup_times=true&include_arrival_times=true",
			"/?view=slim&include_pickup_times=true&include_arrival_times=true",
		} {
			rr := authExec(t, tc, testutil.NewRequest("GET", path, nil), testutil.TeacherTestClaims(int(account.ID)), granted)
			require.Equal(t, http.StatusOK, rr.Code, "GET %s: %s", path, rr.Body.String())
			var envelope struct {
				Data json.RawMessage `json:"data"`
			}
			require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &envelope))
			var row map[string]any
			if strings.HasPrefix(path, "/?") {
				var rows []map[string]any
				require.NoError(t, json.Unmarshal(envelope.Data, &rows))
				require.Len(t, rows, 1)
				row = rows[0]
			} else {
				require.NoError(t, json.Unmarshal(envelope.Data, &row))
			}
			require.Equal(t, "Mila", row["first_name"])
			full := granted[0] != "students:read"
			require.Equal(t, full, row["has_full_access"])
			if !full {
				for _, field := range []string{"extra_info", "health_info", "departure_companion_note", "address_street", "address_city", "address_postal_code", "pickup_notes", "arrival_notes", "group_supervisors"} {
					require.NotContains(t, row, field, "GET %s", path)
				}
				require.NotContains(t, rr.Body.String(), "vertraulich")
			} else if path != "/?view=slim&include_pickup_times=true&include_arrival_times=true" {
				for _, field := range []string{"extra_info", "health_info", "departure_companion_note", "address_street"} {
					require.NotEmpty(t, row[field], "GET %s", path)
				}
				if strings.HasPrefix(path, "/?") {
					require.Contains(t, row["pickup_notes"], "Abholplan vertraulich")
					require.Contains(t, row["arrival_notes"], "Ankunftsplan vertraulich")
				}
			}
		}
	}
}

func TestGetStudentDetail_WithGroup(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	// Create teacher, group, and student
	teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Detail", "Teacher")
	group := testpkg.CreateTestEducationGroup(t, tc.db, "DetailGroup")
	student := testpkg.CreateTestStudent(t, tc.db, "Detail", "Student", "DT1")

	// Assign teacher and student to group
	testpkg.CreateTestGroupTeacher(t, tc.db, group.ID, teacher.ID)
	testpkg.AssignStudentToGroup(t, tc.db, student.ID, group.ID)

	t.Run("supervisor_gets_full_access", func(t *testing.T) {
		req := testutil.NewRequest("GET", fmt.Sprintf("/%d", student.ID), nil)

		claims := testutil.TeacherTestClaims(int(account.ID))
		rr := authExec(t, tc, req, claims, []string{"users:read"})

		assert.Equal(t, http.StatusOK, rr.Code, "Supervisor should get student")
		// Check response includes group name
		assert.Contains(t, rr.Body.String(), "DetailGroup")
	})
}

func TestListStudents_WithTeacherAccess(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	_, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "ListTeacher", "Access")

	req := testutil.NewRequest("GET", "/", nil)

	claims := testutil.TeacherTestClaims(int(account.ID))
	rr := authExec(t, tc, req, claims, []string{"users:read"})

	assert.Equal(t, http.StatusOK, rr.Code, "Teacher should list students")
}

func TestGetStudent_WithTeacherAccess(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	student := testpkg.CreateTestStudent(t, tc.db, "TeacherAccess", "Test", "TAT1")
	_, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "GetTeacher", "Access")

	req := testutil.NewRequest("GET", fmt.Sprintf("/%d", student.ID), nil)

	claims := testutil.TeacherTestClaims(int(account.ID))
	rr := authExec(t, tc, req, claims, []string{"users:read"})

	assert.Equal(t, http.StatusOK, rr.Code, "Teacher should get student")
}
