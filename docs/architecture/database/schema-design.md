# Multi-Schema PostgreSQL Database Design

## Overview

Project Phoenix uses a **multi-schema PostgreSQL database** to organize tables
by domain, providing logical separation while maintaining the benefits of a
single database system.

## Design Rationale

### Why Multi-Schema Instead of Multiple Databases?

| Aspect                | Multi-Schema (Chosen)                | Multiple Databases                    |
| --------------------- | ------------------------------------ | ------------------------------------- |
| **Transactions**      | ✅ ACID across all schemas           | ❌ Distributed transactions complex   |
| **Joins**             | ✅ Cross-schema joins supported      | ❌ Cross-database joins not possible  |
| **Connection Pool**   | ✅ Single pool for all data          | ❌ Multiple pools = resource overhead |
| **Backup/Restore**    | ✅ Single database backup            | ❌ Multiple backups to coordinate     |
| **Logical Isolation** | ✅ Schema-level organization         | ✅ Database-level isolation           |
| **Access Control**    | ⚠️ Schema-level permissions (future) | ✅ Database-level permissions         |
| **Scaling**           | ⚠️ Single server initially           | ✅ Easier to shard later              |

**Decision**: Multi-schema provides the right balance for our current scale
while leaving a migration path to separate databases if needed in the future.

## Schema Organization

### Schema Hierarchy

```
PostgreSQL Database: postgres
├── auth                  # Authentication & Authorization
│   ├── accounts          # User accounts
│   ├── tokens            # JWT refresh tokens
│   ├── roles             # Role definitions
│   ├── permissions       # Permission definitions
│   └── role_permissions  # Role ↔ Permission mapping
├── users                 # Person Management (Hierarchy)
│   ├── persons           # Base person entity
│   ├── staff             # Staff extends Person
│   ├── teachers          # Teacher extends Staff
│   ├── students          # Student extends Person
│   ├── guardians         # Guardian extends Person
│   ├── student_guardians # Student ↔ Guardian relationship
│   └── rfid_cards        # RFID card assignments
├── education             # Educational Structures
│   ├── groups            # Student groups/classes
│   ├── group_teacher     # Group ↔ Teacher assignments
│   └── group_substitution # Temporary teacher substitutions
├── facilities            # Physical Infrastructure
│   ├── rooms             # Rooms/classrooms
│   ├── buildings         # Buildings (future)
│   └── locations         # Geographic locations (future)
├── activities            # Student Activities
│   ├── activities        # Activity definitions
│   ├── enrollments       # Student enrollments
│   └── categories        # Activity categories
├── active                # Real-Time State (High-Traffic!)
│   ├── groups            # Active group sessions
│   ├── visits            # Student visit tracking
│   ├── group_supervisors # Group ↔ Supervisor assignments
│   └── attendance        # Daily attendance records
├── schedule              # Time Management
│   ├── timeframes        # Time periods (start/end time)
│   ├── dateframes        # Date ranges
│   ├── recurrence_patterns # Recurring schedules
│   └── class_schedules   # Class schedule assignments
├── iot                   # Device Management
│   ├── devices           # RFID readers, tablets
│   └── device_logs       # Device activity logs (future)
├── feedback              # User Feedback
│   ├── entries           # Feedback submissions
│   └── comments          # Feedback responses
├── config                # System Configuration
│   ├── settings          # Key-value config
│   └── feature_flags     # Feature toggles
└── audit                 # GDPR Compliance & Auditing
    ├── data_deletions    # Deletion audit log
    └── auth_events       # Authentication events (future)
```

## Schema Design Principles

### 1. **Domain-Driven Schemas**

Each schema represents a **bounded context** from Domain-Driven Design:

```sql
-- Clear ownership: education domain owns groups
CREATE TABLE education.groups (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    room_id BIGINT REFERENCES facilities.rooms(id),
    created_at TIMESTAMP DEFAULT NOW()
);

-- Separate domain: facilities owns rooms
CREATE TABLE facilities.rooms (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    capacity INTEGER NOT NULL DEFAULT 30
);
```

**Benefits**:

- Clear data ownership
- Reduces naming collisions (e.g., `users.settings` vs `config.settings`)
- Easier to reason about domain boundaries

### 2. **Cross-Schema Relationships**

Foreign keys can reference tables in other schemas:

```sql
-- education.groups references facilities.rooms (cross-schema FK)
ALTER TABLE education.groups
    ADD CONSTRAINT fk_room
    FOREIGN KEY (room_id) REFERENCES facilities.rooms(id);

-- users.students references education.groups
ALTER TABLE users.students
    ADD CONSTRAINT fk_group
    FOREIGN KEY (group_id) REFERENCES education.groups(id);
```

