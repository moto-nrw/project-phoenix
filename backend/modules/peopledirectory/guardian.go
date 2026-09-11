package peopledirectory

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync/atomic"
	"time"
)

// Guardian and contact-person capability (#2663). The People Directory owns
// users.guardian_profiles, users.guardian_phone_numbers and
// users.students_guardians: a contact person, the permission to collect a
// child, the emergency-contact flag and the parents-portal access are four
// separate facts on the link, never inferred from one another.

var (
	ErrGuardianNotFound      = errors.New("guardian not found")
	ErrGuardianPhoneNotFound = errors.New("phone number not found")
	ErrGuardianLinkNotFound  = errors.New("relationship not found")
	ErrInvalidGuardian       = errors.New("invalid guardian")
	// ErrGuardianStillLinked refuses a plain delete of a guardian that is
	// still linked to children; GuardianStillLinkedError carries the names.
	ErrGuardianStillLinked = errors.New("guardian is still linked to students")
	// ErrGuardianForceDeleteRequiresAdmin refuses a forced full delete by a
	// non-admin: it reaches siblings the caller may not supervise.
	ErrGuardianForceDeleteRequiresAdmin = errors.New("only administrators can fully delete a guardian linked to students")
	// ErrGuardianDeletePreviewChanged reports that the links changed since
	// the admin confirmed the preview. The text is rendered verbatim.
	ErrGuardianDeletePreviewChanged = errors.New("Die Verknüpfungen haben sich seit der Vorschau geändert. Bitte erneut prüfen.") //nolint:staticcheck // ST1005: user-facing German message
	// ErrGuardianLinkConflict marks a delete that tripped the RESTRICT link
	// constraint because a link was added after the check.
	ErrGuardianLinkConflict = errors.New("guardian is linked to students")
	// ErrPayerRemovalRequiresFinancial refuses to unlink the child's payer
	// without the financial permission. The text is rendered verbatim.
	ErrPayerRemovalRequiresFinancial = errors.New("Diese Person ist als Zahler für das Kind eingetragen. Zum Entfernen ist die Berechtigung für Bankverbindungen nötig. Bitte wenden Sie sich an die Schulleitung.") //nolint:staticcheck // ST1005: user-facing German message

	// Payment input errors (#2608); each one maps to a German sentence in the
	// HTTP adapter.
	ErrGuardianPaymentInvalid       = errors.New("invalid payment value")
	ErrGuardianIBANInvalid          = fmt.Errorf("%w: malformed IBAN", ErrGuardianPaymentInvalid)
	ErrGuardianAccountHolderTooLong = fmt.Errorf("%w: account holder too long", ErrGuardianPaymentInvalid)
	ErrGuardianStudentRequired      = fmt.Errorf("%w: student id is required", ErrGuardianPaymentInvalid)
	ErrGuardianNotLinkedToStudent   = errors.New("guardian is not linked to this student")

	// ErrGuardianProviderUnbound is returned while the composition root has
	// not bound the guardian provider yet; nothing falls back to a foreign
	// read.
	ErrGuardianProviderUnbound = errors.New("people directory: guardian provider is not bound")
)

// InvalidGuardianError carries the user-facing reason of a rejected guardian
// input; the HTTP layer renders Reason as the 400 message.
type InvalidGuardianError struct{ Reason string }

func (e *InvalidGuardianError) Error() string { return e.Reason }
func (e *InvalidGuardianError) Unwrap() error { return ErrInvalidGuardian }

// GuardianStillLinkedError names the children a refused delete would have
// affected; only admins get to see them.
type GuardianStillLinkedError struct{ StudentNames []string }

func (e *GuardianStillLinkedError) Error() string { return ErrGuardianStillLinked.Error() }
func (e *GuardianStillLinkedError) Unwrap() error { return ErrGuardianStillLinked }

// GuardianPermissionPortalAccess is the link permission that lets a portal
// account see the child. It is the one permission the staff directory
// renders (account status); every other parents-portal permission stays
// with the security runtime.
const GuardianPermissionPortalAccess = "parent_portal.access"

