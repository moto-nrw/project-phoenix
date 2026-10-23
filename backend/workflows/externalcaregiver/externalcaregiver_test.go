package externalcaregiver_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/workflows/externalcaregiver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeOwners plays the three owners. It records every call in order and
// fails the step named in failAt.
type fakeOwners struct {
	calls      []string
	failAt     string
	roleID     int64
	noRole     bool
	hasTeacher bool
	staff      []schoolmembership.Staff
	person     *peopledirectory.Person
	pending    []identityaccess.SchoolInvitation
	invited    []identityaccess.SchoolInvitationRequest
	revoked    []int64
	record     schoolmembership.ExternalCaregiverRecordFields
	expires    time.Time
}

var errStep = errors.New("step failed")

func (f *fakeOwners) step(name string) error {
	f.calls = append(f.calls, name)
	if f.failAt == name {
		return errStep
	}
	return nil
}

func (f *fakeOwners) CreatePerson(_ context.Context, input peopledirectory.CreatePerson) (peopledirectory.Person, error) {
	return peopledirectory.Person{ID: 501, FirstName: input.FirstName, LastName: input.LastName}, f.step("create_person")
}

func (f *fakeOwners) FindPerson(context.Context, int64) (peopledirectory.Person, error) {
	if err := f.step("find_person"); err != nil {
		return peopledirectory.Person{}, err
	}
	if f.person == nil {
		return peopledirectory.Person{}, peopledirectory.ErrPersonNotFound
	}
	return *f.person, nil
}

func (f *fakeOwners) CreateStaff(_ context.Context, input schoolmembership.CreateStaff) (schoolmembership.Staff, error) {
	return schoolmembership.Staff{ID: 601, PersonID: input.PersonID}, f.step("create_staff")
}

func (f *fakeOwners) CreateTeacher(_ context.Context, input schoolmembership.CreateTeacher) (schoolmembership.Teacher, error) {
	return schoolmembership.Teacher{ID: 701, StaffID: input.StaffID}, f.step("create_teacher")
}

func (f *fakeOwners) CreateExternalCaregiverRecord(_ context.Context, input schoolmembership.CreateExternalCaregiverRecord) (schoolmembership.ExternalCaregiverRecord, error) {
	f.record = input.ExternalCaregiverRecordFields
	return schoolmembership.ExternalCaregiverRecord{}, f.step("create_external_caregiver_record")
}

func (f *fakeOwners) FindTeacherByStaff(context.Context, int64) (schoolmembership.Teacher, error) {
	if err := f.step("find_teacher"); err != nil {
		return schoolmembership.Teacher{}, err
	}
	if !f.hasTeacher {
		return schoolmembership.Teacher{}, schoolmembership.ErrTeacherNotFound
	}
	return schoolmembership.Teacher{ID: 701}, nil
}

func (f *fakeOwners) ListStaff(context.Context, schoolmembership.StaffFilter) ([]schoolmembership.Staff, error) {
	return f.staff, f.step("list_staff")
}

func (f *fakeOwners) FindExternalCaregiverRole(context.Context) (identityaccess.Role, bool, error) {
	return identityaccess.Role{ID: f.roleID}, !f.noRole, f.step("external_role")
}

func (f *fakeOwners) CreateSchoolInvitation(_ context.Context, request identityaccess.SchoolInvitationRequest) (identityaccess.SchoolInvitation, error) {
	f.invited = append(f.invited, request)
	return identityaccess.SchoolInvitation{ExpiresAt: f.expires, Token: "token"}, f.step("invite")
}

func (f *fakeOwners) RevokeSchoolInvitation(_ context.Context, invitationID, _ int64) error {
	f.revoked = append(f.revoked, invitationID)
	return f.step("revoke")
}

func (f *fakeOwners) ListPendingSchoolInvitations(context.Context) ([]identityaccess.SchoolInvitation, error) {
	return f.pending, f.step("pending_invitations")
}

// fakeSavepoint records the callback's outcome, which a savepoint rolls back
// on.
type fakeSavepoint struct {
	opened     int
	rolledBack error
}

func (s *fakeSavepoint) run(ctx context.Context, fn func(context.Context) error) error {
	s.opened++
	err := fn(ctx)
	s.rolledBack = err
	return err
}

func newWorkflow(t *testing.T, owners *fakeOwners) (*externalcaregiver.Workflow, *fakeSavepoint) {
	t.Helper()
	savepoint := &fakeSavepoint{}
	workflow, err := externalcaregiver.New(owners, owners, owners, savepoint.run)
	require.NoError(t, err)
	return workflow, savepoint
}

func createInput() externalcaregiver.RecordInput {
	return externalcaregiver.RecordInput{
		FirstName: "Jonas", LastName: "Becker", Email: "  Jonas@Example.TEST ", Organization: "Musikschule",
		Actor: externalcaregiver.Actor{AccountID: 7, Permissions: []string{"users:create", "schedules:read"}},
	}
}

func int64Ptr(value int64) *int64    { return &value }
func stringPtr(value string) *string { return &value }

