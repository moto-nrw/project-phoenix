package base

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"
	"unicode"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
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

	// Auto-set tenant_id from context if the entity is tenant-scoped and tenant_id is not yet set
	if ts, ok := any(entity).(modelBase.TenantScoped); ok && ts.GetTenantID() == 0 {
		if tid := tenant.FromContext(ctx); tid != 0 {
			ts.SetTenantID(tid)
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

// FindByIDOrNil behaves like FindByID but returns the zero value and a nil error
// when no row matches, instead of a wrapped sql.ErrNoRows. Repositories whose
// service layer treats "not found" as nil (rather than an error) delegate their
// FindByID override here instead of hand-rolling the query — keeping tenant
// scoping and table/alias derivation in one place (backend-conventions Rule 2).
func (r *Repository[T]) FindByIDOrNil(ctx context.Context, id any) (T, error) {
	entity, err := r.FindByID(ctx, id)
	if err != nil {
		var dbErr *modelBase.DatabaseError
		if errors.As(err, &dbErr) && errors.Is(dbErr.Err, sql.ErrNoRows) {
			var zero T
			return zero, nil
		}
		return entity, err
	}
	return entity, nil
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
	entityVal := r.newEntityValue()

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

	_, err := deleteQuery.Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "delete",
			Err: err,
		}
	}

	return nil
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
		ModelTableExpr(tableExpr).
		ColumnExpr(fmt.Sprintf(`"%s".*`, entityName))

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

// ListWithOptions retrieves entities matching the query options, supporting
// the full Filter operator set plus sorting and pagination. The single-table
// twin of the hand-rolled per-repo List(QueryOptions) copies; returns a nil
// slice when nothing matches (callers that must serialize JSON [] coerce).
func (r *Repository[T]) ListWithOptions(ctx context.Context, options *modelBase.QueryOptions) ([]T, error) {
	var entities []T

	entityName := toSnakeCase(strings.TrimPrefix(r.EntityName, "*"))
	tableExpr := fmt.Sprintf(`%s AS "%s"`, r.TableName, entityName)

	query := GetDB(ctx, r.DB).NewSelect().
		Model(&entities).
		ModelTableExpr(tableExpr).
		ColumnExpr(fmt.Sprintf(`"%s".*`, entityName))

	query = r.applyTenantFilter(ctx, query, entityName)

	if options != nil {
		if options.Filter != nil {
			options.Filter.WithTableAlias(entityName)
		}
		query = options.ApplyToQuery(query)
	}

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "list with options",
			Err: err,
		}
	}

	return entities, nil
}

// CountWithOptions returns the number of entities matching the query options.
// Unlike Count, it supports the full Filter operator set (LessThan, IsNull,
// In, ...). Sorting and pagination are ignored — they cannot change a count.
func (r *Repository[T]) CountWithOptions(ctx context.Context, options *modelBase.QueryOptions) (int, error) {
	entityVal := r.newEntityValue()

	entityName := toSnakeCase(strings.TrimPrefix(r.EntityName, "*"))
	tableExpr := fmt.Sprintf(`%s AS "%s"`, r.TableName, entityName)

	query := GetDB(ctx, r.DB).NewSelect().
		Model(entityVal).
		ModelTableExpr(tableExpr).
		Column(entityName + ".id")

	query = r.applyTenantFilter(ctx, query, entityName)

	if options != nil && options.Filter != nil {
		options.Filter.WithTableAlias(entityName)
		query = options.Filter.ApplyToQuery(query)
	}

	count, err := query.Count(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "count with options",
			Err: err,
		}
	}

	return count, nil
}

// OldestBefore returns the minimum value of dateColumn among matching rows,
// optionally restricted to rows where dateColumn < cutoff (pass nil for the
// absolute minimum). Returns nil when no rows match. Intended for
// retention/cleanup statistics (oldest expired record, preview output).
// dateColumn must be a compile-time constant column name, never user input.
func (r *Repository[T]) OldestBefore(ctx context.Context, dateColumn string, cutoff *timezone.Date) (*timezone.Date, error) {
	entityVal := r.newEntityValue()

	entityName := toSnakeCase(strings.TrimPrefix(r.EntityName, "*"))
	tableExpr := fmt.Sprintf(`%s AS "%s"`, r.TableName, entityName)

	query := GetDB(ctx, r.DB).NewSelect().
		Model(entityVal).
		ModelTableExpr(tableExpr).
		ColumnExpr("MIN(?)", bun.Ident(dateColumn))

	query = r.applyTenantFilter(ctx, query, entityName)

	if cutoff != nil {
		query = query.Where("? < ?", bun.Ident(dateColumn), *cutoff)
	}

	var oldest *timezone.Date
	if err := query.Scan(ctx, &oldest); err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "oldest before",
			Err: err,
		}
	}

	return oldest, nil
}

// DeleteOlderThan deletes all matching rows whose dateColumn is strictly
// before cutoff and returns the number of rows deleted. Intended for
// retention/cleanup jobs (GDPR data expiry, stale-record pruning).
// dateColumn must be a compile-time constant column name, never user input.
func (r *Repository[T]) DeleteOlderThan(ctx context.Context, dateColumn string, cutoff timezone.Date) (int64, error) {
	entityVal := r.newEntityValue()

	entityName := toSnakeCase(strings.TrimPrefix(r.EntityName, "*"))
	tableExpr := fmt.Sprintf(`%s AS "%s"`, r.TableName, entityName)

	query := GetDB(ctx, r.DB).NewDelete().
		Model(entityVal).
		ModelTableExpr(tableExpr).
		Where("? < ?", bun.Ident(dateColumn), cutoff)

	if r.TenantScoped {
		if tenantID := tenant.FromContext(ctx); tenantID > 0 {
			query = query.Where(fmt.Sprintf(`"%s".tenant_id = ?`, entityName), tenantID)
		}
	}

	result, err := query.Exec(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "delete older than",
			Err: err,
		}
	}

	deleted, err := result.RowsAffected()
	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "delete older than",
			Err: err,
		}
	}

	return deleted, nil
}

