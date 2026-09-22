package care

import (
	"errors"
	"fmt"
	"net/mail"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/moto-nrw/project-phoenix/internal/strutil"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
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
	if err := validateContactName(input.FirstName, input.LastName); err != nil {
		return err
	}
	if err := normalizeContactEmail(input); err != nil {
		return err
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
		if err := validateContactPhone(p); err != nil {
			return err
		}
	}
	return nil
}

// validateContactName requires both names and caps their length.
func validateContactName(firstName, lastName string) error {
	first := strings.TrimSpace(firstName)
	last := strings.TrimSpace(lastName)
	if first == "" || last == "" {
		return fmt.Errorf("%w: name is required", ErrGuardianContactInvalid)
	}
	if utf8.RuneCountInString(first) > maxGuardianNameLen || utf8.RuneCountInString(last) > maxGuardianNameLen {
		return fmt.Errorf("%w: name too long", ErrGuardianContactInvalid)
	}
	return nil
}

// normalizeContactEmail validates a non-blank email and rewrites input.Email
// to the bare address. A nil or blank email is left untouched.
func normalizeContactEmail(input *GuardianContactInput) error {
	if input.Email == nil {
		return nil
	}
	email := strings.TrimSpace(*input.Email)
	if email == "" {
		return nil
	}
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
	return nil
}

// validateContactPhone checks one phone entry's number and label.
func validateContactPhone(p GuardianPhoneInput) error {
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
