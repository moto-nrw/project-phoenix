package care

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// UpdateGuardianRelationship edits the per-child pickup/relationship fields of a
// guardian. The CanPickup and IsEmergencyContact flags require
// parent_portal.pickup.manage (safety-relevant authority); PickupNotes requires
// parent_portal.guardian.edit. Each field group is
// gated by its own permission so that a caller who holds only one of the two
// permissions can exercise exactly the capability advertised by
// ListChildGuardians, never receiving a 403 for an action the listing offered.
// The flags may additionally only be set on guardians WITHOUT their own portal
// account: an account holder's pickup/emergency standing is set by themselves
// (out of band) or the school, never by another parent and never self-granted
// through the portal.
func (s *Service) UpdateGuardianRelationship(ctx context.Context, accountID, studentID, guardianProfileID int64, input GuardianRelationshipInput) (*ChildGuardian, error) {
	if input.CanPickup == nil && input.IsEmergencyContact == nil && input.PickupNotes == nil {
		return nil, ErrGuardianNoChange
	}
	if err := validateRelationshipInput(&input); err != nil {
		return nil, err
	}
	edit := relationshipEdit{
		accountID: accountID, studentID: studentID, guardianProfileID: guardianProfileID, input: input,
		editsFlags:   input.CanPickup != nil || input.IsEmergencyContact != nil,
		editsDetails: input.PickupNotes != nil,
	}

	// Resolve on baseline portal access, then gate each field group by its own
	// permission. Resolving on the lower baseline (rather than guardian.edit) is
	// what lets a pickup.manage-only caller flip the flags.
	child, err := s.ResolvePermittedChild(ctx, accountID, studentID, authorize.GuardianPermissionPortalAccess)
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
	if edit.editsFlags && !child.HasPermission(authorize.GuardianPermissionPickupManage) {
		return nil, ErrGuardianPermissionDenied
	}
	if edit.editsDetails && !child.HasPermission(authorize.GuardianPermissionGuardianEdit) {
		return nil, ErrGuardianPermissionDenied
	}
	if s.Guardians == nil {
		return nil, errors.New("parent: guardian records are not configured")
	}
	var result *ChildGuardian
	txErr := InTenant(ctx, child.TenantID, func(txCtx context.Context) error {
		updated, err := s.updateGuardianRelationshipInTx(txCtx, child, edit)
		result = updated
		return err
	})
	if txErr != nil {
		return nil, txErr
	}

	s.Logger.Info("parent updated guardian relationship",
		slog.Int64("account_id", accountID),
		slog.Int64("student_id", studentID),
		slog.Int64("guardian_profile_id", guardianProfileID),
		slog.Int64("tenant_id", child.TenantID),
		slog.Bool("pickup_changed", input.CanPickup != nil),
		slog.Bool("emergency_changed", input.IsEmergencyContact != nil),
	)
	return result, nil
}

// relationshipEdit is one per-child pickup/relationship edit of a guardian.
type relationshipEdit struct {
	accountID         int64
	studentID         int64
	guardianProfileID int64
	input             GuardianRelationshipInput
	editsFlags        bool
	editsDetails      bool
}

// updateGuardianRelationshipInTx locks, authorizes and writes the edit inside
// the child's tenant unit of work.
func (s *Service) updateGuardianRelationshipInTx(ctx context.Context, child *Child, edit relationshipEdit) (*ChildGuardian, error) {
	if err := s.RequireCareRunningForUpdate(ctx, edit.studentID); err != nil {
		return nil, err
	}
	// Lock the guardian profile row for the duration of the tx. The link
	// update below is a read-modify-write on the students_guardians row, and
	// the same profile lock guards the contact path; taking it here serializes
	// concurrent relationship edits (and relationship-vs-contact edits) of the
	// same guardian so independent field writes can't clobber each other in a
	// lost update under read-committed.
	if err := s.GuardianProfileRepo.LockByIDForUpdate(ctx, edit.guardianProfileID); err != nil {
		if errors.Is(err, usersModels.ErrGuardianProfileNotFound) {
			return nil, ErrGuardianNotLinked
		}
		return nil, err
	}
	callerStudents, err := s.callerFamilyStudentSet(ctx, edit.accountID)
	if err != nil {
		return nil, err
	}
	// Lock the relationship row itself FOR UPDATE (profile-then-link order, as
	// in the contact path). The role/account guards below decide on
	// link.GuardianRole and the write mutates this row; without the lock a
	// concurrent staff edit could promote/remove the relationship after the
	// guards pass and before the write, since the staff path edits the
	// students_guardians row without taking the profile lock above. Locking
	// the row makes the authorization decision and the write atomic against it.
	link, err := s.findChildGuardianLinkForUpdate(ctx, edit.studentID, edit.guardianProfileID)
	if err != nil {
		return nil, err
	}
	profile, err := s.GuardianProfileRepo.FindByID(ctx, edit.guardianProfileID)
	if err != nil {
		if errors.Is(err, usersModels.ErrGuardianProfileNotFound) {
			return nil, ErrGuardianNotLinked
		}
		return nil, err
	}
	if err := s.refuseRelationshipEdit(ctx, profile, link, edit, callerStudents); err != nil {
		return nil, err
	}
	oldCanPickup := link.CanPickup
	oldEmergency := link.IsEmergencyContact
	if err := s.writeRelationshipEdit(ctx, link, edit.input); err != nil {
		return nil, err
	}
	if err := s.auditPickupFlagChanges(ctx, edit.accountID, edit.studentID, edit.guardianProfileID, edit.input, oldCanPickup, oldEmergency); err != nil {
		return nil, err
	}
	return s.projectEditedRelationship(ctx, child, profile, link, edit, callerStudents)
}

