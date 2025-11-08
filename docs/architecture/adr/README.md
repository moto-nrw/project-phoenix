# Architecture Decision Records (ADRs)

This directory contains Architecture Decision Records (ADRs) documenting
significant architectural decisions made in Project Phoenix.

## What is an ADR?

An Architecture Decision Record (ADR) captures an important architectural
decision made along with its context and consequences.

### ADR Format

Each ADR follows this structure:

```markdown
# ADR-XXX: [Decision Title]

**Status:** [Proposed | Accepted | Deprecated | Superseded] **Date:** YYYY-MM-DD
**Updated:** YYYY-MM-DD (if applicable) **Deciders:** [Who made this decision]
**Impact:** [Low | Medium | High]

## Context

[What is the issue we're facing that motivates this decision?]

## Decision

[What is the change we're proposing and/or doing?]

## Consequences

[What becomes easier or more difficult due to this change?]

### Positive

[Benefits of this decision]

### Negative

[Drawbacks and challenges]

### Mitigations

[How we address the drawbacks]

## Alternatives Considered

[What other options did we evaluate?]

## Experience Report (Optional)

[After implementation, what did we learn?]

## Related Decisions

[Links to related ADRs]

## References

[External resources, articles, documentation]
```

## ADR Index

### Core Architecture

| ADR                                 | Title                                         | Status   | Impact | Date       |
| ----------------------------------- | --------------------------------------------- | -------- | ------ | ---------- |
| [001](001-factory-pattern.md)       | Factory Pattern for Dependency Injection      | Accepted | High   | 2024-06-09 |
| [002](002-multi-schema-database.md) | Multi-Schema PostgreSQL Database              | Accepted | High   | 2024-06-09 |
| [003](003-bun-orm.md)               | BUN ORM instead of GORM                       | Accepted | High   | 2024-06-09 |
| [006](006-repository-pattern.md)    | Repository Pattern with Interface Segregation | Accepted | High   | 2024-06-09 |

### Security & Authentication

| ADR                            | Title                            | Status   | Impact | Date       |
| ------------------------------ | -------------------------------- | -------- | ------ | ---------- |
| [004](004-jwt-tokens.md)       | JWT with 15-Minute Access Tokens | Accepted | High   | 2024-06-09 |
| [007](007-nextjs-api-proxy.md) | Next.js API Routes as JWT Proxy  | Accepted | High   | 2024-06-09 |

### Real-Time & Communication

| ADR                        | Title                             | Status   | Impact | Date       |
| -------------------------- | --------------------------------- | -------- | ------ | ---------- |
| [005](005-sse-realtime.md) | Server-Sent Events over WebSocket | Accepted | Medium | 2024-06-09 |

## Creating a New ADR

### When to Write an ADR

Write an ADR when making decisions about:

- **Architecture patterns** (e.g., layered architecture, hexagonal architecture)
- **Technology choices** (e.g., database, ORM, framework)
- **Cross-cutting concerns** (e.g., authentication, logging, error handling)
- **Significant design patterns** (e.g., factory pattern, repository pattern)
- **Infrastructure decisions** (e.g., containerization, deployment strategy)
- **Breaking changes** that affect multiple teams/components

### When NOT to Write an ADR

Don't write an ADR for:

- Minor implementation details
- Reversible code-level decisions
- Team process changes (use team documentation instead)
- Obvious technology choices with no alternatives
- Temporary workarounds or experiments

### Process

1. **Propose**: Create ADR with status "Proposed"
2. **Discuss**: Share with team for feedback
3. **Decide**: Update status to "Accepted" or "Rejected"
4. **Implement**: Follow the decision in code
5. **Review**: After 3-6 months, add "Experience Report" section

### Numbering

ADRs are numbered sequentially: `001-factory-pattern.md`,
`002-multi-schema-database.md`, etc.

### File Naming

```
[number]-[short-title].md

Examples:
001-factory-pattern.md
002-multi-schema-database.md
003-bun-orm.md
```

## ADR Lifecycle

### Status Definitions

- **Proposed**: Decision is under discussion
- **Accepted**: Decision is approved and being implemented
- **Deprecated**: Decision is no longer recommended but existing code remains
- **Superseded**: Decision has been replaced by a newer ADR (link to
  replacement)

### Updating ADRs

ADRs should be **immutable** after acceptance. If a decision changes:

1. **Do NOT edit the original ADR**
2. **Create a new ADR** that supersedes the old one
3. **Update the old ADR's status** to "Superseded by ADR-XXX"
4. **Exception**: Adding an "Experience Report" section is allowed

Example:

```markdown
# ADR-003: BUN ORM instead of GORM

**Status:** Superseded by
[ADR-012: Migrate to SQLBoiler](012-migrate-sqlboiler.md)
```

## Key Decisions Summary

### Backend Architecture

**Layered Architecture** (ADR-001, ADR-006):

- API Layer → Service Layer → Repository Layer → Database
- Factory pattern for dependency injection
- Repository pattern for data access abstraction

**Database** (ADR-002, ADR-003):

- PostgreSQL 17+ with 11 schemas
- BUN ORM for type-safe queries
- Multi-schema design for domain separation

**Security** (ADR-004, ADR-007):

- JWT with 15-minute access tokens
- Permission-based authorization
- Next.js API routes as JWT proxy

**Real-Time** (ADR-005):

- Server-Sent Events for supervisor updates
- SSE over WebSocket for simplicity

### Frontend Architecture

**Next.js 15** (ADR-007):

- App Router with Server Components
- API routes proxy all backend calls
- JWT tokens never exposed to client

## Best Practices

### Writing Good ADRs

✅ **DO:**

- Focus on "why" not "what"
- Include alternatives considered
- Document trade-offs explicitly
- Add diagrams/code examples for clarity
- Update with experience after 3-6 months

❌ **DON'T:**

- Write implementation details (use code comments instead)
- Make assumptions without justification
- Skip the "Consequences" section
- Forget to link related ADRs

### Example of Good Context

```markdown
## Context

We needed to organize ~60 tables for different business domains. Options
considered:

1. Single schema with prefixed tables (e.g., `auth_accounts`)
2. Multiple PostgreSQL schemas (e.g., `auth.accounts`)
3. Multiple databases (separate database per domain)

We chose option 2 (multi-schema) because:

- Provides logical separation without distributed transaction complexity
- Supports cross-schema foreign keys for referential integrity
- Maintains single connection pool for simplicity
- Allows future migration to separate databases if needed
```

### Example of Good Consequences

```markdown
## Consequences

### Positive

1. **Domain Separation**: Clear ownership (education.groups → education domain)
2. **ACID Transactions**: Cross-schema FKs enforce referential integrity
3. **Single Connection Pool**: Lower resource usage

### Negative

1. **BUN ORM Pattern**: Must use quoted aliases (easy to forget)
2. **Schema Prefix Overhead**: Every table reference needs schema prefix

### Mitigations

- BeforeAppendModel hook enforces correct table expression
- Documentation emphasizes critical pattern
- Code review checks
```

## References

- [Michael Nygard's ADR Template](https://github.com/joelparkerhenderson/architecture-decision-record)
- [Documenting Architecture Decisions](https://cognitect.com/blog/2011/11/15/documenting-architecture-decisions)
- [ADR GitHub Organization](https://adr.github.io/)

---

**Maintainer**: Project Phoenix Team **Last Updated**: 2025-10-19
