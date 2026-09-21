package care

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/internal/strutil"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// CreateGuardianContact adds an accountless contact to the child.
func (s *Service) CreateGuardianContact(ctx context.Context, accountID, studentID int64, input CreateGuardianContactInput) (*ChildGuardian, error) {
	if err := validateCreateGuardianContactInput(&input); err != nil {
		return nil, err
	}
	child, err := s.ResolvePermittedChild(ctx, accountID, studentID, authorize.GuardianPermissionGuardianEdit)
	if err != nil {
		return nil, err
	}
	// A child whose care at this school has ended keeps read access to what
	// happened, but nothing new can be submitted for them (#2487).
	if err := child.RequireCareRunning(); err != nil {
		return nil, err
	}
	if err := s.requireGuardianManagementEnabled(ctx, child.TenantID); err != nil {
		return nil, err
	}
	if (input.CanPickup || input.IsEmergencyContact) && !child.HasPermission(authorize.GuardianPermissionPickupManage) {
		return nil, ErrGuardianPermissionDenied
	}
	if s.Guardians == nil {
		return nil, errors.New("parent: guardian records are not configured")
	}

	var result *ChildGuardian
	txErr := InTenant(ctx, child.TenantID, func(txCtx context.Context) error {
		created, err := s.createGuardianContactInTx(txCtx, child, accountID, studentID, input)
		result = created
		return err
	})
	if txErr != nil {
		if errors.Is(txErr, ErrGuardianEmailConflict) {
			return nil, ErrGuardianEmailConflict
		}
		return nil, txErr
	}

	s.Logger.Info("parent created guardian contact",
		slog.Int64("account_id", accountID),
		slog.Int64("student_id", studentID),
		slog.Int64("guardian_profile_id", result.GuardianProfileID),
		slog.Int64("tenant_id", child.TenantID),
	)
	return result, nil
}

// createGuardianContactInTx writes the new profile, its phones and its link to
// the child through People Directory and records the change trail, all in the
// child's tenant unit of work.
func (s *Service) createGuardianContactInTx(ctx context.Context, child *Child, accountID, studentID int64, input CreateGuardianContactInput) (*ChildGuardian, error) {
	if err := s.RequireCareRunningForUpdate(ctx, studentID); err != nil {
		return nil, err
	}
	if err := s.requireGuardianEmailFree(ctx, input.Contact.Email, 0); err != nil {
		return nil, err
	}
	profile := &usersModels.GuardianProfile{
		PreferredContactMethod: "phone",
		LanguagePreference:     "de",
	}
	profile.SetTenantID(child.TenantID)
	applyContactInput(profile, &input.Contact)
	if profile.Email != nil {
		profile.PreferredContactMethod = "email"
	}
	if err := s.createGuardianProfile(ctx, profile); err != nil {
		return nil, err
	}
	if err := s.replaceGuardianPhones(ctx, profile, input.Contact.Phones); err != nil {
		return nil, err
	}
	link, err := s.linkGuardianContact(ctx, child.TenantID, studentID, profile.ID, input)
	if err != nil {
		return nil, err
	}

	phones, err := s.GuardianPhoneRepo.FindByGuardianID(ctx, profile.ID)
	if err != nil {
		return nil, err
	}
	if err := s.auditContactChanges(ctx, accountID, studentID, profile.ID, guardianContactSnapshot{}, profile, phones); err != nil {
		return nil, err
	}
	flagInput := GuardianRelationshipInput{
		CanPickup:          &input.CanPickup,
		IsEmergencyContact: &input.IsEmergencyContact,
	}
	if err := s.auditPickupFlagChanges(ctx, accountID, studentID, profile.ID, flagInput, false, false); err != nil {
		return nil, err
	}

	result := projectChildGuardian(profile, link, phones, accountID, true,
		child.HasPermission(authorize.GuardianPermissionPickupManage), false)
	capturedTenant := child.TenantID
	tenant.RegisterAfterCommit(ctx, func() {
		s.broadcastStudentUpdated(capturedTenant, studentID)
	})
	return result, nil
}

