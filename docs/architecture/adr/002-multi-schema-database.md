# ADR-002: Multi-Schema PostgreSQL Database

**Status:** Accepted
**Date:** 2024-06-09
**Updated:** 2025-10-19
**Deciders:** Backend Team, Database Team
**Impact:** High

## Context

We needed to organize ~60 tables for different business domains (auth, users, education, active tracking, IoT devices, etc.). Options considered:

1. **Single schema with prefixed tables** (e.g., `auth_accounts`, `users_students`)
2. **Multiple PostgreSQL schemas** (e.g., `auth.accounts`, `users.students`)
3. **Multiple databases** (separate database per domain)
4. **Microservices** (separate service + database per domain)

## Decision

**Use multiple PostgreSQL schemas within a single database**, with one schema per domain.

### Schema Organization

```
postgres (database)
├── auth           # Authentication & authorization
├── users          # Person hierarchy (staff, teachers, students)
├── education      # Groups, substitutions
├── facilities     # Rooms, buildings
├── activities     # Student activities
├── active         # Real-time session tracking (HIGH TRAFFIC)
├── schedule       # Time management
├── iot            # RFID devices
├── feedback       # User feedback
├── config         # System configuration
└── audit          # GDPR compliance logs
```

### Implementation

**Schema Creation** (Migration `000000_create_schemas.go`):
```sql
CREATE SCHEMA IF NOT EXISTS auth;
CREATE SCHEMA IF NOT EXISTS users;
CREATE SCHEMA IF NOT EXISTS education;
CREATE SCHEMA IF NOT EXISTS facilities;
CREATE SCHEMA IF NOT EXISTS activities;
CREATE SCHEMA IF NOT EXISTS active;
CREATE SCHEMA IF NOT EXISTS schedule;
CREATE SCHEMA IF NOT EXISTS iot;
CREATE SCHEMA IF NOT EXISTS feedback;
CREATE SCHEMA IF NOT EXISTS config;
CREATE SCHEMA IF NOT EXISTS audit;
```

**Table Example**:
```sql
-- education.groups table
CREATE TABLE education.groups (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    room_id BIGINT REFERENCES facilities.rooms(id),  -- Cross-schema FK
    created_at TIMESTAMP DEFAULT NOW()
);

-- facilities.rooms table
CREATE TABLE facilities.rooms (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    capacity INTEGER NOT NULL DEFAULT 30
);
```

**BUN ORM Critical Pattern**:
```go
// ✅ CORRECT - Quoted aliases prevent column resolution errors
query := r.db.NewSelect().
    Model(&groups).
    ModelTableExpr(`education.groups AS "group"`)  // CRITICAL: Quotes required!

// ❌ WRONG - Missing quotes causes runtime errors
ModelTableExpr(`education.groups AS group`)  // NO QUOTES = BUG
```

## Alternatives Considered

### Option 1: Single Schema with Prefixes

```sql
-- All tables in public schema with prefixes
CREATE TABLE auth_accounts (...);
CREATE TABLE users_students (...);
CREATE TABLE education_groups (...);
```

**Pros:**
- Simple to query (no schema qualification)
- Works with any ORM
- Familiar pattern

**Cons:**
- ❌ No logical separation
- ❌ Naming collisions (e.g., `users_settings` vs `config_settings`)
- ❌ Harder to reason about domain boundaries
- ❌ Can't set schema-level permissions
- ❌ Difficult to move to separate databases later

**Decision:** Rejected - lacks domain separation

### Option 2: Multiple Databases

```
database_auth      → auth tables
database_users     → user tables
database_education → education tables
```

**Pros:**
- ✅ Complete isolation
- ✅ Database-level permissions
- ✅ Easier to scale horizontally

**Cons:**
- ❌ Cross-database joins not supported
- ❌ Distributed transactions complex
- ❌ Multiple connection pools (resource overhead)
- ❌ More complex backup/restore
- ❌ Overkill for current scale

**Decision:** Rejected - too much overhead for single-server deployment

### Option 3: Microservices (Service per Domain)

```
auth-service       → auth database
users-service      → users database
education-service  → education database
```

**Pros:**
- ✅ Complete domain isolation
- ✅ Independent scaling
- ✅ Team ownership per service

**Cons:**
- ❌ Massive architectural complexity
- ❌ Network latency between services
- ❌ Distributed transactions nightmare
- ❌ Much higher operational burden
- ❌ Team too small to manage multiple services

**Decision:** Rejected - team size (2-3 developers) doesn't justify microservices

## Comparison Matrix

