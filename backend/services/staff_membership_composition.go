package services

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	authSvc "github.com/moto-nrw/project-phoenix/services/auth"
	educationSvc "github.com/moto-nrw/project-phoenix/services/education"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
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

// StaffMembershipRuntime is the legacy-service side of the School Membership
// HTTP adapter. Every closure keeps the exact semantics of the handler code
// it replaced in api/staff.
type StaffMembershipRuntime struct {
	Person            func(context.Context, int64) (StaffDirectoryPerson, error)
	Persons           func(context.Context, []int64) ([]StaffDirectoryPerson, error)
	PersonIDByAccount func(context.Context, int64) (int64, bool, error)

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

	PINStatus    func(context.Context, int64) (bool, *time.Time, error)
	PINPreflight func(context.Context, int64) error
	UpdatePIN    func(context.Context, int64, *string, string) error
}

// NewStaffMembershipRuntime composes the closures over the service factory.
func (f *Factory) NewStaffMembershipRuntime(db *bun.DB, logger *slog.Logger, hooks StaffMembershipHooks) StaffMembershipRuntime {
	if hooks.ResolveEditorStaffID == nil || hooks.QueueOffboardedDocumentCleanup == nil {
		panic("staff membership runtime: offboarding hooks are required")
	}
	if logger == nil {
		logger = slog.Default()
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
		PersonIDByAccount: func(ctx context.Context, accountID int64) (int64, bool, error) {
			// The legacy PIN handlers treated any lookup failure as "no person
			// linked" (an administrator without a staff record).
			person, err := f.Users.FindByAccountID(ctx, accountID)
			if err != nil || person == nil {
				return 0, false, nil
			}
			return person.ID, true, nil
		},

		PresentStaffIDs: f.WorkSession.GetStaffIDsWithSupervisionToday,
		WorkStatusMap:   f.WorkSession.GetTodayPresenceMap,
		AbsenceMap:      f.StaffAbsence.GetTodayAbsenceMap,
		AbsenceLabelMap: f.StaffAbsence.GetTodayAbsenceLabelMap,
		AccountRoles:    f.Auth.GetAccountRoleNames,
		AccountEmails:   f.Auth.GetAccountEmailsByIDs,
		AccountAvatars:  f.Auth.GetAccountAvatarsByIDs,
		AccountHasRole: func(ctx context.Context, accountID int64, roleName string) bool {
			roles, err := f.Auth.GetAccountRoles(ctx, int(accountID))
			if err != nil {
				return false
			}
			for _, role := range roles {
				if role.Name == roleName {
					return true
				}
			}
			return false
		},

		GrantDefaultPermissions: func(ctx context.Context, accountID int64, isTeacher bool) {
			authSvc.GrantStaffDefaultPermissions(ctx, f.Auth, logger, accountID, isTeacher, usersSvc.DefaultStaffAccountPermission)
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
			deletedByStaffID, err := hooks.ResolveEditorStaffID(ctx)
			if err != nil {
				return err
			}
			tenantID := tenant.FromContext(ctx)
			return tenant.WithTenantTx(ctx, db, tenantID, func(ctx context.Context, _ bun.Tx) error {
				if err := f.StaffOffboarding.OffboardStaff(ctx, staffID, deletedByStaffID, actorUsername); err != nil {
					return err
				}
				return hooks.QueueOffboardedDocumentCleanup(ctx, staffID)
			})
		},

		PINStatus: func(ctx context.Context, accountID int64) (bool, *time.Time, error) {
			return authSvc.StaffPINStatus(ctx, f.Auth, accountID)
		},
		PINPreflight: func(ctx context.Context, accountID int64) error {
			return authSvc.StaffPINPreflight(ctx, f.Auth, accountID)
		},
		UpdatePIN: func(ctx context.Context, accountID int64, currentPIN *string, newPIN string) error {
			return authSvc.ChangeStaffPIN(ctx, f.Auth, db, logger, accountID, currentPIN, newPIN)
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

// ClassifyStaffPINFailure maps the PIN self-service sentinels: account not
// found -> not found, locked -> forbidden, missing current PIN -> invalid
// request, wrong current PIN -> unauthorized, else internal.
func ClassifyStaffPINFailure(err error) (StaffFailureKind, error) {
	switch {
	case errors.Is(err, authSvc.ErrStaffPINAccountNotFound):
		return StaffFailureNotFound, err
	case errors.Is(err, authSvc.ErrStaffPINSelfServiceLocked):
		return StaffFailureForbidden, err
	case errors.Is(err, authSvc.ErrStaffPINCurrentRequired):
		return StaffFailureInvalidRequest, err
	case errors.Is(err, authSvc.ErrStaffPINCurrentWrong):
		return StaffFailureUnauthorized, err
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
