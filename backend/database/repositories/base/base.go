package base

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"strings"
	"unicode"

	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// Repository provides a generic implementation of common CRUD operations
type Repository[T modelBase.Entity] struct {
	DB           *bun.DB
	TableName    string
	EntityName   string
	TenantScoped bool // enables defense-in-depth WHERE tenant_id = ? on queries
}

// NewRepository creates a new base repository instance
func NewRepository[T modelBase.Entity](db *bun.DB, tableName, entityName string) *Repository[T] {
	return &Repository[T]{
		DB:         db,
		TableName:  tableName,
		EntityName: entityName,
	}
}

// applyTenantFilter adds a WHERE tenant_id = ? clause if the repository is tenant-scoped
// and a tenant ID is present in the context. This is a defense-in-depth measure
// layered on top of PostgreSQL RLS policies.
func (r *Repository[T]) applyTenantFilter(ctx context.Context, query *bun.SelectQuery, alias string) *bun.SelectQuery {
	if r.TenantScoped {
		if tenantID := tenant.FromContext(ctx); tenantID > 0 {
			query = query.Where(fmt.Sprintf(`"%s".tenant_id = ?`, alias), tenantID)
		}
	}
	return query
}

// Create inserts a new entity into the database
func (r *Repository[T]) Create(ctx context.Context, entity T) error {
	// Check if entity is nil using reflection
	if reflect.ValueOf(entity).IsZero() {
		return fmt.Errorf("%s cannot be nil or zero value", r.EntityName)
	}

	// Validate entity if it implements the Validator interface
	if validator, ok := any(entity).(modelBase.Validator); ok {
		if err := validator.Validate(); err != nil {
			return err
		}
	}

	// Explicitly set the table name with schema
	_, err := GetDB(ctx, r.DB).NewInsert().
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
func (r *Repository[T]) FindByID(ctx context.Context, id any) (T, error) {
	var entity T

	// Create a new instance of entity type
	entityType := reflect.TypeFor[T]()

	// If it's a pointer type, get the element type
	if entityType.Kind() == reflect.Pointer {
		entityType = entityType.Elem()
	}

	entityVal := reflect.New(entityType).Interface().(T)

	// Use ModelTableExpr to specify the schema-qualified table name with proper alias
	// Convert EntityName from CamelCase to snake_case for consistent alias
	entityName := toSnakeCase(strings.TrimPrefix(r.EntityName, "*"))
	tableExpr := fmt.Sprintf(`%s AS "%s"`, r.TableName, entityName)

	query := GetDB(ctx, r.DB).NewSelect().
		Model(entityVal).
		ModelTableExpr(tableExpr).
		Where(fmt.Sprintf(`"%s".id = ?`, entityName), id)

	query = r.applyTenantFilter(ctx, query, entityName)

	err := query.Scan(ctx)
	if err != nil {
		return entity, &modelBase.DatabaseError{
			Op:  "find by id",
			Err: err,
		}
	}

	return entityVal, nil
}

// Update updates an existing entity in the database
func (r *Repository[T]) Update(ctx context.Context, entity T) error {
	// Check if entity is nil using reflection
	if reflect.ValueOf(entity).IsZero() {
		return fmt.Errorf("%s cannot be nil or zero value", r.EntityName)
	}

	// Validate entity if it implements the Validator interface
	if validator, ok := any(entity).(modelBase.Validator); ok {
		if err := validator.Validate(); err != nil {
			return err
		}
	}

	// Use ModelTableExpr to specify the schema-qualified table name with proper alias
	// Convert EntityName from CamelCase to snake_case for consistent alias
	entityName := toSnakeCase(strings.TrimPrefix(r.EntityName, "*"))
	tableExpr := fmt.Sprintf(`%s AS "%s"`, r.TableName, entityName)

	updateQuery := GetDB(ctx, r.DB).NewUpdate().
		Model(entity).
		ModelTableExpr(tableExpr).
		WherePK()

	if r.TenantScoped {
		if tenantID := tenant.FromContext(ctx); tenantID > 0 {
			updateQuery = updateQuery.Where(fmt.Sprintf(`"%s".tenant_id = ?`, entityName), tenantID)
		}
	}

	result, err := updateQuery.Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "update",
			Err: err,
		}
	}

	return AssertRowsAffected(result, 1, "update "+r.EntityName)
}

