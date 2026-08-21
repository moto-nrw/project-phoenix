package parent

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/mail"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/driver/pgdriver"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/internal/strutil"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/moto-nrw/project-phoenix/models/base"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// Field length bounds for guardian contact edits. Generous but finite so a
// buggy or hostile client can't push unbounded strings into the contact table.
const (
	maxGuardianNameLen  = 100
	maxGuardianEmailLen = 254
	maxGuardianAddrLen  = 200
	maxGuardianPhoneLen = 40
	maxGuardianLabelLen = 50
	maxGuardianPhones   = 5
	maxGuardianNotesLen = 500
)

// Sentinel errors the HTTP layer maps to stable status codes via
// renderParentWriteError. Part of the package contract.
var (
	// ErrGuardianNotLinked means the named guardian profile is not a guardian
	// of the resolved child. Mapped to 404/403 so it never leaks whether the
	// profile exists for another child.
	ErrGuardianNotLinked = errors.New("parent: guardian not linked to child")
	// ErrGuardianHasOwnAccount means the target guardian holds their own portal
	// account, so another parent may not edit their personal contact data. The
	// account holder edits their own data through their own session.
	ErrGuardianHasOwnAccount = errors.New("parent: guardian with own portal account cannot be edited by another parent")
	// ErrGuardianContactInvalid means the submitted contact payload failed
	// validation (empty name, bad email, oversized field, invalid phone).
	ErrGuardianContactInvalid = errors.New("parent: invalid guardian contact input")
	// ErrGuardianEmailConflict means another guardian profile in this tenant
	// already uses the submitted email address.
	ErrGuardianEmailConflict = errors.New("parent: guardian email already in use")
	// ErrGuardianRelationshipInvalid means the submitted pickup/relationship
	// payload failed validation (e.g. priority out of range).
	ErrGuardianRelationshipInvalid = errors.New("parent: invalid guardian relationship input")
	// ErrGuardianSharedAcrossFamilies means the contact-only profile the caller
	// tried to edit is also linked to a child outside the caller's family (e.g. a
	// social worker serving several unrelated children). Propagating a contact
	// edit to the caller's own children is intended; rewriting a contact that
	// also serves another family is not the caller's to do — the school edits it.
	ErrGuardianSharedAcrossFamilies = errors.New("parent: guardian shared with other families cannot be edited")
	// ErrGuardianSocialWorkerManaged means the target relationship is a
	// social-worker role: a school-managed professional contact. A parent may
	// neither read nor rewrite their personal contact data (GDPR: the very
	// presence of a social worker is sensitive, and their contact details are
	// staff data the school mediates, not parent-consumable).
	ErrGuardianSocialWorkerManaged = errors.New("parent: social-worker contact is managed by the school")
	// ErrGuardianRoleManaged means the target relationship is a full guardian
	// role (primary/legal/co). These are real legal guardians, not helpers: a
	// parent may not edit their contact data or pickup/emergency authority even
	// when the guardian has no portal account yet. Only the guardian themselves
	// (once registered) or the school manages them. The account-level guard
	// (ErrGuardianHasOwnAccount) covers registered guardians; this closes the gap
	// for full guardians who simply have not registered (#1667).
	ErrGuardianRoleManaged = errors.New("parent: full guardian is managed by the school")
	// ErrGuardianNoChange means a relationship update carried no editable field.
	ErrGuardianNoChange = errors.New("parent: no editable field supplied")
	// ErrGuardianManagementDisabled means the child's school turned off the
	// guardian contact/pickup management feature
	// (operations.parent_guardian_management_enabled). Reads still list
	// guardians; writes are refused regardless of guardian permission.
	ErrGuardianManagementDisabled = errors.New("parent: guardian management disabled for tenant")
)

// ChildGuardian is the parent-facing projection of one guardian linked to a
// child: contact data (profile-level, shared across siblings) plus the
// per-child pickup/emergency relationship, annotated with what the caller may
// edit. IDs are int64 here; the HTTP layer stringifies them.
type ChildGuardian struct {
	GuardianProfileID  int64
	StudentGuardianID  int64
	FirstName          string
	LastName           string
	Email              string
	Phones             []GuardianPhone
	AddressStreet      string
	AddressCity        string
	AddressPostalCode  string
	RelationshipType   string
	IsPrimary          bool
	IsEmergencyContact bool
	CanPickup          bool
	PickupNotes        string
	// HasAccount is true when the guardian holds their own portal login. Such
	// guardians' contact data is read-only to other parents.
	HasAccount bool
	// IsSelf marks the guardian profile belonging to the requesting account.
	IsSelf bool
	// CanEditContact reports whether the caller may edit this guardian's
	// contact data (profile fields + phones + per-child note/priority).
	CanEditContact bool
	// CanManagePickup reports whether the caller may toggle this guardian's
	// per-child can_pickup / is_emergency_contact flags.
	CanManagePickup bool
	// ContactLockedOwnAccount is true when the caller has contact-edit
	// permission but this guardian holds their own portal account (and is not
	// the caller), so their contact data is intentionally read-only here. It
	// distinguishes "you may not edit this one because they manage it
	// themselves" from "you have no edit rights at all" — both leave
	// CanEditContact false, but only the former warrants an explanation in the
	// UI.
	ContactLockedOwnAccount bool
	// ContactLockedShared is true when the caller has contact-edit permission and
	// this is a contact-only guardian, but the profile is also linked to a child
	// outside the caller's family, so editing it (which would propagate to that
	// other family) is intentionally refused here. Like ContactLockedOwnAccount,
	// it explains an absent edit affordance to a caller who otherwise has rights.
	ContactLockedShared bool
	// ContactLockedSocialWorker is true when the caller has contact-edit
	// permission but this relationship is a social-worker role, so the contact is
	// school-managed and read-only here. Like the other lock reasons it explains
	// an absent edit affordance; unlike them the contact fields are also redacted.
	ContactLockedSocialWorker bool
	// ContactLockedFullGuardian is true when the caller has contact-edit
	// permission but this guardian holds a full guardian role (primary/legal/co)
	// without their own portal account, so they are a real legal guardian managed
	// by themselves or the school, not another parent. Contact stays visible (not
	// redacted) but is read-only here.
	ContactLockedFullGuardian bool
}

