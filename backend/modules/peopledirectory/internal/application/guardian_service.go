package application

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"

	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/userscontract"
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

// emailInUseError keeps the historical message and matches
// ErrGuardianEmailInUse.
// position names the guardian in a batch (1-based); zero means a single one.
type emailInUseError struct {
	email    string
	position int
}

func (e emailInUseError) Error() string {
	if e.position > 0 {
		return fmt.Sprintf("Erziehungsberechtigte/r %d: "+emailInUseMsgFmt, e.position, e.email)
	}
	return fmt.Sprintf(emailInUseMsgFmt, e.email)
}
func (e emailInUseError) Is(err error) bool { return err == peoplePorts.ErrGuardianEmailInUse }

// newEmailInUseError builds the 400 ValidationError for a duplicate guardian
// email. email is trimmed for display so the message matches what the unique
// pre-check compared.
func newEmailInUseError(email string) *userscontract.ValidationError {
	return &userscontract.ValidationError{Err: emailInUseError{email: strings.TrimSpace(email)}}
}

// RetainedGuardianService manages guardian profiles, their student relationships,
// invitations, and phone numbers.
type RetainedGuardianService struct {
	peoplePorts.GuardianServiceDependencies
	transactions peoplePorts.GuardianTransactions
}

// NewRetainedGuardianService creates a new GuardianService instance over the
// transactions the composition binds.
func NewRetainedGuardianService(deps peoplePorts.GuardianServiceDependencies, transactions peoplePorts.GuardianTransactions) *RetainedGuardianService {
	return &RetainedGuardianService{
		GuardianServiceDependencies: deps,
		transactions:                transactions,
	}
}

// CreateGuardian creates a new guardian profile without an account
// Note: Phone numbers should be added separately via AddPhoneNumber
func (s *RetainedGuardianService) CreateGuardian(ctx context.Context, req peoplePorts.GuardianCreateRequest) (*peopleValues.GuardianProfileSnapshot, error) {
	profile := &peopleValues.GuardianProfileSnapshot{
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

	profile.SetTenantID(s.transactions.TenantID(ctx))

	if err := profile.Validate(); err != nil {
		return nil, newGuardianValidationError(err)
	}

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
		if errors.Is(err, peoplePorts.ErrGuardianEmailTaken) && profile.Email != nil {
			return nil, newEmailInUseError(*profile.Email)
		}
		return nil, fmt.Errorf("failed to create guardian profile: %w", err)
	}

	return profile, nil
}

// GetGuardianByID retrieves a guardian profile by ID with phone numbers
func (s *RetainedGuardianService) GetGuardianByID(ctx context.Context, id int64) (*peopleValues.GuardianProfileSnapshot, error) {
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
func (s *RetainedGuardianService) GuardianDisplays(ctx context.Context, ids []int64) ([]peoplePorts.GuardianDisplay, error) {
	if len(ids) == 0 {
		return []peoplePorts.GuardianDisplay{}, nil
	}
	profiles, err := s.GuardianProfileRepo.FindByIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("resolve guardian displays: %w", err)
	}
	displays := make([]peoplePorts.GuardianDisplay, 0, len(profiles))
	for _, profile := range profiles {
		displays = append(displays, peoplePorts.GuardianDisplay{
			GuardianProfileID: profile.ID,
			FirstName:         profile.FirstName,
			LastName:          profile.LastName,
		})
	}
	return displays, nil
}

// UpdateGuardian updates a guardian profile
func (s *RetainedGuardianService) UpdateGuardian(ctx context.Context, id int64, req peoplePorts.GuardianCreateRequest) error {
	// Serialize all guardian contact writers on the profile row. The
	// parents-portal contact path (workflows/parentportal/care UpdateGuardianContact) locks
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

	if err := profile.Validate(); err != nil {
		return newGuardianValidationError(err)
	}

	if err := s.GuardianProfileRepo.Update(ctx, profile); err != nil {
		// TOCTOU guard mirroring CreateGuardian: a concurrent writer can claim
		// the email between the check above and this Update.
		if errors.Is(err, peoplePorts.ErrGuardianEmailTaken) && profile.Email != nil {
			return newEmailInUseError(*profile.Email)
		}
		return err
	}
	return nil
}

