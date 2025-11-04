// backend/database/repositories/users/student_guardian.go
package users

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

// StudentGuardianRepository implements users.StudentGuardianRepository interface
type StudentGuardianRepository struct {
	*base.Repository[*users.StudentGuardian]
	db *bun.DB
}

// NewStudentGuardianRepository creates a new StudentGuardianRepository
func NewStudentGuardianRepository(db *bun.DB) users.StudentGuardianRepository {
	return &StudentGuardianRepository{
		Repository: base.NewRepository[*users.StudentGuardian](db, "users.students_guardians", "student_guardian"),
		db:         db,
	}
}

// FindByStudentID retrieves relationships by student ID
func (r *StudentGuardianRepository) FindByStudentID(ctx context.Context, studentID int64) ([]*users.StudentGuardian, error) {
	var relationships []*users.StudentGuardian
	err := r.db.NewSelect().
		Model(&relationships).
		ModelTableExpr(`users.students_guardians AS "student_guardian"`).
		Where("student_id = ?", studentID).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by student ID",
			Err: err,
		}
	}

	return relationships, nil
}

// FindByGuardianID retrieves relationships by guardian ID
func (r *StudentGuardianRepository) FindByGuardianID(ctx context.Context, guardianID int64) ([]*users.StudentGuardian, error) {
	var relationships []*users.StudentGuardian
	err := r.db.NewSelect().
		Model(&relationships).
		Where("guardian_id = ?", guardianID).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by guardian ID",
			Err: err,
		}
	}

	return relationships, nil
}

// FindPrimaryByStudentID retrieves the primary guardian for a student
func (r *StudentGuardianRepository) FindPrimaryByStudentID(ctx context.Context, studentID int64) (*users.StudentGuardian, error) {
	relationship := new(users.StudentGuardian)
	err := r.db.NewSelect().
		Model(relationship).
		Where("student_id = ? AND is_primary = TRUE", studentID).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find primary by student ID",
			Err: err,
		}
	}

	return relationship, nil
}

// FindEmergencyContactsByStudentID retrieves all emergency contacts for a student
func (r *StudentGuardianRepository) FindEmergencyContactsByStudentID(ctx context.Context, studentID int64) ([]*users.StudentGuardian, error) {
	var relationships []*users.StudentGuardian
	err := r.db.NewSelect().
		Model(&relationships).
		Where("student_id = ? AND is_emergency_contact = TRUE", studentID).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find emergency contacts by student ID",
			Err: err,
		}
	}

	return relationships, nil
}

// FindPickupAuthoritiesByStudentID retrieves all guardians who can pickup a student
func (r *StudentGuardianRepository) FindPickupAuthoritiesByStudentID(ctx context.Context, studentID int64) ([]*users.StudentGuardian, error) {
	var relationships []*users.StudentGuardian
	err := r.db.NewSelect().
		Model(&relationships).
		Where("student_id = ? AND can_pickup = TRUE", studentID).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find pickup authorities by student ID",
			Err: err,
		}
	}

	return relationships, nil
}

// FindByRelationshipType retrieves relationships by relationship type
func (r *StudentGuardianRepository) FindByRelationshipType(ctx context.Context, studentID int64, relationshipType string) ([]*users.StudentGuardian, error) {
	var relationships []*users.StudentGuardian
	err := r.db.NewSelect().
		Model(&relationships).
		Where("student_id = ? AND relationship_type = ?", studentID, relationshipType).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by relationship type",
			Err: err,
		}
	}

	return relationships, nil
}

// SetPrimary sets a guardian as the primary guardian for a student
func (r *StudentGuardianRepository) SetPrimary(ctx context.Context, id int64, isPrimary bool) error {
	// Database has a trigger that automatically manages the primary status
	// Just update the current relationship
	_, err := r.db.NewUpdate().
		Model((*users.StudentGuardian)(nil)).
		Set("is_primary = ?", isPrimary).
		Where("id = ?", id).
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "set primary",
			Err: err,
		}
	}

	return nil
}

// SetEmergencyContact sets whether a guardian is an emergency contact
func (r *StudentGuardianRepository) SetEmergencyContact(ctx context.Context, id int64, isEmergencyContact bool) error {
	_, err := r.db.NewUpdate().
		Model((*users.StudentGuardian)(nil)).
		Set("is_emergency_contact = ?", isEmergencyContact).
		Where("id = ?", id).
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "set emergency contact",
			Err: err,
		}
	}

	return nil
}

// SetCanPickup sets whether a guardian can pickup a student
func (r *StudentGuardianRepository) SetCanPickup(ctx context.Context, id int64, canPickup bool) error {
	_, err := r.db.NewUpdate().
		Model((*users.StudentGuardian)(nil)).
		Set("can_pickup = ?", canPickup).
		Where("id = ?", id).
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "set can pickup",
			Err: err,
		}
	}

	return nil
}

