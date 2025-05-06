package users

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// Profile represents a user profile in the system
type Profile struct {
	base.Model
	AccountID int64                  `bun:"account_id,notnull" json:"account_id"`
	Avatar    string                 `bun:"avatar" json:"avatar,omitempty"`
	Bio       string                 `bun:"bio" json:"bio,omitempty"`
	Settings  map[string]interface{} `bun:"settings,type:jsonb,default:'{}'" json:"settings,omitempty"` // Use map for JSON data
	Account   *auth.Account          `bun:"rel:belongs-to,join:account_id=id" json:"account,omitempty"`
}

// TableName returns the table name for the Profile model
func (p *Profile) TableName() string {
	return "users.profiles"
}

// GetID returns the profile ID
func (p *Profile) GetID() interface{} {
	return p.ID
}

// GetCreatedAt returns the creation timestamp
func (p *Profile) GetCreatedAt() time.Time {
	return p.CreatedAt
}

// GetUpdatedAt returns the last update timestamp
func (p *Profile) GetUpdatedAt() time.Time {
	return p.UpdatedAt
}

// Validate validates the profile fields
func (p *Profile) Validate() error {
	if p.AccountID <= 0 {
		return errors.New("account ID is required")
	}
	return nil
}

// ProfileRepository defines operations for working with profiles
type ProfileRepository interface {
	base.Repository[*Profile]
	FindByAccountID(ctx context.Context, accountID int64) (*Profile, error)
	UpdateAvatar(ctx context.Context, id int64, avatar string) error
	UpdateBio(ctx context.Context, id int64, bio string) error
	UpdateSettings(ctx context.Context, id int64, settings map[string]interface{}) error
}

// DefaultProfileRepository is the default implementation of ProfileRepository
type DefaultProfileRepository struct {
	db *bun.DB
}

// NewProfileRepository creates a new profile repository
func NewProfileRepository(db *bun.DB) ProfileRepository {
	return &DefaultProfileRepository{db: db}
}

// Create inserts a new profile into the database
func (r *DefaultProfileRepository) Create(ctx context.Context, profile *Profile) error {
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
func (r *DefaultProfileRepository) FindByID(ctx context.Context, id interface{}) (*Profile, error) {
	profile := new(Profile)
	err := r.db.NewSelect().Model(profile).Where("id = ?", id).Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_id", Err: err}
	}
	return profile, nil
}

// FindByAccountID retrieves a profile by account ID
func (r *DefaultProfileRepository) FindByAccountID(ctx context.Context, accountID int64) (*Profile, error) {
	profile := new(Profile)
	err := r.db.NewSelect().Model(profile).Where("account_id = ?", accountID).Scan(ctx)
	if err != nil {
		return nil, &base.DatabaseError{Op: "find_by_account_id", Err: err}
	}
	return profile, nil
}

// Update updates an existing profile
func (r *DefaultProfileRepository) Update(ctx context.Context, profile *Profile) error {
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
func (r *DefaultProfileRepository) Delete(ctx context.Context, id interface{}) error {
	_, err := r.db.NewDelete().Model((*Profile)(nil)).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return &base.DatabaseError{Op: "delete", Err: err}
	}
	return nil
}

// List retrieves profiles matching the filters
func (r *DefaultProfileRepository) List(ctx context.Context, filters map[string]interface{}) ([]*Profile, error) {
	var profiles []*Profile
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
func (r *DefaultProfileRepository) UpdateAvatar(ctx context.Context, id int64, avatar string) error {
	_, err := r.db.NewUpdate().
		Model((*Profile)(nil)).
		Set("avatar = ?", avatar).
		Where("id = ?", id).
		Exec(ctx)

	if err != nil {
		return &base.DatabaseError{Op: "update_avatar", Err: err}
	}
	return nil
}

// UpdateBio updates the bio for a profile
func (r *DefaultProfileRepository) UpdateBio(ctx context.Context, id int64, bio string) error {
	_, err := r.db.NewUpdate().
		Model((*Profile)(nil)).
		Set("bio = ?", bio).
		Where("id = ?", id).
		Exec(ctx)

	if err != nil {
		return &base.DatabaseError{Op: "update_bio", Err: err}
	}
	return nil
}

// UpdateSettings updates the settings for a profile
func (r *DefaultProfileRepository) UpdateSettings(ctx context.Context, id int64, settings map[string]interface{}) error {
	_, err := r.db.NewUpdate().
		Model((*Profile)(nil)).
		Set("settings = ?", settings).
		Where("id = ?", id).
		Exec(ctx)

	if err != nil {
		return &base.DatabaseError{Op: "update_settings", Err: err}
	}
	return nil
}