func TestNewRequiresEveryPort(t *testing.T) {
	t.Parallel()
	owners := &fakeOwners{}
	_, err := externalcaregiver.New(owners, owners, owners, nil)
	require.Error(t, err)
}

func TestRecordWritesInOrderInsideOneSavepoint(t *testing.T) {
	t.Parallel()
	expires := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)
	owners := &fakeOwners{roleID: 31, expires: expires}
	workflow, savepoint := newWorkflow(t, owners)

	result, err := workflow.Record(context.Background(), createInput())
	require.NoError(t, err)

	assert.Equal(t, []string{"external_role", "pending_invitations", "create_person", "create_staff", "find_teacher", "create_teacher", "create_external_caregiver_record", "invite"}, owners.calls)
	assert.Equal(t, 1, savepoint.opened)
	assert.Equal(t, externalcaregiver.Result{StaffID: 601, PersonID: 501, InvitationExpiresAt: expires, InvitationToken: "token"}, result)
	assert.Equal(t, schoolmembership.ExternalCaregiverRecordFields{StaffID: 601, Organization: "Musikschule", ActivityExpertise: externalcaregiver.Expertise}, owners.record)
	require.Len(t, owners.invited, 1)
	assert.Equal(t, identityaccess.SchoolInvitationRequest{
		Email: "jonas@example.test", RoleID: 31, PersonID: int64Ptr(501),
		FirstName: stringPtr("Jonas"), LastName: stringPtr("Becker"),
		CreatedBy: 7, ActorPermissions: []string{"users:create", "schedules:read"},
	}, owners.invited[0])
}

func TestRecordRollsTheSavepointBackWhenAStepFails(t *testing.T) {
	t.Parallel()
	for _, step := range []string{"create_person", "create_staff", "find_teacher", "create_teacher", "create_external_caregiver_record", "invite"} {
		t.Run(step, func(t *testing.T) {
			t.Parallel()
			owners := &fakeOwners{roleID: 31, failAt: step}
			workflow, savepoint := newWorkflow(t, owners)

			_, err := workflow.Record(context.Background(), createInput())
			require.ErrorIs(t, err, errStep)
			assert.ErrorIs(t, savepoint.rolledBack, errStep, "the savepoint saw the failure and rolls back")
			assert.Equal(t, step, owners.calls[len(owners.calls)-1], "nothing runs after the failed step")
		})
	}
}

func TestRecordRefusesBeforeAnyWrite(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		email  string
		noRole bool
		want   error
	}{
		"empty email":        {email: "   ", want: externalcaregiver.ErrEmailRequired},
		"not an address":     {email: "jonas", want: externalcaregiver.ErrEmailInvalid},
		"display name form":  {email: "Jonas <jonas@example.test>", want: externalcaregiver.ErrEmailInvalid},
		"address too long":   {email: strings.Repeat("a", 250) + "@x.de", want: externalcaregiver.ErrEmailInvalid},
		"school has no role": {email: "jonas@example.test", noRole: true, want: externalcaregiver.ErrRoleMissing},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			owners := &fakeOwners{noRole: tc.noRole}
			workflow, savepoint := newWorkflow(t, owners)
			input := createInput()
			input.Email = tc.email

			_, err := workflow.Record(context.Background(), input)
			require.ErrorIs(t, err, tc.want)
			assert.Zero(t, savepoint.opened)
			assert.NotContains(t, owners.calls, "create_person")
		})
	}
}

func inviteOwners() *fakeOwners {
	return &fakeOwners{
		roleID: 31,
		staff:  []schoolmembership.Staff{{ID: 601, PersonID: 501, HasExternalCaregiverRecord: true}},
		person: &peopledirectory.Person{ID: 501, FirstName: "Selin", LastName: "Aydın"},
		pending: []identityaccess.SchoolInvitation{
			{ID: 91, PersonID: int64Ptr(501), Email: "selin@example.test"},
			{ID: 92},
			{ID: 93, PersonID: int64Ptr(502)},
			{ID: 94, PersonID: int64Ptr(501)},
		},
	}
}

func TestInviteRevokesThePersonsOpenInvitationsBeforeInviting(t *testing.T) {
	t.Parallel()
	owners := inviteOwners()
	workflow, savepoint := newWorkflow(t, owners)

	result, err := workflow.Invite(context.Background(), externalcaregiver.InviteInput{
		StaffID: 601, Email: "selin@example.test", Actor: externalcaregiver.Actor{AccountID: 7},
	})
	require.NoError(t, err)

	assert.Equal(t, []string{
		"list_staff", "find_person", "external_role", "pending_invitations",
		"find_teacher", "create_teacher", "revoke", "revoke", "invite",
	}, owners.calls)
	assert.Equal(t, []int64{91, 94}, owners.revoked, "only the person's own invitations are spent")
	assert.Equal(t, 1, savepoint.opened)
	require.Len(t, owners.invited, 1)
	assert.Equal(t, identityaccess.SchoolInvitationRequest{
		Email: "selin@example.test", RoleID: 31, PersonID: int64Ptr(501),
		FirstName: stringPtr("Selin"), LastName: stringPtr("Aydın"), CreatedBy: 7,
	}, owners.invited[0])
	assert.Equal(t, int64(601), result.StaffID)
	assert.Equal(t, int64(501), result.PersonID)
}

