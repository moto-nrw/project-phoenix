package users

import (
	"context"
	"fmt"
	"time"

	"github.com/gofrs/uuid"
	authModels "github.com/moto-nrw/project-phoenix/internal/core/domain/auth"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/users"
	"github.com/moto-nrw/project-phoenix/internal/core/logger"
	"github.com/moto-nrw/project-phoenix/internal/core/port"
	authService "github.com/moto-nrw/project-phoenix/internal/core/service/auth"
	"github.com/uptrace/bun"
)

// SendInvitation sends an invitation to a guardian
func (s *guardianService) SendInvitation(ctx context.Context, req GuardianInvitationRequest) (*authModels.GuardianInvitation, error) {
	// Get guardian profile
	profile, err := s.guardianProfileRepo.FindByID(ctx, req.GuardianProfileID)
	if err != nil {
		return nil, fmt.Errorf(errMsgGuardianNotFound, err)
	}

	// Validate guardian can be invited
	if !profile.CanInvite() {
		return nil, fmt.Errorf("guardian cannot be invited: either no email or already has account")
	}

	// Check for pending invitations
	existingInvitations, err := s.guardianInvitationRepo.FindByGuardianProfileID(ctx, profile.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing invitations: %w", err)
	}

	// Check if there's a valid pending invitation
	for _, inv := range existingInvitations {
		if inv.IsValid() {
			return nil, fmt.Errorf("guardian already has a pending invitation")
		}
	}

	// Create invitation
	token := uuid.Must(uuid.NewV4()).String()
	invitation := &authModels.GuardianInvitation{
		Token:             token,
		GuardianProfileID: profile.ID,
		CreatedBy:         req.CreatedBy,
		ExpiresAt:         time.Now().Add(s.invitationExpiry),
	}

	if err := s.guardianInvitationRepo.Create(ctx, invitation); err != nil {
		return nil, fmt.Errorf("failed to create invitation: %w", err)
	}

	// Send invitation email asynchronously
	if s.dispatcher != nil && profile.Email != nil {
		go s.sendInvitationEmail(invitation, profile)
	}

	return invitation, nil
}

// sendInvitationEmail sends the invitation email (called asynchronously)
func (s *guardianService) sendInvitationEmail(invitation *authModels.GuardianInvitation, profile *users.GuardianProfile) {
	if s.dispatcher == nil || profile.Email == nil {
		return
	}

	invitationURL := fmt.Sprintf("%s/guardian/invite?token=%s", s.frontendURL, invitation.Token)
	expiryHours := int(s.invitationExpiry.Hours())

	// P2 FIX: Handle errors gracefully in async email context
	// If we can't load student names, log the error but continue with empty list
	// (better to send the invitation without student names than to fail completely)
	studentNames, err := s.getStudentNamesForGuardian(context.Background(), profile.ID)
	if err != nil {
		logger.Logger.WithError(err).WithFields(map[string]any{
			"guardian_profile_id": profile.ID,
			"invitation_id":       invitation.ID,
		}).Warn("Failed to load student names for guardian invitation email")
		studentNames = []string{} // Use empty list as fallback
	}

	message := port.EmailMessage{
		From:     s.defaultFrom,
		To:       port.EmailAddress{Address: *profile.Email},
		Subject:  "Einladung zum Eltern-Portal",
		Template: "guardian-invitation.html",
		Content: map[string]interface{}{
			"FirstName":     profile.FirstName,
			"LastName":      profile.LastName,
			"InvitationURL": invitationURL,
			"ExpiryHours":   expiryHours,
			"LogoURL":       fmt.Sprintf("%s/logo.png", s.frontendURL),
			"StudentNames":  studentNames,
		},
	}

	meta := port.DeliveryMetadata{
		Type:        "guardian_invitation",
		ReferenceID: invitation.ID,
		Token:       invitation.Token,
		Recipient:   *profile.Email,
	}

	if s.dispatcher != nil {
		s.dispatcher.Dispatch(context.Background(), port.DeliveryRequest{
			Message:  message,
			Metadata: meta,
		})
	}

	// Update email status
	now := time.Now()
	_ = s.guardianInvitationRepo.UpdateEmailStatus(context.Background(), invitation.ID, &now, nil, 0)
}

// getStudentNamesForGuardian retrieves the full names of all students linked to a guardian
// Returns an error if the guardian-student relationships cannot be loaded or if any student/person
// lookup fails. This ensures callers can distinguish between "no students" and "data retrieval failure".
func (s *guardianService) getStudentNamesForGuardian(ctx context.Context, guardianProfileID int64) ([]string, error) {
	relationships, err := s.studentGuardianRepo.FindByGuardianProfileID(ctx, guardianProfileID)
	if err != nil {
		return nil, fmt.Errorf("failed to load guardian-student relationships: %w", err)
	}

	studentNames := make([]string, 0, len(relationships))
	for _, rel := range relationships {
		student, err := s.studentRepo.FindByID(ctx, rel.StudentID)
		if err != nil {
			return nil, fmt.Errorf("failed to load student %d: %w", rel.StudentID, err)
		}

		person, err := s.personRepo.FindByID(ctx, student.PersonID)
		if err != nil {
			return nil, fmt.Errorf("failed to load person %d for student %d: %w", student.PersonID, rel.StudentID, err)
		}

		// P1 FIX: Guard against nil person record (some repositories return (nil, nil) for missing rows)
		if person == nil {
			return nil, fmt.Errorf("person record %d is missing for student %d", student.PersonID, rel.StudentID)
		}

		studentNames = append(studentNames, person.GetFullName())
	}

	return studentNames, nil
}

