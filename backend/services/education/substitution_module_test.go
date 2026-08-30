package education_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	educationModels "github.com/moto-nrw/project-phoenix/models/education"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	substitution "github.com/moto-nrw/project-phoenix/services/education"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

var fixedNow = time.Date(2026, time.August, 29, 10, 0, 0, 0, timezone.Berlin)

func TestGroupHandoverExternalInterface(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	testpkg.Tenant(t)
	repos := repositories.NewFactory(db)
	service := substitution.NewSubstitutionModule(substitution.SubstitutionDependencies{
		Groups: repos.Group, Substitutions: repos.GroupSubstitution, Teachers: repos.Teacher, Staff: repos.Staff,
		Audit: repos.SubstitutionChange, DB: db, Now: func() time.Time { return fixedNow },
	})
	group := testpkg.CreateTestEducationGroup(t, db, "Robins Gruppe")
	owner, ownerAccountID := activeTeacher(t, db, "Robin", "Owner")
	target, _ := activeTeacher(t, db, "Toni", "Target")
	require.NotZero(t, owner.ID)
	require.Equal(t, testpkg.Tenant(t), owner.TenantID)
	require.Equal(t, testpkg.Tenant(t), group.TenantID)
	testpkg.CreateTestGroupTeacher(t, db, group.ID, owner.ID)
	ctx := testpkg.Ctx(t)
	caller := substitutionCaller(t, ownerAccountID, false)
	created, err := service.Assign(ctx, caller, substitution.Assignment{
		Type:          substitution.TargetGroupHandover,
		GroupHandover: &substitution.GroupHandoverAssignment{GroupID: group.ID, TargetStaffID: target.StaffID},
	})
	require.NoError(t, err)
	require.Equal(t, substitution.TargetGroupHandover, created.Type)
	require.Equal(t, fixedNow.Format(time.DateOnly), created.Period.StartDate)
	require.Equal(t, "Toni Target", created.Target.FullName)

	overview, err := service.Overview(ctx, caller, substitution.OverviewQuery{GroupID: group.ID, IncludeTargets: true})
	require.NoError(t, err)
	require.Len(t, overview.GroupHandovers, 1)
	require.True(t, overview.GroupHandovers[0].CanEnd)
	require.Contains(t, overview.Targets, substitution.StaffRef{ID: target.StaffID, FullName: "Toni Target"})
	payload, err := json.Marshal(overview)
	require.NoError(t, err)
	require.NotContains(t, string(payload), "account_id")
	require.NotContains(t, string(payload), "email")
	require.NotContains(t, string(payload), "first_name")

	var auditCount int
	require.NoError(t, db.NewSelect().TableExpr(`audit.substitution_changes AS "change"`).
		ColumnExpr("COUNT(*)").Where(`"change".substitution_id = ?`, created.ID).Scan(ctx, &auditCount))
	require.Equal(t, 1, auditCount)

	_, err = service.Assign(ctx, caller, substitution.Assignment{
		Type:          substitution.TargetGroupHandover,
		GroupHandover: &substitution.GroupHandoverAssignment{GroupID: group.ID, TargetStaffID: target.StaffID},
	})
	require.ErrorIs(t, err, substitution.ErrAlreadyAssigned)
	require.NoError(t, service.End(ctx, caller, substitution.EndRequest{Type: substitution.TargetGroupHandover, ID: created.ID}))
	require.NoError(t, db.NewSelect().TableExpr(`audit.substitution_changes AS "change"`).
		ColumnExpr("COUNT(*)").Where(`"change".substitution_id = ?`, created.ID).Scan(ctx, &auditCount))
	require.Equal(t, 2, auditCount)
}

