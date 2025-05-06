package users

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// Person represents a person entity in the system
type Person struct {
	base.Model
	FirstName string `bun:"first_name,notnull" json:"first_name"`
	LastName  string `bun:"last_name,notnull" json:"last_name"`
	TagID     string `bun:"tag_id" json:"tag_id,omitempty"`
	AccountID int64  `bun:"account_id" json:"account_id,omitempty"`

	// Relations
	RFIDCard *RFIDCard     `bun:"rel:belongs-to,join:tag_id=id" json:"rfid_card,omitempty"`
	Account  *auth.Account `bun:"rel:belongs-to,join:account_id=id" json:"account,omitempty"`
	Teacher  *Teacher      `bun:"rel:has-one,join:id=person_id" json:"teacher,omitempty"`
	Guest    *Guest        `bun:"rel:has-one,join:id=person_id" json:"guest,omitempty"`
}

// TableName returns the table name for the Person model
func (p *Person) TableName() string {
	return "users.persons"
}

// GetID returns the person ID
func (p *Person) GetID() interface{} {
	return p.ID
}

// GetCreatedAt returns the creation timestamp
func (p *Person) GetCreatedAt() time.Time {
	return p.CreatedAt
}

// GetUpdatedAt returns the last update timestamp
func (p *Person) GetUpdatedAt() time.Time {
	return p.UpdatedAt
}

// FullName returns the full name of the person
func (p *Person) FullName() string {
	return p.FirstName + " " + p.LastName
}

// Validate validates the person fields
func (p *Person) Validate() error {
	if strings.TrimSpace(p.FirstName) == "" {
		return errors.New("first name is required")
	}

	if strings.TrimSpace(p.LastName) == "" {
		return errors.New("last name is required")
	}

	// At least one identifier should be present in most cases,
	// but we're not enforcing it at the model level as per the migration comment
	return nil
}

// BeforeAppend sets default values before saving to the database
func (p *Person) BeforeAppend() error {
	// Call parent's BeforeAppend to set timestamps
	if err := p.Model.BeforeAppend(); err != nil {
		return err
	}

	// Trim whitespace from names
	p.FirstName = strings.TrimSpace(p.FirstName)
	p.LastName = strings.TrimSpace(p.LastName)

	return nil
}

// PersonRepository defines operations for working with persons
type PersonRepository interface {
	base.Repository[*Person]
	FindByTagID(ctx context.Context, tagID string) (*Person, error)
	FindByAccountID(ctx context.Context, accountID int64) (*Person, error)
	FindByName(ctx context.Context, firstName, lastName string) ([]*Person, error)
}

// DefaultPersonRepository is the default implementation of PersonRepository
type DefaultPersonRepository struct {
	db *bun.DB
}

// NewPersonRepository creates a new person repository
func NewPersonRepository(db *bun.DB) PersonRepository {
	return &DefaultPersonRepository{db: db}
}

// Create inserts a new person into the database
func (r *DefaultPersonRepository) Create(ctx context.Context, person *Person) error {
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
func (r *DefaultPersonRepository) FindByID(ctx context.Context, id interface{}) (*Person, error) {
	person := new(Person)
	err := r.db.NewSelect().Model(person).Where("id = ?", id).Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_id", Err: err}
	}
	return person, nil
}

// FindByTagID retrieves a person by their tag ID
func (r *DefaultPersonRepository) FindByTagID(ctx context.Context, tagID string) (*Person, error) {
	person := new(Person)
	err := r.db.NewSelect().Model(person).Where("tag_id = ?", tagID).Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_tag_id", Err: err}
	}
	return person, nil
}

// FindByAccountID retrieves a person by their account ID
func (r *DefaultPersonRepository) FindByAccountID(ctx context.Context, accountID int64) (*Person, error) {
	person := new(Person)
	err := r.db.NewSelect().Model(person).Where("account_id = ?", accountID).Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_account_id", Err: err}
	}
	return person, nil
}

// FindByName retrieves persons by their first and last name
func (r *DefaultPersonRepository) FindByName(ctx context.Context, firstName, lastName string) ([]*Person, error) {
	var persons []*Person
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
func (r *DefaultPersonRepository) Update(ctx context.Context, person *Person) error {
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
func (r *DefaultPersonRepository) Delete(ctx context.Context, id interface{}) error {
	_, err := r.db.NewDelete().Model((*Person)(nil)).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "delete", Err: err}
	}
	return nil
}

// List retrieves persons matching the filters
func (r *DefaultPersonRepository) List(ctx context.Context, filters map[string]interface{}) ([]*Person, error) {
	var persons []*Person
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
