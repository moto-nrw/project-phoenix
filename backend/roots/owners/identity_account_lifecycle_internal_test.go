package owners

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
)

// The lifecycle and the guardian invitation seams refuse to compose without
// their dependencies (ADR 0059): a missing one is a construction error, not
// a failure or a silent fallback of the first request.
func TestLifecycleDependenciesRequireEveryDependency(t *testing.T) {
	t.Parallel()

	_, err := LifecycleDependencies(LifecycleRepositories{}, nil, CaregiverProfiles{}, UnsentGuardianMail{}, nil, nil, slog.Default())
	require.EqualError(t, err, "identity access composition: every lifecycle repository and owner command, the audit command and the caregiver profiles are required")
}

func TestGuardianInvitationDependenciesRequireEveryDependency(t *testing.T) {
	t.Parallel()

	_, _, err := GuardianInvitationDependencies(nil, nil, nil, nil, "http://localhost:3000", slog.Default())
	require.EqualError(t, err, "identity access composition: the guardian invitation seams need the school directory, the e-mail outbox and the enrollment claim")

	_, _, err = GuardianInvitationDependencies(nil, struct{ organizationtenancy.Query }{}, struct{ MailResultOutbox }{}, struct {
		GuardianEnrollmentClaims
	}{}, "http://localhost:3000", nil)
	require.EqualError(t, err, "identity access composition: the guardian invitation seams need a logger")
}

func TestStaffDirectoryFindStaffByPersonPrefersLatestRetiredMembership(t *testing.T) {
	t.Parallel()

	retiredAt := time.Time{}
	directory := staffDirectory{repos: LifecycleRepositories{Membership: &lifecycleMembershipStub{staff: []schoolmembership.Staff{
		{ID: 101, PersonID: 42, DeletedAt: &retiredAt},
		{ID: 202, PersonID: 42, DeletedAt: &retiredAt},
	}}}}

	staff, found, err := directory.FindStaffByPerson(context.Background(), 42)

	require.NoError(t, err)
	require.True(t, found)
	require.True(t, staff.Deleted)
	require.Equal(t, int64(202), staff.ID, "the newest retired membership continues on rehire")
}

// A rehire continues the retired membership under its own ID (ADR 0021,
// #3401) instead of creating a second one, and locks the person after it.
func TestStaffDirectoryCreateStaffReinstatesRetiredMembership(t *testing.T) {
	t.Parallel()

	people := &lockingPeople{}
	reinstatement := &reinstatementStub{personID: 42}
	membership := &lifecycleMembershipStub{}
	directory := staffDirectory{repos: LifecycleRepositories{Membership: membership, Reinstatement: reinstatement, People: people}}
	retiredID := int64(7)

	id, err := directory.CreateStaff(context.Background(), 1, 42, &retiredID)

	require.NoError(t, err)
	require.Equal(t, retiredID, id)
	require.Equal(t, []int64{7}, reinstatement.staff)
	require.Zero(t, membership.created, "no second membership")
	require.Equal(t, []int64{42}, people.locked)
}

// A membership the automatic deletion of staff erased cannot be continued; the
// returning person starts a new one. The person is locked on the refusal too.
func TestStaffDirectoryCreateStaffReplacesErasedMembership(t *testing.T) {
	t.Parallel()

	people := &lockingPeople{}
	membership := &lifecycleMembershipStub{}
	directory := staffDirectory{repos: LifecycleRepositories{
		Membership: membership, Reinstatement: &reinstatementStub{staffErr: schoolmembership.ErrStaffNotFound}, People: people,
	}}
	retiredID := int64(7)

	_, err := directory.CreateStaff(context.Background(), 1, 42, &retiredID)

	require.NoError(t, err)
	require.Equal(t, 1, membership.created)
	require.Equal(t, []int64{42}, people.locked)
}

// The deletion may anonymize the person between the identity path's read and
// the reinstatement; neither the reinstated nor a new membership may then
// land on the anonymized person.
func TestStaffDirectoryCreateStaffRefusesAnonymizedPerson(t *testing.T) {
	t.Parallel()

	anonymizedAt := time.Time{}
	for _, staffErr := range []error{nil, schoolmembership.ErrStaffNotFound} {
		membership := &lifecycleMembershipStub{}
		directory := staffDirectory{repos: LifecycleRepositories{
			Membership: membership, Reinstatement: &reinstatementStub{personID: 42, staffErr: staffErr},
			People: &lockingPeople{deletedAt: &anonymizedAt},
		}}
		retiredID := int64(7)
		_, err := directory.CreateStaff(context.Background(), 1, 42, &retiredID)
		require.ErrorIs(t, err, errReinstatedPersonAnonymized)
		require.Zero(t, membership.created)
	}
}

func TestStaffDirectoryCreateStaffSurfacesReinstatementFailure(t *testing.T) {
	t.Parallel()

	failure := errors.New("database down")
	membership := &lifecycleMembershipStub{}
	directory := staffDirectory{repos: LifecycleRepositories{
		Membership: membership, Reinstatement: &reinstatementStub{staffErr: failure}, People: &lockingPeople{},
	}}
	retiredID := int64(7)
	_, err := directory.CreateStaff(context.Background(), 1, 42, &retiredID)
	require.ErrorIs(t, err, failure)
	require.Zero(t, membership.created, "a failed reinstatement never falls back to a second membership")
}

