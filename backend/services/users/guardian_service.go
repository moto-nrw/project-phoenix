package users

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/gofrs/uuid"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/userpass"
	"github.com/moto-nrw/project-phoenix/email"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	authService "github.com/moto-nrw/project-phoenix/services/auth"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/driver/pgdriver"
)

const (
	// errMsgGuardianNotFound is the error message format for guardian profile not found errors
	errMsgGuardianNotFound = "guardian profile not found: %w"
	// errMsgPhoneNotFound is the error message format for phone number not found errors
	errMsgPhoneNotFound = "phone number not found: %w"
	// emailInUseMsgFmt is the user-facing German message (with a %q placeholder
	// for the email) returned as a 400 when a guardian email collides with an
	// existing profile. Single source of truth so the wording stays identical
	// across the create, update, and batch-validate paths (#819).
	emailInUseMsgFmt = "E-Mail-Adresse %q ist bereits vergeben – bitte die vorhandene Person über die Suche auswählen"
)

// newEmailInUseError builds the 400 ValidationError for a duplicate guardian
// email. email is trimmed for display so the message matches what the unique
// pre-check compared.
func newEmailInUseError(email string) *ValidationError {
	//nolint:staticcheck // ST1005: user-facing German message rendered in the 400 response
	return &ValidationError{Err: fmt.Errorf(emailInUseMsgFmt, strings.TrimSpace(email))}
}

// GuardianServiceDependencies contains all dependencies required by the guardian service
type GuardianServiceDependencies struct {
	// Repository dependencies
	GuardianProfileRepo     users.GuardianProfileRepository
	GuardianPhoneNumberRepo users.GuardianPhoneNumberRepository
	StudentGuardianRepo     users.StudentGuardianRepository
	GuardianInvitationRepo  authModels.GuardianInvitationRepository
	AccountRepo             authModels.AccountRepository
	AccountParentRepo       authModels.AccountParentRepository
	AccountTenantRepo       authModels.AccountTenantRepository
	AccountRoleRepo         authModels.AccountRoleRepository
	RoleRepo                authModels.RoleRepository
	StudentRepo             users.StudentRepository
	PersonRepo              users.PersonRepository

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
	guardianProfileRepo     users.GuardianProfileRepository
	guardianPhoneNumberRepo users.GuardianPhoneNumberRepository
	studentGuardianRepo     users.StudentGuardianRepository
	guardianInvitationRepo  authModels.GuardianInvitationRepository
	accountRepo             authModels.AccountRepository
	accountParentRepo       authModels.AccountParentRepository
	accountTenantRepo       authModels.AccountTenantRepository
	accountRoleRepo         authModels.AccountRoleRepository
	roleRepo                authModels.RoleRepository
	studentRepo             users.StudentRepository
	personRepo              users.PersonRepository
	dispatcher              *email.Dispatcher
	frontendURL             string
	defaultFrom             email.Email
	invitationExpiry        time.Duration
	db                      *bun.DB
	txHandler               *base.TxHandler
}

// NewGuardianService creates a new GuardianService instance
func NewGuardianService(deps GuardianServiceDependencies) GuardianService {
	trimmedFrontend := strings.TrimRight(strings.TrimSpace(deps.FrontendURL), "/")
	dispatcher := deps.Dispatcher
	if dispatcher == nil && deps.Mailer != nil {
		dispatcher = email.NewDispatcher(deps.Mailer, slog.Default().With("component", "email"))
	}

	return &guardianService{
		guardianProfileRepo:     deps.GuardianProfileRepo,
		guardianPhoneNumberRepo: deps.GuardianPhoneNumberRepo,
		studentGuardianRepo:     deps.StudentGuardianRepo,
		guardianInvitationRepo:  deps.GuardianInvitationRepo,
		accountRepo:             deps.AccountRepo,
		accountParentRepo:       deps.AccountParentRepo,
		accountTenantRepo:       deps.AccountTenantRepo,
		accountRoleRepo:         deps.AccountRoleRepo,
		roleRepo:                deps.RoleRepo,
		studentRepo:             deps.StudentRepo,
		personRepo:              deps.PersonRepo,
		dispatcher:              dispatcher,
		frontendURL:             trimmedFrontend,
		defaultFrom:             deps.DefaultFrom,
		invitationExpiry:        deps.InvitationExpiry,
		db:                      deps.DB,
		txHandler:               base.NewTxHandler(deps.DB),
	}
}