// Guardian is one contact person of the tenant. PhoneNumbers are loaded by
// FindGuardian and the profile lists; the link projections leave them empty
// unless stated otherwise.
type Guardian struct {
	ID                     int64
	TenantID               int64
	FirstName              string
	LastName               string
	Email                  *string
	AddressStreet          *string
	AddressCity            *string
	AddressPostalCode      *string
	PreferredContactMethod string
	LanguagePreference     string
	Notes                  *string
	HasAccount             bool
	AccountID              *int64
	PhoneNumbers           []GuardianPhone
}

// FullName joins the two name parts, skipping an empty one.
func (g Guardian) FullName() string {
	return strings.TrimSpace(strings.TrimSpace(g.FirstName) + " " + strings.TrimSpace(g.LastName))
}

// GuardianPhone is one phone number of a guardian, ordered by Priority.
type GuardianPhone struct {
	ID                int64
	GuardianProfileID int64
	PhoneNumber       string
	PhoneType         string
	Label             *string
	IsPrimary         bool
	Priority          int
}

// GuardianLink is one users.students_guardians row: the relationship of a
// guardian to one child. Permissions lists the granted parents-portal
// permission names of this link.
type GuardianLink struct {
	ID                 int64
	TenantID           int64
	StudentID          int64
	GuardianProfileID  int64
	RelationshipType   string
	GuardianRole       string
	IsPrimary          bool
	IsEmergencyContact bool
	CanPickup          bool
	PickupNotes        *string
	EmergencyPriority  int
	IsPayer            bool
	Permissions        []string
}

// HasPermission reports whether the link grants the named portal permission.
func (l GuardianLink) HasPermission(name string) bool {
	return slices.Contains(l.Permissions, name)
}

// GuardianWithLink is a child's guardian together with the link and whether
// an invitation is still open for that guardian on that child.
type GuardianWithLink struct {
	Guardian          Guardian
	Link              GuardianLink
	InvitationPending bool
}

// StudentWithLink is a guardian's child together with the link. Graduated
// children are excluded.
type StudentWithLink struct {
	Student Student
	Link    GuardianLink
}

// GuardianMatch is one picker search result: the guardian and how many
// children are linked to it. Child names never leave the owner here.
type GuardianMatch struct {
	Guardian            Guardian
	LinkedChildrenCount int
}

// GuardianDeleteImpact is the exact current blast radius of a full delete.
type GuardianDeleteImpact struct {
	LinkIDs      []int64
	StudentNames []string
}

// GuardianInput is the create payload and the fully merged update payload.
type GuardianInput struct {
	FirstName              string
	LastName               string
	Email                  *string
	AddressStreet          *string
	AddressCity            *string
	AddressPostalCode      *string
	PreferredContactMethod string
	LanguagePreference     string
	Notes                  *string
}

// GuardianDelete describes a confirmed delete. WithLinks removes every link
// first (the admin full delete); ExpectedLinkIDs is the preview token that
// must still match.
type GuardianDelete struct {
	GuardianID      int64
	ActorAccountID  int64
	WithLinks       bool
	ExpectedLinkIDs []int64
}

type GuardianPhoneInput struct {
	PhoneNumber string
	PhoneType   string
	Label       *string
	IsPrimary   bool
}

type GuardianPhoneUpdate struct {
	PhoneNumber *string
	PhoneType   *string
	Label       *string
	IsPrimary   *bool
	Priority    *int
}

// LinkGuardian attaches an existing guardian to a child.
type LinkGuardian struct {
	StudentID          int64
	GuardianProfileID  int64
	RelationshipType   string
	GuardianRole       string
	IsPrimary          bool
	IsEmergencyContact bool
	CanPickup          bool
	PickupNotes        *string
	EmergencyPriority  int
}

type GuardianLinkUpdate struct {
	RelationshipType   *string
	GuardianRole       *string
	IsPrimary          *bool
	IsEmergencyContact *bool
	CanPickup          *bool
	PickupNotes        *string
	EmergencyPriority  *int
}

