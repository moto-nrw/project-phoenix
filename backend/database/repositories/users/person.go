// backend/database/repositories/users/person.go
package users

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

// Error messages (S1192 - avoid duplicate string literals)
const errPersonNotFound = "no person found with ID %d"

// unlinkField sets a person's column to NULL and handles common error patterns.
// The column name is bound as a quoted identifier.
func (r *PersonRepository) unlinkField(ctx context.Context, personID int64, fieldName, opName string) error {
	query := r.runtime.DB(ctx).NewUpdate().
		Model((*users.Person)(nil)).
		ModelTableExpr(`users.persons AS "person"`).
		Set("? = NULL", bun.Ident(fieldName)).
		Where(`"person".id = ?`, personID)

	query = withTenantFilter(ctx, r.runtime, query, "person")

	result, err := query.Exec(ctx)
	if err != nil {
		return &users.DatabaseError{
			Op:  opName,
			Err: translateNotFound(err),
		}
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return &users.DatabaseError{
			Op:  opName + " - check rows affected",
			Err: translateNotFound(err),
		}
	}

	if rowsAffected == 0 {
		return &users.DatabaseError{
			Op:  opName,
			Err: fmt.Errorf(errPersonNotFound, personID),
		}
	}

	return nil
}

// PersonRepository implements users.PersonRepository interface
type PersonRepository struct {
	runtime Runtime
	// accounts resolves the login account Identity & Access owns; without it
	// FindWithAccount fails closed instead of reading auth.accounts itself.
	accounts AccountLookup
}

// PersonOption configures a PersonRepository at construction.
type PersonOption func(*PersonRepository)

// WithAccountLookup installs the Identity & Access account lookup
// FindWithAccount attaches the person's account through (#2720).
func WithAccountLookup(lookup AccountLookup) PersonOption {
	return func(r *PersonRepository) { r.accounts = lookup }
}

// NewPersonRepository creates a new PersonRepository
func NewPersonRepository(runtime Runtime, options ...PersonOption) users.PersonRepository {
	repository := &PersonRepository{runtime: requireRuntime(runtime)}
	for _, option := range options {
		option(repository)
	}
	return repository
}

// FindByTagID retrieves a person by their RFID tag ID
func (r *PersonRepository) FindByTagID(ctx context.Context, tagID string) (*users.Person, error) {
	// Normalize the tag ID to match the stored format
	normalizedTagID := users.NormalizeTagID(tagID)

	person := new(users.Person)
	query := r.runtime.DB(ctx).NewSelect().
		Model(person).
		ModelTableExpr(`users.persons AS "person"`).
		Where(`"person".tag_id = ?`, normalizedTagID)

	query = withTenantFilter(ctx, r.runtime, query, "person")

	err := query.Scan(ctx)

	if err != nil {
		// Handle "no rows found" as a normal case, not an error
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, &users.DatabaseError{
			Op:  "find by tag ID",
			Err: translateNotFound(err),
		}
	}

	return person, nil
}

// FindByAccountID retrieves a person by their account ID
func (r *PersonRepository) FindByAccountID(ctx context.Context, accountID int64) (*users.Person, error) {
	person := new(users.Person)
	query := r.runtime.DB(ctx).NewSelect().
		Model(person).
		ModelTableExpr(`users.persons AS "person"`).
		Where(`"person".account_id = ?`, accountID)

	query = withTenantFilter(ctx, r.runtime, query, "person")

	err := query.Scan(ctx)

	if err != nil {
		// Handle "no rows found" as a normal case, not an error
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, &users.DatabaseError{
			Op:  "find by account ID",
			Err: translateNotFound(err),
		}
	}

	return person, nil
}

