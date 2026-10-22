package api

import (
	"context"
	"errors"
	"log/slog"
	"time"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/workflows/staffoffboarding"
	offboardingcompose "github.com/moto-nrw/project-phoenix/workflows/staffoffboarding/compose"
)

// The School Membership HTTP adapter (#2667) owns the /api/staff directory
// routes but may only depend on the capability itself. Everything it still
// needs from the legacy services is composed here, with plain types, so the
// HTTP root can bind the adapter without importing any service package.

// StaffDirectoryPerson is the People Directory entry behind a staff row.
type StaffDirectoryPerson struct {
	ID        int64
	FirstName string
	LastName  string
	TagID     string
	AccountID *int64
	CreatedAt time.Time
	UpdatedAt time.Time
}

// StaffRoleRow is one staff member selected by account role.
type StaffRoleRow struct {
	StaffID           int64
	PersonID          int64
	TeacherID         int64
	FirstName         string
	LastName          string
	AccountID         int64
	Email             string
	IsActiveCaregiver bool
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// StaffGroup is one group a Betreuungskraft is assigned to.
type StaffGroup struct {
	ID   int64
	Name string
}

type StaffCreateInput struct {
	PersonID         int64
	StaffNotes       string
	IsTeacher        bool
	Specialization   string
	Role             string
	Qualifications   string
	ActorPermissions []string
}

type StaffCreateResult struct {
	Staff                 schoolmembership.Staff
	Teacher               *schoolmembership.Teacher
	TeacherCreationFailed bool
}

type StaffUpdateInput struct {
	StaffID        int64
	PersonID       int64
	StaffNotes     string
	IsTeacher      bool
	Specialization string
	Role           string
	Qualifications string
}

// StaffTeacherAction mirrors peoplePorts.TeacherAction as a stable string.
type StaffTeacherAction string

const (
	StaffTeacherActionNone         StaffTeacherAction = "none"
	StaffTeacherActionExisting     StaffTeacherAction = "existing"
	StaffTeacherActionUpdated      StaffTeacherAction = "updated"
	StaffTeacherActionUpdateFailed StaffTeacherAction = "update_failed"
	StaffTeacherActionCreated      StaffTeacherAction = "created"
	StaffTeacherActionCreateFailed StaffTeacherAction = "create_failed"
)

type StaffUpdateResult struct {
	Staff   schoolmembership.Staff
	Teacher *schoolmembership.Teacher
	Action  StaffTeacherAction
}

// StaffFailureKind classifies a service error for the HTTP renderer without
// exposing the sentinel itself.
type StaffFailureKind string

const (
	StaffFailureInvalidRequest StaffFailureKind = "invalid_request"
	StaffFailureUnauthorized   StaffFailureKind = "unauthorized"
	StaffFailureForbidden      StaffFailureKind = "forbidden"
	StaffFailureNotFound       StaffFailureKind = "not_found"
	StaffFailureConflict       StaffFailureKind = "conflict"
	StaffFailureInternal       StaffFailureKind = "internal"
)

// StaffMembershipHooks are the workforce callbacks the offboarding flow needs
// from the time-tracking admin resource: who acts, and which document files
// to remove after the commit.
type StaffMembershipHooks struct {
	ResolveEditorStaffID           func(context.Context) (int64, error)
	QueueOffboardedDocumentCleanup func(context.Context, int64) error
}

type staffOffboardingActorNameKey struct{}

// StaffMembershipRuntime is the legacy-service side of the School Membership
// HTTP adapter. Every closure keeps the exact semantics of the handler code
// it replaced in api/staff.
type StaffMembershipRuntime struct {
	Person  func(context.Context, int64) (StaffDirectoryPerson, error)
	Persons func(context.Context, []int64) ([]StaffDirectoryPerson, error)

	PresentStaffIDs     func(context.Context) ([]int64, error)
	WorkStatusMap       func(context.Context) (map[int64]string, error)
	AbsenceMap          func(context.Context) (map[int64]string, error)
	AbsenceLabelMap     func(context.Context) (map[int64]string, error)
	AccountRoles        func(context.Context, []int64) (map[int64]string, error)
	AccountEmails       func(context.Context, []int64) (map[int64]string, error)
	AccountAvatars      func(context.Context, []int64) (map[int64]string, error)
	AccountHasRole      func(context.Context, int64, string) bool
	AccountsHoldingRole func(context.Context, []int64, string) (map[int64]bool, error)

	TeacherGroups    func(context.Context, int64) ([]StaffGroup, error)
	SchoolClasses    func(context.Context, int64) ([]string, error)
	SetSchoolClasses func(context.Context, int64, []string, int64) error
	ActiveCaregivers func(context.Context) ([]StaffRoleRow, error)
	StaffByRoles     func(context.Context, []string) ([]StaffRoleRow, error)

	CreateStaff func(context.Context, StaffCreateInput) (StaffCreateResult, error)
	UpdateStaff func(context.Context, StaffUpdateInput) (StaffUpdateResult, error)
	Offboard    func(context.Context, int64, string) error
}

// bindStaffMembershipRuntime composes the closures over the legacy service
// graph and the Identity & Access module the root composed (#4006).
// offboarding carries the database and the employment owner the caller
// composed; the access, the cleanup, the broadcaster, the logger and the
// authorization of the offboarding workflow are bound here.
func bindStaffMembershipRuntime(users peoplePorts.PersonDirectory, education schoolstructure.GroupManagement, staffDay workforceCompose.StaffDayStatus, hub *realtime.Hub, offboarding offboardingcompose.Dependencies, identity identityaccess.Owner, logger *slog.Logger, hooks StaffMembershipHooks) StaffMembershipRuntime {
	if hooks.ResolveEditorStaffID == nil || hooks.QueueOffboardedDocumentCleanup == nil {
		panic("staff membership runtime: offboarding hooks are required")
	}
	if logger == nil {
		logger = slog.Default()
	}
	if identity == nil {
		panic("staff membership runtime: identity offboarding and role administration are required")
	}
	workflow := newStaffOffboardingWorkflow(hub, identity, offboarding, logger, hooks)
	person, persons := staffDirectoryPersons(users)
	activeCaregivers, staffByRoles := membershipRoleRows(users)
	createStaff, updateStaff := staffDirectoryWrites(users)
	return StaffMembershipRuntime{
		Person: person, Persons: persons,

		PresentStaffIDs: staffDay.GetStaffIDsWithSupervisionToday,
		WorkStatusMap:   staffDay.GetTodayPresenceMap,
		AbsenceMap:      staffDay.GetTodayAbsenceMap,
		AbsenceLabelMap: staffDay.GetTodayAbsenceLabelMap,
		AccountRoles:    identity.GetAccountRoleNames,
		AccountEmails:   identity.GetAccountEmails,
		AccountAvatars:  identity.GetAccountAvatars,
		AccountHasRole:  identity.AccountHoldsRole,
		AccountsHoldingRole: func(ctx context.Context, accountIDs []int64, role string) (map[int64]bool, error) {
			matches, err := identity.AccountsHoldingRole(ctx, accountIDs, role)
			if err != nil {
				return nil, err
			}
			return matches, nil
		},

		TeacherGroups:    membershipTeacherGroups(education),
		SchoolClasses:    education.GetStaffSchoolClasses,
		SetSchoolClasses: education.SetStaffSchoolClasses,
		ActiveCaregivers: activeCaregivers,
		StaffByRoles:     staffByRoles,

		CreateStaff: createStaff,
		UpdateStaff: updateStaff,
		Offboard: func(ctx context.Context, staffID int64, actorUsername string) error {
			_, err := workflow.Offboard(context.WithValue(ctx, staffOffboardingActorNameKey{}, actorUsername), staffID)
			if errors.Is(err, staffoffboarding.ErrInUse) {
				return peoplePorts.ErrStaffInUse
			}
			return err
		},
	}
}

// newStaffOffboardingWorkflow composes the offboarding workflow with the
// Identity & Access step, the document cleanup, the realtime hub, the logger
// and the editor authorization.
func newStaffOffboardingWorkflow(hub *realtime.Hub, identity identityaccess.Owner, offboarding offboardingcompose.Dependencies, logger *slog.Logger, hooks StaffMembershipHooks) *staffoffboarding.Workflow {
	offboarding.Access, offboarding.Cleanup = StaffOffboardingAccess(identity), hooks.QueueOffboardedDocumentCleanup
	offboarding.Broadcaster, offboarding.Logger = hub, logger
	offboarding.Authorize = func(ctx context.Context) (staffoffboarding.Actor, error) {
		name, _ := ctx.Value(staffOffboardingActorNameKey{}).(string)
		return owners.AuthorizeStaffOffboarding(ctx, hooks.ResolveEditorStaffID, name)
	}
	workflow, err := NewStaffOffboardingWorkflow(offboarding)
	if err != nil {
		panic(err)
	}
	return workflow
}

// staffDirectoryPersons reads the People Directory entries behind staff rows.
func staffDirectoryPersons(users peoplePorts.PersonDirectory) (func(context.Context, int64) (StaffDirectoryPerson, error), func(context.Context, []int64) ([]StaffDirectoryPerson, error)) {
	person := func(ctx context.Context, id int64) (StaffDirectoryPerson, error) {
		person, err := users.Get(ctx, id)
		if err != nil {
			return StaffDirectoryPerson{}, err
		}
		return toStaffDirectoryPerson(person.ID, person.FirstName, person.LastName, person.TagID, person.AccountID, person.CreatedAt, person.UpdatedAt), nil
	}
	persons := func(ctx context.Context, ids []int64) ([]StaffDirectoryPerson, error) {
		persons, err := users.GetByIDs(ctx, ids)
		if err != nil {
			return nil, err
		}
		result := make([]StaffDirectoryPerson, 0, len(persons))
		for _, person := range persons {
			result = append(result, toStaffDirectoryPerson(person.ID, person.FirstName, person.LastName, person.TagID, person.AccountID, person.CreatedAt, person.UpdatedAt))
		}
		return result, nil
	}
	return person, persons
}

// membershipTeacherGroups lists the groups a Betreuungskraft is assigned to.
func membershipTeacherGroups(education schoolstructure.GroupManagement) func(context.Context, int64) ([]StaffGroup, error) {
	return func(ctx context.Context, teacherID int64) ([]StaffGroup, error) {
		groups, err := education.GetTeacherGroups(ctx, teacherID)
		if err != nil {
			return nil, err
		}
		result := make([]StaffGroup, 0, len(groups))
		for _, group := range groups {
			result = append(result, StaffGroup{ID: group.ID, Name: group.Name})
		}
		return result, nil
	}
}

// membershipRoleRows lists the active caregivers and the staff selected by role.
func membershipRoleRows(users peoplePorts.PersonDirectory) (func(context.Context) ([]StaffRoleRow, error), func(context.Context, []string) ([]StaffRoleRow, error)) {
	activeCaregivers := func(ctx context.Context) ([]StaffRoleRow, error) {
		caregivers, err := users.ListActiveCaregivers(ctx)
		if err != nil {
			return nil, err
		}
		result := make([]StaffRoleRow, 0, len(caregivers))
		for _, caregiver := range caregivers {
			result = append(result, StaffRoleRow{
				StaffID: caregiver.StaffID, PersonID: caregiver.PersonID, TeacherID: caregiver.TeacherID,
				FirstName: caregiver.FirstName, LastName: caregiver.LastName,
				AccountID: caregiver.AccountID, Email: caregiver.Email, IsActiveCaregiver: true,
				CreatedAt: caregiver.CreatedAt, UpdatedAt: caregiver.UpdatedAt,
			})
		}
		return result, nil
	}
	staffByRoles := func(ctx context.Context, roles []string) ([]StaffRoleRow, error) {
		rows, err := users.StaffRows().ListStaffByRoles(ctx, roles)
		if err != nil {
			return nil, err
		}
		// A nil result stays nil on purpose: the endpoint historically
		// answered "data":null for an empty role match.
		var result []StaffRoleRow
		for _, row := range rows {
			result = append(result, StaffRoleRow{
				StaffID: row.StaffID, PersonID: row.PersonID, FirstName: row.FirstName, LastName: row.LastName,
				AccountID: row.AccountID, Email: row.Email, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
			})
		}
		return result, nil
	}
	return activeCaregivers, staffByRoles
}

// staffDirectoryWrites creates and updates staff records with their teacher
// profile through the person service.
func staffDirectoryWrites(users peoplePorts.PersonDirectory) (func(context.Context, StaffCreateInput) (StaffCreateResult, error), func(context.Context, StaffUpdateInput) (StaffUpdateResult, error)) {
	createStaff := func(ctx context.Context, input StaffCreateInput) (StaffCreateResult, error) {
		staff, teacher, teacherCreationFailed, err := users.CreateStaffWithTeacher(ctx, StaffDirectoryCreateInput(input))
		if err != nil {
			return StaffCreateResult{}, err
		}
		return StaffCreateResult{Staff: legacyStaffToMembership(staff), Teacher: legacyTeacherToMembership(teacher), TeacherCreationFailed: teacherCreationFailed}, nil
	}
	updateStaff := func(ctx context.Context, input StaffUpdateInput) (StaffUpdateResult, error) {
		staff, err := users.StaffRows().FindByID(ctx, input.StaffID)
		if err != nil {
			return StaffUpdateResult{}, err
		}
		staff.StaffNotes = input.StaffNotes
		staff.PersonID = input.PersonID
		teacher, action, err := users.UpdateStaffWithTeacher(ctx, staff, input.IsTeacher, input.Specialization, input.Role, input.Qualifications)
		if err != nil {
			return StaffUpdateResult{}, err
		}
		return StaffUpdateResult{Staff: legacyStaffToMembership(staff), Teacher: legacyTeacherToMembership(teacher), Action: StaffTeacherActionFor(action)}, nil
	}
	return createStaff, updateStaff
}

// ClassifyStaffWriteFailure maps the create/update/offboard sentinels:
// adoption not permitted -> forbidden, Lehrkraft caregiver profile ->
// conflict, staff in use -> conflict with its own message, else internal.
// Constraint violations are classified by the HTTP layer, which owns that
// database-error knowledge.
func ClassifyStaffWriteFailure(err error) (StaffFailureKind, error) {
	switch {
	case peoplePorts.IsPersonNotFound(err):
		return StaffFailureNotFound, err
	case errors.Is(err, peoplePorts.ErrStaffAdoptionNotPermitted):
		return StaffFailureForbidden, err
	case errors.Is(err, schoolmembership.ErrStaffPersonConflict):
		return StaffFailureConflict, err
	case errors.Is(err, peoplePorts.ErrStaffLehrkraftCaregiverProfile):
		return StaffFailureConflict, err
	case errors.Is(err, peoplePorts.ErrStaffInUse):
		return StaffFailureConflict, peoplePorts.ErrStaffInUse
	case errors.Is(err, staffoffboarding.ErrConflict):
		return StaffFailureConflict, err
	case errors.Is(err, staffoffboarding.ErrUnauthorized):
		return StaffFailureForbidden, err
	default:
		return StaffFailureInternal, err
	}
}

// ClassifyStaffSchoolClassFailure maps the class-teacher sentinels: unknown
// staff -> not found, empty class name -> invalid request (the bare German
// sentinel, without the "education: {Op}:" prefix), else internal with the
// wrapped error kept for the logs.
func ClassifyStaffSchoolClassFailure(err error) (StaffFailureKind, error) {
	var wrapped *schoolstructure.EducationError
	inner := err
	if errors.As(err, &wrapped) && wrapped.Err != nil {
		inner = wrapped.Err
	}
	switch {
	case errors.Is(inner, schoolstructure.ErrStaffNotFound):
		return StaffFailureNotFound, schoolstructure.ErrStaffNotFound
	case errors.Is(inner, schoolstructure.ErrEmptySchoolClass):
		return StaffFailureInvalidRequest, schoolstructure.ErrEmptySchoolClass
	default:
		return StaffFailureInternal, err
	}
}

func toStaffDirectoryPerson(id int64, firstName, lastName string, tagID *string, accountID *int64, createdAt, updatedAt time.Time) StaffDirectoryPerson {
	person := StaffDirectoryPerson{ID: id, FirstName: firstName, LastName: lastName, AccountID: accountID, CreatedAt: createdAt, UpdatedAt: updatedAt}
	if tagID != nil {
		person.TagID = *tagID
	}
	return person
}

// StaffDirectoryCreateInput is the staff directory's spelling of a create
// request.
func StaffDirectoryCreateInput(input StaffCreateInput) peoplePorts.CreateStaffInput {
	return peoplePorts.CreateStaffInput{
		PersonID: input.PersonID, StaffNotes: input.StaffNotes, IsTeacher: input.IsTeacher,
		Specialization: input.Specialization, Role: input.Role, Qualifications: input.Qualifications,
		ActorPermissions: input.ActorPermissions,
	}
}

// StaffTeacherActionFor names the teacher-record outcome of a staff update.
func StaffTeacherActionFor(action peoplePorts.TeacherAction) StaffTeacherAction {
	switch action {
	case peoplePorts.TeacherActionExisting:
		return StaffTeacherActionExisting
	case peoplePorts.TeacherActionUpdated:
		return StaffTeacherActionUpdated
	case peoplePorts.TeacherActionUpdateFailed:
		return StaffTeacherActionUpdateFailed
	case peoplePorts.TeacherActionCreated:
		return StaffTeacherActionCreated
	case peoplePorts.TeacherActionCreateFailed:
		return StaffTeacherActionCreateFailed
	default:
		return StaffTeacherActionNone
	}
}

// StaffOffboardingAccess binds the staff offboarding workflow to the
// Identity & Access access step. The workflow declares the snapshot and the
// result itself and may not name the owner, so the root maps them (#3364).
func StaffOffboardingAccess(module identityaccess.StaffOffboardingAccess) offboardingcompose.Access {
	if module == nil {
		return nil
	}
	return staffOffboardingAccess{module: module}
}

type staffOffboardingAccess struct {
	module identityaccess.StaffOffboardingAccess
}

func (a staffOffboardingAccess) PreviewStaffOffboarding(ctx context.Context, accountID int64) (offboardingcompose.StaffOffboardingPreview, error) {
	preview, err := a.module.PreviewStaffOffboarding(ctx, accountID)
	return offboardingcompose.StaffOffboardingPreview(preview), err
}

func (a staffOffboardingAccess) ExecuteStaffOffboarding(ctx context.Context, accountID int64, revision string) (offboardingcompose.StaffOffboardingResult, error) {
	result, err := a.module.ExecuteStaffOffboarding(ctx, accountID, revision)
	return offboardingcompose.StaffOffboardingResult(result), err
}
