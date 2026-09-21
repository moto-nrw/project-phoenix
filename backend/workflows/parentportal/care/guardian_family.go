package care

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
)

// callerFamilyStudentSet returns the set of student IDs (in the current tenant)
// the account is a linked guardian of, regardless of parent-portal permission.
// It defines "the caller's family" for the cross-family containment guard: the
// question that guard answers is whether a contact-only profile ALSO serves a
// child the caller does not guard, so the set must include EVERY child the
// caller is linked to — not only the parent_portal.access ones
// ListChildrenForAccount returns. Filtering by portal access would wrongly flag
// a profile shared with the caller's own no-portal-access child as escaping and
// over-redact it. There is exactly one guardian_profile per (account, tenant),
// so the caller's family is precisely the students linked to that profile.
// MUST run inside the tenant transaction — both reads are RLS-scoped.
func (s *Service) callerFamilyStudentSet(ctx context.Context, accountID int64) (map[int64]bool, error) {
	profile, err := s.GuardianProfileRepo.FindByAccountID(ctx, accountID)
	if err != nil {
		// The caller resolved a permitted child in this tenant, so their profile
		// exists; treat a missing one defensively as an empty family (fail-safe:
		// every shared profile then reads as escaping rather than editable).
		if errors.Is(err, usersModels.ErrGuardianProfileNotFound) {
			return map[int64]bool{}, nil
		}
		return nil, err
	}
	links, err := s.StudentGuardianRepo.FindByGuardianProfileID(ctx, profile.ID)
	if err != nil {
		return nil, err
	}
	set := make(map[int64]bool, len(links))
	for _, link := range links {
		set[link.StudentID] = true
	}
	return set, nil
}

// profileEscapesFamily reports whether the guardian profile is linked to any
// student outside callerStudents — i.e. it also serves another family, so its
// contact data must not be editable by this caller. Must run inside the tenant
// transaction (FindByGuardianProfileID is tenant-filtered).
func (s *Service) profileEscapesFamily(ctx context.Context, guardianProfileID int64, callerStudents map[int64]bool) (bool, error) {
	links, err := s.StudentGuardianRepo.FindByGuardianProfileID(ctx, guardianProfileID)
	if err != nil {
		return false, err
	}
	for _, link := range links {
		if !callerStudents[link.StudentID] {
			return true, nil
		}
	}
	return false, nil
}

// profilesEscapingFamily is the batched counterpart of profileEscapesFamily: it
// reports, per guardian profile id, whether that profile is linked to any
// student outside callerStudents (i.e. it also serves another family). One
// query for the whole set. Must run inside the tenant transaction
// (ListLinkedChildrenForGuardians is tenant-filtered via RLS).
func (s *Service) profilesEscapingFamily(ctx context.Context, profileIDs []int64, callerStudents map[int64]bool) (map[int64]bool, error) {
	escapes := make(map[int64]bool, len(profileIDs))
	if len(profileIDs) == 0 {
		return escapes, nil
	}
	rows, err := s.StudentGuardianRepo.ListLinkedChildrenForGuardians(ctx, profileIDs)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		if !callerStudents[row.StudentID] {
			escapes[row.GuardianProfileID] = true
		}
	}
	return escapes, nil
}

// contactProtected reports whether the caller may neither read nor edit this
// guardian's personal contact data (email/phone/address). Protected when the
// guardian is not the caller AND one of: they hold their own portal account
// (a co-parent who manages it themselves), the relationship is a social-worker
// role (school-managed professional), or the profile also serves another family
// (shared contact). The caller's own profile is never protected.
//
// GDPR Datenminimierung: a reading guardian needs only the name plus the
// child's pickup/emergency arrangement, not a co-parent's or social worker's
// contact details. Read and write use the same predicate so the listing never
// shows data the caller could then blank-overwrite.
func contactProtected(profile *usersModels.GuardianProfile, link *usersModels.StudentGuardian, isSelf, sharedAcrossFamilies bool) bool {
	if isSelf {
		return false
	}
	return profile.HasPortalAccount() ||
		link.GuardianRole == authorize.GuardianRoleSocialWorker ||
		sharedAcrossFamilies
}