func TestInviteKeepsAnExistingCaregiverProfile(t *testing.T) {
	t.Parallel()
	owners := inviteOwners()
	owners.hasTeacher = true
	workflow, _ := newWorkflow(t, owners)

	_, err := workflow.Invite(context.Background(), externalcaregiver.InviteInput{StaffID: 601, Email: "selin@example.test"})
	require.NoError(t, err)
	assert.NotContains(t, owners.calls, "create_teacher")
}

func TestInviteRollsBackWhenRevokingFails(t *testing.T) {
	t.Parallel()
	owners := inviteOwners()
	owners.failAt = "revoke"
	workflow, savepoint := newWorkflow(t, owners)

	_, err := workflow.Invite(context.Background(), externalcaregiver.InviteInput{StaffID: 601, Email: "selin@example.test"})
	require.ErrorIs(t, err, errStep)
	assert.ErrorIs(t, savepoint.rolledBack, errStep)
	assert.Empty(t, owners.invited)
}

func TestInviteRefusesWhoCannotBeInvited(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		edit func(*fakeOwners)
		want error
	}{
		"unknown or foreign staff":            {edit: func(f *fakeOwners) { f.staff = nil }, want: externalcaregiver.ErrStaffNotFound},
		"no active external caregiver record": {edit: func(f *fakeOwners) { f.staff[0].HasExternalCaregiverRecord = false }, want: externalcaregiver.ErrNotExternal},
		"person gone":                         {edit: func(f *fakeOwners) { f.person = nil }, want: externalcaregiver.ErrStaffNotFound},
		"already has an account":              {edit: func(f *fakeOwners) { f.person.AccountID = int64Ptr(44) }, want: externalcaregiver.ErrHasAccount},
		"school has no role":                  {edit: func(f *fakeOwners) { f.noRole = true }, want: externalcaregiver.ErrRoleMissing},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			owners := inviteOwners()
			tc.edit(owners)
			workflow, savepoint := newWorkflow(t, owners)

			_, err := workflow.Invite(context.Background(), externalcaregiver.InviteInput{StaffID: 601, Email: "selin@example.test"})
			require.ErrorIs(t, err, tc.want)
			assert.Zero(t, savepoint.opened)
			assert.Empty(t, owners.revoked)
			assert.Empty(t, owners.invited)
		})
	}
}

func TestInviteRequiresAnAddress(t *testing.T) {
	t.Parallel()
	owners := inviteOwners()
	workflow, _ := newWorkflow(t, owners)

	_, err := workflow.Invite(context.Background(), externalcaregiver.InviteInput{StaffID: 601})
	require.ErrorIs(t, err, externalcaregiver.ErrEmailRequired)
	assert.Empty(t, owners.calls)
}

// TestRecordRefusesAnAddressWithAnOpenInvitation: creating the invitation
// spends every open invitation for the address, so one sent to someone else
// (a staff invitation of the Leitung, another external) refuses the record
// before any write (#4573).
func TestRecordRefusesAnAddressWithAnOpenInvitation(t *testing.T) {
	t.Parallel()
	for name, pending := range map[string]identityaccess.SchoolInvitation{
		"staff invitation": {ID: 91, Email: "jonas@example.test"},
		"another external": {ID: 92, Email: "Jonas@Example.test", PersonID: int64Ptr(502)},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			owners := &fakeOwners{roleID: 31, pending: []identityaccess.SchoolInvitation{pending}}
			workflow, savepoint := newWorkflow(t, owners)

			_, err := workflow.Record(context.Background(), createInput())
			require.ErrorIs(t, err, externalcaregiver.ErrEmailInvited)
			assert.Zero(t, savepoint.opened)
			assert.NotContains(t, owners.calls, "create_person")
			assert.Empty(t, owners.invited)
		})
	}
}

// TestInviteRefusesAnAddressInvitedForSomeoneElse: the person's own open
// invitation for the address is sent again; one naming someone else or
// nobody refuses before any write.
func TestInviteRefusesAnAddressInvitedForSomeoneElse(t *testing.T) {
	t.Parallel()
	for name, pending := range map[string]identityaccess.SchoolInvitation{
		"staff invitation": {ID: 95, Email: "selin@example.test"},
		"another external": {ID: 96, Email: "selin@example.test", PersonID: int64Ptr(502)},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			owners := inviteOwners()
			owners.pending = append(owners.pending, pending)
			workflow, savepoint := newWorkflow(t, owners)

			_, err := workflow.Invite(context.Background(), externalcaregiver.InviteInput{StaffID: 601, Email: "selin@example.test"})
			require.ErrorIs(t, err, externalcaregiver.ErrEmailInvited)
			assert.Zero(t, savepoint.opened)
			assert.Empty(t, owners.revoked)
			assert.Empty(t, owners.invited)
		})
	}
}
