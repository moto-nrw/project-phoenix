package users

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	repoBase "github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// Error messages and SQL fragments
const (
	errGuardianPhoneNotFound = "guardian phone number not found"
	sqlSetIsPrimary          = "is_primary = ?"
)

// GuardianPhoneNumberRepository implements the users.GuardianPhoneNumberRepository interface
type GuardianPhoneNumberRepository struct {
	db *bun.DB
}

// NewGuardianPhoneNumberRepository creates a new GuardianPhoneNumberRepository instance
func NewGuardianPhoneNumberRepository(db *bun.DB) users.GuardianPhoneNumberRepository {
	return &GuardianPhoneNumberRepository{db: db}
}

// Create inserts a new phone number into the database
func (r *GuardianPhoneNumberRepository) Create(ctx context.Context, phone *users.GuardianPhoneNumber) error {
	if err := phone.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	repoBase.EnsureTenantID(ctx, phone)

	// Get the database connection (or transaction if in context)
	db := repoBase.GetDB(ctx, r.db)

	_, err := db.NewInsert().
		Model(phone).
		ModelTableExpr(`users.guardian_phone_numbers AS "guardian_phone_number"`).
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("failed to create guardian phone number: %w", err)
	}

	return nil
}

// FindByID retrieves a phone number by its ID
func (r *GuardianPhoneNumberRepository) FindByID(ctx context.Context, id int64) (*users.GuardianPhoneNumber, error) {
	phone := new(users.GuardianPhoneNumber)

	err := repoBase.GetDB(ctx, r.db).NewSelect().
		Model(phone).
		ModelTableExpr(`users.guardian_phone_numbers AS "guardian_phone_number"`).
		Where(`"guardian_phone_number".id = ?`, id).
		Scan(ctx)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New(errGuardianPhoneNotFound)
		}
		return nil, fmt.Errorf("failed to find guardian phone number: %w", err)
	}

	return phone, nil
}

// FindByGuardianID retrieves all phone numbers for a guardian profile
func (r *GuardianPhoneNumberRepository) FindByGuardianID(ctx context.Context, guardianProfileID int64) ([]*users.GuardianPhoneNumber, error) {
	var phones []*users.GuardianPhoneNumber

	err := repoBase.GetDB(ctx, r.db).NewSelect().
		Model(&phones).
		ModelTableExpr(`users.guardian_phone_numbers AS "guardian_phone_number"`).
		Where(`"guardian_phone_number".guardian_profile_id = ?`, guardianProfileID).
		OrderExpr("is_primary DESC, priority ASC, created_at ASC").
		Scan(ctx)

	if err != nil {
		return nil, fmt.Errorf("failed to find guardian phone numbers: %w", err)
	}

	return phones, nil
}

// FindByGuardianIDs retrieves every phone number for the given guardian profile
// ids in a single query, grouped by profile id. Each group is ordered the same
// way FindByGuardianID orders a single guardian's phones (primary first), so
// callers can treat the two interchangeably. Profiles with no phones are absent
// from the map.
func (r *GuardianPhoneNumberRepository) FindByGuardianIDs(ctx context.Context, guardianProfileIDs []int64) (map[int64][]*users.GuardianPhoneNumber, error) {
	if len(guardianProfileIDs) == 0 {
		return make(map[int64][]*users.GuardianPhoneNumber), nil
	}

	var phones []*users.GuardianPhoneNumber
	err := repoBase.GetDB(ctx, r.db).NewSelect().
		Model(&phones).
		ModelTableExpr(`users.guardian_phone_numbers AS "guardian_phone_number"`).
		Where(`"guardian_phone_number".guardian_profile_id IN (?)`, bun.List(guardianProfileIDs)).
		OrderExpr("guardian_profile_id ASC, is_primary DESC, priority ASC, created_at ASC").
		Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to find guardian phone numbers by ids: %w", err)
	}

	result := make(map[int64][]*users.GuardianPhoneNumber, len(guardianProfileIDs))
	for _, phone := range phones {
		result[phone.GuardianProfileID] = append(result[phone.GuardianProfileID], phone)
	}
	return result, nil
}

// Update updates an existing phone number
func (r *GuardianPhoneNumberRepository) Update(ctx context.Context, phone *users.GuardianPhoneNumber) error {
	if err := phone.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	result, err := repoBase.GetDB(ctx, r.db).NewUpdate().
		Model(phone).
		ModelTableExpr(`users.guardian_phone_numbers AS "guardian_phone_number"`).
		Where(`"guardian_phone_number".id = ?`, phone.ID).
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("failed to update guardian phone number: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf(errRowsAffected, err)
	}

	if rowsAffected == 0 {
		return errors.New(errGuardianPhoneNotFound)
	}

	return nil
}

// Delete removes a phone number
func (r *GuardianPhoneNumberRepository) Delete(ctx context.Context, id int64) error {
	result, err := repoBase.GetDB(ctx, r.db).NewDelete().
		Model((*users.GuardianPhoneNumber)(nil)).
		ModelTableExpr(`users.guardian_phone_numbers AS "guardian_phone_number"`).
		Where(`"guardian_phone_number".id = ?`, id).
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("failed to delete guardian phone number: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf(errRowsAffected, err)
	}

	if rowsAffected == 0 {
		return errors.New(errGuardianPhoneNotFound)
	}

	return nil
}

