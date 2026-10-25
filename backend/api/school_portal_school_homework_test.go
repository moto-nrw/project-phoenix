package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// Homework feedback from a Lernzeit block (#4796), driven through the school
// portal's supervision surface: the same handler the OGS reaches under
// /api/timetable/operations. The roster bounds every write.

type homeworkRosterRow struct {
	StudentID int64 `json:"student_id"`
	Homework  []struct {
		Subject     *string `json:"subject"`
		Description string  `json:"description"`
	} `json:"homework"`
	Feedback *struct {
		Status string  `json:"status"`
		Note   *string `json:"note"`
	} `json:"homework_feedback"`
}

type homeworkRoster struct {
	Instance struct {
		ListKind *string `json:"list_kind"`
	} `json:"instance"`
	CanRecord bool                `json:"can_record_homework_feedback"`
	Rows      []homeworkRosterRow `json:"rows"`
}

func decodeHomeworkRoster(t *testing.T, body []byte) homeworkRoster {
	t.Helper()
	var envelope struct {
		Data homeworkRoster `json:"data"`
	}
	require.NoError(t, json.Unmarshal(body, &envelope), string(body))
	return envelope.Data
}

func (r homeworkRoster) row(studentID int64) *homeworkRosterRow {
	for i := range r.Rows {
		if r.Rows[i].StudentID == studentID {
			return &r.Rows[i]
		}
	}
	return nil
}

// supervisionBlock creates today's block, assigns the Lehrkraft, plans the
// children and, for a Lernzeit, sets the list kind.
func supervisionBlock(t *testing.T, db *bun.DB, tenantID, staffID int64, learningTime bool, students ...int64) int64 {
	t.Helper()
	inst := schoolSupervisionTodayInstance(t, db, tenantID, fmt.Sprintf("Block-%d", time.Now().UnixNano()))
	testpkg.CreateTestInstanceStaff(t, db, inst.ID, staffID, testpkg.InstanceStaffOpts{IsPrimary: true})
	for _, student := range students {
		testpkg.CreateTestInstanceStudent(t, db, inst.ID, student, testpkg.ScheduleAttendanceStatusExpected)
	}
	if learningTime {
		_, err := db.ExecContext(testpkg.Ctx(t), `UPDATE schedule.activity_instances SET list_kind = 'learning_time' WHERE id = ?`, inst.ID)
		require.NoError(t, err)
	}
	return inst.ID
}