// DeleteBefore deletes all matching rows whose column (a TIMESTAMPTZ instant)
// is strictly before cutoff and returns the number of rows deleted. The
// time.Time twin of DeleteOlderThan for expiry-style cleanup jobs. op becomes
// the DatabaseError Op so per-repo cleanup wrappers keep their exact error
// strings (they surface in scheduler failure logs, which are frozen).
// column must be a compile-time constant column name, never user input.
func (r *Repository[T]) DeleteBefore(ctx context.Context, column string, cutoff time.Time, op string) (int64, error) {
	entityVal := r.newEntityValue()

	entityName := toSnakeCase(strings.TrimPrefix(r.EntityName, "*"))
	tableExpr := fmt.Sprintf(`%s AS "%s"`, r.TableName, entityName)

	query := GetDB(ctx, r.DB).NewDelete().
		Model(entityVal).
		ModelTableExpr(tableExpr).
		Where("? < ?", bun.Ident(column), cutoff)

	if r.TenantScoped {
		if tenantID := tenant.FromContext(ctx); tenantID > 0 {
			query = query.Where(fmt.Sprintf(`"%s".tenant_id = ?`, entityName), tenantID)
		}
	}

	result, err := query.Exec(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: op, Err: err}
	}

	deleted, err := result.RowsAffected()
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: op, Err: err}
	}

	return deleted, nil
}

// UpdateColumns updates only the named columns of entity (matched by primary
// key) and returns the number of rows affected, so callers own the 0-rows
// decision. Unlike Update, it does not run entity validation: callers update
// a partial projection, so whole-entity invariants may not hold on the
// in-memory value. Include "updated_at" in columns (and set it on the entity)
// when the touch timestamp should move.
// Column names must be compile-time constants, never user input.
func (r *Repository[T]) UpdateColumns(ctx context.Context, entity T, columns ...string) (int64, error) {
	if reflect.ValueOf(entity).IsZero() {
		return 0, fmt.Errorf("%s cannot be nil or zero value", r.EntityName)
	}
	if len(columns) == 0 {
		return 0, fmt.Errorf("update columns %s: at least one column required", r.EntityName)
	}

	entityName := toSnakeCase(strings.TrimPrefix(r.EntityName, "*"))
	tableExpr := fmt.Sprintf(`%s AS "%s"`, r.TableName, entityName)

	query := GetDB(ctx, r.DB).NewUpdate().
		Model(entity).
		ModelTableExpr(tableExpr).
		Column(columns...).
		WherePK()

	if r.TenantScoped {
		if tenantID := tenant.FromContext(ctx); tenantID > 0 {
			query = query.Where(fmt.Sprintf(`"%s".tenant_id = ?`, entityName), tenantID)
		}
	}

	result, err := query.Exec(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "update columns",
			Err: err,
		}
	}

	updated, err := result.RowsAffected()
	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "update columns",
			Err: err,
		}
	}

	return updated, nil
}

// newEntityValue creates a new zero-value instance of the entity type for use
// as a bun model target (table metadata only, never scanned into).
func (r *Repository[T]) newEntityValue() any {
	entityType := reflect.TypeFor[T]()
	if entityType.Kind() == reflect.Pointer {
		entityType = entityType.Elem()
	}
	return reflect.New(entityType).Interface()
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

// WithTenantFilter applies the TenantWhere defense-in-depth tenant_id filter
// for alias to q, returning q unchanged when no tenant is in ctx.
func WithTenantFilter[Q interface{ Where(string, ...any) Q }](ctx context.Context, q Q, alias string) Q {
	if where, val, ok := TenantWhere(ctx, alias); ok {
		return q.Where(where, val)
	}
	return q
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

// AcquireXactLock takes an EXCLUSIVE transaction-scoped advisory lock on the
// hashed key. Requires a transaction in ctx (TenantTxMiddleware / WithTenantTx);
// the lock releases automatically at COMMIT/ROLLBACK. Issue #584: shared helper
// so services never run ExecContext themselves (Rule 11).
func AcquireXactLock(ctx context.Context, db *bun.DB, key string) error {
	_, err := GetDB(ctx, db).ExecContext(ctx, "SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", key)
	return err
}

// AcquireXactLockShared takes a SHARED transaction-scoped advisory lock on the
// hashed key. Shared holders never block one another, so concurrent readers of
// the same key proceed in parallel; a shared holder only conflicts with an
// EXCLUSIVE holder (AcquireXactLock) on the same key. Use this for read paths
// that must be mutually exclusive with a writer but not with each other — it
// avoids serializing every reader behind the exclusive lock. Same tx/auto-release
// semantics as AcquireXactLock.
func AcquireXactLockShared(ctx context.Context, db *bun.DB, key string) error {
	_, err := GetDB(ctx, db).ExecContext(ctx, "SELECT pg_advisory_xact_lock_shared(hashtextextended(?, 0))", key)
	return err
}
