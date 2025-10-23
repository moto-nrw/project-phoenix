# Project Context

## Purpose
Project Phoenix is a GDPR-compliant RFID-based student attendance and room management system for educational institutions. It provides real-time tracking of student locations, activity management, and implements strict privacy controls for student data access in accordance with European data protection regulations.

**Core Capabilities:**
- Real-time student attendance tracking via RFID check-in/checkout
- Active group session management with supervisor assignments
- Room utilization and capacity management
- Activity scheduling and enrollment
- Privacy-compliant student data access control
- Automated data retention and cleanup (GDPR Article 17)
- Multi-supervisor support for educational groups

## Tech Stack

### Backend
- **Language:** Go 1.23+
- **Web Framework:** Chi router (v5)
- **Database:** PostgreSQL 17+ with SSL encryption (multi-schema: 11 schemas)
- **ORM:** BUN ORM (schema-qualified tables with quoted aliases)
- **Authentication:** JWT (15min access, 1hr refresh) with role-based permissions
- **Real-Time:** Server-Sent Events (SSE) via hub pattern
- **Caching:** None (planned: Redis for analytics)
- **Testing:** Go standard testing package, Bruno API testing
- **Deployment:** Docker + Docker Compose

### Frontend
- **Framework:** Next.js 15 (App Router)
- **UI Library:** React 19
- **Styling:** Tailwind CSS v4
- **Language:** TypeScript (strict mode)
- **Auth:** NextAuth with JWT strategy
- **State Management:** React hooks (no global state library)
- **API Client:** Custom wrappers with type mapping helpers
- **Testing:** React Testing Library, MSW (planned)

### Infrastructure
- **Database:** PostgreSQL 17 with SSL (sslmode=require for GDPR)
- **Certificates:** Self-signed SSL certs (1-year expiry, regenerate with `create-certs.sh`)
- **Deployment:** Docker Compose (local dev), Docker (production)
- **Monitoring:** Structured logging only (no APM yet)

## Project Conventions

### Code Style

#### Backend (Go)
- **Formatting:** `go fmt ./...` and `goimports -w .`
- **Linting:** `golangci-lint run --timeout 10m` (zero warnings policy)
- **Package organization:** Domain-driven structure (api/, services/, models/, database/repositories/)
- **Naming:**
  - Interfaces: Service suffix (e.g., `ActiveService`)
  - Implementations: Concrete names (e.g., `ActiveServiceImpl`)
  - Factories: `New` prefix (e.g., `NewActiveService`)
- **Error handling:** Always check errors, use `errors.Wrap` for context
- **Context:** Always pass `context.Context` as first parameter

#### Frontend (TypeScript)
- **Formatting:** Prettier (automatically via npm run format:write)
- **Linting:** ESLint with max-warnings=0 (enforced in CI)
- **Type checking:** TypeScript strict mode (must pass `npm run typecheck`)
- **Naming:**
  - Components: PascalCase (e.g., `StudentList`)
  - Files: kebab-case (e.g., `student-list.tsx`)
  - Hooks: camelCase with `use` prefix (e.g., `useStudents`)
  - API clients: kebab-case with `-api` suffix (e.g., `student-api.ts`)
- **Import organization:** stdlib → external → internal (via goimports pattern)

### Architecture Patterns

#### Backend Patterns
1. **Factory Pattern for DI:**
   ```go
   repoFactory := repositories.NewFactory(db)
   serviceFactory := services.NewFactory(repoFactory, mailer)
   activeService := serviceFactory.NewActiveService()
   ```

2. **Repository Pattern:**
   - Interface in `models/{domain}/repository.go`
   - Implementation in `database/repositories/{domain}/`
   - BUN ORM CRITICAL: Always use schema-qualified tables with quoted aliases
   ```go
   ModelTableExpr(`education.groups AS "group"`)  // CORRECT
   ModelTableExpr(`education.groups AS group`)    // WRONG - causes errors
   ```

