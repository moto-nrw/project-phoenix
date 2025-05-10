// backend/database/repositories/users/guest.go
package users

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

// GuestRepository implements users.GuestRepository interface
type GuestRepository struct {
	*base.Repository[*users.Guest]
	db *bun.DB
}

// NewGuestRepository creates a new GuestRepository
func NewGuestRepository(db *bun.DB) users.GuestRepository {
	return &GuestRepository{
		Repository: base.NewRepository[*users.Guest](db, "users.guests", "Guest"),
		db:         db,
	}
}

// FindByStaffID retrieves a guest by their staff ID
func (r *GuestRepository) FindByStaffID(ctx context.Context, staffID int64) (*users.Guest, error) {
	guest := new(users.Guest)
	err := r.db.NewSelect().
		Model(guest).
		Where("staff_id = ?", staffID).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by staff ID",
			Err: err,
		}
	}

	return guest, nil
}

// FindByOrganization retrieves guests by their organization
func (r *GuestRepository) FindByOrganization(ctx context.Context, organization string) ([]*users.Guest, error) {
	var guests []*users.Guest
	err := r.db.NewSelect().
		Model(&guests).
		Where("LOWER(organization) = LOWER(?)", organization).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by organization",
			Err: err,
		}
	}

	return guests, nil
}

// FindByExpertise retrieves guests by their activity expertise
func (r *GuestRepository) FindByExpertise(ctx context.Context, expertise string) ([]*users.Guest, error) {
	var guests []*users.Guest
	err := r.db.NewSelect().
		Model(&guests).
		Where("LOWER(activity_expertise) LIKE LOWER(?)", "%"+expertise+"%").
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by expertise",
			Err: err,
		}
	}

	return guests, nil
}

// FindActive retrieves currently active guests
func (r *GuestRepository) FindActive(ctx context.Context) ([]*users.Guest, error) {
	var guests []*users.Guest
	now := time.Now()

	err := r.db.NewSelect().
		Model(&guests).
		Where("(start_date IS NULL OR start_date <= ?) AND (end_date IS NULL OR end_date >= ?)", now, now).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find active",
			Err: err,
		}
	}

	return guests, nil
}

// SetDateRange sets a guest's start and end dates
func (r *GuestRepository) SetDateRange(ctx context.Context, id int64, startDate, endDate time.Time) error {
	_, err := r.db.NewUpdate().
		Model((*users.Guest)(nil)).
		Set("start_date = ?", startDate).
		Set("end_date = ?", endDate).
		Where("id = ?", id).
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "set date range",
			Err: err,
		}
	}

	return nil
}

// Create overrides the base Create method to handle validation
func (r *GuestRepository) Create(ctx context.Context, guest *users.Guest) error {
	if guest == nil {
		return fmt.Errorf("guest cannot be nil")
	}

	// Validate guest
	if err := guest.Validate(); err != nil {
		return err
	}

	// Use the base Create method
	return r.Repository.Create(ctx, guest)
}

// Update overrides the base Update method to handle validation
func (r *GuestRepository) Update(ctx context.Context, guest *users.Guest) error {
	if guest == nil {
		return fmt.Errorf("guest cannot be nil")
	}

	// Validate guest
	if err := guest.Validate(); err != nil {
		return err
	}

	// Use the base Update method
	return r.Repository.Update(ctx, guest)
}

// Legacy method to maintain compatibility with old interface
func (r *GuestRepository) List(ctx context.Context, filters map[string]interface{}) ([]*users.Guest, error) {
	// Convert old filter format to new QueryOptions
	options := modelBase.NewQueryOptions()
	filter := modelBase.NewFilter()

	for field, value := range filters {
		if value != nil {
			switch field {
			case "organization_like":
				if strValue, ok := value.(string); ok {
					filter.ILike("organization", "%"+strValue+"%")
				}
			case "expertise_like":
				if strValue, ok := value.(string); ok {
					filter.ILike("activity_expertise", "%"+strValue+"%")
				}
			case "active":
				if boolValue, ok := value.(bool); ok && boolValue {
					now := time.Now()
					// Create separate where conditions
					filter.Where("start_date IS NULL OR start_date <= ?", modelBase.OpEqual, now)
					filter.Where("end_date IS NULL OR end_date >= ?", modelBase.OpEqual, now)
				}
			case "current_date":
				if dateValue, ok := value.(time.Time); ok {
					// Create separate where conditions
					filter.Where("start_date IS NULL OR start_date <= ?", modelBase.OpEqual, dateValue)
					filter.Where("end_date IS NULL OR end_date >= ?", modelBase.OpEqual, dateValue)
				}
			case "has_organization":
				if boolValue, ok := value.(bool); ok && boolValue {
					filter.IsNotNull("organization")
				} else if boolValue, ok := value.(bool); ok && !boolValue {
					filter.IsNull("organization")
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

// ListWithOptions provides a type-safe way to list guests with query options
func (r *GuestRepository) ListWithOptions(ctx context.Context, options *modelBase.QueryOptions) ([]*users.Guest, error) {
	var guests []*users.Guest
	query := r.db.NewSelect().Model(&guests)

	// Apply query options
	if options != nil {
		query = options.ApplyToQuery(query)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "list with options",
			Err: err,
		}
	}

	return guests, nil
}

// FindWithStaff retrieves a guest with their associated staff data
func (r *GuestRepository) FindWithStaff(ctx context.Context, id int64) (*users.Guest, error) {
	guest := new(users.Guest)
	err := r.db.NewSelect().
		Model(guest).
		Relation("Staff").
		Where("id = ?", id).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find with staff",
			Err: err,
		}
	}

	return guest, nil
}

// FindWithStaffAndPerson retrieves a guest with their associated staff and person data
func (r *GuestRepository) FindWithStaffAndPerson(ctx context.Context, id int64) (*users.Guest, error) {
	guest := new(users.Guest)
	err := r.db.NewSelect().
		Model(guest).
		Relation("Staff").
		Relation("Staff.Person").
		Where("id = ?", id).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find with staff and person",
			Err: err,
		}
	}

	return guest, nil
}
