package compose

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// childQuotas is the Kinderkontingent per tenant, read the way the real
// Organisation & Tenancy binding reads it: for the tenant in context only.
// A tenant without an entry has no Kinderkontingent.
type childQuotas struct {
	mu     sync.Mutex
	limits map[int64]int
}

func newChildQuotas() *childQuotas { return &childQuotas{limits: map[int64]int{}} }

func (q *childQuotas) set(tenantID int64, limit int) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.limits[tenantID] = limit
}

func (q *childQuotas) ChildQuotaLimit(ctx context.Context) (int, bool, error) {
	tenantID := tenant.FromContext(ctx)
	if tenantID <= 0 {
		return 0, false, errors.New("tenant is required")
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	limit, ok := q.limits[tenantID]
	return limit, ok, nil
}

func quotaModule(t *testing.T, db *bun.DB, quotas *childQuotas) *schoolmembership.Module {
	t.Helper()
	module, err := New(Dependencies{DB: db, Observe: func(Observation) {}, Employment: testEmployment, ChildQuota: quotas})
	require.NoError(t, err)
	return module
}

// setMembership rewrites the fixture's membership row directly: the states
// under test (graduated, deleted, ended care) are arrangement, not behaviour.
func setMembership(t *testing.T, db *bun.DB, tenantID, studentID int64, assignment string, args ...any) {
	t.Helper()
	query := "UPDATE users.student_school_memberships SET " + assignment + " WHERE tenant_id = ? AND student_profile_id = ?"
	_, err := db.ExecContext(testpkg.Ctx(t), query, append(args, tenantID, studentID)...)
	require.NoError(t, err)
}

// unenrolledStudent is a child profile whose membership is soft-deleted, so
// Enroll can give it a fresh one and it does not count before that.
func unenrolledStudent(t *testing.T, db *bun.DB, tenantID int64, firstName string) int64 {
	t.Helper()
	student := testpkg.CreateTestStudentForTenant(t, db, tenantID, firstName, "Kontingent", "1a")
	setMembership(t, db, tenantID, student.ID, "deleted_at = NOW()")
	return student.ID
}

func enrollment(studentID int64) schoolmembership.StudentEnrollment {
	return schoolmembership.StudentEnrollment{StudentID: studentID, SchoolClass: "1a", Status: "active"}
}

func requireQuotaReached(t *testing.T, err error, booked, occupied, requested int) {
	t.Helper()
	require.ErrorIs(t, err, schoolmembership.ErrChildQuotaReached)
	var reached *schoolmembership.ChildQuotaReachedError
	require.ErrorAs(t, err, &reached)
	require.Equal(t, schoolmembership.ChildQuotaReachedError{Booked: booked, Occupied: occupied, Requested: requested}, *reached)
	require.Equal(t, "students.child_quota_reached", reached.ErrorCode())
}

func liveMembership(t *testing.T, db *bun.DB, tenantID, studentID int64) bool {
	t.Helper()
	count, err := db.NewSelect().TableExpr("users.student_school_memberships").
		Where("tenant_id = ?", tenantID).Where("student_profile_id = ?", studentID).
		Where("deleted_at IS NULL").Count(testpkg.Ctx(t))
	require.NoError(t, err)
	return count > 0
}

// TestChildQuotaCountsActiveAndPendingChildren pins the Kontingentzahl: the
// actively managed children of the Stichtagszahl plus the pending ones.
// Inactive, graduated, deleted and ended children free their place. With a
// Kinderkontingent of 3 and exactly one active and one pending child, one more
// child fits and reaches it exactly; the next one is refused.
func TestChildQuotaCountsActiveAndPendingChildren(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)
	quotas := newChildQuotas()
	quotas.set(tenantID, 3)
	module := quotaModule(t, db, quotas)

	testpkg.CreateTestStudent(t, db, "Aktiv", "Zählt", "1a")
	pending := testpkg.CreateTestStudent(t, db, "Vorgemerkt", "Zählt", "1a")
	setMembership(t, db, tenantID, pending.ID, "status = 'pending', enrolled_from = '2099-08-01'")
	pendingStarted := testpkg.CreateTestStudent(t, db, "Vorgemerkt", "Begonnen", "1a")
	setMembership(t, db, tenantID, pendingStarted.ID, "status = 'pending', enrolled_from = '2020-07-31'")
	for _, status := range []string{"inactive", "alumnus"} {
		student := testpkg.CreateTestStudent(t, db, "Frei", status, "1a")
		setMembership(t, db, tenantID, student.ID, "status = ?", status)
	}
	deleted := testpkg.CreateTestStudent(t, db, "Frei", "Gelöscht", "1a")
	setMembership(t, db, tenantID, deleted.ID, "deleted_at = NOW()")
	ended := testpkg.CreateTestStudent(t, db, "Frei", "Beendet", "1a")
	setMembership(t, db, tenantID, ended.ID, "enrolled_until = '2020-07-31'")
	endedPending := testpkg.CreateTestStudent(t, db, "Frei", "Vorgemerkt beendet", "1a")
	setMembership(t, db, tenantID, endedPending.ID, "status = 'pending', enrolled_until = '2020-07-31'")

	third := unenrolledStudent(t, db, tenantID, "Dritt")
	_, err := module.Enroll(ctx, enrollment(third))
	require.NoError(t, err, "the third counted child reaches the Kinderkontingent exactly")

	fourth := unenrolledStudent(t, db, tenantID, "Viert")
	_, err = module.Enroll(ctx, enrollment(fourth))
	requireQuotaReached(t, err, 3, 3, 1)
	require.False(t, liveMembership(t, db, tenantID, fourth), "a refused enrollment writes nothing")

	pendingChild := unenrolledStudent(t, db, tenantID, "Später")
	input := enrollment(pendingChild)
	input.Status = "pending"
	input.EnrolledFrom = "2099-08-01"
	_, err = module.Enroll(ctx, input)
	requireQuotaReached(t, err, 3, 3, 1)

	inactive := unenrolledStudent(t, db, tenantID, "Inaktiv")
	input = enrollment(inactive)
	input.Status = "inactive"
	_, err = module.Enroll(ctx, input)
	require.NoError(t, err, "a child that does not count never hits the Kinderkontingent")
}