// GuardianPhone is one phone number in the parent-facing projection.
type GuardianPhone struct {
	PhoneNumber string
	PhoneType   string
	Label       string
	IsPrimary   bool
}

// GuardianContactInput is the validated payload for a contact edit. Profile
// fields are replaced wholesale; Phones replaces the entire phone list.
type GuardianContactInput struct {
	FirstName         string
	LastName          string
	Email             *string
	AddressStreet     *string
	AddressCity       *string
	AddressPostalCode *string
	Phones            []GuardianPhoneInput
}

// GuardianPhoneInput is one phone row in a contact edit.
type GuardianPhoneInput struct {
	PhoneNumber string
	PhoneType   string
	Label       *string
	IsPrimary   bool
}

// GuardianRelationshipInput is the validated payload for a per-child pickup /
// relationship edit. Every field is optional: a nil field is left unchanged.
type GuardianRelationshipInput struct {
	CanPickup          *bool
	IsEmergencyContact *bool
	PickupNotes        *string
}

// CreateGuardianContactInput creates an accountless person and their
// relationship to one child. App access is granted only by the separate
// related-account invitation flow.
type CreateGuardianContactInput struct {
	Contact            GuardianContactInput
	RelationshipType   string
	CanPickup          bool
	IsEmergencyContact bool
	PickupNotes        *string
}

// CreateGuardianContact adds an accountless contact to the child.
func (s *service) CreateGuardianContact(ctx context.Context, accountID, studentID int64, input CreateGuardianContactInput) (*ChildGuardian, error) {
	if err := validateCreateGuardianContactInput(&input); err != nil {
		return nil, err
	}
	child, err := s.resolvePermittedChild(ctx, accountID, studentID, authorize.GuardianPermissionGuardianEdit)
	if err != nil {
		return nil, err
	}
	// A child whose care at this school has ended keeps read access to what
	// happened, but nothing new can be submitted for them (#2487).
	if err := child.requireCareRunning(); err != nil {
		return nil, err
	}
	if err := s.requireGuardianManagementEnabled(ctx, child.tenantID); err != nil {
		return nil, err
	}
	if (input.CanPickup || input.IsEmergencyContact) && !child.hasPermission(authorize.GuardianPermissionPickupManage) {
		return nil, ErrGuardianPermissionDenied
	}

	var result *ChildGuardian
	txErr := tenant.WithTenantTx(ctx, s.DB, child.tenantID, func(txCtx context.Context, _ bun.Tx) error {
		if input.Contact.Email != nil {
			email := strings.TrimSpace(*input.Contact.Email)
			if email != "" {
				existing, findErr := s.GuardianProfileRepo.FindByEmail(txCtx, email)
				if findErr != nil && !errors.Is(findErr, usersModels.ErrGuardianProfileNotFound) {
					return findErr
				}
				if existing != nil {
					return ErrGuardianEmailConflict
				}
			}
		}

		profile := &usersModels.GuardianProfile{
			PreferredContactMethod: "phone",
			LanguagePreference:     "de",
		}
		profile.SetTenantID(child.tenantID)
		applyContactInput(profile, &input.Contact)
		if profile.Email != nil {
			profile.PreferredContactMethod = "email"
		}
		if err := s.GuardianProfileRepo.Create(txCtx, profile); err != nil {
			return err
		}
		if err := s.replaceGuardianPhones(txCtx, profile, input.Contact.Phones); err != nil {
			return err
		}

		link := &usersModels.StudentGuardian{
			StudentID:          studentID,
			GuardianProfileID:  profile.ID,
			RelationshipType:   strings.TrimSpace(input.RelationshipType),
			IsEmergencyContact: input.IsEmergencyContact,
			CanPickup:          input.CanPickup,
			PickupNotes:        strutil.TrimPtrToNil(input.PickupNotes),
			EmergencyPriority:  1,
		}
		authorize.ApplyDefaultStudentGuardianRole(link)
		link.SetTenantID(child.tenantID)
		inserted, err := s.StudentGuardianRepo.LinkIfNotExists(txCtx, link)
		if err != nil {
			return err
		}
		if !inserted {
			return fmt.Errorf("parent: new guardian profile %d was already linked to student %d", profile.ID, studentID)
		}

		phones, err := s.GuardianPhoneRepo.FindByGuardianID(txCtx, profile.ID)
		if err != nil {
			return err
		}
		if err := s.auditContactChanges(txCtx, child.tenantID, accountID, studentID, profile.ID, guardianContactSnapshot{}, profile, phones); err != nil {
			return err
		}
		flagInput := GuardianRelationshipInput{
			CanPickup:          &input.CanPickup,
			IsEmergencyContact: &input.IsEmergencyContact,
		}
		if err := s.auditPickupFlagChanges(txCtx, child.tenantID, accountID, studentID, profile.ID, flagInput, false, false); err != nil {
			return err
		}

		result = projectChildGuardian(profile, link, phones, accountID, true,
			child.hasPermission(authorize.GuardianPermissionPickupManage), false)
		capturedTenant := child.tenantID
		tenant.RegisterAfterCommit(txCtx, func() {
			s.broadcastStudentUpdated(capturedTenant, studentID)
		})
		return nil
	})
	if txErr != nil {
		if isGuardianEmailUniqueViolation(txErr) {
			return nil, ErrGuardianEmailConflict
		}
		return nil, txErr
	}

	s.Logger.Info("parent created guardian contact",
		slog.Int64("account_id", accountID),
		slog.Int64("student_id", studentID),
		slog.Int64("guardian_profile_id", result.GuardianProfileID),
		slog.Int64("tenant_id", child.tenantID),
	)
	return result, nil
}

