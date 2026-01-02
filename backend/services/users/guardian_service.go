package users

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gofrs/uuid"
	"github.com/moto-nrw/project-phoenix/auth/userpass"
	authService "github.com/moto-nrw/project-phoenix/services/auth"
	"github.com/moto-nrw/project-phoenix/email"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

const (
	// errMsgGuardianNotFound is the error message format for guardian profile not found errors
	errMsgGuardianNotFound = "guardian profile not found: %w"
)

// GuardianServiceDependencies contains all dependencies required by the guardian service
type GuardianServiceDependencies struct {
	// Repository dependencies
	GuardianProfileRepo    users.GuardianProfileRepository
	StudentGuardianRepo    users.StudentGuardianRepository
	GuardianInvitationRepo authModels.GuardianInvitationRepository
	AccountParentRepo      authModels.AccountParentRepository
	StudentRepo            users.StudentRepository
	PersonRepo             users.PersonRepository

	// Email dependencies
	Mailer           email.Mailer
	Dispatcher       *email.Dispatcher
	FrontendURL      string
	DefaultFrom      email.Email
	InvitationExpiry time.Duration

	// Infrastructure
	DB *bun.DB
}

type guardianService struct {
	guardianProfileRepo    users.GuardianProfileRepository
	studentGuardianRepo    users.StudentGuardianRepository
	guardianInvitationRepo authModels.GuardianInvitationRepository
	accountParentRepo      authModels.AccountParentRepository
	studentRepo            users.StudentRepository
	personRepo             users.PersonRepository
	dispatcher             *email.Dispatcher
	frontendURL            string
	defaultFrom            email.Email
	invitationExpiry       time.Duration
	db                     *bun.DB
	txHandler              *base.TxHandler
}

// NewGuardianService creates a new GuardianService instance
func NewGuardianService(deps GuardianServiceDependencies) GuardianService {
	trimmedFrontend := strings.TrimRight(strings.TrimSpace(deps.FrontendURL), "/")
	dispatcher := deps.Dispatcher
	if dispatcher == nil && deps.Mailer != nil {
		dispatcher = email.NewDispatcher(deps.Mailer)
	}

	return &guardianService{
		guardianProfileRepo:    deps.GuardianProfileRepo,
		studentGuardianRepo:    deps.StudentGuardianRepo,
		guardianInvitationRepo: deps.GuardianInvitationRepo,
		accountParentRepo:      deps.AccountParentRepo,
		studentRepo:            deps.StudentRepo,
		personRepo:             deps.PersonRepo,
		dispatcher:             dispatcher,
		frontendURL:            trimmedFrontend,
		defaultFrom:            deps.DefaultFrom,
		invitationExpiry:       deps.InvitationExpiry,
		db:                     deps.DB,
		txHandler:              base.NewTxHandler(deps.DB),
	}
}

// WithTx returns a new service instance with repositories bound to the transaction
func (s *guardianService) WithTx(tx bun.Tx) interface{} {
	var guardianProfileRepo = s.guardianProfileRepo
	var studentGuardianRepo = s.studentGuardianRepo
	var guardianInvitationRepo = s.guardianInvitationRepo
	var accountParentRepo = s.accountParentRepo
	var studentRepo = s.studentRepo
	var personRepo = s.personRepo

	if txRepo, ok := s.guardianProfileRepo.(base.TransactionalRepository); ok {
		guardianProfileRepo = txRepo.WithTx(tx).(users.GuardianProfileRepository)
	}
	if txRepo, ok := s.studentGuardianRepo.(base.TransactionalRepository); ok {
		studentGuardianRepo = txRepo.WithTx(tx).(users.StudentGuardianRepository)
	}
	if txRepo, ok := s.guardianInvitationRepo.(base.TransactionalRepository); ok {
		guardianInvitationRepo = txRepo.WithTx(tx).(authModels.GuardianInvitationRepository)
	}
	if txRepo, ok := s.accountParentRepo.(base.TransactionalRepository); ok {
		accountParentRepo = txRepo.WithTx(tx).(authModels.AccountParentRepository)
	}
	if txRepo, ok := s.studentRepo.(base.TransactionalRepository); ok {
		studentRepo = txRepo.WithTx(tx).(users.StudentRepository)
	}
	if txRepo, ok := s.personRepo.(base.TransactionalRepository); ok {
		personRepo = txRepo.WithTx(tx).(users.PersonRepository)
	}

	return &guardianService{
		guardianProfileRepo:    guardianProfileRepo,
		studentGuardianRepo:    studentGuardianRepo,
		guardianInvitationRepo: guardianInvitationRepo,
		accountParentRepo:      accountParentRepo,
		studentRepo:            studentRepo,
		personRepo:             personRepo,
		dispatcher:             s.dispatcher,
		frontendURL:            s.frontendURL,
		defaultFrom:            s.defaultFrom,
		invitationExpiry:       s.invitationExpiry,
		db:                     s.db,
		txHandler:              s.txHandler.WithTx(tx),
	}
}