// DeleteGuardian removes a guardian profile WITHOUT touching its student links.
//
// Since migration 1.15.127 the relationship → guardian_profiles FK is
// ON DELETE RESTRICT, so this fails with a foreign-key violation when the
// guardian is still linked to any student — the handler turns that into a 409.
// Use this only for guardians with no remaining links; for the deliberate
// full delete use DeleteGuardianWithLinks.
func (s *RetainedGuardianService) DeleteGuardian(ctx context.Context, id, changedByAccountID int64) error {
	return s.transactions.RunInTx(ctx, func(txCtx context.Context) error {
		return s.deleteGuardian(txCtx, id, changedByAccountID)
	})
}

func (s *RetainedGuardianService) deleteGuardian(ctx context.Context, id, changedByAccountID int64) error {
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
func (s *RetainedGuardianService) DeleteGuardianWithLinks(ctx context.Context, id int64, expectedLinkIDs []int64, changedByAccountID int64) error {
	return s.transactions.RunInTx(ctx, func(txCtx context.Context) error {
		return s.deleteGuardianWithLinks(txCtx, id, expectedLinkIDs, changedByAccountID)
	})
}

func (s *RetainedGuardianService) deleteGuardianWithLinks(ctx context.Context, id int64, expectedLinkIDs []int64, changedByAccountID int64) error {
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
		return peoplePorts.ErrGuardianDeletePreviewChanged
	}
	if err := s.deleteGuardianLinks(ctx, links, changedByAccountID); err != nil {
		return err
	}
	return s.GuardianProfileRepo.Delete(ctx, id)
}

// deleteGuardianLinks removes the confirmed links of a full guardian delete,
// auditing each payer mark that goes with them.
func (s *RetainedGuardianService) deleteGuardianLinks(ctx context.Context, links []*peopleValues.GuardianRelationshipSnapshot, changedByAccountID int64) error {
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
	return nil
}

