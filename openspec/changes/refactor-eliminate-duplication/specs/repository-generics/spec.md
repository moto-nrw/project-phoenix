# Repository Generics Enhancement Capability

## MODIFIED Requirements

### Requirement: Enhanced Generic Repository with ListWithOptions
The base.Repository[T] SHALL provide generic `ListWithOptions()` to eliminate 13 duplicate override implementations.

**Context**: Currently 13 repositories override `ListWithOptions()` with nearly identical 20-line implementations because base repository doesn't provide this method.

#### Scenario: Generic list with query options
- **GIVEN** repository extends `base.Repository[T]`
- **WHEN** calling `repo.ListWithOptions(ctx, options)`
- **THEN** base implementation SHALL build query with ModelTableExpr for the configured table
- **AND** SHALL apply query options (filters, sorting, pagination) if provided
- **AND** SHALL scan results into slice of T
- **AND** SHALL wrap database errors in DatabaseError

#### Scenario: Schema-qualified table expression
- **GIVEN** repository is initialized with schema-qualified table name
- **WHEN** generic `ListWithOptions()` executes
- **THEN** SHALL use `ModelTableExpr` with properly quoted alias
- **AND** alias SHALL be derived from entity type name
- **AND** SHALL handle multi-schema queries correctly

### Requirement: Generic FindByField Method
The base.Repository[T] SHALL provide `FindByField()` to eliminate ~50 duplicate FindByForeignKey implementations.

#### Scenario: Find by foreign key field
- **GIVEN** repository needs to find entities by foreign key
- **WHEN** calling `repo.FindByField(ctx, "group_id", groupID)`
- **THEN** base SHALL build WHERE clause for specified field
- **AND** SHALL return slice of matching entities
- **AND** SHALL handle no results (return empty slice, not error)
- **AND** SHALL wrap database errors appropriately

#### Scenario: Find single entity by field
- **GIVEN** repository expects single result
- **WHEN** calling `repo.FindOneByField(ctx, "email", email)`
- **THEN** SHALL query for single row with WHERE clause
- **AND** SHALL return entity or nil if not found
- **AND** SHALL not error on sql.ErrNoRows
- **AND** SHALL return DatabaseError for actual database errors

#### Scenario: Type-safe field names
- **GIVEN** caller provides field name as string
- **WHEN** using `FindByField(ctx, fieldName, value)`
- **THEN** SHALL validate field exists on entity type (compile-time or runtime check)
- **AND** SHALL return clear error if field doesn't exist
- **AND** SHALL prevent SQL injection through field validation

## ADDED Requirements

### Requirement: Automatic Validation Hooks in Base Repository
The base.Repository[T] SHALL automatically validate entities before Create/Update, eliminating 15 × 20 = 300 lines of duplicate validation wrappers.

#### Scenario: Validation on create
- **GIVEN** entity type implements Validator interface
- **WHEN** calling `repo.Create(ctx, entity)`
- **THEN** base SHALL automatically call `entity.Validate()` before database insert
- **AND** SHALL return validation error without attempting insert
- **AND** repository SHALL NOT need to override Create just for validation

#### Scenario: Validation on update
- **GIVEN** entity implements Validator interface
- **WHEN** calling `repo.Update(ctx, entity)`
- **THEN** base SHALL call `entity.Validate()` before database update
- **AND** SHALL return validation error without attempting update

#### Scenario: Skip validation for entities without Validator interface
- **GIVEN** entity does NOT implement Validator interface
- **WHEN** calling Create or Update
- **THEN** base SHALL skip validation check
- **AND** SHALL proceed directly to database operation

### Requirement: Repository Error Wrapping Utilities
The system SHALL provide error wrapping utilities to eliminate 395 duplicate DatabaseError wrapping occurrences.

#### Scenario: Automatic error wrapping
- **GIVEN** base repository method encounters database error
- **WHEN** returning error to caller
- **THEN** SHALL automatically wrap in `modelBase.DatabaseError{Op: "operation", Err: err}`
- **AND** operation name SHALL be derived from method name
- **AND** repositories SHALL NOT manually wrap errors

#### Scenario: Error context preservation
- **GIVEN** database operation fails
- **WHEN** error is wrapped by base repository
- **THEN** SHALL preserve original error for unwrapping
- **AND** SHALL include operation context (method name, entity type)
- **AND** errors.Is and errors.As SHALL work correctly

### Requirement: Generic Relation Loading
The base.Repository[T] SHALL provide helpers for loading related entities, reducing duplication in Join queries.

#### Scenario: Load single relation
- **GIVEN** entity has foreign key to another entity
- **WHEN** calling `repo.WithRelation(ctx, entity, "RelationName")`
- **THEN** SHALL load related entity using BUN's Relation() method
- **AND** SHALL populate entity's relation field
- **AND** SHALL handle nil foreign keys gracefully

#### Scenario: Batch relation loading
- **GIVEN** slice of entities with same relation
- **WHEN** calling `repo.WithRelations(ctx, entities, "RelationName")`
- **THEN** SHALL efficiently load all relations (avoid N+1 queries)
- **AND** SHALL use IN query for batch loading

### Requirement: Transaction Context Utilities
The base.Repository[T] SHALL provide transaction context helpers to standardize transaction handling across repositories.

#### Scenario: Detect active transaction
- **GIVEN** repository method executes in transaction context
- **WHEN** method needs database connection
- **THEN** base utility SHALL check context for active transaction
- **AND** SHALL use transaction if present
- **AND** SHALL use standard db connection if no transaction

#### Scenario: Transaction-aware query building
- **GIVEN** repository builds query
- **WHEN** calling base helper `r.NewSelect(ctx)`
- **THEN** helper SHALL return query bound to active transaction if present
- **AND** SHALL return query bound to standard db otherwise
- **AND** repositories SHALL NOT manually check for transaction

## Migration Notes
- Base repository already exists with basic CRUD - this enhances it
- Existing custom repository methods still work (not breaking)
- 39 repositories automatically gain new methods via inheritance
- 6 specialized repositories (audit, rate limiting, etc.) can opt-out by not using base
- Generic FindByField supplements (not replaces) custom query methods for complex cases
