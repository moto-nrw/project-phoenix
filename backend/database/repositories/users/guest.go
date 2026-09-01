// backend/database/repositories/users/guest.go
package users

import (
	"context"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
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
func NewGuestRepository(db *bun.DB) users.GuestRepository {
	repo := base.NewRepository[*users.Guest](db, tableUsersGuests, "Guest")
	repo.TenantScoped = true
	return &GuestRepository{
		Repository: repo,
		db:         db,
	}
}

// FindByStaffID retrieves a guest by their staff ID
func (r *GuestRepository) FindByStaffID(ctx context.Context, staffID int64) (*users.Guest, error) {
	guest := new(users.Guest)
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(guest).
		ModelTableExpr(tableExprGuestsAsGuest).
		Where("staff_id = ?", staffID)

	query = base.WithTenantFilter(ctx, query, "guest")

	err := query.Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by staff ID",
			Err: base.TranslateNotFound(err),
		}
	}

	return guest, nil
}

// FindActive retrieves currently active guests
func (r *GuestRepository) FindActive(ctx context.Context) ([]*users.Guest, error) {
	var guests []*users.Guest
	today := timezone.TodayDate()

	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&guests).
		ModelTableExpr(tableExprGuestsAsGuest).
		Where("(start_date IS NULL OR start_date <= ?) AND (end_date IS NULL OR end_date >= ?)", today, today)

	query = base.WithTenantFilter(ctx, query, "guest")

	err := query.Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find active",
			Err: base.TranslateNotFound(err),
		}
	}

	return guests, nil
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
		today := timezone.TodayDate()
		filter.Where("start_date IS NULL OR start_date <= ?", modelBase.OpEqual, today)
		filter.Where("end_date IS NULL OR end_date >= ?", modelBase.OpEqual, today)
	}
}

// applyCustomDateRangeFilter applies date range filter for a specific date
func applyCustomDateRangeFilter(filter *modelBase.Filter, value interface{}) {
	if dateValue, ok := value.(timezone.Date); ok {
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

// FindWithStaffAndPerson retrieves a guest with their associated staff and person data
func (r *GuestRepository) FindWithStaffAndPerson(ctx context.Context, id int64) (*users.Guest, error) {
	guest := new(users.Guest)
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(guest).
		ModelTableExpr(tableExprGuestsAsGuest).
		Relation("Staff").
		Relation("Staff.Person").
		Where(`"guest".id = ?`, id)

	query = base.WithTenantFilter(ctx, query, "guest")

	err := query.Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find with staff and person",
			Err: base.TranslateNotFound(err),
		}
	}

	return guest, nil
}
