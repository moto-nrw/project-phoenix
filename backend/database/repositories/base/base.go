package base

import (
	"context"
	"fmt"
	"reflect"

	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// Repository provides a generic implementation of common CRUD operations
type Repository[T modelBase.Entity] struct {
	DB         *bun.DB
	TableName  string
	EntityName string
}

// NewRepository creates a new base repository instance
func NewRepository[T modelBase.Entity](db *bun.DB, tableName, entityName string) *Repository[T] {
	return &Repository[T]{
		DB:         db,
		TableName:  tableName,
		EntityName: entityName,
	}
}

// Create inserts a new entity into the database
func (r *Repository[T]) Create(ctx context.Context, entity T) error {
	// Check if entity is nil using reflection
	if reflect.ValueOf(entity).IsZero() {
		return fmt.Errorf("%s cannot be nil or zero value", r.EntityName)
	}

	// Validate entity if it implements the Validator interface
	if validator, ok := interface{}(entity).(modelBase.Validator); ok {
		if err := validator.Validate(); err != nil {
			return err
		}
	}

	// Explicitly set the table name with schema
	_, err := r.DB.NewInsert().
		Model(entity).
		ModelTableExpr(r.TableName).
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "create",
			Err: err,
		}
	}

	return nil
}

// FindByID retrieves an entity by its ID
func (r *Repository[T]) FindByID(ctx context.Context, id interface{}) (T, error) {
	var entity T

	// Create a new instance of entity type
	entityType := reflect.TypeOf((*T)(nil)).Elem()

	// If it's a pointer type, get the element type
	if entityType.Kind() == reflect.Ptr {
		entityType = entityType.Elem()
	}

	entityVal := reflect.New(entityType).Interface().(T)

	// Use ModelTableExpr to specify the schema-qualified table name with the standard "room" alias
	// This keeps consistency with the rest of the codebase
	tableExpr := fmt.Sprintf("%s AS room", r.TableName)

	err := r.DB.NewSelect().
		Model(entityVal).
		ModelTableExpr(tableExpr).
		Where("id = ?", id).
		Scan(ctx)
	if err != nil {
		return entity, &modelBase.DatabaseError{
			Op:  "find by id",
			Err: err,
		}
	}

	return entityVal, nil
}

// lastIndexOfChar returns the last index of the character c in string s, or -1 if not found
func lastIndexOfChar(s string, c byte) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == c {
			return i
		}
	}
	return -1
}

// Update updates an existing entity in the database
func (r *Repository[T]) Update(ctx context.Context, entity T) error {
	// Check if entity is nil using reflection
	if reflect.ValueOf(entity).IsZero() {
		return fmt.Errorf("%s cannot be nil or zero value", r.EntityName)
	}

	// Validate entity if it implements the Validator interface
	if validator, ok := interface{}(entity).(modelBase.Validator); ok {
		if err := validator.Validate(); err != nil {
			return err
		}
	}

	// Use ModelTableExpr to specify the schema-qualified table name with the standard "room" alias
	// This keeps consistency with the rest of the codebase
	tableExpr := fmt.Sprintf("%s AS room", r.TableName)

	_, err := r.DB.NewUpdate().
		Model(entity).
		ModelTableExpr(tableExpr).
		WherePK().
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "update",
			Err: err,
		}
	}

	return nil
}

// Delete removes an entity from the database
func (r *Repository[T]) Delete(ctx context.Context, id interface{}) error {
	// Create a new instance of entity type
	entityType := reflect.TypeOf((*T)(nil)).Elem()

	// If it's a pointer type, get the element type
	if entityType.Kind() == reflect.Ptr {
		entityType = entityType.Elem()
	}

	entityVal := reflect.New(entityType).Interface()

	// Use ModelTableExpr to specify the schema-qualified table name with the standard "room" alias
	// This keeps consistency with the rest of the codebase
	tableExpr := fmt.Sprintf("%s AS room", r.TableName)

	_, err := r.DB.NewDelete().
		Model(entityVal).
		ModelTableExpr(tableExpr).
		Where("id = ?", id).
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "delete",
			Err: err,
		}
	}

	return nil
}

// List retrieves entities matching the filters
func (r *Repository[T]) List(ctx context.Context, filters map[string]interface{}) ([]T, error) {
	var entities []T

	// Use ModelTableExpr to specify the schema-qualified table name with the standard "room" alias
	// This keeps consistency with the rest of the codebase
	tableExpr := fmt.Sprintf("%s AS room", r.TableName)

	query := r.DB.NewSelect().
		Model(&entities).
		ModelTableExpr(tableExpr)

	// Apply filters
	for field, value := range filters {
		if value != nil {
			query = query.Where("? = ?", bun.Ident(field), value)
		}
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "list",
			Err: err,
		}
	}

	return entities, nil
}

// Count returns the number of entities matching the filters
func (r *Repository[T]) Count(ctx context.Context, filters map[string]interface{}) (int, error) {
	// Create a new instance of entity type
	entityType := reflect.TypeOf((*T)(nil)).Elem()

	// If it's a pointer type, get the element type
	if entityType.Kind() == reflect.Ptr {
		entityType = entityType.Elem()
	}

	entityVal := reflect.New(entityType).Interface()

	// Use ModelTableExpr to specify the schema-qualified table name with the standard "room" alias
	// This keeps consistency with the rest of the codebase
	tableExpr := fmt.Sprintf("%s AS room", r.TableName)

	query := r.DB.NewSelect().
		Model(entityVal).
		ModelTableExpr(tableExpr).
		Column("id")

	// Apply filters
	for field, value := range filters {
		if value != nil {
			query = query.Where("? = ?", bun.Ident(field), value)
		}
	}

	count, err := query.Count(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "count",
			Err: err,
		}
	}

	return count, nil
}

// Transaction executes a function within a database transaction
func (r *Repository[T]) Transaction(ctx context.Context, fn func(tx bun.Tx) error) error {
	return r.DB.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		return fn(tx)
	})
}