func TestGroupHandoverPermissionsAndPeriod(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	testpkg.Tenant(t)
	repos := repositories.NewFactory(db)
	service := substitution.NewSubstitutionModule(substitution.SubstitutionDependencies{
		Groups: repos.Group, Substitutions: repos.GroupSubstitution, Teachers: repos.Teacher, Staff: repos.Staff,
		Audit: repos.SubstitutionChange, DB: db, Now: func() time.Time { return fixedNow },
	})
	owned := testpkg.CreateTestEducationGroup(t, db, "Own")
	foreign := testpkg.CreateTestEducationGroup(t, db, "Other")
	owner, accountID := activeTeacher(t, db, "Alex", "Owner")
	target, targetAccountID := activeTeacher(t, db, "Chris", "Target")
	testpkg.CreateTestGroupTeacher(t, db, owned.ID, owner.ID)
	ctx := testpkg.Ctx(t)
	caller := substitutionCaller(t, accountID, false)
	unauthorized := caller
	unauthorized.Roles = nil
	_, err := service.Overview(ctx, unauthorized, substitution.OverviewQuery{})
	require.ErrorIs(t, err, substitution.ErrForbidden)

	_, err = service.Assign(ctx, caller, substitution.Assignment{Type: substitution.TargetGroupHandover,
		GroupHandover: &substitution.GroupHandoverAssignment{GroupID: owned.ID, TargetStaffID: owner.StaffID}})
	require.ErrorIs(t, err, substitution.ErrInvalidTarget)

	_, err = service.Assign(ctx, caller, substitution.Assignment{Type: substitution.TargetGroupHandover,
		GroupHandover: &substitution.GroupHandoverAssignment{GroupID: foreign.ID, TargetStaffID: target.StaffID}})
	require.ErrorIs(t, err, substitution.ErrNotFound)
	tomorrow := timezone.DateFromTime(fixedNow).AddDays(1)
	_, err = service.Assign(ctx, caller, substitution.Assignment{Type: substitution.TargetGroupHandover,
		GroupHandover: &substitution.GroupHandoverAssignment{GroupID: owned.ID, TargetStaffID: target.StaffID, StartDate: &tomorrow, EndDate: &tomorrow}})
	require.ErrorIs(t, err, substitution.ErrInvalidPeriod)
	received, err := service.Assign(ctx, caller, substitution.Assignment{Type: substitution.TargetGroupHandover,
		GroupHandover: &substitution.GroupHandoverAssignment{GroupID: owned.ID, TargetStaffID: target.StaffID}})
	require.NoError(t, err)
	receivedOverview, err := service.Overview(ctx, substitutionCaller(t, targetAccountID, false), substitution.OverviewQuery{GroupID: owned.ID})
	require.NoError(t, err)
	require.Len(t, receivedOverview.GroupHandovers, 1)
	require.False(t, receivedOverview.GroupHandovers[0].CanEnd)
	require.ErrorIs(t, service.End(ctx, substitutionCaller(t, targetAccountID, false), substitution.EndRequest{
		Type: substitution.TargetGroupHandover, ID: received.ID,
	}), substitution.ErrNotFound)
	require.NoError(t, service.End(ctx, caller, substitution.EndRequest{Type: substitution.TargetGroupHandover, ID: received.ID}))
	dualRoleCaller := caller
	dualRoleCaller.Admin = true
	todayHandover, err := service.Assign(ctx, dualRoleCaller, substitution.Assignment{
		Type: substitution.TargetGroupHandover,
		GroupHandover: &substitution.GroupHandoverAssignment{
			GroupID: owned.ID, TargetStaffID: target.StaffID,
		},
	})
	require.NoError(t, err)
	require.Equal(t, timezone.DateFromTime(fixedNow).String(), todayHandover.Period.StartDate)
	require.Equal(t, todayHandover.Period.StartDate, todayHandover.Period.EndDate)
	require.NoError(t, service.End(ctx, dualRoleCaller, substitution.EndRequest{
		Type: substitution.TargetGroupHandover, ID: todayHandover.ID,
	}))

	admin := testpkg.CreateTestAccount(t, db, "substitution-admin")
	end := tomorrow.AddDays(3)
	adminCaller := substitutionCaller(t, admin.ID, true)
	_, err = service.Assign(ctx, adminCaller, substitution.Assignment{Type: substitution.TargetGroupHandover,
		GroupHandover: &substitution.GroupHandoverAssignment{
			GroupID: foreign.ID, TargetStaffID: target.StaffID, StartDate: &tomorrow,
		}})
	require.ErrorIs(t, err, substitution.ErrInvalidPeriod)
	created, err := service.Assign(ctx, adminCaller, substitution.Assignment{Type: substitution.TargetGroupHandover,
		GroupHandover: &substitution.GroupHandoverAssignment{GroupID: foreign.ID, TargetStaffID: target.StaffID, StartDate: &tomorrow, EndDate: &end}})
	require.NoError(t, err)
	require.Equal(t, end.String(), created.Period.EndDate)
	futureOwn, err := service.Assign(ctx, adminCaller, substitution.Assignment{Type: substitution.TargetGroupHandover,
		GroupHandover: &substitution.GroupHandoverAssignment{GroupID: owned.ID, TargetStaffID: target.StaffID, StartDate: &tomorrow, EndDate: &end}})
	require.NoError(t, err)
	ownerOverview, err := service.Overview(ctx, caller, substitution.OverviewQuery{})
	require.NoError(t, err)
	require.Empty(t, ownerOverview.GroupHandovers)
	require.ErrorIs(t, service.End(ctx, caller, substitution.EndRequest{Type: substitution.TargetGroupHandover, ID: futureOwn.ID}), substitution.ErrNotRunning)

	otherTenant, _ := testpkg.CreateTestTenant(t, db)
	otherGroup := testpkg.CreateTestEducationGroupForTenant(t, db, otherTenant, "Other tenant")
	_, err = service.Assign(ctx, adminCaller, substitution.Assignment{Type: substitution.TargetGroupHandover,
		GroupHandover: &substitution.GroupHandoverAssignment{GroupID: otherGroup.ID, TargetStaffID: target.StaffID, StartDate: &tomorrow, EndDate: &tomorrow}})
	require.ErrorIs(t, err, substitution.ErrNotFound)

	regularStaffID := owner.StaffID
	legacy := testpkg.CreateTestGroupSubstitution(t, db, foreign.ID, &regularStaffID, target.StaffID, tomorrow, tomorrow)
	adminOverview, err := service.Overview(ctx, adminCaller, substitution.OverviewQuery{On: &tomorrow})
	require.NoError(t, err)
	for _, handover := range adminOverview.GroupHandovers {
		require.NotEqual(t, legacy.ID, handover.ID)
	}
	require.ErrorIs(t, service.End(ctx, adminCaller, substitution.EndRequest{Type: substitution.TargetGroupHandover, ID: legacy.ID}), substitution.ErrNotFound)
	_, err = repos.GroupSubstitution.FindByID(ctx, legacy.ID)
	require.NoError(t, err)
}