// FindByAccountIDs retrieves persons for the given account IDs in one query,
// keyed by account ID. Accounts without a person row are simply absent.
func (r *PersonRepository) FindByAccountIDs(ctx context.Context, accountIDs []int64) (map[int64]*users.Person, error) {
	if len(accountIDs) == 0 {
		return make(map[int64]*users.Person), nil
	}

	var persons []*users.Person
	query := r.runtime.DB(ctx).NewSelect().
		Model(&persons).
		ModelTableExpr(`users.persons AS "person"`).
		Where(`"person".account_id IN (?)`, bun.List(accountIDs))

	query = withTenantFilter(ctx, r.runtime, query, "person")

	if err := query.Scan(ctx); err != nil {
		return nil, &users.DatabaseError{
			Op:  "find by account IDs",
			Err: translateNotFound(err),
		}
	}

	result := make(map[int64]*users.Person, len(persons))
	for _, person := range persons {
		if person.AccountID != nil {
			result[*person.AccountID] = person
		}
	}
	return result, nil
}

// FindByIDForUpdate fetches and locks a person row for the current tenant
// transaction. Staff approval uses this before applying reviewed name/birthday
// changes so concurrent approvals of different person fields cannot overwrite
// each other with stale full-row snapshots.
func (r *PersonRepository) FindByIDForUpdate(ctx context.Context, id int64) (*users.Person, error) {
	person := new(users.Person)
	query := r.runtime.DB(ctx).NewSelect().
		Model(person).
		ModelTableExpr(`users.persons AS "person"`).
		Where(`"person".id = ?`, id).
		For("UPDATE")

	query = withTenantFilter(ctx, r.runtime, query, "person")

	if err := query.Scan(ctx); err != nil {
		return nil, &users.DatabaseError{Op: "find person for update", Err: translateNotFound(err)}
	}
	return person, nil
}

// FindByIDs retrieves multiple persons by their IDs in a single query
func (r *PersonRepository) FindByIDs(ctx context.Context, ids []int64) (map[int64]*users.Person, error) {
	if len(ids) == 0 {
		return make(map[int64]*users.Person), nil
	}

	var persons []*users.Person
	query := r.runtime.DB(ctx).NewSelect().
		Model(&persons).
		ModelTableExpr(`users.persons AS "person"`).
		Where(`"person".id IN (?)`, bun.List(ids))

	query = withTenantFilter(ctx, r.runtime, query, "person")

	err := query.Scan(ctx)

	if err != nil {
		return nil, &users.DatabaseError{
			Op:  "find by IDs",
			Err: translateNotFound(err),
		}
	}

	// Convert to map for O(1) lookups
	result := make(map[int64]*users.Person, len(persons))
	for _, person := range persons {
		result[person.ID] = person
	}

	return result, nil
}

// LinkToAccount associates a person with an account
func (r *PersonRepository) LinkToAccount(ctx context.Context, personID int64, accountID int64) error {
	query := r.runtime.DB(ctx).NewUpdate().
		Model((*users.Person)(nil)).
		ModelTableExpr(`users.persons AS "person"`).
		Set("account_id = ?", accountID).
		Where(`"person".id = ?`, personID)

	query = withTenantFilter(ctx, r.runtime, query, "person")

	result, err := query.Exec(ctx)

	if err != nil {
		return &users.DatabaseError{
			Op:  "link to account",
			Err: translateNotFound(err),
		}
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return &users.DatabaseError{
			Op:  "link to account - check rows affected",
			Err: translateNotFound(err),
		}
	}

	if rowsAffected == 0 {
		return &users.DatabaseError{
			Op:  "link to account",
			Err: fmt.Errorf(errPersonNotFound, personID),
		}
	}

	return nil
}

// UnlinkFromAccount removes account association from a person
func (r *PersonRepository) UnlinkFromAccount(ctx context.Context, personID int64) error {
	return r.unlinkField(ctx, personID, "account_id", "unlink from account")
}

// LinkToRFIDCard associates a person with an RFID card
func (r *PersonRepository) LinkToRFIDCard(ctx context.Context, personID int64, tagID string) error {
	// Normalize the tag ID to match RFID card format
	normalizedTagID := users.NormalizeTagID(tagID)

	query := r.runtime.DB(ctx).NewUpdate().
		Model((*users.Person)(nil)).
		ModelTableExpr(`users.persons AS "person"`).
		Set("tag_id = ?", normalizedTagID).
		Where(`"person".id = ?`, personID)

	query = withTenantFilter(ctx, r.runtime, query, "person")

	result, err := query.Exec(ctx)

	if err != nil {
		return &users.DatabaseError{
			Op:  "link to RFID card",
			Err: translateNotFound(err),
		}
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return &users.DatabaseError{
			Op:  "link to RFID card - check rows affected",
			Err: translateNotFound(err),
		}
	}

	if rowsAffected == 0 {
		return &users.DatabaseError{
			Op:  "link to RFID card",
			Err: fmt.Errorf(errPersonNotFound, personID),
		}
	}

	return nil
}

