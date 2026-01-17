package users

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/internal/adapter/repository/postgres/base"
	modelBase "github.com/moto-nrw/project-phoenix/internal/core/domain/base"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/users"
	userPort "github.com/moto-nrw/project-phoenix/internal/core/port/users"
	"github.com/uptrace/bun"
)

// StudentGuardianRepository implements users.StudentGuardianRepository interface
type StudentGuardianRepository struct {
	*base.Repository[*users.StudentGuardian]
	db *bun.DB
}

// NewStudentGuardianRepository creates a new StudentGuardianRepository
func NewStudentGuardianRepository(db *bun.DB) userPort.StudentGuardianRepository {
	return &StudentGuardianRepository{
		Repository: base.NewRepository[*users.StudentGuardian](db, "users.students_guardians", "StudentGuardian"),
		db:         db,
	}
}

// FindByStudentID retrieves relationships by student ID
func (r *StudentGuardianRepository) FindByStudentID(ctx context.Context, studentID int64) ([]*users.StudentGuardian, error) {
	var relationships []*users.StudentGuardian
	err := r.db.NewSelect().
		Model(&relationships).
		ModelTableExpr(`users.students_guardians AS "student_guardian"`).
		Where(`"student_guardian".student_id = ?`, studentID).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by student ID",
			Err: err,
		}
	}

	return relationships, nil
}

// FindByGuardianProfileID retrieves relationships by guardian profile ID
func (r *StudentGuardianRepository) FindByGuardianProfileID(ctx context.Context, guardianProfileID int64) ([]*users.StudentGuardian, error) {
	var relationships []*users.StudentGuardian
	err := r.db.NewSelect().
		Model(&relationships).
		ModelTableExpr(`users.students_guardians AS "student_guardian"`).
		Where(`"student_guardian".guardian_profile_id = ?`, guardianProfileID).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by guardian profile ID",
			Err: err,
		}
	}

	return relationships, nil
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

// FindWithStudent retrieves a student guardian relationship with its associated student
func (r *StudentGuardianRepository) FindWithStudent(ctx context.Context, id int64) (*users.StudentGuardian, error) {
	relationship := new(users.StudentGuardian)
	err := r.db.NewSelect().
		Model(relationship).
		ModelTableExpr(`users.students_guardians AS "student_guardian"`).
		Relation("Student").
		Where(`"student_guardian".id = ?`, id).
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
		ModelTableExpr(`users.students_guardians AS "student_guardian"`).
		Relation("Student").
		Relation("Student.Person").
		Where(`"student_guardian".id = ?`, id).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find with student and person",
			Err: err,
		}
	}

	return relationship, nil
}
