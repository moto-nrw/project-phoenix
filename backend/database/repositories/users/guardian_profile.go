package users

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

// Error messages (S1192 - avoid duplicate string literals)
const (
	errGuardianProfileNotFound = "guardian profile not found"
	errRowsAffected            = "failed to get rows affected: %w"
)

// GuardianProfileRepository implements the users.GuardianProfileRepository interface
type GuardianProfileRepository struct {
	db *bun.DB
}

// NewGuardianProfileRepository creates a new GuardianProfileRepository instance
func NewGuardianProfileRepository(db *bun.DB) users.GuardianProfileRepository {
	return &GuardianProfileRepository{db: db}
}

// Create inserts a new guardian profile into the database
func (r *GuardianProfileRepository) Create(ctx context.Context, profile *users.GuardianProfile) error {
	if err := profile.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	_, err := r.db.NewInsert().
		Model(profile).
		ModelTableExpr(`users.guardian_profiles AS "guardian_profile"`).
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("failed to create guardian profile: %w", err)
	}

	return nil
}

// FindByID retrieves a guardian profile by their ID
func (r *GuardianProfileRepository) FindByID(ctx context.Context, id int64) (*users.GuardianProfile, error) {
	profile := new(users.GuardianProfile)

	err := r.db.NewSelect().
		Model(profile).
		ModelTableExpr(`users.guardian_profiles AS "guardian_profile"`).
		Where(`"guardian_profile".id = ?`, id).
		Scan(ctx)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New(errGuardianProfileNotFound)
		}
		return nil, fmt.Errorf("failed to find guardian profile: %w", err)
	}

	return profile, nil
}

// FindByEmail retrieves a guardian profile by their email address
func (r *GuardianProfileRepository) FindByEmail(ctx context.Context, email string) (*users.GuardianProfile, error) {
	profile := new(users.GuardianProfile)

	err := r.db.NewSelect().
		Model(profile).
		ModelTableExpr(`users.guardian_profiles AS "guardian_profile"`).
		Where(`LOWER("guardian_profile".email) = LOWER(?)`, email).
		Scan(ctx)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New(errGuardianProfileNotFound)
		}
		return nil, fmt.Errorf("failed to find guardian profile by email: %w", err)
	}

	return profile, nil
}

// FindByAccountID retrieves a guardian profile by their account ID
func (r *GuardianProfileRepository) FindByAccountID(ctx context.Context, accountID int64) (*users.GuardianProfile, error) {
	profile := new(users.GuardianProfile)

	err := r.db.NewSelect().
		Model(profile).
		ModelTableExpr(`users.guardian_profiles AS "guardian_profile"`).
		Where(`"guardian_profile".account_id = ?`, accountID).
		Scan(ctx)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New(errGuardianProfileNotFound)
		}
		return nil, fmt.Errorf("failed to find guardian profile by account ID: %w", err)
	}

	return profile, nil
}

// FindWithoutAccount retrieves guardian profiles without portal accounts
func (r *GuardianProfileRepository) FindWithoutAccount(ctx context.Context) ([]*users.GuardianProfile, error) {
	var profiles []*users.GuardianProfile

	err := r.db.NewSelect().
		Model(&profiles).
		ModelTableExpr(`users.guardian_profiles AS "guardian_profile"`).
		Where(`"guardian_profile".account_id IS NULL`).
		Where(`"guardian_profile".has_account = ?`, false).
		Order(`last_name ASC`, `first_name ASC`).
		Scan(ctx)

	if err != nil {
		return nil, fmt.Errorf("failed to find guardians without account: %w", err)
	}

	return profiles, nil
}

// FindInvitable retrieves guardians who can be invited (has email, no account)
func (r *GuardianProfileRepository) FindInvitable(ctx context.Context) ([]*users.GuardianProfile, error) {
	var profiles []*users.GuardianProfile

	err := r.db.NewSelect().
		Model(&profiles).
		ModelTableExpr(`users.guardian_profiles AS "guardian_profile"`).
		Where(`"guardian_profile".email IS NOT NULL`).
		Where(`"guardian_profile".email != ''`).
		Where(`"guardian_profile".account_id IS NULL`).
		Where(`"guardian_profile".has_account = ?`, false).
		Order(`last_name ASC`, `first_name ASC`).
		Scan(ctx)

	if err != nil {
		return nil, fmt.Errorf("failed to find invitable guardians: %w", err)
	}

	return profiles, nil
}

