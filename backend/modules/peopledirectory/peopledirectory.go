// Package peopledirectory is the public People Directory capability. It owns
// users.persons: every read or write of a person row by another owner goes
// through Query or Command instead of a foreign SQL join.
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
// prefix and contains matches are case-insensitive.
type PersonFilter struct {
	FirstNamePrefix  string
	LastNamePrefix   string
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
	ListPersonsByAccount(context.Context, []int64) ([]Person, error)
	SearchPersons(context.Context, PersonFilter) ([]Person, error)
	// CountPersonsByTenant counts non-deleted persons per tenant across the
	// platform; it requires an admin transaction.
	CountPersonsByTenant(context.Context) (map[int64]int, error)
}

type Command interface {
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
	Query
	Command
}

type engine interface {
	Create(context.Context, CreatePerson) (Person, error)
	Update(context.Context, UpdatePerson) (Person, error)
	Delete(context.Context, int64) error
	FindByID(context.Context, int64, string) (Person, error)
	FindByAccount(context.Context, int64) (Person, error)
	FindByTag(context.Context, string) (Person, error)
	ListByIDs(context.Context, []int64) ([]Person, error)
	ListAcrossTenantsByIDs(context.Context, []int64) ([]Person, error)
	ListByAccounts(context.Context, []int64) ([]Person, error)
	Search(context.Context, PersonFilter) ([]Person, error)
	CountByTenant(context.Context) (map[int64]int, error)
	LinkAccount(context.Context, int64, int64) error
	UnlinkAccount(context.Context, int64) error
	LinkTag(context.Context, int64, string) error
	UnlinkTag(context.Context, int64) error
	ReleaseTags(context.Context, []int64) ([]ReleasedTag, error)
	RestoreTag(context.Context, int64, string) (bool, error)
}

type Module struct{ engine engine }

func NewModule(engine engine) *Module {
	if engine == nil {
		panic("people directory: engine is required")
	}
	return &Module{engine: engine}
}

func (m *Module) CreatePerson(ctx context.Context, input CreatePerson) (Person, error) {
	input.FirstName, input.LastName = normalizeNames(input.FirstName, input.LastName)
	input.TagID = normalizeTag(input.TagID)
	if err := validateNames(input.FirstName, input.LastName); err != nil {
		return Person{}, err
	}
	if err := validateReferences(input.TagID, input.AccountID); err != nil {
		return Person{}, err
	}
	if err := validateBirthday(input.Birthday); err != nil {
		return Person{}, err
	}
	return m.engine.Create(ctx, input)
}

func (m *Module) UpdatePerson(ctx context.Context, input UpdatePerson) (Person, error) {
	input.FirstName, input.LastName = normalizeNames(input.FirstName, input.LastName)
	input.TagID = normalizeTag(input.TagID)
	if input.ID <= 0 {
		return Person{}, invalid("person ID is required")
	}
	if err := validateNames(input.FirstName, input.LastName); err != nil {
		return Person{}, err
	}
	if err := validateReferences(input.TagID, input.AccountID); err != nil {
		return Person{}, err
	}
	if err := validateBirthday(input.Birthday); err != nil {
		return Person{}, err
	}
	return m.engine.Update(ctx, input)
}

func (m *Module) DeletePerson(ctx context.Context, id int64) error {
	if id <= 0 {
		return invalid("person ID is required")
	}
	return m.engine.Delete(ctx, id)
}

func (m *Module) FindPerson(ctx context.Context, id int64) (Person, error) {
	if id <= 0 {
		return Person{}, invalid("person ID is required")
	}
	return m.engine.FindByID(ctx, id, "")
}

func (m *Module) FindPersonForMutation(ctx context.Context, id int64) (Person, error) {
	if id <= 0 {
		return Person{}, invalid("person ID is required")
	}
	return m.engine.FindByID(ctx, id, "UPDATE")
}

func (m *Module) FindPersonByAccount(ctx context.Context, accountID int64) (Person, error) {
	if accountID <= 0 {
		return Person{}, invalid("account ID is required")
	}
	return m.engine.FindByAccount(ctx, accountID)
}

func (m *Module) FindPersonByTag(ctx context.Context, tagID string) (Person, error) {
	tagID = NormalizeTagID(tagID)
	if tagID == "" {
		return Person{}, invalid("tag ID is required")
	}
	return m.engine.FindByTag(ctx, tagID)
}

func (m *Module) ListPersonsByID(ctx context.Context, ids []int64) ([]Person, error) {
	ids = uniquePositive(ids)
	if len(ids) == 0 {
		return []Person{}, nil
	}
	return m.engine.ListByIDs(ctx, ids)
}

