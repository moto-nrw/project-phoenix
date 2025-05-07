package users

import (
	"context"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

// ProfileRepository implements users.ProfileRepository
type ProfileRepository struct {
	db *bun.DB
}

// NewProfileRepository creates a new profile repository
func NewProfileRepository(db *bun.DB) users.ProfileRepository {
	return &ProfileRepository{db: db}
}

// Create inserts a new profile into the database
func (r *ProfileRepository) Create(ctx context.Context, profile *users.Profile) error {
	if err := profile.Validate(); err != nil {
		return err
	}

	// Initialize settings if empty
	if profile.Settings == nil {
		profile.Settings = make(map[string]interface{})
	}

	_, err := r.db.NewInsert().Model(profile).Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "create", Err: err}
	}

	return nil
}

// FindByID retrieves a profile by its ID
func (r *ProfileRepository) FindByID(ctx context.Context, id interface{}) (*users.Profile, error) {
	profile := new(users.Profile)
	err := r.db.NewSelect().Model(profile).Where("id = ?", id).Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_id", Err: err}
	}
	return profile, nil
}

// FindByAccountID retrieves a profile by account ID
func (r *ProfileRepository) FindByAccountID(ctx context.Context, accountID int64) (*users.Profile, error) {
	profile := new(users.Profile)
	err := r.db.NewSelect().Model(profile).Where("account_id = ?", accountID).Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_account_id", Err: err}
	}
	return profile, nil
}

// Update updates an existing profile
func (r *ProfileRepository) Update(ctx context.Context, profile *users.Profile) error {
	if err := profile.Validate(); err != nil {
		return err
	}

	_, err := r.db.NewUpdate().Model(profile).WherePK().Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "update", Err: err}
	}
	return nil
}

// Delete removes a profile
func (r *ProfileRepository) Delete(ctx context.Context, id interface{}) error {
	_, err := r.db.NewDelete().Model((*users.Profile)(nil)).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "delete", Err: err}
	}
	return nil
}

// List retrieves profiles matching the filters
func (r *ProfileRepository) List(ctx context.Context, filters map[string]interface{}) ([]*users.Profile, error) {
	var profiles []*users.Profile
	query := r.db.NewSelect().Model(&profiles)

	// Apply filters
	for key, value := range filters {
		query = query.Where("? = ?", bun.Ident(key), value)
	}

	// Execute the query
	err := query.Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "list", Err: err}
	}

	return profiles, nil
}

// UpdateAvatar updates the avatar for a profile
func (r *ProfileRepository) UpdateAvatar(ctx context.Context, id int64, avatar string) error {
	_, err := r.db.NewUpdate().
		Model((*users.Profile)(nil)).
		Set("avatar = ?", avatar).
		Where("id = ?", id).
		Exec(ctx)

	if err != nil {
		return &base.DatabaseError{Op: "update_avatar", Err: err}
	}
	return nil
}

// UpdateBio updates the bio for a profile
func (r *ProfileRepository) UpdateBio(ctx context.Context, id int64, bio string) error {
	_, err := r.db.NewUpdate().
		Model((*users.Profile)(nil)).
		Set("bio = ?", bio).
		Where("id = ?", id).
		Exec(ctx)

	if err != nil {
		return &base.DatabaseError{Op: "update_bio", Err: err}
	}
	return nil
}

// UpdateSettings updates the settings for a profile
func (r *ProfileRepository) UpdateSettings(ctx context.Context, id int64, settings map[string]interface{}) error {
	_, err := r.db.NewUpdate().
		Model((*users.Profile)(nil)).
		Set("settings = ?", settings).
		Where("id = ?", id).
		Exec(ctx)

	if err != nil {
		return &base.DatabaseError{Op: "update_settings", Err: err}
	}
	return nil
}
