# Project Phoenix - Architecture Documentation

**Comprehensive architecture documentation for Project Phoenix, a GDPR-compliant
RFID-based student attendance system.**

## 📚 Documentation Structure

```
docs/architecture/
├── 00-OVERVIEW.md              # Start here - High-level architecture overview
├── c4-diagrams/                 # Visual C4 model diagrams (PlantUML)
│   ├── 01-system-context.puml   # Level 1: System boundaries and external actors
│   ├── 02-container-diagram.puml # Level 2: Frontend, Backend, Database containers
│   ├── 03-component-backend.puml # Level 3: Backend component details
│   ├── 04-component-frontend.puml # Level 3: Frontend component details
│   └── 05-sequence-flows.puml   # Level 4: Key sequence diagrams
├── database/                    # Database architecture
│   ├── schema-design.md         # Multi-schema PostgreSQL design rationale
│   ├── entity-relationships.md  # ER diagrams (TODO)
│   └── migration-strategy.md    # Migration system (TODO)
├── adr/                         # Architecture Decision Records
│   ├── README.md                # ADR format and index
│   ├── 001-factory-pattern.md   # Dependency injection pattern
│   ├── 002-multi-schema-database.md # Database organization
│   ├── 003-bun-orm.md           # ORM choice (TODO)
│   ├── 004-jwt-tokens.md        # Authentication strategy (TODO)
│   ├── 005-sse-realtime.md      # Real-time updates (TODO)
│   ├── 006-repository-pattern.md # Data access pattern (TODO)
│   └── 007-nextjs-api-proxy.md  # Frontend security (TODO)
├── security/                    # Security architecture
│   ├── authentication-flow.md   # JWT authentication details
│   ├── authorization-model.md   # Permission system (TODO)
│   └── gdpr-compliance.md       # GDPR implementation (TODO)
├── api-flows/                   # Key API workflows
│   ├── student-checkin-flow.md  # RFID check-in process (TODO)
│   ├── session-management-flow.md # Group session lifecycle (TODO)
│   └── data-cleanup-flow.md     # GDPR data cleanup (TODO)
└── deployment/                  # Infrastructure
    ├── docker-architecture.md   # Container architecture (TODO)
    └── ssl-configuration.md     # SSL/TLS setup (TODO)
```

## 🚀 Quick Start Guide

### For New Developers

**Read in this order:**

1. **[00-OVERVIEW.md](00-OVERVIEW.md)** (15 min)
   - Understand the high-level architecture
   - See the layered design (API → Service → Repository → Database)
   - Review key design principles

2. **[C4 Diagrams](c4-diagrams/)** (15 min)
   - Start with `01-system-context.puml` for the big picture
   - Move to `02-container-diagram.puml` for container details
   - Dive into `03-component-backend.puml` and `04-component-frontend.puml` for
     component-level understanding
   - Review `05-sequence-flows.puml` for key workflows

3. **[ADR Index](adr/README.md)** (30 min)
   - Read ADRs for decisions that affect your work:
     - Backend dev: ADR-001 (Factory), ADR-002 (Multi-Schema), ADR-003 (BUN ORM)
     - Frontend dev: ADR-007 (Next.js Proxy), ADR-005 (SSE)
     - Full-stack: ADR-004 (JWT), ADR-006 (Repository Pattern)

4. **[Database Schema Design](database/schema-design.md)** (20 min)
   - Understand the 11 PostgreSQL schemas
   - Learn the critical BUN ORM pattern (quoted aliases!)
   - Review cross-schema relationships

5. **[Authentication Flow](security/authentication-flow.md)** (20 min)
   - Understand JWT token lifecycle
   - Learn how NextAuth integrates with the backend
   - Review security best practices

**Total time: ~2 hours**

### For Architects

**Focus areas:**

- **[00-OVERVIEW.md](00-OVERVIEW.md)** - Architectural style and design
  principles
- **[ADR Directory](adr/)** - All architectural decisions with rationale
- **[C4 Diagrams](c4-diagrams/)** - Visual architecture at multiple levels
- **[Database Schema Design](database/schema-design.md)** - Data architecture
  and scaling path

### For Security Auditors

**Security-focused docs:**

- **[Authentication Flow](security/authentication-flow.md)** - JWT
  implementation
- **[Authorization Model](security/authorization-model.md)** (TODO) - Permission
  system
- **[GDPR Compliance](security/gdpr-compliance.md)** (TODO) - Data protection
  measures
- **[ADR-004: JWT Tokens](adr/004-jwt-tokens.md)** (TODO) - Security decisions

## 📊 Visual Architecture Overview

### System Context (Level 1)

