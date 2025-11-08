# Project Phoenix - Architecture Overview

**Version:** 4.5.x **Last Updated:** 2025-10-19 **Status:** Production

## Table of Contents

- [Introduction](#introduction)
- [Architectural Style](#architectural-style)
- [High-Level Architecture](#high-level-architecture)
- [Key Design Principles](#key-design-principles)
- [Domain Organization](#domain-organization)
- [Technology Stack](#technology-stack)
- [Architectural Decisions](#architectural-decisions)
- [Documentation Structure](#documentation-structure)

## Introduction

Project Phoenix is a GDPR-compliant RFID-based student attendance and room
management system for educational institutions. The architecture emphasizes:

- **Security First**: JWT-based authentication, GDPR compliance, SSL encryption
- **Type Safety**: Go strict typing + TypeScript strict mode
- **Domain-Driven Design**: 11 distinct business domains with clear boundaries
- **Testability**: Factory pattern + repository interfaces enable comprehensive
  testing
- **Performance**: Optimized queries, connection pooling, real-time SSE updates
- **Maintainability**: Explicit dependencies, clear separation of concerns

## Architectural Style

### Layered Hexagonal Architecture

```
┌──────────────────────────────────────────────────────────────┐
│                 Presentation Layer (Frontend)                 │
│  Next.js 15 + React 19 + TypeScript + Tailwind CSS          │
│  - App Router (33 pages)                                     │
│  - 124 API route handlers (proxy layer)                      │
│  - Server Components + Client Components                     │
│  - Real-time SSE integration                                 │
└────────────────────────┬─────────────────────────────────────┘
                         │ HTTPS/JSON
                         ↓
┌──────────────────────────────────────────────────────────────┐
│                   API Layer (Backend)                         │
│  Go 1.23+ with Chi Router                                    │
│  - HTTP handlers (11 domain-based routers)                   │
│  - Middleware (Auth, CORS, Rate Limiting, Security Headers)  │
│  - Request/Response mapping                                  │
│  - Permission checks                                         │
└────────────────────────┬─────────────────────────────────────┘
                         │
                         ↓
┌──────────────────────────────────────────────────────────────┐
│                     Service Layer                             │
│  - Business logic orchestration                              │
│  - Transaction management                                    │
│  - Domain rule enforcement                                   │
│  - Cross-domain coordination                                 │
└────────────────────────┬─────────────────────────────────────┘
                         │
                         ↓
┌──────────────────────────────────────────────────────────────┐
│                   Repository Layer                            │
│  - Data access abstraction (interfaces in models/)           │
│  - BUN ORM query building                                    │
│  - Schema-qualified SQL queries                              │
│  - Transaction support                                       │
└────────────────────────┬─────────────────────────────────────┘
                         │
                         ↓
┌──────────────────────────────────────────────────────────────┐
│              PostgreSQL 17+ Multi-Schema Database             │
│  auth | users | education | facilities | activities |        │
│  active | schedule | iot | feedback | config | audit         │
└──────────────────────────────────────────────────────────────┘
```

### Domain-Driven Design

**11 Bounded Contexts**:

| Domain         | Schema       | Purpose                        | Key Entities                                  |
| -------------- | ------------ | ------------------------------ | --------------------------------------------- |
| **Auth**       | `auth`       | Authentication & Authorization | Accounts, Tokens, Roles, Permissions          |
| **Users**      | `users`      | Person Management              | Persons, Staff, Teachers, Students, Guardians |
| **Education**  | `education`  | Educational Structures         | Groups, Substitutions, Assignments            |
| **Facilities** | `facilities` | Physical Spaces                | Rooms, Buildings, Locations                   |
| **Activities** | `activities` | Student Activities             | Categories, Enrollments, Schedules            |
| **Active**     | `active`     | Real-time Tracking             | Sessions, Visits, Supervisors, Attendance     |
| **Schedule**   | `schedule`   | Time Management                | Timeframes, Dateframes, Recurrence            |
| **IoT**        | `iot`        | Device Management              | RFID Devices, Tags, API Keys                  |
| **Feedback**   | `feedback`   | User Feedback                  | Entries, Comments, Ratings                    |
| **Config**     | `config`     | System Settings                | Settings, Preferences, Feature Flags          |
| **Audit**      | `audit`      | GDPR Compliance                | Data Deletions, Auth Events, Change Logs      |

## High-Level Architecture

### Backend Flow (Request Lifecycle)

```
1. HTTP Request (RFID device, browser, API client)
   ↓
2. Chi Router (api/base.go)
   - Route matching: /api/groups, /api/students, /iot/checkin, etc.
   ↓
3. Middleware Chain
   - JWT Verifier → Extract token
   - Authenticator → Validate signature, inject claims into context
   - Permission Check → RequiresPermission("groups:read")
   - Rate Limiter (optional) → Protect against abuse
   - Security Headers → CSP, HSTS, X-Frame-Options
   ↓
4. API Handler (api/{domain}/api.go)
   - Parse request (query params, path params, body)
   - Validate input with ozzo-validation
   - Build QueryOptions
   ↓
5. Service Layer (services/{domain}/)
   - Enforce business rules
   - Orchestrate multi-repository operations
   - Manage transactions with txHandler
   ↓
6. Repository Layer (database/repositories/{domain}/)
   - Build BUN ORM queries
   - Apply filters, pagination, sorting
   - Execute schema-qualified queries (MUST use quoted aliases!)
   ↓
7. BUN ORM → PostgreSQL
   - Generate SQL with prepared statements
   - Execute query with connection pooling
   ↓
8. Response Mapping
   - Map entities to DTOs
   - Load eager relationships
   - Wrap in standard response format
   ↓
9. HTTP Response
   {
     "status": "success",
     "data": [...],
     "message": "Groups retrieved successfully"
   }
```

### Frontend Flow (Component → Backend)

```
1. User Interaction (Click "Create Group")
   ↓
2. Client Component (components/groups/group-form.tsx)
   - React state management (useState)
   - Form validation
   ↓
3. fetch("/api/groups", { method: "POST", body: JSON.stringify(data) })
   ↓
4. Next.js API Route Handler (app/api/groups/route.ts)
   - Extract JWT from session (server-side only!)
   - Parse request body
   - Call route wrapper (createPostHandler)
   ↓
5. Route Wrapper (lib/route-wrapper.ts)
   - auth() → Get NextAuth session
   - Extract token from session.user.token
   - Handle Next.js 15 async params (await context.params)
   - Retry on 401 with token refresh
   ↓
6. API Client (lib/api-client.ts)
   - Axios POST with Bearer token
   - apiPost(`${API_URL}/api/groups`, token, body)
   ↓
7. Backend API (Go Chi Router)
   - Middleware validates JWT
   - Handler processes request
   ↓
8. Response Mapping
   - Backend returns snake_case + int64
   - Frontend maps to camelCase + string
   ↓
9. UI Update
   - Component receives response
   - Updates React state
   - Re-renders UI
```

## Key Design Principles

### 1. Explicit Over Implicit

**Factory Pattern for Dependency Injection**:

```go
// All dependencies explicitly wired
serviceFactory := services.NewFactory(repoFactory, db)
authService := serviceFactory.NewAuthService()
```

**Benefits**:

- No magic (no reflection-based DI)
- Compile-time safety (missing dependencies = build errors)
- Testability (easy to inject mocks)

### 2. Security by Default

**Permission-Based Authorization**:

```go
r.With(authorize.RequiresPermission(permissions.GroupsRead)).Get("/", rs.listGroups)
```

**JWT Token Isolation**:

- Frontend: Tokens never exposed to client (stored in NextAuth session)
- Backend: Every protected endpoint validates JWT signature + expiry

### 3. Type Safety End-to-End

**Backend (Go)**:

- Static typing with interfaces
- BUN ORM type-safe queries
- Compile-time error detection

**Frontend (TypeScript)**:

- Strict mode enabled
- Type mapping helpers for backend responses
- Zod schemas for runtime validation

### 4. Clear Boundaries

**Repository Pattern**:

```go
// Interface in models/ (domain logic doesn't know about DB)
type GroupRepository interface {
    FindByID(ctx context.Context, id int64) (*Group, error)
    ListWithOptions(ctx context.Context, options *base.QueryOptions) ([]*Group, error)
}

// Implementation in database/repositories/ (DB-specific)
type GroupRepository struct {
    db *bun.DB
}
```

### 5. Transactional Consistency

**Context-Based Transaction Propagation**:

```go
err := s.txHandler.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
    // All operations within this closure use the same transaction
    return txService.DeleteGroup(ctx, id)
})
```

## Technology Stack

### Backend

- **Language**: Go 1.23+
- **Router**: Chi v5 (lightweight, composable)
- **ORM**: BUN (fast, type-safe, PostgreSQL-native)
- **Database**: PostgreSQL 17+ with SSL encryption
- **Auth**: JWT with refresh tokens (Argon2id password hashing)
- **Validation**: ozzo-validation v4

### Frontend

- **Framework**: Next.js 15+ (App Router)
- **UI Library**: React 19+ (Server Components + Client Components)
- **Language**: TypeScript 5+ (strict mode)
- **Styling**: Tailwind CSS 4+
- **Authentication**: NextAuth.js with JWT strategy
- **HTTP Client**: Axios

### Infrastructure

- **Containerization**: Docker + Docker Compose
- **Database**: PostgreSQL 17-alpine
- **SSL**: Self-signed certs (dev), CA-signed (production)
- **Logging**: Structured JSON logging
- **Monitoring**: Health checks, metrics endpoints (future: Prometheus)

## Architectural Decisions

**Critical ADRs** (See `/docs/architecture/adr/` for details):

| ADR                                         | Decision                     | Rationale                                                    |
| ------------------------------------------- | ---------------------------- | ------------------------------------------------------------ |
| [ADR-001](adr/001-factory-pattern.md)       | Use Factory Pattern for DI   | Explicit wiring, compile-time safety, no reflection overhead |
| [ADR-002](adr/002-multi-schema-database.md) | Multi-Schema PostgreSQL      | Domain isolation, clear data ownership, future scalability   |
| [ADR-003](adr/003-bun-orm.md)               | BUN ORM instead of GORM      | Better performance, fewer allocations, multi-schema support  |
| [ADR-004](adr/004-jwt-tokens.md)            | JWT with 15min access tokens | Balance security vs UX, automatic refresh every 4 minutes    |
| [ADR-005](adr/005-sse-realtime.md)          | SSE over WebSocket           | Simpler protocol, auto-reconnect, one-way sufficient         |
| [ADR-006](adr/006-repository-pattern.md)    | Repository Pattern           | Testability, database agnostic, clear separation             |
| [ADR-007](adr/007-nextjs-api-proxy.md)      | Next.js API Routes as Proxy  | JWT never exposed to client, CORS simplified                 |

## Documentation Structure

```
docs/architecture/
├── 00-OVERVIEW.md                    # This document
├── c4-diagrams/                       # Visual architecture diagrams
│   ├── 01-system-context.puml        # Level 1: System boundaries
│   ├── 02-container-diagram.puml     # Level 2: High-level containers
│   ├── 03-component-backend.puml     # Level 3: Backend components
│   ├── 04-component-frontend.puml    # Level 3: Frontend components
│   └── 05-code-flows.puml            # Level 4: Key sequence diagrams
├── database/                          # Database architecture
│   ├── schema-design.md              # Multi-schema design rationale
│   ├── entity-relationships.md       # ER diagrams and relationships
│   └── migration-strategy.md         # Migration system documentation
├── adr/                               # Architecture Decision Records
│   ├── 001-factory-pattern.md
│   ├── 002-multi-schema-database.md
│   ├── 003-bun-orm.md
│   ├── 004-jwt-tokens.md
│   ├── 005-sse-realtime.md
│   ├── 006-repository-pattern.md
│   └── 007-nextjs-api-proxy.md
├── security/                          # Security architecture
│   ├── authentication-flow.md        # JWT lifecycle
│   ├── authorization-model.md        # Permission system
│   └── gdpr-compliance.md            # GDPR implementation
├── api-flows/                         # Key API workflows
│   ├── student-checkin-flow.md
│   ├── session-management-flow.md
│   ├── group-creation-flow.md
│   └── data-cleanup-flow.md
└── deployment/                        # Infrastructure documentation
    ├── docker-architecture.md
    ├── ssl-configuration.md
    └── production-deployment.md
```

## Key Architectural Strengths

1. **Clear Separation of Concerns**: Each layer has a single responsibility
2. **Domain-Driven Design**: Business domains drive code organization
3. **Type Safety**: Go + TypeScript eliminate entire classes of bugs
4. **Testability**: Factory pattern + interfaces = easy mocking
5. **Security by Default**: Permission checks at route level, JWT validation
6. **GDPR Compliance**: Audit logging + automated data cleanup
7. **Multi-Schema Isolation**: Clear data ownership, future scaling path
8. **Real-Time Capabilities**: SSE for live supervisor updates

## Key Architectural Challenges

1. **BUN ORM Quoted Aliases**: Easy to forget `ModelTableExpr` quotes (runtime
   errors)
2. **Docker Rebuild Friction**: Go code changes require container rebuild
3. **Factory Boilerplate**: Manual wiring verbose (trade-off for explicitness)
4. **In-Memory SSE Hub**: Doesn't scale horizontally (needs Redis for
   multi-server)
5. **Type Mapping Overhead**: Backend → Frontend transformation adds complexity

## Next Steps

For detailed architecture information:

1. **Start with C4 Diagrams** → [c4-diagrams/](c4-diagrams/) - Visual
   architecture overview
2. **Understand Key Decisions** → [adr/](adr/) - Why we chose certain patterns
3. **Database Deep Dive** → [database/](database/) - Multi-schema design
4. **Security Details** → [security/](security/) - Auth, permissions, GDPR
5. **API Workflows** → [api-flows/](api-flows/) - Request flow documentation
6. **Deployment Guide** → [deployment/](deployment/) - Infrastructure setup

## Revision History

| Version | Date       | Changes                                    |
| ------- | ---------- | ------------------------------------------ |
| 1.0     | 2025-10-19 | Initial architecture documentation created |

---

**Maintainers**: Project Phoenix Team **Last Review**: 2025-10-19 **Next
Review**: 2025-11-19