// UnlinkFromRFIDCard removes RFID card association from a person
func (r *PersonRepository) UnlinkFromRFIDCard(ctx context.Context, personID int64) error {
	return r.unlinkField(ctx, personID, "tag_id", "unlink from RFID card")
}

// Update overrides the base Update method to handle validation
func (r *PersonRepository) Update(ctx context.Context, person *users.Person) error {
	if person == nil {
		return fmt.Errorf("person cannot be nil")
	}

	// Validate person
	if err := person.Validate(); err != nil {
		return err
	}

	// Explicitly update all person fields (including NULL values)
	query := r.runtime.DB(ctx).NewUpdate().
		Model(person).
		ModelTableExpr(`users.persons AS "person"`).
		Column("first_name", "last_name", "birthday", "tag_id", "account_id").
		WherePK()

	query = withTenantFilter(ctx, r.runtime, query, "person")

	result, err := query.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to update person: %w", err)
	}

	return assertRowsAffected(result, 1, "update person")
}

// AccountLookup resolves the login account behind a person. Identity &
// Access owns auth.accounts, so the relation that used to be a LEFT JOIN is
// injected as an owner lookup; a missing account resolves to nil.
type AccountLookup func(ctx context.Context, accountID int64) (*users.PersonAccount, error)

// FindWithAccount retrieves a person with their associated account. The
// person row is read here; the account comes from the Identity & Access
// owner through the bound lookup and is attached when the person has one.
func (r *PersonRepository) FindWithAccount(ctx context.Context, id int64) (*users.Person, error) {
	if r.accounts == nil {
		return nil, &users.DatabaseError{Op: "find with account", Err: errors.New("account lookup is required")}
	}

	person := new(users.Person)
	query := r.runtime.DB(ctx).NewSelect().
		Model(person).
		ModelTableExpr(`users.persons AS "person"`).
		Where(`"person".id = ?`, id).
		Where(`"person".deleted_at IS NULL`)

	query = withTenantFilter(ctx, r.runtime, query, "person")

	if err := query.Scan(ctx); err != nil {
		return nil, &users.DatabaseError{
			Op:  "find with account",
			Err: translateNotFound(err),
		}
	}

	if person.AccountID == nil {
		return person, nil
	}
	account, err := r.accounts(ctx, *person.AccountID)
	if err != nil {
		return nil, &users.DatabaseError{Op: "find with account", Err: err}
	}
	if account != nil && account.ID != 0 {
		person.Account = account
	}

	return person, nil
}

// Legacy method to maintain compatibility with old interface
func (r *PersonRepository) List(ctx context.Context, filters map[string]interface{}) ([]*users.Person, error) {
	options := users.NewQueryOptions()
	filter := users.NewQueryFilter()

	for field, value := range filters {
		if value != nil {
			applyPersonFilter(filter, field, value)
		}
	}

	options.Filter = filter
	return r.ListWithOptions(ctx, options)
}

// applyPersonFilter applies a single filter based on field name
func applyPersonFilter(filter *users.QueryFilter, field string, value interface{}) {
	switch field {
	case "first_name_like":
		applyPersonStringLikeFilter(filter, "first_name", value)
	case "last_name_like":
		applyPersonStringLikeFilter(filter, "last_name", value)
	case "has_account":
		applyNullableFieldFilter(filter, "account_id", value)
	case "has_tag":
		applyNullableFieldFilter(filter, "tag_id", value)
	default:
		filter.Equal(field, value)
	}
}

