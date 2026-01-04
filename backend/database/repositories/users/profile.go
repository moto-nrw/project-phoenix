package users

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

// ProfileRepository implements users.ProfileRepository interface
type ProfileRepository struct {
	*base.Repository[*users.Profile]
	db *bun.DB
}

// NewProfileRepository creates a new ProfileRepository
func NewProfileRepository(db *bun.DB) users.ProfileRepository {
	return &ProfileRepository{
		Repository: base.NewRepository[*users.Profile](db, "users.profiles", "Profile"),
		db:         db,
	}
}

// FindByAccountID retrieves a profile by account ID
func (r *ProfileRepository) FindByAccountID(ctx context.Context, accountID int64) (*users.Profile, error) {
	profile := new(users.Profile)
	err := r.db.NewSelect().
		Model(profile).
		ModelTableExpr(`users.profiles AS "profile"`).
		Where(`"profile".account_id = ?`, accountID).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by account ID",
			Err: err,
		}
	}

	return profile, nil
}

// UpdateAvatar updates a profile's avatar
func (r *ProfileRepository) UpdateAvatar(ctx context.Context, id int64, avatar string) error {
	_, err := r.db.NewUpdate().
		Model((*users.Profile)(nil)).
		ModelTableExpr(`users.profiles AS "profile"`).
		Set("avatar = ?", avatar).
		Where(`"profile".id = ?`, id).
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "update avatar",
			Err: err,
		}
	}

	return nil
}

// UpdateBio updates a profile's bio
func (r *ProfileRepository) UpdateBio(ctx context.Context, id int64, bio string) error {
	_, err := r.db.NewUpdate().
		Model((*users.Profile)(nil)).
		ModelTableExpr(`users.profiles AS "profile"`).
		Set("bio = ?", bio).
		Where(`"profile".id = ?`, id).
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "update bio",
			Err: err,
		}
	}

	return nil
}

// UpdateSettings updates a profile's settings
func (r *ProfileRepository) UpdateSettings(ctx context.Context, id int64, settings string) error {
	_, err := r.db.NewUpdate().
		Model((*users.Profile)(nil)).
		ModelTableExpr(`users.profiles AS "profile"`).
		Set("settings = ?", settings).
		Where(`"profile".id = ?`, id).
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "update settings",
			Err: err,
		}
	}

	return nil
}

// Create overrides the base Create method to handle validation
func (r *ProfileRepository) Create(ctx context.Context, profile *users.Profile) error {
	if profile == nil {
		return fmt.Errorf("profile cannot be nil")
	}

	// Validate profile
	if err := profile.Validate(); err != nil {
		return err
	}

	// Use the base Create method
	return r.Repository.Create(ctx, profile)
}

// Update overrides the base Update method to handle validation
func (r *ProfileRepository) Update(ctx context.Context, profile *users.Profile) error {
	if profile == nil {
		return fmt.Errorf("profile cannot be nil")
	}

	// Validate profile
	if err := profile.Validate(); err != nil {
		return err
	}

	// Use the base Update method
	return r.Repository.Update(ctx, profile)
}

// Delete overrides the base Delete method
func (r *ProfileRepository) Delete(ctx context.Context, id interface{}) error {
	return r.Repository.Delete(ctx, id)
}

// List retrieves profiles matching the provided filters
func (r *ProfileRepository) List(ctx context.Context, filters map[string]interface{}) ([]*users.Profile, error) {
	var profiles []*users.Profile
	query := r.db.NewSelect().
		Model(&profiles).
		ModelTableExpr(`users.profiles AS "profile"`)

	// Apply filters
	for field, value := range filters {
		if value != nil {
			query = applyProfileFilter(query, field, value)
		}
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "list",
			Err: err,
		}
	}

	return profiles, nil
}

// applyProfileFilter applies a single filter to the query based on field name
func applyProfileFilter(query *bun.SelectQuery, field string, value interface{}) *bun.SelectQuery {
	switch field {
	case "account_id":
		return query.Where(`"profile".account_id = ?`, value)
	case "has_avatar":
		return applyNonEmptyStringFilter(query, `"profile".avatar`, value)
	case "has_bio":
		return applyNonEmptyStringFilter(query, `"profile".bio`, value)
	case "bio_like":
		return applyProfileStringLikeFilter(query, `"profile".bio`, value)
	default:
		return query.Where("? = ?", bun.Ident(field), value)
	}
}

// applyProfileStringLikeFilter applies LIKE filter for string fields
func applyProfileStringLikeFilter(query *bun.SelectQuery, column string, value interface{}) *bun.SelectQuery {
	if strValue, ok := value.(string); ok {
		return query.Where(column+" ILIKE ?", "%"+strValue+"%")
	}
	return query
}

// applyNonEmptyStringFilter applies filter for non-empty or empty string fields
func applyNonEmptyStringFilter(query *bun.SelectQuery, column string, value interface{}) *bun.SelectQuery {
	if boolValue, ok := value.(bool); ok {
		if boolValue {
			return query.Where(column + " IS NOT NULL AND " + column + " != ''")
		}
		return query.Where(column + " IS NULL OR " + column + " = ''")
	}
	return query
}
