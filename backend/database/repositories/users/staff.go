// backend/database/repositories/users/staff.go
package users

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

// StaffRepository implements users.StaffRepository interface
type StaffRepository struct {
	*base.Repository[*users.Staff]
	db *bun.DB
}

// NewStaffRepository creates a new StaffRepository
func NewStaffRepository(db *bun.DB) users.StaffRepository {
	return &StaffRepository{
		Repository: base.NewRepository[*users.Staff](db, "users.staff", "Staff"),
		db:         db,
	}
}

// FindByPersonID retrieves a staff member by their person ID
func (r *StaffRepository) FindByPersonID(ctx context.Context, personID int64) (*users.Staff, error) {
	staff := new(users.Staff)
	err := r.db.NewSelect().
		Model(staff).
		ModelTableExpr(`users.staff AS "staff"`).
		Where(`"staff".person_id = ?`, personID).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by person ID",
			Err: err,
		}
	}

	return staff, nil
}

// Create overrides the base Create method to handle validation
func (r *StaffRepository) Create(ctx context.Context, staff *users.Staff) error {
	if staff == nil {
		return fmt.Errorf("staff cannot be nil")
	}

	// Validate staff
	if err := staff.Validate(); err != nil {
		return err
	}

	// Use the base Create method
	return r.Repository.Create(ctx, staff)
}

// Update overrides the base Update method to handle validation
func (r *StaffRepository) Update(ctx context.Context, staff *users.Staff) error {
	if staff == nil {
		return fmt.Errorf("staff cannot be nil")
	}

	// Validate staff
	if err := staff.Validate(); err != nil {
		return err
	}

	// Use the base Update method
	return r.Repository.Update(ctx, staff)
}

// Legacy method to maintain compatibility with old interface
func (r *StaffRepository) List(ctx context.Context, filters map[string]interface{}) ([]*users.Staff, error) {
	// Convert old filter format to new QueryOptions
	options := modelBase.NewQueryOptions()
	filter := modelBase.NewFilter()

	for field, value := range filters {
		if value != nil {
			filter.Equal(field, value)
		}
	}

	options.Filter = filter

	return r.ListWithOptions(ctx, options)
}

// ListWithOptions provides a type-safe way to list staff with query options
func (r *StaffRepository) ListWithOptions(ctx context.Context, options *modelBase.QueryOptions) ([]*users.Staff, error) {
	var staffMembers []*users.Staff
	query := r.db.NewSelect().
		Model(&staffMembers).
		ModelTableExpr(`users.staff AS "staff"`)

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

	return staffMembers, nil
}

// FindWithPerson retrieves a staff member with their associated person data
func (r *StaffRepository) FindWithPerson(ctx context.Context, id int64) (*users.Staff, error) {
	// First get the staff member
	staff := new(users.Staff)
	err := r.db.NewSelect().
		Model(staff).
		ModelTableExpr(`users.staff AS "staff"`).
		Where(`"staff".id = ?`, id).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find with person - staff",
			Err: err,
		}
	}

	// Then get the person if exists
	if staff.PersonID > 0 {
		person := new(users.Person)
		personErr := r.db.NewSelect().
			Model(person).
			ModelTableExpr(`users.persons AS "person"`).
			Where(`"person".id = ?`, staff.PersonID).
			Scan(ctx)

		if personErr == nil {
			staff.Person = person
		} else if !errors.Is(personErr, sql.ErrNoRows) {
			// Only ignore "not found" errors - propagate all other DB errors
			return nil, &modelBase.DatabaseError{
				Op:  "find with person - load person",
				Err: personErr,
			}
		}
		// Person not found is acceptable - staff.Person remains nil
	}

	return staff, nil
}

// AddNotes adds notes to a staff member's existing notes
func (r *StaffRepository) AddNotes(ctx context.Context, id int64, notes string) error {
	// First, retrieve the current staff record to get existing notes
	staff, err := r.FindByID(ctx, id)
	if err != nil {
		return err
	}

	// Add the new notes to existing notes
	staff.AddNotes(notes)

	// Update the staff record
	_, err = r.db.NewUpdate().
		Model(staff).
		ModelTableExpr(`users.staff AS "staff"`).
		Column(`"staff".staff_notes`).
		WherePK().
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "add notes",
			Err: err,
		}
	}

	return nil
}