3. **Service Layer:**
   - Business logic in `services/{domain}/`
   - Services orchestrate repositories, no direct DB access in API handlers
   - Transaction handling via `TxHandler.RunInTx`

4. **API Layer:**
   - Thin handlers in `api/{domain}/`
   - Chi router with middleware composition
   - Permission checks via `authorize.RequiresPermission` middleware
   - Response format: `{status: "success|error", data: {...}, message: "..."}`

5. **Permission System:**
   - Constants in `auth/authorize/permissions/permissions.go`
   - Applied per endpoint via middleware
   - Policy-based access control for data filtering (GDPR compliance)

#### Frontend Patterns
1. **Type Mapping:** Backend int64 → Frontend string, snake_case → camelCase
2. **API Proxy:** All backend calls go through Next.js API routes (JWT never exposed to client)
3. **Suspense Boundaries:** Required for components using `useSearchParams()`
4. **Next.js 15 Params:** Route params are `Promise<Record<string, string>>` (must await)

### Testing Strategy

#### Backend Testing
- **Unit Tests:** Service layer logic, repository queries
- **Integration Tests:** API endpoints with real test database
- **Test Helpers:** `test/helpers.go` for common setup
- **Coverage Goal:** 70%+ for critical paths (auth, GDPR, payments)
- **Run:** `go test ./...` or `go test -race ./...` for race detection

#### API Testing (Bruno)
- **Location:** `bruno/` directory
- **Consolidated Tests:** 11 files, 59 scenarios, ~270ms execution time
- **Usage:** `cd bruno && bru run --env Local 0*.bru`
- **Coverage:** All RFID workflows, auth flows, session lifecycle
- **Hermetic:** Each test file self-contained with setup/cleanup

#### Frontend Testing
- **Planned:** React Testing Library + MSW
- **Current:** Manual testing only
- **Quality Gate:** `npm run check` (lint + typecheck) must pass before commit

### Git Workflow

#### Branching Strategy
- **Main Branch:** `main` (production-ready code)
- **Development Branch:** `development` (integration branch)
- **Feature Branches:** `feature/feature-name` (branch from `development`)
- **Hotfix Branches:** `hotfix/issue-name` (branch from `main`)

#### Pull Request Rules
- **DEFAULT TARGET:** `development` (NEVER create PRs to `main` unless explicitly instructed)
- **PR Creation:** `gh pr create --base development`
- **Required Checks:** golangci-lint (backend), npm run check (frontend)
- **Review:** At least 1 approval required
- **Merge:** Squash and merge with conventional commit message

#### Commit Conventions
- **Format:** `<type>: <subject>` (no body unless needed)
- **Types:** feat, fix, refactor, chore, docs, test, style
- **Subject:** Lowercase, imperative mood, no period
- **Example:** `feat: add student privacy consent tracking`
- **NEVER:** Credit Claude in commit messages

## Domain Context

### Education Domain
- **Groups:** Educational classes (e.g., "4A", "Math Advanced") or activity groups
- **Teachers:** Staff members assigned to groups as representatives
- **Students:** 120 students across grades, linked to guardians
- **Substitutions:** Temporary teacher replacements (no backdating allowed)
- **Activities:** Optional student activities with scheduling and enrollment

### Active Sessions Domain
- **Active Groups:** Currently running educational sessions with room assignments
- **Combined Groups:** Multiple groups combined into single session (e.g., joint classes)
- **Visits:** Student check-in/checkout events tracked in `active.visits` table
- **Supervisors:** Multiple supervisors per group with role assignments (supervisor, assistant)
- **Scheduled Checkouts:** Future-dated checkout automation

### GDPR & Privacy
- **Data Retention:** Student-specific retention settings (1-31 days, default 30)
- **Access Control:** Teachers see FULL data only for students in their groups
- **Privacy Consent:** Versioned consent tracking with expiration
- **Audit Logging:** All deletions logged in `audit.data_deletions` table
- **Cleanup:** Automated daily cleanup at 2:00 AM + manual CLI commands

