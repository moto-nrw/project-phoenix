package presence

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type checkinRoutingStudents struct {
	PresenceStudents
	student *StudentRecord
	err     error
}

func (r checkinRoutingStudents) FindByIDForUpdate(context.Context, int64) (*StudentRecord, error) {
	return r.student, r.err
}

type checkinRoutingVisits struct {
	StudentPresence
	beforeWrite func()
}

func (r checkinRoutingVisits) ListVisits(context.Context, studentpresence.VisitFilter) ([]studentpresence.Visit, error) {
	return nil, nil
}
func (r checkinRoutingVisits) ListAttendance(context.Context, studentpresence.AttendanceFilter) ([]studentpresence.Attendance, error) {
	return []studentpresence.Attendance{{StudentID: 42}}, nil
}
func (r checkinRoutingVisits) RecordVisit(_ context.Context, visit studentpresence.Visit) (studentpresence.Visit, error) {
	if r.beforeWrite != nil {
		r.beforeWrite()
	}
	visit.ID = 71
	return visit, nil
}

type checkinRoutingPublisher struct {
	EventPublisher
	visits   []studentpresence.LiveVisitChange
	groups   [][]string
	order    []string
	tenants  []int64
	detached bool
}

func (p *checkinRoutingPublisher) VisitCheckedIn(ctx context.Context, visit studentpresence.LiveVisitChange) {
	p.visits = append(p.visits, visit)
	p.order = append(p.order, "checkin")
	p.tenants = append(p.tenants, tenant.FromContext(ctx))
	_, inTx := tenant.TransactionFromContext(ctx)
	p.detached = !inTx
}
func (p *checkinRoutingPublisher) SupervisionRefresh(_ context.Context, _, _ string, groups []string) {
	p.groups = append(p.groups, groups)
	p.order = append(p.order, "refresh")
}

func TestCreateVisitRoutingUsesOnlyKnownGuardGroup(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		known     bool
		group     *int64
		lookupErr error
	}{
		{name: "known NULL", known: true},
		{name: "known zero", known: true, group: routingGroupPointer(0)},
		{name: "known positive", known: true, group: routingGroupPointer(17)},
		{name: "fallback NULL"},
		{name: "fallback zero", group: routingGroupPointer(0)},
		{name: "fallback positive", group: routingGroupPointer(17)},
		{name: "fallback error discards partial result", group: routingGroupPointer(17), lookupErr: errors.New("routing unavailable")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			want := tc.group
			if want != nil {
				want = routingGroupPointer(*want)
			}
			if tc.lookupErr != nil {
				want = nil
			}
			lookups := 0
			publisher := &checkinRoutingPublisher{}
			waker := &recordingWaker{}
			student := &StudentRecord{ID: 42, GroupID: tc.group}
			svc := &service{ServiceDependencies: ServiceDependencies{
				PrincipalReader: testAttendancePrincipal,
				GroupRepo:       &groupRepoForActiveWrapperTest{group: &ports.ActiveGroup{ID: 93}},
				SchoolPresence: checkinRoutingVisits{beforeWrite: func() {
					// No pointer into the guard record may escape into the event hook.
					if tc.known && student.GroupID != nil {
						*student.GroupID = 999
					}
				}},
				EducationGroupRepo: checkoutGroupsQuery{readOne: func(context.Context, int64) (*int64, error) { lookups++; return tc.group, tc.lookupErr }},
				AttendanceSyncer:   &recordingAttendanceSyncer{}, Broadcaster: publisher,
			}, guardianWaker: waker, settings: &fakeSettingsResolver{resolved: PresenceModeDetailed}}
			if tc.known {
				svc.StudentRepo = checkinRoutingStudents{student: student}
			}
			ctx, commit := tenant.WithAfterCommitHooksForTest(tenant.WithTenantID(context.Background(), 84))
			require.NoError(t, svc.createVisit(ctx, &studentpresence.Visit{StudentID: 42, ActiveGroupID: 93, EntryTime: time.Now()}))
			if tc.known {
				assert.Zero(t, lookups)
			} else {
				assert.Equal(t, 1, lookups)
			}
			assert.Empty(t, publisher.visits)
			assert.Empty(t, waker.calls)
			commit()
			require.Len(t, publisher.visits, 1)
			assert.Equal(t, want, publisher.visits[0].EducationGroupID)
			assert.Equal(t, "present", publisher.visits[0].Attendance.Status)
			assert.Equal(t, []string{"checkin", "refresh"}, publisher.order)
			assert.Equal(t, [][]string{eduGroupIDsOf(want)}, publisher.groups)
			assert.Equal(t, []int64{84}, publisher.tenants)
			assert.True(t, publisher.detached)
			require.Len(t, waker.calls, 1)
			assert.Equal(t, int64(84), waker.calls[0].tenantID)
			assert.Equal(t, int64(42), waker.calls[0].studentID)
		})
	}
}

func routingGroupPointer(id int64) *int64 { return &id }

func TestCreateVisitWithoutBroadcasterStillWakesGuardians(t *testing.T) {
	t.Parallel()
	for _, withWaker := range []bool{false, true} {
		t.Run(map[bool]string{false: "nil guardian waker", true: "guardian waker"}[withWaker], func(t *testing.T) {
			t.Parallel()
			waker := &recordingWaker{}
			svc := &service{}
			if withWaker {
				svc.guardianWaker = waker
			}
			ctx, commit := tenant.WithAfterCommitHooksForTest(tenant.WithTenantID(context.Background(), 84))
			// Neither known routing nor fallback may touch an unwired group reader.
			svc.broadcastVisitCreated(ctx, &studentpresence.Visit{StudentID: 42}, nil)
			svc.broadcastVisitCreated(ctx, &studentpresence.Visit{StudentID: 43}, nil, nil)
			assert.Empty(t, waker.calls)
			commit()
			if withWaker {
				require.Len(t, waker.calls, 2)
			} else {
				assert.Empty(t, waker.calls)
			}
		})
	}
}