func TestChildQuotaWithoutKinderkontingentHasNoLimit(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)
	module := quotaModule(t, db, newChildQuotas())

	for _, name := range []string{"Eins", "Zwei", "Drei"} {
		_, err := module.Enroll(ctx, enrollment(unenrolledStudent(t, db, tenantID, name)))
		require.NoError(t, err)
	}
}

// TestChildQuotaUnboundFailsClosed pins that a graph without the
// Kinderkontingent port cannot enroll past it by accident.
func TestChildQuotaUnboundFailsClosed(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, err := New(Dependencies{DB: db, Observe: func(Observation) {}, Employment: testEmployment})
	require.NoError(t, err)
	_, err = module.Enroll(testpkg.Ctx(t), enrollment(unenrolledStudent(t, db, testpkg.Tenant(t), "Ungebunden")))
	require.ErrorContains(t, err, "child quota is not bound")
}

// TestChildQuotaLetsCountedChildrenChange pins what a full school may still
// do: renew a child that already counts, and let a pending child that already
// counts start. It may even be above a lowered Kinderkontingent.
func TestChildQuotaLetsCountedChildrenChange(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)
	quotas := newChildQuotas()
	quotas.set(tenantID, 1)
	module := quotaModule(t, db, quotas)

	counted := testpkg.CreateTestStudent(t, db, "Schon", "Da", "1a")
	pending := testpkg.CreateTestStudent(t, db, "Bald", "Da", "1a")
	setMembership(t, db, tenantID, pending.ID, "status = 'pending', enrolled_from = '2099-08-01'")

	renewal := enrollment(counted.ID)
	renewal.SchoolClass = "2a"
	id, err := module.RenewEnrollment(ctx, renewal)
	require.NoError(t, err)
	require.Positive(t, id)

	moved, err := module.TransitionStatus(ctx, pending.ID, "pending", "active")
	require.NoError(t, err)
	require.True(t, moved, "a transition that does not raise the Kontingentzahl passes")

	_, err = module.Enroll(ctx, enrollment(unenrolledStudent(t, db, tenantID, "Neu")))
	requireQuotaReached(t, err, 1, 2, 1)
}

