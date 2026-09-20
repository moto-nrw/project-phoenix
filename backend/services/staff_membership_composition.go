package services

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	educationSvc "github.com/moto-nrw/project-phoenix/services/education"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/workflows/staffoffboarding"
	offboardingcompose "github.com/moto-nrw/project-phoenix/workflows/staffoffboarding/compose"
	"github.com/uptrace/bun"
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

// StaffTeacherAction mirrors usersSvc.TeacherAction as a stable string.
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

	PresentStaffIDs func(context.Context) ([]int64, error)
	WorkStatusMap   func(context.Context) (map[int64]string, error)
	AbsenceMap      func(context.Context) (map[int64]string, error)
	AbsenceLabelMap func(context.Context) (map[int64]string, error)
	AccountRoles    func(context.Context, []int64) (map[int64]string, error)
	AccountEmails   func(context.Context, []int64) (map[int64]string, error)
	AccountAvatars  func(context.Context, []int64) (map[int64]string, error)
	AccountHasRole  func(context.Context, int64, string) bool

	GrantDefaultPermissions func(context.Context, int64, bool)

	TeacherGroups    func(context.Context, int64) ([]StaffGroup, error)
	SchoolClasses    func(context.Context, int64) ([]string, error)
	SetSchoolClasses func(context.Context, int64, []string, int64) error
	ActiveCaregivers func(context.Context) ([]StaffRoleRow, error)
	StaffByRoles     func(context.Context, []string) ([]StaffRoleRow, error)

	CreateStaff func(context.Context, StaffCreateInput) (StaffCreateResult, error)
	UpdateStaff func(context.Context, StaffUpdateInput) (StaffUpdateResult, error)
	Offboard    func(context.Context, int64, string) error
}

// NewStaffMembershipRuntime composes the closures over the service factory.
func (f *Factory) NewStaffMembershipRuntime(db *bun.DB, logger *slog.Logger, hooks StaffMembershipHooks) StaffMembershipRuntime {
	if hooks.ResolveEditorStaffID == nil || hooks.QueueOffboardedDocumentCleanup == nil {
		panic("staff membership runtime: offboarding hooks are required")
	}
	if logger == nil {
		logger = slog.Default()
	}
	if f.Auth == nil {
		panic("staff membership runtime: identity offboarding capability is required")
	}
	access := StaffOffboardingAccess(f.Auth)
	roles := f.AccountAuthentication()
	if roles == nil {
		panic("staff membership runtime: identity role administration is required")
	}
	offboarding, err := offboardingcompose.New(offboardingcompose.Dependencies{
		DB: db, Access: access, Cleanup: hooks.QueueOffboardedDocumentCleanup,
		Broadcaster: f.RealtimeHub, Logger: logger,
		Authorize: func(ctx context.Context) (staffoffboarding.Actor, error) {
			name, _ := ctx.Value(staffOffboardingActorNameKey{}).(string)
			return offboardingcompose.Authorize(ctx, hooks.ResolveEditorStaffID, name)
		},
	})
	if err != nil {
		panic(err)
	}
	return StaffMembershipRuntime{
		Person: func(ctx context.Context, id int64) (StaffDirectoryPerson, error) {
			person, err := f.Users.Get(ctx, id)
			if err != nil {
				return StaffDirectoryPerson{}, err
			}
			return toStaffDirectoryPerson(person.ID, person.FirstName, person.LastName, person.TagID, person.AccountID, person.CreatedAt, person.UpdatedAt), nil
		},
		Persons: func(ctx context.Context, ids []int64) ([]StaffDirectoryPerson, error) {
			persons, err := f.Users.GetByIDs(ctx, ids)
			if err != nil {
				return nil, err
			}
			result := make([]StaffDirectoryPerson, 0, len(persons))
			for _, person := range persons {
				result = append(result, toStaffDirectoryPerson(person.ID, person.FirstName, person.LastName, person.TagID, person.AccountID, person.CreatedAt, person.UpdatedAt))
			}
			return result, nil
		},

		PresentStaffIDs: f.WorkSession.GetStaffIDsWithSupervisionToday,
		WorkStatusMap:   f.WorkSession.GetTodayPresenceMap,
		AbsenceMap:      f.StaffAbsence.GetTodayAbsenceMap,
		AbsenceLabelMap: f.StaffAbsence.GetTodayAbsenceLabelMap,
		AccountRoles:    roles.GetAccountRoleNames,
		AccountEmails:   roles.GetAccountEmails,
		AccountAvatars:  roles.GetAccountAvatars,
		AccountHasRole: func(ctx context.Context, accountID int64, roleName string) bool {
			held, err := roles.GetAccountRoles(ctx, accountID)
			if err != nil {
				return false
			}
			for _, role := range held {
				if role.Name == roleName {
					return true
				}
			}
			return false
		},

		GrantDefaultPermissions: func(ctx context.Context, accountID int64, isTeacher bool) {
			roles.GrantStaffDefaultPermission(ctx, accountID, isTeacher, usersSvc.DefaultStaffAccountPermission)
		},

		TeacherGroups: func(ctx context.Context, teacherID int64) ([]StaffGroup, error) {
			groups, err := f.Education.GetTeacherGroups(ctx, teacherID)
			if err != nil {
				return nil, err
			}
			result := make([]StaffGroup, 0, len(groups))
			for _, group := range groups {
				result = append(result, StaffGroup{ID: group.ID, Name: group.Name})
			}
			return result, nil
		},
		SchoolClasses:    f.Education.GetStaffSchoolClasses,
		SetSchoolClasses: f.Education.SetStaffSchoolClasses,
		ActiveCaregivers: func(ctx context.Context) ([]StaffRoleRow, error) {
			directory, err := usersSvc.CaregiverDirectoryFromPersonService(f.Users)
			if err != nil {
				return nil, err
			}
			caregivers, err := directory.ListActiveCaregivers(ctx)
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
		},
		StaffByRoles: func(ctx context.Context, roles []string) ([]StaffRoleRow, error) {
			rows, err := f.Users.ListStaffByRoles(ctx, roles)
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
		},

		CreateStaff: func(ctx context.Context, input StaffCreateInput) (StaffCreateResult, error) {
			staff, teacher, teacherCreationFailed, err := f.Users.CreateStaffWithTeacher(ctx, usersSvc.CreateStaffInput{
				PersonID: input.PersonID, StaffNotes: input.StaffNotes, IsTeacher: input.IsTeacher,
				Specialization: input.Specialization, Role: input.Role, Qualifications: input.Qualifications,
				ActorPermissions: input.ActorPermissions,
			})
			if err != nil {
				return StaffCreateResult{}, err
			}
			return StaffCreateResult{Staff: legacyStaffToMembership(staff), Teacher: legacyTeacherToMembership(teacher), TeacherCreationFailed: teacherCreationFailed}, nil
		},
		UpdateStaff: func(ctx context.Context, input StaffUpdateInput) (StaffUpdateResult, error) {
			staff, err := f.Users.GetStaffByID(ctx, input.StaffID)
			if err != nil {
				return StaffUpdateResult{}, err
			}
			staff.StaffNotes = input.StaffNotes
			staff.PersonID = input.PersonID
			teacher, action, err := f.Users.UpdateStaffWithTeacher(ctx, staff, input.IsTeacher, input.Specialization, input.Role, input.Qualifications)
			if err != nil {
				return StaffUpdateResult{}, err
			}
			return StaffUpdateResult{Staff: legacyStaffToMembership(staff), Teacher: legacyTeacherToMembership(teacher), Action: staffTeacherAction(action)}, nil
		},
		Offboard: func(ctx context.Context, staffID int64, actorUsername string) error {
			_, err := offboarding.Offboard(context.WithValue(ctx, staffOffboardingActorNameKey{}, actorUsername), staffID)
			if errors.Is(err, staffoffboarding.ErrInUse) {
				return usersSvc.ErrStaffInUse
			}
			return err
		},
	}
}