func TestGroupHandoverAllStaffVisibilityDoesNotGrantActions(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	testpkg.Tenant(t)
	repos := repositories.NewFactory(db)
	service := substitution.NewSubstitutionModule(substitution.SubstitutionDependencies{
		Groups: repos.Group, Substitutions: repos.GroupSubstitution, Teachers: repos.Teacher, Staff: repos.Staff,
		Audit: repos.SubstitutionChange, DB: db, Now: func() time.Time { return fixedNow },
		CanSeeAll: func(context.Context, bool, bool, bool) (bool, error) { return true, nil },
	})
	group := testpkg.CreateTestEducationGroup(t, db, "Visible foreign group")
	owner, ownerAccountID := activeTeacher(t, db, "Olivia", "Owner")
	_, observerAccountID := activeTeacher(t, db, "Vera", "Viewer")
	target, _ := activeTeacher(t, db, "Toni", "Target")
	testpkg.CreateTestGroupTeacher(t, db, group.ID, owner.ID)
	ctx := testpkg.Ctx(t)

	created, err := service.Assign(ctx, substitutionCaller(t, ownerAccountID, false), substitution.Assignment{
		Type: substitution.TargetGroupHandover,
		GroupHandover: &substitution.GroupHandoverAssignment{
			GroupID: group.ID, TargetStaffID: target.StaffID,
		},
	})
	require.NoError(t, err)

	observerCaller := substitutionCaller(t, observerAccountID, false)
	overview, err := service.Overview(ctx, observerCaller, substitution.OverviewQuery{GroupID: group.ID})
	require.NoError(t, err)
	require.Len(t, overview.GroupHandovers, 1)
	require.False(t, overview.GroupHandovers[0].CanEnd)

	_, err = service.Assign(ctx, observerCaller, substitution.Assignment{
		Type: substitution.TargetGroupHandover,
		GroupHandover: &substitution.GroupHandoverAssignment{
			GroupID: group.ID, TargetStaffID: target.StaffID,
		},
	})
	require.ErrorIs(t, err, substitution.ErrNotFound)
	require.ErrorIs(t, service.End(ctx, observerCaller, substitution.EndRequest{
		Type: substitution.TargetGroupHandover, ID: created.ID,
	}), substitution.ErrNotFound)

	schoolCaller := observerCaller
	schoolCaller.Scope = "school"
	_, err = service.Overview(ctx, schoolCaller, substitution.OverviewQuery{GroupID: group.ID})
	require.ErrorIs(t, err, substitution.ErrForbidden)
}