// ListChildGuardians returns every guardian linked to the child with contact +
// pickup detail and the caller's per-guardian edit capabilities. Authorization
// only (parent_portal.access).
func (s *service) ListChildGuardians(ctx context.Context, accountID, studentID int64) ([]*ChildGuardian, error) {
	child, err := s.resolvePermittedChild(ctx, accountID, studentID, authorize.GuardianPermissionPortalAccess)
	if err != nil {
		return nil, err
	}
	canEdit := child.hasPermission(authorize.GuardianPermissionGuardianEdit)
	canManage := child.hasPermission(authorize.GuardianPermissionPickupManage)
	// A school can disable the whole feature: when off, the list still shows
	// guardians (read) but advertises no edit affordances, so the UI hides them.
	if canEdit || canManage {
		enabled, err := s.guardianManagementEnabled(ctx, child.tenantID)
		if err != nil {
			return nil, err
		}
		if !enabled {
			canEdit = false
			canManage = false
		}
	}
	var out []*ChildGuardian
	txErr := tenant.WithTenantTx(ctx, s.DB, child.tenantID, func(txCtx context.Context, _ bun.Tx) error {
		callerStudents, err := s.callerFamilyStudentSet(txCtx, accountID)
		if err != nil {
			return err
		}
		links, err := s.StudentGuardianRepo.FindByStudentID(txCtx, studentID)
		if err != nil {
			return err
		}
		// Batch-load profiles, phones, and cross-family reach for every guardian
		// of the child in three queries total, rather than three per guardian.
		profileIDs := make([]int64, 0, len(links))
		for _, link := range links {
			profileIDs = append(profileIDs, link.GuardianProfileID)
		}
		profiles, err := s.GuardianProfileRepo.FindByIDs(txCtx, profileIDs)
		if err != nil {
			return err
		}
		phonesByProfile, err := s.GuardianPhoneRepo.FindByGuardianIDs(txCtx, profileIDs)
		if err != nil {
			return err
		}
		escapesByProfile, err := s.profilesEscapingFamily(txCtx, profileIDs, callerStudents)
		if err != nil {
			return err
		}
		out = make([]*ChildGuardian, 0, len(links))
		for _, link := range links {
			profile, ok := profiles[link.GuardianProfileID]
			if !ok {
				// A link pointing to a missing profile is a data-integrity fault
				// (orphaned students_guardians row), not an authorization outcome.
				// Surface it as an internal error (→ 500, caught by 5xx alerting)
				// instead of masking it as ErrGuardianNotLinked (403), which would
				// hide the inconsistency from monitoring.
				s.Logger.Error("parent child guardian link points to missing profile",
					slog.Int64("tenant_id", child.tenantID),
					slog.Int64("student_id", studentID),
					slog.Int64("student_guardian_id", link.ID),
					slog.Int64("guardian_profile_id", link.GuardianProfileID),
				)
				return fmt.Errorf("parent: student_guardian %d references missing guardian profile %d", link.ID, link.GuardianProfileID)
			}
			out = append(out, projectChildGuardian(profile, link, phonesByProfile[profile.ID], accountID, canEdit, canManage, escapesByProfile[profile.ID]))
		}
		return nil
	})
	if txErr != nil {
		return nil, txErr
	}
	return out, nil
}