| Aspect | Single Schema | Multi-Schema (CHOSEN) | Multi-Database | Microservices |
|--------|--------------|----------------------|----------------|---------------|
| **Domain Separation** | ❌ Weak | ✅ Strong | ✅ Complete | ✅ Complete |
| **Transactions** | ✅ Simple | ✅ Simple | ❌ Distributed | ❌ Distributed |
| **Joins** | ✅ Easy | ✅ Supported | ❌ Not possible | ❌ Not possible |
| **Connection Pool** | ✅ Single | ✅ Single | ❌ Multiple | ❌ Many |
| **Backup/Restore** | ✅ Simple | ✅ Simple | ⚠️ Complex | ❌ Very complex |
| **Operational Overhead** | ✅ Low | ✅ Low | ⚠️ Medium | ❌ High |
| **Migration Path** | ❌ Difficult | ✅ Easy | ✅ N/A | ✅ N/A |
| **Performance** | ✅ Fast | ✅ Fast | ⚠️ Network latency | ❌ Network latency |

## Consequences

### Positive

1. **Domain-Driven Organization**
   - Clear ownership: `education.groups` → education domain
   - Reduces naming collisions
   - Easier to reason about domain boundaries

2. **ACID Transactions**
   - Cross-schema foreign keys enforce referential integrity
   - Transactions work across all schemas
   - No distributed transaction complexity

3. **Single Connection Pool**
   - Lower resource usage
   - Simpler configuration
   - Better connection utilization

4. **Future Migration Path**
   - Can move `active` schema to separate database (high write volume)
   - Can move `audit` schema to separate database (compliance logs)
   - Schema → database migration straightforward

5. **Schema-Level Permissions (Future)**
   ```sql
   GRANT USAGE ON SCHEMA education TO teacher_role;
   GRANT SELECT ON ALL TABLES IN SCHEMA education TO teacher_role;
   REVOKE ALL ON SCHEMA audit FROM teacher_role;
   ```

### Negative

1. **BUN ORM Quoted Aliases Requirement**
   - **CRITICAL**: All queries MUST use `ModelTableExpr` with quoted aliases
   - Easy to forget → runtime errors
   - Developers must remember this pattern

   ```go
   // CORRECT
   ModelTableExpr(`education.groups AS "group"`)

   // WRONG - Will fail
   ModelTableExpr(`education.groups AS group`)
   ```

2. **Schema Prefix Overhead**
   - Every table reference needs schema prefix
   - Slightly more verbose SQL
   - Migration files must specify schema

3. **Cross-Schema Queries More Complex**
   ```sql
   -- Requires explicit JOIN across schemas
   SELECT *
   FROM education.groups AS "group"
   LEFT JOIN facilities.rooms AS "room" ON "room".id = "group".room_id;
   ```

4. **ORM Support Required**
   - Not all ORMs handle multi-schema well
   - BUN ORM specifically chosen for this reason

### Mitigations

**For BUN ORM Pattern**:
- `BeforeAppendModel` hook enforces correct table expression
- Documentation emphasizes critical pattern
- Code review checks

**For Schema Prefix Overhead**:
- Accepted as minor trade-off for domain clarity
- BUN ORM abstracts most of this away

**For Cross-Schema Query Complexity**:
- Use ORM eager loading: `Relation("Room")`
- Repository layer handles complexity

## Experience Report

**After 6 months:**

✅ **What worked well:**
- Domain separation made codebase easier to navigate
- Cross-schema foreign keys prevented many bugs
- Single connection pool simplified operations
- Migration from schema to database is clear path

⚠️ **What was challenging:**
- Developers occasionally forgot quoted aliases (caught by tests)
- BUN ORM learning curve for multi-schema queries
- Initial setup more complex than single schema

❌ **What failed:**
- One developer created table in `public` schema (migration rejected)
- Forgot to add schema to new migration (caught by migration validator)

**Would we do it again?** ✅ **Yes**

## Future Scalability

### Phase 1: Read Replicas (0-6 months)
```
Primary (Write)     → All schemas
Replica (Read-Only) → All schemas (analytics, reporting)
```

### Phase 2: High-Traffic Schema Separation (6-12 months)
```
Database 1 (Primary)    → auth, users, education, facilities
Database 2 (Real-Time)  → active (high write volume)
Database 3 (Audit)      → audit (compliance logs, long retention)
```

**Challenge**: Cross-database joins not possible
**Solution**: Denormalize or use API calls

### Phase 3: Horizontal Sharding (12+ months)
```
active.visits_2025_01  # January 2025 visits
active.visits_2025_02  # February 2025 visits
...
```

## Related Decisions

- [ADR-003: BUN ORM](003-bun-orm.md) - ORM choice influenced by multi-schema support
- [ADR-006: Repository Pattern](006-repository-pattern.md) - Abstracts schema complexity
- [Database Schema Design](../database/schema-design.md) - Detailed schema documentation

## References

- [PostgreSQL Schemas](https://www.postgresql.org/docs/current/ddl-schemas.html)
- [BUN Multi-Schema Support](https://bun.uptrace.dev/)
- [Domain-Driven Design](https://martinfowler.com/bliki/BoundedContext.html)