func TestGroupHandoverAuditFailureRollsBack(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	testpkg.Tenant(t)
	repos := repositories.NewFactory(db)
	service := substitution.NewSubstitutionModule(substitution.SubstitutionDependencies{
		Groups: repos.Group, Substitutions: repos.GroupSubstitution, Teachers: repos.Teacher, Staff: repos.Staff,
		Audit: failingAudit{}, DB: db, Now: func() time.Time { return fixedNow },
	})
	group := testpkg.CreateTestEducationGroup(t, db, "Rollback")
	target, _ := activeTeacher(t, db, "Sam", "Target")
	admin := testpkg.CreateTestAccount(t, db, "rollback-admin")
	today := timezone.DateFromTime(fixedNow)
	_, err := service.Assign(testpkg.Ctx(t), substitutionCaller(t, admin.ID, true), substitution.Assignment{Type: substitution.TargetGroupHandover,
		GroupHandover: &substitution.GroupHandoverAssignment{GroupID: group.ID, TargetStaffID: target.StaffID, StartDate: &today, EndDate: &today}})
	require.Error(t, err)
	rows, listErr := repos.GroupSubstitution.FindByGroup(testpkg.Ctx(t), group.ID)
	require.NoError(t, listErr)
	require.Empty(t, rows)

	workingService := substitution.NewSubstitutionModule(substitution.SubstitutionDependencies{
		Groups: repos.Group, Substitutions: repos.GroupSubstitution, Teachers: repos.Teacher, Staff: repos.Staff,
		Audit: repos.SubstitutionChange, DB: db, Now: func() time.Time { return fixedNow },
	})
	created, err := workingService.Assign(testpkg.Ctx(t), substitutionCaller(t, admin.ID, true), substitution.Assignment{
		Type: substitution.TargetGroupHandover,
		GroupHandover: &substitution.GroupHandoverAssignment{
			GroupID: group.ID, TargetStaffID: target.StaffID, StartDate: &today, EndDate: &today,
		},
	})
	require.NoError(t, err)
	require.Error(t, service.End(testpkg.Ctx(t), substitutionCaller(t, admin.ID, true), substitution.EndRequest{
		Type: substitution.TargetGroupHandover, ID: created.ID,
	}))
	_, err = repos.GroupSubstitution.FindByID(testpkg.Ctx(t), created.ID)
	require.NoError(t, err)
}

func TestGroupHandoverSignalsOnlyAfterCommit(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	testpkg.Tenant(t)
	repos := repositories.NewFactory(db)
	broadcaster := testpkg.NewRecordingBroadcaster()
	service := substitution.NewSubstitutionModule(substitution.SubstitutionDependencies{
		Groups: repos.Group, Substitutions: repos.GroupSubstitution, Teachers: repos.Teacher, Staff: repos.Staff,
		Audit: repos.SubstitutionChange, DB: db, Broadcaster: broadcaster,
		Now: func() time.Time { return fixedNow },
	})
	group := testpkg.CreateTestEducationGroup(t, db, "Signals")
	target, _ := activeTeacher(t, db, "Siggi", "Signal")
	admin := testpkg.CreateTestAccount(t, db, "signal-admin")
	today := timezone.DateFromTime(fixedNow)
	caller := substitutionCaller(t, admin.ID, true)

	assignCtx, commitAssign := testpkg.WithAfterCommitHooks(testpkg.Ctx(t))
	created, err := service.Assign(assignCtx, caller, substitution.Assignment{
		Type: substitution.TargetGroupHandover,
		GroupHandover: &substitution.GroupHandoverAssignment{
			GroupID: group.ID, TargetStaffID: target.StaffID, StartDate: &today, EndDate: &today,
		},
	})
	require.NoError(t, err)
	require.Empty(t, broadcaster.Events())
	commitAssign()
	require.Len(t, broadcaster.Events(), 1)
	require.Equal(t, "group_access_changed", string(broadcaster.Events()[0].Type))

	endCtx, commitEnd := testpkg.WithAfterCommitHooks(testpkg.Ctx(t))
	require.NoError(t, service.End(endCtx, caller, substitution.EndRequest{
		Type: substitution.TargetGroupHandover, ID: created.ID,
	}))
	require.Len(t, broadcaster.Events(), 1)
	commitEnd()
	require.Len(t, broadcaster.Events(), 2)
	require.Equal(t, "group_access_changed", string(broadcaster.Events()[1].Type))
}

