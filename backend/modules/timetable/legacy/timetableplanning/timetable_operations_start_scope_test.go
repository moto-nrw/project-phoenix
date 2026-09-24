package timetableplanning

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// startScopeNow is a fixed Tuesday afternoon for the start-scope tests.
var startScopeNow = time.Date(2026, time.May, 12, 14, 0, 0, 0, timezone.Berlin)

const (
	startScopeAccount  = int64(3622)
	startScopePerson   = int64(3623)
	startScopeStaff    = int64(3624)
	startScopeInstance = int64(3625)
	startScopeTenant   = int64(3626)
)

type startScopeActor struct {
	name     string
	staff    []*scheduleModel.InstanceStaff
	admin    bool
	scope    string
	noPerson bool
}

func newStartScopeDeps(t *testing.T, startScope string, actor startScopeActor) (*timetableOpsTestDeps, context.Context) {
	t.Helper()
	deps := newTimetableOpsDeps()
	deps.service.(*timetableOperationsService).deps.Now = func() time.Time { return startScopeNow }
	wireAssignedStaff(deps, startScopeAccount, startScopePerson, startScopeStaff, startScopeInstance)
	deps.staffRepo.byInstance[startScopeInstance] = actor.staff
	if actor.noPerson {
		deps.personService.accountPerson = nil
	}
	deps.settings.scope = configModel.OverviewScopeAllStaff
	deps.settings.startScope = startScope
	deps.instanceRepo.byID[startScopeInstance] = instanceWithTimes(startScopeInstance, scheduleModel.InstanceStatusPlanned,
		startScopeNow.Add(-5*time.Minute), startScopeNow.Add(time.Hour))
	deps.instanceRepo.byDate = []*scheduleModel.ActivityInstance{deps.instanceRepo.byID[startScopeInstance]}
	ctx := tenant.WithTenantID(context.Background(), startScopeTenant)
	ctx = testpkg.IdentityContext(ctx, startScopeAccount, startScopeTenant, actor.scope, []string{"schedules:read"})
	return deps, ctx
}

// Starting follows operations.block_start_scope; completing keeps the
// assignment rule, whatever the setting says (#3622).
func TestTimetableStartScopeMatrix(t *testing.T) {
	t.Parallel()
	assigned := []*scheduleModel.InstanceStaff{{StaffID: startScopeStaff}}
	other := []*scheduleModel.InstanceStaff{{StaffID: 9999}}
	absent := []*scheduleModel.InstanceStaff{{StaffID: startScopeStaff, IsAbsent: true}}
	for _, tc := range []struct {
		actor                startScopeActor
		startOwn, startAll   bool
		completeWithAllStaff bool
	}{
		{actor: startScopeActor{name: "assigned", staff: assigned}, startOwn: true, startAll: true, completeWithAllStaff: true},
		{actor: startScopeActor{name: "not assigned", staff: other}, startAll: true},
		{actor: startScopeActor{name: "absent", staff: absent}, startAll: true},
		{actor: startScopeActor{name: "admin", staff: other, admin: true}, startOwn: true, startAll: true, completeWithAllStaff: true},
		{actor: startScopeActor{name: "school portal assigned", staff: assigned, scope: "school"}, startOwn: true, startAll: true, completeWithAllStaff: true},
		{actor: startScopeActor{name: "school portal not assigned", staff: other, scope: "school"}},
		{actor: startScopeActor{name: "parent portal", staff: other, scope: "parent"}},
	} {
		for _, startScope := range []string{configModel.BlockStartScopeOwn, configModel.BlockStartScopeAllStaff} {
			t.Run(tc.actor.name+"/"+startScope, func(t *testing.T) {
				t.Parallel()
				deps, ctx := newStartScopeDeps(t, startScope, tc.actor)
				want := tc.startOwn
				if startScope == configModel.BlockStartScopeAllStaff {
					want = tc.startAll
				}

				_, err := deps.service.Start(ctx, startScopeAccount, tc.actor.admin, startScopeInstance)
				if want {
					require.NoError(t, err)
					require.Len(t, deps.instanceService.started, 1)
					assert.Equal(t, startScopeStaff, deps.instanceService.started[0].staffID, "started_by names the starter")
				} else {
					require.ErrorIs(t, err, ErrTimetableOperationForbidden)
					assert.Empty(t, deps.instanceService.started)
				}
				assert.Equal(t, tc.actor.staff, deps.staffRepo.byInstance[startScopeInstance], "starting never changes the plan")

				planned, err := deps.service.PlannedNow(ctx, startScopeAccount, tc.actor.admin, timezone.DateFromTime(startScopeNow), startScopeNow, PlannedNowOptions{})
				require.NoError(t, err)
				if want {
					require.Len(t, planned, 1)
				}
				for _, instance := range planned {
					assert.Equal(t, want, instance.CanStart, "the list offers exactly the starts the command accepts")
				}

				deps.instanceRepo.byID[startScopeInstance] = activeInstance(startScopeInstance, 3627)
				_, err = deps.service.Complete(ctx, startScopeAccount, tc.actor.admin, startScopeInstance)
				if startScope == configModel.BlockStartScopeAllStaff && !tc.completeWithAllStaff {
					require.ErrorIs(t, err, ErrTimetableOperationForbidden)
				}
			})
		}
	}
}