// CreateGuardian creates a new guardian profile without an account
// Note: Phone numbers should be added separately via AddPhoneNumber
func (s *guardianService) CreateGuardian(ctx context.Context, req GuardianCreateRequest) (*users.GuardianProfile, error) {
	profile := &users.GuardianProfile{
		FirstName:              req.FirstName,
		LastName:               req.LastName,
		Email:                  req.Email,
		AddressStreet:          req.AddressStreet,
		AddressCity:            req.AddressCity,
		AddressPostalCode:      req.AddressPostalCode,
		PreferredContactMethod: req.PreferredContactMethod,
		LanguagePreference:     req.LanguagePreference,
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

	profile.SetTenantID(tenant.FromContext(ctx))

	// Reject a duplicate email up front. The tenant-scoped UNIQUE(tenant_id,
	// email) index forbids two guardians sharing an email, so without this
	// pre-check the INSERT fails with a raw 23505 that surfaces to the user as a
	// generic 500. Detect it as bad input instead and steer the caller to the
	// existing-guardian picker (#1513) — the same German message and 400
	// semantics as the batch student-create path (ValidateNewGuardians).
	// FindByEmail matches case-insensitively (LOWER(email)), so it is at least
	// as strict as the index and never lets a colliding email slip through.
	if profile.Email != nil && strings.TrimSpace(*profile.Email) != "" {
		if existing, err := s.guardianProfileRepo.FindByEmail(ctx, strings.TrimSpace(*profile.Email)); err == nil && existing != nil {
			return nil, newEmailInUseError(*profile.Email)
		}
	}

	if err := s.guardianProfileRepo.Create(ctx, profile); err != nil {
		// TOCTOU guard: two concurrent creates can both pass the FindByEmail
		// pre-check above and then race on the UNIQUE(tenant_id, email) index.
		// The loser gets a raw 23505 — classify it as the same bad-input
		// ValidationError (HTTP 400) the pre-check produces, never a 500.
		if isGuardianUniqueViolation(err) && profile.Email != nil {
			return nil, newEmailInUseError(*profile.Email)
		}
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

	profile, err := s.CreateGuardian(ctx, req)
	if err != nil {
		return nil, nil, err
	}

	invitationReq := GuardianInvitationRequest{
		GuardianProfileID: profile.ID,
		CreatedBy:         createdBy,
	}
	invitation, err := s.SendInvitation(ctx, invitationReq)
	if err != nil {
		return nil, nil, err
	}

	return profile, invitation, nil
}

// GetGuardianByID retrieves a guardian profile by ID with phone numbers
func (s *guardianService) GetGuardianByID(ctx context.Context, id int64) (*users.GuardianProfile, error) {
	profile, err := s.guardianProfileRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Load phone numbers for this guardian
	phoneNumbers, err := s.guardianPhoneNumberRepo.FindByGuardianID(ctx, profile.ID)
	if err == nil {
		profile.PhoneNumbers = phoneNumbers
	}

	return profile, nil
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

	// Reject assigning an email already owned by a DIFFERENT guardian. Without
	// this the UNIQUE(tenant_id, email) index fires on Update and the handler
	// maps it to a raw 500. The existing.ID != id guard is essential: saving the
	// guardian without changing the email must NOT collide with the row itself.
	// FindByEmail is case-insensitive (LOWER=LOWER), matching CreateGuardian.
	if req.Email != nil && strings.TrimSpace(*req.Email) != "" {
		if existing, ferr := s.guardianProfileRepo.FindByEmail(ctx, strings.TrimSpace(*req.Email)); ferr == nil && existing != nil && existing.ID != id {
			return newEmailInUseError(*req.Email)
		}
	}

	// Update fields (phone numbers are managed separately)
	profile.FirstName = req.FirstName
	profile.LastName = req.LastName
	profile.Email = req.Email
	profile.AddressStreet = req.AddressStreet
	profile.AddressCity = req.AddressCity
	profile.AddressPostalCode = req.AddressPostalCode
	profile.Notes = req.Notes

	if req.PreferredContactMethod != "" {
		profile.PreferredContactMethod = req.PreferredContactMethod
	}
	if req.LanguagePreference != "" {
		profile.LanguagePreference = req.LanguagePreference
	}

	if err := s.guardianProfileRepo.Update(ctx, profile); err != nil {
		// TOCTOU guard mirroring CreateGuardian: a concurrent writer can claim
		// the email between the check above and this Update.
		if isGuardianUniqueViolation(err) && profile.Email != nil {
			return newEmailInUseError(*profile.Email)
		}
		return err
	}
	return nil
}

// DeleteGuardian removes a guardian profile WITHOUT touching its student links.
//
// Since migration 1.15.127 the students_guardians → guardian_profiles FK is
// ON DELETE RESTRICT, so this fails with a foreign-key violation when the
// guardian is still linked to any student — the handler turns that into a 409.
// Use this only for guardians with no remaining links; for the deliberate
// full delete use DeleteGuardianWithLinks.
func (s *guardianService) DeleteGuardian(ctx context.Context, id int64) error {
	return s.guardianProfileRepo.Delete(ctx, id)
}

// DeleteGuardianWithLinks deletes a guardian together with all of its
// student↔guardian links. Under the RESTRICT FK (1.15.127) the link rows MUST
// be removed before the guardian, so order matters. The caller is expected to
// run this inside a tenant transaction (the HTTP handler does, via
// WithTenantTx) so the two steps commit atomically — a mid-way failure rolls
// back and leaves the guardian and its links intact.
//
// This is the "Komplett löschen" path from #819 and is gated to admins at the
// handler, because it reaches across every linked student — including siblings
// in groups the caller may not supervise.
func (s *guardianService) DeleteGuardianWithLinks(ctx context.Context, id int64, expectedLinkIDs []int64) error {
	if err := s.guardianProfileRepo.LockByIDForUpdate(ctx, id); err != nil {
		return fmt.Errorf("failed to lock guardian profile %d: %w", id, err)
	}

	links, err := s.studentGuardianRepo.FindByGuardianProfileID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to load guardian links: %w", err)
	}
	currentLinkIDs := make([]int64, 0, len(links))
	for _, link := range links {
		currentLinkIDs = append(currentLinkIDs, link.ID)
	}
	if !sameInt64Set(currentLinkIDs, expectedLinkIDs) {
		return ErrGuardianDeletePreviewChanged
	}
	for _, link := range links {
		if err := s.studentGuardianRepo.Delete(ctx, link.ID); err != nil {
			return fmt.Errorf("failed to remove guardian link %d: %w", link.ID, err)
		}
	}
	return s.guardianProfileRepo.Delete(ctx, id)
}

// GetLinkedStudentNames returns the full names of every student currently
// linked to the guardian. Backs the delete-confirmation flow: an empty slice
// means a plain delete is safe, a non-empty one drives the 409 warning that
// lists exactly which children (incl. siblings) would lose the guardian.
func (s *guardianService) GetLinkedStudentNames(ctx context.Context, guardianProfileID int64) ([]string, error) {
	impact, err := s.GetGuardianDeleteImpact(ctx, guardianProfileID)
	if err != nil {
		return nil, err
	}
	return impact.StudentNames, nil
}

// GetGuardianDeleteImpact returns the exact current affected links and display
// names for a full guardian delete preview.
func (s *guardianService) GetGuardianDeleteImpact(ctx context.Context, guardianProfileID int64) (*GuardianDeleteImpact, error) {
	return s.getGuardianDeleteImpact(ctx, guardianProfileID)
}

func sameInt64Set(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}

	aCopy := slices.Clone(a)
	bCopy := slices.Clone(b)
	slices.Sort(aCopy)
	slices.Sort(bCopy)
	return slices.Equal(aCopy, bCopy)
}