// projectChildGuardian builds the parent-facing projection of one guardian.
// sharedAcrossFamilies marks a contact-only profile that also serves a child
// outside the caller's family — editable only by the school, not this caller.
func projectChildGuardian(profile *usersModels.GuardianProfile, link *usersModels.StudentGuardian, phones []*usersModels.GuardianPhoneNumber, accountID int64, canEdit, canManage, sharedAcrossFamilies bool) *ChildGuardian {
	isSelf := profile.AccountID != nil && *profile.AccountID == accountID
	// "Account holder" derives from has_account OR account_id (HasPortalAccount),
	// matching every write guard, so a drifted row (account_id set,
	// has_account=false) is read-redacted and lock-marked exactly as the write
	// path refuses it — read and write agree (#1667 review).
	hasAccount := profile.HasPortalAccount()
	isSocialWorker := link.GuardianRole == authorize.GuardianRoleSocialWorker
	// A full guardian (primary/legal/co) is a real legal guardian, not a helper.
	// Self is exempt: the caller's own profile (always a full guardian) stays
	// editable via the isSelf branches below.
	isFullGuardian := !isSelf && authorize.IsFullGuardianRole(link.GuardianRole)
	protected := contactProtected(profile, link, isSelf, sharedAcrossFamilies)
	g := &ChildGuardian{
		GuardianProfileID:  profile.ID,
		StudentGuardianID:  link.ID,
		FirstName:          profile.FirstName,
		LastName:           profile.LastName,
		RelationshipType:   link.RelationshipType,
		IsPrimary:          link.IsPrimary,
		IsEmergencyContact: link.IsEmergencyContact,
		CanPickup:          link.CanPickup,
		HasAccount:         hasAccount,
		IsSelf:             isSelf,
		// Contact editing needs the edit permission and an unprotected target that
		// is not a full guardian (real legal guardians are school/self-managed).
		// The caller's own profile is always editable regardless of reach/role.
		CanEditContact: canEdit && !protected && !isFullGuardian,
		// Pickup/emergency flags may be managed only for contact-only helpers
		// (grandma): an account holder's standing is theirs and the school's, a
		// social worker's is the school's, and a full guardian's is the
		// guardian's/school's even without an account. Mirrors the write guard in
		// UpdateGuardianRelationship so the UI never offers a control the backend
		// rejects.
		CanManagePickup: canManage && !hasAccount && !isSocialWorker && !isFullGuardian,
		// Surface exactly one "why is this read-only" explanation to a caller who
		// could otherwise edit (own account > social worker > full guardian >
		// shared). For a caller without edit rights none is the reason the
		// affordance is absent.
		ContactLockedOwnAccount:   canEdit && !isSelf && hasAccount,
		ContactLockedSocialWorker: canEdit && !isSelf && !hasAccount && isSocialWorker,
		ContactLockedFullGuardian: canEdit && !isSelf && !hasAccount && !isSocialWorker && isFullGuardian,
		ContactLockedShared:       canEdit && !isSelf && !hasAccount && !isSocialWorker && !isFullGuardian && sharedAcrossFamilies,
	}
	// PickupNotes is a per-child annotation the reading guardian authors about
	// their own child's care; it is shown even for protected guardians.
	if link.PickupNotes != nil {
		g.PickupNotes = *link.PickupNotes
	}
	g.Phones = make([]GuardianPhone, 0, len(phones))
	if protected {
		// Redact personal contact identifiers: name + flags only.
		return g
	}
	applyGuardianContactDetails(g, profile, phones)
	return g
}

// applyGuardianContactDetails copies the personal contact identifiers (email,
// address, phones) of an unprotected guardian into the projection.
func applyGuardianContactDetails(g *ChildGuardian, profile *usersModels.GuardianProfile, phones []*usersModels.GuardianPhoneNumber) {
	if profile.Email != nil {
		g.Email = *profile.Email
	}
	if profile.AddressStreet != nil {
		g.AddressStreet = *profile.AddressStreet
	}
	if profile.AddressCity != nil {
		g.AddressCity = *profile.AddressCity
	}
	if profile.AddressPostalCode != nil {
		g.AddressPostalCode = *profile.AddressPostalCode
	}
	for _, p := range phones {
		label := ""
		if p.Label != nil {
			label = *p.Label
		}
		g.Phones = append(g.Phones, GuardianPhone{
			PhoneNumber: p.PhoneNumber,
			PhoneType:   string(p.PhoneType),
			Label:       label,
			IsPrimary:   p.IsPrimary,
		})
	}
}