// requireGuardianEmailFree refuses an e-mail another guardian profile of the
// school already uses. Only the clean not-found sentinel means "no conflicting
// profile"; any other lookup failure aborts the write. The case-insensitive
// unique index stays the race-closing backstop, which People Directory reports
// as the same conflict.
func (s *Service) requireGuardianEmailFree(ctx context.Context, email *string, ownProfileID int64) error {
	if email == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*email)
	if trimmed == "" {
		return nil
	}
	existing, err := s.GuardianProfileRepo.FindByEmail(ctx, trimmed)
	if err != nil && !errors.Is(err, usersModels.ErrGuardianProfileNotFound) {
		return err
	}
	if existing != nil && existing.ID != ownProfileID {
		return ErrGuardianEmailConflict
	}
	return nil
}

// createGuardianProfile validates and normalises the profile the way the
// directory stores it (trimmed names, lower-case e-mail) and inserts it
// through People Directory.
func (s *Service) createGuardianProfile(ctx context.Context, profile *usersModels.GuardianProfile) error {
	if err := profile.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}
	id, err := s.Guardians.CreateGuardianContact(ctx, guardianContactRecord(profile))
	if err != nil {
		return err
	}
	profile.ID = id
	return nil
}

// updateGuardianProfile validates the edited profile and rewrites its contact
// slice through People Directory.
func (s *Service) updateGuardianProfile(ctx context.Context, profile *usersModels.GuardianProfile) error {
	if err := profile.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}
	return s.Guardians.UpdateGuardianContact(ctx, profile.ID, guardianContactRecord(profile))
}

func guardianContactRecord(profile *usersModels.GuardianProfile) GuardianContactRecord {
	return GuardianContactRecord{
		FirstName: profile.FirstName, LastName: profile.LastName, Email: profile.Email,
		AddressStreet: profile.AddressStreet, AddressCity: profile.AddressCity, AddressPostalCode: profile.AddressPostalCode,
		PreferredContactMethod: profile.PreferredContactMethod, LanguagePreference: profile.LanguagePreference,
	}
}

// linkGuardianContact links the new profile to the child with the default
// role of its relationship. A fresh profile that is already linked is a data
// fault, not a conflict.
func (s *Service) linkGuardianContact(ctx context.Context, tenantID, studentID, profileID int64, input CreateGuardianContactInput) (*usersModels.StudentGuardian, error) {
	link := &usersModels.StudentGuardian{
		StudentID:          studentID,
		GuardianProfileID:  profileID,
		RelationshipType:   strings.TrimSpace(input.RelationshipType),
		IsEmergencyContact: input.IsEmergencyContact,
		CanPickup:          input.CanPickup,
		PickupNotes:        strutil.TrimPtrToNil(input.PickupNotes),
		EmergencyPriority:  1,
	}
	authorize.ApplyDefaultStudentGuardianRole(link)
	link.SetTenantID(tenantID)
	if err := link.Validate(); err != nil {
		return nil, err
	}
	record, err := guardianContactLink(link)
	if err != nil {
		return nil, err
	}
	linkID, inserted, err := s.Guardians.LinkGuardianContact(ctx, record)
	if err != nil {
		return nil, err
	}
	if !inserted {
		return nil, fmt.Errorf("parent: new guardian profile %d was already linked to student %d", profileID, studentID)
	}
	link.ID = linkID
	return link, nil
}

// ListChildGuardians returns every guardian linked to the child with contact +
// pickup detail and the caller's per-guardian edit capabilities. Authorization
// only (parent_portal.access).
func (s *Service) ListChildGuardians(ctx context.Context, accountID, studentID int64) ([]*ChildGuardian, error) {
	child, err := s.ResolvePermittedChild(ctx, accountID, studentID, authorize.GuardianPermissionPortalAccess)
	if err != nil {
		return nil, err
	}
	canEdit := child.HasPermission(authorize.GuardianPermissionGuardianEdit)
	canManage := child.HasPermission(authorize.GuardianPermissionPickupManage)
	// A school can disable the whole feature: when off, the list still shows
	// guardians (read) but advertises no edit affordances, so the UI hides them.
	if canEdit || canManage {
		enabled, err := s.guardianManagementEnabled(ctx, child.TenantID)
		if err != nil {
			return nil, err
		}
		if !enabled {
			canEdit = false
			canManage = false
		}
	}
	var out []*ChildGuardian
	txErr := InTenant(ctx, child.TenantID, func(txCtx context.Context) error {
		var loadErr error
		out, loadErr = s.loadChildGuardians(txCtx, child.TenantID, accountID, studentID, canEdit, canManage)
		return loadErr
	})
	if txErr != nil {
		return nil, txErr
	}
	return out, nil
}

