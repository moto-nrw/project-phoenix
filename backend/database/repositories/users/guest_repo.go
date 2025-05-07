package users

import (
	"context"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

// GuestRepository implements users.GuestRepository
type GuestRepository struct {
	db *bun.DB
}

// NewGuestRepository creates a new guest repository
func NewGuestRepository(db *bun.DB) users.GuestRepository {
	return &GuestRepository{db: db}
}

// Create inserts a new guest into the database
func (r *GuestRepository) Create(ctx context.Context, guest *users.Guest) error {
	if err := guest.Validate(); err != nil {
		return err
	}

	_, err := r.db.NewInsert().Model(guest).Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "create", Err: err}
	}

	return nil
}

// FindByID retrieves a guest by their ID
func (r *GuestRepository) FindByID(ctx context.Context, id interface{}) (*users.Guest, error) {
	guest := new(users.Guest)
	err := r.db.NewSelect().Model(guest).Where("id = ?", id).Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_id", Err: err}
	}
	return guest, nil
}

// FindByPersonID retrieves a guest by their person ID
func (r *GuestRepository) FindByPersonID(ctx context.Context, personID int64) (*users.Guest, error) {
	guest := new(users.Guest)
	err := r.db.NewSelect().Model(guest).Where("person_id = ?", personID).Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_person_id", Err: err}
	}
	return guest, nil
}

// FindByOrganization retrieves guests by their organization
func (r *GuestRepository) FindByOrganization(ctx context.Context, organization string) ([]*users.Guest, error) {
	var guests []*users.Guest
	err := r.db.NewSelect().
		Model(&guests).
		Where("organization ILIKE ?", "%"+organization+"%").
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_organization", Err: err}
	}
	return guests, nil
}

// FindByActivityExpertise retrieves guests by their activity expertise
func (r *GuestRepository) FindByActivityExpertise(ctx context.Context, expertise string) ([]*users.Guest, error) {
	var guests []*users.Guest
	err := r.db.NewSelect().
		Model(&guests).
		Where("activity_expertise ILIKE ?", "%"+expertise+"%").
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_activity_expertise", Err: err}
	}
	return guests, nil
}

// FindWithPerson retrieves a guest with their associated person data
func (r *GuestRepository) FindWithPerson(ctx context.Context, id int64) (*users.Guest, error) {
	guest := new(users.Guest)
	err := r.db.NewSelect().
		Model(guest).
		Relation("Person").
		Where("guests.id = ?", id).
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_with_person", Err: err}
	}
	return guest, nil
}

// Update updates an existing guest
func (r *GuestRepository) Update(ctx context.Context, guest *users.Guest) error {
	if err := guest.Validate(); err != nil {
		return err
	}

	_, err := r.db.NewUpdate().Model(guest).WherePK().Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "update", Err: err}
	}
	return nil
}

// Delete removes a guest
func (r *GuestRepository) Delete(ctx context.Context, id interface{}) error {
	_, err := r.db.NewDelete().Model((*users.Guest)(nil)).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "delete", Err: err}
	}
	return nil
}

// List retrieves guests matching the filters
func (r *GuestRepository) List(ctx context.Context, filters map[string]interface{}) ([]*users.Guest, error) {
	var guests []*users.Guest
	query := r.db.NewSelect().Model(&guests)

	// Apply filters
	for key, value := range filters {
		query = query.Where("? = ?", bun.Ident(key), value)
	}

	// Execute the query
	err := query.Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "list", Err: err}
	}

	return guests, nil
}
