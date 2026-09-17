package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

// The stub sessions double also serves the lifecycle port (#3225) for the
// retained role, link and registration flows under test: the role
// classification is the owner's public decision, the school identity chain
// is provisioned through the real repositories the stub carries, and the
// flows that moved out of this package entirely (staff preview, staff
// offboarding, parent accounts, guardian relative access) are not
// supported here; their behaviour tests compose the owner module.

var errStubLifecycleNotSupported = errors.New("account lifecycle stub: operation not supported in this test")

func stubRoleFacts(role *RoleFacts) *identityaccess.RoleFacts {
	if role == nil {
		return nil
	}
	facts := identityaccess.RoleFacts(*role)
	return &facts
}

func (s *stubAccountSessions) RoleNeedsStaffRecord(role *RoleFacts) bool {
	return identityaccess.RoleNeedsStaffRecord(stubRoleFacts(role))
}

func (s *stubAccountSessions) RoleNeedsCaregiverProfile(role *RoleFacts) bool {
	return identityaccess.RoleNeedsCaregiverProfile(stubRoleFacts(role))
}

func (s *stubAccountSessions) IsPlatformCaregiverRole(role *RoleFacts) bool {
	return identityaccess.IsPlatformCaregiverRole(stubRoleFacts(role))
}

// EnsureSchoolIdentity is the idempotent person -> staff -> teacher chain
// over the stub's repositories, without the transponder and import-hint
// handling the owner module covers.
func (s *stubAccountSessions) EnsureSchoolIdentity(ctx context.Context, in SchoolIdentityInput) (*SchoolIdentity, error) {
	if s.repos == nil {
		return nil, errStubLifecycleNotSupported
	}
	if !s.RoleNeedsStaffRecord(in.Role) {
		return nil, nil
	}
	person, err := s.repos.Person.FindByAccountID(ctx, in.AccountID)
	if err != nil {
		return nil, err
	}
	if person == nil || person.DeletedAt != nil {
		if !in.CreatePerson {
			return nil, nil
		}
		if in.FirstName == "" || in.LastName == "" {
			return nil, ErrSchoolIdentityNamesRequired
		}
		person = &userModels.Person{FirstName: in.FirstName, LastName: in.LastName, TagID: in.TagID}
		person.SetTenantID(in.TenantID)
		if err := s.repos.Person.Create(ctx, person); err != nil {
			return nil, fmt.Errorf("create person: %w", err)
		}
		if err := s.repos.Person.LinkToAccount(ctx, person.ID, in.AccountID); err != nil {
			return nil, fmt.Errorf("link person to account: %w", err)
		}
	}
	staff, err := s.repos.Staff.FindByPersonID(ctx, person.ID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if staff == nil || staff.DeletedAt != nil {
		staff = &userModels.Staff{PersonID: person.ID}
		staff.SetTenantID(in.TenantID)
		if err := s.repos.Staff.Create(ctx, staff); err != nil {
			return nil, fmt.Errorf("create staff: %w", err)
		}
	}
	identity := &SchoolIdentity{PersonID: person.ID, StaffID: staff.ID}
	if !s.RoleNeedsCaregiverProfile(in.Role) && !in.CaregiverUpgrade {
		return identity, nil
	}
	teacher, err := s.repos.Teacher.FindByStaffID(ctx, staff.ID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if teacher == nil || teacher.DeletedAt != nil {
		teacher = &userModels.Teacher{StaffID: staff.ID, Role: in.Position}
		teacher.SetTenantID(in.TenantID)
		if err := s.repos.Teacher.Create(ctx, teacher); err != nil {
			return nil, fmt.Errorf("create teacher: %w", err)
		}
	}
	identity.TeacherID = teacher.ID
	return identity, nil
}

func (s *stubAccountSessions) StartStaffPreview(context.Context, int64, int64, int64, string, string, string) (*StaffPreviewSession, error) {
	return nil, errStubLifecycleNotSupported
}

func (s *stubAccountSessions) EndStaffPreview(context.Context, string, string, string) (int64, error) {
	return 0, errStubLifecycleNotSupported
}

func (s *stubAccountSessions) ListStaffPreviewCandidates(context.Context, int64, int64) ([]StaffPreviewCandidate, error) {
	return nil, errStubLifecycleNotSupported
}

func (s *stubAccountSessions) PreviewStaffOffboarding(context.Context, int64) (StaffOffboardingPreview, error) {
	return StaffOffboardingPreview{}, errStubLifecycleNotSupported
}

func (s *stubAccountSessions) ExecuteStaffOffboarding(context.Context, int64, string) (StaffOffboardingResult, error) {
	return StaffOffboardingResult{}, errStubLifecycleNotSupported
}

func (s *stubAccountSessions) CreateParentAccount(context.Context, string, string, string) (ParentAccountRecord, error) {
	return ParentAccountRecord{}, errStubLifecycleNotSupported
}

func (s *stubAccountSessions) GetParentAccountByID(context.Context, int64) (ParentAccountRecord, error) {
	return ParentAccountRecord{}, errStubLifecycleNotSupported
}

func (s *stubAccountSessions) GetParentAccountByEmail(context.Context, string) (ParentAccountRecord, error) {
	return ParentAccountRecord{}, errStubLifecycleNotSupported
}

func (s *stubAccountSessions) UpdateParentAccount(context.Context, ParentAccountRecord) error {
	return errStubLifecycleNotSupported
}

func (s *stubAccountSessions) ActivateParentAccount(context.Context, int64) error {
	return errStubLifecycleNotSupported
}

func (s *stubAccountSessions) DeactivateParentAccount(context.Context, int64) error {
	return errStubLifecycleNotSupported
}

func (s *stubAccountSessions) ListParentAccounts(context.Context, string, *bool) ([]ParentAccountRecord, error) {
	return nil, errStubLifecycleNotSupported
}

func (s *stubAccountSessions) InviteToStudent(context.Context, InviteToStudentRequest) (*InviteToStudentResult, error) {
	return nil, errStubLifecycleNotSupported
}

func (s *stubAccountSessions) ApproveInvitation(context.Context, int64, int64) error {
	return errStubLifecycleNotSupported
}

func (s *stubAccountSessions) RejectInvitation(context.Context, int64, int64) error {
	return errStubLifecycleNotSupported
}

func (s *stubAccountSessions) PendingInvitationStudentID(context.Context, int64) (int64, error) {
	return 0, errStubLifecycleNotSupported
}

func (s *stubAccountSessions) ListPendingApprovalsDetailed(context.Context) ([]*PendingApprovalView, error) {
	return nil, errStubLifecycleNotSupported
}

func (s *stubAccountSessions) RevokeAccess(context.Context, RevokeAccessRequest) error {
	return errStubLifecycleNotSupported
}

var _ AccountLifecycle = (*stubAccountSessions)(nil)