// isGuardianUniqueViolation reports whether err (or a wrapped DatabaseError)
// carries PostgreSQL code 23505 (unique_violation) — used to classify a raced
// duplicate-email insert/update as bad input rather than a 500.
func isGuardianUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	var dbErr *base.DatabaseError
	if errors.As(err, &dbErr) {
		err = dbErr.Err
	}
	var pgErr pgdriver.Error
	if errors.As(err, &pgErr) {
		return pgErr.IntegrityViolation() && pgErr.Field('C') == "23505"
	}
	return false
}

// SendInvitation sends an invitation to a guardian.
//
// Deprecated: Replaced by services/auth.GuardianInvitationService.Create
// (parent-enrollment PR 3). Frontend does not call this. Cleanup PR pending.
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
	now := time.Now()
	for _, inv := range existingInvitations {
		if authService.GuardianInvitationValid(inv, now) {
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
	invitation.SetTenantID(tenant.FromContext(ctx))

	if err := s.guardianInvitationRepo.Create(ctx, invitation); err != nil {
		return nil, fmt.Errorf("failed to create invitation: %w", err)
	}

	// Send invitation email asynchronously, pass tenant context for DB calls
	if s.dispatcher != nil && profile.Email != nil {
		tenantCtx := tenant.WithTenantID(context.Background(), tenant.FromContext(ctx))
		go s.sendInvitationEmail(tenantCtx, invitation, profile)
	}

	return invitation, nil
}

// sendInvitationEmail sends the invitation email (called asynchronously).
// ctx should carry tenant context but NOT a transaction (use tenant.WithTenantID on Background).
func (s *guardianService) sendInvitationEmail(ctx context.Context, invitation *authModels.GuardianInvitation, profile *users.GuardianProfile) {
	if s.dispatcher == nil || profile.Email == nil {
		return
	}

	invitationURL := fmt.Sprintf("%s/guardian/invite?token=%s", s.frontendURL, invitation.Token)
	expiryHours := int(s.invitationExpiry.Hours())

	// P2 FIX: Handle errors gracefully in async email context
	// If we can't load student names, log the error but continue with empty list
	// (better to send the invitation without student names than to fail completely)
	studentNames, err := s.getStudentNamesForGuardian(ctx, profile.ID)
	if err != nil {
		slog.Warn("failed to load student names for guardian invitation email",
			slog.Int64("guardian_id", profile.ID),
			slog.String("error", err.Error()),
		)
		studentNames = []string{} // Use empty list as fallback
	}

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
			"StudentNames":  studentNames,
		},
	}

	meta := email.DeliveryMetadata{
		Type:        "guardian_invitation",
		ReferenceID: invitation.ID,
		Token:       invitation.Token,
		Recipient:   *profile.Email,
	}

	if s.dispatcher != nil {
		s.dispatcher.Dispatch(ctx, email.DeliveryRequest{
			Message:  message,
			Metadata: meta,
		})
	}

	// Update email status
	now := time.Now()
	_ = s.guardianInvitationRepo.UpdateEmailStatus(ctx, invitation.ID, &now, nil, 0)
}

// getStudentNamesForGuardian retrieves the full names of all students linked to a guardian
// Returns an error if the guardian-student relationships cannot be loaded or if any student/person
// lookup fails. This ensures callers can distinguish between "no students" and "data retrieval failure".
func (s *guardianService) getStudentNamesForGuardian(ctx context.Context, guardianProfileID int64) ([]string, error) {
	impact, err := s.getGuardianDeleteImpact(ctx, guardianProfileID)
	if err != nil {
		return nil, err
	}
	return impact.StudentNames, nil
}