// RemoveGuardian unlinks one guardian from one child. MayClearPayer says
// whether the actor holds the financial permission that owns the payer mark.
type RemoveGuardian struct {
	StudentID         int64
	GuardianProfileID int64
	ActorAccountID    int64
	MayClearPayer     bool
}

// NewGuardianPhone is one phone number of a guardian created together with
// a child.
type NewGuardianPhone struct {
	PhoneNumber string `json:"phone_number"`
	PhoneType   string `json:"phone_type,omitempty"`
	Label       string `json:"label,omitempty"`
	IsPrimary   bool   `json:"is_primary,omitempty"`
}

// NewStudentGuardian is one guardian to attach to a child in one atomic
// request: either an EXISTING profile (GuardianProfileID, the sibling case)
// or a NEW one (name, contact, phone numbers), plus the relationship flags
// for that child. It is the wire shape shared by the student create flow
// and the guardian batch endpoint, so it carries the JSON tags.
type NewStudentGuardian struct {
	GuardianProfileID *int64 `json:"guardian_profile_id,omitempty"`

	FirstName              string `json:"first_name"`
	LastName               string `json:"last_name"`
	Email                  string `json:"email,omitempty"`
	AddressStreet          string `json:"address_street,omitempty"`
	AddressCity            string `json:"address_city,omitempty"`
	AddressPostalCode      string `json:"address_postal_code,omitempty"`
	PreferredContactMethod string `json:"preferred_contact_method,omitempty"`
	LanguagePreference     string `json:"language_preference,omitempty"`
	Notes                  string `json:"notes,omitempty"`

	RelationshipType   string `json:"relationship_type"`
	GuardianRole       string `json:"guardian_role,omitempty"`
	IsPrimary          bool   `json:"is_primary,omitempty"`
	IsEmergencyContact bool   `json:"is_emergency_contact,omitempty"`
	CanPickup          bool   `json:"can_pickup,omitempty"`
	PickupNotes        string `json:"pickup_notes,omitempty"`
	EmergencyPriority  int    `json:"emergency_priority,omitempty"`

	PhoneNumbers []NewGuardianPhone `json:"phone_numbers,omitempty"`
}

// CreatesProfile reports whether the entry creates a new guardian profile
// instead of linking an existing one.
func (g NewStudentGuardian) CreatesProfile() bool { return g.GuardianProfileID == nil }

// GuardianPaymentActor is the audited reader of bank data.
type GuardianPaymentActor struct {
	AccountID int64
	Role      string
}

// GuardianPayment is the bank section of one guardian: masked after the
// default read, unmasked after the audited reveal.
type GuardianPayment struct {
	GuardianProfileID int64
	IBAN              *string
	AccountHolder     *string
}

type GuardianPaymentInput struct {
	IBAN           *string
	AccountHolder  *string
	Note           string
	ActorAccountID int64
}

// StudentPayer names the guardian charged for a child; a nil GuardianProfileID
// clears the assignment.
type StudentPayer struct {
	StudentID         int64
	GuardianProfileID *int64
	ActorAccountID    int64
}

// GuardianPaymentRow is one line of the Bankverbindungen list. IBAN carries
// the full value only on the export path.
type GuardianPaymentRow struct {
	StudentID         int64
	StudentName       string
	SchoolClass       string
	GuardianProfileID *int64
	GuardianName      string
	RelationshipType  string
	AccountHolder     string
	IBAN              string
	IBANMasked        string
}

// HasIBAN reports whether a usable bank account is stored for this row.
func (r GuardianPaymentRow) HasIBAN() bool { return r.IBAN != "" || r.IBANMasked != "" }

