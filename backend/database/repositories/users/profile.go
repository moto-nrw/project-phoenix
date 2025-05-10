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
		Where("account_id = ?", accountID).
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
		Set("avatar = ?", avatar).
		Where("id = ?", id).
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
		Set("bio = ?", bio).
		Where("id = ?", id).
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
		Set("settings = ?", settings).
		Where("id = ?", id).
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
	query := r.db.NewSelect().Model(&profiles)

	// Apply filters
	for field, value := range filters {
		if value != nil {
			switch field {
			case "account_id":
				query = query.Where("account_id = ?", value)
			case "has_avatar":
				if boolValue, ok := value.(bool); ok && boolValue {
					query = query.Where("avatar IS NOT NULL AND avatar != ''")
				} else if boolValue, ok := value.(bool); ok && !boolValue {
					query = query.Where("avatar IS NULL OR avatar = ''")
				}
			case "has_bio":
				if boolValue, ok := value.(bool); ok && boolValue {
					query = query.Where("bio IS NOT NULL AND bio != ''")
				} else if boolValue, ok := value.(bool); ok && !boolValue {
					query = query.Where("bio IS NULL OR bio = ''")
				}
			case "bio_like":
				if strValue, ok := value.(string); ok {
					query = query.Where("bio ILIKE ?", "%"+strValue+"%")
				}
			default:
				// Default to exact match for other fields
				query = query.Where("? = ?", bun.Ident(field), value)
			}
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