```
[Users: Teachers, Admins, Guardians]
           ↓
    [Project Phoenix]
           ↓
  [PostgreSQL Database]

[RFID Devices] → [Project Phoenix]
```

### Container Diagram (Level 2)

```
┌─────────────────────────────────────────┐
│  Frontend (Next.js 15 + React 19)       │
│  - 33 pages (App Router)                │
│  - 124 API route handlers               │
│  - Real-time SSE dashboard              │
└──────────────┬──────────────────────────┘
               │ HTTPS/JSON (JWT)
               ↓
┌─────────────────────────────────────────┐
│  Backend (Go 1.23+ + Chi + BUN ORM)     │
│  - 11 domain-based routers              │
│  - JWT authentication + permissions     │
│  - Real-time SSE hub                    │
│  - RFID device integration              │
└──────────────┬──────────────────────────┘
               │ PostgreSQL (SSL)
               ↓
┌─────────────────────────────────────────┐
│  PostgreSQL 17+ Multi-Schema            │
│  auth | users | education | facilities │
│  active | schedule | iot | audit | ...  │
└─────────────────────────────────────────┘
```

### Backend Components (Level 3)

```
HTTP Request
    ↓
Chi Router (api/base.go)
    ↓
Middleware (JWT, Permissions, Security)
    ↓
API Handlers (api/{domain}/api.go)
    ↓
Service Layer (services/{domain}/)
    ↓
Repository Layer (database/repositories/{domain}/)
    ↓
BUN ORM (schema-qualified queries)
    ↓
PostgreSQL (11 schemas)
```

## 🎯 Key Architectural Decisions

### Backend Architecture

| Decision                                                                  | Rationale                                           | Status      |
| ------------------------------------------------------------------------- | --------------------------------------------------- | ----------- |
| **Factory Pattern for DI** ([ADR-001](adr/001-factory-pattern.md))        | Explicit wiring, compile-time safety, no reflection | ✅ Accepted |
| **Multi-Schema PostgreSQL** ([ADR-002](adr/002-multi-schema-database.md)) | Domain separation, single DB benefits, scaling path | ✅ Accepted |
| **BUN ORM** ([ADR-003](adr/003-bun-orm.md))                               | Type safety, performance, multi-schema support      | ✅ Accepted |
| **Repository Pattern** ([ADR-006](adr/006-repository-pattern.md))         | Data access abstraction, testability                | ✅ Accepted |

### Security & Authentication

| Decision                                                               | Rationale                   | Status      |
| ---------------------------------------------------------------------- | --------------------------- | ----------- |
| **JWT (15min access, 1hr refresh)** ([ADR-004](adr/004-jwt-tokens.md)) | Balance security vs UX      | ✅ Accepted |
| **Next.js API Proxy** ([ADR-007](adr/007-nextjs-api-proxy.md))         | JWT never exposed to client | ✅ Accepted |

### Real-Time Communication

| Decision                                                    | Rationale                                            | Status      |
| ----------------------------------------------------------- | ---------------------------------------------------- | ----------- |
| **SSE over WebSocket** ([ADR-005](adr/005-sse-realtime.md)) | Simpler protocol, auto-reconnect, one-way sufficient | ✅ Accepted |

## 🔑 Critical Patterns to Remember

### 1. BUN ORM Quoted Aliases (MANDATORY!)

```go
// ✅ CORRECT - Quotes prevent "column not found" errors
query := r.db.NewSelect().
    Model(&groups).
    ModelTableExpr(`education.groups AS "group"`)

// ❌ WRONG - Will fail at runtime
ModelTableExpr(`education.groups AS group`)  // Missing quotes = BUG
```

### 2. Docker Backend Rebuild

```bash
# CRITICAL: Go code changes require rebuild
docker compose build server
docker compose up -d server
```

### 3. Type Mapping (Frontend ↔ Backend)

```typescript
// Backend: snake_case, int64
interface BackendGroup {
  id: number;
  room_id: number | null;
}

// Frontend: camelCase, string
interface Group {
  id: string; // Convert to string!
  roomId: string | null; // camelCase + string
}
```

### 4. Next.js 15 Async Params

```typescript
export const GET = createGetHandler(async (request, token, params) => {
  // params automatically awaited by route wrapper
  const { id } = params; // Direct access
});
```

### 5. JWT Tokens Never Exposed

```typescript
// ✅ CORRECT - Tokens stay server-side
const session = await auth();
const token = session?.user?.token; // Server-side only!

// ❌ WRONG - Never do this!
localStorage.setItem("token", token); // Client-side = INSECURE
```

## 📈 Architecture Metrics

### Code Organization

