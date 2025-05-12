// backend/database/repositories/users/person.go
package users

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

// PersonRepository implements users.PersonRepository interface
type PersonRepository struct {
	*base.Repository[*users.Person]
	db *bun.DB
}

// NewPersonRepository creates a new PersonRepository
func NewPersonRepository(db *bun.DB) users.PersonRepository {
	return &PersonRepository{
		Repository: base.NewRepository[*users.Person](db, "users.persons", "Person"),
		db:         db,
	}
}

// FindByTagID retrieves a person by their RFID tag ID
func (r *PersonRepository) FindByTagID(ctx context.Context, tagID string) (*users.Person, error) {
	person := new(users.Person)
	err := r.db.NewSelect().
		Model(person).
		Where("tag_id = ?", tagID).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by tag ID",
			Err: err,
		}
	}

	return person, nil
}

// FindByAccountID retrieves a person by their account ID
func (r *PersonRepository) FindByAccountID(ctx context.Context, accountID int64) (*users.Person, error) {
	person := new(users.Person)
	err := r.db.NewSelect().
		Model(person).
		Where("account_id = ?", accountID).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by account ID",
			Err: err,
		}
	}

	return person, nil
}

// LinkToAccount associates a person with an account
func (r *PersonRepository) LinkToAccount(ctx context.Context, personID int64, accountID int64) error {
	_, err := r.db.NewUpdate().
		Model((*users.Person)(nil)).
		Set("account_id = ?", accountID).
		Where("id = ?", personID).
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "link to account",
			Err: err,
		}
	}

	return nil
}

// UnlinkFromAccount removes account association from a person
func (r *PersonRepository) UnlinkFromAccount(ctx context.Context, personID int64) error {
	_, err := r.db.NewUpdate().
		Model((*users.Person)(nil)).
		Set("account_id = NULL").
		Where("id = ?", personID).
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "unlink from account",
			Err: err,
		}
	}

	return nil
}

// LinkToRFIDCard associates a person with an RFID card
func (r *PersonRepository) LinkToRFIDCard(ctx context.Context, personID int64, tagID string) error {
	_, err := r.db.NewUpdate().
		Model((*users.Person)(nil)).
		Set("tag_id = ?", tagID).
		Where("id = ?", personID).
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "link to RFID card",
			Err: err,
		}
	}

	return nil
}

// UnlinkFromRFIDCard removes RFID card association from a person
func (r *PersonRepository) UnlinkFromRFIDCard(ctx context.Context, personID int64) error {
	_, err := r.db.NewUpdate().
		Model((*users.Person)(nil)).
		Set("tag_id = NULL").
		Where("id = ?", personID).
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "unlink from RFID card",
			Err: err,
		}
	}

	return nil
}

// Create overrides the base Create method to handle validation
func (r *PersonRepository) Create(ctx context.Context, person *users.Person) error {
	if person == nil {
		return fmt.Errorf("person cannot be nil")
	}

	// Validate person
	if err := person.Validate(); err != nil {
		return err
	}

	// Use the base Create method
	return r.Repository.Create(ctx, person)
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

	// Use the base Update method
	return r.Repository.Update(ctx, person)
}

// ListWithOptions retrieves persons matching the provided query options
func (r *PersonRepository) ListWithOptions(ctx context.Context, options *modelBase.QueryOptions) ([]*users.Person, error) {
	var persons []*users.Person
	query := r.db.NewSelect().Model(&persons)

	// Apply query options
	if options != nil {
		query = options.ApplyToQuery(query)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "list",
			Err: err,
		}
	}

	return persons, nil
}

// FindWithAccount retrieves a person with their associated account
func (r *PersonRepository) FindWithAccount(ctx context.Context, id int64) (*users.Person, error) {
	person := new(users.Person)
	err := r.db.NewSelect().
		Model(person).
		Relation("Account").
		Where("id = ?", id).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find with account",
			Err: err,
		}
	}

	return person, nil
}

// FindWithRFIDCard retrieves a person with their associated RFID card
func (r *PersonRepository) FindWithRFIDCard(ctx context.Context, id int64) (*users.Person, error) {
	person := new(users.Person)
	err := r.db.NewSelect().
		Model(person).
		Relation("RFIDCard").
		Where("id = ?", id).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find with RFID card",
			Err: err,
		}
	}

	return person, nil
}

// Legacy method to maintain compatibility with old interface
func (r *PersonRepository) List(ctx context.Context, filters map[string]interface{}) ([]*users.Person, error) {
	// Convert old filter format to new QueryOptions
	options := modelBase.NewQueryOptions()
	filter := modelBase.NewFilter()

	for field, value := range filters {
		if value != nil {
			switch field {
			case "first_name_like":
				if strValue, ok := value.(string); ok {
					filter.Like("first_name", "%"+strValue+"%")
				}
			case "last_name_like":
				if strValue, ok := value.(string); ok {
					filter.Like("last_name", "%"+strValue+"%")
				}
			case "has_account":
				if boolValue, ok := value.(bool); ok && boolValue {
					filter.IsNotNull("account_id")
				} else if boolValue, ok := value.(bool); ok && !boolValue {
					filter.IsNull("account_id")
				}
			case "has_tag":
				if boolValue, ok := value.(bool); ok && boolValue {
					filter.IsNotNull("tag_id")
				} else if boolValue, ok := value.(bool); ok && !boolValue {
					filter.IsNull("tag_id")
				}
			default:
				// Default to exact match for other fields
				filter.Equal(field, value)
			}
		}
	}

	options.Filter = filter

	return r.ListWithOptions(ctx, options)
}

