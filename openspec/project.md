# Project Context

## Purpose
Project Phoenix is a **GDPR-compliant RFID-based student attendance and room management system** for educational institutions. It implements strict privacy controls for student data access with automated data retention and comprehensive audit logging.

**Key Goals:**
- Real-time student attendance tracking via RFID devices
- Privacy-first architecture with role-based data access
- Automated GDPR compliance (data retention, audit logging)
- Room utilization and capacity management
- Teacher assignment and substitution workflows
- Automated data cleanup based on student-specific retention policies

## Tech Stack

### Backend
- **Language**: Go 1.23+
- **Framework**: Chi router (HTTP routing)
- **ORM**: BUN ORM for PostgreSQL
- **Database**: PostgreSQL 17+ with SSL encryption (multi-schema architecture)
- **Authentication**: JWT-based auth with role-based access control
- **Password Hashing**: Argon2id
- **Validation**: ozzo-validation

### Frontend
- **Framework**: Next.js 15+ (App Router)
- **UI Library**: React 19+
- **Styling**: Tailwind CSS 4+
- **Type Safety**: TypeScript (strict mode)
- **Authentication**: NextAuth with JWT strategy
- **HTTP Client**: Custom API client with JWT injection

### Infrastructure
- **Deployment**: Docker + Docker Compose
- **Database Encryption**: SSL/TLS (minimum TLS 1.2)
- **Testing**: Bruno CLI for API testing
- **Linting**: golangci-lint (Go), ESLint (TypeScript)
- **Code Quality**: Zero warnings policy enforced

### Database Schema Organization
11 PostgreSQL schemas for domain separation:
- `auth`: Authentication, tokens, permissions, roles
- `users`: User profiles, students, teachers, staff
- `education`: Groups and educational structures
- `facilities`: Rooms and physical locations
- `activities`: Student activities and enrollments
- `active`: Real-time session tracking (visits, attendance)
- `schedule`: Time and schedule management
- `iot`: RFID device management
- `feedback`: User feedback
- `config`: System configuration
- `audit`: GDPR audit logging

## Project Conventions

### Code Style

**Backend (Go):**
- Standard Go formatting: `gofmt` + `goimports`
- Import grouping: stdlib → external → internal
- Error handling: Always check errors, never ignore
- Context: Pass `context.Context` as first parameter
- Naming: Use Go idioms (short receiver names, exported/unexported)
- Comments: Document exported functions with complete sentences

**Frontend (TypeScript):**
- Prettier for formatting (enforced via hooks)
- ESLint: Zero warnings policy (max-warnings=0)
- Naming: camelCase for variables/functions, PascalCase for components
- Type safety: No `any` types, prefer explicit interfaces
- File organization: Domain-based folder structure

**Database:**
- Snake_case for all table/column names
- Schema-qualified table names in queries
- Migration files: `{version}_{description}.go`
- Always specify dependencies between migrations

### Architecture Patterns

**Backend Layered Architecture:**
```
HTTP Request → Chi Router → Middleware (JWT, Permissions) →
API Handler → Service Layer (business logic) → Repository (data access) →
BUN ORM → PostgreSQL
```

**Key Patterns:**
1. **Factory Pattern**: Dependency injection via factory constructors (no DI framework)
   ```go
   repoFactory := repositories.NewFactory(db)
   serviceFactory := services.NewFactory(repoFactory, mailer)
   ```

2. **Repository Pattern**: Interfaces in `models/`, implementations in `database/repositories/`
   ```go
   type GroupRepository interface {
       GetByID(ctx context.Context, id int64) (*Group, error)
       List(ctx context.Context, options *QueryOptions) ([]*Group, error)
   }
   ```

3. **BUN ORM Schema-Qualified Tables** (CRITICAL):
   ```go
   // ALWAYS quote table aliases to prevent runtime errors
   ModelTableExpr(`education.groups AS "group"`)  // ✓ CORRECT
   ModelTableExpr(`education.groups AS group`)    // ✗ WRONG
   ```

4. **Frontend Type Mapping**: Backend int64 → Frontend string for all IDs
   ```typescript
   export function mapGroupResponse(data: BackendGroup): Group {
       return {
           id: data.id.toString(),  // Always convert
           roomId: data.room_id?.toString() ?? null
       };
   }
   ```

5. **API Proxy Pattern**: All backend calls proxied through Next.js API routes
   ```typescript
   // Client → Next.js Route → Backend (JWT never exposed to browser)
   export const GET = createGetHandler(async (request, token, params) => {
       const response = await apiGet('/api/resources', token);
       return response.data;
   });
   ```

### Testing Strategy

**Backend Testing:**
- Unit tests for repository and service layers
- Integration tests with real test database
- Test helpers in `test/helpers.go`
- Race condition detection: `go test -race ./...`
- Coverage focus: Business logic, edge cases, error handling

**API Testing (Bruno):**
- Consolidated test suite: 59 scenarios across 11 files
- Hermetic tests: Self-contained setup/cleanup per file
- Fast execution: ~270ms for full suite
- Domain-based tests: `./dev-test.sh groups`, `./dev-test.sh students`
- Authentication: Uses real JWT tokens with test accounts

