// Router-level tests for the notes the OGS released for the school (#4797):
// only the Lehrkraft assigned to the child's class in
// education.class_teachers reads them, the Schulverwaltung does not even
// with class_day:all_classes, a note that is not released stays invisible,
// a withdrawn release takes effect on the next read, and every read writes
// one access-log row.
package classdayhttp_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

type sharedNotesFixture struct {
	*arrivalExceptionFixture
	tenantID  int64
	studentID int64
	otherID   int64
	authorID  int64
}

// setupSharedNotesFixture builds one Lehrkraft with an assigned class and a
// child Ida in it, a child of another class, and an OGS staff member who
// writes the notes.
func setupSharedNotesFixture(t *testing.T) *sharedNotesFixture {
	t.Helper()
	base := setupArrivalExceptionFixture(t)
	ida := testpkg.CreateTestStudent(t, base.db, "Ida", "Beobachtung", base.class)
	other := testpkg.CreateTestStudent(t, base.db, "Olga", "Andereklasse", fmt.Sprintf("y%d", time.Now().UnixNano()%100000))
	_, author := testpkg.CreateTestStaffWithAccount(t, base.db, "Olaf", fmt.Sprintf("OGS-%d", time.Now().UnixNano()))
	return &sharedNotesFixture{
		arrivalExceptionFixture: base,
		tenantID:                testpkg.Tenant(t),
		studentID:               ida.ID, otherID: other.ID, authorID: author.ID,
	}
}

// note writes one OGS note about the child, released for the school or not.
func (f *sharedNotesFixture) note(t *testing.T, studentID int64, body string, shared bool) int64 {
	t.Helper()
	var id int64
	err := f.db.NewRaw(`INSERT INTO users.student_notes
		(tenant_id, student_id, author_account_id, origin, kind, visibility, body, shared_with_school)
		VALUES (?, ?, ?, 'staff', 'permanent', 'all_staff', ?, ?) RETURNING id`,
		f.tenantID, studentID, f.authorID, body, shared).Scan(testpkg.Ctx(t), &id)
	require.NoError(t, err)
	return id
}

func (f *sharedNotesFixture) path(class string, studentID int64) string {
	return fmt.Sprintf("/shared-notes/%s/students/%d", class, studentID)
}