// refuseRelationshipEdit applies the eligibility of each field group.
func (s *Service) refuseRelationshipEdit(ctx context.Context, profile *usersModels.GuardianProfile, link *usersModels.StudentGuardian, edit relationshipEdit, callerStudents map[int64]bool) error {
	isSelf := profile.AccountID != nil && *profile.AccountID == edit.accountID
	if edit.editsFlags {
		if err := refusePickupFlagEdit(profile, link); err != nil {
			return err
		}
	}
	if edit.editsDetails && !isSelf {
		if err := refusePickupNoteEdit(profile, link); err != nil {
			return err
		}
	}
	// A full guardian (primary/legal/co) is a real legal guardian, not a
	// helper: neither their pickup/emergency authority nor their per-child note
	// is another parent's to change, even without a portal account. Mirrors the
	// contact-edit guard and the read-side CanManagePickup gate. Self is exempt
	// (the account guard above already blocks self flags; a parent may still
	// annotate their own relationship).
	if !isSelf && authorize.IsFullGuardianRole(link.GuardianRole) {
		return ErrGuardianRoleManaged
	}
	// A contact-only profile that ALSO serves a child outside the caller's
	// family is the school's to manage (its contact is redacted on read), so a
	// note edit is refused here too — same containment guard as the contact
	// path. Per-child though the note is, the contact-coupled rule keeps "note
	// editable iff contact editable" intact. Only notes need this; the flags
	// are per-child authority already gated above.
	if edit.editsDetails && !isSelf {
		return s.refuseSharedContactNote(ctx, profile.ID, callerStudents)
	}
	return nil
}

// refuseSharedContactNote refuses a note on a contact that also serves a child
// outside the caller's family.
func (s *Service) refuseSharedContactNote(ctx context.Context, profileID int64, callerStudents map[int64]bool) error {
	escapes, err := s.profileEscapesFamily(ctx, profileID, callerStudents)
	if err != nil {
		return err
	}
	if escapes {
		return ErrGuardianSharedAcrossFamilies
	}
	return nil
}

// refusePickupFlagEdit guards the safety-critical can_pickup /
// is_emergency_contact flags.
func refusePickupFlagEdit(profile *usersModels.GuardianProfile, link *usersModels.StudentGuardian) error {
	// can_pickup / is_emergency_contact grant safety-critical pickup/emergency
	// AUTHORITY. A parent may set them only for guardians WITHOUT their own
	// portal account (helpers like grandma). A guardian who holds their own
	// account owns their standing: nobody else may change it here, and a
	// parent may not grant it to themselves either (the caller's own profile
	// is an account holder, so this also blocks self-granting). This closes
	// the custody-dispute griefing vector; the school sets these flags for
	// account-holding guardians. "Account holder" derives from has_account OR
	// account_id (HasPortalAccount) so a drifted row can't slip the safety
	// guard (#1667 review).
	if profile.HasPortalAccount() {
		return ErrGuardianHasOwnAccount
	}
	// A social worker's pickup/emergency standing is set by the school, not by
	// a parent (mirrors the read-side CanManagePickup gate).
	if link.GuardianRole == authorize.GuardianRoleSocialWorker {
		return ErrGuardianSocialWorkerManaged
	}
	return nil
}

// refusePickupNoteEdit guards another guardian's per-child pickup note.
func refusePickupNoteEdit(profile *usersModels.GuardianProfile, link *usersModels.StudentGuardian) error {
	// The per-child pickup note follows CONTACT-edit eligibility (#1667 review):
	// a parent may edit it only for a guardian whose contact data they may also
	// edit/see — their own profile, or an account-less helper that is not a
	// social worker, not a full guardian, and not shared with another family.
	// For every gated guardian the contact is read-only/redacted, so the note
	// is read-only here too ("can't see/edit the contact ⇒ can't edit the
	// note"). These mirror the UpdateGuardianContact guards exactly, closing the
	// UI-vs-service split the review flagged. Flags above are governed
	// separately by pickup.manage.
	if profile.HasPortalAccount() {
		return ErrGuardianHasOwnAccount
	}
	if link.GuardianRole == authorize.GuardianRoleSocialWorker {
		return ErrGuardianSocialWorkerManaged
	}
	return nil
}

