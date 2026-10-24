// Package peopledirectory is the public People Directory capability. It owns
// users.persons, users.students, users.guardian_profiles,
// users.guardian_phone_numbers and users.student_guardian_relationships: every read or
// write of a person, student or guardian row by another owner goes through
// Query or Command instead of a foreign SQL join.
package peopledirectory

import (
	"context"
	"errors"
	"strings"
	"time"
)

const MaxNameLength = 200

// MaxPageSize caps a directory page; the legacy handler used the same limit.
const MaxPageSize = 100

var (
	ErrPersonNotFound  = errors.New("person not found")
	ErrInvalidPerson   = errors.New("invalid person")
	ErrTagConflict     = errors.New("tag is already linked to another person")
	ErrAccountConflict = errors.New("account is already linked to another person")
)

type InvalidPersonError struct{ Reason string }

func (e *InvalidPersonError) Error() string { return e.Reason }
func (e *InvalidPersonError) Unwrap() error { return ErrInvalidPerson }

// BirthdayLayout is the calendar-date wire format of Person.Birthday.
const BirthdayLayout = "2006-01-02"

// Person is the directory entry. Names are exposed for display; a deleted
// person carries DeletedAt and is excluded from every query. Birthday is a
// calendar date in BirthdayLayout, empty when unknown.
type Person struct {
	ID        int64      `json:"id"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	TenantID  int64      `json:"tenant_id"`
	FirstName string     `json:"first_name"`
	LastName  string     `json:"last_name"`
	Birthday  string     `json:"birthday,omitempty"`
	TagID     *string    `json:"tag_id,omitempty"`
	AccountID *int64     `json:"account_id,omitempty"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
}

func (p Person) FullName() string { return p.FirstName + " " + p.LastName }

// PersonDisplayName is the trimmed "first last" display name of a person, so
// a missing part leaves no stray space.
func PersonDisplayName(firstName, lastName string) string {
	return strings.TrimSpace(firstName + " " + lastName)
}

// PersonIsLive reports a person the directory has not soft-deleted: a deleted
// person carries its deletion time.
func PersonIsLive(deletedAt *time.Time) bool { return deletedAt == nil }

type CreatePerson struct {
	FirstName string
	LastName  string
	Birthday  string
	TagID     *string
	AccountID *int64
}

type UpdatePerson struct {
	ID        int64
	FirstName string
	LastName  string
	Birthday  string
	TagID     *string
	AccountID *int64
}

// PersonFilter narrows a directory search. Every field is optional; the
// prefix, contains and equals matches are case-insensitive, and the equals
// matches also ignore surrounding whitespace on both sides.
type PersonFilter struct {
	FirstNamePrefix  string
	LastNamePrefix   string
	FirstNameEquals  string
	LastNameEquals   string
	FullNameContains string
	TagID            string
	AccountIDs       []int64
	Page             int
	PageSize         int
}

// ReleasedTag records one tag the ReleaseTags command detached.
type ReleasedTag struct {
	PersonID int64
	TagID    string
}

type Query interface {
	FamilyProtectionQuery
	StudentDepartureQuery
	StudentFieldReviewQuery
	StudentQuery
	StudentDirectoryQuery
	StudentAuditQuery
	StudentPhotoQuery
	StudentConsentQuery
	StudentNoteQuery
	GuardianQuery
	GuardianPortalReachQuery
	GuardianPortalContacts
	// FindPerson returns one non-deleted person of the current tenant.
	FindPerson(context.Context, int64) (Person, error)
	// FindPersonForMutation locks the row for the caller's transaction.
	FindPersonForMutation(context.Context, int64) (Person, error)
	FindPersonByAccount(context.Context, int64) (Person, error)
	FindPersonByTag(context.Context, string) (Person, error)
	// ListPersonsByID returns the non-deleted persons of the current tenant
	// (or of every tenant inside an admin transaction) for the given IDs.
	ListPersonsByID(context.Context, []int64) ([]Person, error)
	// ListPersonsAcrossTenantsByID resolves names for visiting students
	// (holiday care at a partner school): it reads the given persons in a
	// separate admin transaction, so the hosting tenant's row-level security
	// does not hide the visitors' home-school rows. Callers must already hold
	// a reference to the person (a visit row) and must only use the names.
	ListPersonsAcrossTenantsByID(context.Context, []int64) ([]Person, error)
	ListPersonsByTenantIDs(context.Context, []int64) ([]Person, error)
	ListPersonsByAccount(context.Context, []int64) ([]Person, error)
	SearchPersons(context.Context, PersonFilter) ([]Person, error)
	// CountPersonsByTenant counts non-deleted persons per tenant across the
	// platform; it requires an admin transaction.
	CountPersonsByTenant(context.Context) (map[int64]int, error)
}

type Command interface {
	StudentCommand
	StudentDirectoryCommand
	StudentWriteCommand
	StudentDeletionCommand
	StudentAuditCommand
	StudentPhotoCommand
	StudentNoteCommand
	GuardianCommand
	GuardianPortalCommand
	StudentPortalCommand
	FamilyProtectionCommand
	AccountLifecycleCommand
	PersonDetailsCommand
	EnrollmentPersonCommand
	PersonRemovalCommand
	CreatePerson(context.Context, CreatePerson) (Person, error)
	UpdatePerson(context.Context, UpdatePerson) (Person, error)
	// DeletePerson soft-deletes the person (deleted_at), keeping the row.
	DeletePerson(context.Context, int64) error
	LinkAccount(context.Context, int64, int64) error
	UnlinkAccount(context.Context, int64) error
	LinkTag(context.Context, int64, string) error
	UnlinkTag(context.Context, int64) error
	// ReleaseTags locks the given persons and clears their tags, returning
	// the tags that were held. Persons without a tag are skipped.
	ReleaseTags(context.Context, []int64) ([]ReleasedTag, error)
	// RestoreTag re-links a tag to a person that currently holds none. It
	// reports false when the person already holds a tag or the tag has a new
	// holder in the tenant; that outcome never fails the caller's transaction.
	RestoreTag(context.Context, int64, string) (bool, error)
}

