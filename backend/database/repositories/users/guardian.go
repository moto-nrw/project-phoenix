package users

import (
	"context"
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

// GuardianRepository is the implementation of the GuardianRepository interface
type GuardianRepository struct {
	db *bun.DB
}

// NewGuardianRepository creates a new instance of GuardianRepository
func NewGuardianRepository(db *bun.DB) *GuardianRepository {
	return &GuardianRepository{db: db}
}

// Create inserts a new guardian into the database
func (r *GuardianRepository) Create(ctx context.Context, guardian *users.Guardian) error {
	if err := guardian.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	_, err := r.db.NewInsert().
		Model(guardian).
		Exec(ctx)

	return err
}

// FindByID retrieves a guardian by their ID
func (r *GuardianRepository) FindByID(ctx context.Context, id interface{}) (*users.Guardian, error) {
	guardian := new(users.Guardian)
	err := r.db.NewSelect().
		Model(guardian).
		ModelTableExpr(`users.guardians AS "guardian"`).
		Where(`"guardian".id = ?`, id).
		Scan(ctx)

	if err != nil {
		return nil, err
	}
	return guardian, nil
}

// FindByEmail retrieves a guardian by their email
func (r *GuardianRepository) FindByEmail(ctx context.Context, email string) (*users.Guardian, error) {
	guardian := new(users.Guardian)
	err := r.db.NewSelect().
		Model(guardian).
		ModelTableExpr(`users.guardians AS "guardian"`).
		Where(`LOWER("guardian".email) = LOWER(?)`, email).
		Scan(ctx)

	if err != nil {
		return nil, err
	}
	return guardian, nil
}

// FindByPhone retrieves a guardian by their phone number
func (r *GuardianRepository) FindByPhone(ctx context.Context, phone string) (*users.Guardian, error) {
	guardian := new(users.Guardian)
	err := r.db.NewSelect().
		Model(guardian).
		ModelTableExpr(`users.guardians AS "guardian"`).
		Where(`"guardian".phone = ?`, phone).
		Scan(ctx)

	if err != nil {
		return nil, err
	}
	return guardian, nil
}

// FindByAccountID retrieves a guardian by their account ID
func (r *GuardianRepository) FindByAccountID(ctx context.Context, accountID int64) (*users.Guardian, error) {
	guardian := new(users.Guardian)
	err := r.db.NewSelect().
		Model(guardian).
		ModelTableExpr(`users.guardians AS "guardian"`).
		Where(`"guardian".account_id = ?`, accountID).
		Scan(ctx)

	if err != nil {
		return nil, err
	}
	return guardian, nil
}

// FindByStudentID retrieves all guardians for a student
func (r *GuardianRepository) FindByStudentID(ctx context.Context, studentID int64) ([]*users.Guardian, error) {
	var guardians []*users.Guardian
	err := r.db.NewSelect().
		Model(&guardians).
		ModelTableExpr(`users.guardians AS "guardian"`).
		Join(`INNER JOIN users.students_guardians AS sg ON sg.guardian_id = "guardian".id`).
		Where(`sg.student_id = ?`, studentID).
		Where(`"guardian".active = ?`, true).
		Order(`sg.is_primary DESC`).  // Primary guardian first
		Order(`"guardian".last_name`).
		Scan(ctx)

	return guardians, err
}

// Update updates an existing guardian
func (r *GuardianRepository) Update(ctx context.Context, guardian *users.Guardian) error {
	if err := guardian.Validate(); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	result, err := r.db.NewUpdate().
		Model(guardian).
		WherePK().
		Exec(ctx)

	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return fmt.Errorf("guardian not found")
	}

	return nil
}

// Delete removes a guardian
func (r *GuardianRepository) Delete(ctx context.Context, id interface{}) error {
	result, err := r.db.NewDelete().
		Model((*users.Guardian)(nil)).
		Where("id = ?", id).
		Exec(ctx)

	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return fmt.Errorf("guardian not found")
	}

	return nil
}