// Delete removes an entity from the database
func (r *Repository[T]) Delete(ctx context.Context, id any) error {
	// Create a new instance of entity type
	entityType := reflect.TypeFor[T]()

	// If it's a pointer type, get the element type
	if entityType.Kind() == reflect.Pointer {
		entityType = entityType.Elem()
	}

	entityVal := reflect.New(entityType).Interface()

	// Use ModelTableExpr to specify the schema-qualified table name with proper alias
	// Convert EntityName from CamelCase to snake_case for consistent alias
	entityName := toSnakeCase(strings.TrimPrefix(r.EntityName, "*"))
	tableExpr := fmt.Sprintf(`%s AS "%s"`, r.TableName, entityName)

	deleteQuery := GetDB(ctx, r.DB).NewDelete().
		Model(entityVal).
		ModelTableExpr(tableExpr).
		Where(fmt.Sprintf(`"%s".id = ?`, entityName), id)

	if r.TenantScoped {
		if tenantID := tenant.FromContext(ctx); tenantID > 0 {
			deleteQuery = deleteQuery.Where(fmt.Sprintf(`"%s".tenant_id = ?`, entityName), tenantID)
		}
	}

	result, err := deleteQuery.Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "delete",
			Err: err,
		}
	}

	return AssertRowsAffected(result, 1, "delete "+r.EntityName)
}

// List retrieves entities matching the filters
func (r *Repository[T]) List(ctx context.Context, filters map[string]any) ([]T, error) {
	var entities []T

	// Use ModelTableExpr to specify the schema-qualified table name with proper alias
	// Convert EntityName from CamelCase to snake_case for consistent alias
	entityName := toSnakeCase(strings.TrimPrefix(r.EntityName, "*"))
	tableExpr := fmt.Sprintf(`%s AS "%s"`, r.TableName, entityName)

	query := GetDB(ctx, r.DB).NewSelect().
		Model(&entities).
		ModelTableExpr(tableExpr)

	query = r.applyTenantFilter(ctx, query, entityName)

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
func (r *Repository[T]) Count(ctx context.Context, filters map[string]any) (int, error) {
	// Create a new instance of entity type
	entityType := reflect.TypeFor[T]()

	// If it's a pointer type, get the element type
	if entityType.Kind() == reflect.Pointer {
		entityType = entityType.Elem()
	}

	entityVal := reflect.New(entityType).Interface()

	// Use ModelTableExpr to specify the schema-qualified table name with proper alias
	// Get the entity name in lowercase to use as alias
	entityName := strings.ToLower(strings.TrimPrefix(r.EntityName, "*"))
	tableExpr := fmt.Sprintf(`%s AS "%s"`, r.TableName, entityName)

	query := GetDB(ctx, r.DB).NewSelect().
		Model(entityVal).
		ModelTableExpr(tableExpr).
		Column("id")

	if r.TenantScoped {
		if tenantID := tenant.FromContext(ctx); tenantID > 0 {
			query = query.Where(fmt.Sprintf(`"%s".tenant_id = ?`, entityName), tenantID)
		}
	}

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

// AssertRowsAffected checks that a DML statement affected exactly the expected number of rows.
func AssertRowsAffected(result sql.Result, expected int64, op string) error {
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("%s: rows affected: %w", op, err)
	}
	if n != expected {
		return &modelBase.DatabaseError{
			Op:  op,
			Err: fmt.Errorf("expected %d rows affected, got %d", expected, n),
		}
	}
	return nil
}

// TenantWhere returns a WHERE clause fragment and value for tenant filtering.
// Use this in custom repository methods to add defense-in-depth tenant_id checks.
func TenantWhere(ctx context.Context, alias string) (string, int64, bool) {
	tenantID := tenant.FromContext(ctx)
	if tenantID > 0 {
		return fmt.Sprintf(`"%s".tenant_id = ?`, alias), tenantID, true
	}
	return "", 0, false
}

// toSnakeCase converts a CamelCase string to snake_case
// Example: "StudentGuardian" -> "student_guardian"
func toSnakeCase(s string) string {
	var result strings.Builder
	for i, r := range s {
		if unicode.IsUpper(r) {
			if i > 0 {
				result.WriteRune('_')
			}
			result.WriteRune(unicode.ToLower(r))
		} else {
			result.WriteRune(r)
		}
	}
	return result.String()
}