// CreateGuardian creates a new guardian profile without an account
func (s *guardianService) CreateGuardian(ctx context.Context, req GuardianCreateRequest) (*users.GuardianProfile, error) {
	profile := &users.GuardianProfile{
		FirstName:              req.FirstName,
		LastName:               req.LastName,
		Email:                  req.Email,
		Phone:                  req.Phone,
		MobilePhone:            req.MobilePhone,
		AddressStreet:          req.AddressStreet,
		AddressCity:            req.AddressCity,
		AddressPostalCode:      req.AddressPostalCode,
		PreferredContactMethod: req.PreferredContactMethod,
		LanguagePreference:     req.LanguagePreference,
		Occupation:             req.Occupation,
		Employer:               req.Employer,
		Notes:                  req.Notes,
		HasAccount:             false,
	}

	// Set defaults if not provided
	if profile.PreferredContactMethod == "" {
		profile.PreferredContactMethod = "phone"
	}
	if profile.LanguagePreference == "" {
		profile.LanguagePreference = "de"
	}

	if err := s.guardianProfileRepo.Create(ctx, profile); err != nil {
		return nil, fmt.Errorf("failed to create guardian profile: %w", err)
	}

	return profile, nil
}

// CreateGuardianWithInvitation creates a guardian profile and sends an invitation
func (s *guardianService) CreateGuardianWithInvitation(ctx context.Context, req GuardianCreateRequest, createdBy int64) (*users.GuardianProfile, *authModels.GuardianInvitation, error) {
	// Validate email is provided for invitation
	if req.Email == nil || strings.TrimSpace(*req.Email) == "" {
		return nil, nil, fmt.Errorf("email is required to send invitation")
	}

	// Check if email already has an account
	if existingProfile, err := s.guardianProfileRepo.FindByEmail(ctx, *req.Email); err == nil && existingProfile.HasAccount {
		return nil, nil, fmt.Errorf("guardian with this email already has an account")
	}

	var profile *users.GuardianProfile
	var invitation *authModels.GuardianInvitation

	// Run in transaction
	err := s.txHandler.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		svc := s.WithTx(tx).(*guardianService)

		// Create guardian profile
		var err error
		profile, err = svc.CreateGuardian(ctx, req)
		if err != nil {
			return err
		}

		// Create invitation
		invitationReq := GuardianInvitationRequest{
			GuardianProfileID: profile.ID,
			CreatedBy:         createdBy,
		}
		invitation, err = svc.SendInvitation(ctx, invitationReq)
		if err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return nil, nil, err
	}

	return profile, invitation, nil
}

// GetGuardianByID retrieves a guardian profile by ID
func (s *guardianService) GetGuardianByID(ctx context.Context, id int64) (*users.GuardianProfile, error) {
	return s.guardianProfileRepo.FindByID(ctx, id)
}

// GetGuardianByEmail retrieves a guardian profile by email
func (s *guardianService) GetGuardianByEmail(ctx context.Context, email string) (*users.GuardianProfile, error) {
	return s.guardianProfileRepo.FindByEmail(ctx, email)
}

// UpdateGuardian updates a guardian profile
func (s *guardianService) UpdateGuardian(ctx context.Context, id int64, req GuardianCreateRequest) error {
	profile, err := s.guardianProfileRepo.FindByID(ctx, id)
	if err != nil {
		return err
	}

	// Update fields
	profile.FirstName = req.FirstName
	profile.LastName = req.LastName
	profile.Email = req.Email
	profile.Phone = req.Phone
	profile.MobilePhone = req.MobilePhone
	profile.AddressStreet = req.AddressStreet
	profile.AddressCity = req.AddressCity
	profile.AddressPostalCode = req.AddressPostalCode
	profile.Occupation = req.Occupation
	profile.Employer = req.Employer
	profile.Notes = req.Notes

	if req.PreferredContactMethod != "" {
		profile.PreferredContactMethod = req.PreferredContactMethod
	}
	if req.LanguagePreference != "" {
		profile.LanguagePreference = req.LanguagePreference
	}

	return s.guardianProfileRepo.Update(ctx, profile)
}