// loadChildGuardians batch-loads profiles, phones, and cross-family reach for
// every guardian of the child in three queries total, rather than three per
// guardian.
func (s *Service) loadChildGuardians(ctx context.Context, tenantID, accountID, studentID int64, canEdit, canManage bool) ([]*ChildGuardian, error) {
	callerStudents, err := s.callerFamilyStudentSet(ctx, accountID)
	if err != nil {
		return nil, err
	}
	links, err := s.StudentGuardianRepo.FindByStudentID(ctx, studentID)
	if err != nil {
		return nil, err
	}
	profileIDs := make([]int64, 0, len(links))
	for _, link := range links {
		profileIDs = append(profileIDs, link.GuardianProfileID)
	}
	profiles, err := s.GuardianProfileRepo.FindByIDs(ctx, profileIDs)
	if err != nil {
		return nil, err
	}
	phonesByProfile, err := s.GuardianPhoneRepo.FindByGuardianIDs(ctx, profileIDs)
	if err != nil {
		return nil, err
	}
	escapesByProfile, err := s.profilesEscapingFamily(ctx, profileIDs, callerStudents)
	if err != nil {
		return nil, err
	}
	out := make([]*ChildGuardian, 0, len(links))
	for _, link := range links {
		profile, ok := profiles[link.GuardianProfileID]
		if !ok {
			// A link pointing to a missing profile is a data-integrity fault
			// (orphaned students_guardians row), not an authorization outcome.
			// Surface it as an internal error (→ 500, caught by 5xx alerting)
			// instead of masking it as ErrGuardianNotLinked (403), which would
			// hide the inconsistency from monitoring.
			s.Logger.Error("parent child guardian link points to missing profile",
				slog.Int64("tenant_id", tenantID),
				slog.Int64("student_id", studentID),
				slog.Int64("student_guardian_id", link.ID),
				slog.Int64("guardian_profile_id", link.GuardianProfileID),
			)
			return nil, fmt.Errorf("parent: student_guardian %d references missing guardian profile %d", link.ID, link.GuardianProfileID)
		}
		out = append(out, projectChildGuardian(profile, link, phonesByProfile[profile.ID], accountID, canEdit, canManage, escapesByProfile[profile.ID]))
	}
	return out, nil
}

// replaceGuardianPhones replaces the guardian's entire phone list with the
// submitted set. A wholesale replace keeps the edit atomic and avoids per-row
// diffing the portal would otherwise have to drive.
func (s *Service) replaceGuardianPhones(ctx context.Context, profile *usersModels.GuardianProfile, phones []GuardianPhoneInput) error {
	records := make([]GuardianPhoneRecord, 0, len(phones))
	submittedPrimary := hasSubmittedPrimaryPhone(phones)
	primarySeen := false
	for i, p := range phones {
		phoneType := usersModels.PhoneType(strings.TrimSpace(p.PhoneType))
		if !usersModels.ValidPhoneTypes[phoneType] {
			phoneType = usersModels.PhoneTypeMobile
		}
		isPrimary := p.IsPrimary
		if i == 0 && !submittedPrimary {
			isPrimary = true
		}
		if isPrimary {
			if primarySeen {
				isPrimary = false
			} else {
				primarySeen = true
			}
		}
		entity := &usersModels.GuardianPhoneNumber{
			GuardianProfileID: profile.ID,
			PhoneNumber:       strings.TrimSpace(p.PhoneNumber),
			PhoneType:         phoneType,
			Label:             p.Label,
			IsPrimary:         isPrimary,
			Priority:          i + 1,
		}
		// The directory stores the phone the way its validator normalises it
		// (trimmed number, empty label as none); a refused row aborts the whole
		// replace.
		if err := entity.Validate(); err != nil {
			return fmt.Errorf("validation failed: %w", err)
		}
		records = append(records, GuardianPhoneRecord{
			PhoneNumber: entity.PhoneNumber, PhoneType: string(entity.PhoneType), Label: entity.Label,
			IsPrimary: entity.IsPrimary, Priority: entity.Priority,
		})
	}
	return s.Guardians.ReplaceGuardianPhones(ctx, profile.ID, records)
}

func hasSubmittedPrimaryPhone(phones []GuardianPhoneInput) bool {
	for _, p := range phones {
		if p.IsPrimary {
			return true
		}
	}
	return false
}