// auditGuardianFinancialDataDeletion records each stored payment value before
// the guardian delete cascades to its financial-data row. The caller holds the
// guardian profile lock and runs in the surrounding tenant transaction.
func (s *RetainedGuardianService) auditGuardianFinancialDataDeletion(ctx context.Context, guardianProfileID, changedByAccountID int64) error {
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
		changes = append(changes, maskedChange(peoplePorts.GuardianPaymentFieldIBAN, maskTailPtr(data.IBAN, 4), nil))
	}
	if data.AccountHolder != nil && *data.AccountHolder != "" {
		changes = append(changes, maskedChange(peoplePorts.GuardianPaymentFieldAccountHolder,
			maskAllPtr(data.AccountHolder), nil))
	}
	for _, change := range changes {
		if err := s.GuardianFinancialAudit.RecordGuardianFinancialChange(ctx, peoplePorts.GuardianFinancialChange{
			GuardianProfileID: guardianProfileID,
			ChangedBy:         changedByAccountID,
			Field:             change.field,
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
func (s *RetainedGuardianService) GetGuardianDeleteImpact(ctx context.Context, guardianProfileID int64) (*peoplePorts.GuardianDeleteImpact, error) {
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
func (s *RetainedGuardianService) EvaluateGuardianDelete(ctx context.Context, guardianProfileID int64, force, isAdmin bool) (hasLinks bool, err error) {
	impact, err := s.getGuardianDeleteImpact(ctx, guardianProfileID)
	if err != nil {
		return false, err
	}

	if len(impact.StudentNames) == 0 {
		return false, nil
	}

	if !force {
		return true, &peoplePorts.GuardianStillLinkedError{StudentNames: impact.StudentNames}
	}
	if !isAdmin {
		return true, peoplePorts.ErrGuardianForceDeleteRequiresAdmin
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

// getGuardianDeleteImpact retrieves the link IDs and full names of all students
// linked to a guardian from the same relationship snapshot.
func (s *RetainedGuardianService) getGuardianDeleteImpact(ctx context.Context, guardianProfileID int64) (*peoplePorts.GuardianDeleteImpact, error) {
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

	return &peoplePorts.GuardianDeleteImpact{
		LinkIDs:      linkIDs,
		StudentNames: studentNames,
	}, nil
}

// ListGuardians retrieves guardians with pagination and filters, including phone numbers
func (s *RetainedGuardianService) ListGuardians(ctx context.Context, options *peoplePorts.QueryOptions) ([]*peopleValues.GuardianProfileSnapshot, error) {
	profiles, err := s.GuardianProfileRepo.ListWithOptions(ctx, options)
	if err != nil {
		return nil, err
	}

	profileIDs := make([]int64, 0, len(profiles))
	for _, profile := range profiles {
		profileIDs = append(profileIDs, profile.ID)
	}
	phonesByProfile, err := s.GuardianPhoneNumberRepo.FindByGuardianIDs(ctx, profileIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to load guardian phone numbers: %w", err)
	}
	for _, profile := range profiles {
		profile.PhoneNumbers = phonesByProfile[profile.ID]
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
func (s *RetainedGuardianService) SearchGuardiansForPicker(ctx context.Context, searchText string, limit int) ([]*peoplePorts.GuardianPickerMatch, error) {
	profiles, err := s.GuardianProfileRepo.SearchByText(ctx, searchText, limit)
	if err != nil {
		return nil, err
	}
	if len(profiles) == 0 {
		return []*peoplePorts.GuardianPickerMatch{}, nil
	}

	ids := make([]int64, len(profiles))
	for i, p := range profiles {
		ids[i] = p.ID
	}

	children, err := s.StudentGuardianRepo.ListLinkedChildrenForGuardians(ctx, ids)
	if err != nil {
		return nil, err
	}

	byGuardian := make(map[int64][]*peopleValues.GuardianLinkedChildSnapshot, len(profiles))
	for _, c := range children {
		byGuardian[c.GuardianProfileID] = append(byGuardian[c.GuardianProfileID], c)
	}

	matches := make([]*peoplePorts.GuardianPickerMatch, len(profiles))
	for i, p := range profiles {
		matches[i] = &peoplePorts.GuardianPickerMatch{Profile: p, Children: byGuardian[p.ID]}
	}
	return matches, nil
}

// GetGuardiansWithoutAccount retrieves guardians who don't have portal accounts
func (s *RetainedGuardianService) GetGuardiansWithoutAccount(ctx context.Context) ([]*peopleValues.GuardianProfileSnapshot, error) {
	return s.GuardianProfileRepo.FindWithoutAccount(ctx)
}

// GetInvitableGuardians retrieves guardians who can be invited
func (s *RetainedGuardianService) GetInvitableGuardians(ctx context.Context) ([]*peopleValues.GuardianProfileSnapshot, error) {
	return s.GuardianProfileRepo.FindInvitable(ctx)
}

// GetPendingInvitations retrieves the guardian invitations whose link can
// still be spent.
func (s *RetainedGuardianService) GetPendingInvitations(ctx context.Context) ([]peoplePorts.GuardianInvitationRecord, error) {
	return s.GuardianInvitations.ListRedeemable(ctx)
}

// ============================================================================
// Phone Number Management
// ============================================================================

// GuardianProfiles is the retained guardian profile repository the service
// reads, for the directory reads that look a profile up by e-mail.
func (s *RetainedGuardianService) GuardianProfiles() peoplePorts.GuardianProfileRepository {
	return s.GuardianProfileRepo
}