// getGuardianDeleteImpact retrieves the link IDs and full names of all students
// linked to a guardian from the same relationship snapshot.
func (s *guardianService) getGuardianDeleteImpact(ctx context.Context, guardianProfileID int64) (*GuardianDeleteImpact, error) {
	relationships, err := s.studentGuardianRepo.FindByGuardianProfileID(ctx, guardianProfileID)
	if err != nil {
		return nil, fmt.Errorf("failed to load guardian-student relationships: %w", err)
	}

	linkIDs := make([]int64, 0, len(relationships))
	studentNames := make([]string, 0, len(relationships))
	for _, rel := range relationships {
		linkIDs = append(linkIDs, rel.ID)
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

	return &GuardianDeleteImpact{
		LinkIDs:      linkIDs,
		StudentNames: studentNames,
	}, nil
}

// ValidateInvitation validates an invitation token.
//
// Deprecated: Replaced by services/auth.GuardianInvitationService.Validate
// (parent-enrollment PR 3). Frontend does not call this. Cleanup PR pending.
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

// AcceptInvitation accepts an invitation and creates a guardian account.
//
// Deprecated: This flow writes to the orphaned auth.accounts_parents table
// and is unused by the frontend. The parent-enrollment plan replaces this
// with services/auth.GuardianInvitationService (PR 3, 2026-04). Will be
// removed in a separate cleanup PR once /api/guardians/invitations/...
// routes are confirmed unused. New code should not call this method.
func (s *guardianService) AcceptInvitation(ctx context.Context, req GuardianInvitationAcceptRequest) (*authModels.AccountParent, error) {
	if err := s.validateInvitationAcceptRequest(req); err != nil {
		return nil, err
	}

	invitation, profile, err := s.validateInvitationAndProfile(ctx, req.Token)
	if err != nil {
		return nil, err
	}

	var account *authModels.AccountParent
	tenantCtx := tenant.WithTenantID(ctx, invitation.TenantID)
	err = s.txHandler.RunInTx(tenantCtx, func(txCtx context.Context, _ bun.Tx) error {
		var innerErr error
		account, innerErr = s.createGuardianAccountFromInvitation(txCtx, profile, req.Password, invitation.TenantID)
		if innerErr != nil {
			return innerErr
		}
		return s.finalizeInvitationAcceptance(txCtx, invitation.ID, profile.ID, account.ID)
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
	now := time.Now()
	if authService.GuardianInvitationValid(invitation, now) {
		return nil
	}

	if authService.GuardianInvitationExpired(invitation, now) {
		return fmt.Errorf("invitation has expired")
	}

	if invitation.IsAccepted() {
		return fmt.Errorf("invitation has already been accepted")
	}

	return fmt.Errorf("invitation is no longer valid")
}

// createGuardianAccountFromInvitation creates a new guardian account with hashed password.
// tenantID is passed explicitly because guardian invitation acceptance is a public route
// where tenant.FromContext(ctx) would return 0.
func (s *guardianService) createGuardianAccountFromInvitation(ctx context.Context, profile *users.GuardianProfile, password string, tenantID int64) (*authModels.AccountParent, error) {
	passwordHash, err := userpass.HashPassword(password, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	emailAddress := strings.ToLower(strings.TrimSpace(*profile.Email))
	account, err := s.createOrUpdateGuardianAccount(ctx, emailAddress, passwordHash)
	if err != nil {
		return nil, err
	}

	if err := s.ensureGuardianTenantAccess(ctx, account.ID, tenantID); err != nil {
		return nil, err
	}

	legacyAccount := &authModels.AccountParent{
		Email:        account.Email,
		Username:     account.Username,
		PasswordHash: account.PasswordHash,
		Active:       account.Active,
	}
	legacyAccount.ID = account.ID
	legacyAccount.SetTenantID(tenantID)

	return legacyAccount, nil
}

func (s *guardianService) createOrUpdateGuardianAccount(ctx context.Context, emailAddress, passwordHash string) (*authModels.Account, error) {
	existing, err := s.accountRepo.FindByEmail(ctx, emailAddress)
	if err == nil && existing != nil {
		return existing, nil
	}
	if err != nil && !isGuardianServiceNotFound(err) {
		return nil, fmt.Errorf("failed to find account: %w", err)
	}

	account := &authModels.Account{
		Email:        emailAddress,
		PasswordHash: &passwordHash,
		Active:       true,
	}
	if err := s.accountRepo.Create(ctx, account); err != nil {
		return nil, fmt.Errorf("failed to create account: %w", err)
	}

	return account, nil
}

func (s *guardianService) ensureGuardianTenantAccess(ctx context.Context, accountID, tenantID int64) error {
	if err := s.ensureGuardianRole(ctx, accountID, tenantID); err != nil {
		return err
	}

	now := time.Now()
	mapping := &authModels.AccountTenant{
		AccountID:   accountID,
		TenantID:    tenantID,
		Status:      authModels.AccountTenantStatusActive,
		ActivatedAt: &now,
	}
	if err := s.accountTenantRepo.EnsureActive(ctx, mapping); err != nil {
		return fmt.Errorf("failed to link account to tenant: %w", err)
	}

	return nil
}

func (s *guardianService) ensureGuardianRole(ctx context.Context, accountID, tenantID int64) error {
	role, err := s.roleRepo.FindByName(ctx, authModels.BaseRoleGuardian)
	if err != nil {
		return fmt.Errorf("failed to find guardian role: %w", err)
	}
	if role == nil {
		return fmt.Errorf("guardian role not found")
	}

	existingRole, err := s.accountRoleRepo.FindByAccountAndRole(ctx, accountID, role.ID)
	if err == nil && existingRole != nil {
		return nil
	}
	if err != nil && !isGuardianServiceNotFound(err) {
		return fmt.Errorf("failed to find account role: %w", err)
	}

	accountRole := &authModels.AccountRole{AccountID: accountID, RoleID: role.ID}
	accountRole.SetTenantID(tenantID)
	if err := s.accountRoleRepo.Create(ctx, accountRole); err != nil {
		return fmt.Errorf("failed to assign guardian role: %w", err)
	}

	return nil
}

func isGuardianServiceNotFound(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "not found") || strings.Contains(message, "no rows")
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

		// Load phone numbers for this guardian
		phoneNumbers, err := s.guardianPhoneNumberRepo.FindByGuardianID(ctx, profile.ID)
		if err == nil {
			profile.PhoneNumbers = phoneNumbers
		}

		result = append(result, &GuardianWithRelationship{
			Profile:           profile,
			Relationship:      rel,
			InvitationPending: !profile.HasAccount && s.hasOpenInvitation(ctx, profile.ID),
		})
	}

	return result, nil
}

// hasOpenInvitation reports whether the guardian profile has an invitation that
// is neither accepted, expired, nor rejected — i.e. an outstanding invite the
// staff UI should surface as "Einladung offen". Best-effort: a lookup error is
// treated as "no pending invite" so the guardian list still renders.
func (s *guardianService) hasOpenInvitation(ctx context.Context, guardianProfileID int64) bool {
	invitations, err := s.guardianInvitationRepo.FindByGuardianProfileID(ctx, guardianProfileID)
	if err != nil {
		return false
	}
	now := time.Now()
	for _, inv := range invitations {
		if inv.IsAccepted() {
			continue
		}
		if !inv.ExpiresAt.IsZero() && now.After(inv.ExpiresAt) {
			continue
		}
		if inv.ApprovalStatus == authModels.GuardianInvitationApprovalRejected {
			continue
		}
		return true
	}
	return false
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

	role := req.GuardianRole
	if strings.TrimSpace(role) == "" {
		role = authorize.DefaultStudentGuardianRole(req.RelationshipType, req.IsPrimary, req.IsEmergencyContact, req.CanPickup)
	}

	// Build the new relationship.
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
	authorize.ApplyStudentGuardianRole(relationship, role)
	relationship.SetTenantID(tenant.FromContext(ctx))

	// Idempotent on an existing link: re-linking a guardian already attached to
	// this student is a pure NO-OP, never an error. LinkIfNotExists inserts via
	// ON CONFLICT DO NOTHING, so a duplicate — whether sequential, concurrent, or
	// retried — neither raises a 500 nor aborts the surrounding tenant
	// transaction (a raw duplicate INSERT would do both in PostgreSQL). This is
	// the same "re-selecting is not an error" semantics as the batch create path
	// in AddGuardiansToStudent.
	inserted, err := s.studentGuardianRepo.LinkIfNotExists(ctx, relationship)
	if err != nil {
		return nil, fmt.Errorf("failed to create relationship: %w", err)
	}
	if inserted {
		return relationship, nil
	}

	// The link already existed. Return it UNCHANGED — linking is a create, not an
	// edit. Changing an existing relationship's flags goes through the dedicated
	// update path (UpdateStudentGuardianRelationship, the detail page's "edit"
	// action). Never upserting the flags on a re-link keeps safety-relevant fields
	// (who can pick up the child, who the emergency contact is) mutable only via
	// that explicit endpoint, with no silent overwrite from a stray re-link.
	existingLinks, err := s.studentGuardianRepo.FindByStudentID(ctx, req.StudentID)
	if err != nil {
		return nil, fmt.Errorf("failed to load existing guardian link: %w", err)
	}
	for _, link := range existingLinks {
		if link.GuardianProfileID == req.GuardianProfileID {
			return link, nil
		}
	}

	return nil, fmt.Errorf("guardian link reported as existing but not found for student %d", req.StudentID)
}

// germanGuardianValidationMessage translates the English validation messages
// emitted by GuardianProfile.Validate() / GuardianPhoneNumber.Validate() into
// German for the user-facing 400 response. The model methods are shared with
// other call sites and must stay language-neutral, so the translation lives
// here. Unknown messages fall back to the original text rather than masking it.
func germanGuardianValidationMessage(err error) string {
	switch err.Error() {
	case "invalid email format":
		return "ungültiges E-Mail-Format"
	case "invalid preferred contact method":
		return "ungültige bevorzugte Kontaktmethode"
	case "phone number is required":
		return "Telefonnummer ist erforderlich"
	case "invalid phone number format":
		return "ungültiges Telefonnummer-Format"
	case "phone number must contain at least 3 digits":
		return "Telefonnummer muss mindestens 3 Ziffern enthalten"
	default:
		return err.Error()
	}
}

// ValidateNewGuardians validates guardian input without persisting anything.
// See the interface doc comment for why callers must run this before the first
// write. Every failure is returned as a *ValidationError (HTTP 400) carrying a
// user-facing German message.
func (s *guardianService) ValidateNewGuardians(ctx context.Context, guardians []NewStudentGuardian) error {
	// Track emails seen within this single request so two new guardians sharing
	// one email are rejected up front rather than colliding on insert.
	seenEmails := make(map[string]struct{}, len(guardians))

	for i := range guardians {
		// Select-existing path (#1513): the entry links an already-existing
		// profile instead of creating one. There is no profile/email/phone
		// input to validate — only that the profile exists (FindByID is
		// tenant-scoped via RLS, so a cross-tenant or stale id surfaces as a
		// clean 400, not a 500) and that the relationship fields are valid,
		// matching the new-guardian path below and the detail-page link route.
		if id := guardians[i].ExistingProfileID; id != nil {
			if _, err := s.guardianProfileRepo.FindByID(ctx, *id); err != nil {
				//nolint:staticcheck // ST1005: user-facing German message rendered in the 400 response
				return &ValidationError{Err: fmt.Errorf("Erziehungsberechtigte/r %d: ausgewählte Person nicht gefunden", i+1)}
			}
			if !users.IsValidRelationshipType(guardians[i].Relationship.RelationshipType) {
				//nolint:staticcheck // ST1005: user-facing German message rendered in the 400 response
				return &ValidationError{Err: fmt.Errorf("Erziehungsberechtigte/r %d: ungültiger Beziehungstyp", i+1)}
			}
			if guardians[i].Relationship.EmergencyPriority < 1 {
				//nolint:staticcheck // ST1005: user-facing German message rendered in the 400 response
				return &ValidationError{Err: fmt.Errorf("Erziehungsberechtigte/r %d: Notfall-Priorität muss mindestens 1 sein", i+1)}
			}
			continue
		}

		// Profile rules (email format, contact method) via the shared model
		// Validate() — the same checks the repository runs on insert.
		probe := &users.GuardianProfile{
			FirstName:              guardians[i].Profile.FirstName,
			LastName:               guardians[i].Profile.LastName,
			Email:                  guardians[i].Profile.Email,
			PreferredContactMethod: guardians[i].Profile.PreferredContactMethod,
		}
		if err := probe.Validate(); err != nil {
			//nolint:staticcheck // ST1005: user-facing German message rendered in the 400 response
			return &ValidationError{Err: fmt.Errorf("Erziehungsberechtigte/r %d: %s", i+1, germanGuardianValidationMessage(err))}
		}

		// Relationship type must be one of the allowed values. Without this,
		// an unsupported value passes Bind, then fails at insert as a plain
		// error (HTTP 500). users.IsValidRelationshipType is the single source
		// of truth shared with StudentGuardian.Validate() and the detail-page
		// link path, so the allowed set cannot drift across request paths.
		if !users.IsValidRelationshipType(guardians[i].Relationship.RelationshipType) {
			//nolint:staticcheck // ST1005: user-facing German message rendered in the 400 response
			return &ValidationError{Err: fmt.Errorf("Erziehungsberechtigte/r %d: ungültiger Beziehungstyp", i+1)}
		}

		// Emergency priority must be >= 1, matching the detail-page link
		// endpoint (StudentGuardianLinkRequest.Bind rejects < 1).
		if guardians[i].Relationship.EmergencyPriority < 1 {
			//nolint:staticcheck // ST1005: user-facing German message rendered in the 400 response
			return &ValidationError{Err: fmt.Errorf("Erziehungsberechtigte/r %d: Notfall-Priorität muss mindestens 1 sein", i+1)}
		}

		// Duplicate email: a sibling sharing a guardian email hits the
		// UNIQUE(tenant_id, email) index on insert and would otherwise surface
		// as a 500 and roll the whole student back. Detect it as bad input.
		// (Reusing the existing guardian profile is deferred to #1513.)
		if probe.Email != nil && *probe.Email != "" {
			email := *probe.Email // already trimmed + lowercased by probe.Validate()
			if _, dup := seenEmails[email]; dup {
				//nolint:staticcheck // ST1005: user-facing German message rendered in the 400 response
				return &ValidationError{Err: fmt.Errorf("Erziehungsberechtigte/r %d: E-Mail-Adresse %q ist mehrfach angegeben", i+1, email)}
			}
			if existing, err := s.guardianProfileRepo.FindByEmail(ctx, email); err == nil && existing != nil {
				//nolint:staticcheck // ST1005: user-facing German message rendered in the 400 response
				return &ValidationError{Err: fmt.Errorf("Erziehungsberechtigte/r %d: "+emailInUseMsgFmt, i+1, email)}
			}
			seenEmails[email] = struct{}{}
		}

		// Validate phone numbers the same way AddPhoneNumber does on insert.
		// GuardianProfileID is set to a placeholder (1) only to pass the model's
		// "ID required" guard — the real ID is assigned after CreateGuardian. The
		// phone type is coerced to mobile for unknown values exactly like
		// AddPhoneNumber, so the probe never rejects a type the real path accepts.
		for j := range guardians[i].PhoneNumbers {
			phoneType := users.PhoneType(guardians[i].PhoneNumbers[j].PhoneType)
			if !users.ValidPhoneTypes[phoneType] {
				phoneType = users.PhoneTypeMobile
			}
			phoneProbe := &users.GuardianPhoneNumber{
				GuardianProfileID: 1,
				PhoneNumber:       guardians[i].PhoneNumbers[j].PhoneNumber,
				PhoneType:         phoneType,
			}
			if err := phoneProbe.Validate(); err != nil {
				//nolint:staticcheck // ST1005: user-facing German message rendered in the 400 response
				return &ValidationError{Err: fmt.Errorf("Erziehungsberechtigte/r %d, Telefonnummer %d: %s", i+1, j+1, germanGuardianValidationMessage(err))}
			}
		}
	}

	return nil
}

// AddGuardiansToStudent creates each guardian profile, links it to the student,
// and adds its phone numbers. It reuses CreateGuardian, LinkGuardianToStudent,
// and AddPhoneNumber, all of which join the ambient tenant transaction via the
// context, so a failure on any guardian aborts the surrounding transaction and
// leaves no partial student/guardian data behind.
func (s *guardianService) AddGuardiansToStudent(ctx context.Context, studentID int64, guardians []NewStudentGuardian) error {
	// Defense-in-depth: re-validate before any write so this method is safe
	// even if a caller forgets the pre-write ValidateNewGuardians call. Bad
	// client input surfaces as a classified ValidationError (HTTP 400) instead
	// of a generic 500.
	if err := s.ValidateNewGuardians(ctx, guardians); err != nil {
		return err
	}

	// Track profile ids already linked to this student within this request so a
	// guardian selected (or typed) twice doesn't collide on the
	// UNIQUE(student_id, guardian_profile_id) index. Re-selecting the same
	// person is not an error — the duplicate link is skipped silently (#1513).
	linked := make(map[int64]struct{}, len(guardians))

	for i := range guardians {
		g := guardians[i]

		var profileID int64
		if g.ExistingProfileID != nil {
			// Select-existing path: link the existing profile as-is. Never
			// create it and never touch its phone numbers — the existing
			// profile's own data stays untouched.
			profileID = *g.ExistingProfileID
			if _, dup := linked[profileID]; dup {
				continue
			}
		} else {
			profile, err := s.CreateGuardian(ctx, g.Profile)
			if err != nil {
				return fmt.Errorf("failed to create guardian at index %d: %w", i, err)
			}
			profileID = profile.ID
		}

		// RelationshipType and EmergencyPriority were already checked by
		// ValidateNewGuardians (>= 1, allowed type) — parity with the
		// detail-page link path (LinkGuardianToStudent).
		if _, err := s.LinkGuardianToStudent(ctx, StudentGuardianCreateRequest{
			StudentID:          studentID,
			GuardianProfileID:  profileID,
			RelationshipType:   g.Relationship.RelationshipType,
			GuardianRole:       g.Relationship.GuardianRole,
			IsPrimary:          g.Relationship.IsPrimary,
			IsEmergencyContact: g.Relationship.IsEmergencyContact,
			CanPickup:          g.Relationship.CanPickup,
			PickupNotes:        g.Relationship.PickupNotes,
			EmergencyPriority:  g.Relationship.EmergencyPriority,
		}); err != nil {
			return fmt.Errorf("failed to link guardian at index %d: %w", i, err)
		}
		linked[profileID] = struct{}{}

		// Phone numbers only apply to newly created profiles. An existing
		// profile keeps its own phone numbers untouched.
		if g.ExistingProfileID == nil {
			for j := range g.PhoneNumbers {
				if _, err := s.AddPhoneNumber(ctx, profileID, g.PhoneNumbers[j]); err != nil {
					return fmt.Errorf("failed to add phone number %d for guardian at index %d: %w", j, i, err)
				}
			}
		}
	}

	return nil
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
	if req.GuardianRole != nil {
		authorize.ApplyStudentGuardianRole(relationship, *req.GuardianRole)
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

// ListGuardians retrieves guardians with pagination and filters, including phone numbers
func (s *guardianService) ListGuardians(ctx context.Context, options *base.QueryOptions) ([]*users.GuardianProfile, error) {
	profiles, err := s.guardianProfileRepo.ListWithOptions(ctx, options)
	if err != nil {
		return nil, err
	}

	// Load phone numbers for each guardian
	for _, profile := range profiles {
		phoneNumbers, err := s.guardianPhoneNumberRepo.FindByGuardianID(ctx, profile.ID)
		if err == nil {
			profile.PhoneNumbers = phoneNumbers
		}
	}

	return profiles, nil
}

// SearchGuardiansForPicker retrieves guardians matching the search text and
// enriches each with its linked children. It backs the guardian picker (link an
// existing guardian to a student). Tenant isolation is enforced by RLS on the
// ambient tenant transaction. Phone numbers are intentionally not loaded — the
// picker identifies people by name, email, and linked children.
//
// The linked children are fetched in ONE batch query for all matched guardians
// and grouped in memory, so the picker never falls into a per-guardian N+1.
func (s *guardianService) SearchGuardiansForPicker(ctx context.Context, searchText string, limit int) ([]*GuardianPickerMatch, error) {
	profiles, err := s.guardianProfileRepo.SearchByText(ctx, searchText, limit)
	if err != nil {
		return nil, err
	}
	if len(profiles) == 0 {
		return []*GuardianPickerMatch{}, nil
	}

	ids := make([]int64, len(profiles))
	for i, p := range profiles {
		ids[i] = p.ID
	}

	children, err := s.studentGuardianRepo.ListLinkedChildrenForGuardians(ctx, ids)
	if err != nil {
		return nil, err
	}

	byGuardian := make(map[int64][]*users.GuardianLinkedChild, len(profiles))
	for _, c := range children {
		byGuardian[c.GuardianProfileID] = append(byGuardian[c.GuardianProfileID], c)
	}

	matches := make([]*GuardianPickerMatch, len(profiles))
	for i, p := range profiles {
		matches[i] = &GuardianPickerMatch{Profile: p, Children: byGuardian[p.ID]}
	}
	return matches, nil
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

// ============================================================================
// Phone Number Management
// ============================================================================

// AddPhoneNumber adds a new phone number to a guardian
func (s *guardianService) AddPhoneNumber(ctx context.Context, guardianID int64, req PhoneNumberCreateRequest) (*users.GuardianPhoneNumber, error) {
	// Verify guardian exists
	if _, err := s.guardianProfileRepo.FindByID(ctx, guardianID); err != nil {
		return nil, fmt.Errorf(errMsgGuardianNotFound, err)
	}

	// Get current count to determine if this should be primary
	count, err := s.guardianPhoneNumberRepo.CountByGuardianID(ctx, guardianID)
	if err != nil {
		return nil, fmt.Errorf("failed to count existing phone numbers: %w", err)
	}

	// Get next priority
	priority, err := s.guardianPhoneNumberRepo.GetNextPriority(ctx, guardianID)
	if err != nil {
		return nil, fmt.Errorf("failed to get next priority: %w", err)
	}

	// Determine phone type
	phoneType := users.PhoneType(req.PhoneType)
	if !users.ValidPhoneTypes[phoneType] {
		phoneType = users.PhoneTypeMobile // Default to mobile
	}

	// If this is the first phone number or explicitly set as primary, make it primary
	isPrimary := req.IsPrimary || count == 0

	phone := &users.GuardianPhoneNumber{
		GuardianProfileID: guardianID,
		PhoneNumber:       req.PhoneNumber,
		PhoneType:         phoneType,
		Label:             req.Label,
		IsPrimary:         isPrimary,
		Priority:          priority,
	}
	phone.SetTenantID(tenant.FromContext(ctx))

	// If setting as primary, unset existing primaries first
	if isPrimary && count > 0 {
		if err := s.guardianPhoneNumberRepo.UnsetAllPrimary(ctx, guardianID); err != nil {
			return nil, fmt.Errorf("failed to unset existing primary: %w", err)
		}
		if err := s.guardianPhoneNumberRepo.Create(ctx, phone); err != nil {
			return nil, fmt.Errorf("failed to create phone number: %w", err)
		}
		return phone, nil
	}

	if err := s.guardianPhoneNumberRepo.Create(ctx, phone); err != nil {
		return nil, fmt.Errorf("failed to create phone number: %w", err)
	}

	return phone, nil
}

// UpdatePhoneNumber updates an existing phone number
func (s *guardianService) UpdatePhoneNumber(ctx context.Context, phoneID int64, req PhoneNumberUpdateRequest) error {
	phone, err := s.guardianPhoneNumberRepo.FindByID(ctx, phoneID)
	if err != nil {
		return fmt.Errorf(errMsgPhoneNotFound, err)
	}

	// Update fields if provided
	if req.PhoneNumber != nil {
		phone.PhoneNumber = *req.PhoneNumber
	}
	if req.PhoneType != nil {
		phoneType := users.PhoneType(*req.PhoneType)
		if users.ValidPhoneTypes[phoneType] {
			phone.PhoneType = phoneType
		}
	}
	if req.Label != nil {
		phone.Label = req.Label
	}
	if req.Priority != nil {
		phone.Priority = *req.Priority
	}

	// Handle primary flag change
	if req.IsPrimary != nil && *req.IsPrimary && !phone.IsPrimary {
		phone.IsPrimary = true
		if err := s.guardianPhoneNumberRepo.UnsetAllPrimary(ctx, phone.GuardianProfileID); err != nil {
			return fmt.Errorf("failed to unset existing primary: %w", err)
		}
		return s.guardianPhoneNumberRepo.Update(ctx, phone)
	} else if req.IsPrimary != nil && !*req.IsPrimary && phone.IsPrimary {
		// Unsetting primary - need to promote another number
		phone.IsPrimary = false
	}

	return s.guardianPhoneNumberRepo.Update(ctx, phone)
}

// DeletePhoneNumber removes a phone number
func (s *guardianService) DeletePhoneNumber(ctx context.Context, phoneID int64) error {
	phone, err := s.guardianPhoneNumberRepo.FindByID(ctx, phoneID)
	if err != nil {
		return fmt.Errorf(errMsgPhoneNotFound, err)
	}

	wasPrimary := phone.IsPrimary
	guardianID := phone.GuardianProfileID

	// Delete the phone number
	if err := s.guardianPhoneNumberRepo.Delete(ctx, phoneID); err != nil {
		return fmt.Errorf("failed to delete phone number: %w", err)
	}

	// If deleted number was primary, promote the next one
	if wasPrimary {
		phones, err := s.guardianPhoneNumberRepo.FindByGuardianID(ctx, guardianID)
		if err != nil {
			return nil // Deletion succeeded, just couldn't promote - not fatal
		}

		// Promote the first remaining phone number (already sorted by priority)
		if len(phones) > 0 {
			_ = s.guardianPhoneNumberRepo.SetPrimary(ctx, phones[0].ID, guardianID)
		}
	}

	return nil
}

// SetPrimaryPhone sets a phone number as the primary contact
func (s *guardianService) SetPrimaryPhone(ctx context.Context, phoneID int64) error {
	phone, err := s.guardianPhoneNumberRepo.FindByID(ctx, phoneID)
	if err != nil {
		return fmt.Errorf(errMsgPhoneNotFound, err)
	}

	return s.guardianPhoneNumberRepo.SetPrimary(ctx, phoneID, phone.GuardianProfileID)
}

// GetGuardianPhoneNumbers retrieves all phone numbers for a guardian, sorted by priority
func (s *guardianService) GetGuardianPhoneNumbers(ctx context.Context, guardianID int64) ([]*users.GuardianPhoneNumber, error) {
	return s.guardianPhoneNumberRepo.FindByGuardianID(ctx, guardianID)
}

// GetPhoneNumberByID retrieves a phone number by ID
func (s *guardianService) GetPhoneNumberByID(ctx context.Context, phoneID int64) (*users.GuardianPhoneNumber, error) {
	return s.guardianPhoneNumberRepo.FindByID(ctx, phoneID)
}
