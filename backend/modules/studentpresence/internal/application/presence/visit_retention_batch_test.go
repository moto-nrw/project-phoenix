package presence_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api"
	activeService "github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/application/presence"
	"github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCleanupExpiredVisitsStatementsStayFlat counts the statements of a
// complete visit cleanup (#6455): the expired visits of every child of a
// batch go in one DELETE and their audit rows in one INSERT, so the count
// does not grow with the cohort.
func TestCleanupExpiredVisitsStatementsStayFlat(t *testing.T) {
	t.Parallel()
	testpkg.SetupTestDB(t)
	for _, count := range []int{3, 12} {
		t.Run(fmt.Sprintf("N%d", count), func(t *testing.T) {
			testpkg.OwnTenant(t)
			db := testpkg.SetupIsolatedTestDB(t)
			cleanup := setupCleanupService(t, db)
			group := testpkg.CreateTestActiveGroupForTenant(t, db, testpkg.Tenant(t))
			students := make([]int64, 0, count)
			for i := range count {
				student := testpkg.CreateTestStudent(t, db, "Synthetisch", fmt.Sprintf("Kohorte%d", i), "3a")
				if i%2 == 1 {
					insertConsent(t, db, student.ID, true, 20)
				}
				insertRetentionVisit(t, db, student.ID, group.ID, 45, true)
				students = append(students, student.ID)
			}

			counter := testpkg.CaptureQueries(t, db)
			result, err := cleanup.CleanupExpiredVisits(testpkg.Ctx(t))
			queries := counter.Queries()
			counter.Stop()
			require.NoError(t, err)
			require.True(t, result.Success, "errors: %v", result.Errors)
			assert.Equal(t, count, result.StudentsProcessed)
			assert.EqualValues(t, count, result.RecordsDeleted)
			for i, id := range students {
				audits := visitRetentionAudits(t, db, id)
				require.Len(t, audits, 1, "student %d", id)
				assert.Equal(t, 1, audits[0].RecordsDeleted)
				want := 30
				if i%2 == 1 {
					want = 20
				}
				assert.Equal(t, want, auditRetentionDays(t, audits[0]), "student %d", id)
			}
			t.Logf("visit cleanup N%d: %d statements", count, len(queries))
			if count == 3 {
				querybudgets.AssertQueryBudget(t, "modules.studentpresence.visit_retention.students_3", counter.Measurement(queries))
			} else {
				querybudgets.AssertQueryBudget(t, "modules.studentpresence.visit_retention.students_12", counter.Measurement(queries))
			}
		})
	}
}

// studentAuditFault fails the audit of one student after the real batch
// append, so the failure follows written audit rows of every student.
type studentAuditFault struct {
	activeService.DeletionAudit
	studentID int64
	err       error
}

func (f *studentAuditFault) CreateAll(ctx context.Context, events []*activeService.DeletionEvent) error {
	if err := f.DeletionAudit.CreateAll(ctx, events); err != nil {
		return err
	}
	for _, event := range events {
		if event.StudentID != nil && *event.StudentID == f.studentID {
			return f.err
		}
	}
	return nil
}

// TestCleanupExpiredVisitsIsolatesFailingStudent keeps the per-child error
// boundary of the batch cleanup: one child's failed audit rolls back only
// that child's deletion; the others are deleted, audited and counted.
func TestCleanupExpiredVisitsIsolatesFailingStudent(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	presence := testSchoolPresence(t, db)
	group := testpkg.CreateTestActiveGroupForTenant(t, db, testpkg.Tenant(t))
	var students, visits []int64
	for i := range 3 {
		student := testpkg.CreateTestStudent(t, db, "Synthetisch", fmt.Sprintf("Fehlergrenze%d", i), "3a")
		students = append(students, student.ID)
		visits = append(visits, insertRetentionVisit(t, db, student.ID, group.ID, 45, true))
	}
	open := insertRetentionVisit(t, db, students[0], group.ID, 45, false)
	recent := insertRetentionVisit(t, db, students[2], group.ID, 5, true)

	command, err := api.NewCleanupAuditCommand(slog.Default())
	require.NoError(t, err)
	repos := api.NewRetentionCleanupRepositories(testpkg.NewDatabase(t, db), command)
	injected := errors.New("synthetic audit failure")
	fault := &studentAuditFault{DeletionAudit: owners.NewDeletionAudit(repos.Deletion), studentID: students[1], err: injected}
	cleanup := activeService.NewCleanupService(presence, sessionSupervisors(repos.Sessions), fault, visitRetentionSetting{days: 30})

	result, err := cleanup.CleanupExpiredVisits(testpkg.Ctx(t))
	require.NoError(t, err, "per-student failures are reported in the cleanup result")
	assert.False(t, result.Success)
	assert.Equal(t, 3, result.StudentsProcessed)
	assert.EqualValues(t, 2, result.RecordsDeleted)
	require.Len(t, result.Errors, 1)
	assert.Equal(t, students[1], result.Errors[0].StudentID)
	assert.Contains(t, result.Errors[0].Error, injected.Error())

	assert.True(t, visitExists(t, db, visits[1]), "the failing child's deletion rolls back with its audit")
	assert.Empty(t, visitRetentionAudits(t, db, students[1]))
	for _, i := range []int{0, 2} {
		assert.False(t, visitExists(t, db, visits[i]), "student %d", students[i])
		audits := visitRetentionAudits(t, db, students[i])
		require.Len(t, audits, 1, "student %d", students[i])
		assert.Equal(t, 1, audits[0].RecordsDeleted)
	}
	assert.True(t, visitExists(t, db, open), "open visits stay")
	assert.True(t, visitExists(t, db, recent), "visits inside the window stay")

	fault.err = nil
	result, err = cleanup.CleanupExpiredVisits(testpkg.Ctx(t))
	require.NoError(t, err)
	require.True(t, result.Success, "errors: %v", result.Errors)
	assert.EqualValues(t, 1, result.RecordsDeleted)
	assert.False(t, visitExists(t, db, visits[1]))
	require.Len(t, visitRetentionAudits(t, db, students[1]), 1)
}

