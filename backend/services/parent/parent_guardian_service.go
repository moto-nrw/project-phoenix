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
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/moto-nrw/project-phoenix/models/base"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// Field length bounds for guardian contact edits. Generous but finite so a
// buggy or hostile client can't push unbounded strings into the contact table.
const (
	maxGuardianNameLen   = 100
	maxGuardianEmailLen  = 254
	maxGuardianAddrLen   = 200
	maxGuardianPhoneLen  = 40
	maxGuardianPhones    = 5
	maxGuardianNotesLen  = 500
	maxEmergencyPriority = 99
	minEmergencyPriority = 1
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
	EmergencyPriority  int
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
	EmergencyPriority  *int
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
	callerStudents, err := s.callerTenantStudentSet(ctx, accountID, child.tenantID)
	if err != nil {
		return nil, err
	}

	var out []*ChildGuardian
	txErr := tenant.WithTenantTx(ctx, s.db, child.tenantID, func(txCtx context.Context, _ bun.Tx) error {
		links, err := s.studentGuardianRepo.FindByStudentID(txCtx, studentID)
		if err != nil {
			return err
		}
		out = make([]*ChildGuardian, 0, len(links))
		for _, link := range links {
			profile, err := s.guardianProfileRepo.FindByID(txCtx, link.GuardianProfileID)
			if err != nil {
				if errors.Is(err, usersModels.ErrGuardianProfileNotFound) {
					s.logger.Error("parent child guardian link points to missing profile",
						slog.Int64("tenant_id", child.tenantID),
						slog.Int64("student_id", studentID),
						slog.Int64("student_guardian_id", link.ID),
						slog.Int64("guardian_profile_id", link.GuardianProfileID),
					)
					return ErrGuardianNotLinked
				}
				return err
			}
			phones, err := s.guardianPhoneRepo.FindByGuardianID(txCtx, profile.ID)
			if err != nil {
				return err
			}
			escapes, err := s.profileEscapesFamily(txCtx, profile.ID, callerStudents)
			if err != nil {
				return err
			}
			out = append(out, projectChildGuardian(profile, link, phones, accountID, canEdit, canManage, escapes))
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
	if err := s.requireGuardianManagementEnabled(ctx, child.tenantID); err != nil {
		return nil, err
	}
	callerStudents, err := s.callerTenantStudentSet(ctx, accountID, child.tenantID)
	if err != nil {
		return nil, err
	}

	var result *ChildGuardian
	txErr := tenant.WithTenantTx(ctx, s.db, child.tenantID, func(txCtx context.Context, _ bun.Tx) error {
		link, err := s.findChildGuardianLink(txCtx, studentID, guardianProfileID)
		if err != nil {
			return err
		}
		profile, err := s.guardianProfileRepo.FindByID(txCtx, guardianProfileID)
		if err != nil {
			if errors.Is(err, usersModels.ErrGuardianProfileNotFound) {
				return ErrGuardianNotLinked
			}
			return err
		}
		// HasAccount and AccountID are independent columns. The guard below keys
		// off HasAccount; isSelf keys off AccountID. They are expected to stay
		// consistent (an account holder has both set), but if they ever diverge
		// (HasAccount=true, AccountID=nil) this fails safe: the edit is refused.
		isSelf := profile.AccountID != nil && *profile.AccountID == accountID
		// Only contact-only guardians (or the caller's own profile) are editable.
		// Editing another account holder's personal data is forbidden — the UI is
		// not the boundary, the backend is.
		if profile.HasAccount && !isSelf {
			return ErrGuardianHasOwnAccount
		}
		// A social worker is a school-managed professional contact; a parent may
		// not rewrite their personal data (mirrors the read redaction).
		if !isSelf && link.GuardianRole == authorize.GuardianRoleSocialWorker {
			return ErrGuardianSocialWorkerManaged
		}
		// A contact edit propagates to every child the profile serves (intended for
		// siblings). Reaching past the contact-only guard means !isSelf ⟺ a
		// contact-only profile; refuse it when the profile also serves a child
		// outside the caller's family, so one parent can't rewrite a contact shared
		// with another family.
		if !isSelf {
			escapes, err := s.profileEscapesFamily(txCtx, guardianProfileID, callerStudents)
			if err != nil {
				return err
			}
			if escapes {
				return ErrGuardianSharedAcrossFamilies
			}
		}

		applyContactInput(profile, &input)
		if err := s.guardianProfileRepo.Update(txCtx, profile); err != nil {
			return err
		}
		if err := s.replaceGuardianPhones(txCtx, profile, input.Phones); err != nil {
			return err
		}

		phones, err := s.guardianPhoneRepo.FindByGuardianID(txCtx, profile.ID)
		if err != nil {
			return err
		}
		// Containment passed (or self), so the edited profile does not escape the
		// caller's family: not shared-locked from this caller's view.
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

	s.logger.Info("parent updated guardian contact",
		slog.Int64("account_id", accountID),
		slog.Int64("student_id", studentID),
		slog.Int64("guardian_profile_id", guardianProfileID),
		slog.Int64("tenant_id", child.tenantID),
	)
	return result, nil
}

// UpdateGuardianRelationship edits the per-child pickup/relationship fields of a
// guardian. The CanPickup and IsEmergencyContact flags require
// parent_portal.pickup.manage (safety-relevant authority); PickupNotes and
// EmergencyPriority require parent_portal.guardian.edit. Each field group is
// gated by its own permission so that a caller who holds only one of the two
// permissions can exercise exactly the capability advertised by
// ListChildGuardians, never receiving a 403 for an action the listing offered.
// The flags may additionally only be set on guardians WITHOUT their own portal
// account: an account holder's pickup/emergency standing is set by themselves
// (out of band) or the school, never by another parent and never self-granted
// through the portal.
func (s *service) UpdateGuardianRelationship(ctx context.Context, accountID, studentID, guardianProfileID int64, input GuardianRelationshipInput) (*ChildGuardian, error) {
	if input.CanPickup == nil && input.IsEmergencyContact == nil && input.PickupNotes == nil && input.EmergencyPriority == nil {
		return nil, ErrGuardianNoChange
	}
	if err := validateRelationshipInput(&input); err != nil {
		return nil, err
	}
	editsFlags := input.CanPickup != nil || input.IsEmergencyContact != nil
	editsDetails := input.PickupNotes != nil || input.EmergencyPriority != nil

	// Resolve on baseline portal access, then gate each field group by its own
	// permission. Resolving on the lower baseline (rather than guardian.edit) is
	// what lets a pickup.manage-only caller flip the flags.
	child, err := s.resolvePermittedChild(ctx, accountID, studentID, authorize.GuardianPermissionPortalAccess)
	if err != nil {
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
	callerStudents, err := s.callerTenantStudentSet(ctx, accountID, child.tenantID)
	if err != nil {
		return nil, err
	}

	var result *ChildGuardian
	txErr := tenant.WithTenantTx(ctx, s.db, child.tenantID, func(txCtx context.Context, _ bun.Tx) error {
		link, err := s.findChildGuardianLink(txCtx, studentID, guardianProfileID)
		if err != nil {
			return err
		}
		profile, err := s.guardianProfileRepo.FindByID(txCtx, guardianProfileID)
		if err != nil {
			if errors.Is(err, usersModels.ErrGuardianProfileNotFound) {
				return ErrGuardianNotLinked
			}
			return err
		}
		// can_pickup / is_emergency_contact grant safety-critical pickup/emergency
		// AUTHORITY. A parent may set them only for guardians WITHOUT their own
		// portal account (helpers like grandma). A guardian who holds their own
		// account owns their standing: nobody else may change it here, and a
		// parent may not grant it to themselves either (the caller's own profile
		// is an account holder, so this also blocks self-granting). This closes
		// the custody-dispute griefing vector; the school sets these flags for
		// account-holding guardians. Notes/priority (annotations, not authority)
		// remain editable under guardian.edit.
		if editsFlags && profile.HasAccount {
			return ErrGuardianHasOwnAccount
		}
		// A social worker's pickup/emergency standing is set by the school, not by
		// a parent (mirrors the read-side CanManagePickup gate).
		if editsFlags && link.GuardianRole == authorize.GuardianRoleSocialWorker {
			return ErrGuardianSocialWorkerManaged
		}
		oldCanPickup := link.CanPickup
		oldEmergency := link.IsEmergencyContact
		if input.CanPickup != nil {
			link.CanPickup = *input.CanPickup
		}
		if input.IsEmergencyContact != nil {
			link.IsEmergencyContact = *input.IsEmergencyContact
		}
		if input.PickupNotes != nil {
			trimmed := strings.TrimSpace(*input.PickupNotes)
			if trimmed == "" {
				link.PickupNotes = nil
			} else {
				link.PickupNotes = &trimmed
			}
		}
		if input.EmergencyPriority != nil {
			link.EmergencyPriority = *input.EmergencyPriority
		}
		if err := s.studentGuardianRepo.Update(txCtx, link); err != nil {
			return err
		}
		if err := s.auditPickupFlagChanges(txCtx, child.tenantID, accountID, studentID, guardianProfileID, input, oldCanPickup, oldEmergency); err != nil {
			return err
		}

		phones, err := s.guardianPhoneRepo.FindByGuardianID(txCtx, profile.ID)
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

	s.logger.Info("parent updated guardian relationship",
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
	enabled, err := s.settings.ResolveBoolForTenant(ctx, tenantID, configModels.KeyParentGuardianManagementEnabled)
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

// auditPickupFlagChanges writes one append-only audit row per safety-critical
// flag (can_pickup / is_emergency_contact) that actually changed value. Notes
// and priority are annotations and are not audited. Must run inside the same
// tenant transaction as the change so the trail is atomic with it.
func (s *service) auditPickupFlagChanges(ctx context.Context, tenantID, accountID, studentID, guardianProfileID int64, input GuardianRelationshipInput, oldCanPickup, oldEmergency bool) error {
	if s.guardianPickupAuditRepo == nil {
		return nil
	}
	type flagChange struct {
		field    string
		from, to bool
	}
	var changes []flagChange
	if input.CanPickup != nil && *input.CanPickup != oldCanPickup {
		changes = append(changes, flagChange{auditModels.GuardianPickupFieldCanPickup, oldCanPickup, *input.CanPickup})
	}
	if input.IsEmergencyContact != nil && *input.IsEmergencyContact != oldEmergency {
		changes = append(changes, flagChange{auditModels.GuardianPickupFieldEmergencyContact, oldEmergency, *input.IsEmergencyContact})
	}
	if len(changes) == 0 {
		return nil
	}
	actorName, actorEmail := s.actorSnapshot(ctx, accountID)
	actorID := accountID
	for _, c := range changes {
		entry := &auditModels.GuardianPickupChange{
			StudentID:          studentID,
			GuardianProfileID:  guardianProfileID,
			ActorAccountID:     &actorID,
			ActorNameSnapshot:  actorName,
			ActorEmailSnapshot: actorEmail,
			FieldName:          c.field,
			OldValue:           c.from,
			NewValue:           c.to,
		}
		entry.SetTenantID(tenantID)
		if err := s.guardianPickupAuditRepo.Create(ctx, entry); err != nil {
			return err
		}
	}
	return nil
}

// actorSnapshot returns the acting guardian's name/email for the audit trail so
// it survives a later account deletion. Best-effort: a missing profile yields
// nils rather than failing the audited operation.
func (s *service) actorSnapshot(ctx context.Context, accountID int64) (*string, *string) {
	actor, err := s.guardianProfileRepo.FindByAccountID(ctx, accountID)
	if err != nil || actor == nil {
		return nil, nil
	}
	var name *string
	if full := strings.TrimSpace(actor.FirstName + " " + actor.LastName); full != "" {
		name = &full
	}
	return name, actor.Email
}

// findChildGuardianLink returns the students_guardians row joining the child and
// the named guardian profile, or ErrGuardianNotLinked when none exists.
func (s *service) findChildGuardianLink(ctx context.Context, studentID, guardianProfileID int64) (*usersModels.StudentGuardian, error) {
	links, err := s.studentGuardianRepo.FindByStudentID(ctx, studentID)
	if err != nil {
		return nil, err
	}
	for _, link := range links {
		if link.GuardianProfileID == guardianProfileID {
			return link, nil
		}
	}
	return nil, ErrGuardianNotLinked
}

// replaceGuardianPhones replaces the guardian's entire phone list with the
// submitted set. A wholesale replace keeps the edit atomic and avoids per-row
// diffing the portal would otherwise have to drive.
func (s *service) replaceGuardianPhones(ctx context.Context, profile *usersModels.GuardianProfile, phones []GuardianPhoneInput) error {
	if err := s.guardianPhoneRepo.DeleteByGuardianID(ctx, profile.ID); err != nil {
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
		if err := s.guardianPhoneRepo.Create(ctx, entity); err != nil {
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

// callerTenantStudentSet returns the set of per-tenant student IDs (within the
// given tenant) the account is a linked guardian of. StudentID is unique only
// per tenant, so the set is scoped to one tenant and compared only against
// links read in that same tenant transaction.
func (s *service) callerTenantStudentSet(ctx context.Context, accountID, tenantID int64) (map[int64]bool, error) {
	children, err := s.ListChildrenForAccount(ctx, accountID)
	if err != nil {
		return nil, err
	}
	set := make(map[int64]bool, len(children))
	for _, c := range children {
		if c.TenantID == tenantID {
			set[c.StudentID] = true
		}
	}
	return set, nil
}

// profileEscapesFamily reports whether the guardian profile is linked to any
// student outside callerStudents — i.e. it also serves another family, so its
// contact data must not be editable by this caller. Must run inside the tenant
// transaction (FindByGuardianProfileID is tenant-filtered).
func (s *service) profileEscapesFamily(ctx context.Context, guardianProfileID int64, callerStudents map[int64]bool) (bool, error) {
	links, err := s.studentGuardianRepo.FindByGuardianProfileID(ctx, guardianProfileID)
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
	return profile.HasAccount ||
		link.GuardianRole == authorize.GuardianRoleSocialWorker ||
		sharedAcrossFamilies
}

// projectChildGuardian builds the parent-facing projection of one guardian.
// sharedAcrossFamilies marks a contact-only profile that also serves a child
// outside the caller's family — editable only by the school, not this caller.
func projectChildGuardian(profile *usersModels.GuardianProfile, link *usersModels.StudentGuardian, phones []*usersModels.GuardianPhoneNumber, accountID int64, canEdit, canManage, sharedAcrossFamilies bool) *ChildGuardian {
	isSelf := profile.AccountID != nil && *profile.AccountID == accountID
	isSocialWorker := link.GuardianRole == authorize.GuardianRoleSocialWorker
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
		EmergencyPriority:  link.EmergencyPriority,
		HasAccount:         profile.HasAccount,
		IsSelf:             isSelf,
		// Contact editing needs the edit permission and an unprotected target.
		// The caller's own profile is always editable regardless of reach.
		CanEditContact: canEdit && !protected,
		// Pickup/emergency flags may be managed only for contact-only helpers
		// (grandma): an account holder's standing is theirs and the school's, and
		// a social worker's is the school's. Mirrors the write guard in
		// UpdateGuardianRelationship so the UI never offers a control the backend
		// rejects.
		CanManagePickup: canManage && !profile.HasAccount && !isSocialWorker,
		// Surface exactly one "why is this read-only" explanation to a caller who
		// could otherwise edit (own account > social worker > shared). For a
		// caller without edit rights none is the reason the affordance is absent.
		ContactLockedOwnAccount:   canEdit && !isSelf && profile.HasAccount,
		ContactLockedSocialWorker: canEdit && !isSelf && !profile.HasAccount && isSocialWorker,
		ContactLockedShared:       canEdit && !isSelf && !profile.HasAccount && !isSocialWorker && sharedAcrossFamilies,
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
	profile.Email = normalizeOptional(input.Email)
	profile.AddressStreet = normalizeOptional(input.AddressStreet)
	profile.AddressCity = normalizeOptional(input.AddressCity)
	profile.AddressPostalCode = normalizeOptional(input.AddressPostalCode)
}

// normalizeOptional trims an optional string, returning nil for blank values so
// a cleared field stores NULL rather than an empty string.
func normalizeOptional(v *string) *string {
	if v == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*v)
	if trimmed == "" {
		return nil
	}
	return &trimmed
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
			if _, err := mail.ParseAddress(email); err != nil {
				return fmt.Errorf("%w: invalid email", ErrGuardianContactInvalid)
			}
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
	if input.EmergencyPriority != nil {
		p := *input.EmergencyPriority
		if p < minEmergencyPriority || p > maxEmergencyPriority {
			return fmt.Errorf("%w: emergency priority out of range", ErrGuardianRelationshipInvalid)
		}
	}
	return nil
}
