// Package externalcaregiver records an external caregiver with the access
// level chosen for them (#4573, ADR 0091), and invites an existing external
// caregiver who has no account yet. Being external is the external caregiver
// record; the access is a separate choice: none (a placeholder without an
// account), view (an invitation into the system role guest „Gast“) or
// supervise (an invitation into the system role ag-leitung „AG-Leitung“).
// The writes span the People Directory (person), School Membership (staff,
// caregiver profile and external caregiver record) and Identity & Access
// (invitation).
//
// The workflow owns no data and issues no SQL. It declares the owner
// surfaces it coordinates as consumer-owned ports in the owners' public
// vocabulary, so the owners' public capabilities satisfy them directly
// (the care-exit shape). Every call joins the caller's tenant transaction.
package externalcaregiver

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
)

var (
	// ErrEmailRequired: the request names no e-mail address.
	ErrEmailRequired = errors.New("email is required")
	// ErrEmailInvalid: the e-mail address is not a plain address.
	ErrEmailInvalid = errors.New("email is invalid")
	// ErrAccessInvalid: the access level is missing or unknown, or names
	// no account where the call invites one.
	ErrAccessInvalid = errors.New("access is invalid")
	// ErrNotExternal: the staff member has no active external caregiver record.
	ErrNotExternal = errors.New("staff member has no active external caregiver record")
	// ErrHasAccount: the external caregiver already has an account.
	ErrHasAccount = errors.New("external caregiver already has an account")
	// ErrStaffNotFound: no live staff member of the school has the ID.
	ErrStaffNotFound = errors.New("staff member not found")
	// ErrEmailInvited: an open invitation of the school for the address
	// belongs to someone else. Inviting would spend it silently.
	ErrEmailInvited = errors.New("the address has an open invitation for someone else")
)

// Access is the access level of an external caregiver (ADR 0091).
type Access string

const (
	// AccessNone records a placeholder for the planning: no account, no
	// e-mail, no invitation.
	AccessNone Access = "none"
	// AccessView invites into the system role guest („Gast“, only views).
	AccessView Access = "view"
	// AccessSupervise invites into the system role ag-leitung
	// („AG-Leitung“, runs the own blocks).
	AccessSupervise Access = "supervise"
)

// roleName is the system role the access level invites into; "" for none.
func (a Access) roleName() string {
	switch a {
	case AccessView:
		return identityaccess.GuestRoleName
	case AccessSupervise:
		return identityaccess.AGLeitungRoleName
	default:
		return ""
	}
}

// parseAccess accepts the three access levels; invited additionally
// refuses none.
func parseAccess(raw Access, invited bool) (Access, error) {
	switch raw {
	case AccessView, AccessSupervise:
		return raw, nil
	case AccessNone:
		if !invited {
			return raw, nil
		}
	}
	return "", ErrAccessInvalid
}

// errSystemRoleMissing: the platform system role an access level invites
// into does not exist; migrations 1.0.4 and 1.15.478 create them, so this is
// a broken deployment, not a request error.
var errSystemRoleMissing = errors.New("external caregiver: system role missing")

// Expertise fills the required expertise of the external caregiver record: what the person
// does follows from the blocks they are planned for.
const Expertise = "Externe Betreuung"

// Actor is the account that records or invites, with its permissions; the
// role-grant check of the invitation compares them with the role's.
type Actor struct {
	AccountID   int64
	Permissions []string
}

// RecordInput records a new external caregiver. The names and the
// organization arrive trimmed and length-checked. Email is required for
// view and supervise and ignored for none.
type RecordInput struct {
	FirstName    string
	LastName     string
	Email        string
	Organization string
	Access       Access
	Actor        Actor
}

// InviteInput invites an existing external caregiver without an account;
// Access is view or supervise.
type InviteInput struct {
	StaffID int64
	Email   string
	Access  Access
	Actor   Actor
}

