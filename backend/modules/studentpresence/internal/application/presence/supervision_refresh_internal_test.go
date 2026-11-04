package presence

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/stretchr/testify/require"
)

type supervisionRefresh struct {
	activeGroupID int64
	reason        string
}

// refreshRecordingPublisher records the queued supervision changes.
type refreshRecordingPublisher struct {
	recordingEventPublisher
	refreshes []supervisionRefresh
}

func (p *refreshRecordingPublisher) SupervisionChanged(_ context.Context, activeGroupID int64, reason string) {
	p.refreshes = append(p.refreshes, supervisionRefresh{activeGroupID: activeGroupID, reason: reason})
}

type visitOwnerForRefreshTest struct {
	StudentPresence
	visit     *studentpresence.Visit
	deleteErr error
}

func (r *visitOwnerForRefreshTest) FindVisit(context.Context, int64) (*studentpresence.Visit, error) {
	if r.visit == nil {
		return nil, studentpresence.ErrVisitNotFound
	}
	return r.visit, nil
}

func (r *visitOwnerForRefreshTest) DeleteVisit(context.Context, int64) error { return r.deleteErr }

// refreshTestContext runs units of work in place, with after-commit hooks:
// the refresh must wait for the commit and vanish on a rollback (#5017).
func refreshTestContext(t *testing.T) context.Context {
	t.Helper()
	runtime, err := tenant.NewUnitOfWork(
		func(ctx context.Context, _ int64, fn func(context.Context, any) error) error {
			return fn(ctx, struct{}{})
		},
		func(ctx context.Context, fn func(context.Context, any) error) error { return fn(ctx, struct{}{}) },
		func(context.Context, tenant.SavepointAction) error { return nil },
		func(error) bool { return false },
		func(context.Context, string, bool) error { return nil },
	)
	require.NoError(t, err)
	return tenant.WithTenantID(tenant.WithUnitOfWork(context.Background(), runtime), 42)
}

func TestClaimAnnouncesTheNewSupervisor(t *testing.T) {
	t.Parallel()
	ctx := refreshTestContext(t)
	publisher := &refreshRecordingPublisher{}
	owner := &claimOwnerForActiveWrapperTest{}
	svc := &service{ServiceDependencies: ServiceDependencies{PrincipalReader: testAttendancePrincipal,
		StaffRepo:      &staffRepoForActiveWrapperTest{found: true},
		SchoolPresence: owner,
		Broadcaster:    publisher,
	}}

	_, err := svc.ClaimActiveGroup(ctx, 7, 84, "")
	require.NoError(t, err)
	require.Equal(t, []supervisionRefresh{{activeGroupID: 7, reason: activeSupervisionReasonSupervisorsChanged}}, publisher.refreshes)

	owner.err = studentpresence.ErrAlreadySupervising
	_, err = svc.ClaimActiveGroup(ctx, 7, 84, "")
	require.ErrorIs(t, err, ErrStaffAlreadySupervising)
	require.Len(t, publisher.refreshes, 1, "a refused claim announces nothing")
}

func TestDeleteVisitAnnouncesItsSession(t *testing.T) {
	t.Parallel()
	ctx := refreshTestContext(t)
	publisher := &refreshRecordingPublisher{}
	owner := &visitOwnerForRefreshTest{visit: &studentpresence.Visit{ID: 3, ActiveGroupID: 11}}
	svc := &service{ServiceDependencies: ServiceDependencies{SchoolPresence: owner, Broadcaster: publisher}}

	require.NoError(t, svc.DeleteVisit(ctx, 3))
	require.Equal(t, []supervisionRefresh{{activeGroupID: 11, reason: activeSupervisionReasonStudentMoved}}, publisher.refreshes)

	owner.deleteErr = errors.New("delete failed")
	require.Error(t, svc.DeleteVisit(ctx, 3))
	require.Len(t, publisher.refreshes, 1, "a failed delete announces nothing")
}
