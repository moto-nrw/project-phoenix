package users

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// Guest represents a guest instructor entity in the system
type Guest struct {
	base.Model
	PersonID          int64     `bun:"person_id,notnull" json:"person_id"`
	Organization      string    `bun:"organization" json:"organization,omitempty"`
	ContactEmail      string    `bun:"contact_email" json:"contact_email,omitempty"`
	ContactPhone      string    `bun:"contact_phone" json:"contact_phone,omitempty"`
	ActivityExpertise string    `bun:"activity_expertise,notnull" json:"activity_expertise"`
	StartDate         time.Time `bun:"start_date" json:"start_date,omitempty"`
	EndDate           time.Time `bun:"end_date" json:"end_date,omitempty"`
	Notes             string    `bun:"notes" json:"notes,omitempty"`

	// Relations
	Person *Person `bun:"rel:belongs-to,join:person_id=id" json:"person,omitempty"`
}

// TableName returns the table name for the Guest model
func (g *Guest) TableName() string {
	return "users.guests"
}

// GetID returns the guest ID
func (g *Guest) GetID() interface{} {
	return g.ID
}

// GetCreatedAt returns the creation timestamp
func (g *Guest) GetCreatedAt() time.Time {
	return g.CreatedAt
}

// GetUpdatedAt returns the last update timestamp
func (g *Guest) GetUpdatedAt() time.Time {
	return g.UpdatedAt
}

// Validate validates the guest fields
func (g *Guest) Validate() error {
	if g.PersonID <= 0 {
		return errors.New("person ID is required")
	}

	if strings.TrimSpace(g.ActivityExpertise) == "" {
		return errors.New("activity expertise is required")
	}

	return nil
}

// BeforeAppend sets default values before saving to the database
func (g *Guest) BeforeAppend() error {
	// Call parent's BeforeAppend to set timestamps
	if err := g.Model.BeforeAppend(); err != nil {
		return err
	}

	// Trim whitespace
	g.Organization = strings.TrimSpace(g.Organization)
	g.ContactEmail = strings.TrimSpace(g.ContactEmail)
	g.ContactPhone = strings.TrimSpace(g.ContactPhone)
	g.ActivityExpertise = strings.TrimSpace(g.ActivityExpertise)
	g.Notes = strings.TrimSpace(g.Notes)

	return nil
}

// GuestRepository defines operations for working with guests
type GuestRepository interface {
	base.Repository[*Guest]
	FindByPersonID(ctx context.Context, personID int64) (*Guest, error)
	FindByOrganization(ctx context.Context, organization string) ([]*Guest, error)
	FindByActivityExpertise(ctx context.Context, expertise string) ([]*Guest, error)
	FindWithPerson(ctx context.Context, id int64) (*Guest, error)
}

// DefaultGuestRepository is the default implementation of GuestRepository
type DefaultGuestRepository struct {
	db *bun.DB
}

// NewGuestRepository creates a new guest repository
func NewGuestRepository(db *bun.DB) GuestRepository {
	return &DefaultGuestRepository{db: db}
}

// Create inserts a new guest into the database
func (r *DefaultGuestRepository) Create(ctx context.Context, guest *Guest) error {
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
func (r *DefaultGuestRepository) FindByID(ctx context.Context, id interface{}) (*Guest, error) {
	guest := new(Guest)
	err := r.db.NewSelect().Model(guest).Where("id = ?", id).Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_id", Err: err}
	}
	return guest, nil
}

// FindByPersonID retrieves a guest by their person ID
func (r *DefaultGuestRepository) FindByPersonID(ctx context.Context, personID int64) (*Guest, error) {
	guest := new(Guest)
	err := r.db.NewSelect().Model(guest).Where("person_id = ?", personID).Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_person_id", Err: err}
	}
	return guest, nil
}

// FindByOrganization retrieves guests by their organization
func (r *DefaultGuestRepository) FindByOrganization(ctx context.Context, organization string) ([]*Guest, error) {
	var guests []*Guest
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
func (r *DefaultGuestRepository) FindByActivityExpertise(ctx context.Context, expertise string) ([]*Guest, error) {
	var guests []*Guest
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
func (r *DefaultGuestRepository) FindWithPerson(ctx context.Context, id int64) (*Guest, error) {
	guest := new(Guest)
	err := r.db.NewSelect().
		Model(guest).
		Relation("Person").
		Where("guest.id = ?", id).
		Scan(ctx)

	if err != nil {
		return nil, &base.DatabaseError{Op: "find_with_person", Err: err}
	}
	return guest, nil
}

// Update updates an existing guest
func (r *DefaultGuestRepository) Update(ctx context.Context, guest *Guest) error {
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
func (r *DefaultGuestRepository) Delete(ctx context.Context, id interface{}) error {
	_, err := r.db.NewDelete().Model((*Guest)(nil)).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "delete", Err: err}
	}
	return nil
}

// List retrieves guests matching the filters
func (r *DefaultGuestRepository) List(ctx context.Context, filters map[string]interface{}) ([]*Guest, error) {
	var guests []*Guest
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