// ListWithOptions retrieves guardian profiles with pagination and filters
func (r *GuardianProfileRepository) ListWithOptions(ctx context.Context, options *base.QueryOptions) ([]*users.GuardianProfile, error) {
	var profiles []*users.GuardianProfile

	query := r.db.NewSelect().
		Model(&profiles).
		ModelTableExpr(`users.guardian_profiles AS "guardian_profile"`)

	// Apply query options with table alias
	if options != nil {
		if options.Filter != nil {
			options.Filter.WithTableAlias("guardian_profile")
		}
		query = options.ApplyToQuery(query)
	}

	// Default ordering
	query = query.Order(`last_name ASC`, `first_name ASC`)

	err := query.Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list guardian profiles: %w", err)
	}

	return profiles, nil
}

// Count returns the total number of guardian profiles
func (r *GuardianProfileRepository) Count(ctx context.Context) (int, error) {
	count, err := r.db.NewSelect().
		Model((*users.GuardianProfile)(nil)).
		ModelTableExpr(`users.guardian_profiles AS "guardian_profile"`).
		Count(ctx)

	if err != nil {
		return 0, fmt.Errorf("failed to count guardian profiles: %w", err)
	}

	return count, nil
}

// Update updates an existing guardian profile
func (r *GuardianProfileRepository) Update(ctx context.Context, profile *users.GuardianProfile) error {
	if err := profile.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	result, err := r.db.NewUpdate().
		Model(profile).
		ModelTableExpr(`users.guardian_profiles AS "guardian_profile"`).
		Where(`"guardian_profile".id = ?`, profile.ID).
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("failed to update guardian profile: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf(errRowsAffected, err)
	}

	if rowsAffected == 0 {
		return errors.New(errGuardianProfileNotFound)
	}

	return nil
}

// Delete removes a guardian profile
func (r *GuardianProfileRepository) Delete(ctx context.Context, id int64) error {
	result, err := r.db.NewDelete().
		Model((*users.GuardianProfile)(nil)).
		ModelTableExpr(`users.guardian_profiles AS "guardian_profile"`).
		Where(`"guardian_profile".id = ?`, id).
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("failed to delete guardian profile: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf(errRowsAffected, err)
	}

	if rowsAffected == 0 {
		return errors.New(errGuardianProfileNotFound)
	}

	return nil
}

// LinkAccount links a guardian profile to a parent account
func (r *GuardianProfileRepository) LinkAccount(ctx context.Context, profileID int64, accountID int64) error {
	result, err := r.db.NewUpdate().
		Model((*users.GuardianProfile)(nil)).
		ModelTableExpr(`users.guardian_profiles AS "guardian_profile"`).
		Set("account_id = ?", accountID).
		Set("has_account = ?", true).
		Where(`"guardian_profile".id = ?`, profileID).
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("failed to link account: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf(errRowsAffected, err)
	}

	if rowsAffected == 0 {
		return errors.New(errGuardianProfileNotFound)
	}

	return nil
}

// UnlinkAccount unlinks a guardian profile from their account
func (r *GuardianProfileRepository) UnlinkAccount(ctx context.Context, profileID int64) error {
	result, err := r.db.NewUpdate().
		Model((*users.GuardianProfile)(nil)).
		ModelTableExpr(`users.guardian_profiles AS "guardian_profile"`).
		Set("account_id = NULL").
		Set("has_account = ?", false).
		Where(`"guardian_profile".id = ?`, profileID).
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("failed to unlink account: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf(errRowsAffected, err)
	}

	if rowsAffected == 0 {
		return errors.New(errGuardianProfileNotFound)
	}

	return nil
}

// GetStudentCount returns the number of students for a guardian
func (r *GuardianProfileRepository) GetStudentCount(ctx context.Context, profileID int64) (int, error) {
	count, err := r.db.NewSelect().
		Model((*users.StudentGuardian)(nil)).
		ModelTableExpr(`users.students_guardians AS "student_guardian"`).
		Where(`"student_guardian".guardian_profile_id = ?`, profileID).
		Count(ctx)

	if err != nil {
		return 0, fmt.Errorf("failed to count students: %w", err)
	}

	return count, nil
}
