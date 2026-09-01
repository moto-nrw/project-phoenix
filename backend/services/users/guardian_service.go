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
	"github.com/moto-nrw/project-phoenix/email"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	authService "github.com/moto-nrw/project-phoenix/services/auth"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
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

	// Payment data (#2608). Kept as separate, narrow dependencies so the
	// guardians:financial code path is the only one holding a reference to
	// the bank rows. Both audit writers are mandatory for that path: it
	// refuses to serve or change an IBAN when it cannot write the trail.
	GuardianFinancialRepo  users.GuardianFinancialDataRepository
	GuardianFinancialAudit auditModels.GuardianFinancialChangeCreator
	DataAccessLog          auditModels.DataAccessLogRepository

	// Email dependencies
	Mailer           email.Mailer
	Dispatcher       *email.Dispatcher
	FrontendURL      string
	DefaultFrom      email.Email
	InvitationExpiry time.Duration

	// MailIdentity points replies to this invitation at the OGS instead of
	// moto (#1936). Optional: nil sends without a Reply-To, exactly as before.
	// This send bypasses the outbox, so it stamps the header itself.
	MailIdentity email.ReplyToResolver

	// Infrastructure
	DB *bun.DB
}

// GuardianService manages guardian profiles, their student relationships,
// invitations, and phone numbers.
type GuardianService struct {
	GuardianServiceDependencies
	txHandler *tenant.TransactionRunner
}

// GuardianDisplay is the People Directory projection used outside this domain.
type GuardianDisplay struct {
	GuardianProfileID int64
	FirstName         string
	LastName          string
}

// NewGuardianService creates a new GuardianService instance
func NewGuardianService(deps GuardianServiceDependencies) *GuardianService {
	deps.FrontendURL = strings.TrimRight(strings.TrimSpace(deps.FrontendURL), "/")
	if deps.Dispatcher == nil && deps.Mailer != nil {
		deps.Dispatcher = email.NewDispatcher(deps.Mailer, slog.Default().With("component", "email"))
	}

	return &GuardianService{
		GuardianServiceDependencies: deps,
		txHandler:                   tenant.NewTransactionRunner(),
	}
}

// CreateGuardian creates a new guardian profile without an account
// Note: Phone numbers should be added separately via AddPhoneNumber
func (s *GuardianService) CreateGuardian(ctx context.Context, req GuardianCreateRequest) (*users.GuardianProfile, error) {
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
		if existing, err := s.GuardianProfileRepo.FindByEmail(ctx, strings.TrimSpace(*profile.Email)); err == nil && existing != nil {
			return nil, newEmailInUseError(*profile.Email)
		}
	}

	if err := s.GuardianProfileRepo.Create(ctx, profile); err != nil {
		// TOCTOU guard: two concurrent creates can both pass the FindByEmail
		// pre-check above and then race on the UNIQUE(tenant_id, email) index.
		// The loser gets a raw 23505 — classify it as the same bad-input
		// ValidationError (HTTP 400) the pre-check produces, never a 500.
		if base.IsUniqueViolation(err) && profile.Email != nil {
			return nil, newEmailInUseError(*profile.Email)
		}
		return nil, fmt.Errorf("failed to create guardian profile: %w", err)
	}

	return profile, nil
}