// ValidateInvitation validates an invitation token
func (s *guardianService) ValidateInvitation(ctx context.Context, token string) (*GuardianInvitationValidationResult, error) {
	invitation, err := s.guardianInvitationRepo.FindByToken(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("invitation not found: %w", err)
	}

	if err := s.validateInvitationStatus(invitation); err != nil {
		return nil, err
	}

	// Get guardian profile
	profile, err := s.guardianProfileRepo.FindByID(ctx, invitation.GuardianProfileID)
	if err != nil {
		return nil, fmt.Errorf(errMsgGuardianNotFound, err)
	}

	// P2 FIX: Propagate errors from student name lookup instead of swallowing them
	studentNames, err := s.getStudentNamesForGuardian(ctx, profile.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to load student information for guardian %d: %w", profile.ID, err)
	}

	email := ""
	if profile.Email != nil {
		email = *profile.Email
	}

	return &GuardianInvitationValidationResult{
		GuardianFirstName: profile.FirstName,
		GuardianLastName:  profile.LastName,
		Email:             email,
		StudentNames:      studentNames,
		ExpiresAt:         invitation.ExpiresAt.Format(time.RFC3339),
	}, nil
}

// AcceptInvitation accepts an invitation and creates a guardian account
func (s *guardianService) AcceptInvitation(ctx context.Context, req GuardianInvitationAcceptRequest) (*authModels.AccountParent, error) {
	if err := s.validateInvitationAcceptRequest(req); err != nil {
		return nil, err
	}

	var account *authModels.AccountParent
	err := s.txHandler.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		svc := s.WithTx(tx).(*guardianService)

		invitation, profile, err := svc.validateInvitationAndProfile(ctx, req.Token)
		if err != nil {
			return err
		}

		account, err = svc.createGuardianAccountFromInvitation(ctx, profile, req.Password)
		if err != nil {
			return err
		}

		return svc.finalizeInvitationAcceptance(ctx, invitation.ID, profile.ID, account.ID)
	})

	if err != nil {
		return nil, err
	}

	return account, nil
}

// validateInvitationAcceptRequest validates the invitation acceptance request
func (s *guardianService) validateInvitationAcceptRequest(req GuardianInvitationAcceptRequest) error {
	if req.Password != req.ConfirmPassword {
		return fmt.Errorf("passwords do not match")
	}

	if err := authService.ValidatePasswordStrength(req.Password); err != nil {
		return fmt.Errorf("password validation failed: %w", err)
	}

	return nil
}

// validateInvitationAndProfile validates invitation and retrieves guardian profile
func (s *guardianService) validateInvitationAndProfile(ctx context.Context, token string) (*authModels.GuardianInvitation, *users.GuardianProfile, error) {
	invitation, err := s.guardianInvitationRepo.FindByToken(ctx, token)
	if err != nil {
		return nil, nil, fmt.Errorf("invitation not found: %w", err)
	}

	if err := s.validateInvitationStatus(invitation); err != nil {
		return nil, nil, err
	}

	profile, err := s.guardianProfileRepo.FindByID(ctx, invitation.GuardianProfileID)
	if err != nil {
		return nil, nil, fmt.Errorf(errMsgGuardianNotFound, err)
	}

	if profile.Email == nil || *profile.Email == "" {
		return nil, nil, fmt.Errorf("guardian profile has no email")
	}

	return invitation, profile, nil
}

// validateInvitationStatus checks if invitation is valid and returns appropriate error
func (s *guardianService) validateInvitationStatus(invitation *authModels.GuardianInvitation) error {
	if invitation.IsValid() {
		return nil
	}

	if invitation.IsExpired() {
		return fmt.Errorf("invitation has expired")
	}

	if invitation.IsAccepted() {
		return fmt.Errorf("invitation has already been accepted")
	}

	return fmt.Errorf("invitation is no longer valid")
}

// createGuardianAccountFromInvitation creates a new guardian account with hashed password
func (s *guardianService) createGuardianAccountFromInvitation(ctx context.Context, profile *users.GuardianProfile, password string) (*authModels.AccountParent, error) {
	passwordHash, err := authService.HashPassword(password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	account := &authModels.AccountParent{
		Email:        *profile.Email,
		PasswordHash: &passwordHash,
		Active:       true,
	}

	if err := s.accountParentRepo.Create(ctx, account); err != nil {
		return nil, fmt.Errorf("failed to create account: %w", err)
	}

	return account, nil
}

// finalizeInvitationAcceptance links account to profile and marks invitation as accepted
func (s *guardianService) finalizeInvitationAcceptance(ctx context.Context, invitationID, profileID, accountID int64) error {
	if err := s.guardianProfileRepo.LinkAccount(ctx, profileID, accountID); err != nil {
		return fmt.Errorf("failed to link account to profile: %w", err)
	}

	if err := s.guardianInvitationRepo.MarkAsAccepted(ctx, invitationID); err != nil {
		return fmt.Errorf("failed to mark invitation as accepted: %w", err)
	}

	return nil
}

// GetPendingInvitations retrieves all pending guardian invitations
func (s *guardianService) GetPendingInvitations(ctx context.Context) ([]*authModels.GuardianInvitation, error) {
	return s.guardianInvitationRepo.FindPending(ctx)
}

// CleanupExpiredInvitations deletes expired invitations
func (s *guardianService) CleanupExpiredInvitations(ctx context.Context) (int, error) {
	return s.guardianInvitationRepo.DeleteExpired(ctx)
}