// UpdateGuardianContact replaces the contact data (name, email, address,
// phones) of a contact-only guardian of the child, or the caller's own
// profile. Requires parent_portal.guardian.edit. A guardian holding their own
// portal account is rejected (ErrGuardianHasOwnAccount) unless it is the caller.
func (s *service) UpdateGuardianContact(ctx context.Context, accountID, studentID, guardianProfileID int64, input GuardianContactInput) (*ChildGuardian, error) {
	if err := validateContactInput(&input); err != nil {
		return nil, err
	}
	child, err := s.resolvePermittedChild(ctx, accountID, studentID, authorize.GuardianPermissionGuardianEdit)
	if err != nil {
		return nil, err
	}
	// A child whose care at this school has ended keeps read access to what
	// happened, but nothing new can be submitted for them (#2487).
	if err := child.requireCareRunning(); err != nil {
		return nil, err
	}
	if err := s.requireGuardianManagementEnabled(ctx, child.tenantID); err != nil {
		return nil, err
	}
	var result *ChildGuardian
	txErr := tenant.WithTenantTx(ctx, s.DB, child.tenantID, func(txCtx context.Context, _ bun.Tx) error {
		// Lock the profile row so concurrent contact edits of the same guardian
		// serialize: the read-modify-write below plus the wholesale phone-list
		// replace (delete-all then re-insert) would otherwise race under
		// read-committed and could lose an update or duplicate phone rows.
		if err := s.GuardianProfileRepo.LockByIDForUpdate(txCtx, guardianProfileID); err != nil {
			if errors.Is(err, usersModels.ErrGuardianProfileNotFound) {
				return ErrGuardianNotLinked
			}
			return err
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
		link, err := s.findChildGuardianLinkForUpdate(txCtx, studentID, guardianProfileID)
		if err != nil {
			return err
		}
		callerStudents, err := s.callerFamilyStudentSet(txCtx, accountID)
		if err != nil {
			return err
		}
		profile, err := s.GuardianProfileRepo.FindByID(txCtx, guardianProfileID)
		if err != nil {
			if errors.Is(err, usersModels.ErrGuardianProfileNotFound) {
				return ErrGuardianNotLinked
			}
			return err
		}
		isSelf := profile.AccountID != nil && *profile.AccountID == accountID
		// Only contact-only guardians (or the caller's own profile) are editable.
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
		// A contact edit propagates to every child the profile serves (intended for
		// siblings). Reaching past the contact-only guard means !isSelf ⟺ a
		// contact-only profile; refuse it when the profile also serves a child
		// outside the caller's family, so one parent can't rewrite a contact shared
		// with another family.
		// Load every child this profile is linked to: it drives both the
		// cross-family containment guard (a contact-only profile that also serves a
		// child outside the caller's family is the school's to edit, not this
		// caller's) and the post-commit broadcast (a contact edit is shared across
		// all the profile's children, so every sibling's parent view must refresh,
		// not just the one the edit was routed through).
		profileLinks, err := s.StudentGuardianRepo.FindByGuardianProfileID(txCtx, guardianProfileID)
		if err != nil {
			return err
		}
		if !isSelf {
			for _, pl := range profileLinks {
				if !callerStudents[pl.StudentID] {
					return ErrGuardianSharedAcrossFamilies
				}
			}
		}

		// Snapshot the pre-edit contact state (incl. phones) so the change can be
		// audited field-by-field after the wholesale replace below.
		oldPhones, err := s.GuardianPhoneRepo.FindByGuardianID(txCtx, profile.ID)
		if err != nil {
			return err
		}
		before := snapshotGuardianContact(profile, oldPhones)

		// Reject an email already owned by a DIFFERENT guardian profile before we
		// write, returning a friendly ErrGuardianEmailConflict. The
		// idx_guardian_profiles_tenant_email index is now UNIQUE on
		// (tenant_id, LOWER(email)) (migration 1.15.145), matching how every
		// guardian email lookup (FindByEmail, invite/account matching) compares —
		// case-insensitively. That functional index is the ATOMIC guarantee: two
		// concurrent edits setting case variants of the same address (Oma@x.de /
		// oma@x.de) on different profiles can both pass this precheck, but only one
		// can commit — the second hits 23505 on the LOWER(email) index and is
		// mapped to ErrGuardianEmailConflict by the post-commit catch below. This
		// precheck is the friendly fast path; the index is the race-closing
		// backstop. Mirrors the staff-side guardian service precheck. FindByEmail
		// runs in the tenant tx, so it is RLS-scoped to this school.
		if input.Email != nil {
			if email := strings.TrimSpace(*input.Email); email != "" {
				existing, ferr := s.GuardianProfileRepo.FindByEmail(txCtx, email)
				// Only the clean not-found sentinel means "no conflicting profile".
				// Any other error (DB/RLS failure) MUST abort the edit, not be
				// swallowed as "no conflict": treating a lookup failure as success
				// would let the write commit while a genuine duplicate could exist,
				// and the case-insensitive index is the only remaining guard
				// (#1667 review).
				if ferr != nil && !errors.Is(ferr, usersModels.ErrGuardianProfileNotFound) {
					return ferr
				}
				if existing != nil && existing.ID != guardianProfileID {
					return ErrGuardianEmailConflict
				}
			}
		}

		applyContactInput(profile, &input)
		if err := s.GuardianProfileRepo.Update(txCtx, profile); err != nil {
			return err
		}
		if err := s.replaceGuardianPhones(txCtx, profile, input.Phones); err != nil {
			return err
		}

		phones, err := s.GuardianPhoneRepo.FindByGuardianID(txCtx, profile.ID)
		if err != nil {
			return err
		}
		if err := s.auditContactChanges(txCtx, child.tenantID, accountID, studentID, guardianProfileID, before, profile, phones); err != nil {
			return err
		}
		// Containment passed (or self), so the edited profile does not escape the
		// caller's family: not shared-locked from this caller's view.
		result = projectChildGuardian(profile, link, phones, accountID, true,
			child.hasPermission(authorize.GuardianPermissionPickupManage), false)

		affectedStudents := make([]int64, 0, len(profileLinks))
		for _, pl := range profileLinks {
			affectedStudents = append(affectedStudents, pl.StudentID)
		}
		capturedTenant := child.tenantID
		tenant.RegisterAfterCommit(txCtx, func() {
			for _, sid := range affectedStudents {
				s.broadcastStudentUpdated(capturedTenant, sid)
			}
		})
		return nil
	})
	if txErr != nil {
		if isGuardianEmailUniqueViolation(txErr) {
			return nil, ErrGuardianEmailConflict
		}
		return nil, txErr
	}

	s.Logger.Info("parent updated guardian contact",
		slog.Int64("account_id", accountID),
		slog.Int64("student_id", studentID),
		slog.Int64("guardian_profile_id", guardianProfileID),
		slog.Int64("tenant_id", child.tenantID),
	)
	return result, nil
}

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
func (s *service) UpdateGuardianRelationship(ctx context.Context, accountID, studentID, guardianProfileID int64, input GuardianRelationshipInput) (*ChildGuardian, error) {
	if input.CanPickup == nil && input.IsEmergencyContact == nil && input.PickupNotes == nil {
		return nil, ErrGuardianNoChange
	}
	if err := validateRelationshipInput(&input); err != nil {
		return nil, err
	}
	editsFlags := input.CanPickup != nil || input.IsEmergencyContact != nil
	editsDetails := input.PickupNotes != nil

	// Resolve on baseline portal access, then gate each field group by its own
	// permission. Resolving on the lower baseline (rather than guardian.edit) is
	// what lets a pickup.manage-only caller flip the flags.
	child, err := s.resolvePermittedChild(ctx, accountID, studentID, authorize.GuardianPermissionPortalAccess)
	if err != nil {
		return nil, err
	}
	// A child whose care at this school has ended keeps read access to what
	// happened, but nothing new can be submitted for them (#2487).
	if err := child.requireCareRunning(); err != nil {
		return nil, err
	}
	if err := s.requireGuardianManagementEnabled(ctx, child.tenantID); err != nil {
		return nil, err
	}
	if editsFlags && !child.hasPermission(authorize.GuardianPermissionPickupManage) {
		return nil, ErrGuardianPermissionDenied
	}
	if editsDetails && !child.hasPermission(authorize.GuardianPermissionGuardianEdit) {
		return nil, ErrGuardianPermissionDenied
	}
	var result *ChildGuardian
	txErr := tenant.WithTenantTx(ctx, s.DB, child.tenantID, func(txCtx context.Context, _ bun.Tx) error {
		// Lock the guardian profile row for the duration of the tx. The link
		// update below is a read-modify-write on the students_guardians row, and
		// the same profile lock guards the contact path; taking it here serializes
		// concurrent relationship edits (and relationship-vs-contact edits) of the
		// same guardian so independent field writes can't clobber each other in a
		// lost update under read-committed.
		if err := s.GuardianProfileRepo.LockByIDForUpdate(txCtx, guardianProfileID); err != nil {
			if errors.Is(err, usersModels.ErrGuardianProfileNotFound) {
				return ErrGuardianNotLinked
			}
			return err
		}
		callerStudents, err := s.callerFamilyStudentSet(txCtx, accountID)
		if err != nil {
			return err
		}
		// Lock the relationship row itself FOR UPDATE (profile-then-link order, as
		// in the contact path). The role/account guards below decide on
		// link.GuardianRole and the UpdateColumns write mutates this row; without
		// the lock a concurrent staff edit could promote/remove the relationship
		// after the guards pass and before the write, since the staff path edits
		// the students_guardians row without taking the profile lock above. Locking
		// the row makes the authorization decision and the write atomic against it.
		link, err := s.findChildGuardianLinkForUpdate(txCtx, studentID, guardianProfileID)
		if err != nil {
			return err
		}
		profile, err := s.GuardianProfileRepo.FindByID(txCtx, guardianProfileID)
		if err != nil {
			if errors.Is(err, usersModels.ErrGuardianProfileNotFound) {
				return ErrGuardianNotLinked
			}
			return err
		}
		isSelf := profile.AccountID != nil && *profile.AccountID == accountID
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
		if editsFlags && profile.HasPortalAccount() {
			return ErrGuardianHasOwnAccount
		}
		// A social worker's pickup/emergency standing is set by the school, not by
		// a parent (mirrors the read-side CanManagePickup gate).
		if editsFlags && link.GuardianRole == authorize.GuardianRoleSocialWorker {
			return ErrGuardianSocialWorkerManaged
		}
		// The per-child pickup note follows CONTACT-edit eligibility (#1667 review):
		// a parent may edit it only for a guardian whose contact data they may also
		// edit/see — their own profile, or an account-less helper that is not a
		// social worker, not a full guardian, and not shared with another family.
		// For every gated guardian the contact is read-only/redacted, so the note
		// is read-only here too ("can't see/edit the contact ⇒ can't edit the
		// note"). These mirror the UpdateGuardianContact guards exactly, closing the
		// UI-vs-service split the review flagged. Flags above are governed
		// separately by pickup.manage.
		if editsDetails && !isSelf {
			if profile.HasPortalAccount() {
				return ErrGuardianHasOwnAccount
			}
			if link.GuardianRole == authorize.GuardianRoleSocialWorker {
				return ErrGuardianSocialWorkerManaged
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
		if editsDetails && !isSelf {
			escapes, err := s.profileEscapesFamily(txCtx, profile.ID, callerStudents)
			if err != nil {
				return err
			}
			if escapes {
				return ErrGuardianSharedAcrossFamilies
			}
		}
		oldCanPickup := link.CanPickup
		oldEmergency := link.IsEmergencyContact
		// Write ONLY the columns this request actually changed. A full-row Update
		// would re-send guardian_role, permissions, relationship_type, etc. with the
		// values loaded before any concurrent staff edit; scoping to the pickup
		// fields removes that clobber. But scoping to a FIXED can_pickup/
		// is_emergency_contact/pickup_notes triple is still too wide: a note-only
		// edit would re-send the two flag columns with their stale read values and
		// clobber a staff toggle of can_pickup/is_emergency_contact made between our
		// read and write (the guardian_profiles lock above serializes parent-vs-
		// parent edits, but the staff path edits the students_guardians row without
		// taking it). Building the column set from exactly the supplied fields makes
		// the write-set equal the touched-set, so an untouched flag is never written.
		cols := make([]string, 0, 3)
		if input.CanPickup != nil {
			link.CanPickup = *input.CanPickup
			cols = append(cols, "can_pickup")
		}
		if input.IsEmergencyContact != nil {
			link.IsEmergencyContact = *input.IsEmergencyContact
			cols = append(cols, "is_emergency_contact")
		}
		if input.PickupNotes != nil {
			trimmed := strings.TrimSpace(*input.PickupNotes)
			if trimmed == "" {
				link.PickupNotes = nil
			} else {
				link.PickupNotes = &trimmed
			}
			cols = append(cols, "pickup_notes")
		}
		// cols is guaranteed non-empty: the all-nil input guard at the top of the
		// function already returned ErrGuardianNoChange.
		updated, err := s.StudentGuardianRepo.UpdateColumns(txCtx, link, cols...)
		if err != nil {
			return err
		}
		if updated == 0 {
			// The relationship row vanished between our read and write (e.g. the
			// link was removed concurrently). Surface it as not-linked rather than
			// silently reporting success.
			return ErrGuardianNotLinked
		}
		if err := s.auditPickupFlagChanges(txCtx, child.tenantID, accountID, studentID, guardianProfileID, input, oldCanPickup, oldEmergency); err != nil {
			return err
		}

		phones, err := s.GuardianPhoneRepo.FindByGuardianID(txCtx, profile.ID)
		if err != nil {
			return err
		}
		escapes, err := s.profileEscapesFamily(txCtx, profile.ID, callerStudents)
		if err != nil {
			return err
		}
		result = projectChildGuardian(profile, link, phones, accountID,
			child.hasPermission(authorize.GuardianPermissionGuardianEdit),
			child.hasPermission(authorize.GuardianPermissionPickupManage), escapes)

		capturedTenant := child.tenantID
		tenant.RegisterAfterCommit(txCtx, func() {
			s.broadcastStudentUpdated(capturedTenant, studentID)
		})
		return nil
	})
	if txErr != nil {
		return nil, txErr
	}

	s.Logger.Info("parent updated guardian relationship",
		slog.Int64("account_id", accountID),
		slog.Int64("student_id", studentID),
		slog.Int64("guardian_profile_id", guardianProfileID),
		slog.Int64("tenant_id", child.tenantID),
		slog.Bool("pickup_changed", input.CanPickup != nil),
		slog.Bool("emergency_changed", input.IsEmergencyContact != nil),
	)
	return result, nil
}

// guardianManagementEnabled reports whether the child's school has the guardian
// contact/pickup management feature switched on
// (operations.parent_guardian_management_enabled).
func (s *service) guardianManagementEnabled(ctx context.Context, tenantID int64) (bool, error) {
	enabled, err := s.Settings.ResolveBoolForTenant(ctx, tenantID, configModels.KeyParentGuardianManagementEnabled)
	if err != nil {
		return false, fmt.Errorf("parent: resolve guardian-management setting: %w", err)
	}
	return enabled, nil
}

// requireGuardianManagementEnabled returns ErrGuardianManagementDisabled when the
// child's school has switched the feature off, so write paths refuse uniformly.
func (s *service) requireGuardianManagementEnabled(ctx context.Context, tenantID int64) error {
	enabled, err := s.guardianManagementEnabled(ctx, tenantID)
	if err != nil {
		return err
	}
	if !enabled {
		return ErrGuardianManagementDisabled
	}
	return nil
}

// guardianChangeEntry is one pending audit row: change type, field, and the
// before/after values (nil = NULL, e.g. a cleared field).
type guardianChangeEntry struct {
	changeType string
	field      string
	oldValue   *string
	newValue   *string
}

// writeGuardianChanges appends one append-only audit row per supplied change,
// snapshotting the acting guardian once. A nil repo or empty change set is a
// no-op. Must run inside the same tenant transaction as the change so the trail
// is atomic with it.
func (s *service) writeGuardianChanges(ctx context.Context, tenantID, accountID, studentID, guardianProfileID int64, changes []guardianChangeEntry) error {
	if s.GuardianChangeAuditRepo == nil || len(changes) == 0 {
		return nil
	}
	actorName, actorEmail := s.actorSnapshot(ctx, accountID)
	actorID := accountID
	for _, c := range changes {
		entry := &auditModels.GuardianChange{
			StudentID:          studentID,
			GuardianProfileID:  guardianProfileID,
			ActorAccountID:     &actorID,
			ActorNameSnapshot:  actorName,
			ActorEmailSnapshot: actorEmail,
			ChangeType:         c.changeType,
			FieldName:          c.field,
			OldValue:           c.oldValue,
			NewValue:           c.newValue,
		}
		entry.SetTenantID(tenantID)
		if err := s.GuardianChangeAuditRepo.Create(ctx, entry); err != nil {
			return err
		}
	}
	return nil
}

// auditPickupFlagChanges records one row per safety-critical flag (can_pickup /
// is_emergency_contact) that actually changed value. Notes and priority are
// annotations and are not audited.
func (s *service) auditPickupFlagChanges(ctx context.Context, tenantID, accountID, studentID, guardianProfileID int64, input GuardianRelationshipInput, oldCanPickup, oldEmergency bool) error {
	var changes []guardianChangeEntry
	if input.CanPickup != nil && *input.CanPickup != oldCanPickup {
		changes = append(changes, guardianChangeEntry{auditModels.GuardianChangeTypePickup, auditModels.GuardianFieldCanPickup, boolAuditValue(oldCanPickup), boolAuditValue(*input.CanPickup)})
	}
	if input.IsEmergencyContact != nil && *input.IsEmergencyContact != oldEmergency {
		changes = append(changes, guardianChangeEntry{auditModels.GuardianChangeTypePickup, auditModels.GuardianFieldEmergencyContact, boolAuditValue(oldEmergency), boolAuditValue(*input.IsEmergencyContact)})
	}
	return s.writeGuardianChanges(ctx, tenantID, accountID, studentID, guardianProfileID, changes)
}

// guardianContactSnapshot captures the auditable contact fields of a profile
// (plus a stable rendering of its phone list) taken before an edit, so the
// change can be diffed against the post-edit state.
type guardianContactSnapshot struct {
	firstName  string
	lastName   string
	email      string
	street     string
	city       string
	postalCode string
	phones     string
}

func snapshotGuardianContact(profile *usersModels.GuardianProfile, phones []*usersModels.GuardianPhoneNumber) guardianContactSnapshot {
	return guardianContactSnapshot{
		firstName:  profile.FirstName,
		lastName:   profile.LastName,
		email:      base.Deref(profile.Email),
		street:     base.Deref(profile.AddressStreet),
		city:       base.Deref(profile.AddressCity),
		postalCode: base.Deref(profile.AddressPostalCode),
		phones:     formatPhonesForAudit(phones),
	}
}

// auditContactChanges records one row per contact field that actually changed
// between the pre-edit snapshot and the post-edit profile (incl. the phone
// list as a single rendered field).
//
// The before/after VALUES are deliberately not persisted (old/new left NULL):
// contact fields (name, email, phone, address) are third-party PII, and keeping
// their history in an append-only log is a Datenminimierung problem — the live
// profile always holds the current value, and the trail's purpose is "who
// changed which field, when", not a value diff. This mirrors the pickup-notes
// decision (notes content is never logged). Because no values are stored, the
// rows die with the child/guardian via the table's ON DELETE CASCADE and need
// no separate retention window.
func (s *service) auditContactChanges(ctx context.Context, tenantID, accountID, studentID, guardianProfileID int64, before guardianContactSnapshot, profile *usersModels.GuardianProfile, newPhones []*usersModels.GuardianPhoneNumber) error {
	var changes []guardianChangeEntry
	add := func(field, oldVal, newVal string) {
		if oldVal != newVal {
			changes = append(changes, guardianChangeEntry{auditModels.GuardianChangeTypeContact, field, nil, nil})
		}
	}
	add(auditModels.GuardianFieldFirstName, before.firstName, profile.FirstName)
	add(auditModels.GuardianFieldLastName, before.lastName, profile.LastName)
	add(auditModels.GuardianFieldEmail, before.email, base.Deref(profile.Email))
	add(auditModels.GuardianFieldAddressStreet, before.street, base.Deref(profile.AddressStreet))
	add(auditModels.GuardianFieldAddressCity, before.city, base.Deref(profile.AddressCity))
	add(auditModels.GuardianFieldAddressPostalCode, before.postalCode, base.Deref(profile.AddressPostalCode))
	add(auditModels.GuardianFieldPhones, before.phones, formatPhonesForAudit(newPhones))
	return s.writeGuardianChanges(ctx, tenantID, accountID, studentID, guardianProfileID, changes)
}

func boolAuditValue(b bool) *string {
	v := "false"
	if b {
		v = "true"
	}
	return &v
}

// formatPhonesForAudit renders a guardian's phone list to a stable string for
// the audit trail. Old and new lists arrive from the repo in the same order
// (primary first), so an unchanged list formats identically and is not logged.
// The label is part of the rendering: a label-only edit (same number/type/
// primary) is still an accepted change and must produce a distinct string so it
// writes an audit row rather than diffing identically and being dropped.
func formatPhonesForAudit(phones []*usersModels.GuardianPhoneNumber) string {
	if len(phones) == 0 {
		return ""
	}
	parts := make([]string, 0, len(phones))
	for _, p := range phones {
		entry := p.PhoneNumber + " (" + string(p.PhoneType)
		if p.IsPrimary {
			entry += ", primär"
		}
		entry += ")"
		if p.Label != nil {
			if label := strings.TrimSpace(*p.Label); label != "" {
				entry += " [" + label + "]"
			}
		}
		parts = append(parts, entry)
	}
	return strings.Join(parts, "; ")
}

// actorSnapshot returns the acting guardian's name/email for the audit trail so
// it survives a later account deletion. Best-effort: a missing profile yields
// nils rather than failing the audited operation.
func (s *service) actorSnapshot(ctx context.Context, accountID int64) (*string, *string) {
	actor, err := s.GuardianProfileRepo.FindByAccountID(ctx, accountID)
	if err != nil || actor == nil {
		return nil, nil
	}
	var name *string
	if full := strings.TrimSpace(actor.FirstName + " " + actor.LastName); full != "" {
		name = &full
	}
	return name, actor.Email
}

// findChildGuardianLinkForUpdate returns the students_guardians row joining the
// child and guardian profile LOCKED FOR UPDATE for the current tx, or
// ErrGuardianNotLinked when none exists. Write paths use it (after locking the
// guardian profile) so a concurrent staff edit/delete of the relationship row
// cannot change role, account, or existence between the authorization checks and
// the write.
func (s *service) findChildGuardianLinkForUpdate(ctx context.Context, studentID, guardianProfileID int64) (*usersModels.StudentGuardian, error) {
	link, err := s.StudentGuardianRepo.FindByStudentAndGuardianForUpdate(ctx, studentID, guardianProfileID)
	if err != nil {
		if errors.Is(err, usersModels.ErrStudentGuardianNotFound) {
			return nil, ErrGuardianNotLinked
		}
		return nil, err
	}
	return link, nil
}

// replaceGuardianPhones replaces the guardian's entire phone list with the
// submitted set. A wholesale replace keeps the edit atomic and avoids per-row
// diffing the portal would otherwise have to drive.
func (s *service) replaceGuardianPhones(ctx context.Context, profile *usersModels.GuardianProfile, phones []GuardianPhoneInput) error {
	if err := s.GuardianPhoneRepo.DeleteByGuardianID(ctx, profile.ID); err != nil {
		return err
	}
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
		entity.SetTenantID(profile.TenantID)
		if err := s.GuardianPhoneRepo.Create(ctx, entity); err != nil {
			return err
		}
	}
	return nil
}

func hasSubmittedPrimaryPhone(phones []GuardianPhoneInput) bool {
	for _, p := range phones {
		if p.IsPrimary {
			return true
		}
	}
	return false
}

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
func (s *service) callerFamilyStudentSet(ctx context.Context, accountID int64) (map[int64]bool, error) {
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
func (s *service) profileEscapesFamily(ctx context.Context, guardianProfileID int64, callerStudents map[int64]bool) (bool, error) {
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
func (s *service) profilesEscapingFamily(ctx context.Context, profileIDs []int64, callerStudents map[int64]bool) (map[int64]bool, error) {
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
	return g
}

// applyContactInput overwrites the profile's editable contact fields from the
// validated input. Phones are handled separately (replaceGuardianPhones).
func applyContactInput(profile *usersModels.GuardianProfile, input *GuardianContactInput) {
	profile.FirstName = strings.TrimSpace(input.FirstName)
	profile.LastName = strings.TrimSpace(input.LastName)
	profile.Email = strutil.TrimPtrToNil(input.Email)
	profile.AddressStreet = strutil.TrimPtrToNil(input.AddressStreet)
	profile.AddressCity = strutil.TrimPtrToNil(input.AddressCity)
	profile.AddressPostalCode = strutil.TrimPtrToNil(input.AddressPostalCode)
}

func validateContactInput(input *GuardianContactInput) error {
	first := strings.TrimSpace(input.FirstName)
	last := strings.TrimSpace(input.LastName)
	if first == "" || last == "" {
		return fmt.Errorf("%w: name is required", ErrGuardianContactInvalid)
	}
	if utf8.RuneCountInString(first) > maxGuardianNameLen || utf8.RuneCountInString(last) > maxGuardianNameLen {
		return fmt.Errorf("%w: name too long", ErrGuardianContactInvalid)
	}
	if input.Email != nil {
		email := strings.TrimSpace(*input.Email)
		if email != "" {
			if utf8.RuneCountInString(email) > maxGuardianEmailLen {
				return fmt.Errorf("%w: email too long", ErrGuardianContactInvalid)
			}
			addr, err := mail.ParseAddress(email)
			if err != nil {
				return fmt.Errorf("%w: invalid email", ErrGuardianContactInvalid)
			}
			// Store the bare addr-spec, never the raw input. mail.ParseAddress
			// also accepts display-name forms ("Oma <oma@x.de>"); without this
			// the "Oma <...>" wrapper would be persisted as the email and later
			// break FindByEmail / parent-facing mail. Normalize to addr.Address
			// so only the address lands in the column.
			normalized := addr.Address
			input.Email = &normalized
		}
	}
	for _, addr := range []*string{input.AddressStreet, input.AddressCity, input.AddressPostalCode} {
		if addr != nil && utf8.RuneCountInString(*addr) > maxGuardianAddrLen {
			return fmt.Errorf("%w: address field too long", ErrGuardianContactInvalid)
		}
	}
	if len(input.Phones) > maxGuardianPhones {
		return fmt.Errorf("%w: too many phone numbers", ErrGuardianContactInvalid)
	}
	for _, p := range input.Phones {
		num := strings.TrimSpace(p.PhoneNumber)
		if num == "" {
			return fmt.Errorf("%w: empty phone number", ErrGuardianContactInvalid)
		}
		if utf8.RuneCountInString(num) > maxGuardianPhoneLen {
			return fmt.Errorf("%w: phone number too long", ErrGuardianContactInvalid)
		}
		if !guardianPhoneNumberPattern.MatchString(num) {
			return fmt.Errorf("%w: invalid phone number", ErrGuardianContactInvalid)
		}
		if len(guardianPhoneDigitPattern.FindAllString(num, -1)) < minGuardianPhoneDigits {
			return fmt.Errorf("%w: phone number too short", ErrGuardianContactInvalid)
		}
		if p.Label != nil && utf8.RuneCountInString(*p.Label) > maxGuardianLabelLen {
			return fmt.Errorf("%w: phone label too long", ErrGuardianContactInvalid)
		}
	}
	return nil
}

const (
	guardianEmailUniqueIndex = "idx_guardian_profiles_tenant_email"
	minGuardianPhoneDigits   = 3
)

var (
	guardianPhoneNumberPattern = regexp.MustCompile(`^[\d\s\+\-\(\)]+$`)
	guardianPhoneDigitPattern  = regexp.MustCompile(`\d`)
)

func isGuardianEmailUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	if strings.Contains(err.Error(), "SQLSTATE=23505") && strings.Contains(err.Error(), guardianEmailUniqueIndex) {
		return true
	}
	var dbErr *base.DatabaseError
	if errors.As(err, &dbErr) {
		err = dbErr.Err
	}
	var pgErr pgdriver.Error
	if !errors.As(err, &pgErr) {
		return false
	}
	if pgErr.Field('C') != "23505" {
		return false
	}
	return pgErr.Field('n') == guardianEmailUniqueIndex || strings.Contains(pgErr.Error(), guardianEmailUniqueIndex)
}

func validateRelationshipInput(input *GuardianRelationshipInput) error {
	if input.PickupNotes != nil && utf8.RuneCountInString(*input.PickupNotes) > maxGuardianNotesLen {
		return fmt.Errorf("%w: pickup note too long", ErrGuardianRelationshipInvalid)
	}
	return nil
}

func validateCreateGuardianContactInput(input *CreateGuardianContactInput) error {
	if err := validateContactInput(&input.Contact); err != nil {
		return err
	}
	input.RelationshipType = strings.ToLower(strings.TrimSpace(input.RelationshipType))
	if !usersModels.IsValidRelationshipType(input.RelationshipType) {
		return fmt.Errorf("%w: invalid relationship type", ErrGuardianRelationshipInvalid)
	}
	return validateRelationshipInput(&GuardianRelationshipInput{PickupNotes: input.PickupNotes})
}