// TestDisabledVisitHistoryBatchKeepsDayBoundaryAudit deletes the completed
// visits of several children before the Berlin day boundary, in the nightly
// cleanup and when the setting is switched off, and records the boundary in
// each child's audit row. Another school's visits stay.
func TestDisabledVisitHistoryBatchKeepsDayBoundaryAudit(t *testing.T) {
	t.Parallel()
	for _, path := range []string{"cleanup", "setting"} {
		t.Run(path, func(t *testing.T) {
			t.Parallel()
			testpkg.OwnTenant(t)
			db := testpkg.SetupTestDB(t)
			day := calendar.NewDate(2026, 10, 25)
			command, err := api.NewCleanupAuditCommand(slog.Default())
			require.NoError(t, err)
			deletions := owners.NewDeletionAudit(api.NewRetentionCleanupRepositories(testpkg.NewDatabase(t, db), command).Deletion)
			cleanup := activeService.NewCleanupService(testSchoolPresence(t, db), nil, deletions, disabledVisitHistory{}, func() calendar.Date { return day })
			group := testpkg.CreateTestActiveGroupForTenant(t, db, testpkg.Tenant(t))
			before := day.BerlinMidnight().Add(-time.Microsecond)
			at := day.BerlinMidnight()
			type child struct{ id, old, today, open int64 }
			var children []child
			for i := range 2 {
				student := testpkg.CreateTestStudent(t, db, "Synthetisch", fmt.Sprintf("Tagesgrenze%d", i), "3a")
				insertConsent(t, db, student.ID, true, 31)
				children = append(children, child{
					id:    student.ID,
					old:   testpkg.CreateTestVisit(t, db, student.ID, group.ID, before.Add(-time.Hour), &before).ID,
					today: testpkg.CreateTestVisit(t, db, student.ID, group.ID, before.Add(-time.Hour), &at).ID,
					open:  testpkg.CreateTestVisit(t, db, student.ID, group.ID, before.Add(-time.Hour), nil).ID,
				})
			}
			otherTenant, _ := testpkg.CreateTestTenant(t, db)
			otherStudent := testpkg.CreateTestStudentForTenant(t, db, otherTenant, "Synthetisch", "AndereSchule", "3a")
			otherGroup := testpkg.CreateTestActiveGroupForTenant(t, db, otherTenant)
			otherVisit := testpkg.CreateTestVisitForTenant(t, db, otherTenant, otherStudent.ID, otherGroup.ID, before.Add(-time.Hour), &before)

			if path == "cleanup" {
				result, err := cleanup.CleanupExpiredVisits(testpkg.Ctx(t))
				require.NoError(t, err)
				require.True(t, result.Success, "errors: %v", result.Errors)
				assert.EqualValues(t, 2, result.RecordsDeleted)
			} else {
				require.NoError(t, testpkg.TenantCommand1(t, testpkg.Ctx(t), db, cleanup.ApplyAttendanceLogSetting, false))
			}
			for _, c := range children {
				assert.False(t, visitExists(t, db, c.old), "student %d", c.id)
				assert.True(t, visitExists(t, db, c.today), "student %d", c.id)
				assert.True(t, visitExists(t, db, c.open), "student %d", c.id)
				audits := visitRetentionAudits(t, db, c.id)
				require.Len(t, audits, 1, "student %d", c.id)
				assert.Equal(t, 1, audits[0].RecordsDeleted)
				var metadata struct {
					Enabled         *bool  `json:"attendance_log_enabled"`
					CompletedBefore string `json:"completed_before"`
					BatchCleanup    bool   `json:"batch_cleanup"`
				}
				require.NoError(t, json.Unmarshal(audits[0].Metadata, &metadata))
				require.NotNil(t, metadata.Enabled)
				assert.False(t, *metadata.Enabled)
				assert.True(t, metadata.BatchCleanup)
				completedBefore, err := time.Parse(time.RFC3339, metadata.CompletedBefore)
				require.NoError(t, err)
				assert.True(t, completedBefore.Equal(at), "the audit records the Berlin day boundary, got %s", metadata.CompletedBefore)
			}
			assert.True(t, visitExists(t, db, otherVisit.ID), "another school's completed visits must remain")
			assert.Empty(t, visitRetentionAudits(t, db, otherStudent.ID))
		})
	}
}

type disabledVisitHistory struct{}

func (disabledVisitHistory) VisitHistoryRetention(context.Context) (bool, int, error) {
	return false, 30, nil
}