**Critical**: Cross-schema FKs enforce referential integrity across domain
boundaries.

### 3. **Schema Migration Strategy**

Migrations create schemas before tables:

```go
// Migration: 000000_create_schemas.go
const (
    Version     = "0.0.0"
    Description = "Create PostgreSQL schemas"
)

func init() {
    Migrations.MustRegister(
        func(ctx context.Context, db *bun.DB) error {
            schemas := []string{
                "auth", "users", "education", "schedule",
                "activities", "facilities", "iot", "feedback",
                "active", "config", "audit",
            }
            for _, schema := range schemas {
                _, err := db.ExecContext(ctx, fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS %s", schema))
                if err != nil {
                    return err
                }
            }
            return nil
        },
        func(ctx context.Context, db *bun.DB) error {
            // Rollback: Drop schemas CASCADE
            schemas := []string{...}
            for _, schema := range schemas {
                _, err := db.ExecContext(ctx, fmt.Sprintf("DROP SCHEMA IF EXISTS %s CASCADE", schema))
                if err != nil {
                    return err
                }
            }
            return nil
        },
    )
}
```

## BUN ORM Schema-Qualified Queries

### Critical Pattern: Quoted Aliases

**MANDATORY**: All BUN queries on schema-qualified tables MUST use quoted
aliases.

```go
// ✅ CORRECT - Quotes prevent column resolution errors
query := r.db.NewSelect().
    Model(&groups).
    ModelTableExpr(`education.groups AS "group"`)  // CRITICAL!

// ❌ WRONG - Will cause "column not found" errors
ModelTableExpr(`education.groups AS group`)  // Missing quotes = BUG
```

### Why Quotes Are Required

BUN ORM needs explicit table aliases to resolve columns when multiple tables are
involved:

```go
// Without quotes, BUN doesn't know "group" is an alias
// It tries to find a column named "group" instead of the table

// With quotes, BUN correctly identifies "group" as the table alias
// SELECT "group".id, "group".name FROM education.groups AS "group"
```

### Implementing BeforeAppendModel Hook

Models should implement `BeforeAppendModel` to automatically set the correct
table expression:

```go
// models/education/group.go
func (g *Group) BeforeAppendModel(ctx context.Context, query schema.Query) error {
    switch q := query.(type) {
    case *bun.SelectQuery:
        q.ModelTableExpr(`education.groups AS "group"`)
    case *bun.UpdateQuery:
        q.ModelTableExpr(`education.groups AS "group"`)
    case *bun.DeleteQuery:
        q.ModelTableExpr(`education.groups AS "group"`)
    }
    return nil
}
```

**Benefits**:

- Automatic table expression for all queries
- Developers can't forget to add it
- Consistent across all operations

### Cross-Schema Joins

```go
// Join across schemas with explicit table prefixes
type Result struct {
    Group *education.Group `bun:"group"`
    Room  *facilities.Room  `bun:"room"`
}

err := db.NewSelect().
    Model(&result).
    ModelTableExpr(`education.groups AS "group"`).
    Join(`LEFT JOIN facilities.rooms AS "room" ON "room".id = "group".room_id`).
    Scan(ctx)
```

## Schema Access Patterns

### High-Traffic Schemas

**active schema** (real-time tracking):

- Heavy write load (check-ins, check-outs every few seconds)
- Frequent reads (supervisor dashboards)
- Indexes critical: `idx_active_groups_education_group_id`,
  `idx_active_visits_student_id`

**Optimization**:

- Partitioning by date (future improvement)
- Separate read replicas (future scaling)

### Low-Traffic Schemas

**config schema** (system settings):

- Rarely written
- Cached in application memory
- No special indexing needed

## Future Scalability

### Phase 1: Read Replicas

```
Primary (Write):
  ├── auth.*
  ├── users.*
  ├── education.*
  ├── facilities.*
  ├── activities.*
  └── active.* (high write volume)

Replica (Read):
  ├── Reporting queries
  ├── Dashboard analytics
  └── Historical data access
```

### Phase 2: Schema Separation

Move high-traffic schemas to dedicated databases:

```
Database 1 (Primary):
  ├── auth.*
  ├── users.*
  ├── education.*
  └── facilities.*

Database 2 (Real-Time):
  └── active.* (moved to separate DB)

Database 3 (Audit):
  └── audit.* (compliance logs, long retention)
```