// DeleteGuardian removes a guardian profile
func (s *guardianService) DeleteGuardian(ctx context.Context, id int64) error {
	// CASCADE will handle student_guardians relationships
	return s.guardianProfileRepo.Delete(ctx, id)
}

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

	// Calculate expiry in hours
	expiryHours := int(s.invitationExpiry.Hours())

	// TODO: Get student names for this guardian
	// For now, just send basic invitation

	message := email.Message{
		From:     s.defaultFrom,
		To:       email.NewEmail("", *profile.Email),
		Subject:  "Einladung zum Eltern-Portal",
		Template: "guardian-invitation.html",
		Content: map[string]interface{}{
			"FirstName":     profile.FirstName,
			"LastName":      profile.LastName,
			"InvitationURL": invitationURL,
			"ExpiryHours":   expiryHours,
			"LogoURL":       fmt.Sprintf("%s/logo.png", s.frontendURL),
		},
	}

	meta := email.DeliveryMetadata{
		Type:        "guardian_invitation",
		ReferenceID: invitation.ID,
		Token:       invitation.Token,
		Recipient:   *profile.Email,
	}

	if s.dispatcher != nil {
		s.dispatcher.Dispatch(email.DeliveryRequest{
			Message:  message,
			Metadata: meta,
		})
	}

	// Update email status
	now := time.Now()
	_ = s.guardianInvitationRepo.UpdateEmailStatus(context.Background(), invitation.ID, &now, nil, 0)
}