// Result names the external caregiver and the invitation. Invited is false
// for a placeholder (access none); the invitation fields are then empty.
type Result struct {
	StaffID             int64
	PersonID            int64
	Invited             bool
	InvitationExpiresAt time.Time
	// InvitationToken redeems the invitation. Only the local seeder may see
	// it (the HTTP adapter decides); it is never logged.
	InvitationToken string
}

// People creates and reads the person of an external caregiver.
type People interface {
	CreatePerson(context.Context, peopledirectory.CreatePerson) (peopledirectory.Person, error)
	// FindPerson reads a live person of the school in context.
	FindPerson(context.Context, int64) (peopledirectory.Person, error)
}

// Membership records the staff member and its caregiver profile and external caregiver record.
type Membership interface {
	CreateStaff(context.Context, schoolmembership.CreateStaff) (schoolmembership.Staff, error)
	CreateTeacher(context.Context, schoolmembership.CreateTeacher) (schoolmembership.Teacher, error)
	CreateExternalCaregiverRecord(context.Context, schoolmembership.CreateExternalCaregiverRecord) (schoolmembership.ExternalCaregiverRecord, error)
	FindTeacherByStaff(context.Context, int64) (schoolmembership.Teacher, error)
	// ListStaff reads live staff of the school in context; HasExternalCaregiverRecord
	// is the active external caregiver record.
	ListStaff(context.Context, schoolmembership.StaffFilter) ([]schoolmembership.Staff, error)
}

// Invitations resolves the system role an access level invites into and
// issues, lists and revokes the school's account invitations.
type Invitations interface {
	// ListRoles reads the roles visible to the school in context; the
	// workflow picks the platform system role by name.
	ListRoles(ctx context.Context, filter identityaccess.RoleFilter) ([]identityaccess.Role, error)
	CreateSchoolInvitation(ctx context.Context, request identityaccess.SchoolInvitationRequest) (identityaccess.SchoolInvitation, error)
	RevokeSchoolInvitation(ctx context.Context, invitationID, actorAccountID int64) error
	ListPendingSchoolInvitations(ctx context.Context) ([]identityaccess.SchoolInvitation, error)
}

// Savepoint runs fn inside a savepoint of the caller's transaction and
// rolls it back when fn fails.
type Savepoint func(ctx context.Context, fn func(context.Context) error) error

// Workflow coordinates the writes.
type Workflow struct {
	people      People
	membership  Membership
	invitations Invitations
	savepoint   Savepoint
}

// New builds the workflow; every port is required.
func New(people People, membership Membership, invitations Invitations, savepoint Savepoint) (*Workflow, error) {
	if people == nil || membership == nil || invitations == nil || savepoint == nil {
		return nil, errors.New("external caregiver: all dependencies required")
	}
	return &Workflow{people: people, membership: membership, invitations: invitations, savepoint: savepoint}, nil
}

// Record records person, staff member, caregiver profile and external
// caregiver record and, for view and supervise, invites the address into
// the access level's system role. Everything runs in one savepoint: the
// request transaction commits 4xx answers, so a refused invitation (address
// already at the school, role grant denied) must not leave a half-created
// person behind. An open invitation of the school for the address refuses
// before any write (ErrEmailInvited). A placeholder (none) needs no address
// and gets no invitation; the caregiver profile keeps it plannable.
func (w *Workflow) Record(ctx context.Context, in RecordInput) (Result, error) {
	access, err := parseAccess(in.Access, false)
	if err != nil {
		return Result{}, err
	}
	var email string
	var roleID int64
	if access != AccessNone {
		if email, err = normalizeEmail(in.Email); err != nil {
			return Result{}, err
		}
		if roleID, err = w.systemRole(ctx, access); err != nil {
			return Result{}, err
		}
		// The person is new, so every open invitation for the address
		// belongs to someone else.
		if _, err := w.pendingOf(ctx, email, 0); err != nil {
			return Result{}, err
		}
	}
	var result Result
	err = w.savepoint(ctx, func(ctx context.Context) error {
		var err error
		result, err = w.createRecords(ctx, in, email, roleID)
		return err
	})
	if err != nil {
		return Result{}, err
	}
	return result, nil
}

