package care

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// UpdateGuardianContact replaces the contact data (name, email, address,
// phones) of a contact-only guardian of the child, or the caller's own
// profile. Requires parent_portal.guardian.edit. A guardian holding their own
// portal account is rejected (ErrGuardianHasOwnAccount) unless it is the caller.
func (s *Service) UpdateGuardianContact(ctx context.Context, accountID, studentID, guardianProfileID int64, input GuardianContactInput) (*ChildGuardian, error) {
	if err := validateContactInput(&input); err != nil {
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
	if s.Guardians == nil {
		return nil, errors.New("parent: guardian records are not configured")
	}
	var result *ChildGuardian
	txErr := InTenant(ctx, child.TenantID, func(txCtx context.Context) error {
		updated, err := s.updateGuardianContactInTx(txCtx, child, accountID, studentID, guardianProfileID, input)
		result = updated
		return err
	})
	if txErr != nil {
		if errors.Is(txErr, ErrGuardianEmailConflict) {
			return nil, ErrGuardianEmailConflict
		}
		return nil, txErr
	}

	s.Logger.Info("parent updated guardian contact",
		slog.Int64("account_id", accountID),
		slog.Int64("student_id", studentID),
		slog.Int64("guardian_profile_id", guardianProfileID),
		slog.Int64("tenant_id", child.TenantID),
	)
	return result, nil
}

// updateGuardianContactInTx locks, authorizes and rewrites one guardian's
// contact inside the child's tenant unit of work.
func (s *Service) updateGuardianContactInTx(ctx context.Context, child *Child, accountID, studentID, guardianProfileID int64, input GuardianContactInput) (*ChildGuardian, error) {
	if err := s.RequireCareRunningForUpdate(ctx, studentID); err != nil {
		return nil, err
	}
	target, err := s.lockEditableGuardianContact(ctx, accountID, studentID, guardianProfileID)
	if err != nil {
		return nil, err
	}
	profile := target.profile

	// Snapshot the pre-edit contact state (incl. phones) so the change can be
	// audited field-by-field after the wholesale replace below.
	oldPhones, err := s.GuardianPhoneRepo.FindByGuardianID(ctx, profile.ID)
	if err != nil {
		return nil, err
	}
	before := snapshotGuardianContact(profile, oldPhones)

	// Reject an email already owned by a DIFFERENT guardian profile before we
	// write, returning a friendly ErrGuardianEmailConflict. The
	// idx_guardian_profiles_tenant_email index is UNIQUE on
	// (tenant_id, LOWER(email)) (migration 1.15.145), matching how every
	// guardian email lookup (FindByEmail, invite/account matching) compares —
	// case-insensitively. That functional index is the ATOMIC guarantee: two
	// concurrent edits setting case variants of the same address on different
	// profiles can both pass this precheck, but only one can commit — People
	// Directory reports the loser as the same conflict. This precheck is the
	// friendly fast path; the index is the race-closing backstop. FindByEmail
	// runs in the tenant tx, so it is RLS-scoped to this school.
	if err := s.requireGuardianEmailFree(ctx, input.Email, guardianProfileID); err != nil {
		return nil, err
	}

	applyContactInput(profile, &input)
	if err := s.updateGuardianProfile(ctx, profile); err != nil {
		return nil, err
	}
	if err := s.replaceGuardianPhones(ctx, profile, input.Phones); err != nil {
		return nil, err
	}

	phones, err := s.GuardianPhoneRepo.FindByGuardianID(ctx, profile.ID)
	if err != nil {
		return nil, err
	}
	if err := s.auditContactChanges(ctx, accountID, studentID, guardianProfileID, before, profile, phones); err != nil {
		return nil, err
	}
	// Containment passed (or self), so the edited profile does not escape the
	// caller's family: not shared-locked from this caller's view.
	result := projectChildGuardian(profile, target.link, phones, accountID, true,
		child.HasPermission(authorize.GuardianPermissionPickupManage), false)
	s.broadcastProfileChildrenAfterCommit(ctx, child.TenantID, target.profileLinks)
	return result, nil
}

// editableGuardianContact is a guardian profile the caller may edit, locked
// together with its relationship to the child.
type editableGuardianContact struct {
	profile      *usersModels.GuardianProfile
	link         *usersModels.StudentGuardian
	profileLinks []*usersModels.StudentGuardian
}

// lockEditableGuardianContact locks the profile, then its relationship row,
// and refuses every guardian the caller may not edit.
func (s *Service) lockEditableGuardianContact(ctx context.Context, accountID, studentID, guardianProfileID int64) (editableGuardianContact, error) {
	// Lock the profile row so concurrent contact edits of the same guardian
	// serialize: the read-modify-write below plus the wholesale phone-list
	// replace (delete-all then re-insert) would otherwise race under
	// read-committed and could lose an update or duplicate phone rows.
	if err := s.GuardianProfileRepo.LockByIDForUpdate(ctx, guardianProfileID); err != nil {
		if errors.Is(err, usersModels.ErrGuardianProfileNotFound) {
			return editableGuardianContact{}, ErrGuardianNotLinked
		}
		return editableGuardianContact{}, err
	}
	// Then lock the relationship row itself FOR UPDATE. The role check below
	// reads link.GuardianRole; without this lock a concurrent staff edit could
	// promote/demote or remove the relationship between that check and the
	// contact write (the staff path edits the students_guardians row without
	// taking the profile lock above). Locking the row serializes against it —
	// a plain staff UPDATE/DELETE blocks on this lock — so the authorization
	// decision is made on a row that cannot change until we commit. Profile is
	// locked before the link in BOTH write paths, so the lock order is
	// consistent and cannot deadlock.
	link, err := s.findChildGuardianLinkForUpdate(ctx, studentID, guardianProfileID)
	if err != nil {
		return editableGuardianContact{}, err
	}
	callerStudents, err := s.callerFamilyStudentSet(ctx, accountID)
	if err != nil {
		return editableGuardianContact{}, err
	}
	profile, err := s.findLinkedGuardianProfile(ctx, guardianProfileID)
	if err != nil {
		return editableGuardianContact{}, err
	}
	isSelf := profile.AccountID != nil && *profile.AccountID == accountID
	if err := refuseContactEdit(profile, link, isSelf); err != nil {
		return editableGuardianContact{}, err
	}
	// A contact edit propagates to every child the profile serves (intended for
	// siblings). Load every child this profile is linked to: it drives both the
	// cross-family containment guard (a contact-only profile that also serves a
	// child outside the caller's family is the school's to edit, not this
	// caller's) and the post-commit broadcast (every sibling's parent view must
	// refresh, not just the one the edit was routed through).
	profileLinks, err := s.StudentGuardianRepo.FindByGuardianProfileID(ctx, guardianProfileID)
	if err != nil {
		return editableGuardianContact{}, err
	}
	if !isSelf && !profileStaysInFamily(profileLinks, callerStudents) {
		return editableGuardianContact{}, ErrGuardianSharedAcrossFamilies
	}
	return editableGuardianContact{profile: profile, link: link, profileLinks: profileLinks}, nil
}

// profileStaysInFamily reports whether every child the profile serves belongs
// to the caller's family.
func profileStaysInFamily(profileLinks []*usersModels.StudentGuardian, callerStudents map[int64]bool) bool {
	for _, pl := range profileLinks {
		if !callerStudents[pl.StudentID] {
			return false
		}
	}
	return true
}

// refuseContactEdit applies the contact-edit eligibility: only contact-only
// helpers or the caller's own profile are editable.
func refuseContactEdit(profile *usersModels.GuardianProfile, link *usersModels.StudentGuardian, isSelf bool) error {
	// Editing another account holder's personal data is forbidden — the UI is
	// not the boundary, the backend is. "Account holder" is derived from
	// has_account OR account_id (HasPortalAccount): isSelf already keys off
	// account_id, so a drifted row (account_id set, has_account=false) must be
	// treated as account-backed here too, or another parent could edit a
	// helper-with-account's contact data (#1667 review).
	if profile.HasPortalAccount() && !isSelf {
		return ErrGuardianHasOwnAccount
	}
	// A social worker is a school-managed professional contact; a parent may
	// not rewrite their personal data (mirrors the read redaction).
	if !isSelf && link.GuardianRole == authorize.GuardianRoleSocialWorker {
		return ErrGuardianSocialWorkerManaged
	}
	// A full guardian (primary/legal/co) is a real legal guardian, not a
	// helper. Even without their own portal account they are not another
	// parent's to edit (a non-registered co-parent must not be editable just
	// because they lack a login). Only the guardian themselves or the school
	// manages them. Helper roles (pickup_only/emergency_contact/custom) stay
	// editable — that is the intended account-less helper case.
	if !isSelf && authorize.IsFullGuardianRole(link.GuardianRole) {
		return ErrGuardianRoleManaged
	}
	return nil
}

// broadcastProfileChildrenAfterCommit refreshes the live view of every child
// the edited profile serves, once the edit has committed.
func (s *Service) broadcastProfileChildrenAfterCommit(ctx context.Context, tenantID int64, profileLinks []*usersModels.StudentGuardian) {
	affectedStudents := make([]int64, 0, len(profileLinks))
	for _, pl := range profileLinks {
		affectedStudents = append(affectedStudents, pl.StudentID)
	}
	tenant.RegisterAfterCommit(ctx, func() {
		for _, sid := range affectedStudents {
			s.broadcastStudentUpdated(tenantID, sid)
		}
	})
}

// guardianContactLink is the relationship as People Directory stores it. The
// permissions travel as their stored JSON object.
func guardianContactLink(link *usersModels.StudentGuardian) (GuardianContactLink, error) {
	var permissions json.RawMessage
	if link.Permissions != nil {
		encoded, err := json.Marshal(link.Permissions)
		if err != nil {
			return GuardianContactLink{}, err
		}
		permissions = encoded
	}
	return GuardianContactLink{
		StudentID: link.StudentID, GuardianProfileID: link.GuardianProfileID,
		RelationshipType: link.RelationshipType, GuardianRole: link.GuardianRole,
		IsPrimary: link.IsPrimary, IsEmergencyContact: link.IsEmergencyContact, CanPickup: link.CanPickup,
		PickupNotes: link.PickupNotes, EmergencyPriority: link.EmergencyPriority, IsPayer: link.IsPayer,
		Permissions: permissions,
	}, nil
}

// findLinkedGuardianProfile reads the locked profile; a missing one is
// reported as not linked.
func (s *Service) findLinkedGuardianProfile(ctx context.Context, guardianProfileID int64) (*usersModels.GuardianProfile, error) {
	profile, err := s.GuardianProfileRepo.FindByID(ctx, guardianProfileID)
	if errors.Is(err, usersModels.ErrGuardianProfileNotFound) {
		return nil, ErrGuardianNotLinked
	}
	return profile, err
}