// ValidateInvitation validates an invitation token
func (s *guardianService) ValidateInvitation(ctx context.Context, token string) (*GuardianInvitationValidationResult, error) {
	invitation, err := s.guardianInvitationRepo.FindByToken(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("invitation not found: %w", err)
	}

	if !invitation.IsValid() {
		if invitation.IsExpired() {
			return nil, fmt.Errorf("invitation has expired")
		}
		if invitation.IsAccepted() {
			return nil, fmt.Errorf("invitation has already been accepted")
		}
		return nil, fmt.Errorf("invitation is no longer valid")
	}

	// Get guardian profile
	profile, err := s.guardianProfileRepo.FindByID(ctx, invitation.GuardianProfileID)
	if err != nil {
		return nil, fmt.Errorf(errMsgGuardianNotFound, err)
	}

	// Get student names
	relationships, err := s.studentGuardianRepo.FindByGuardianProfileID(ctx, profile.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get student relationships: %w", err)
	}

	studentNames := make([]string, 0)
	for _, rel := range relationships {
		student, err := s.studentRepo.FindByID(ctx, rel.StudentID)
		if err != nil {
			continue
		}
		person, err := s.personRepo.FindByID(ctx, student.PersonID)
		if err != nil {
			continue
		}
		studentNames = append(studentNames, person.GetFullName())
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
	passwordHash, err := userpass.HashPassword(password, nil)
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

// GetStudentGuardians retrieves all guardians for a student
func (s *guardianService) GetStudentGuardians(ctx context.Context, studentID int64) ([]*GuardianWithRelationship, error) {
	relationships, err := s.studentGuardianRepo.FindByStudentID(ctx, studentID)
	if err != nil {
		return nil, err
	}

	result := make([]*GuardianWithRelationship, 0, len(relationships))
	for _, rel := range relationships {
		profile, err := s.guardianProfileRepo.FindByID(ctx, rel.GuardianProfileID)
		if err != nil {
			continue // Skip if profile not found
		}

		result = append(result, &GuardianWithRelationship{
			Profile:      profile,
			Relationship: rel,
		})
	}

	return result, nil
}

// GetGuardianStudents retrieves all students for a guardian
func (s *guardianService) GetGuardianStudents(ctx context.Context, guardianProfileID int64) ([]*StudentWithRelationship, error) {
	relationships, err := s.studentGuardianRepo.FindByGuardianProfileID(ctx, guardianProfileID)
	if err != nil {
		return nil, err
	}

	result := make([]*StudentWithRelationship, 0, len(relationships))
	for _, rel := range relationships {
		student, err := s.studentRepo.FindByID(ctx, rel.StudentID)
		if err != nil {
			continue // Skip if student not found
		}

		result = append(result, &StudentWithRelationship{
			Student:      student,
			Relationship: rel,
		})
	}

	return result, nil
}

// LinkGuardianToStudent creates a relationship between guardian and student
func (s *guardianService) LinkGuardianToStudent(ctx context.Context, req StudentGuardianCreateRequest) (*users.StudentGuardian, error) {
	// Validate guardian profile exists
	if _, err := s.guardianProfileRepo.FindByID(ctx, req.GuardianProfileID); err != nil {
		return nil, fmt.Errorf(errMsgGuardianNotFound, err)
	}

	// Validate student exists
	if _, err := s.studentRepo.FindByID(ctx, req.StudentID); err != nil {
		return nil, fmt.Errorf("student not found: %w", err)
	}

	// Create relationship
	relationship := &users.StudentGuardian{
		StudentID:          req.StudentID,
		GuardianProfileID:  req.GuardianProfileID,
		RelationshipType:   req.RelationshipType,
		IsPrimary:          req.IsPrimary,
		IsEmergencyContact: req.IsEmergencyContact,
		CanPickup:          req.CanPickup,
		PickupNotes:        req.PickupNotes,
		EmergencyPriority:  req.EmergencyPriority,
	}

	if err := s.studentGuardianRepo.Create(ctx, relationship); err != nil {
		return nil, fmt.Errorf("failed to create relationship: %w", err)
	}

	return relationship, nil
}

// GetStudentGuardianRelationship retrieves a student-guardian relationship by ID
func (s *guardianService) GetStudentGuardianRelationship(ctx context.Context, relationshipID int64) (*users.StudentGuardian, error) {
	relationship, err := s.studentGuardianRepo.FindByID(ctx, relationshipID)
	if err != nil {
		return nil, fmt.Errorf("relationship not found: %w", err)
	}
	return relationship, nil
}

// UpdateStudentGuardianRelationship updates a student-guardian relationship
func (s *guardianService) UpdateStudentGuardianRelationship(ctx context.Context, relationshipID int64, req StudentGuardianUpdateRequest) error {
	relationship, err := s.studentGuardianRepo.FindByID(ctx, relationshipID)
	if err != nil {
		return fmt.Errorf("relationship not found: %w", err)
	}

	// Update fields if provided
	if req.RelationshipType != nil {
		relationship.RelationshipType = *req.RelationshipType
	}
	if req.IsPrimary != nil {
		relationship.IsPrimary = *req.IsPrimary
	}
	if req.IsEmergencyContact != nil {
		relationship.IsEmergencyContact = *req.IsEmergencyContact
	}
	if req.CanPickup != nil {
		relationship.CanPickup = *req.CanPickup
	}
	if req.PickupNotes != nil {
		relationship.PickupNotes = req.PickupNotes
	}
	if req.EmergencyPriority != nil {
		relationship.EmergencyPriority = *req.EmergencyPriority
	}

	return s.studentGuardianRepo.Update(ctx, relationship)
}

// RemoveGuardianFromStudent removes a guardian from a student
func (s *guardianService) RemoveGuardianFromStudent(ctx context.Context, studentID, guardianProfileID int64) error {
	// Find the relationship
	relationships, err := s.studentGuardianRepo.FindByStudentID(ctx, studentID)
	if err != nil {
		return err
	}

	for _, rel := range relationships {
		if rel.GuardianProfileID == guardianProfileID {
			return s.studentGuardianRepo.Delete(ctx, rel.ID)
		}
	}

	return errors.New("relationship not found")
}

// ListGuardians retrieves guardians with pagination and filters
func (s *guardianService) ListGuardians(ctx context.Context, options *base.QueryOptions) ([]*users.GuardianProfile, error) {
	return s.guardianProfileRepo.ListWithOptions(ctx, options)
}

// GetGuardiansWithoutAccount retrieves guardians who don't have portal accounts
func (s *guardianService) GetGuardiansWithoutAccount(ctx context.Context) ([]*users.GuardianProfile, error) {
	return s.guardianProfileRepo.FindWithoutAccount(ctx)
}

// GetInvitableGuardians retrieves guardians who can be invited
func (s *guardianService) GetInvitableGuardians(ctx context.Context) ([]*users.GuardianProfile, error) {
	return s.guardianProfileRepo.FindInvitable(ctx)
}

// GetPendingInvitations retrieves all pending guardian invitations
func (s *guardianService) GetPendingInvitations(ctx context.Context) ([]*authModels.GuardianInvitation, error) {
	return s.guardianInvitationRepo.FindPending(ctx)
}

// CleanupExpiredInvitations deletes expired invitations
func (s *guardianService) CleanupExpiredInvitations(ctx context.Context) (int, error) {
	return s.guardianInvitationRepo.DeleteExpired(ctx)
}