// createRecords writes the records and invites when roleID names a role.
func (w *Workflow) createRecords(ctx context.Context, in RecordInput, email string, roleID int64) (Result, error) {
	person, err := w.people.CreatePerson(ctx, peopledirectory.CreatePerson{FirstName: in.FirstName, LastName: in.LastName})
	if err != nil {
		return Result{}, err
	}
	staff, err := w.membership.CreateStaff(ctx, schoolmembership.CreateStaff{StaffFields: schoolmembership.StaffFields{PersonID: person.ID}})
	if err != nil {
		return Result{}, err
	}
	if err := w.ensureCaregiverProfile(ctx, staff.ID); err != nil {
		return Result{}, err
	}
	if _, err := w.membership.CreateExternalCaregiverRecord(ctx, schoolmembership.CreateExternalCaregiverRecord{ExternalCaregiverRecordFields: schoolmembership.ExternalCaregiverRecordFields{
		StaffID: staff.ID, Organization: in.Organization, ActivityExpertise: Expertise,
	}}); err != nil {
		return Result{}, err
	}
	if roleID == 0 {
		return Result{StaffID: staff.ID, PersonID: person.ID}, nil
	}
	return w.invite(ctx, staff.ID, person, email, roleID, in.Actor)
}

// Invite („Konto einladen“) invites an external caregiver without an account,
// also to send an open invitation again or to another address. Every open
// invitation of the person is revoked first: acceptance attaches the account
// to the person named in the first invitation redeemed, so a mistyped earlier
// address must not stay redeemable. An open invitation for the address that
// names someone else refuses before any write (ErrEmailInvited). The access
// is view or supervise; none invites nobody (ErrAccessInvalid).
func (w *Workflow) Invite(ctx context.Context, in InviteInput) (Result, error) {
	access, err := parseAccess(in.Access, true)
	if err != nil {
		return Result{}, err
	}
	email, err := normalizeEmail(in.Email)
	if err != nil {
		return Result{}, err
	}
	person, err := w.invitablePerson(ctx, in.StaffID)
	if err != nil {
		return Result{}, err
	}
	roleID, err := w.systemRole(ctx, access)
	if err != nil {
		return Result{}, err
	}
	own, err := w.pendingOf(ctx, email, person.ID)
	if err != nil {
		return Result{}, err
	}
	var result Result
	err = w.savepoint(ctx, func(ctx context.Context) error {
		var err error
		result, err = w.reinvite(ctx, in, person, own, email, roleID)
		return err
	})
	if err != nil {
		return Result{}, err
	}
	return result, nil
}

func (w *Workflow) reinvite(ctx context.Context, in InviteInput, person peopledirectory.Person, own []identityaccess.SchoolInvitation, email string, roleID int64) (Result, error) {
	// Externals recorded before #4573 have no caregiver profile yet.
	if err := w.ensureCaregiverProfile(ctx, in.StaffID); err != nil {
		return Result{}, err
	}
	for _, invitation := range own {
		if err := w.invitations.RevokeSchoolInvitation(ctx, invitation.ID, in.Actor.AccountID); err != nil {
			return Result{}, err
		}
	}
	return w.invite(ctx, in.StaffID, person, email, roleID, in.Actor)
}