// ClassifyStaffWriteFailure maps the create/update/offboard sentinels:
// adoption not permitted -> forbidden, Lehrkraft caregiver profile ->
// conflict, staff in use -> conflict with its own message, else internal.
// Constraint violations are classified by the HTTP layer, which owns that
// database-error knowledge.
func ClassifyStaffWriteFailure(err error) (StaffFailureKind, error) {
	switch {
	case errors.Is(err, usersSvc.ErrStaffAdoptionNotPermitted):
		return StaffFailureForbidden, err
	case errors.Is(err, schoolmembership.ErrStaffPersonConflict):
		return StaffFailureConflict, err
	case errors.Is(err, usersSvc.ErrStaffLehrkraftCaregiverProfile):
		return StaffFailureConflict, err
	case errors.Is(err, usersSvc.ErrStaffInUse):
		return StaffFailureConflict, usersSvc.ErrStaffInUse
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
	var wrapped *educationSvc.EducationError
	inner := err
	if errors.As(err, &wrapped) && wrapped.Err != nil {
		inner = wrapped.Err
	}
	switch {
	case errors.Is(inner, educationSvc.ErrStaffNotFound):
		return StaffFailureNotFound, educationSvc.ErrStaffNotFound
	case errors.Is(inner, educationSvc.ErrEmptySchoolClass):
		return StaffFailureInvalidRequest, educationSvc.ErrEmptySchoolClass
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

func staffTeacherAction(action usersSvc.TeacherAction) StaffTeacherAction {
	switch action {
	case usersSvc.TeacherActionExisting:
		return StaffTeacherActionExisting
	case usersSvc.TeacherActionUpdated:
		return StaffTeacherActionUpdated
	case usersSvc.TeacherActionUpdateFailed:
		return StaffTeacherActionUpdateFailed
	case usersSvc.TeacherActionCreated:
		return StaffTeacherActionCreated
	case usersSvc.TeacherActionCreateFailed:
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