// SetPrimary sets a phone number as primary and unsets others for the guardian.
// Uses the tenant transaction from context (via GetDB) for RLS compliance.
// If no ambient transaction exists, creates one to ensure both updates are atomic —
// without this, a failure on the second update would leave the guardian with no primary number.
func (r *GuardianPhoneNumberRepository) SetPrimary(ctx context.Context, id int64, guardianProfileID int64) error {
	// If already in a transaction (from middleware or service), reuse it.
	// Otherwise start one to keep both updates atomic.
	if _, hasTx := tenant.TransactionFromContext(ctx); hasTx {
		return r.setPrimaryInTx(ctx, id, guardianProfileID)
	}
	return tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		return r.setPrimaryInTx(txCtx, id, guardianProfileID)
	})
}

// setPrimaryInTx performs the two-step primary flag update within the current transaction.
func (r *GuardianPhoneNumberRepository) setPrimaryInTx(ctx context.Context, id int64, guardianProfileID int64) error {
	db := repoBase.GetDB(ctx, r.db)

	// First, unset all primary flags for this guardian
	_, err := db.NewUpdate().
		Model((*users.GuardianPhoneNumber)(nil)).
		ModelTableExpr(`users.guardian_phone_numbers AS "guardian_phone_number"`).
		Set(sqlSetIsPrimary, false).
		Where(`"guardian_phone_number".guardian_profile_id = ?`, guardianProfileID).
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("failed to unset primary flags: %w", err)
	}

	// Then, set the specified phone as primary
	result, err := db.NewUpdate().
		Model((*users.GuardianPhoneNumber)(nil)).
		ModelTableExpr(`users.guardian_phone_numbers AS "guardian_phone_number"`).
		Set(sqlSetIsPrimary, true).
		Where(`"guardian_phone_number".id = ?`, id).
		Where(`"guardian_phone_number".guardian_profile_id = ?`, guardianProfileID).
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("failed to set primary phone number: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf(errRowsAffected, err)
	}

	if rowsAffected == 0 {
		return errors.New(errGuardianPhoneNotFound)
	}

	return nil
}

// UnsetAllPrimary unsets primary flag for all phone numbers of a guardian
func (r *GuardianPhoneNumberRepository) UnsetAllPrimary(ctx context.Context, guardianProfileID int64) error {
	_, err := repoBase.GetDB(ctx, r.db).NewUpdate().
		Model((*users.GuardianPhoneNumber)(nil)).
		ModelTableExpr(`users.guardian_phone_numbers AS "guardian_phone_number"`).
		Set(sqlSetIsPrimary, false).
		Where(`"guardian_phone_number".guardian_profile_id = ?`, guardianProfileID).
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("failed to unset primary flags: %w", err)
	}

	return nil
}

// CountByGuardianID returns the number of phone numbers for a guardian
func (r *GuardianPhoneNumberRepository) CountByGuardianID(ctx context.Context, guardianProfileID int64) (int, error) {
	count, err := repoBase.GetDB(ctx, r.db).NewSelect().
		Model((*users.GuardianPhoneNumber)(nil)).
		ModelTableExpr(`users.guardian_phone_numbers AS "guardian_phone_number"`).
		Where(`"guardian_phone_number".guardian_profile_id = ?`, guardianProfileID).
		Count(ctx)

	if err != nil {
		return 0, fmt.Errorf("failed to count guardian phone numbers: %w", err)
	}

	return count, nil
}

// DeleteByGuardianID removes all phone numbers for a guardian
func (r *GuardianPhoneNumberRepository) DeleteByGuardianID(ctx context.Context, guardianProfileID int64) error {
	_, err := repoBase.GetDB(ctx, r.db).NewDelete().
		Model((*users.GuardianPhoneNumber)(nil)).
		ModelTableExpr(`users.guardian_phone_numbers AS "guardian_phone_number"`).
		Where(`"guardian_phone_number".guardian_profile_id = ?`, guardianProfileID).
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("failed to delete guardian phone numbers: %w", err)
	}

	return nil
}

// GetNextPriority returns the next priority value for a guardian's phone numbers
func (r *GuardianPhoneNumberRepository) GetNextPriority(ctx context.Context, guardianProfileID int64) (int, error) {
	var maxPriority int

	err := repoBase.GetDB(ctx, r.db).NewSelect().
		Model((*users.GuardianPhoneNumber)(nil)).
		ModelTableExpr(`users.guardian_phone_numbers AS "guardian_phone_number"`).
		ColumnExpr("COALESCE(MAX(priority), 0)").
		Where(`"guardian_phone_number".guardian_profile_id = ?`, guardianProfileID).
		Scan(ctx, &maxPriority)

	if err != nil {
		return 1, fmt.Errorf("failed to get max priority: %w", err)
	}

	return maxPriority + 1, nil
}
