package users

import (
	"context"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

// PersonRepository implements users.PersonRepository
type PersonRepository struct {
	db *bun.DB
}

// NewPersonRepository creates a new person repository
func NewPersonRepository(db *bun.DB) users.PersonRepository {
	return &PersonRepository{db: db}
}

// Create inserts a new person into the database
func (r *PersonRepository) Create(ctx context.Context, person *users.Person) error {
	if err := person.Validate(); err != nil {
		return err
	}

	_, err := r.db.NewInsert().Model(person).Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "create", Err: err}
	}

	return nil
}

// FindByID retrieves a person by their ID
func (r *PersonRepository) FindByID(ctx context.Context, id interface{}) (*users.Person, error) {
	person := new(users.Person)
	err := r.db.NewSelect().Model(person).Where("id = ?", id).Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_id", Err: err}
	}
	return person, nil
}

// FindByTagID retrieves a person by their tag ID
func (r *PersonRepository) FindByTagID(ctx context.Context, tagID string) (*users.Person, error) {
	person := new(users.Person)
	err := r.db.NewSelect().Model(person).Where("tag_id = ?", tagID).Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_tag_id", Err: err}
	}
	return person, nil
}

// FindByAccountID retrieves a person by their account ID
func (r *PersonRepository) FindByAccountID(ctx context.Context, accountID int64) (*users.Person, error) {
	person := new(users.Person)
	err := r.db.NewSelect().Model(person).Where("account_id = ?", accountID).Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_account_id", Err: err}
	}
	return person, nil
}

// FindByName retrieves persons by their first and last name
func (r *PersonRepository) FindByName(ctx context.Context, firstName, lastName string) ([]*users.Person, error) {
	var persons []*users.Person
	err := r.db.NewSelect().
		Model(&persons).
		Where("first_name ILIKE ?", firstName+"%").
		Where("last_name ILIKE ?", lastName+"%").
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_name", Err: err}
	}
	return persons, nil
}

// Update updates an existing person
func (r *PersonRepository) Update(ctx context.Context, person *users.Person) error {
	if err := person.Validate(); err != nil {
		return err
	}

	_, err := r.db.NewUpdate().Model(person).WherePK().Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "update", Err: err}
	}
	return nil
}

// Delete removes a person
func (r *PersonRepository) Delete(ctx context.Context, id interface{}) error {
	_, err := r.db.NewDelete().Model((*users.Person)(nil)).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "delete", Err: err}
	}
	return nil
}

// List retrieves persons matching the filters
func (r *PersonRepository) List(ctx context.Context, filters map[string]interface{}) ([]*users.Person, error) {
	var persons []*users.Person
	query := r.db.NewSelect().Model(&persons)

	// Apply filters
	for key, value := range filters {
		query = query.Where("? = ?", bun.Ident(key), value)
	}

	// Execute the query
	err := query.Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "list", Err: err}
	}

	return persons, nil
}