func (f *sharedNotesFixture) read(t *testing.T, claims sessiontoken.AppClaims, path string, perms ...string) (int, []map[string]any) {
	t.Helper()
	f.claims, claims = claims, f.claims
	rec := f.do(t, http.MethodGet, path, "", perms...)
	f.claims = claims
	if rec.Code != http.StatusOK {
		return rec.Code, nil
	}
	var envelope struct {
		Data struct {
			StudentID string           `json:"student_id"`
			Notes     []map[string]any `json:"notes"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope), rec.Body.String())
	require.Equal(t, fmt.Sprint(f.studentID), envelope.Data.StudentID)
	return rec.Code, envelope.Data.Notes
}

func (f *sharedNotesFixture) accessRows(t *testing.T, accountID int64) int {
	t.Helper()
	var count int
	err := f.db.NewRaw(`SELECT COUNT(*) FROM audit.data_access_log
		WHERE tenant_id = ? AND actor_account_id = ? AND resource_type = 'school_shared_notes' AND student_id = ?`,
		f.tenantID, accountID, f.studentID).Scan(testpkg.Ctx(t), &count)
	require.NoError(t, err)
	return count
}

func TestSharedNotesOnlyReleasedNotesReachTheClassTeacher(t *testing.T) {
	t.Parallel()
	f := setupSharedNotesFixture(t)
	f.note(t, f.studentID, "Ida spielt gern mit Bruno.", true)
	f.note(t, f.studentID, "Interne Notiz der OGS.", false)

	code, notes := f.read(t, f.claims, f.path(f.class, f.studentID), classDayReadPermission)
	require.Equal(t, http.StatusOK, code)
	require.Len(t, notes, 1, "a note the OGS did not release stays invisible")
	assert.Equal(t, "Ida spielt gern mit Bruno.", notes[0]["body"])
	assert.NotEmpty(t, notes[0]["author_name"])
	assert.NotContains(t, notes[0], "visibility", "the Lehrkraft reads no OGS audience")

	report := f.dayReport(t, classDayReadPermission)
	assert.Equal(t, true, report["can_read_shared_notes"])
}

func TestSharedNotesRefuseAnotherClass(t *testing.T) {
	t.Parallel()
	f := setupSharedNotesFixture(t)
	f.note(t, f.otherID, "Olga braucht Ruhe.", true)
	f.note(t, f.studentID, "Ida spielt gern mit Bruno.", true)

	// The child of another class is not reachable through the own class.
	code, _ := f.read(t, f.claims, f.path(f.class, f.otherID), classDayReadPermission)
	assert.Equal(t, http.StatusNotFound, code)

	// A Lehrkraft of another class reaches neither class.
	staff, account := testpkg.CreateTestStaffWithAccount(t, f.db, "Anna", fmt.Sprintf("Andere-%d", time.Now().UnixNano()))
	_ = testpkg.CreateTestClassTeacher(t, f.db, staff.ID, fmt.Sprintf("z%d", time.Now().UnixNano()%100000))
	other := f.claims
	other.ID, other.Sub = int(account.ID), account.Email
	code, _ = f.read(t, other, f.path(f.class, f.studentID), classDayReadPermission)
	assert.Equal(t, http.StatusForbidden, code)
	assert.Zero(t, f.accessRows(t, account.ID), "a refused read is no read")
}

func TestSharedNotesRefuseTheSchoolAdministration(t *testing.T) {
	t.Parallel()
	f := setupSharedNotesFixture(t)
	f.note(t, f.studentID, "Ida spielt gern mit Bruno.", true)

	_, account := testpkg.CreateTestStaffWithAccount(t, f.db, "Sabine", fmt.Sprintf("Sekretariat-%d", time.Now().UnixNano()))
	admin := f.claims
	admin.ID, admin.Sub, admin.Roles = int(account.ID), account.Email, []string{"schulverwaltung"}
	code, _ := f.read(t, admin, f.path(f.class, f.studentID), classDayReadPermission, classDayAllClassesPermission)
	assert.Equal(t, http.StatusForbidden, code, "class_day:all_classes does not open the shared notes")
	assert.Zero(t, f.accessRows(t, account.ID))

	// The class view the Schulverwaltung reaches offers no link either.
	f.claims, admin = admin, f.claims
	report := f.dayReport(t, classDayReadPermission, classDayAllClassesPermission)
	f.claims = admin
	assert.Equal(t, false, report["can_read_shared_notes"])
}

func TestSharedNotesWithdrawalTakesEffectImmediately(t *testing.T) {
	t.Parallel()
	f := setupSharedNotesFixture(t)
	noteID := f.note(t, f.studentID, "Ida spielt gern mit Bruno.", true)

	code, notes := f.read(t, f.claims, f.path(f.class, f.studentID), classDayReadPermission)
	require.Equal(t, http.StatusOK, code)
	require.Len(t, notes, 1)

	_, err := f.db.ExecContext(testpkg.Ctx(t),
		`UPDATE users.student_notes SET shared_with_school = FALSE WHERE id = ?`, noteID)
	require.NoError(t, err)

	code, notes = f.read(t, f.claims, f.path(f.class, f.studentID), classDayReadPermission)
	require.Equal(t, http.StatusOK, code)
	assert.Empty(t, notes, "the withdrawn note is gone on the next read")
}

func TestSharedNotesRecordEveryRead(t *testing.T) {
	t.Parallel()
	f := setupSharedNotesFixture(t)
	f.note(t, f.studentID, "Ida spielt gern mit Bruno.", true)

	for range 2 {
		code, _ := f.read(t, f.claims, f.path(f.class, f.studentID), classDayReadPermission)
		require.Equal(t, http.StatusOK, code)
	}
	assert.Equal(t, 2, f.accessRows(t, int64(f.claims.ID)), "every read writes its own row, without deduplication")

	var metadata string
	err := f.db.NewRaw(`SELECT metadata::text FROM audit.data_access_log
		WHERE tenant_id = ? AND actor_account_id = ? AND resource_type = 'school_shared_notes'
		ORDER BY id DESC LIMIT 1`, f.tenantID, f.claims.ID).Scan(testpkg.Ctx(t), &metadata)
	require.NoError(t, err)
	assert.Contains(t, metadata, `"note_count": 1`)
	assert.Contains(t, metadata, f.class)
}

func TestSharedNotesMissingStudentReturnsNotFound(t *testing.T) {
	t.Parallel()
	f := setupSharedNotesFixture(t)
	var missingID int64
	err := f.db.NewRaw(`SELECT nextval(pg_get_serial_sequence('users.student_profiles', 'id'))`).Scan(testpkg.Ctx(t), &missingID)
	require.NoError(t, err)

	code, _ := f.read(t, f.claims, f.path(f.class, missingID), classDayReadPermission)
	assert.Equal(t, http.StatusNotFound, code)
	assert.Zero(t, f.accessRows(t, int64(f.claims.ID)))
}

// dayReport reads today's report of the class as the route serves it.
func (f *sharedNotesFixture) dayReport(t *testing.T, perms ...string) map[string]any {
	t.Helper()
	rec := f.do(t, http.MethodGet, "/?class="+f.class, "", perms...)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var envelope struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
	return envelope.Data
}