// applyPersonStringLikeFilter applies LIKE filter for string fields
func applyPersonStringLikeFilter(filter *users.QueryFilter, column string, value interface{}) {
	if strValue, ok := value.(string); ok {
		filter.Like(column, "%"+strValue+"%")
	}
}

// applyNullableFieldFilter applies NULL/NOT NULL filter based on boolean value
func applyNullableFieldFilter(filter *users.QueryFilter, column string, value interface{}) {
	if boolValue, ok := value.(bool); ok {
		if boolValue {
			filter.IsNotNull(column)
		} else {
			filter.IsNull(column)
		}
	}
}

// AnonymizeAndSoftDelete overwrites the person's PII with placeholder values
// and stamps deleted_at. Custom method (backend-conventions Rule 2): GDPR
// person-deletion step combining anonymization and soft delete in one
// statement; used by operator SoftDeletePerson. Cross-tenant by design when
// the context carries no tenant (operator admin transactions).
func (r *PersonRepository) AnonymizeAndSoftDelete(ctx context.Context, personID int64) error {
	query := r.runtime.DB(ctx).NewUpdate().
		Model((*users.Person)(nil)).
		ModelTableExpr(`users.persons AS "person"`).
		Set(`first_name = ?`, "Gelöscht").
		Set(`last_name = ?`, "Benutzer").
		Set(`birthday = NULL`).
		Set(`deleted_at = NOW()`).
		Where(`"person".id = ?`, personID)

	query = withTenantFilter(ctx, r.runtime, query, "person")

	if _, err := query.Exec(ctx); err != nil {
		return &users.DatabaseError{
			Op:  "anonymize and soft delete person",
			Err: translateNotFound(err),
		}
	}
	return nil
}

// Create inserts a new person, stamping the context's tenant when the row has
// none yet.
func (r *PersonRepository) Create(ctx context.Context, person *users.Person) error {
	if person == nil {
		return fmt.Errorf("%s cannot be nil or zero value", "Person")
	}
	if err := person.Validate(); err != nil {
		return err
	}
	ensureTenantID(ctx, r.runtime, person)
	if _, err := r.runtime.DB(ctx).NewInsert().Model(person).ModelTableExpr(`users.persons`).Exec(ctx); err != nil {
		return &users.DatabaseError{Op: "create", Err: err}
	}
	return nil
}

// FindByID retrieves a person by ID.
func (r *PersonRepository) FindByID(ctx context.Context, id any) (*users.Person, error) {
	person := new(users.Person)
	query := r.runtime.DB(ctx).NewSelect().
		Model(person).
		ModelTableExpr(`users.persons AS "person"`).
		Where(`"person".id = ?`, id)
	query = withTenantFilter(ctx, r.runtime, query, "person")
	if err := query.Scan(ctx); err != nil {
		return nil, &users.DatabaseError{Op: "find by id", Err: translateNotFound(err)}
	}
	return person, nil
}

// Delete removes a person row.
func (r *PersonRepository) Delete(ctx context.Context, id any) error {
	query := r.runtime.DB(ctx).NewDelete().
		Model((*users.Person)(nil)).
		ModelTableExpr(`users.persons AS "person"`).
		Where(`"person".id = ?`, id)
	query = withTenantFilter(ctx, r.runtime, query, "person")
	if _, err := query.Exec(ctx); err != nil {
		return &users.DatabaseError{Op: "delete", Err: err}
	}
	return nil
}

// ListWithOptions retrieves the persons matching the query options; no match
// is an empty list, not nil.
func (r *PersonRepository) ListWithOptions(ctx context.Context, options *users.QueryOptions) ([]*users.Person, error) {
	persons := make([]*users.Person, 0)
	query := r.runtime.DB(ctx).NewSelect().
		Model(&persons).
		ModelTableExpr(`users.persons AS "person"`)
	query = withTenantFilter(ctx, r.runtime, query, "person")
	if options != nil && options.Filter != nil {
		options.Filter.WithTableAlias("person")
	}
	query = applyQueryOptions(query, options)
	if err := query.Scan(ctx); err != nil {
		return nil, &users.DatabaseError{Op: "list with options", Err: err}
	}
	return persons, nil
}