// CreateGuardianWithInvitation creates a guardian profile and sends an invitation
func (s *GuardianService) CreateGuardianWithInvitation(ctx context.Context, req GuardianCreateRequest, createdBy int64) (*users.GuardianProfile, *authModels.GuardianInvitation, error) {
	// Validate email is provided for invitation
	if req.Email == nil || strings.TrimSpace(*req.Email) == "" {
		return nil, nil, fmt.Errorf("email is required to send invitation")
	}

	// Check if email already has an account
	if existingProfile, err := s.GuardianProfileRepo.FindByEmail(ctx, *req.Email); err == nil && existingProfile.HasAccount {
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
func (s *GuardianService) GetGuardianByID(ctx context.Context, id int64) (*users.GuardianProfile, error) {
	profile, err := s.GuardianProfileRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Load phone numbers for this guardian
	phoneNumbers, err := s.GuardianPhoneNumberRepo.FindByGuardianID(ctx, profile.ID)
	if err == nil {
		profile.PhoneNumbers = phoneNumbers
	}

	return profile, nil
}

// GuardianDisplays resolves names without exposing People Directory models or
// persistence to consumers.
func (s *GuardianService) GuardianDisplays(ctx context.Context, ids []int64) ([]GuardianDisplay, error) {
	if len(ids) == 0 {
		return []GuardianDisplay{}, nil
	}
	profiles, err := s.GuardianProfileRepo.FindByIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("resolve guardian displays: %w", err)
	}
	displays := make([]GuardianDisplay, 0, len(profiles))
	for _, profile := range profiles {
		displays = append(displays, GuardianDisplay{
			GuardianProfileID: profile.ID,
			FirstName:         profile.FirstName,
			LastName:          profile.LastName,
		})
	}
	return displays, nil
}

// UpdateGuardian updates a guardian profile
func (s *GuardianService) UpdateGuardian(ctx context.Context, id int64, req GuardianCreateRequest) error {
	// Serialize all guardian contact writers on the profile row. The
	// parents-portal contact path (services/parent.UpdateGuardianContact) locks
	// this same row FOR UPDATE before its read-modify-write plus wholesale phone
	// replace; taking the lock here — BEFORE the read — makes this staff profile
	// edit serialize against it, so a stale-read full-row Update can't clobber the
	// parent's save and vice versa (#1667 review). The handler wraps this call in
	// WithTenantTx, so the lock is held for the whole request transaction; without
	// a transaction (CLI/seed) the SELECT FOR UPDATE is a harmless no-op.
	if err := s.GuardianProfileRepo.LockByIDForUpdate(ctx, id); err != nil {
		return err
	}
	profile, err := s.GuardianProfileRepo.FindByID(ctx, id)
	if err != nil {
		return err
	}

	// Reject assigning an email already owned by a DIFFERENT guardian. Without
	// this the UNIQUE(tenant_id, email) index fires on Update and the handler
	// maps it to a raw 500. The existing.ID != id guard is essential: saving the
	// guardian without changing the email must NOT collide with the row itself.
	// FindByEmail is case-insensitive (LOWER=LOWER), matching CreateGuardian.
	if req.Email != nil && strings.TrimSpace(*req.Email) != "" {
		if existing, ferr := s.GuardianProfileRepo.FindByEmail(ctx, strings.TrimSpace(*req.Email)); ferr == nil && existing != nil && existing.ID != id {
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

	if err := s.GuardianProfileRepo.Update(ctx, profile); err != nil {
		// TOCTOU guard mirroring CreateGuardian: a concurrent writer can claim
		// the email between the check above and this Update.
		if base.IsUniqueViolation(err) && profile.Email != nil {
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
func (s *GuardianService) DeleteGuardian(ctx context.Context, id, changedByAccountID int64) error {
	return s.txHandler.RunInTx(ctx, func(txCtx context.Context) error {
		return s.deleteGuardian(txCtx, id, changedByAccountID)
	})
}

func (s *GuardianService) deleteGuardian(ctx context.Context, id, changedByAccountID int64) error {
	if err := s.GuardianProfileRepo.LockByIDForUpdate(ctx, id); err != nil {
		return fmt.Errorf("failed to lock guardian profile %d: %w", id, err)
	}
	if err := s.auditGuardianFinancialDataDeletion(ctx, id, changedByAccountID); err != nil {
		return err
	}
	return s.GuardianProfileRepo.Delete(ctx, id)
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
// in groups the caller may not supervise. The service owns the transaction;
// an ambient handler transaction is joined rather than nested.
func (s *GuardianService) DeleteGuardianWithLinks(ctx context.Context, id int64, expectedLinkIDs []int64, changedByAccountID int64) error {
	return s.txHandler.RunInTx(ctx, func(txCtx context.Context) error {
		return s.deleteGuardianWithLinks(txCtx, id, expectedLinkIDs, changedByAccountID)
	})
}

func (s *GuardianService) deleteGuardianWithLinks(ctx context.Context, id int64, expectedLinkIDs []int64, changedByAccountID int64) error {
	if err := s.GuardianProfileRepo.LockByIDForUpdate(ctx, id); err != nil {
		return fmt.Errorf("failed to lock guardian profile %d: %w", id, err)
	}
	if err := s.auditGuardianFinancialDataDeletion(ctx, id, changedByAccountID); err != nil {
		return err
	}

	links, err := s.StudentGuardianRepo.FindByGuardianProfileID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to load guardian links: %w", err)
	}
	studentIDs := make([]int64, 0, len(links))
	for _, link := range links {
		studentIDs = append(studentIDs, link.StudentID)
	}
	if _, err := s.StudentRepo.FindByIDsForUpdate(ctx, studentIDs); err != nil {
		return fmt.Errorf("failed to lock guardian students: %w", err)
	}
	// Reload after acquiring the shared serialization points. A waiting payer
	// assignment now commits before this read, so its final is_payer state is
	// either audited below or cannot change until this delete commits.
	links, err = s.StudentGuardianRepo.FindByGuardianProfileID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to reload guardian links: %w", err)
	}
	currentLinkIDs := make([]int64, 0, len(links))
	for _, link := range links {
		currentLinkIDs = append(currentLinkIDs, link.ID)
	}
	if !sameInt64Set(currentLinkIDs, expectedLinkIDs) {
		return ErrGuardianDeletePreviewChanged
	}
	for _, link := range links {
		if link.IsPayer {
			if err := s.recordPayerChange(ctx, link.GuardianProfileID, link.StudentID, changedByAccountID, "true", "false"); err != nil {
				return err
			}
		}
		if err := s.StudentGuardianRepo.Delete(ctx, link.ID); err != nil {
			return fmt.Errorf("failed to remove guardian link %d: %w", link.ID, err)
		}
	}
	return s.GuardianProfileRepo.Delete(ctx, id)
}

// auditGuardianFinancialDataDeletion records each stored payment value before
// the guardian delete cascades to its financial-data row. The caller holds the
// guardian profile lock and runs in the surrounding tenant transaction.
func (s *GuardianService) auditGuardianFinancialDataDeletion(ctx context.Context, guardianProfileID, changedByAccountID int64) error {
	if s.GuardianFinancialRepo == nil {
		return fmt.Errorf("guardian financial repository is not wired; refusing unaudited deletion")
	}
	data, err := s.GuardianFinancialRepo.FindByGuardianProfileID(ctx, guardianProfileID)
	if err != nil {
		return err
	}
	if data == nil || !data.HasData() {
		return nil
	}
	if s.GuardianFinancialAudit == nil {
		return fmt.Errorf("guardian financial audit repository is not wired; refusing unaudited deletion")
	}
	if changedByAccountID <= 0 {
		return fmt.Errorf("changed-by actor id is required")
	}

	changes := []stammdatenChange{}
	if data.IBAN != nil && *data.IBAN != "" {
		changes = append(changes, maskedChange(auditModels.GuardianPaymentFieldIBAN, maskTailPtr(data.IBAN, 4), nil))
	}
	if data.AccountHolder != nil && *data.AccountHolder != "" {
		changes = append(changes, maskedChange(auditModels.GuardianPaymentFieldAccountHolder,
			maskAllPtr(data.AccountHolder), nil))
	}
	for _, change := range changes {
		if err := s.GuardianFinancialAudit.Create(ctx, &auditModels.GuardianFinancialChange{
			GuardianProfileID: guardianProfileID,
			ChangedBy:         changedByAccountID,
			FieldName:         change.field,
			OldValue:          change.oldValue,
			NewValue:          change.newValue,
			Note:              "Erziehungsberechtigte Person gelöscht",
		}); err != nil {
			return fmt.Errorf("write guardian payment deletion audit: %w", err)
		}
	}
	return nil
}

// GetGuardianDeleteImpact returns the exact current affected links and display
// names for a full guardian delete preview.
func (s *GuardianService) GetGuardianDeleteImpact(ctx context.Context, guardianProfileID int64) (*GuardianDeleteImpact, error) {
	return s.getGuardianDeleteImpact(ctx, guardianProfileID)
}

// EvaluateGuardianDelete decides whether a guardian may be deleted and reports
// whether the guardian still has student links, so the caller can pick the
// correct delete path. It reads the current blast radius and applies the two
// business rules a full delete is gated on:
//
//   - a guardian still linked to students may not be deleted without an explicit
//     force request (returns *GuardianStillLinkedError, carrying the affected
//     children for the admin warning); and
//   - a forced full delete reaches across siblings the caller may not supervise,
//     so it is restricted to admins (returns ErrGuardianForceDeleteRequiresAdmin).
//
// hasLinks is only meaningful when err is nil. A read failure is returned as-is.
func (s *GuardianService) EvaluateGuardianDelete(ctx context.Context, guardianProfileID int64, force, isAdmin bool) (hasLinks bool, err error) {
	impact, err := s.getGuardianDeleteImpact(ctx, guardianProfileID)
	if err != nil {
		return false, err
	}

	if len(impact.StudentNames) == 0 {
		return false, nil
	}

	if !force {
		return true, &GuardianStillLinkedError{StudentNames: impact.StudentNames}
	}
	if !isAdmin {
		return true, ErrGuardianForceDeleteRequiresAdmin
	}

	return true, nil
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

// SendInvitation sends an invitation to a guardian.
//
// Deprecated: Replaced by services/auth.GuardianInvitationService.Create
// (parent-enrollment PR 3). Frontend does not call this. Cleanup PR pending.
func (s *GuardianService) SendInvitation(ctx context.Context, req GuardianInvitationRequest) (*authModels.GuardianInvitation, error) {
	// Get guardian profile
	profile, err := s.GuardianProfileRepo.FindByID(ctx, req.GuardianProfileID)
	if err != nil {
		return nil, fmt.Errorf(errMsgGuardianNotFound, err)
	}

	// Validate guardian can be invited
	if !profile.CanInvite() {
		return nil, fmt.Errorf("guardian cannot be invited: either no email or already has account")
	}

	// Check for pending invitations
	existingInvitations, err := s.GuardianInvitationRepo.FindByGuardianProfileID(ctx, profile.ID)
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
		ExpiresAt:         time.Now().Add(s.InvitationExpiry),
	}
	invitation.SetTenantID(tenant.FromContext(ctx))

	if err := s.GuardianInvitationRepo.Create(ctx, invitation); err != nil {
		return nil, fmt.Errorf("failed to create invitation: %w", err)
	}

	// Send invitation email asynchronously, pass tenant context for DB calls
	if s.Dispatcher != nil && profile.Email != nil {
		tenantCtx := context.WithoutCancel(tenant.ContextWithoutTransaction(ctx))
		go s.sendInvitationEmail(tenantCtx, invitation, profile)
	}

	return invitation, nil
}

// sendInvitationEmail sends the invitation email (called asynchronously).
// ctx should carry tenant context but not an ambient request transaction.
func (s *GuardianService) sendInvitationEmail(ctx context.Context, invitation *authModels.GuardianInvitation, profile *users.GuardianProfile) {
	if s.Dispatcher == nil || profile.Email == nil {
		return
	}

	invitationURL := fmt.Sprintf("%s/guardian/invite?token=%s", s.FrontendURL, invitation.Token)
	expiryHours := int(s.InvitationExpiry.Hours())

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
		From:     s.DefaultFrom,
		ReplyTo:  s.resolveReplyTo(ctx),
		To:       email.NewEmail("", *profile.Email),
		Subject:  "Einladung zum Eltern-Portal",
		Template: "guardian-invitation.html",
		Content: map[string]interface{}{
			"FirstName":     profile.FirstName,
			"LastName":      profile.LastName,
			"InvitationURL": invitationURL,
			"ExpiryHours":   expiryHours,
			"LogoURL":       fmt.Sprintf("%s/images/moto-logo-mit-schriftzug.png", s.FrontendURL),
			"StudentNames":  studentNames,
		},
	}

	meta := email.DeliveryMetadata{
		Type:        "guardian_invitation",
		ReferenceID: invitation.ID,
		Token:       invitation.Token,
		Recipient:   *profile.Email,
	}

	if s.Dispatcher != nil {
		s.Dispatcher.Dispatch(ctx, email.DeliveryRequest{
			Message:  message,
			Metadata: meta,
		})
	}

	// Update email status
	now := time.Now()
	_ = s.GuardianInvitationRepo.UpdateEmailStatus(ctx, invitation.ID, &now, nil, 0)
}

// getStudentNamesForGuardian retrieves the full names of all students linked to a guardian
// Returns an error if the guardian-student relationships cannot be loaded or if any student/person
// lookup fails. This ensures callers can distinguish between "no students" and "data retrieval failure".
func (s *GuardianService) getStudentNamesForGuardian(ctx context.Context, guardianProfileID int64) ([]string, error) {
	impact, err := s.getGuardianDeleteImpact(ctx, guardianProfileID)
	if err != nil {
		return nil, err
	}
	return impact.StudentNames, nil
}

// getGuardianDeleteImpact retrieves the link IDs and full names of all students
// linked to a guardian from the same relationship snapshot.
func (s *GuardianService) getGuardianDeleteImpact(ctx context.Context, guardianProfileID int64) (*GuardianDeleteImpact, error) {
	relationships, err := s.StudentGuardianRepo.FindByGuardianProfileID(ctx, guardianProfileID)
	if err != nil {
		return nil, fmt.Errorf("failed to load guardian-student relationships: %w", err)
	}

	linkIDs := make([]int64, 0, len(relationships))
	studentIDs := make([]int64, 0, len(relationships))
	for _, rel := range relationships {
		linkIDs = append(linkIDs, rel.ID)
		studentIDs = append(studentIDs, rel.StudentID)
	}

	// Batch-load students and their persons (replaces the per-relationship
	// 2N+1 lookups). Missing rows stay hard errors, as before.
	students, err := s.StudentRepo.FindByIDs(ctx, studentIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to load students: %w", err)
	}
	personIDs := make([]int64, 0, len(students))
	for _, student := range students {
		personIDs = append(personIDs, student.PersonID)
	}
	persons, err := s.PersonRepo.FindByIDs(ctx, personIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to load persons: %w", err)
	}

	studentNames := make([]string, 0, len(relationships))
	for _, rel := range relationships {
		student, ok := students[rel.StudentID]
		if !ok {
			return nil, fmt.Errorf("failed to load student %d: record missing", rel.StudentID)
		}
		person, ok := persons[student.PersonID]
		if !ok || person == nil {
			return nil, fmt.Errorf("person record %d is missing for student %d", student.PersonID, rel.StudentID)
		}
		studentNames = append(studentNames, person.GetFullName())
	}

	return &GuardianDeleteImpact{
		LinkIDs:      linkIDs,
		StudentNames: studentNames,
	}, nil
}

// GetStudentGuardians retrieves all guardians for a student
func (s *GuardianService) GetStudentGuardians(ctx context.Context, studentID int64) ([]*GuardianWithRelationship, error) {
	relationships, err := s.StudentGuardianRepo.FindByStudentID(ctx, studentID)
	if err != nil {
		return nil, err
	}

	profileIDs := make([]int64, 0, len(relationships))
	for _, rel := range relationships {
		profileIDs = append(profileIDs, rel.GuardianProfileID)
	}
	profiles, err := s.GuardianProfileRepo.FindByIDs(ctx, profileIDs)
	if err != nil {
		return nil, err
	}

	result := make([]*GuardianWithRelationship, 0, len(relationships))
	for _, rel := range relationships {
		profile, ok := profiles[rel.GuardianProfileID]
		if !ok {
			continue // Skip if profile not found
		}

		// Load phone numbers for this guardian
		phoneNumbers, err := s.GuardianPhoneNumberRepo.FindByGuardianID(ctx, profile.ID)
		if err == nil {
			profile.PhoneNumbers = phoneNumbers
		}

		result = append(result, &GuardianWithRelationship{
			Profile:           profile,
			Relationship:      rel,
			InvitationPending: s.invitationPendingForStudent(ctx, profile, rel.StudentID),
		})
	}

	return result, nil
}

// invitationPendingForStudent decides whether the staff list should surface
// "Einladung offen" for this guardian on this child. Guardians without an
// account keep the historical profile-wide check (any open invite). Guardians
// WITH an account can still have an open invitation — a pending-approval
// role-upgrade request (#2172) — but only one anchored to this child counts,
// so a sibling's invite never marks an unrelated row as pending.
func (s *GuardianService) invitationPendingForStudent(ctx context.Context, profile *users.GuardianProfile, studentID int64) bool {
	if !profile.HasAccount {
		return s.hasOpenInvitation(ctx, profile.ID, 0)
	}
	return s.hasOpenInvitation(ctx, profile.ID, studentID)
}

// hasOpenInvitation reports whether the guardian profile has an invitation that
// is neither accepted, expired, nor rejected — i.e. an outstanding invite the
// staff UI should surface as "Einladung offen". With studentID > 0, only
// invitations anchored to that child count. Best-effort: a lookup error is
// treated as "no pending invite" so the guardian list still renders.
func (s *GuardianService) hasOpenInvitation(ctx context.Context, guardianProfileID, studentID int64) bool {
	invitations, err := s.GuardianInvitationRepo.FindByGuardianProfileID(ctx, guardianProfileID)
	if err != nil {
		return false
	}
	now := time.Now()
	for _, inv := range invitations {
		if studentID > 0 && (inv.StudentID == nil || *inv.StudentID != studentID) {
			continue
		}
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
func (s *GuardianService) GetGuardianStudents(ctx context.Context, guardianProfileID int64) ([]*StudentWithRelationship, error) {
	relationships, err := s.StudentGuardianRepo.FindByGuardianProfileID(ctx, guardianProfileID)
	if err != nil {
		return nil, err
	}

	studentIDs := make([]int64, 0, len(relationships))
	for _, rel := range relationships {
		studentIDs = append(studentIDs, rel.StudentID)
	}
	students, err := s.StudentRepo.FindByIDs(ctx, studentIDs)
	if err != nil {
		return nil, err
	}

	result := make([]*StudentWithRelationship, 0, len(relationships))
	for _, rel := range relationships {
		student, ok := students[rel.StudentID]
		if !ok || student.IsAlumnus() {
			continue // Skip missing and graduated students
		}

		result = append(result, &StudentWithRelationship{
			Student:      student,
			Relationship: rel,
		})
	}

	return result, nil
}

// LinkGuardianToStudent creates a relationship between guardian and student
func (s *GuardianService) LinkGuardianToStudent(ctx context.Context, req StudentGuardianCreateRequest) (*users.StudentGuardian, error) {
	// Validate guardian profile exists
	if _, err := s.GuardianProfileRepo.FindByID(ctx, req.GuardianProfileID); err != nil {
		return nil, fmt.Errorf(errMsgGuardianNotFound, err)
	}

	// Validate student exists
	if _, err := s.StudentRepo.FindByID(ctx, req.StudentID); err != nil {
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
	inserted, err := s.StudentGuardianRepo.LinkIfNotExists(ctx, relationship)
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
	existingLinks, err := s.StudentGuardianRepo.FindByStudentID(ctx, req.StudentID)
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

// ValidateNewGuardians checks guardian input (profile, relationship type,
// emergency priority, phone numbers, and duplicate email) WITHOUT writing
// anything. Callers that persist a student and its guardians in one
// transaction MUST call this before the first write so a ValidationError
// rolls back an empty transaction instead of committing an orphaned
// student (TenantTxMiddleware only rolls back on 5xx). Every failure is
// returned as a *ValidationError (HTTP 400) carrying a user-facing German
// message.
func (s *GuardianService) ValidateNewGuardians(ctx context.Context, guardians []NewStudentGuardian) error {
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
			if _, err := s.GuardianProfileRepo.FindByID(ctx, *id); err != nil {
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
			if existing, err := s.GuardianProfileRepo.FindByEmail(ctx, email); err == nil && existing != nil {
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
func (s *GuardianService) AddGuardiansToStudent(ctx context.Context, studentID int64, guardians []NewStudentGuardian) error {
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
func (s *GuardianService) GetStudentGuardianRelationship(ctx context.Context, relationshipID int64) (*users.StudentGuardian, error) {
	relationship, err := s.StudentGuardianRepo.FindByID(ctx, relationshipID)
	if err != nil {
		return nil, fmt.Errorf("relationship not found: %w", err)
	}
	return relationship, nil
}

// UpdateStudentGuardianRelationship updates a student-guardian relationship
func (s *GuardianService) UpdateStudentGuardianRelationship(ctx context.Context, relationshipID int64, req StudentGuardianUpdateRequest) error {
	relationship, err := s.StudentGuardianRepo.FindByID(ctx, relationshipID)
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

	if err := relationship.Validate(); err != nil {
		return err
	}

	// Column-limited on purpose: is_payer is owned by SetStudentPayer, which
	// is gated on guardians:financial and writes an audit row. A whole-row
	// update here would carry the is_payer value this request read back into
	// the row, so a payer assigned in between (or removed in between) would be
	// silently undone by an unrelated relationship edit — without the
	// permission and without a trace.
	updated, err := s.StudentGuardianRepo.UpdateColumns(ctx, relationship,
		"relationship_type",
		"guardian_role",
		"permissions",
		"is_primary",
		"is_emergency_contact",
		"can_pickup",
		"pickup_notes",
		"emergency_priority",
	)
	if err != nil {
		return err
	}
	if updated == 0 {
		return fmt.Errorf("relationship not found: %d", relationshipID)
	}
	return nil
}

// RemoveGuardianFromStudent removes a guardian from a student.
//
// mayClearPayer says whether the caller holds guardians:financial. Unlinking
// the child's payer clears the payer mark, which is a financial decision; a
// caller without that permission gets ErrPayerRemovalRequiresFinancial and the
// relationship stays intact. The check runs under the student lock so a payer
// assigned concurrently cannot slip past it.
func (s *GuardianService) RemoveGuardianFromStudent(ctx context.Context, studentID, guardianProfileID, changedByAccountID int64, mayClearPayer bool) error {
	return s.txHandler.RunInTx(ctx, func(txCtx context.Context) error {
		return s.removeGuardianFromStudent(txCtx, studentID, guardianProfileID, changedByAccountID, mayClearPayer)
	})
}

func (s *GuardianService) removeGuardianFromStudent(ctx context.Context, studentID, guardianProfileID, changedByAccountID int64, mayClearPayer bool) error {
	// The student row serializes this deletion with SetStudentPayer, which
	// takes the same lock before it reads a relationship's is_payer state.
	if _, err := s.StudentRepo.FindByIDForUpdate(ctx, studentID); err != nil {
		return err
	}

	// Find the relationship after acquiring the lock so IsPayer is current.
	relationships, err := s.StudentGuardianRepo.FindByStudentID(ctx, studentID)
	if err != nil {
		return err
	}

	for _, rel := range relationships {
		if rel.GuardianProfileID == guardianProfileID {
			if rel.IsPayer {
				if !mayClearPayer {
					return ErrPayerRemovalRequiresFinancial
				}
				if err := s.recordPayerChange(ctx, rel.GuardianProfileID, rel.StudentID, changedByAccountID, "true", "false"); err != nil {
					return err
				}
			}
			return s.StudentGuardianRepo.Delete(ctx, rel.ID)
		}
	}

	return errors.New("relationship not found")
}

// ListGuardians retrieves guardians with pagination and filters, including phone numbers
func (s *GuardianService) ListGuardians(ctx context.Context, options *base.QueryOptions) ([]*users.GuardianProfile, error) {
	profiles, err := s.GuardianProfileRepo.ListWithOptions(ctx, options)
	if err != nil {
		return nil, err
	}

	// Load phone numbers for each guardian
	for _, profile := range profiles {
		phoneNumbers, err := s.GuardianPhoneNumberRepo.FindByGuardianID(ctx, profile.ID)
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
func (s *GuardianService) SearchGuardiansForPicker(ctx context.Context, searchText string, limit int) ([]*GuardianPickerMatch, error) {
	profiles, err := s.GuardianProfileRepo.SearchByText(ctx, searchText, limit)
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

	children, err := s.StudentGuardianRepo.ListLinkedChildrenForGuardians(ctx, ids)
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
func (s *GuardianService) GetGuardiansWithoutAccount(ctx context.Context) ([]*users.GuardianProfile, error) {
	return s.GuardianProfileRepo.FindWithoutAccount(ctx)
}

// GetInvitableGuardians retrieves guardians who can be invited
func (s *GuardianService) GetInvitableGuardians(ctx context.Context) ([]*users.GuardianProfile, error) {
	return s.GuardianProfileRepo.FindInvitable(ctx)
}

// GetPendingInvitations retrieves all pending guardian invitations
func (s *GuardianService) GetPendingInvitations(ctx context.Context) ([]*authModels.GuardianInvitation, error) {
	return s.GuardianInvitationRepo.FindPending(ctx)
}

// CleanupExpiredInvitations deletes expired invitations
func (s *GuardianService) CleanupExpiredInvitations(ctx context.Context) (int, error) {
	return s.GuardianInvitationRepo.DeleteExpired(ctx)
}

// ============================================================================
// Phone Number Management
// ============================================================================

// AddPhoneNumber adds a new phone number to a guardian
func (s *GuardianService) AddPhoneNumber(ctx context.Context, guardianID int64, req PhoneNumberCreateRequest) (*users.GuardianPhoneNumber, error) {
	// Lock the guardian profile so phone-list writers serialize with the
	// parents-portal contact path, which locks the same row before its wholesale
	// phone replace (delete-all + re-insert) — see UpdateGuardian. Without this a
	// staff phone add could land between the parent's read-old-phones and its
	// replace, then be wiped by the replace. Held for the request transaction.
	if err := s.GuardianProfileRepo.LockByIDForUpdate(ctx, guardianID); err != nil {
		return nil, fmt.Errorf(errMsgGuardianNotFound, err)
	}
	// Verify guardian exists
	if _, err := s.GuardianProfileRepo.FindByID(ctx, guardianID); err != nil {
		return nil, fmt.Errorf(errMsgGuardianNotFound, err)
	}

	// Get current count to determine if this should be primary
	count, err := s.GuardianPhoneNumberRepo.CountByGuardianID(ctx, guardianID)
	if err != nil {
		return nil, fmt.Errorf("failed to count existing phone numbers: %w", err)
	}

	// Get next priority
	priority, err := s.GuardianPhoneNumberRepo.GetNextPriority(ctx, guardianID)
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
		if err := s.GuardianPhoneNumberRepo.UnsetAllPrimary(ctx, guardianID); err != nil {
			return nil, fmt.Errorf("failed to unset existing primary: %w", err)
		}
		if err := s.GuardianPhoneNumberRepo.Create(ctx, phone); err != nil {
			return nil, fmt.Errorf("failed to create phone number: %w", err)
		}
		return phone, nil
	}

	if err := s.GuardianPhoneNumberRepo.Create(ctx, phone); err != nil {
		return nil, fmt.Errorf("failed to create phone number: %w", err)
	}

	return phone, nil
}

// UpdatePhoneNumber updates an existing phone number
func (s *GuardianService) UpdatePhoneNumber(ctx context.Context, phoneID int64, req PhoneNumberUpdateRequest) error {
	phone, err := s.GuardianPhoneNumberRepo.FindByID(ctx, phoneID)
	if err != nil {
		return fmt.Errorf(errMsgPhoneNotFound, err)
	}
	// Lock the owning guardian profile so this edit serializes with the
	// parents-portal contact path's wholesale phone replace (see UpdateGuardian).
	// The phone→profile association is immutable, so reading the phone first and
	// then locking its profile is safe.
	if err := s.GuardianProfileRepo.LockByIDForUpdate(ctx, phone.GuardianProfileID); err != nil {
		return fmt.Errorf(errMsgGuardianNotFound, err)
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
		if err := s.GuardianPhoneNumberRepo.UnsetAllPrimary(ctx, phone.GuardianProfileID); err != nil {
			return fmt.Errorf("failed to unset existing primary: %w", err)
		}
		return s.GuardianPhoneNumberRepo.Update(ctx, phone)
	} else if req.IsPrimary != nil && !*req.IsPrimary && phone.IsPrimary {
		// Unsetting primary - need to promote another number
		phone.IsPrimary = false
	}

	return s.GuardianPhoneNumberRepo.Update(ctx, phone)
}

// DeletePhoneNumber removes a phone number
func (s *GuardianService) DeletePhoneNumber(ctx context.Context, phoneID int64) error {
	phone, err := s.GuardianPhoneNumberRepo.FindByID(ctx, phoneID)
	if err != nil {
		return fmt.Errorf(errMsgPhoneNotFound, err)
	}
	// Lock the owning guardian profile so this delete (and the primary-promotion
	// below) serializes with the parents-portal contact path's wholesale phone
	// replace (see UpdateGuardian).
	if err := s.GuardianProfileRepo.LockByIDForUpdate(ctx, phone.GuardianProfileID); err != nil {
		return fmt.Errorf(errMsgGuardianNotFound, err)
	}

	wasPrimary := phone.IsPrimary
	guardianID := phone.GuardianProfileID

	// Delete the phone number
	if err := s.GuardianPhoneNumberRepo.Delete(ctx, phoneID); err != nil {
		return fmt.Errorf("failed to delete phone number: %w", err)
	}

	// If deleted number was primary, promote the next one
	if wasPrimary {
		phones, err := s.GuardianPhoneNumberRepo.FindByGuardianID(ctx, guardianID)
		if err != nil {
			return nil // Deletion succeeded, just couldn't promote - not fatal
		}

		// Promote the first remaining phone number (already sorted by priority)
		if len(phones) > 0 {
			_ = s.GuardianPhoneNumberRepo.SetPrimary(ctx, phones[0].ID, guardianID)
		}
	}

	return nil
}

// SetPrimaryPhone sets a phone number as the primary contact
func (s *GuardianService) SetPrimaryPhone(ctx context.Context, phoneID int64) error {
	phone, err := s.GuardianPhoneNumberRepo.FindByID(ctx, phoneID)
	if err != nil {
		return fmt.Errorf(errMsgPhoneNotFound, err)
	}
	// Lock the owning guardian profile so this primary-flag flip serializes with
	// the parents-portal contact path's wholesale phone replace (delete-all +
	// re-insert) — see UpdateGuardian. Without it a staff set-primary could land
	// between the parent's read-old-phones and its replace, then be wiped by the
	// replace. The phone→profile association is immutable, so reading the phone
	// first and then locking its profile is safe. Held for the request tx.
	if err := s.GuardianProfileRepo.LockByIDForUpdate(ctx, phone.GuardianProfileID); err != nil {
		return fmt.Errorf(errMsgGuardianNotFound, err)
	}

	return s.GuardianPhoneNumberRepo.SetPrimary(ctx, phoneID, phone.GuardianProfileID)
}

// GetGuardianPhoneNumbers retrieves all phone numbers for a guardian, sorted by priority
func (s *GuardianService) GetGuardianPhoneNumbers(ctx context.Context, guardianID int64) ([]*users.GuardianPhoneNumber, error) {
	return s.GuardianPhoneNumberRepo.FindByGuardianID(ctx, guardianID)
}

// GetPhoneNumberByID retrieves a phone number by ID
func (s *GuardianService) GetPhoneNumberByID(ctx context.Context, phoneID int64) (*users.GuardianPhoneNumber, error) {
	return s.GuardianPhoneNumberRepo.FindByID(ctx, phoneID)
}

// resolveReplyTo returns the OGS reply address for this tenant, or the zero
// value when none is configured. This send bypasses the outbox, so it stamps
// the header itself; the degradation policy is shared (#1936).
func (s *GuardianService) resolveReplyTo(ctx context.Context) email.Email {
	identity := email.ResolveReplyToIdentity(ctx, s.MailIdentity, tenant.FromContext(ctx), nil)
	return email.NewEmail(identity.Name, identity.Address)
}