- **Backend**: 11 domains, ~60 tables, 40+ repositories, 11+ services
- **Frontend**: 33 pages, 124 API routes, 20+ domain components
- **Database**: 11 PostgreSQL schemas, ~60 tables

### Key Technologies

- **Backend**: Go 1.23+, Chi v5, BUN ORM, PostgreSQL 17+
- **Frontend**: Next.js 15, React 19, TypeScript 5, Tailwind CSS 4
- **Infrastructure**: Docker, PostgreSQL (SSL), Caddy (future)

### Performance Targets

- **API Response Time**: < 100ms (p95)
- **Database Query Time**: < 50ms (p95)
- **Page Load Time**: < 1s (p95)
- **SSE Latency**: < 500ms (real-time updates)

## 🛠️ Architecture Evolution

### Phase 1 (Current): Single-Server Monolith

```
┌──────────────────────────────────────┐
│  Docker Compose                      │
│  ├── Frontend Container (Next.js)   │
│  ├── Backend Container (Go)         │
│  └── PostgreSQL Container (17+)     │
└──────────────────────────────────────┘
```

**Pros**: Simple, low operational overhead, ACID transactions **Cons**: Single
point of failure, limited horizontal scaling

### Phase 2 (6 months): Read Replicas

```
Primary DB (Write)          Replica DB (Read-Only)
    ├── auth                    ├── auth
    ├── users                   ├── users
    ├── education               ├── education
    └── active                  └── active (analytics)
```

**Benefits**: Scale read traffic, separate reporting from transactional load

### Phase 3 (12 months): Schema Separation

```
Primary DB                  Real-Time DB           Audit DB
├── auth                    └── active             └── audit
├── users                      (high write)           (compliance)
├── education
└── facilities
```

**Benefits**: Isolate high-traffic schemas, optimize independently

### Phase 4 (18+ months): Microservices (If Needed)

```
Auth Service        Education Service      Active Service
    ↓                      ↓                     ↓
  Auth DB            Education DB          Active DB
```

**When**: Team size > 10 developers, clear ownership boundaries **Not before**:
Adds significant operational complexity

## 🧪 Testing Strategy

### Backend Testing

- **Unit Tests**: Service layer with mock repositories
- **Integration Tests**: Repository layer with test database
- **API Tests**: Bruno API test suite (59 scenarios, ~270ms)

### Frontend Testing

- **Type Checking**: `npm run typecheck` (strict mode)
- **Linting**: `npm run lint` (zero warnings policy)
- **Integration Tests**: API route testing with mock backend

## 📦 Related Documentation

### Main Documentation

- **[CLAUDE.md](../../CLAUDE.md)** - Development guide with critical patterns
- **[README.md](../../README.md)** - Project overview and quick start
- **[Backend CLAUDE.md](../../backend/CLAUDE.md)** - Backend-specific guide
- **[Frontend CLAUDE.md](../../frontend/CLAUDE.md)** - Frontend-specific guide

### API Documentation

- **[routes.md](../../backend/routes.md)** - Auto-generated API routes (run
  `go run main.go gendoc`)
- **[OpenAPI Spec](../../backend/docs/openapi.yaml)** - Machine-readable API
  definition

### Feature Documentation

- **[RFID Implementation Guide](../RFID_IMPLEMENTATION_GUIDE.md)** - RFID device
  integration
- **[Security Guide](../security.md)** - Security best practices
- **[SSL Setup](../ssl-setup.md)** - SSL certificate configuration

## 🤝 Contributing to Architecture

### When to Update Architecture Docs

Update architecture documentation when:

- ✅ Making a **significant architectural decision** (create ADR)
- ✅ Adding a **new domain** or schema (update database docs)
- ✅ Changing **authentication/authorization** (update security docs)
- ✅ Modifying **deployment architecture** (update deployment docs)
- ✅ Adding **new external integration** (update C4 diagrams)

### How to Update

1. **Create/Update ADR** for architectural decisions
2. **Update C4 Diagrams** if system/container/component structure changes
3. **Update Database Docs** if schema structure changes
4. **Update Flow Diagrams** if key workflows change
5. **Run `go run main.go gendoc`** to regenerate API documentation

### Review Process

1. Create architecture doc updates in feature branch
2. Submit PR with architecture changes clearly marked
3. Request review from architect or senior developer
4. Update based on feedback
5. Merge with main documentation

## 📞 Contact

**Questions about architecture?**

- **Team Lead**: [Your name]
- **Slack Channel**: #architecture
- **Documentation Issues**:
  [GitHub Issues](https://github.com/moto-nrw/project-phoenix/issues)

---

**Last Updated**: 2025-10-19 **Next Architecture Review**: 2025-11-19
**Maintainers**: Project Phoenix Team