**Frontend Testing:**
- Component testing with React Testing Library
- Type safety with TypeScript strict mode
- Zero warnings policy: `npm run check` MUST PASS before commits
- Linting: ESLint with max-warnings=0
- Type checking: Full TypeScript validation

**Test Principles:**
- Tests must verify actual business logic, not just syntax
- Exercise boundary conditions and error cases
- Tests should fail when code is intentionally broken
- No placeholder or trivial tests

### Git Workflow

**Branching Strategy:**
- `main`: Production-ready code (protected)
- `development`: Integration branch for features (DEFAULT PR target)
- `feature/*`: Feature development branches
- `fix/*`: Bug fix branches

**CRITICAL**: Always create PRs to `development` branch unless explicitly told otherwise!

**Commit Conventions:**
```
<type>: <subject>

<body>

<footer>
```

**Types**: `feat`, `fix`, `refactor`, `chore`, `docs`, `test`, `style`

**Examples:**
- `feat: add RFID device authentication with two-layer security`
- `fix: prevent duplicate visit entries for same student`
- `refactor: simplify group supervisor assignment logic`
- `chore: update Go dependencies to latest versions`

**Rules:**
- Never use `git add .` (use explicit file paths)
- Never force push to main/development
- Never credit AI assistants in commits or PRs
- Run `git diff --cached` before committing
- Atomic commits: One logical change per commit

## Domain Context

**Educational Institution Domain:**
- **Students**: Children with guardians, privacy consent, RFID cards
- **Teachers**: Staff members who supervise groups and activities
- **Groups**: Educational cohorts with assigned rooms and schedules
- **Activities**: Scheduled educational activities with student enrollments
- **Rooms**: Physical locations with capacity limits
- **Sessions**: Real-time tracking of group activities in rooms
- **Visits**: Individual student movements (check-in/check-out)

**RFID Integration:**
- Devices authenticate via API key + teacher PIN (two-layer auth)
- Students check in/out using RFID cards
- Real-time location tracking via `active.visits` table
- Device health monitoring and status tracking

**Privacy & Access Control:**
- Teachers see FULL data only for students in their assigned groups
- Other staff see ONLY student names + responsible person
- Admin accounts reserved for GDPR compliance tasks (exports, deletions)
- All data deletions logged in audit tables
- Student-specific data retention periods (1-31 days, default 30)

**Student Location Tracking (CRITICAL):**
- ✅ **CORRECT**: `active.visits` + `active.attendance` tables
- ❌ **DEPRECATED**: `users.students` boolean flags (`in_house`, `wc`, `school_yard`) - being phased out
- Always use `active.visits` for current location queries

## Important Constraints

**GDPR Compliance (MANDATORY):**
- SSL encryption for database connections (sslmode=require)
- Automated data retention and cleanup
- Audit logging for all data deletions (Article 30 compliance)
- Right to erasure: Hard delete all student data
- Privacy consent tracking and expiration
- Data access restrictions based on teacher assignments

**Security Requirements:**
- Never commit `.env*` files (except `*.example`)
- Passwords hashed with Argon2id
- JWT tokens expire (15min access, 1hr refresh)
- Permission checks on all API routes
- Input validation on all user inputs
- SSL certificates for PostgreSQL (self-signed in dev, proper CA in production)

**Technical Constraints:**
- Docker backend MUST be rebuilt after Go code changes (`docker compose build server`)
- BUN ORM requires quoted table aliases in schema-qualified queries
- Next.js 15: Route params are async (`await params`)
- Zero warnings policy: `npm run check` must pass before commits
- PostgreSQL 17+ required for advanced features
- Go 1.23+ required for backend

**Performance:**
- API responses paginated by default (50 items)
- Eager loading with `Relation()` to avoid N+1 queries
- Index foreign keys and frequently queried fields
- Bruno test suite must complete in < 500ms

## External Dependencies

**RFID Devices:**
- Custom API endpoints for device communication (`/iot/checkin`, `/iot/checkout`)
- Two-layer authentication: Device API key + Teacher PIN
- Device health monitoring via ping endpoints
- RFID tag assignments tracked per person

**Email Service:**
- Email templating and delivery (configuration in backend)
- Used for notifications and alerts
- Template-based system with validation

**PostgreSQL Database:**
- Version: 17+
- SSL encryption required
- Multi-schema architecture (11 schemas)
- Custom functions and triggers for data integrity

**Docker Infrastructure:**
- Docker Compose for orchestration
- Health checks and restart policies
- Volume persistence for database and SSL certificates
- Resource limits configured per service

**Development Tools:**
- Bruno CLI: API testing framework
- golangci-lint: Go code linting
- goimports: Go import organization
- Prettier: TypeScript/JavaScript formatting
- ESLint: TypeScript linting

**External Services (Future):**
- Export functionality (data portability) - planned
- SSO integration - potential future requirement
- Mobile app API - under consideration
