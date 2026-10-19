package owners

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
)

// The lifecycle and the guardian invitation seams refuse to compose without
// their dependencies (ADR 0059): a missing one is a construction error, not
// a failure or a silent fallback of the first request.
func TestLifecycleDependenciesRequireEveryDependency(t *testing.T) {
	t.Parallel()

	_, err := LifecycleDependencies(LifecycleRepositories{}, nil, CaregiverProfiles{}, UnsentGuardianMail{}, nil, slog.Default())
	require.EqualError(t, err, "identity access composition: every lifecycle repository and owner command, the audit command and the caregiver profiles are required")
}

func TestGuardianInvitationDependenciesRequireEveryDependency(t *testing.T) {
	t.Parallel()

	_, _, err := GuardianInvitationDependencies(nil, nil, nil, nil, "http://localhost:3000", time.Hour, slog.Default())
	require.EqualError(t, err, "identity access composition: the guardian invitation seams need the school directory, the e-mail outbox and the enrollment claim")

	_, _, err = GuardianInvitationDependencies(nil, struct{ organizationtenancy.Query }{}, struct{ MailResultOutbox }{}, struct {
		GuardianEnrollmentClaims
	}{}, "http://localhost:3000", time.Hour, nil)
	require.EqualError(t, err, "identity access composition: the guardian invitation seams need a logger")
}

func TestStaffDirectoryFindStaffByPersonPrefersLatestRetiredMembership(t *testing.T) {
	t.Parallel()

	retiredAt := time.Time{}
	directory := staffDirectory{repos: LifecycleRepositories{Membership: lifecycleMembershipStub{staff: []schoolmembership.Staff{
		{ID: 101, TenantID: 1, PersonID: 42, DeletedAt: &retiredAt},
		{ID: 202, TenantID: 1, PersonID: 42, DeletedAt: &retiredAt},
	}}}}

	staff, found, err := directory.FindStaffByPerson(context.Background(), 42)

	require.NoError(t, err)
	require.True(t, found)
	require.True(t, staff.Deleted)
	require.Equal(t, int64(202), staff.ID, "the newest retired membership continues on rehire")
}

type lifecycleMembershipStub struct{ staff []schoolmembership.Staff }

func (s lifecycleMembershipStub) ListStaff(context.Context, schoolmembership.StaffFilter) ([]schoolmembership.Staff, error) {
	return s.staff, nil
}

func (lifecycleMembershipStub) CreateStaff(context.Context, schoolmembership.CreateStaff) (schoolmembership.Staff, error) {
	return schoolmembership.Staff{}, nil
}

func (lifecycleMembershipStub) CreateTeacher(context.Context, schoolmembership.CreateTeacher) (schoolmembership.Teacher, error) {
	return schoolmembership.Teacher{}, nil
}
