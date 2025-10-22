# Migration Framework Enhancement Capability

## ADDED Requirements

### Requirement: Migration Code Generation
The system SHALL auto-generate migration boilerplate to eliminate 3,876 lines of duplicate code across 57 migration files.

**Context**: Every migration file contains ~68 lines of identical boilerplate (Version, Description, Dependencies, init() registration, transaction defer patterns). This accounts for 49.6% of all migration code.

#### Scenario: Generate version and description from filename
- **GIVEN** migration file named `001003005_users_students.go`
- **WHEN** migration generator processes the file
- **THEN** SHALL extract version "1.3.5" from filename prefix
- **AND** SHALL extract description "users students" from filename suffix
- **AND** SHALL auto-generate constants without manual declaration

#### Scenario: Auto-generate init registration
- **GIVEN** migration SQL is defined
- **WHEN** generator creates migration file
- **THEN** SHALL auto-generate `init() { Migrations.MustRegister(...) }` block
- **AND** SHALL include both up and down functions
- **AND** developers SHALL only write SQL, not registration code

#### Scenario: Auto-generate transaction defer pattern
- **GIVEN** migration needs transactional execution
- **WHEN** generator creates up/down functions
- **THEN** SHALL auto-generate transaction defer with error checking
- **AND** pattern SHALL be identical across all migrations (currently 114 manual duplicates)
- **AND** SHALL include proper rollback on error

### Requirement: Common Migration Pattern Helpers
The system SHALL provide helper functions for common migration patterns used across 40+ migrations.

#### Scenario: Updated-at trigger helper
- **GIVEN** table needs automatic `updated_at` timestamp updates
- **WHEN** migration calls `CreateUpdatedAtTrigger(tableName)`
- **THEN** helper SHALL generate trigger SQL for updating `updated_at` on row updates
- **AND** trigger SQL SHALL be identical to current manual implementations
- **AND** SHALL work across all schema-qualified tables

**Context**: 40+ migrations manually write identical `updated_at` trigger SQL - this helper eliminates that duplication.

#### Scenario: Foreign key constraint helper
- **GIVEN** table has foreign key to another table
- **WHEN** migration calls `CreateForeignKey(table, column, refTable, refColumn)`
- **THEN** helper SHALL generate standard foreign key constraint SQL
- **AND** SHALL include ON DELETE and ON UPDATE clauses with sensible defaults
- **AND** SHALL generate proper constraint naming

#### Scenario: Index creation helper
- **GIVEN** table needs index on columns
- **WHEN** migration calls `CreateIndex(table, columns, unique bool)`
- **THEN** helper SHALL generate CREATE INDEX statement
- **AND** SHALL use standard naming convention: `idx_{table}_{columns}`
- **AND** SHALL support unique and non-unique indexes

#### Scenario: Drop table with cascade helper
- **GIVEN** rollback needs to drop table
- **WHEN** migration calls `DropTableCascade(tableName)`
- **THEN** helper SHALL generate `DROP TABLE IF EXISTS {table} CASCADE`
- **AND** SHALL use consistent format across all migrations

### Requirement: Auto-Generated Rollback Functions
The migration framework SHALL auto-generate rollback functions from CREATE statements where possible.

#### Scenario: Simple table creation rollback
- **GIVEN** migration creates table with CREATE TABLE statement
- **WHEN** generator analyzes migration SQL
- **THEN** SHALL auto-generate rollback as `DROP TABLE IF EXISTS {table} CASCADE`
- **AND** developers SHALL NOT manually write rollback for simple creates

#### Scenario: Complex migration rollback
- **GIVEN** migration performs complex multi-statement operations
- **WHEN** auto-generation not possible
- **THEN** framework SHALL allow manual rollback definition
- **AND** SHALL clearly mark manual vs auto-generated rollbacks

### Requirement: Compile-Time Migration Dependency Validation
The migration system SHALL validate dependency graphs at compile time, not runtime.

#### Scenario: Detect circular dependencies
- **GIVEN** migration A depends on B, B depends on C, C depends on A
- **WHEN** running dependency validator
- **THEN** SHALL detect circular dependency at compile time
- **AND** SHALL fail with clear error message showing the cycle
- **AND** SHALL prevent runtime migration failures

#### Scenario: Detect missing dependencies
- **GIVEN** migration depends on version that doesn't exist
- **WHEN** validating dependency graph
- **THEN** SHALL fail at compile time with clear error
- **AND** SHALL list all missing dependencies

#### Scenario: Validate dependency order
- **GIVEN** migration dependencies form directed acyclic graph
- **WHEN** validator analyzes dependencies
- **THEN** SHALL compute correct execution order
- **AND** SHALL verify no conflicts
- **AND** SHALL output visualization of dependency tree

### Requirement: Migration Testing Framework
The system SHALL provide automated testing for migration up/down cycles.

#### Scenario: Automated migration test generation
- **GIVEN** migration file exists
- **WHEN** test generator processes migration
- **THEN** SHALL create test that:
  - Applies migration (up)
  - Verifies schema changes applied
  - Rolls back migration (down)
  - Verifies schema returned to original state

#### Scenario: Schema drift detection
- **GIVEN** database migrations applied to production
- **WHEN** running schema drift check
- **THEN** SHALL compare production schema to migration-generated schema
- **AND** SHALL report any differences (manual changes, missing migrations)
- **AND** SHALL fail CI if drift detected

### Requirement: Migration DSL or Declarative Format
The system SHALL support declarative migration format to reduce manual SQL writing for common patterns.

#### Scenario: Table definition in Go structs
- **GIVEN** developer wants to create new table
- **WHEN** defining migration using Go struct with tags
- **THEN** generator SHALL produce CREATE TABLE SQL from struct definition
- **AND** SHALL infer column types from Go types
- **AND** SHALL use struct tags for constraints (NOT NULL, UNIQUE, etc.)

**Example**:
```go
type StudentTable struct {
    ID          int64  `db:"id,pk,autoincrement"`
    PersonID    int64  `db:"person_id,notnull,fk:users.persons.id"`
    GroupID     *int64 `db:"group_id,fk:education.groups.id"`
    SchoolClass string `db:"school_class,notnull"`
}
// Auto-generates full CREATE TABLE with constraints
```

#### Scenario: Rollback auto-generation from struct
- **GIVEN** migration uses struct-based table definition
- **WHEN** generator creates migration
- **THEN** SHALL auto-generate rollback as DROP TABLE
- **AND** SHALL include CASCADE if foreign keys reference this table

## Migration Notes
- Migration framework enhancements are additive - existing migrations continue working
- Code generation optional initially - can adopt per migration file
- Struct-based migrations coexist with SQL-based migrations
- Migration testing framework validates both old and new style migrations
- Compile-time validation catches errors before runtime
