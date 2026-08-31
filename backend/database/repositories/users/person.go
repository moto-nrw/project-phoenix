// backend/database/repositories/users/person.go
package users

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	modelAuth "github.com/moto-nrw/project-phoenix/models/auth"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

// Error messages (S1192 - avoid duplicate string literals)
const errPersonNotFound = "no person found with ID %d"

// unlinkField sets a person's field to NULL and handles common error patterns
func (r *PersonRepository) unlinkField(ctx context.Context, personID int64, fieldName, opName string) error {
	query := base.GetDB(ctx, r.db).NewUpdate().
		Model((*users.Person)(nil)).
		ModelTableExpr(`users.persons AS "person"`).
		Set(fieldName+" = NULL").
		Where(`"person".id = ?`, personID)

	query = base.WithTenantFilter(ctx, query, "person")

	result, err := query.Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  opName,
			Err: base.TranslateNotFound(err),
		}
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  opName + " - check rows affected",
			Err: base.TranslateNotFound(err),
		}
	}

	if rowsAffected == 0 {
		return &modelBase.DatabaseError{
			Op:  opName,
			Err: fmt.Errorf(errPersonNotFound, personID),
		}
	}

	return nil
}

// PersonRepository implements users.PersonRepository interface
type PersonRepository struct {
	*base.Repository[*users.Person]
	db *bun.DB
}

// NewPersonRepository creates a new PersonRepository
func NewPersonRepository(db *bun.DB) users.PersonRepository {
	repo := base.NewRepository[*users.Person](db, "users.persons", "Person")
	repo.TenantScoped = true
	return &PersonRepository{
		Repository: repo,
		db:         db,
	}
}

// FindByTagID retrieves a person by their RFID tag ID
func (r *PersonRepository) FindByTagID(ctx context.Context, tagID string) (*users.Person, error) {
	// Normalize the tag ID to match the stored format
	normalizedTagID := users.NormalizeTagID(tagID)

	person := new(users.Person)
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(person).
		ModelTableExpr(`users.persons AS "person"`).
		Where(`"person".tag_id = ?`, normalizedTagID)

	query = base.WithTenantFilter(ctx, query, "person")

	err := query.Scan(ctx)

	if err != nil {
		// Handle "no rows found" as a normal case, not an error
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, &modelBase.DatabaseError{
			Op:  "find by tag ID",
			Err: base.TranslateNotFound(err),
		}
	}

	return person, nil
}

// FindByAccountID retrieves a person by their account ID
func (r *PersonRepository) FindByAccountID(ctx context.Context, accountID int64) (*users.Person, error) {
	person := new(users.Person)
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(person).
		ModelTableExpr(`users.persons AS "person"`).
		Where(`"person".account_id = ?`, accountID)

	query = base.WithTenantFilter(ctx, query, "person")

	err := query.Scan(ctx)

	if err != nil {
		// Handle "no rows found" as a normal case, not an error
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, &modelBase.DatabaseError{
			Op:  "find by account ID",
			Err: base.TranslateNotFound(err),
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
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&persons).
		ModelTableExpr(`users.persons AS "person"`).
		Where(`"person".account_id IN (?)`, bun.List(accountIDs))

	query = base.WithTenantFilter(ctx, query, "person")

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by account IDs",
			Err: base.TranslateNotFound(err),
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
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(person).
		ModelTableExpr(`users.persons AS "person"`).
		Where(`"person".id = ?`, id).
		For("UPDATE")

	query = base.WithTenantFilter(ctx, query, "person")

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "find person for update", Err: base.TranslateNotFound(err)}
	}
	return person, nil
}

