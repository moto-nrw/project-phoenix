package audit

import (
	"testing"

	audit "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// correctionFixture holds the two rows a correction references. The audit row
// carries real foreign keys, so a fabricated instance/student id would be
// rejected by the database rather than by the repository.
type correctionFixture struct {
	instanceID int64
	studentID  int64
}

// newCorrectionFixture takes the tenant explicitly: the isolation test needs a
// pair that belongs to somebody else.
func newCorrectionFixture(t *testing.T, tenantID int64) correctionFixture {
	t.Helper()
	instanceID, studentID := testpkg.CreateTestCompletedInstanceWithStudentForTenant(
		t, testpkg.SetupTestDB(t), tenantID)
	return correctionFixture{instanceID: instanceID, studentID: studentID}
}

func newCorrectionRepo(t *testing.T) audit.AttendanceCorrectionRepository {
	t.Helper()
	return NewAttendanceCorrectionRepository(NewRuntime(testpkg.SetupTestDB(t), auditTestTenantID))
}

func newCorrection(fixture correctionFixture, field, reason string) *audit.AttendanceCorrection {
	return &audit.AttendanceCorrection{
		InstanceID: fixture.instanceID,
		StudentID:  fixture.studentID,
		FieldName:  field,
		Reason:     reason,
	}
}

func TestAttendanceCorrectionRepository_CreateBatchAndList(t *testing.T) {
	t.Parallel()

	ctx := testpkg.Ctx(t)
	repo := newCorrectionRepo(t)
	fixture := newCorrectionFixture(t, testpkg.Tenant(t))

	require.NoError(t, testpkg.TenantCommand1(t, ctx, testpkg.SetupTestDB(t), repo.CreateBatch, []*audit.AttendanceCorrection{
		newCorrection(fixture, audit.AttendanceFieldStatus, "Verwechslung"),
		newCorrection(fixture, audit.AttendanceFieldNote, "Verwechslung"),
	}))

	rows, err := testpkg.TenantQuery2(t, ctx, testpkg.SetupTestDB(t), repo.ListByInstanceAndStudent, fixture.instanceID, fixture.studentID)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	for _, row := range rows {
		assert.Equal(t, testpkg.Tenant(t), row.TenantID, "the repository must stamp the tenant")
		assert.False(t, row.CreatedAt.IsZero(), "created_at is the correction timestamp")
	}
}

func TestAttendanceCorrectionRepository_CreateBatchEmptyIsNoOp(t *testing.T) {
	t.Parallel()

	repo := newCorrectionRepo(t)

	require.NoError(t, testpkg.TenantCommand1(t, testpkg.Ctx(t), testpkg.SetupTestDB(t), repo.CreateBatch, nil))
	require.NoError(t, testpkg.TenantCommand1(t, testpkg.Ctx(t), testpkg.SetupTestDB(t), repo.CreateBatch, []*audit.AttendanceCorrection{}))
}

// A row that would be unreadable must not reach the table — the repository
// validates before it inserts, so a caller bug fails loudly instead of
// producing a trail entry nobody can interpret.
func TestAttendanceCorrectionRepository_CreateBatchRejectsInvalidRow(t *testing.T) {
	t.Parallel()

	ctx := testpkg.Ctx(t)
	repo := newCorrectionRepo(t)
	fixture := newCorrectionFixture(t, testpkg.Tenant(t))

	noReason := newCorrection(fixture, audit.AttendanceFieldNote, "")
	require.Error(t, testpkg.TenantCommand1(t, ctx, testpkg.SetupTestDB(t), repo.CreateBatch, []*audit.AttendanceCorrection{noReason}))

	rows, err := testpkg.TenantQuery2(t, ctx, testpkg.SetupTestDB(t), repo.ListByInstanceAndStudent, fixture.instanceID, fixture.studentID)
	require.NoError(t, err)
	assert.Empty(t, rows, "a rejected batch must leave nothing behind")
}

func TestAttendanceCorrectionRepository_CountByInstanceAndStudents(t *testing.T) {
	t.Parallel()

	ctx := testpkg.Ctx(t)
	repo := newCorrectionRepo(t)
	fixture := newCorrectionFixture(t, testpkg.Tenant(t))
	untouched := newCorrectionFixture(t, testpkg.Tenant(t))

	require.NoError(t, testpkg.TenantCommand1(t, ctx, testpkg.SetupTestDB(t), repo.CreateBatch, []*audit.AttendanceCorrection{
		newCorrection(fixture, audit.AttendanceFieldStatus, "Grund"),
		newCorrection(fixture, audit.AttendanceFieldNote, "Grund"),
	}))

	counts, err := testpkg.TenantQuery2(t, ctx, testpkg.SetupTestDB(t), repo.CountByInstanceAndStudents, fixture.instanceID, []int64{fixture.studentID, untouched.studentID})
	require.NoError(t, err)
	assert.Equal(t, 2, counts[fixture.studentID])
	assert.NotContains(t, counts, untouched.studentID, "a child without corrections is absent, not zero")

	empty, err := testpkg.TenantQuery2(t, ctx, testpkg.SetupTestDB(t), repo.CountByInstanceAndStudents, fixture.instanceID, nil)
	require.NoError(t, err)
	assert.Empty(t, empty)
}

// The trail is tenant-scoped like every other audit table: a correction of one
// school must be invisible to another.
func TestAttendanceCorrectionRepository_IsolatesTenants(t *testing.T) {
	t.Parallel()

	repo := newCorrectionRepo(t)

	ownCtx := testpkg.Ctx(t)
	own := newCorrectionFixture(t, testpkg.Tenant(t))
	require.NoError(t, testpkg.TenantCommand1(t, ownCtx, testpkg.SetupTestDB(t), repo.CreateBatch, []*audit.AttendanceCorrection{
		newCorrection(own, audit.AttendanceFieldNote, "eigener Mandant"),
	}))

	otherScope := testpkg.NewTenantScope(t, testpkg.SetupTestDB(t))
	otherCtx := otherScope.Context()
	other := newCorrectionFixture(t, otherScope.TenantID)
	require.NoError(t, testpkg.TenantCommand1(t, otherCtx, testpkg.SetupTestDB(t), repo.CreateBatch, []*audit.AttendanceCorrection{
		newCorrection(other, audit.AttendanceFieldNote, "fremder Mandant"),
	}))

	rows, err := testpkg.TenantQuery2(t, ownCtx, testpkg.SetupTestDB(t), repo.ListByInstanceAndStudent, other.instanceID, other.studentID)
	require.NoError(t, err)
	assert.Empty(t, rows, "the other tenant's correction must not be readable")

	counts, err := testpkg.TenantQuery2(t, ownCtx, testpkg.SetupTestDB(t), repo.CountByInstanceAndStudents, other.instanceID, []int64{other.studentID})
	require.NoError(t, err)
	assert.Empty(t, counts)
}