// UpdatePermissions updates a guardian's permissions
func (r *StudentGuardianRepository) UpdatePermissions(ctx context.Context, id int64, permissions string) error {
	// Parse the JSON string to ensure it's valid
	var permissionsMap map[string]interface{}
	if err := json.Unmarshal([]byte(permissions), &permissionsMap); err != nil {
		return fmt.Errorf("invalid permissions JSON format: %w", err)
	}

	_, err := r.db.NewUpdate().
		Model((*users.StudentGuardian)(nil)).
		Set("permissions = ?", permissions).
		Where("id = ?", id).
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "update permissions",
			Err: err,
		}
	}

	return nil
}

// Create overrides the base Create method to handle validation
func (r *StudentGuardianRepository) Create(ctx context.Context, relationship *users.StudentGuardian) error {
	if relationship == nil {
		return fmt.Errorf("student guardian relationship cannot be nil")
	}

	// Validate relationship
	if err := relationship.Validate(); err != nil {
		return err
	}

	// Use the base Create method
	return r.Repository.Create(ctx, relationship)
}

// Update overrides the base Update method to handle validation
func (r *StudentGuardianRepository) Update(ctx context.Context, relationship *users.StudentGuardian) error {
	if relationship == nil {
		return fmt.Errorf("student guardian relationship cannot be nil")
	}

	// Validate relationship
	if err := relationship.Validate(); err != nil {
		return err
	}

	// Use the base Update method
	return r.Repository.Update(ctx, relationship)
}

// Legacy method to maintain compatibility with old interface
func (r *StudentGuardianRepository) List(ctx context.Context, filters map[string]interface{}) ([]*users.StudentGuardian, error) {
	// Convert old filter format to new QueryOptions
	options := modelBase.NewQueryOptions()
	filter := modelBase.NewFilter()

	for field, value := range filters {
		if value != nil {
			switch field {
			case "is_primary":
				filter.Equal("is_primary", value)
			case "is_emergency_contact":
				filter.Equal("is_emergency_contact", value)
			case "can_pickup":
				filter.Equal("can_pickup", value)
			case "relationship_type":
				if strValue, ok := value.(string); ok {
					filter.Equal("relationship_type", strValue)
				}
			default:
				// Default to exact match for other fields
				filter.Equal(field, value)
			}
		}
	}

	options.Filter = filter

	return r.ListWithOptions(ctx, options)
}

// ListWithOptions provides a type-safe way to list student guardian relationships with query options
func (r *StudentGuardianRepository) ListWithOptions(ctx context.Context, options *modelBase.QueryOptions) ([]*users.StudentGuardian, error) {
	var relationships []*users.StudentGuardian
	query := r.db.NewSelect().Model(&relationships)

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

	return relationships, nil
}

// FindWithStudent retrieves a student guardian relationship with its associated student
func (r *StudentGuardianRepository) FindWithStudent(ctx context.Context, id int64) (*users.StudentGuardian, error) {
	relationship := new(users.StudentGuardian)
	err := r.db.NewSelect().
		Model(relationship).
		Relation("Student").
		Where("id = ?", id).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find with student",
			Err: err,
		}
	}

	return relationship, nil
}

// FindWithStudentAndPerson retrieves a student guardian relationship with its associated student and person
func (r *StudentGuardianRepository) FindWithStudentAndPerson(ctx context.Context, id int64) (*users.StudentGuardian, error) {
	relationship := new(users.StudentGuardian)
	err := r.db.NewSelect().
		Model(relationship).
		Relation("Student").
		Relation("Student.Person").
		Where("id = ?", id).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find with student and person",
			Err: err,
		}
	}

	return relationship, nil
}

// FindWithGuardian retrieves a student guardian relationship with the guardian details
func (r *StudentGuardianRepository) FindWithGuardian(ctx context.Context, id int64) (*users.StudentGuardian, error) {
	relationship := new(users.StudentGuardian)
	err := r.db.NewSelect().
		Model(relationship).
		Relation("Guardian").
		Where("id = ?", id).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find with guardian",
			Err: err,
		}
	}

	return relationship, nil
}

// FindByStudentIDWithGuardians retrieves all guardian relationships for a student with guardian details
func (r *StudentGuardianRepository) FindByStudentIDWithGuardians(ctx context.Context, studentID int64) ([]*users.StudentGuardian, error) {
	var relationships []*users.StudentGuardian
	err := r.db.NewSelect().
		Model(&relationships).
		Relation("Guardian").
		Where("student_id = ?", studentID).
		Order("is_primary DESC").  // Primary guardian first
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by student ID with guardians",
			Err: err,
		}
	}

	return relationships, nil
}
