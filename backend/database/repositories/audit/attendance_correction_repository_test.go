package audit_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// correctionFixture builds the two rows a correction references. The audit row
// carries real foreign keys, so a fabricated instance/student id would be
// rejected by the database rather than by the repository.
type correctionFixture struct {
	instanceID int64
	studentID  int64
}

func newCorrectionFixture(t *testing.T, db *bun.DB, ctx context.Context, tenantID int64) correctionFixture {
	t.Helper()

	room := testpkg.CreateTestRoomForTenant(t, db, tenantID, fmt.Sprintf("AC-Room-%d", time.Now().UnixNano()))
	student := testpkg.CreateTestStudentForTenant(t, db, tenantID, "AC-Stu", "Child", "2a")

	// Built inline rather than through CreateTestActivityInstance: the
	// cross-tenant case needs the instance in a tenant other than the test's
	// own, and the fixture has no ForTenant variant.
	instance := &scheduleModels.ActivityInstance{
		Date:      timezone.NewDate(2026, 4, 22),
		Title:     "AC-Instance",
		StartTime: time.Date(1, 1, 1, 14, 0, 0, 0, time.UTC),
		EndTime:   time.Date(1, 1, 1, 15, 0, 0, 0, time.UTC),
		RoomID:    room.ID,
		Status:    scheduleModels.InstanceStatusCompleted,
	}
	instance.SetTenantID(tenantID)
	_, err := db.NewInsert().Model(instance).ModelTableExpr(`schedule.activity_instances`).Exec(ctx)
	require.NoError(t, err)

	return correctionFixture{instanceID: instance.ID, studentID: student.ID}
}

func newCorrection(fixture correctionFixture, field, reason string) *auditModels.AttendanceCorrection {
	return &auditModels.AttendanceCorrection{
		InstanceID: fixture.instanceID,
		StudentID:  fixture.studentID,
		FieldName:  field,
		Reason:     reason,
	}
}

func TestAttendanceCorrectionRepository_CreateBatchAndList(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	repo := repositories.NewFactory(db).AttendanceCorrection
	fixture := newCorrectionFixture(t, db, ctx, testpkg.Tenant(t))

	require.NoError(t, repo.CreateBatch(ctx, []*auditModels.AttendanceCorrection{
		newCorrection(fixture, auditModels.AttendanceFieldStatus, "Verwechslung"),
		newCorrection(fixture, auditModels.AttendanceFieldNote, "Verwechslung"),
	}))

	rows, err := repo.ListByInstanceAndStudent(ctx, fixture.instanceID, fixture.studentID)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	for _, row := range rows {
		assert.Equal(t, testpkg.Tenant(t), row.TenantID, "the repository must stamp the tenant")
		assert.False(t, row.CreatedAt.IsZero(), "created_at is the correction timestamp")
	}
}

func TestAttendanceCorrectionRepository_CreateBatchEmptyIsNoOp(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repo := repositories.NewFactory(db).AttendanceCorrection

	require.NoError(t, repo.CreateBatch(testpkg.Ctx(t), nil))
	require.NoError(t, repo.CreateBatch(testpkg.Ctx(t), []*auditModels.AttendanceCorrection{}))
}

// A row that would be unreadable must not reach the table — the repository
// validates before it inserts, so a caller bug fails loudly instead of
// producing a trail entry nobody can interpret.
func TestAttendanceCorrectionRepository_CreateBatchRejectsInvalidRow(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	repo := repositories.NewFactory(db).AttendanceCorrection
	fixture := newCorrectionFixture(t, db, ctx, testpkg.Tenant(t))

	noReason := newCorrection(fixture, auditModels.AttendanceFieldNote, "")
	require.Error(t, repo.CreateBatch(ctx, []*auditModels.AttendanceCorrection{noReason}))

	rows, err := repo.ListByInstanceAndStudent(ctx, fixture.instanceID, fixture.studentID)
	require.NoError(t, err)
	assert.Empty(t, rows, "a rejected batch must leave nothing behind")
}

func TestAttendanceCorrectionRepository_CountByInstanceAndStudents(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	repo := repositories.NewFactory(db).AttendanceCorrection
	fixture := newCorrectionFixture(t, db, ctx, testpkg.Tenant(t))
	untouched := testpkg.CreateTestStudentForTenant(t, db, testpkg.Tenant(t), "AC-Stu", "Untouched", "2a")

	require.NoError(t, repo.CreateBatch(ctx, []*auditModels.AttendanceCorrection{
		newCorrection(fixture, auditModels.AttendanceFieldStatus, "Grund"),
		newCorrection(fixture, auditModels.AttendanceFieldNote, "Grund"),
	}))

	counts, err := repo.CountByInstanceAndStudents(ctx, fixture.instanceID,
		[]int64{fixture.studentID, untouched.ID})
	require.NoError(t, err)
	assert.Equal(t, 2, counts[fixture.studentID])
	assert.NotContains(t, counts, untouched.ID, "a child without corrections is absent, not zero")

	empty, err := repo.CountByInstanceAndStudents(ctx, fixture.instanceID, nil)
	require.NoError(t, err)
	assert.Empty(t, empty)
}

// The trail is tenant-scoped like every other audit table: a correction of one
// school must be invisible to another.
func TestAttendanceCorrectionRepository_IsolatesTenants(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repo := repositories.NewFactory(db).AttendanceCorrection

	ownCtx := testpkg.Ctx(t)
	own := newCorrectionFixture(t, db, ownCtx, testpkg.Tenant(t))
	require.NoError(t, repo.CreateBatch(ownCtx, []*auditModels.AttendanceCorrection{
		newCorrection(own, auditModels.AttendanceFieldNote, "eigener Mandant"),
	}))

	otherScope := testpkg.NewTenantScope(t, db)
	otherCtx := otherScope.Context()
	other := newCorrectionFixture(t, db, otherCtx, otherScope.TenantID)
	require.NoError(t, repo.CreateBatch(otherCtx, []*auditModels.AttendanceCorrection{
		newCorrection(other, auditModels.AttendanceFieldNote, "fremder Mandant"),
	}))

	rows, err := repo.ListByInstanceAndStudent(ownCtx, other.instanceID, other.studentID)
	require.NoError(t, err)
	assert.Empty(t, rows, "the other tenant's correction must not be readable")

	counts, err := repo.CountByInstanceAndStudents(ownCtx, other.instanceID, []int64{other.studentID})
	require.NoError(t, err)
	assert.Empty(t, counts)
}
