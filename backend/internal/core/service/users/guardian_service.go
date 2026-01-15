package users

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	authModels "github.com/moto-nrw/project-phoenix/internal/core/domain/auth"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/base"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/users"
	"github.com/moto-nrw/project-phoenix/internal/core/port"
	authPort "github.com/moto-nrw/project-phoenix/internal/core/port/auth"
	userPort "github.com/moto-nrw/project-phoenix/internal/core/port/users"
	"github.com/uptrace/bun"
)

const (
	// errMsgGuardianNotFound is the error message format for guardian profile not found errors
	errMsgGuardianNotFound = "guardian profile not found: %w"
)

// GuardianServiceDependencies contains all dependencies required by the guardian service
type GuardianServiceDependencies struct {
	// Repository dependencies
	GuardianProfileRepo    userPort.GuardianProfileRepository
	StudentGuardianRepo    userPort.StudentGuardianRepository
	GuardianInvitationRepo authPort.GuardianInvitationRepository
	AccountParentRepo      authPort.AccountParentRepository
	StudentRepo            userPort.StudentRepository
	PersonRepo             userPort.PersonRepository

	// Email dependencies
	Dispatcher       port.EmailDispatcher
	FrontendURL      string
	DefaultFrom      port.EmailAddress
	InvitationExpiry time.Duration

	// Infrastructure
	DB *bun.DB
}

type guardianService struct {
	guardianProfileRepo    userPort.GuardianProfileRepository
	studentGuardianRepo    userPort.StudentGuardianRepository
	guardianInvitationRepo authPort.GuardianInvitationRepository
	accountParentRepo      authPort.AccountParentRepository
	studentRepo            userPort.StudentRepository
	personRepo             userPort.PersonRepository
	dispatcher             port.EmailDispatcher
	frontendURL            string
	defaultFrom            port.EmailAddress
	invitationExpiry       time.Duration
	db                     *bun.DB
	txHandler              *base.TxHandler
}

// NewGuardianService creates a new GuardianService instance
func NewGuardianService(deps GuardianServiceDependencies) GuardianService {
	trimmedFrontend := strings.TrimRight(strings.TrimSpace(deps.FrontendURL), "/")

	return &guardianService{
		guardianProfileRepo:    deps.GuardianProfileRepo,
		studentGuardianRepo:    deps.StudentGuardianRepo,
		guardianInvitationRepo: deps.GuardianInvitationRepo,
		accountParentRepo:      deps.AccountParentRepo,
		studentRepo:            deps.StudentRepo,
		personRepo:             deps.PersonRepo,
		dispatcher:             deps.Dispatcher,
		frontendURL:            trimmedFrontend,
		defaultFrom:            deps.DefaultFrom,
		invitationExpiry:       deps.InvitationExpiry,
		db:                     deps.DB,
		txHandler:              base.NewTxHandler(deps.DB),
	}
}

// WithTx returns a new service instance with repositories bound to the transaction
func (s *guardianService) WithTx(tx bun.Tx) any {
	return &guardianService{
		guardianProfileRepo:    base.WithTxIfSupported(s.guardianProfileRepo, tx),
		studentGuardianRepo:    base.WithTxIfSupported(s.studentGuardianRepo, tx),
		guardianInvitationRepo: base.WithTxIfSupported(s.guardianInvitationRepo, tx),
		accountParentRepo:      base.WithTxIfSupported(s.accountParentRepo, tx),
		studentRepo:            base.WithTxIfSupported(s.studentRepo, tx),
		personRepo:             base.WithTxIfSupported(s.personRepo, tx),
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