// List retrieves guardians matching the filters
func (r *GuardianRepository) List(ctx context.Context, filters map[string]interface{}) ([]*users.Guardian, error) {
	var guardians []*users.Guardian
	query := r.db.NewSelect().
		Model(&guardians).
		ModelTableExpr(`users.guardians AS "guardian"`)

	// Apply filters
	for key, value := range filters {
		switch key {
		case "active":
			query = query.Where(`"guardian".active = ?`, value)
		case "email":
			query = query.Where(`LOWER("guardian".email) = LOWER(?)`, value)
		case "phone":
			query = query.Where(`"guardian".phone = ?`, value)
		case "country":
			query = query.Where(`"guardian".country = ?`, value)
		case "is_emergency_contact":
			query = query.Where(`"guardian".is_emergency_contact = ?`, value)
		}
	}

	err := query.Order(`"guardian".last_name`, `"guardian".first_name`).Scan(ctx)
	return guardians, err
}

// ListWithOptions retrieves guardians with query options
func (r *GuardianRepository) ListWithOptions(ctx context.Context, options *base.QueryOptions) ([]*users.Guardian, error) {
	var guardians []*users.Guardian
	query := r.db.NewSelect().
		Model(&guardians).
		ModelTableExpr(`users.guardians AS "guardian"`)

	// Apply query options (filters, sorting, pagination)
	if options != nil {
		// Set table alias for filter conditions
		if options.Filter != nil {
			options.Filter.WithTableAlias("guardian")
		}
		query = options.ApplyToQuery(query)
	} else {
		query = query.Order(`"guardian".last_name`, `"guardian".first_name`)
	}

	err := query.Scan(ctx)
	return guardians, err
}

// CountWithOptions counts guardians matching the query options
func (r *GuardianRepository) CountWithOptions(ctx context.Context, options *base.QueryOptions) (int, error) {
	query := r.db.NewSelect().
		Model((*users.Guardian)(nil)).
		ModelTableExpr(`users.guardians AS "guardian"`)

	// Apply filters if present
	if options != nil && options.Filter != nil {
		options.Filter.WithTableAlias("guardian")
		query = options.Filter.ApplyToQuery(query)
	}

	count, err := query.Count(ctx)
	return count, err
}

// Search searches for guardians by name, email, or phone
func (r *GuardianRepository) Search(ctx context.Context, query string, limit int) ([]*users.Guardian, error) {
	var guardians []*users.Guardian
	searchPattern := "%" + strings.ToLower(query) + "%"

	err := r.db.NewSelect().
		Model(&guardians).
		ModelTableExpr(`users.guardians AS "guardian"`).
		Where(`LOWER("guardian".first_name) LIKE ?`, searchPattern).
		WhereOr(`LOWER("guardian".last_name) LIKE ?`, searchPattern).
		WhereOr(`LOWER("guardian".email) LIKE ?`, searchPattern).
		WhereOr(`"guardian".phone LIKE ?`, searchPattern).
		Where(`"guardian".active = ?`, true).
		Order(`"guardian".last_name`, `"guardian".first_name`).
		Limit(limit).
		Scan(ctx)

	return guardians, err
}

// LinkToAccount associates a guardian with an account
func (r *GuardianRepository) LinkToAccount(ctx context.Context, guardianID int64, accountID int64) error {
	result, err := r.db.NewUpdate().
		Model((*users.Guardian)(nil)).
		Set("account_id = ?", accountID).
		Where("id = ?", guardianID).
		Exec(ctx)

	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return fmt.Errorf("guardian not found")
	}

	return nil
}

// UnlinkFromAccount removes account association from a guardian
func (r *GuardianRepository) UnlinkFromAccount(ctx context.Context, guardianID int64) error {
	result, err := r.db.NewUpdate().
		Model((*users.Guardian)(nil)).
		Set("account_id = NULL").
		Where("id = ?", guardianID).
		Exec(ctx)

	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return fmt.Errorf("guardian not found")
	}

	return nil
}

// FindActive retrieves all active guardians
func (r *GuardianRepository) FindActive(ctx context.Context) ([]*users.Guardian, error) {
	var guardians []*users.Guardian
	err := r.db.NewSelect().
		Model(&guardians).
		ModelTableExpr(`users.guardians AS "guardian"`).
		Where(`"guardian".active = ?`, true).
		Order(`"guardian".last_name`, `"guardian".first_name`).
		Scan(ctx)

	return guardians, err
}
