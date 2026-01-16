package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/gofrs/uuid"
	authModels "github.com/moto-nrw/project-phoenix/internal/core/domain/auth"
	modelBase "github.com/moto-nrw/project-phoenix/internal/core/domain/base"
)

// validateInvitationRequest validates all required fields and returns the normalized email.
func (s *invitationService) validateInvitationRequest(ctx context.Context, req InvitationRequest) (string, error) {
	emailAddress := strings.TrimSpace(strings.ToLower(req.Email))
	if emailAddress == "" {
		return "", &AuthError{Op: opCreateInvitation, Err: fmt.Errorf("email is required")}
	}

	if _, err := mail.ParseAddress(emailAddress); err != nil {
		return "", &AuthError{Op: opCreateInvitation, Err: fmt.Errorf("invalid email address")}
	}

	if err := s.ensureEmailNotRegistered(ctx, emailAddress, opCreateInvitation); err != nil {
		return "", err
	}

	if req.RoleID <= 0 {
		return "", &AuthError{Op: opCreateInvitation, Err: fmt.Errorf("role id is required")}
	}

	if req.CreatedBy <= 0 {
		return "", &AuthError{Op: opCreateInvitation, Err: fmt.Errorf("created_by is required")}
	}

	if err := s.ensureRoleExists(ctx, req.RoleID); err != nil {
		return "", err
	}

	return emailAddress, nil
}

// ensureEmailNotRegistered checks that no account exists with the given email.
func (s *invitationService) ensureEmailNotRegistered(ctx context.Context, email, op string) error {
	_, err := s.accountRepo.FindByEmail(ctx, email)
	if err == nil {
		return &AuthError{Op: op, Err: ErrEmailAlreadyExists}
	}
	if !isNotFoundError(err) {
		return &AuthError{Op: op, Err: err}
	}
	return nil
}

// ensureRoleExists verifies the role ID is valid.
func (s *invitationService) ensureRoleExists(ctx context.Context, roleID int64) error {
	_, err := s.roleRepo.FindByID(ctx, roleID)
	if err == nil {
		return nil
	}
	if isNotFoundError(err) {
		return &AuthError{Op: opCreateInvitation, Err: fmt.Errorf("role not found")}
	}
	return &AuthError{Op: opCreateInvitation, Err: err}
}

// invalidatePreviousInvitations marks any pending invitations for this email as used.
func (s *invitationService) invalidatePreviousInvitations(ctx context.Context, email string) error {
	_, err := s.invitationRepo.InvalidateByEmail(ctx, email)
	if err != nil {
		return &AuthError{Op: "invalidate invitations", Err: err}
	}
	return nil
}

// buildInvitationToken constructs the invitation token with optional fields.
func (s *invitationService) buildInvitationToken(email string, req InvitationRequest) *authModels.InvitationToken {
	invitation := &authModels.InvitationToken{
		Email:     email,
		Token:     uuid.Must(uuid.NewV4()).String(),
		RoleID:    req.RoleID,
		CreatedBy: req.CreatedBy,
		ExpiresAt: time.Now().Add(s.invitationExpiry),
	}

	if req.FirstName != nil {
		firstName := strings.TrimSpace(*req.FirstName)
		invitation.FirstName = &firstName
	}
	if req.LastName != nil {
		lastName := strings.TrimSpace(*req.LastName)
		invitation.LastName = &lastName
	}
	if req.Position != nil {
		position := strings.TrimSpace(*req.Position)
		invitation.Position = &position
	}

	return invitation
}

// attachRoleAndCreator populates the Role and Creator fields on the invitation.
func (s *invitationService) attachRoleAndCreator(ctx context.Context, invitation *authModels.InvitationToken) error {
	roleName, _ := s.lookupRoleName(ctx, invitation.RoleID)
	if roleName != "" {
		invitation.Role = &authModels.Role{
			Model: modelBase.Model{ID: invitation.RoleID},
			Name:  roleName,
		}
	}

	creator, err := s.accountRepo.FindByID(ctx, invitation.CreatedBy)
	if err != nil && !isNotFoundError(err) {
		return &AuthError{Op: "lookup creator", Err: err}
	}
	if creator != nil {
		invitation.Creator = &authModels.Account{
			Model: modelBase.Model{ID: creator.ID},
			Email: creator.Email,
		}
	}

	return nil
}

func (s *invitationService) fetchValidInvitation(ctx context.Context, token string) (*authModels.InvitationToken, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, &AuthError{Op: opFetchInvitation, Err: ErrInvitationNotFound}
	}

	invitation, err := s.invitationRepo.FindByToken(ctx, token)
	if err != nil {
		if isNotFoundError(err) {
			return nil, &AuthError{Op: opFetchInvitation, Err: ErrInvitationNotFound}
		}
		return nil, &AuthError{Op: opFetchInvitation, Err: err}
	}

	if invitation.IsUsed() {
		return nil, &AuthError{Op: opFetchInvitation, Err: ErrInvitationUsed}
	}

	if invitation.IsExpired() {
		return nil, &AuthError{Op: opFetchInvitation, Err: ErrInvitationExpired}
	}

	return invitation, nil
}

func (s *invitationService) lookupRoleName(ctx context.Context, roleID int64) (string, error) {
	role, err := s.roleRepo.FindByID(ctx, roleID)
	if err != nil {
		if isNotFoundError(err) {
			return "", &AuthError{Op: "lookup role", Err: fmt.Errorf("role not found")}
		}
		return "", &AuthError{Op: "lookup role", Err: err}
	}
	return role.Name, nil
}

func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, sql.ErrNoRows) {
		return true
	}

	var dbErr *modelBase.DatabaseError
	if errors.As(err, &dbErr) {
		return errors.Is(dbErr.Err, sql.ErrNoRows)
	}

	return false
}