func TestGroupHandoverRechecksOwnershipAfterGroupLock(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	testpkg.Tenant(t)
	repos := repositories.NewFactory(db)
	group := testpkg.CreateTestEducationGroup(t, db, "OwnershipRace")
	owner, accountID := activeTeacher(t, db, "Owner", "Race")
	target, _ := activeTeacher(t, db, "Target", "Race")
	testpkg.CreateTestGroupTeacher(t, db, group.ID, owner.ID)
	groups := &ownershipRevokingGroups{
		GroupStore: repos.Group, links: repos.GroupTeacher, teacherID: owner.ID,
	}
	service := substitution.NewSubstitutionModule(substitution.SubstitutionDependencies{
		Groups: groups, Substitutions: repos.GroupSubstitution, Teachers: repos.Teacher, Staff: repos.Staff,
		Audit: repos.SubstitutionChange, DB: db, Now: func() time.Time { return fixedNow },
	})

	_, err := service.Assign(testpkg.Ctx(t), substitutionCaller(t, accountID, false), substitution.Assignment{
		Type: substitution.TargetGroupHandover,
		GroupHandover: &substitution.GroupHandoverAssignment{
			GroupID: group.ID, TargetStaffID: target.StaffID,
		},
	})
	require.ErrorIs(t, err, substitution.ErrNotFound)
	rows, err := repos.GroupSubstitution.FindByGroup(testpkg.Ctx(t), group.ID)
	require.NoError(t, err)
	require.Empty(t, rows)
	relations, err := repos.GroupTeacher.FindByGroup(testpkg.Ctx(t), group.ID)
	require.NoError(t, err)
	require.Len(t, relations, 1, "the simulated concurrent removal must roll back with the rejected assignment")
}

type ownershipRevokingGroups struct {
	substitution.GroupStore
	links     educationModels.GroupTeacherRepository
	teacherID int64
}

func (g *ownershipRevokingGroups) FindByIDForUpdate(ctx context.Context, id any) (*educationModels.Group, error) {
	group, err := g.GroupStore.FindByIDForUpdate(ctx, id)
	if err != nil {
		return nil, err
	}
	relations, err := g.links.FindByGroup(ctx, group.ID)
	if err != nil {
		return nil, err
	}
	for _, relation := range relations {
		if relation.TeacherID == g.teacherID {
			if err := g.links.Delete(ctx, relation.ID); err != nil {
				return nil, err
			}
			break
		}
	}
	return group, nil
}

type failingAudit struct{}

func (failingAudit) Create(context.Context, *auditModels.SubstitutionChange) error {
	return errors.New("audit unavailable")
}

func activeTeacher(t *testing.T, db *bun.DB, firstName, lastName string) (*userModels.Teacher, int64) {
	t.Helper()
	staff, account := testpkg.CreateTestCalendarStaff(t, db, firstName, lastName)
	teacher := &userModels.Teacher{StaffID: staff.ID, Staff: staff}
	teacher.SetTenantID(testpkg.Tenant(t))
	_, err := db.NewInsert().Model(teacher).ModelTableExpr("users.teachers").Exec(context.Background())
	require.NoError(t, err)
	return teacher, account.ID
}

func substitutionCaller(t *testing.T, accountID int64, admin bool) substitution.SubstitutionCaller {
	t.Helper()
	return substitution.SubstitutionCaller{
		AccountID: accountID, TenantID: testpkg.Tenant(t), Roles: []string{"user"}, Admin: admin,
	}
}
