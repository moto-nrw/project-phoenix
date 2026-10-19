package care_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/workflows/parentportal"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// childResolver binds messaging's ChildResolver to the real guardian-child
// resolution, as the portal composition does in production.
func childResolver(db *bun.DB, repos *api.RepositoryTestFactory, reposPeople *api.PeopleRepositorySuiteFactory) parentportal.ChildResolver {
	return parentportal.NewChildFlows(parentportal.ChildConfig{
		GuardianPolicy: owners.NewParentGuardianPolicy(),

		Runtime:   api.ParentPortalRuntime{},
		ChildRepo: repos.ParentChildReads(), StudentRepo: owners.NewParentStudentReads(reposPeople.Student),
		RequestSharing: unusedRequestSharer{},

		DeparturePolicy: owners.ParentDeparturePolicy(),

		CarePlanPolicy: owners.NewParentCarePlanPolicy(),
	})
}

// unusedRequestSharer satisfies parentportal.NewChildFlows for the child resolution these tests
// need; child resolution never reaches the sharing ledger.
type unusedRequestSharer struct{}

func (unusedRequestSharer) ShareRequestInTx(context.Context, int64, int64, string, int64, []int64) error {
	panic("messaging tests: child resolution must not share requests")
}

func (unusedRequestSharer) LoadRequestShareVisibility(context.Context, int64) (parentportal.RequestShareVisibility, error) {
	panic("messaging tests: child resolution must not read request shares")
}

// messagingSettingsStub answers ResolveBoolForTenant / ResolveStringForTenant
// from maps; every other SettingsService method panics via the embedded nil
// interface. Test-only copy of the workflows/parentportal behaviour-test stub.
type messagingSettingsStub struct {
	parentportal.SettingsService
	boolValues   map[string]bool
	stringValues map[string]string
}

func (s messagingSettingsStub) ResolveBoolForTenant(_ context.Context, _ int64, key string) (bool, error) {
	if v, ok := s.boolValues[key]; ok {
		return v, nil
	}
	if key == parentportal.KeyParentSickReportsEnabled || key == parentportal.KeyParentExcusedReportsEnabled {
		return true, nil
	}
	return false, nil
}

func (s messagingSettingsStub) ResolveStringForTenant(_ context.Context, _ int64, key string) (string, error) {
	return s.stringValues[key], nil
}

func (s messagingSettingsStub) ResolveStringForTenantInTx(_ context.Context, _ int64, key string) (string, error) {
	if value, ok := s.boolValues[key]; ok {
		return strconv.FormatBool(value), nil
	}
	return s.stringValues[key], nil
}

// endCareFor ends the child's care the day before the fixed test day (test-only
// copy of the workflows/parentportal behaviour-test helper).
func endCareFor(t *testing.T, db *bun.DB, studentID int64) {
	t.Helper()
	_, err := db.NewUpdate().
		TableExpr("users.student_school_memberships").
		Set("enrolled_until = ?", calendar.NewDate(2026, 8, 24).AddDays(-1)).
		Where("student_profile_id = ?", studentID).Where("deleted_at IS NULL").
		Exec(testpkg.WithPackageTenantRuntime(context.Background()))
	require.NoError(t, err)
}

// revokeDeclarationPermission takes parent_portal.declarations.submit away
// from every relationship of the account, as a staff edit would (#3430).
func revokeDeclarationPermission(t *testing.T, db *bun.DB, accountID int64) {
	t.Helper()
	_, err := db.NewUpdate().TableExpr("auth.guardian_student_access").
		Set("permissions = permissions - 'parent_portal.declarations.submit'").
		Where("account_id = ?", accountID).
		Exec(testpkg.WithPackageTenantRuntime(context.Background()))
	require.NoError(t, err)
}

// moveDeclarationDeadline sets the deadline of a published Erklärung, which
// the product only does before publication; the test needs it to pass.
func moveDeclarationDeadline(t *testing.T, db *bun.DB, announcementID int64, deadline time.Time) {
	t.Helper()
	_, err := db.NewUpdate().TableExpr("users.parent_announcements").
		Set("response_deadline = ?", deadline).
		Where("id = ?", announcementID).
		Exec(testpkg.WithPackageTenantRuntime(context.Background()))
	require.NoError(t, err)
}