// invitablePerson resolves the person of an external caregiver without an
// account.
func (w *Workflow) invitablePerson(ctx context.Context, staffID int64) (peopledirectory.Person, error) {
	if staffID <= 0 {
		return peopledirectory.Person{}, ErrStaffNotFound
	}
	staff, err := w.membership.ListStaff(ctx, schoolmembership.StaffFilter{IDs: []int64{staffID}, MembershipOnly: true})
	switch {
	case err != nil:
		return peopledirectory.Person{}, err
	case len(staff) == 0:
		return peopledirectory.Person{}, ErrStaffNotFound
	case !staff[0].HasExternalCaregiverRecord:
		return peopledirectory.Person{}, ErrNotExternal
	}
	person, err := w.people.FindPerson(ctx, staff[0].PersonID)
	switch {
	case errors.Is(err, peopledirectory.ErrPersonNotFound):
		return peopledirectory.Person{}, ErrStaffNotFound
	case err != nil:
		return peopledirectory.Person{}, err
	case person.AccountID != nil:
		return peopledirectory.Person{}, ErrHasAccount
	}
	return person, nil
}

// ensureCaregiverProfile creates the caregiver (teacher) profile unless the
// staff member has one. Every external has one: it keeps the person
// plannable whatever the access, and ag-leitung (base_role 'user') needs it.
func (w *Workflow) ensureCaregiverProfile(ctx context.Context, staffID int64) error {
	_, err := w.membership.FindTeacherByStaff(ctx, staffID)
	if !errors.Is(err, schoolmembership.ErrTeacherNotFound) {
		return err
	}
	_, err = w.membership.CreateTeacher(ctx, schoolmembership.CreateTeacher{TeacherFields: schoolmembership.TeacherFields{StaffID: staffID}})
	return err
}

// pendingOf returns the school's open invitations naming the person
// (personID 0 names nobody). It refuses with ErrEmailInvited when an open
// invitation for the address names anyone else: creating the new invitation
// spends every open invitation for the address, which would silently revoke,
// for example, a staff invitation the Leitung sent.
func (w *Workflow) pendingOf(ctx context.Context, email string, personID int64) ([]identityaccess.SchoolInvitation, error) {
	pending, err := w.invitations.ListPendingSchoolInvitations(ctx)
	if err != nil {
		return nil, err
	}
	for _, invitation := range pending {
		named := invitation.PersonID != nil && *invitation.PersonID == personID && personID > 0
		if !named && strings.EqualFold(strings.TrimSpace(invitation.Email), email) {
			return nil, ErrEmailInvited
		}
	}
	return identityaccess.PendingInvitationsByPerson(pending)[personID], nil
}

func (w *Workflow) invite(ctx context.Context, staffID int64, person peopledirectory.Person, email string, roleID int64, actor Actor) (Result, error) {
	personID := person.ID
	firstName, lastName := person.FirstName, person.LastName
	stored, err := w.invitations.CreateSchoolInvitation(ctx, identityaccess.SchoolInvitationRequest{
		Email: email, RoleID: roleID, PersonID: &personID,
		FirstName: &firstName, LastName: &lastName,
		CreatedBy: actor.AccountID, ActorPermissions: actor.Permissions,
	})
	if err != nil {
		return Result{}, err
	}
	return Result{StaffID: staffID, PersonID: personID, Invited: true, InvitationExpiresAt: stored.ExpiresAt, InvitationToken: stored.Token}, nil
}

// systemRole resolves the platform system role the access level invites
// into. A school's own role of the same name never matches.
func (w *Workflow) systemRole(ctx context.Context, access Access) (int64, error) {
	name := access.roleName()
	roles, err := w.invitations.ListRoles(ctx, identityaccess.RoleFilter{Name: name})
	if err != nil {
		return 0, err
	}
	role, found := identityaccess.PlatformSystemRole(roles, name)
	if !found {
		return 0, fmt.Errorf("%w: %s", errSystemRoleMissing, name)
	}
	return role.ID, nil
}

// normalizeEmail is the address in the form the invitation stores
// (Identity & Access's normalizer), with the workflow's refusals.
func normalizeEmail(raw string) (string, error) {
	email, err := identityaccess.NormalizeInvitationEmail(raw)
	switch {
	case errors.Is(err, identityaccess.ErrInvitationEmailRequired):
		return "", ErrEmailRequired
	case err != nil:
		return "", ErrEmailInvalid
	}
	return email, nil
}
