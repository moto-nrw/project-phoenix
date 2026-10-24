package timetracking_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/workforcetest"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The typed root composes the workforce graph the internal route tests of
// this directory run over. It imports their package, so this external test
// file binds the seams they read.
func init() {
	workforcetest.ComposeAdmin = func(t *testing.T, clocks ...func() time.Time) (*bun.DB, workforcetest.AdminCapabilities) {
		t.Helper()
		db := testpkg.SetupTestDB(t)
		svc, users, err := api.NewWorkforceTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db), clocks...)
		require.NoError(t, err)
		capabilities := owners.NewWorkforceAdminCapabilities(users, svc.StaffAdmin, svc.TimeTracking)
		return db, workforcetest.AdminCapabilities{
			Staff: capabilities.Staff, Documents: capabilities.Documents, WorkSessions: capabilities.WorkSessions,
			StaffAbsences: capabilities.StaffAbsences, WorkTimeMonth: capabilities.WorkTimeMonth,
			BalanceAdjustments: capabilities.BalanceAdjustments, MonthClosing: capabilities.MonthClosing,
			Overview: capabilities.Overview, AuditLog: capabilities.AuditLog, TimeExport: capabilities.TimeExport,
			AbsenceWrites: svc.SickReports,
		}
	}
	workforcetest.ComposeCapability = func(t *testing.T, db *bun.DB, now func() time.Time) workforce.Capability {
		t.Helper()
		membership, err := owners.NewSchoolMembership(testpkg.NewDatabase(t, db))
		require.NoError(t, err)
		capability, err := owners.NewWorkforceWithClock(testpkg.NewDatabase(t, db), membership, now)
		require.NoError(t, err)
		return capability
	}
	workforcetest.ComposeRows = func(t *testing.T, db *bun.DB) workforcetest.Rows {
		t.Helper()
		repos, err := api.NewWorkforceTestRepositories(testpkg.NewDatabase(t, db))
		require.NoError(t, err)
		timetable, err := api.NewTimetableTestRepositories(testpkg.NewDatabase(t, db))
		require.NoError(t, err)
		members := owners.MustNewMembershipTestRepositories(testpkg.NewDatabase(t, db))
		return workforcetest.Rows{
			StaffAbsence: repos.StaffAbsence,
			FindShift: func(ctx context.Context, shiftID int64) (*workforcetest.ShiftRow, error) {
				shift, err := timetable.StaffShift.FindByID(ctx, shiftID)
				if err != nil {
					return nil, err
				}
				return &workforcetest.ShiftRow{Cancelled: shift.Cancelled, SickAbsenceID: shift.SickAbsenceID}, nil
			},
			FindStaff: func(ctx context.Context, staffID int64) (*workforcetest.StaffRow, error) {
				staff, err := members.Staff.FindByID(ctx, staffID)
				if err != nil {
					return nil, err
				}
				return &workforcetest.StaffRow{WorkTimeModelID: staff.WorkTimeModelID}, nil
			},
		}
	}
}