func TestHomeworkFeedbackIsBoundToTheLernzeitRoster(t *testing.T) {
	t.Parallel()
	_, _, tenantID, router, claims, staffID := setupSupervisionFixture(t)
	// The package's one shared pool, the same the fixture wired.
	db := testpkg.SetupTestDB(t)
	day := calendar.NewDate(2026, 8, 24)
	suffix := time.Now().UnixNano() % 100000
	class, quietClass := fmt.Sprintf("3h%d", suffix), fmt.Sprintf("3q%d", suffix)
	child := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Lina", "Lernzeit", class)
	withoutHomework := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Max", "Ohne", quietClass)
	outsider := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Olga", "Fremd", class)
	_, err := db.ExecContext(testpkg.Ctx(t), `INSERT INTO education.class_homework (tenant_id, school_class, date, subject, description)
		VALUES (?, ?, ?, 'Mathe', 'Seite 12')`, tenantID, strings.ToUpper(class), day)
	require.NoError(t, err)

	lernzeit := supervisionBlock(t, db, tenantID, staffID, true, child.ID, withoutHomework.ID)
	feedback := func(instanceID, studentID int64, body string) (int, string) {
		rec := schoolSupervisionRequest(t, router, claims, http.MethodPut,
			fmt.Sprintf("/supervisions/%d/students/%d/homework-feedback", instanceID, studentID), strings.NewReader(body))
		return rec.Code, rec.Body.String()
	}

	// The Lernzeit roster shows the class's homework; the class name matches
	// case-insensitively.
	rec := schoolSupervisionRequest(t, router, claims, http.MethodGet, fmt.Sprintf("/supervisions/%d/roster", lernzeit), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	roster := decodeHomeworkRoster(t, rec.Body.Bytes())
	require.NotNil(t, roster.Instance.ListKind)
	assert.Equal(t, "learning_time", *roster.Instance.ListKind)
	assert.True(t, roster.CanRecord)
	require.NotNil(t, roster.row(child.ID))
	require.Len(t, roster.row(child.ID).Homework, 1)
	assert.Equal(t, "Seite 12", roster.row(child.ID).Homework[0].Description)
	assert.Empty(t, roster.row(withoutHomework.ID).Homework)

	code, body := feedback(lernzeit, child.ID, `{"status":"difficult","note":" Aufgabe 3 war schwer "}`)
	require.Equal(t, http.StatusOK, code, body)
	code, body = feedback(lernzeit, child.ID, `{"status":"done"}`)
	require.Equal(t, http.StatusOK, code, body, "a second feedback of the day replaces the first")
	rec = schoolSupervisionRequest(t, router, claims, http.MethodGet, fmt.Sprintf("/supervisions/%d/roster", lernzeit), nil)
	roster = decodeHomeworkRoster(t, rec.Body.Bytes())
	require.NotNil(t, roster.row(child.ID).Feedback)
	assert.Equal(t, "done", roster.row(child.ID).Feedback.Status)
	assert.Nil(t, roster.row(child.ID).Feedback.Note)

	// A child outside the supervision is refused.
	code, body = feedback(lernzeit, outsider.ID, `{"status":"done"}`)
	assert.Equal(t, http.StatusForbidden, code, body)
	assert.Contains(t, body, "timetable.homework_feedback_child_not_in_supervision")

	// A child whose class has no homework that day gets no feedback.
	code, body = feedback(lernzeit, withoutHomework.ID, `{"status":"not_done"}`)
	assert.Equal(t, http.StatusConflict, code, body)
	assert.Contains(t, body, "timetable.homework_feedback_without_homework")

	// An unknown status and an overlong note are refused.
	code, body = feedback(lernzeit, child.ID, `{"status":"late"}`)
	assert.Equal(t, http.StatusBadRequest, code, body)
	code, body = feedback(lernzeit, child.ID, `{"status":"done","note":"`+strings.Repeat("a", 201)+`"}`)
	assert.Equal(t, http.StatusBadRequest, code, body)

	// A block that is no Lernzeit takes no feedback, even for the same child.
	other := supervisionBlock(t, db, tenantID, staffID, false, child.ID)
	rec = schoolSupervisionRequest(t, router, claims, http.MethodGet, fmt.Sprintf("/supervisions/%d/roster", other), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	roster = decodeHomeworkRoster(t, rec.Body.Bytes())
	assert.False(t, roster.CanRecord)
	assert.Empty(t, roster.row(child.ID).Homework, "only a Lernzeit roster reads homework")
	code, body = feedback(other, child.ID, `{"status":"done"}`)
	assert.Equal(t, http.StatusForbidden, code, body)
	assert.Contains(t, body, "timetable.homework_feedback_not_allowed")

	// Another teacher's Lernzeit stays closed.
	otherStaff, _ := testpkg.CreateTestStaffWithAccountForTenant(t, db, tenantID, "Andere", fmt.Sprintf("Kraft-%d", time.Now().UnixNano()))
	foreign := supervisionBlock(t, db, tenantID, otherStaff.ID, true, child.ID)
	code, body = feedback(foreign, child.ID, `{"status":"done"}`)
	assert.Equal(t, http.StatusForbidden, code, body)

	var rows int
	require.NoError(t, db.NewRaw(`SELECT COUNT(*) FROM education.homework_feedback WHERE tenant_id = ?`, tenantID).Scan(testpkg.Ctx(t), &rows))
	assert.Equal(t, 1, rows, "only the accepted feedback is stored, once")
}