// The caregiver profile is part of the chain: the newest retired one is
// revived instead of a second profile appearing next to it.
func TestStaffDirectoryCreateCaregiverProfileRevivesRetired(t *testing.T) {
	t.Parallel()

	retiredAt := time.Time{}
	reinstatement := &reinstatementStub{}
	membership := &lifecycleMembershipStub{teachers: []schoolmembership.Teacher{
		{ID: 11, StaffID: 7, DeletedAt: &retiredAt},
		{ID: 12, StaffID: 7, DeletedAt: &retiredAt},
	}}
	directory := staffDirectory{repos: LifecycleRepositories{Membership: membership, Reinstatement: reinstatement}}

	id, err := directory.CreateCaregiverProfile(context.Background(), 1, 7, "Betreuung")

	require.NoError(t, err)
	require.Equal(t, int64(12), id)
	require.Equal(t, []int64{12}, reinstatement.teachers)
	require.Zero(t, membership.createdTeachers)
}

func TestStaffDirectoryCreateCaregiverProfileCreatesWithoutRetired(t *testing.T) {
	t.Parallel()

	reinstatement := &reinstatementStub{}
	membership := &lifecycleMembershipStub{}
	directory := staffDirectory{repos: LifecycleRepositories{Membership: membership, Reinstatement: reinstatement}}

	_, err := directory.CreateCaregiverProfile(context.Background(), 1, 7, "Betreuung")

	require.NoError(t, err)
	require.Equal(t, 1, membership.createdTeachers)
	require.Empty(t, reinstatement.teachers)
}

type lockingPeople struct {
	lifecyclePeople
	deletedAt *time.Time
	locked    []int64
}

func (p *lockingPeople) FindPersonForMutation(_ context.Context, id int64) (peopledirectory.Person, error) {
	p.locked = append(p.locked, id)
	return peopledirectory.Person{ID: id, DeletedAt: p.deletedAt}, nil
}

type reinstatementStub struct {
	personID int64
	staffErr error
	staff    []int64
	teachers []int64
}

func (r *reinstatementStub) ReinstateStaff(_ context.Context, id int64) (schoolmembership.Staff, error) {
	r.staff = append(r.staff, id)
	if r.staffErr != nil {
		return schoolmembership.Staff{}, r.staffErr
	}
	return schoolmembership.Staff{ID: id, PersonID: r.personID}, nil
}

func (r *reinstatementStub) ReinstateTeacher(_ context.Context, id int64) (schoolmembership.Teacher, error) {
	r.teachers = append(r.teachers, id)
	return schoolmembership.Teacher{ID: id}, nil
}

type lifecycleMembershipStub struct {
	staff           []schoolmembership.Staff
	teachers        []schoolmembership.Teacher
	created         int
	createdTeachers int
}

func (s *lifecycleMembershipStub) ListStaff(context.Context, schoolmembership.StaffFilter) ([]schoolmembership.Staff, error) {
	return s.staff, nil
}

func (s *lifecycleMembershipStub) CreateStaff(context.Context, schoolmembership.CreateStaff) (schoolmembership.Staff, error) {
	s.created++
	return schoolmembership.Staff{ID: 99}, nil
}

func (s *lifecycleMembershipStub) ListTeachers(context.Context, schoolmembership.TeacherFilter) ([]schoolmembership.Teacher, error) {
	return s.teachers, nil
}

func (s *lifecycleMembershipStub) CreateTeacher(context.Context, schoolmembership.CreateTeacher) (schoolmembership.Teacher, error) {
	s.createdTeachers++
	return schoolmembership.Teacher{ID: 98}, nil
}

type guardianExpirySettings struct {
	fakeWelcomeSettings
	hours int
	err   error
}

func (s guardianExpirySettings) ResolveInt(context.Context, string) (int, error) {
	return s.hours, s.err
}

func TestGuardianInvitationExpiryPropagatesSettingsFailures(t *testing.T) {
	t.Parallel()
	lookupErr := errors.New("settings lookup failed")
	for _, tc := range []struct {
		name  string
		hours int
		err   error
	}{
		{name: "resolved registry default", hours: 48},
		{name: "school override", hours: 12},
		{name: "read failure", err: lookupErr},
		{name: "invalid lifetime"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			delivery := guardianInvitationDelivery{settings: guardianExpirySettings{hours: tc.hours, err: tc.err}}
			expiry, err := delivery.InvitationExpiry(context.Background())
			if tc.err != nil {
				require.ErrorIs(t, err, lookupErr)
				require.Zero(t, expiry)
			} else if tc.hours == 0 {
				require.Error(t, err)
				require.Zero(t, expiry)
			} else {
				require.NoError(t, err)
				require.Equal(t, time.Duration(tc.hours)*time.Hour, expiry)
			}
		})
	}
}
