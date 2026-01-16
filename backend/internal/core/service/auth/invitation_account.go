package auth

import (
	"context"
	"strings"

	authModels "github.com/moto-nrw/project-phoenix/internal/core/domain/auth"
	userModels "github.com/moto-nrw/project-phoenix/internal/core/domain/users"
)

// validateAndHashPassword validates password match and strength, then returns the hash.
func (s *invitationService) validateAndHashPassword(userData UserRegistrationData) (string, error) {
	if userData.Password != userData.ConfirmPassword {
		return "", &AuthError{Op: opAcceptInvitation, Err: ErrPasswordMismatch}
	}

	if err := ValidatePasswordStrength(userData.Password); err != nil {
		return "", &AuthError{Op: opAcceptInvitation, Err: err}
	}

	passwordHash, err := HashPassword(userData.Password)
	if err != nil {
		return "", &AuthError{Op: opAcceptInvitation, Err: err}
	}

	return passwordHash, nil
}

// resolveNames resolves first and last name from user data or invitation fallback.
func (s *invitationService) resolveNames(userData UserRegistrationData, invitation *authModels.InvitationToken) (string, string, error) {
	firstName := strings.TrimSpace(userData.FirstName)
	lastName := strings.TrimSpace(userData.LastName)

	if firstName == "" && invitation.FirstName != nil {
		firstName = strings.TrimSpace(*invitation.FirstName)
	}
	if lastName == "" && invitation.LastName != nil {
		lastName = strings.TrimSpace(*invitation.LastName)
	}

	if firstName == "" || lastName == "" {
		return "", "", &AuthError{Op: opAcceptInvitation, Err: ErrInvitationNameRequired}
	}

	return firstName, lastName, nil
}

// createAccountWithRole creates person, account, role assignment, and optional staff/teacher records.
func (s *invitationService) createAccountWithRole(
	ctx context.Context,
	invitation *authModels.InvitationToken,
	passwordHash, firstName, lastName string,
) (*authModels.Account, error) {
	person, err := s.createPerson(ctx, firstName, lastName)
	if err != nil {
		return nil, err
	}

	account, err := s.createAccount(ctx, invitation.Email, passwordHash)
	if err != nil {
		return nil, err
	}

	if err := s.personRepo.LinkToAccount(ctx, person.ID, account.ID); err != nil {
		return nil, &AuthError{Op: "link person to account", Err: err}
	}

	if err := s.assignRole(ctx, account.ID, invitation.RoleID); err != nil {
		return nil, err
	}

	if err := s.createStaffAndTeacherIfSystemRole(ctx, person.ID, invitation); err != nil {
		return nil, err
	}

	if err := s.invitationRepo.MarkAsUsed(ctx, invitation.ID); err != nil {
		return nil, &AuthError{Op: "mark invitation used", Err: err}
	}

	return account, nil
}

// createPerson creates a new person record.
func (s *invitationService) createPerson(ctx context.Context, firstName, lastName string) (*userModels.Person, error) {
	person := &userModels.Person{
		FirstName: firstName,
		LastName:  lastName,
	}
	if err := s.personRepo.Create(ctx, person); err != nil {
		return nil, &AuthError{Op: "create person", Err: err}
	}
	return person, nil
}

// createAccount creates a new account record.
func (s *invitationService) createAccount(ctx context.Context, email, passwordHash string) (*authModels.Account, error) {
	account := &authModels.Account{
		Email:        email,
		Active:       true,
		PasswordHash: &passwordHash,
	}
	if err := s.accountRepo.Create(ctx, account); err != nil {
		return nil, &AuthError{Op: "create account", Err: err}
	}
	return account, nil
}

// assignRole assigns a role to an account.
func (s *invitationService) assignRole(ctx context.Context, accountID, roleID int64) error {
	accountRole := &authModels.AccountRole{
		AccountID: accountID,
		RoleID:    roleID,
	}
	if err := s.accountRoleRepo.Create(ctx, accountRole); err != nil {
		return &AuthError{Op: "assign role", Err: err}
	}
	return nil
}

// createStaffAndTeacherIfSystemRole creates staff and teacher records for system roles.
func (s *invitationService) createStaffAndTeacherIfSystemRole(
	ctx context.Context,
	personID int64,
	invitation *authModels.InvitationToken,
) error {
	role, err := s.roleRepo.FindByID(ctx, invitation.RoleID)
	if err != nil || role == nil || !role.IsSystem {
		return nil // Not a system role or error looking up - skip staff/teacher creation
	}

	staff := &userModels.Staff{PersonID: personID}
	if err := s.staffRepo.Create(ctx, staff); err != nil {
		return &AuthError{Op: "create staff", Err: err}
	}

	teacher := &userModels.Teacher{StaffID: staff.ID}
	if invitation.Position != nil {
		teacher.Role = *invitation.Position
	}
	if err := s.teacherRepo.Create(ctx, teacher); err != nil {
		return &AuthError{Op: "create teacher", Err: err}
	}

	return nil
}