// TestChildQuotaRejectsStartedPendingActivation pins the scheduler boundary:
// a pending child with a started care interval does not count before the
// transition, so pending → active must be checked atomically.
func TestChildQuotaRejectsStartedPendingActivation(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)
	quotas := newChildQuotas()
	quotas.set(tenantID, 1)
	module := quotaModule(t, db, quotas)

	testpkg.CreateTestStudent(t, db, "Voll", "Belegt", "1a")
	pending := testpkg.CreateTestStudent(t, db, "Begonnen", "Vorgemerkt", "1a")
	setMembership(t, db, tenantID, pending.ID, "status = 'pending', enrolled_from = '2020-07-31'")

	moved, err := module.TransitionStatus(ctx, pending.ID, "pending", "active")
	requireQuotaReached(t, err, 1, 1, 1)
	require.False(t, moved)

	var status string
	err = db.NewSelect().TableExpr("users.student_school_memberships").
		ColumnExpr("status").Where("tenant_id = ?", tenantID).
		Where("student_profile_id = ?", pending.ID).Scan(ctx, &status)
	require.NoError(t, err)
	require.Equal(t, "pending", status, "a refused transition leaves the membership unchanged")
}

// TestChildQuotaChecksEveryWayBack pins the counting writes that bring a
// child back: renewing, setting the status, resuming care and reactivating a
// graduate. Reverting a grade transition skips the check by name.
func TestChildQuotaChecksEveryWayBack(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)
	quotas := newChildQuotas()
	quotas.set(tenantID, 1)
	module := quotaModule(t, db, quotas)

	testpkg.CreateTestStudent(t, db, "Voll", "Belegt", "1a")
	inactive := testpkg.CreateTestStudent(t, db, "Wieder", "Inaktiv", "1a")
	setMembership(t, db, tenantID, inactive.ID, "status = 'inactive'")
	ended := testpkg.CreateTestStudent(t, db, "Wieder", "Beendet", "1a")
	setMembership(t, db, tenantID, ended.ID, "enrolled_until = '2020-07-31'")
	graduates := []int64{
		testpkg.CreateTestStudent(t, db, "Abgang", "Eins", "4a").ID,
		testpkg.CreateTestStudent(t, db, "Abgang", "Zwei", "4a").ID,
	}
	for _, id := range graduates {
		setMembership(t, db, tenantID, id, "status = 'alumnus'")
	}

	_, err := module.RenewEnrollment(ctx, enrollment(inactive.ID))
	requireQuotaReached(t, err, 1, 1, 1)

	_, err = module.SetStatus(ctx, inactive.ID, "active")
	requireQuotaReached(t, err, 1, 1, 1)

	_, err = module.ResumeCare(ctx, ended.ID, "2099-09-01", "pending", "2099-08-31")
	requireQuotaReached(t, err, 1, 1, 1)
	resumed, err := module.ResumeCare(ctx, ended.ID, "2099-09-01", "inactive", "2099-08-31")
	require.NoError(t, err)
	require.True(t, resumed, "the refused resume left the ended interval in place")

	_, err = module.Reactivate(ctx, graduates, "active", schoolmembership.EnforceChildQuota)
	requireQuotaReached(t, err, 1, 1, 2)

	restored, err := module.Reactivate(ctx, graduates, "active", schoolmembership.SkipChildQuotaForGradeTransitionRevert)
	require.NoError(t, err)
	require.ElementsMatch(t, graduates, restored, "a revert brings every graduate back, even past the Kinderkontingent")
}