func (m *Module) ListPersonsAcrossTenantsByID(ctx context.Context, ids []int64) ([]Person, error) {
	ids = uniquePositive(ids)
	if len(ids) == 0 {
		return []Person{}, nil
	}
	return m.engine.ListAcrossTenantsByIDs(ctx, ids)
}

func (m *Module) ListPersonsByAccount(ctx context.Context, accountIDs []int64) ([]Person, error) {
	accountIDs = uniquePositive(accountIDs)
	if len(accountIDs) == 0 {
		return []Person{}, nil
	}
	return m.engine.ListByAccounts(ctx, accountIDs)
}

func (m *Module) SearchPersons(ctx context.Context, filter PersonFilter) ([]Person, error) {
	filter.FirstNamePrefix = strings.TrimSpace(filter.FirstNamePrefix)
	filter.LastNamePrefix = strings.TrimSpace(filter.LastNamePrefix)
	filter.FullNameContains = strings.TrimSpace(filter.FullNameContains)
	filter.TagID = NormalizeTagID(filter.TagID)
	filter.AccountIDs = uniquePositive(filter.AccountIDs)
	if filter.Page < 0 || filter.PageSize < 0 {
		return nil, invalid("page and page size must not be negative")
	}
	if filter.PageSize > MaxPageSize {
		filter.PageSize = MaxPageSize
	}
	return m.engine.Search(ctx, filter)
}

func (m *Module) CountPersonsByTenant(ctx context.Context) (map[int64]int, error) {
	return m.engine.CountByTenant(ctx)
}

func (m *Module) LinkAccount(ctx context.Context, personID, accountID int64) error {
	if personID <= 0 || accountID <= 0 {
		return invalid("person ID and account ID are required")
	}
	return m.engine.LinkAccount(ctx, personID, accountID)
}

func (m *Module) UnlinkAccount(ctx context.Context, personID int64) error {
	if personID <= 0 {
		return invalid("person ID is required")
	}
	return m.engine.UnlinkAccount(ctx, personID)
}

func (m *Module) LinkTag(ctx context.Context, personID int64, tagID string) error {
	tagID = NormalizeTagID(tagID)
	if personID <= 0 || tagID == "" {
		return invalid("person ID and tag ID are required")
	}
	return m.engine.LinkTag(ctx, personID, tagID)
}

func (m *Module) UnlinkTag(ctx context.Context, personID int64) error {
	if personID <= 0 {
		return invalid("person ID is required")
	}
	return m.engine.UnlinkTag(ctx, personID)
}

func (m *Module) ReleaseTags(ctx context.Context, personIDs []int64) ([]ReleasedTag, error) {
	personIDs = uniquePositive(personIDs)
	if len(personIDs) == 0 {
		return []ReleasedTag{}, nil
	}
	return m.engine.ReleaseTags(ctx, personIDs)
}

func (m *Module) RestoreTag(ctx context.Context, personID int64, tagID string) (bool, error) {
	tagID = NormalizeTagID(tagID)
	if personID <= 0 {
		return false, invalid("person ID is required")
	}
	if tagID == "" {
		return false, nil
	}
	return m.engine.RestoreTag(ctx, personID, tagID)
}

// NormalizeTagID mirrors the RFID card identifier format: trimmed, upper
// case, no separators. The same rule lives in models/users for the legacy
// provider; the directory applies it before every tag comparison.
func NormalizeTagID(tagID string) string {
	tagID = strings.ToUpper(strings.TrimSpace(tagID))
	return strings.NewReplacer(":", "", "-", "", " ", "").Replace(tagID)
}

func normalizeNames(firstName, lastName string) (string, string) {
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

func validateReferences(tagID *string, accountID *int64) error {
	if accountID != nil && *accountID <= 0 {
		return invalid("account ID must be positive")
	}
	if tagID != nil && *tagID == "" {
		return invalid("tag ID must not be empty")
	}
	return nil
}

func validateBirthday(birthday string) error {
	if birthday == "" {
		return nil
	}
	if _, err := time.Parse(BirthdayLayout, birthday); err != nil {
		return invalid("birthday must be a calendar date in YYYY-MM-DD format")
	}
	return nil
}

func uniquePositive(ids []int64) []int64 {
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

func invalid(reason string) error { return &InvalidPersonError{Reason: reason} }

// ErrorCode is the stable label recorded per operation outcome.
func ErrorCode(err error) string {
	switch {
	case err == nil:
		return "none"
	case errors.Is(err, ErrPersonNotFound):
		return "not_found"
	case errors.Is(err, ErrInvalidPerson):
		return "invalid"
	case errors.Is(err, ErrTagConflict):
		return "tag_conflict"
	case errors.Is(err, ErrAccountConflict):
		return "account_conflict"
	default:
		return "internal_error"
	}
}