// FindByIDs retrieves multiple persons by their IDs in a single query
func (r *PersonRepository) FindByIDs(ctx context.Context, ids []int64) (map[int64]*users.Person, error) {
	if len(ids) == 0 {
		return make(map[int64]*users.Person), nil
	}

	var persons []*users.Person
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&persons).
		ModelTableExpr(`users.persons AS "person"`).
		Where(`"person".id IN (?)`, bun.List(ids))

	query = base.WithTenantFilter(ctx, query, "person")

	err := query.Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by IDs",
			Err: base.TranslateNotFound(err),
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
	query := base.GetDB(ctx, r.db).NewUpdate().
		Model((*users.Person)(nil)).
		ModelTableExpr(`users.persons AS "person"`).
		Set("account_id = ?", accountID).
		Where(`"person".id = ?`, personID)

	query = base.WithTenantFilter(ctx, query, "person")

	result, err := query.Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "link to account",
			Err: base.TranslateNotFound(err),
		}
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "link to account - check rows affected",
			Err: base.TranslateNotFound(err),
		}
	}

	if rowsAffected == 0 {
		return &modelBase.DatabaseError{
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

	query := base.GetDB(ctx, r.db).NewUpdate().
		Model((*users.Person)(nil)).
		ModelTableExpr(`users.persons AS "person"`).
		Set("tag_id = ?", normalizedTagID).
		Where(`"person".id = ?`, personID)

	query = base.WithTenantFilter(ctx, query, "person")

	result, err := query.Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "link to RFID card",
			Err: base.TranslateNotFound(err),
		}
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "link to RFID card - check rows affected",
			Err: base.TranslateNotFound(err),
		}
	}

	if rowsAffected == 0 {
		return &modelBase.DatabaseError{
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
	query := base.GetDB(ctx, r.db).NewUpdate().
		Model(person).
		ModelTableExpr(`users.persons AS "person"`).
		Column("first_name", "last_name", "birthday", "tag_id", "account_id").
		WherePK()

	query = base.WithTenantFilter(ctx, query, "person")

	result, err := query.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to update person: %w", err)
	}

	return base.AssertRowsAffected(result, 1, "update person")
}

// FindWithAccount retrieves a person with their associated account
func (r *PersonRepository) FindWithAccount(ctx context.Context, id int64) (*users.Person, error) {
	// Use a more explicit approach with result struct to avoid table name conflicts
	type personAccountResult struct {
		Person  *users.Person      `bun:"person"`
		Account *modelAuth.Account `bun:"account"`
	}

	result := &personAccountResult{
		Person:  new(users.Person),
		Account: new(modelAuth.Account),
	}

	query := base.GetDB(ctx, r.db).NewSelect().
		Model(result).
		ModelTableExpr(`users.persons AS "person"`).
		// Person columns with proper aliasing
		ColumnExpr(`"person".id AS "person__id", "person".created_at AS "person__created_at", "person".updated_at AS "person__updated_at"`).
		ColumnExpr(`"person".tenant_id AS "person__tenant_id"`).
		ColumnExpr(`"person".first_name AS "person__first_name", "person".last_name AS "person__last_name"`).
		ColumnExpr(`"person".birthday AS "person__birthday"`).
		ColumnExpr(`"person".tag_id AS "person__tag_id", "person".account_id AS "person__account_id"`).
		// Account columns
		ColumnExpr(`"account".id AS "account__id", "account".created_at AS "account__created_at", "account".updated_at AS "account__updated_at"`).
		ColumnExpr(`"account".email AS "account__email", "account".username AS "account__username"`).
		ColumnExpr(`"account".active AS "account__active", "account".last_login AS "account__last_login"`).
		ColumnExpr(`"account".pin_hash AS "account__pin_hash", "account".pin_attempts AS "account__pin_attempts", "account".pin_locked_until AS "account__pin_locked_until"`).
		// JOIN - Fixed to use auth.accounts directly rather than joining to a table alias "accounts"
		Join(`LEFT JOIN auth.accounts AS "account" ON ("account".id = "person".account_id)`).
		Where(`"person".id = ?`, id).
		Where(`"person".deleted_at IS NULL`)

	query = base.WithTenantFilter(ctx, query, "person")

	err := query.Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find with account",
			Err: base.TranslateNotFound(err),
		}
	}

	// Connect the account to the person
	if result.Account != nil && result.Account.ID != 0 {
		result.Person.Account = result.Account
	}

	return result.Person, nil
}

