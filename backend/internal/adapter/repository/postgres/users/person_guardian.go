// backend/database/repositories/users/person_guardian.go
package users

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/internal/adapter/repository/postgres/base"
	modelBase "github.com/moto-nrw/project-phoenix/internal/core/domain/base"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/users"
	"github.com/uptrace/bun"
)

// Table and query constants (S1192 - avoid duplicate string literals)
const (
	tableUsersPersonsGuardians    = "users.persons_guardians"
	tableExprPersonsGuardiansAsPG = `users.persons_guardians AS "person_guardian"`
	wherePersonGuardianIDEquals   = `"person_guardian".id = ?`
	wherePersonGuardianIDEqualsPK = "id = ?"
)

// PersonGuardianRepository implements users.PersonGuardianRepository interface
type PersonGuardianRepository struct {
	*base.Repository[*users.PersonGuardian]
	db *bun.DB
}

// NewPersonGuardianRepository creates a new PersonGuardianRepository
func NewPersonGuardianRepository(db *bun.DB) users.PersonGuardianRepository {
	return &PersonGuardianRepository{
		Repository: base.NewRepository[*users.PersonGuardian](db, tableUsersPersonsGuardians, "PersonGuardian"),
		db:         db,
	}
}

// FindByPersonID retrieves relationships by person ID
func (r *PersonGuardianRepository) FindByPersonID(ctx context.Context, personID int64) ([]*users.PersonGuardian, error) {
	var relationships []*users.PersonGuardian
	err := r.db.NewSelect().
		Model(&relationships).
		ModelTableExpr(tableExprPersonsGuardiansAsPG).
		Where(`"person_guardian".person_id = ?`, personID).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by person ID",
			Err: err,
		}
	}

	return relationships, nil
}

// FindByGuardianID retrieves relationships by guardian account ID
func (r *PersonGuardianRepository) FindByGuardianID(ctx context.Context, guardianID int64) ([]*users.PersonGuardian, error) {
	var relationships []*users.PersonGuardian
	err := r.db.NewSelect().
		Model(&relationships).
		ModelTableExpr(tableExprPersonsGuardiansAsPG).
		Where(`"person_guardian".guardian_account_id = ?`, guardianID).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by guardian ID",
			Err: err,
		}
	}

	return relationships, nil
}

// FindPrimaryByPersonID retrieves the primary guardian for a person
func (r *PersonGuardianRepository) FindPrimaryByPersonID(ctx context.Context, personID int64) (*users.PersonGuardian, error) {
	relationship := new(users.PersonGuardian)
	err := r.db.NewSelect().
		Model(relationship).
		ModelTableExpr(tableExprPersonsGuardiansAsPG).
		Where(`"person_guardian".person_id = ? AND "person_guardian".is_primary = TRUE`, personID).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find primary by person ID",
			Err: err,
		}
	}

	return relationship, nil
}

// FindByRelationshipType retrieves relationships by relationship type
func (r *PersonGuardianRepository) FindByRelationshipType(ctx context.Context, personID int64, relationshipType users.RelationshipType) ([]*users.PersonGuardian, error) {
	var relationships []*users.PersonGuardian
	err := r.db.NewSelect().
		Model(&relationships).
		ModelTableExpr(tableExprPersonsGuardiansAsPG).
		Where(`"person_guardian".person_id = ? AND "person_guardian".relationship_type = ?`, personID, relationshipType).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by relationship type",
			Err: err,
		}
	}

	return relationships, nil
}

// SetPrimary sets a guardian as the primary guardian for a person
func (r *PersonGuardianRepository) SetPrimary(ctx context.Context, id int64, isPrimary bool) error {
	// Database has a trigger that automatically manages the primary status
	// Just update the current relationship
	_, err := r.db.NewUpdate().
		Model((*users.PersonGuardian)(nil)).
		ModelTableExpr(tableExprPersonsGuardiansAsPG).
		Set("is_primary = ?", isPrimary).
		Where(wherePersonGuardianIDEquals, id).
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "set primary",
			Err: err,
		}
	}

	return nil
}