// writeRelationshipEdit writes ONLY the columns this request actually changed.
// A full-row update would re-send guardian_role, permissions,
// relationship_type, etc. with the values loaded before any concurrent staff
// edit; scoping to the pickup fields removes that clobber. But scoping to a
// FIXED can_pickup/is_emergency_contact/pickup_notes triple is still too wide:
// a note-only edit would re-send the two flag columns with their stale read
// values and clobber a staff toggle made between our read and write (the
// guardian_profiles lock serializes parent-vs-parent edits, but the staff path
// edits the students_guardians row without taking it). Building the patch from
// exactly the supplied fields makes the write-set equal the touched-set, so an
// untouched flag is never written.
func (s *Service) writeRelationshipEdit(ctx context.Context, link *usersModels.StudentGuardian, input GuardianRelationshipInput) error {
	patch := GuardianLinkPickupPatch{LinkID: link.ID}
	if input.CanPickup != nil {
		link.CanPickup = *input.CanPickup
		patch.CanPickup = input.CanPickup
	}
	if input.IsEmergencyContact != nil {
		link.IsEmergencyContact = *input.IsEmergencyContact
		patch.IsEmergencyContact = input.IsEmergencyContact
	}
	if input.PickupNotes != nil {
		trimmed := strings.TrimSpace(*input.PickupNotes)
		if trimmed == "" {
			link.PickupNotes = nil
		} else {
			link.PickupNotes = &trimmed
		}
		patch.SetPickupNotes = true
		patch.PickupNotes = link.PickupNotes
	}
	// The patch is guaranteed non-empty: the all-nil input guard at the top of
	// UpdateGuardianRelationship already returned ErrGuardianNoChange.
	updated, err := s.Guardians.PatchGuardianLinkPickup(ctx, patch)
	if err != nil {
		return err
	}
	if updated == 0 {
		// The relationship row vanished between our read and write (e.g. the
		// link was removed concurrently). Surface it as not-linked rather than
		// silently reporting success.
		return ErrGuardianNotLinked
	}
	return nil
}

// projectEditedRelationship reads the phones and family reach for the response
// and refreshes the child's live view after the commit.
func (s *Service) projectEditedRelationship(ctx context.Context, child *Child, profile *usersModels.GuardianProfile, link *usersModels.StudentGuardian, edit relationshipEdit, callerStudents map[int64]bool) (*ChildGuardian, error) {
	phones, err := s.GuardianPhoneRepo.FindByGuardianID(ctx, profile.ID)
	if err != nil {
		return nil, err
	}
	escapes, err := s.profileEscapesFamily(ctx, profile.ID, callerStudents)
	if err != nil {
		return nil, err
	}
	result := projectChildGuardian(profile, link, phones, edit.accountID,
		child.HasPermission(authorize.GuardianPermissionGuardianEdit),
		child.HasPermission(authorize.GuardianPermissionPickupManage), escapes)

	capturedTenant := child.TenantID
	tenant.RegisterAfterCommit(ctx, func() {
		s.broadcastStudentUpdated(capturedTenant, edit.studentID)
	})
	return result, nil
}

// guardianManagementEnabled reports whether the child's school has the guardian
// contact/pickup management feature switched on
// (operations.parent_guardian_management_enabled).
func (s *Service) guardianManagementEnabled(ctx context.Context, tenantID int64) (bool, error) {
	enabled, err := s.Settings.ResolveBoolForTenant(ctx, tenantID, configModels.KeyParentGuardianManagementEnabled)
	if err != nil {
		return false, fmt.Errorf("parent: resolve guardian-management setting: %w", err)
	}
	return enabled, nil
}

// requireGuardianManagementEnabled returns ErrGuardianManagementDisabled when the
// child's school has switched the feature off, so write paths refuse uniformly.
func (s *Service) requireGuardianManagementEnabled(ctx context.Context, tenantID int64) error {
	enabled, err := s.guardianManagementEnabled(ctx, tenantID)
	if err != nil {
		return err
	}
	if !enabled {
		return ErrGuardianManagementDisabled
	}
	return nil
}

// findChildGuardianLinkForUpdate returns the students_guardians row joining the
// child and guardian profile LOCKED FOR UPDATE for the current tx, or
// ErrGuardianNotLinked when none exists. Write paths use it (after locking the
// guardian profile) so a concurrent staff edit/delete of the relationship row
// cannot change role, account, or existence between the authorization checks and
// the write.
func (s *Service) findChildGuardianLinkForUpdate(ctx context.Context, studentID, guardianProfileID int64) (*usersModels.StudentGuardian, error) {
	link, err := s.StudentGuardianRepo.FindByStudentAndGuardianForUpdate(ctx, studentID, guardianProfileID)
	if err != nil {
		if errors.Is(err, usersModels.ErrStudentGuardianNotFound) {
			return nil, ErrGuardianNotLinked
		}
		return nil, err
	}
	return link, nil
}
