package users

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"

	"github.com/uptrace/bun"

	users "github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/rows"
)

// Error messages and SQL fragments
const (
	errGuardianPhoneNotFound = "guardian phone number not found"
	sqlSetIsPrimary          = "is_primary = ?"
)

// guardianPhoneNumberRepository implements the GuardianPhoneNumberRepository interface.
type guardianPhoneNumberRepository struct {
	runtime Runtime
}

// newRawGuardianPhoneNumberRepository creates a new GuardianPhoneNumberRepository instance
func newRawGuardianPhoneNumberRepository(runtime Runtime) rawGuardianPhoneNumberRepository {
	return &guardianPhoneNumberRepository{runtime: requireRuntime(runtime)}
}

// Create inserts a new phone number into the database
func (r *guardianPhoneNumberRepository) Create(ctx context.Context, phone *users.GuardianPhoneNumber) error {
	if err := phone.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	ensureTenantID(ctx, r.runtime, phone)

	// Get the database connection (or transaction if in context)
	ambientDB1, databaseErr1 := r.runtime.DB(ctx)
	if databaseErr1 != nil {
		return databaseErr1
	}
	db := ambientDB1

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
func (r *guardianPhoneNumberRepository) FindByID(ctx context.Context, id int64) (*users.GuardianPhoneNumber, error) {
	phone := new(users.GuardianPhoneNumber)

	ambientDB2, databaseErr2 := r.runtime.DB(ctx)
	if databaseErr2 != nil {
		return *new(*users.GuardianPhoneNumber), databaseErr2
	}
	err := ambientDB2.NewSelect().
		Model(phone).
		ModelTableExpr(`users.guardian_phone_numbers AS "guardian_phone_number"`).
		Where(`"guardian_phone_number".id = ?`, id).
		Scan(ctx)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, guardianPhoneNotFound{}
		}
		return nil, fmt.Errorf("failed to find guardian phone number: %w", err)
	}

	return phone, nil
}

// FindByGuardianID retrieves all phone numbers for a guardian profile
func (r *guardianPhoneNumberRepository) FindByGuardianID(ctx context.Context, guardianProfileID int64) ([]*users.GuardianPhoneNumber, error) {
	var phones []*users.GuardianPhoneNumber

	ambientDB3, databaseErr3 := r.runtime.DB(ctx)
	if databaseErr3 != nil {
		return *new([]*users.GuardianPhoneNumber), databaseErr3
	}
	err := ambientDB3.NewSelect().
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
func (r *guardianPhoneNumberRepository) FindByGuardianIDs(ctx context.Context, guardianProfileIDs []int64) (map[int64][]*users.GuardianPhoneNumber, error) {
	if len(guardianProfileIDs) == 0 {
		return make(map[int64][]*users.GuardianPhoneNumber), nil
	}

	var phones []*users.GuardianPhoneNumber
	ambientDB4, databaseErr4 := r.runtime.DB(ctx)
	if databaseErr4 != nil {
		return *new(map[int64][]*users.GuardianPhoneNumber), databaseErr4
	}
	err := ambientDB4.NewSelect().
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
func (r *guardianPhoneNumberRepository) Update(ctx context.Context, phone *users.GuardianPhoneNumber) error {
	if err := phone.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	ambientDB5, databaseErr5 := r.runtime.DB(ctx)
	if databaseErr5 != nil {
		return databaseErr5
	}
	result, err := ambientDB5.NewUpdate().
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
		return guardianPhoneNotFound{}
	}

	return nil
}

// Delete removes a phone number
func (r *guardianPhoneNumberRepository) Delete(ctx context.Context, id int64) error {
	ambientDB6, databaseErr6 := r.runtime.DB(ctx)
	if databaseErr6 != nil {
		return databaseErr6
	}
	result, err := ambientDB6.NewDelete().
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
		return guardianPhoneNotFound{}
	}

	return nil
}

// SetPrimary changes both flags atomically inside the caller's unit of work.
func (r *guardianPhoneNumberRepository) SetPrimary(ctx context.Context, id int64, guardianProfileID int64) error {
	return inExistingTransaction(ctx, r.runtime, func(txCtx context.Context) error {
		return r.setPrimaryInTx(txCtx, id, guardianProfileID)
	})
}

// setPrimaryInTx performs the two-step primary flag update within the current transaction.
func (r *guardianPhoneNumberRepository) setPrimaryInTx(ctx context.Context, id int64, guardianProfileID int64) error {
	ambientDB7, databaseErr7 := r.runtime.DB(ctx)
	if databaseErr7 != nil {
		return databaseErr7
	}
	db := ambientDB7

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
		return guardianPhoneNotFound{}
	}

	return nil
}

// UnsetAllPrimary unsets primary flag for all phone numbers of a guardian
func (r *guardianPhoneNumberRepository) UnsetAllPrimary(ctx context.Context, guardianProfileID int64) error {
	ambientDB8, databaseErr8 := r.runtime.DB(ctx)
	if databaseErr8 != nil {
		return databaseErr8
	}
	_, err := ambientDB8.NewUpdate().
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
func (r *guardianPhoneNumberRepository) CountByGuardianID(ctx context.Context, guardianProfileID int64) (int, error) {
	ambientDB9, databaseErr9 := r.runtime.DB(ctx)
	if databaseErr9 != nil {
		return *new(int), databaseErr9
	}
	count, err := ambientDB9.NewSelect().
		Model((*users.GuardianPhoneNumber)(nil)).
		ModelTableExpr(`users.guardian_phone_numbers AS "guardian_phone_number"`).
		Where(`"guardian_phone_number".guardian_profile_id = ?`, guardianProfileID).
		Count(ctx)

	if err != nil {
		return 0, fmt.Errorf("failed to count guardian phone numbers: %w", err)
	}

	return int(count), nil
}

// GetNextPriority returns the next priority value for a guardian's phone numbers
func (r *guardianPhoneNumberRepository) GetNextPriority(ctx context.Context, guardianProfileID int64) (int, error) {
	var maxPriority int

	ambientDB10, databaseErr10 := r.runtime.DB(ctx)
	if databaseErr10 != nil {
		return *new(int), databaseErr10
	}
	err := ambientDB10.NewSelect().
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

// Preserve the store's diagnostic while exposing the public not-found identity.
type guardianPhoneNotFound struct{}

func (guardianPhoneNotFound) Error() string { return errGuardianPhoneNotFound }
func (guardianPhoneNotFound) Is(target error) bool {
	return target == peopledirectory.ErrGuardianPhoneNotFound
}