// FindWithRFIDCard retrieves a person with their associated RFID card
func (r *PersonRepository) FindWithRFIDCard(ctx context.Context, id int64) (*users.Person, error) {
	// Use a more explicit approach with result struct to avoid table name conflicts
	type personRFIDResult struct {
		Person   *users.Person   `bun:"person"`
		RFIDCard *users.RFIDCard `bun:"rfid_card"`
	}

	result := &personRFIDResult{
		Person:   new(users.Person),
		RFIDCard: new(users.RFIDCard),
	}

	query := base.GetDB(ctx, r.db).NewSelect().
		Model(result).
		ModelTableExpr(`users.persons AS "person"`).
		// Person columns with proper aliasing
		ColumnExpr(`"person".id AS "person__id", "person".created_at AS "person__created_at", "person".updated_at AS "person__updated_at"`).
		ColumnExpr(`"person".tenant_id AS "person__tenant_id"`).
		ColumnExpr(`"person".first_name AS "person__first_name", "person".last_name AS "person__last_name"`).
		ColumnExpr(`"person".tag_id AS "person__tag_id", "person".account_id AS "person__account_id"`).
		// RFID card columns
		ColumnExpr(`"rfid_card".id AS "rfid_card__id", "rfid_card".created_at AS "rfid_card__created_at", "rfid_card".updated_at AS "rfid_card__updated_at"`).
		ColumnExpr(`"rfid_card".is_active AS "rfid_card__is_active", "rfid_card".last_used AS "rfid_card__last_used"`).
		// JOIN
		Join(`LEFT JOIN users.rfid_cards AS "rfid_card" ON "rfid_card".id = "person".tag_id`).
		Where(`"person".id = ?`, id).
		Where(`"person".deleted_at IS NULL`)

	query = base.WithTenantFilter(ctx, query, "person")

	err := query.Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find with RFID card",
			Err: base.TranslateNotFound(err),
		}
	}

	// Connect the RFID card to the person
	if result.RFIDCard != nil && result.RFIDCard.ID != "" {
		result.Person.RFIDCard = result.RFIDCard
	}

	return result.Person, nil
}

// Legacy method to maintain compatibility with old interface
func (r *PersonRepository) List(ctx context.Context, filters map[string]interface{}) ([]*users.Person, error) {
	options := modelBase.NewQueryOptions()
	filter := modelBase.NewFilter()

	for field, value := range filters {
		if value != nil {
			applyPersonFilter(filter, field, value)
		}
	}

	options.Filter = filter
	return r.ListWithOptions(ctx, options)
}

// applyPersonFilter applies a single filter based on field name
func applyPersonFilter(filter *modelBase.Filter, field string, value interface{}) {
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
func applyPersonStringLikeFilter(filter *modelBase.Filter, column string, value interface{}) {
	if strValue, ok := value.(string); ok {
		filter.Like(column, "%"+strValue+"%")
	}
}

// applyNullableFieldFilter applies NULL/NOT NULL filter based on boolean value
func applyNullableFieldFilter(filter *modelBase.Filter, column string, value interface{}) {
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
	query := base.GetDB(ctx, r.db).NewUpdate().
		Model((*users.Person)(nil)).
		ModelTableExpr(`users.persons AS "person"`).
		Set(`first_name = ?`, "Gelöscht").
		Set(`last_name = ?`, "Benutzer").
		Set(`birthday = NULL`).
		Set(`deleted_at = NOW()`).
		Where(`"person".id = ?`, personID)

	query = base.WithTenantFilter(ctx, query, "person")

	if _, err := query.Exec(ctx); err != nil {
		return &modelBase.DatabaseError{
			Op:  "anonymize and soft delete person",
			Err: base.TranslateNotFound(err),
		}
	}
	return nil
}