// GuardianQuery reads guardians, their phone numbers and their links.
type GuardianQuery interface {
	FindGuardian(context.Context, int64) (Guardian, error)
	ListGuardians(ctx context.Context, page, pageSize int) ([]Guardian, error)
	ListGuardiansWithoutAccount(context.Context) ([]Guardian, error)
	ListInvitableGuardians(context.Context) ([]Guardian, error)
	// SearchGuardians matches name and email; limit caps the slice.
	SearchGuardians(ctx context.Context, text string, limit int) ([]GuardianMatch, error)
	GuardianDeleteImpact(context.Context, int64) (GuardianDeleteImpact, error)
	ListGuardianPhones(context.Context, int64) ([]GuardianPhone, error)
	FindGuardianPhone(context.Context, int64) (GuardianPhone, error)
	ListStudentGuardians(context.Context, int64) ([]GuardianWithLink, error)
	ListGuardianStudents(context.Context, int64) ([]StudentWithLink, error)
	FindGuardianLink(context.Context, int64) (GuardianLink, error)
	// ListGuardianLinksByAccount returns every link of the guardian profiles
	// linked to the account. Inside an admin transaction it spans every
	// tenant; the caller decides which tenants the account may act at.
	ListGuardianLinksByAccount(context.Context, int64) ([]GuardianLink, error)
	// ListGuardiansByAccount returns the profiles (without phone numbers)
	// linked to the accounts, scoped to the tenant in context.
	ListGuardiansByAccount(context.Context, []int64) ([]Guardian, error)
	// ListGuardiansByID returns the profiles (without phone numbers) for
	// the ids, scoped to the tenant in context.
	ListGuardiansByID(context.Context, []int64) ([]Guardian, error)
	// CountGuardianLinks counts the links per guardian; guardians without a
	// link are absent from the result.
	CountGuardianLinks(context.Context, []int64) (map[int64]int, error)
	GuardianPaymentMasked(context.Context, int64, GuardianPaymentActor) (GuardianPayment, error)
	ListPaymentOverview(context.Context, GuardianPaymentActor) ([]GuardianPaymentRow, error)
	ListPaymentExportRows(ctx context.Context, actor GuardianPaymentActor, format string) ([]GuardianPaymentRow, error)
}

// GuardianCommand changes guardians, phone numbers, links and payment data.
// Every command joins the caller's tenant transaction or opens one.
type GuardianCommand interface {
	CreateGuardian(context.Context, GuardianInput) (Guardian, error)
	UpdateGuardian(context.Context, int64, GuardianInput) error
	// EvaluateGuardianDelete applies the two delete rules and reports whether
	// links exist: a linked guardian needs force, a forced delete an admin.
	EvaluateGuardianDelete(ctx context.Context, id int64, force, isAdmin bool) (bool, error)
	DeleteGuardian(context.Context, GuardianDelete) error
	AddGuardianPhone(context.Context, int64, GuardianPhoneInput) (GuardianPhone, error)
	UpdateGuardianPhone(context.Context, int64, GuardianPhoneUpdate) error
	DeleteGuardianPhone(context.Context, int64) error
	SetPrimaryGuardianPhone(context.Context, int64) error
	LinkGuardianToStudent(context.Context, LinkGuardian) (GuardianLink, error)
	ValidateNewGuardians(context.Context, []NewStudentGuardian) error
	AddGuardiansToStudent(context.Context, int64, []NewStudentGuardian) error
	UpdateGuardianLink(context.Context, int64, GuardianLinkUpdate) error
	RemoveGuardianFromStudent(context.Context, RemoveGuardian) error
	RevealGuardianPayment(context.Context, int64, GuardianPaymentActor) (GuardianPayment, error)
	UpdateGuardianPayment(context.Context, int64, GuardianPaymentInput) error
	SetStudentPayer(context.Context, StudentPayer) error
}

