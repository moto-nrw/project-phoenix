// backend/database/repositories/users/guest.go
package users

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/adapter/repository/postgres/base"
	modelBase "github.com/moto-nrw/project-phoenix/internal/core/domain/base"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/users"
	userPort "github.com/moto-nrw/project-phoenix/internal/core/port/users"
	"github.com/uptrace/bun"
)

// Table and query constants (S1192 - avoid duplicate string literals)
const (
	tableUsersGuests       = "users.guests"
	tableExprGuestsAsGuest = `users.guests AS "guest"`
	whereGuestIDEquals     = "id = ?"
)

// GuestRepository implements users.GuestRepository interface
type GuestRepository struct {
	*base.Repository[*users.Guest]
	db *bun.DB
}

// NewGuestRepository creates a new GuestRepository
func NewGuestRepository(db *bun.DB) userPort.GuestRepository {
	return &GuestRepository{
		Repository: base.NewRepository[*users.Guest](db, tableUsersGuests, "Guest"),
		db:         db,
	}
}

// FindByStaffID retrieves a guest by their staff ID
func (r *GuestRepository) FindByStaffID(ctx context.Context, staffID int64) (*users.Guest, error) {
	guest := new(users.Guest)
	err := r.db.NewSelect().
		Model(guest).
		ModelTableExpr(tableExprGuestsAsGuest).
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
		ModelTableExpr(tableExprGuestsAsGuest).
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
		ModelTableExpr(tableExprGuestsAsGuest).
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
		ModelTableExpr(tableExprGuestsAsGuest).
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
		Table(tableUsersGuests).
		Set("start_date = ?", startDate).
		Set("end_date = ?", endDate).
		Where(whereGuestIDEquals, id).
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
	options := modelBase.NewQueryOptions()
	filter := modelBase.NewFilter()

	for field, value := range filters {
		if value != nil {
			applyGuestFilter(filter, field, value)
		}
	}

	options.Filter = filter
	return r.ListWithOptions(ctx, options)
}

// applyGuestFilter applies a single filter based on field name
func applyGuestFilter(filter *modelBase.Filter, field string, value interface{}) {
	switch field {
	case "organization_like":
		applyGuestStringLikeFilter(filter, "organization", value)
	case "expertise_like":
		applyGuestStringLikeFilter(filter, "activity_expertise", value)
	case "active":
		applyActiveDateRangeFilter(filter, value)
	case "current_date":
		applyCustomDateRangeFilter(filter, value)
	case "has_organization":
		applyHasOrganizationFilter(filter, value)
	default:
		filter.Equal(field, value)
	}
}

// applyGuestStringLikeFilter applies LIKE filter for string fields
func applyGuestStringLikeFilter(filter *modelBase.Filter, column string, value interface{}) {
	if strValue, ok := value.(string); ok {
		filter.ILike(column, "%"+strValue+"%")
	}
}

// applyActiveDateRangeFilter applies active date range filter using current time
func applyActiveDateRangeFilter(filter *modelBase.Filter, value interface{}) {
	if boolValue, ok := value.(bool); ok && boolValue {
		now := time.Now()
		filter.Where("start_date IS NULL OR start_date <= ?", modelBase.OpEqual, now)
		filter.Where("end_date IS NULL OR end_date >= ?", modelBase.OpEqual, now)
	}
}

// applyCustomDateRangeFilter applies date range filter for a specific date
func applyCustomDateRangeFilter(filter *modelBase.Filter, value interface{}) {
	if dateValue, ok := value.(time.Time); ok {
		filter.Where("start_date IS NULL OR start_date <= ?", modelBase.OpEqual, dateValue)
		filter.Where("end_date IS NULL OR end_date >= ?", modelBase.OpEqual, dateValue)
	}
}

// applyHasOrganizationFilter applies NULL/NOT NULL filter for organization field
func applyHasOrganizationFilter(filter *modelBase.Filter, value interface{}) {
	if boolValue, ok := value.(bool); ok {
		if boolValue {
			filter.IsNotNull("organization")
		} else {
			filter.IsNull("organization")
		}
	}
}

// ListWithOptions provides a type-safe way to list guests with query options
func (r *GuestRepository) ListWithOptions(ctx context.Context, options *modelBase.QueryOptions) ([]*users.Guest, error) {
	var guests []*users.Guest
	query := r.db.NewSelect().Model(&guests).ModelTableExpr(tableExprGuestsAsGuest)

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
		Where(whereGuestIDEquals, id).
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
		Where(whereGuestIDEquals, id).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find with staff and person",
			Err: err,
		}
	}

	return guest, nil
}