func TestTimetableStartScopePreservesBoundaries(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		change func(*timetableOpsTestDeps, *context.Context)
		err    error
	}{
		{name: "personal visibility", change: func(d *timetableOpsTestDeps, _ *context.Context) {
			d.settings.scope = configModel.OverviewScopeOwn
		}, err: ErrTimetableOperationForbidden},
		{name: "unknown scope", change: func(d *timetableOpsTestDeps, _ *context.Context) {
			d.settings.startScope = "unknown"
		}, err: ErrTimetableOperationForbidden},
		{name: "block on another day", change: func(d *timetableOpsTestDeps, _ *context.Context) {
			d.instanceRepo.byID[startScopeInstance].Date = scheduleModel.Date(timezone.DateFromTime(startScopeNow).AddDays(1))
		}, err: ErrTimetableOperationForbidden},
		{name: "not staff", change: func(d *timetableOpsTestDeps, _ *context.Context) {
			d.personService.accountPerson = nil
		}, err: ErrTimetableOperationForbidden},
		{name: "other tenant token", change: func(_ *timetableOpsTestDeps, ctx *context.Context) {
			*ctx = testpkg.IdentityContext(*ctx, startScopeAccount, startScopeTenant+1, "", []string{"schedules:read"})
		}, err: ErrTimetableOperationForbidden},
		{name: "missing schedule permission", change: func(_ *timetableOpsTestDeps, ctx *context.Context) {
			*ctx = testpkg.IdentityContext(*ctx, startScopeAccount, startScopeTenant, "", nil)
		}, err: ErrTimetableOperationForbidden},
		{name: "settings failure", change: func(d *timetableOpsTestDeps, _ *context.Context) {
			d.settings.stringErr = errors.New("settings unavailable")
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			deps, ctx := newStartScopeDeps(t, configModel.BlockStartScopeAllStaff, startScopeActor{staff: []*scheduleModel.InstanceStaff{{StaffID: 9999}}})
			tc.change(deps, &ctx)

			_, err := deps.service.Start(ctx, startScopeAccount, false, startScopeInstance)

			require.Error(t, err)
			if tc.err != nil {
				require.ErrorIs(t, err, tc.err)
			}
			assert.Empty(t, deps.instanceService.started)
		})
	}
}

// The roster carries can_start from the same rule, next to can_operate.
func TestTimetableRosterReportsCanStart(t *testing.T) {
	t.Parallel()
	t.Run("planned block of the caller", func(t *testing.T) {
		t.Parallel()
		deps, ctx := newStartScopeDeps(t, configModel.BlockStartScopeOwn, startScopeActor{staff: []*scheduleModel.InstanceStaff{{StaffID: startScopeStaff}}})
		deps.settings.scope = configModel.OverviewScopeOwn

		roster, err := deps.service.Roster(ctx, startScopeAccount, false, startScopeInstance)

		require.NoError(t, err)
		assert.True(t, roster.CanStart)
		assert.True(t, roster.CanOperate)
	})
	t.Run("running block with the whole team starting", func(t *testing.T) {
		t.Parallel()
		deps, ctx := newStartScopeDeps(t, configModel.BlockStartScopeAllStaff, startScopeActor{staff: []*scheduleModel.InstanceStaff{{StaffID: 9999}}})
		deps.instanceRepo.byID[startScopeInstance] = activeInstance(startScopeInstance, 3628)
		deps.instanceRepo.byID[startScopeInstance].Date = scheduleModel.Date(timezone.DateFromTime(startScopeNow))

		roster, err := deps.service.Roster(ctx, startScopeAccount, false, startScopeInstance)

		require.NoError(t, err)
		assert.False(t, roster.CanOperate)
		assert.False(t, roster.CanStart, "a running block cannot be started again")
	})
}

// A school-wide scope only adds people: whoever the own rule admits keeps the
// action, also where the school-wide rule does not reach (a planned block for
// attendance, a personal overview, an unknown value).
func TestTimetableSchoolWideScopeNeverAdmitsFewerThanOwn(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		change func(*timetableOpsTestDeps)
	}{
		{name: "planned block", change: func(*timetableOpsTestDeps) {}},
		{name: "personal overview", change: func(d *timetableOpsTestDeps) { d.settings.scope = configModel.OverviewScopeOwn }},
		{name: "unknown value", change: func(d *timetableOpsTestDeps) {
			d.settings.attendanceScope, d.settings.startScope = "unknown", "unknown"
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			deps, ctx := newStartScopeDeps(t, configModel.BlockStartScopeAllStaff, startScopeActor{staff: []*scheduleModel.InstanceStaff{{StaffID: startScopeStaff}}})
			deps.settings.attendanceScope = configModel.AttendanceEditScopeAllStaff
			tc.change(deps)
			service := deps.service.(*timetableOperationsService)

			editor, err := service.requireCanEditAttendance(ctx, startScopeAccount, false, startScopeInstance)
			require.NoError(t, err)
			assert.Equal(t, startScopeStaff, editor)
			starter, err := service.requireCanStart(ctx, startScopeAccount, false, startScopeInstance)
			require.NoError(t, err)
			assert.Equal(t, startScopeStaff, starter)
		})
	}
}