// UpdatePermissions updates a guardian's permissions
func (r *PersonGuardianRepository) UpdatePermissions(ctx context.Context, id int64, permissions string) error {
	_, err := r.db.NewUpdate().
		Model((*users.PersonGuardian)(nil)).
		ModelTableExpr(tableExprPersonsGuardiansAsPG).
		Set("permissions = ?", permissions).
		Where(wherePersonGuardianIDEquals, id).
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
func (r *PersonGuardianRepository) Create(ctx context.Context, relationship *users.PersonGuardian) error {
	if relationship == nil {
		return fmt.Errorf("person guardian relationship cannot be nil")
	}

	// Validate relationship
	if err := relationship.Validate(); err != nil {
		return err
	}

	// Use the base Create method
	return r.Repository.Create(ctx, relationship)
}

// Update overrides the base Update method to handle validation
func (r *PersonGuardianRepository) Update(ctx context.Context, relationship *users.PersonGuardian) error {
	if relationship == nil {
		return fmt.Errorf("person guardian relationship cannot be nil")
	}

	// Validate relationship
	if err := relationship.Validate(); err != nil {
		return err
	}

	// Use the base Update method
	return r.Repository.Update(ctx, relationship)
}

// Legacy method to maintain compatibility with old interface
func (r *PersonGuardianRepository) List(ctx context.Context, filters map[string]interface{}) ([]*users.PersonGuardian, error) {
	// Convert old filter format to new QueryOptions
	options := modelBase.NewQueryOptions()
	filter := modelBase.NewFilter()

	for field, value := range filters {
		if value != nil {
			switch field {
			case "is_primary":
				filter.Equal("is_primary", value)
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

// ListWithOptions provides a type-safe way to list person guardian relationships with query options
func (r *PersonGuardianRepository) ListWithOptions(ctx context.Context, options *modelBase.QueryOptions) ([]*users.PersonGuardian, error) {
	var relationships []*users.PersonGuardian
	query := r.db.NewSelect().
		Model(&relationships).
		ModelTableExpr(tableExprPersonsGuardiansAsPG)

	// Apply query options with table alias
	if options != nil {
		if options.Filter != nil {
			options.Filter.WithTableAlias("person_guardian")
		}
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

// FindWithPerson retrieves a person guardian relationship with its associated person
func (r *PersonGuardianRepository) FindWithPerson(ctx context.Context, id int64) (*users.PersonGuardian, error) {
	// First get the person guardian relationship
	relationship := new(users.PersonGuardian)
	err := r.db.NewSelect().
		Model(relationship).
		ModelTableExpr(tableExprPersonsGuardiansAsPG).
		Where(wherePersonGuardianIDEquals, id).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find with person",
			Err: err,
		}
	}

	// Then load the person if PersonID is set
	if relationship.PersonID > 0 {
		person := new(users.Person)
		personErr := r.db.NewSelect().
			Model(person).
			ModelTableExpr(`users.persons AS "person"`).
			Where(`"person".id = ?`, relationship.PersonID).
			Scan(ctx)

		if personErr == nil {
			relationship.Person = person
		} else if !errors.Is(personErr, sql.ErrNoRows) {
			// Only ignore "not found" errors - propagate all other DB errors
			return nil, &modelBase.DatabaseError{
				Op:  "find with person - load person",
				Err: personErr,
			}
		}
		// Person not found is acceptable - relationship.Person remains nil
	}

	return relationship, nil
}

// GrantPermissionToGuardian grants a specific permission to a guardian
func (r *PersonGuardianRepository) GrantPermissionToGuardian(ctx context.Context, id int64, permission string) error {
	// Get the current relationship
	relationship, err := r.FindByID(ctx, id)
	if err != nil {
		return err
	}

	// Grant the permission
	if err := relationship.GrantPermission(permission); err != nil {
		return err
	}

	// Update the relationship
	_, err = r.db.NewUpdate().
		Model(relationship).
		ModelTableExpr(tableExprPersonsGuardiansAsPG).
		Column("permissions").
		Where(wherePersonGuardianIDEquals, id).
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "grant permission",
			Err: err,
		}
	}

	return nil
}

// RevokePermissionFromGuardian revokes a specific permission from a guardian
func (r *PersonGuardianRepository) RevokePermissionFromGuardian(ctx context.Context, id int64, permission string) error {
	// Get the current relationship
	relationship, err := r.FindByID(ctx, id)
	if err != nil {
		return err
	}

	// Revoke the permission
	if err := relationship.RevokePermission(permission); err != nil {
		return err
	}

	// Update the relationship
	_, err = r.db.NewUpdate().
		Model(relationship).
		ModelTableExpr(tableExprPersonsGuardiansAsPG).
		Column("permissions").
		Where(wherePersonGuardianIDEquals, id).
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "revoke permission",
			Err: err,
		}
	}

	return nil
}