### IoT Device Integration
- **RFID Devices:** ESP32-based RFID readers for student check-in/checkout
- **Authentication:** Two-layer (Device API key + Staff PIN)
- **Device Management:** Health monitoring, firmware version tracking
- **Session Timeout:** Configurable auto-checkout after inactivity

## Important Constraints

### GDPR Compliance (CRITICAL)
- **Encryption:** All database connections must use SSL (sslmode=require minimum)
- **Data Minimization:** Only collect necessary data, delete after retention period
- **Access Control:** Role-based permissions enforced at API and service layers
- **Right to Erasure:** Hard delete with cascade, log in audit table
- **Consent Tracking:** Versioned privacy policies with expiration dates
- **Audit Trail:** Immutable logs for compliance reporting (Article 30)

### Performance Requirements
- **API Response Time:** <100ms for reads, <200ms for writes (p95)
- **Dashboard Analytics:** <500ms for aggregated queries
- **Real-Time Updates:** <2s end-to-end latency for check-in events
- **Concurrent Users:** Support 150 concurrent users (120 students + 30 staff)

### Data Integrity
- **Student Location:** Use `active.visits` table as source of truth
- **DEPRECATED:** Boolean flags in `users.students` (in_house, wc, school_yard) are BROKEN
- **Bus Flag:** Administrative permission flag only ("Buskind"), NOT a location indicator
- **Transaction Boundaries:** Complex operations (combined groups, substitutions) require transactions

### Deployment Constraints
- **IoT Firmware:** Can't be updated remotely - API endpoints for IoT must remain stable
- **Docker Backend:** Code changes require container rebuild (`docker compose build server`)
- **SSL Certificates:** Self-signed certs expire after 1 year, must regenerate manually
- **Database Migrations:** Sequential execution, dependency tracking required

### Scale Constraints (Small Institution)
- **Students:** 120 students
- **Staff:** 30 teachers/staff
- **Rooms:** 24 rooms across 3 buildings
- **Active Sessions:** Typically <10 concurrent sessions
- **Peak Load:** ~50 concurrent API requests during class changes

## External Dependencies

### Production Dependencies
- **PostgreSQL 17+:** Multi-schema database with SSL encryption
- **SMTP Server:** Email delivery for invitations and password resets
- **Redis (Planned):** Caching layer for analytics queries

### Development Dependencies
- **Docker & Docker Compose:** Local development environment
- **golangci-lint:** Go linting (install: `brew install golangci-lint`)
- **goimports:** Go import organizer (install: `go install golang.org/x/tools/cmd/goimports@latest`)
- **Bruno CLI:** API testing (`bru run`)
- **Next.js Dev Server:** Frontend development

### Optional Dependencies
- **OpenAPI Generator:** Type generation from OpenAPI spec
- **Zod:** Runtime validation for API requests (planned for frontend)
- **React Query:** Data fetching and caching (planned for mobile)

## Migration to OpenSpec

### Current State
- **API Endpoints:** 220 endpoints across 17 domains (backend/routes.md)
- **~70 Unused Endpoints:** Identified via frontend usage analysis
- **Real API Surface:** ~150 actively used endpoints
- **Documentation:** OpenAPI spec + routes.md (generated via `go run main.go gendoc`)
- **Real-Time:** SSE used in only 2 pages (myroom, ogs_groups)

### Planned Changes
- **Phase 1 (Month 1):** Audit and delete unused endpoints (220 → 150)
- **Phase 2 (Month 2):** Add Mobile BFF tier for optimized mobile responses
- **Phase 3 (Month 3):** Selective consolidation of simple list endpoints (150 → 120)

### OpenSpec Usage
- **Capabilities:** Will be created per domain (Active Sessions, Students, Activities, etc.)
- **Change Proposals:** Required for API changes, breaking changes, new features
- **Skip Proposals:** Bug fixes, typos, dependency updates, config changes