// TestChildQuotaJudgesABatchAsAWhole pins all or nothing: two graduates
// with one free place are refused together, not one by one.
func TestChildQuotaJudgesABatchAsAWhole(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)
	quotas := newChildQuotas()
	quotas.set(tenantID, 2)
	module := quotaModule(t, db, quotas)

	testpkg.CreateTestStudent(t, db, "Eins", "Belegt", "1a")
	graduates := []int64{
		testpkg.CreateTestStudent(t, db, "Abgang", "Eins", "4a").ID,
		testpkg.CreateTestStudent(t, db, "Abgang", "Zwei", "4a").ID,
	}
	for _, id := range graduates {
		setMembership(t, db, tenantID, id, "status = 'alumnus'")
	}

	_, err := module.Reactivate(ctx, graduates, "active", schoolmembership.EnforceChildQuota)
	requireQuotaReached(t, err, 2, 1, 2)

	var alumni int
	err = db.NewSelect().TableExpr("users.student_school_memberships").ColumnExpr("COUNT(*)").
		Where("tenant_id = ?", tenantID).Where("student_profile_id IN (?)", bun.List(graduates)).
		Where("status = 'alumnus'").Scan(ctx, &alumni)
	require.NoError(t, err)
	require.Equal(t, 2, alumni, "neither graduate came back")
}

// TestChildQuotaRefusalSurvivesACommittingCaller pins that a refused write
// is undone even when its caller swallows the error and commits.
func TestChildQuotaRefusalSurvivesACommittingCaller(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)
	quotas := newChildQuotas()
	quotas.set(tenantID, 1)
	module := quotaModule(t, db, quotas)
	testpkg.CreateTestStudent(t, db, "Voll", "Belegt", "1a")
	refused := unenrolledStudent(t, db, tenantID, "Abgelehnt")

	require.NoError(t, tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		_, err := module.Enroll(txCtx, enrollment(refused))
		requireQuotaReached(t, err, 1, 1, 1)
		return nil
	}))
	require.False(t, liveMembership(t, db, tenantID, refused))
}

// TestChildQuotaIsPerSchool pins tenant isolation: a full school does not
// block another one, and another school's children never count here.
func TestChildQuotaIsPerSchool(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)
	otherCtx, otherTenantID := otherTenantContext(t, db)
	quotas := newChildQuotas()
	quotas.set(tenantID, 1)
	quotas.set(otherTenantID, 1)
	module := quotaModule(t, db, quotas)

	testpkg.CreateTestStudent(t, db, "Voll", "Hier", "1a")
	_, err := module.Enroll(ctx, enrollment(unenrolledStudent(t, db, tenantID, "Hier")))
	requireQuotaReached(t, err, 1, 1, 1)

	_, err = module.Enroll(otherCtx, enrollment(unenrolledStudent(t, db, otherTenantID, "Dort")))
	require.NoError(t, err, "the other school has its own Kinderkontingent")
}

// TestChildQuotaHoldsUnderParallelEnrollments pins the exclusive lock: many
// enrollments racing for the last places never exceed the Kinderkontingent.
func TestChildQuotaHoldsUnderParallelEnrollments(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)
	const limit, contenders = 3, 6
	quotas := newChildQuotas()
	quotas.set(tenantID, limit)
	module := quotaModule(t, db, quotas)

	students := make([]int64, contenders)
	for i := range students {
		students[i] = unenrolledStudent(t, db, tenantID, "Parallel")
	}

	start := make(chan struct{})
	errs := make([]error, contenders)
	var wg sync.WaitGroup
	for i, id := range students {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, errs[i] = module.Enroll(ctx, enrollment(id))
		}()
	}
	close(start)
	wg.Wait()

	enrolled := 0
	for _, err := range errs {
		if err == nil {
			enrolled++
			continue
		}
		require.ErrorIs(t, err, schoolmembership.ErrChildQuotaReached)
	}
	require.Equal(t, limit, enrolled)
	live := 0
	for _, id := range students {
		if liveMembership(t, db, tenantID, id) {
			live++
		}
	}
	require.Equal(t, limit, live)
}