// GuardianProvider is the owner's application layer behind the guardian
// facade. The composition root binds it once the legacy service graph
// exists; the row-level reads never go through it.
type GuardianProvider interface {
	FindGuardian(context.Context, int64) (Guardian, error)
	ListGuardians(ctx context.Context, page, pageSize int) ([]Guardian, error)
	ListGuardiansWithoutAccount(context.Context) ([]Guardian, error)
	ListInvitableGuardians(context.Context) ([]Guardian, error)
	SearchGuardians(ctx context.Context, text string, limit int) ([]GuardianMatch, error)
	GuardianDeleteImpact(context.Context, int64) (GuardianDeleteImpact, error)
	ListGuardianPhones(context.Context, int64) ([]GuardianPhone, error)
	FindGuardianPhone(context.Context, int64) (GuardianPhone, error)
	ListStudentGuardians(context.Context, int64) ([]GuardianWithLink, error)
	ListGuardianStudents(context.Context, int64) ([]StudentWithLink, error)
	FindGuardianLink(context.Context, int64) (GuardianLink, error)
	GuardianPaymentMasked(context.Context, int64, GuardianPaymentActor) (GuardianPayment, error)
	ListPaymentOverview(context.Context, GuardianPaymentActor) ([]GuardianPaymentRow, error)
	ListPaymentExportRows(ctx context.Context, actor GuardianPaymentActor, format string) ([]GuardianPaymentRow, error)

	CreateGuardian(context.Context, GuardianInput) (Guardian, error)
	UpdateGuardian(context.Context, int64, GuardianInput) error
	EvaluateGuardianDelete(ctx context.Context, id int64, force, isAdmin bool) (bool, error)
	DeleteGuardian(context.Context, GuardianDelete) error
	AddGuardianPhone(context.Context, int64, GuardianPhoneInput) (GuardianPhone, error)
	UpdateGuardianPhone(context.Context, int64, GuardianPhoneUpdate) error
	DeleteGuardianPhone(context.Context, int64) error
	SetPrimaryGuardianPhone(context.Context, int64) error
	LinkGuardianToStudent(context.Context, LinkGuardian) (GuardianLink, error)
	ValidateNewGuardians(context.Context, []NewStudentGuardian) error
	AddGuardiansToStudent(context.Context, int64, []NewStudentGuardian) error
	UpdateGuardianLink(context.Context, int64, GuardianLinkUpdate) error
	RemoveGuardianFromStudent(context.Context, RemoveGuardian) error
	RevealGuardianPayment(context.Context, int64, GuardianPaymentActor) (GuardianPayment, error)
	UpdateGuardianPayment(context.Context, int64, GuardianPaymentInput) error
	SetStudentPayer(context.Context, StudentPayer) error
}

// guardianEngine is the row-level half the composition supplies together
// with the person and student engines, plus the observation seam every
// provider-backed operation reports through so the guardian capability
// records the same per-operation evidence as the person and student reads.
type guardianEngine interface {
	ListGuardianLinksByAccount(context.Context, int64) ([]GuardianLink, error)
	ListGuardiansByAccounts(context.Context, []int64) ([]Guardian, error)
	ListGuardiansByIDs(context.Context, []int64) ([]Guardian, error)
	CountGuardianLinks(context.Context, []int64) (map[int64]int, error)
	ObserveGuardianOperation(operation string, duration time.Duration, err error)
}

// observed runs one provider-backed operation and reports its outcome under
// a stable operation name.
func (m *Module) observed(operation string, fn func() error) error {
	started := time.Now()
	err := fn()
	m.engine.ObserveGuardianOperation(operation, time.Since(started), err)
	return err
}

// BindGuardianProvider installs the application-level guardian provider.
// It is called once at composition time; a second call replaces the
// provider so test graphs can rebuild it.
func (m *Module) BindGuardianProvider(provider GuardianProvider) {
	if provider == nil {
		panic("people directory: guardian provider is required")
	}
	m.guardians.Store(&provider)
}

type guardianProviderSlot struct {
	value atomic.Pointer[GuardianProvider]
}

func (s *guardianProviderSlot) Store(provider *GuardianProvider) { s.value.Store(provider) }

func (s *guardianProviderSlot) Load() (GuardianProvider, error) {
	provider := s.value.Load()
	if provider == nil {
		return nil, ErrGuardianProviderUnbound
	}
	return *provider, nil
}

func invalidGuardian(reason string) error { return &InvalidGuardianError{Reason: reason} }