type Capability interface {
	StudentTagRelease
	Query
	Command
}

// NormalizeTagID mirrors the RFID card identifier format: trimmed, upper
// case, no separators. The same rule lives in models/users for the legacy
// provider; the directory applies it before every tag comparison.
func NormalizeTagID(tagID string) string {
	tagID = strings.ToUpper(strings.TrimSpace(tagID))
	return strings.NewReplacer(":", "", "-", "", " ", "").Replace(tagID)
}

// NormalizePersonNames trims the names without changing their spelling.
func NormalizePersonNames(firstName, lastName string) (string, string) {
	return strings.TrimSpace(firstName), strings.TrimSpace(lastName)
}

func normalizeTag(tagID *string) *string {
	if tagID == nil {
		return nil
	}
	normalized := NormalizeTagID(*tagID)
	if normalized == "" {
		return nil
	}
	return &normalized
}

func validateNames(firstName, lastName string) error {
	if firstName == "" {
		return invalid("first name is required")
	}
	if lastName == "" {
		return invalid("last name is required")
	}
	if len(firstName) > MaxNameLength || len(lastName) > MaxNameLength {
		return invalid("names must not exceed 200 characters")
	}
	return nil
}

// ValidatePersonReferences checks optional account and tag references.
func ValidatePersonReferences(tagID *string, accountID *int64) error {
	if accountID != nil && *accountID <= 0 {
		return invalid("account ID must be positive")
	}
	if tagID != nil && *tagID == "" {
		return invalid("tag ID must not be empty")
	}
	return nil
}

// ValidateBirthday accepts an empty value or a valid calendar date.
func ValidateBirthday(birthday string) error {
	if birthday == "" {
		return nil
	}
	if _, err := time.Parse(BirthdayLayout, birthday); err != nil {
		return invalid("birthday must be a calendar date in YYYY-MM-DD format")
	}
	return nil
}

// PositiveIDs preserves the order of distinct positive identifiers.
func PositiveIDs(ids []int64) []int64 {
	result := make([]int64, 0, len(ids))
	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result
}

// ErrorCode is the stable label recorded per operation outcome.
func ErrorCode(err error) string {
	switch {
	case err == nil:
		return "none"
	case errors.Is(err, ErrPersonNotFound), errors.Is(err, ErrStudentNotFound),
		errors.Is(err, ErrGuardianNotFound), errors.Is(err, ErrGuardianPhoneNotFound), errors.Is(err, ErrGuardianLinkNotFound):
		return "not_found"
	case errors.Is(err, ErrInvalidPerson), errors.Is(err, ErrInvalidStudent), errors.Is(err, ErrInvalidGuardian), errors.Is(err, ErrGuardianPaymentInvalid):
		return "invalid"
	case errors.Is(err, ErrGuardianStillLinked), errors.Is(err, ErrGuardianDeletePreviewChanged), errors.Is(err, ErrGuardianLinkConflict):
		return "guardian_linked"
	case errors.Is(err, ErrGuardianForceDeleteRequiresAdmin), errors.Is(err, ErrPayerRemovalRequiresFinancial):
		return "forbidden"
	case errors.Is(err, ErrGuardianProviderUnbound):
		return "unbound"
	case errors.Is(err, ErrTagConflict):
		return "tag_conflict"
	case errors.Is(err, ErrAccountConflict):
		return "account_conflict"
	default:
		return "internal_error"
	}
}

// NormalizePersonCreate applies the person command's input contract.
func NormalizePersonCreate(input CreatePerson) (CreatePerson, error) {
	input.FirstName, input.LastName = NormalizePersonNames(input.FirstName, input.LastName)
	input.TagID = normalizeTag(input.TagID)
	if err := validateNames(input.FirstName, input.LastName); err != nil {
		return input, err
	}
	if err := ValidatePersonReferences(input.TagID, input.AccountID); err != nil {
		return input, err
	}
	if err := ValidateBirthday(input.Birthday); err != nil {
		return input, err
	}
	return input, nil
}

// NormalizePersonUpdate applies the person command's input contract.
func NormalizePersonUpdate(input UpdatePerson) (UpdatePerson, error) {
	input.FirstName, input.LastName = NormalizePersonNames(input.FirstName, input.LastName)
	input.TagID = normalizeTag(input.TagID)
	if input.ID <= 0 {
		return input, invalid("person ID is required")
	}
	if err := validateNames(input.FirstName, input.LastName); err != nil {
		return input, err
	}
	if err := ValidatePersonReferences(input.TagID, input.AccountID); err != nil {
		return input, err
	}
	if err := ValidateBirthday(input.Birthday); err != nil {
		return input, err
	}
	return input, nil
}

func invalid(reason string) error { return &InvalidPersonError{Reason: reason} }

// Owner is the directory capability and its scheduled retention commands.
type Owner interface {
	Capability
	StudentStatusFlagCapability
	ChildlessGuardianRetention
	RetiredStaffRetention
}

// RetiredStaffRetention erases a retired person only while the read revision still holds.
type RetiredStaffRetention interface {
	AnonymizeRetiredStaffPerson(context.Context, int64, time.Time) (bool, error)
}