**Challenge**: Cross-database joins not possible → Use API calls or
denormalization.

### Phase 3: Horizontal Sharding

Shard high-volume tables by tenant or date:

```
active.visits_2025_01  # January 2025
active.visits_2025_02  # February 2025
...
```

## Schema-Level Permissions (Future)

PostgreSQL supports schema-level access control:

```sql
-- Grant schema access to specific roles
GRANT USAGE ON SCHEMA education TO teacher_role;
GRANT SELECT ON ALL TABLES IN SCHEMA education TO teacher_role;

-- Revoke access to sensitive schemas
REVOKE ALL ON SCHEMA audit FROM teacher_role;
GRANT SELECT ON SCHEMA audit TO admin_role;
```

**Not currently implemented** but architecture supports it.

## Backup & Restore Strategy

### Single-Database Backup

```bash
# Full database backup (all schemas)
pg_dump -h localhost -U postgres -d postgres > backup_full.sql

# Schema-specific backup
pg_dump -h localhost -U postgres -d postgres -n education > backup_education.sql

# Restore specific schema
psql -h localhost -U postgres -d postgres < backup_education.sql
```

### GDPR Compliance

**audit schema** tracks all data deletions:

```sql
CREATE TABLE audit.data_deletions (
    id BIGSERIAL PRIMARY KEY,
    entity_type VARCHAR(255) NOT NULL,   -- "Student", "Visit", etc.
    entity_id BIGINT NOT NULL,
    deleted_by BIGINT NOT NULL,          -- Account ID
    deleted_at TIMESTAMP DEFAULT NOW(),
    reason TEXT
);
```

**Retention policy**:

- `active.visits`: Deleted after student-specific retention period (1-31 days,
  default 30)
- `audit.data_deletions`: Retained indefinitely for compliance

## Best Practices

### DO ✅

1. **Always use quoted aliases** in BUN ORM queries
2. **Implement BeforeAppendModel** for schema-qualified tables
3. **Use schema prefix** in all table references (`education.groups`, not
   `groups`)
4. **Create indexes** on cross-schema foreign keys
5. **Document schema ownership** in migration files

### DON'T ❌

1. **Don't use unquoted aliases** (`AS group` instead of `AS "group"`)
2. **Don't assume default schema** (always specify: `education.groups`)
3. **Don't create tables in public schema** (deprecated, use domain schemas)
4. **Don't use SELECT \*** without table prefix in cross-schema joins
5. **Don't modify existing migrations** (create new migrations instead)

## Monitoring & Observability

### Key Metrics to Track

1. **Schema Size**:

   ```sql
   SELECT schema_name, pg_size_pretty(SUM(pg_total_relation_size(quote_ident(schemaname) || '.' || quote_ident(tablename)))::bigint)
   FROM pg_tables
   WHERE schemaname NOT IN ('pg_catalog', 'information_schema')
   GROUP BY schema_name
   ORDER BY SUM(pg_total_relation_size(quote_ident(schemaname) || '.' || quote_ident(tablename))) DESC;
   ```

2. **Table Bloat** (active schema concern):

   ```sql
   SELECT schemaname || '.' || tablename AS table,
          pg_size_pretty(pg_total_relation_size(quote_ident(schemaname) || '.' || quote_ident(tablename))) AS size
   FROM pg_tables
   WHERE schemaname = 'active'
   ORDER BY pg_total_relation_size(quote_ident(schemaname) || '.' || quote_ident(tablename)) DESC;
   ```

3. **Index Usage**:
   ```sql
   SELECT schemaname, tablename, indexname, idx_scan
   FROM pg_stat_user_indexes
   WHERE schemaname IN ('auth', 'users', 'education', 'active')
   ORDER BY idx_scan DESC;
   ```

## Summary

**Multi-schema PostgreSQL design provides**:

- ✅ Clear domain separation
- ✅ Single database benefits (ACID, connection pooling)
- ✅ Cross-schema referential integrity
- ✅ Future scalability path (schema → database migration)
- ⚠️ Requires discipline (quoted aliases, schema prefixes)

**Critical Pattern to Remember**:

```go
ModelTableExpr(`education.groups AS "group"`)  // Always quote aliases!
```

---

**See Also**:

- [Entity Relationships](entity-relationships.md) - ER diagrams
- [Migration Strategy](migration-strategy.md) - Version control for schema
- [ADR-002: Multi-Schema Database](../adr/002-multi-schema-database.md) -
  Decision rationale
