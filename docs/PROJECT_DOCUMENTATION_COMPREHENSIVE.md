# Project Phoenix - Comprehensive Technical Documentation

**Generated:** 2025-10-02
**Purpose:** Complete technical reference for Claude Code optimization and developer onboarding

---

## Table of Contents

1. [Project Overview](#1-project-overview)
2. [Technical Architecture](#2-technical-architecture)
3. [Codebase Patterns & Conventions](#3-codebase-patterns--conventions)
4. [Testing Strategy](#4-testing-strategy)
5. [Build & Development](#5-build--development)
6. [Dependencies & Integrations](#6-dependencies--integrations)
7. [Common Workflows](#7-common-workflows)
8. [Error Handling](#8-error-handling)
9. [Project-Specific Quirks](#9-project-specific-quirks)
10. [File Inventory](#10-file-inventory)

---

## 1. Project Overview

### Business Purpose
Project Phoenix is a **GDPR-compliant RFID-based student attendance and room management system** designed for educational institutions in Germany. It provides:

- **Real-time student location tracking** via RFID card scanning
- **Room occupancy management** with capacity and availability tracking
- **Educational group management** with substitution support
- **Activity scheduling** and enrollment management
- **Privacy-compliant data retention** with automated cleanup
- **Multi-supervisor assignments** for groups and activities
- **Two-layer authentication** for RFID devices (API key + staff PIN)

### Target Users
1. **Teachers/Staff**: Track student attendance, manage groups, view occupancy
2. **Administrators**: Full system access, GDPR compliance tasks, data exports
3. **RFID Devices**: IoT readers for automated student check-in/check-out
4. **Students** (indirect): Tracked via RFID cards, no direct login

### Key Features & Capabilities
- **Active Session Tracking**: Real-time monitoring of which students are in which rooms
- **Combined Groups**: Merge multiple groups for joint activities
- **Substitution System**: Temporary staff replacements with no-backdating rule
- **Privacy Consent Management**: Per-student data retention policies (1-31 days, default 30)
- **Automated Data Cleanup**: Scheduler runs daily at 2:00 AM to delete expired visit records
- **Audit Logging**: All data deletions tracked in `audit.data_deletions` table
- **SSL-Encrypted Database**: PostgreSQL with TLS 1.2+ for GDPR compliance
- **Role-Based Access Control**: Permission-based authorization system
- **PIN Authentication**: Staff can set 4-digit PINs for RFID device access

### Current Development Stage
- **Status**: Active development with production-ready core features
- **Architecture**: Stable, undergoing refactoring for modernization
- **Backend**: 326 Go files across 12 domain packages
- **Frontend**: 332 TypeScript/TSX files with Next.js 15 App Router
- **Database**: 100+ migrations defining multi-schema PostgreSQL structure
- **Testing**: Bruno API tests (~252ms full suite), limited unit test coverage
- **Deployment**: Docker Compose with health checks and SSL certificates

---

## 2. Technical Architecture

### Complete Tech Stack

#### Backend (Go)
- **Language**: Go 1.23.0 (toolchain 1.24.1)
- **HTTP Framework**: Chi v5.2.0 (router with middleware)
- **ORM**: Bun v1.2.11 with PostgreSQL dialect
- **Database Driver**: pgdriver v1.2.11 (Bun's native driver)
- **Authentication**: JWT via jwtauth/v5 v5.3.2 + lestrrat-go/jwx/v2 v2.1.3
- **Password Hashing**: golang.org/x/crypto v0.36.0 (Argon2id)
- **Validation**: ozzo-validation v3.6.0
- **Logging**: logrus v1.9.3 (structured logging)
- **CLI Framework**: cobra v1.8.1 + viper v1.19.0 (config)
- **Email**: go-mail v0.6.2 (SMTP) + go-premailer v1.22.0 (HTML emails)
- **CORS**: go-chi/cors v1.2.1
- **Rate Limiting**: golang.org/x/time v0.11.0
- **UUID**: gofrs/uuid v4.4.0
- **Testing**: testify v1.10.0

#### Frontend (Next.js)
- **Framework**: Next.js v15.2.3 (App Router)
- **React**: v19.0.0 (with React DOM v19.0.0)
- **TypeScript**: v5.8.2 (strict mode)
- **Styling**: Tailwind CSS v4.0.15 with @tailwindcss/postcss v4.0.15
- **Authentication**: next-auth v5.0.0-beta.25 (JWT strategy)
- **HTTP Client**: axios v1.8.4
- **Environment Validation**: @t3-oss/env-nextjs v0.12.0 with zod v3.24.2
- **Icons**: lucide-react v0.509.0
- **Fonts**: @fontsource/geist-sans v5.2.5
- **UI Effects**: canvas-confetti v1.9.3
- **Linting**: eslint v9.23.0 with typescript-eslint v8.27.0
- **Formatting**: prettier v3.5.3 with prettier-plugin-tailwindcss v0.6.11
- **Package Manager**: npm v11.3.0 (specified in package.json)

#### Database
- **RDBMS**: PostgreSQL 17-alpine (Docker image)
- **SSL/TLS**: Minimum TLS 1.2 with strong ciphers (GDPR requirement)
- **Schemas**: 11 separate schemas (auth, users, education, schedule, activities, facilities, iot, feedback, active, config, meta)
- **Connection Pooling**: Via Bun ORM with pgdriver
- **Migrations**: Custom migration system with dependency tracking (100+ migrations)

#### Infrastructure
- **Containerization**: Docker with multi-stage builds
- **Orchestration**: Docker Compose v2+ (with watch mode for development)
- **SSL Certificates**: Self-signed for development (config/ssl/postgres/)
- **Timezone**: Europe/Berlin (enforced across all containers)
- **Health Checks**: PostgreSQL (1s interval), Frontend (30s interval)

#### Development Tools
- **API Testing**: Bruno CLI with custom dev-test.sh wrapper
- **Code Quality (Go)**: golangci-lint with timeout 10m
- **Code Quality (TypeScript)**: ESLint + Prettier with 0 warnings policy
- **Import Organization (Go)**: goimports (`/Users/yonnock/go/bin/goimports`)
- **API Documentation**: Custom `gendoc` command generates routes.md and OpenAPI spec
- **Version Control**: Git with GitHub (main/development branch strategy)

### Architecture Pattern
**Hybrid Monorepo** with clear frontend/backend separation:
- **Backend**: Domain-Driven Design with layered architecture
- **Frontend**: Next.js App Router with domain-based organization
- **Communication**: RESTful API with JWT authentication
- **Data Flow**: Frontend → Next.js API Routes → Backend API → Services → Repositories → Database

### Complete Directory Structure

```
project-phoenix/
├── .claude/                      # Claude Code configuration
│   └── settings.local.json       # User-specific Claude settings
├── .git/                         # Git repository
├── .github/                      # GitHub Actions workflows
├── .scannerwork/                 # SonarQube analysis artifacts
├── backend/                      # Go backend service (326 Go files)
│   ├── api/                      # HTTP handlers (thin layer)
│   │   ├── active/              # Real-time session/visit tracking
│   │   ├── activities/          # Activity management
│   │   ├── auth/                # Login, logout, refresh
│   │   ├── common/              # Shared API utilities
│   │   ├── config/              # System configuration
│   │   ├── database/            # Database health checks
│   │   ├── feedback/            # User feedback
│   │   ├── groups/              # Educational group management
│   │   ├── iot/                 # RFID device integration
│   │   ├── rooms/               # Room management
│   │   ├── schedules/           # Time-based scheduling
│   │   ├── staff/               # Staff management + PIN endpoints
│   │   ├── students/            # Student CRUD operations
│   │   ├── substitutions/       # Staff substitution system
│   │   ├── usercontext/         # User context and permissions
│   │   ├── users/               # General user management
│   │   └── base.go              # Main router setup with domain mounting
│   ├── auth/                     # Authentication & authorization
│   │   ├── authorize/           # Permission checks and policies
│   │   │   ├── policies/        # GDPR-compliant access policies
│   │   │   └── permissions/     # Permission constants
│   │   ├── device/              # RFID device authentication
│   │   ├── jwt/                 # JWT middleware
│   │   └── userpass/            # Username/password auth
│   ├── cmd/                      # CLI commands (Cobra)
│   │   ├── root.go              # Root command with config initialization
│   │   ├── serve.go             # HTTP server command
│   │   ├── migrate.go           # Database migration commands
│   │   ├── seed.go              # Test data seeding
│   │   ├── cleanup.go           # GDPR data cleanup commands
│   │   └── gendoc.go            # API documentation generation
│   ├── database/                 # Data access layer
│   │   ├── migrations/          # 100+ versioned migrations
│   │   │   ├── 000000_create_schemas.go    # Creates 11 PostgreSQL schemas
│   │   │   ├── 001000001_auth_accounts.go  # Auth schema tables
│   │   │   ├── 002000001_users_persons.go  # Users schema tables
│   │   │   └── ...                         # Chronologically ordered
│   │   ├── repositories/        # Repository implementations
│   │   │   ├── active/          # Visit/attendance repositories
│   │   │   ├── activities/      # Activity repositories
│   │   │   ├── auth/            # Auth repositories
│   │   │   ├── education/       # Group repositories
│   │   │   ├── facilities/      # Room repositories
│   │   │   ├── iot/             # Device repositories
│   │   │   ├── schedule/        # Schedule repositories
│   │   │   ├── users/           # User repositories
│   │   │   └── factory.go       # Repository factory pattern
│   │   └── db.go                # Database connection setup
│   ├── docs/                     # Generated API documentation
│   │   ├── routes.md            # All routes with middleware chains
│   │   ├── openapi.yaml         # OpenAPI 3.0.3 specification
│   │   ├── rfid-integration-guide.md
│   │   └── rfid-examples.md
│   ├── email/                    # Email templating
│   │   └── templates/
│   │       └── email/           # HTML email templates
│   ├── logging/                  # Structured logging utilities
│   ├── middleware/               # HTTP middleware
│   │   └── security_headers.go  # Security headers (CSP, HSTS, etc.)
│   ├── models/                   # Domain models (12 domains)
│   │   ├── active/              # Visit, Attendance, GroupSupervisor
│   │   ├── activities/          # Activity, Enrollment
│   │   ├── audit/               # DataDeletion (GDPR audit log)
│   │   ├── auth/                # Account, Token, Role, Permission
│   │   ├── base/                # QueryOptions, Filter, Pagination
│   │   ├── config/              # SystemConfig
│   │   ├── education/           # Group, Substitution
│   │   ├── facilities/          # Room, Building
│   │   ├── feedback/            # Feedback
│   │   ├── iot/                 # Device, RFIDCard
│   │   ├── schedule/            # Timeframe, Dateframe
│   │   └── users/               # Person, Staff, Student, Teacher, Guardian, PrivacyConsent
│   ├── public/                   # Static file serving
│   │   └── uploads/             # User-uploaded files
│   ├── seed/                     # Test data generation
│   │   ├── fixed/               # Fixed seed data (rooms, etc.)
│   │   └── runtime/             # Runtime-generated seed data
│   ├── services/                 # Business logic layer
│   │   ├── active/              # Session management, cleanup service
│   │   ├── activities/          # Activity orchestration
│   │   ├── auth/                # Authentication logic
│   │   ├── config/              # Configuration management
│   │   ├── database/            # Database utilities
│   │   ├── education/           # Group management
│   │   ├── facilities/          # Room management
│   │   ├── feedback/            # Feedback processing
│   │   ├── iot/                 # Device management
│   │   ├── schedule/            # Schedule calculations
│   │   ├── scheduler/           # Automated task scheduler
│   │   ├── usercontext/         # GDPR-compliant user context service
│   │   ├── users/               # User lifecycle management
│   │   └── factory.go           # Service factory pattern
│   ├── templates/                # Template rendering
│   ├── test/                     # Integration tests
│   ├── Dockerfile                # Multi-stage Go build
│   ├── go.mod                    # Go module dependencies
│   ├── go.sum                    # Dependency checksums
│   ├── main.go                   # Entry point (delegates to cmd/)
│   ├── dev.env.example           # Backend environment template
│   └── CLAUDE.md                 # Backend-specific Claude instructions
├── bruno/                        # API testing suite
│   ├── dev/                      # 52 development test files
│   │   ├── auth.bru             # Authentication tests
│   │   ├── groups.bru           # Group CRUD tests (25 groups)
│   │   ├── students.bru         # Student tests (50 students)
│   │   ├── rooms.bru            # Room tests (24 rooms)
│   │   ├── device-*.bru         # RFID device integration tests
│   │   ├── attendance-*.bru     # Attendance tracking tests
│   │   └── ...
│   ├── environments/             # Test environment configs
│   ├── examples/                 # API usage examples
│   ├── manual/                   # Pre-release manual tests
│   ├── bruno.json                # Bruno CLI configuration
│   └── dev-test.sh               # Test runner wrapper (gets fresh tokens)
├── config/                       # Configuration files
│   └── ssl/
│       └── postgres/            # PostgreSQL SSL certificates
│           ├── certs/           # Generated certificates (git-ignored)
│           ├── create-certs.sh  # Certificate generation script
│           ├── postgresql.conf  # PostgreSQL SSL config
│           └── pg_hba.conf      # PostgreSQL host-based auth
├── docs/                         # Project documentation
│   ├── features/                # Feature specifications
│   ├── routes.md                # Generated API route listing
│   └── openapi.yaml             # Generated OpenAPI spec
├── frontend/                     # Next.js frontend (332 TS/TSX files)
│   ├── public/                   # Static assets
│   │   └── images/              # Image assets
│   ├── src/
│   │   ├── app/                 # Next.js 15 App Router
│   │   │   ├── (auth)/         # Auth-protected routes
│   │   │   │   ├── dashboard/  # Main dashboard
│   │   │   │   ├── groups/     # Group management UI
│   │   │   │   ├── students/   # Student management UI
│   │   │   │   ├── rooms/      # Room management UI
│   │   │   │   └── ...
│   │   │   ├── api/            # Next.js API routes (proxy to backend)
│   │   │   │   ├── active/     # Active session endpoints
│   │   │   │   ├── activities/ # Activity endpoints
│   │   │   │   ├── auth/       # Auth endpoints (login, refresh)
│   │   │   │   ├── dashboard/  # Dashboard data endpoints
│   │   │   │   ├── groups/     # Group endpoints
│   │   │   │   ├── iot/        # RFID device endpoints
│   │   │   │   ├── persons/    # Person endpoints
│   │   │   │   ├── rfid-cards/ # RFID card management
│   │   │   │   └── ...
│   │   │   ├── login/          # Login page
│   │   │   ├── layout.tsx      # Root layout
│   │   │   └── page.tsx        # Home page
│   │   ├── components/          # React components (domain-organized)
│   │   │   ├── activities/     # Activity components
│   │   │   ├── auth/           # Auth components
│   │   │   ├── dashboard/      # Dashboard widgets
│   │   │   ├── facilities/     # Room/building components
│   │   │   ├── groups/         # Group management components
│   │   │   ├── rooms/          # Room selection/display
│   │   │   ├── ui/             # Shared UI components
│   │   │   ├── animated-background.tsx
│   │   │   ├── background-wrapper.tsx
│   │   │   └── session-provider.tsx
│   │   ├── lib/                 # Domain services and utilities
│   │   │   ├── active-api.ts   # Active session API client
│   │   │   ├── active-helpers.ts # Active session type mapping
│   │   │   ├── active-service.ts # Active session business logic
│   │   │   ├── activity-api.ts
│   │   │   ├── activity-helpers.ts
│   │   │   ├── activity-service.ts
│   │   │   ├── api-client.ts   # Base axios instance
│   │   │   ├── api-helpers.ts  # Error handling utilities
│   │   │   ├── api.ts          # API client exports
│   │   │   ├── auth-api.ts
│   │   │   ├── auth-helpers.ts
│   │   │   ├── auth-service.ts
│   │   │   ├── auth-utils.ts
│   │   │   ├── dashboard-api.ts
│   │   │   ├── dashboard-helpers.ts
│   │   │   ├── route-wrapper.ts # Next.js route handler wrappers
│   │   │   └── ...             # 15+ domain API clients
│   │   ├── server/             # Server-side utilities
│   │   │   └── auth.ts         # NextAuth configuration
│   │   ├── styles/             # Global styles
│   │   │   └── globals.css     # Tailwind directives
│   │   └── env.js              # Environment validation with Zod
│   ├── tmp/                     # Temporary files (git-ignored)
│   ├── Dockerfile               # Multi-stage Node build
│   ├── package.json             # npm v11.3.0 dependencies
│   ├── package-lock.json        # Dependency lock file
│   ├── tsconfig.json            # TypeScript strict mode config
│   ├── eslint.config.js         # ESLint v9 flat config
│   ├── prettier.config.js       # Prettier with Tailwind plugin
│   ├── postcss.config.js        # PostCSS with Tailwind
│   ├── tailwind.config.js       # Tailwind v4 config (directory-based)
│   ├── next.config.js           # Next.js config (imports env validation)
│   ├── .env.example             # Frontend environment template
│   └── CLAUDE.md                # Frontend-specific Claude instructions
├── Project Seminar Infos/       # Academic project documentation
├── scripts/                      # Utility scripts
│   └── setup-dev.sh             # Automated dev environment setup
├── .env                          # Docker Compose environment (git-ignored)
├── .env.example                  # Docker Compose environment template
├── .env.sonar                    # SonarQube configuration
├── .gitignore                    # Git ignore patterns (IDE, build, secrets)
├── docker-compose.yml            # Main development stack
├── docker-compose.example.yml    # Example Docker Compose config
├── docker-compose.cleanup.yml    # GDPR cleanup scheduler service
├── docker-compose.scheduler.yml  # Automated task scheduler
├── docker-compose.sonar.yml      # SonarQube analysis stack
├── sonar-project.properties      # SonarQube project config
├── CLAUDE.md                     # Project-wide Claude instructions (34KB)
├── CLAUDE.local.md               # User-specific local Claude instructions
├── README.md                     # Project README
├── LICENSE                       # MIT License
├── RFID_IMPLEMENTATION_GUIDE.md  # Comprehensive RFID integration guide
├── FRONTEND_BACKEND_IMPLEMENTATION_STATUS.md # Feature status tracking
└── WEBSITE_DESIGN_SYNTHESIS.md  # Design system documentation
```

### Package/Module Organization

#### Backend Go Package Structure
```
github.com/moto-nrw/project-phoenix/
├── api/                # HTTP layer (handles HTTP requests/responses)
│   └── {domain}/       # Each domain has api.go + handlers
├── models/             # Domain models (data structures + validation)
│   └── {domain}/       # Includes repository interfaces
├── services/           # Business logic (orchestration, complex operations)
│   └── {domain}/       # Service interfaces + implementations
├── database/           # Data persistence
│   ├── migrations/     # Database schema evolution
│   └── repositories/   # Repository pattern implementations
├── auth/               # Cross-cutting authentication/authorization
├── middleware/         # HTTP middleware (logging, security)
├── cmd/                # CLI commands (Cobra-based)
└── logging/            # Logging utilities
```

#### Frontend TypeScript Module Structure
```
~/                      # Root (Next.js convention)
├── app/                # Next.js App Router (file-system routing)
│   ├── (auth)/        # Route groups (layout sharing)
│   ├── api/           # API route handlers
│   └── {page}/        # Page components
├── components/         # React components (domain-organized)
│   └── {domain}/      # Feature-specific components
├── lib/                # Business logic and utilities
│   ├── {domain}-api.ts      # Backend API calls
│   ├── {domain}-helpers.ts  # Type transformations
│   ├── {domain}-service.ts  # Complex client-side logic
│   └── route-wrapper.ts     # API route utilities
├── server/             # Server-only code (auth config)
└── styles/             # Global CSS and Tailwind
```

### Dependency Graph Between Components

#### Backend Data Flow
```
HTTP Request
  ↓
Chi Router (api/base.go)
  ↓
Domain Router (api/{domain}/api.go)
  ↓
Middleware Chain:
  - RequestID
  - RealIP
  - Logger
  - Recoverer
  - SecurityHeaders
  - JWT Authentication (auth/jwt/)
  - Permission Check (auth/authorize/)
  ↓
Handler (api/{domain}/handlers.go)
  ↓
Service Factory (services/factory.go)
  ↓
Domain Service (services/{domain}/service.go)
  ↓
Repository Factory (database/repositories/factory.go)
  ↓
Domain Repository (database/repositories/{domain}/)
  ↓
Bun ORM Query
  ↓
PostgreSQL Database
```

#### Frontend Data Flow
```
User Interaction
  ↓
React Component (components/{domain}/)
  ↓
Service Layer (lib/{domain}-service.ts)
  ↓
API Client (lib/{domain}-api.ts)
  ↓
Next.js API Route (app/api/{domain}/route.ts)
  ↓
Route Wrapper (lib/route-wrapper.ts)
  - Extracts JWT from session
  - Handles auth errors
  - Retries with refreshed token
  ↓
Backend API (HTTP request with Bearer token)
  ↓
Response Mapping (lib/{domain}-helpers.ts)
  ↓
React Component Update
```

#### Critical Dependencies
1. **Repository → Service**: Services depend on repository factory
2. **Service → API**: API handlers depend on service factory
3. **Frontend API → Backend**: All frontend API routes proxy to backend
4. **Authentication**: Both frontend and backend validate JWT tokens
5. **Type Mapping**: Frontend helpers transform backend types (int64 → string, snake_case → camelCase)

---

## 3. Codebase Patterns & Conventions

### Backend Go Patterns

#### File Naming Conventions
- **Packages**: Lowercase, singular nouns (`user`, not `users`)
- **Files**: Snake case with descriptive names
  - `repository.go` - Repository interface
  - `service.go` - Service implementation
  - `api.go` - Router setup
  - `handlers.go` - HTTP handlers
  - `{model}_test.go` - Test files alongside source
- **Migrations**: `{version}_{description}.go` (e.g., `001000001_auth_accounts.go`)

#### Go Naming Conventions
- **Exported**: PascalCase (`func GetUser`, `type UserService`)
- **Unexported**: camelCase (`func validateInput`, `var dbConn`)
- **Constants**: PascalCase or SCREAMING_SNAKE_CASE for groups
  ```go
  const (
      UsersRead  = "users.read"
      UsersWrite = "users.write"
  )
  ```
- **Interfaces**: Often end with `-er` (`Repository`, `Service`) or match implementation name

#### Import Organization (goimports)
```go
import (
    // Standard library (alphabetical)
    "context"
    "fmt"
    "time"

    // External packages (alphabetical)
    "github.com/go-chi/chi/v5"
    "github.com/uptrace/bun"

    // Internal packages (alphabetical, relative to module root)
    "github.com/moto-nrw/project-phoenix/auth/authorize"
    "github.com/moto-nrw/project-phoenix/models/users"
    "github.com/moto-nrw/project-phoenix/services"
)
```

#### Critical BUN ORM Patterns

**Schema-Qualified Table Expressions (MANDATORY)**:
```go
// CORRECT - Quotes around alias prevent BUN mapping errors
query := r.db.NewSelect().
    Model(&groups).
    ModelTableExpr(`education.groups AS "group"`)

// WRONG - Missing quotes causes "column not found" errors
ModelTableExpr(`education.groups AS group`)  // DO NOT USE
```

**Loading Nested Relationships**:
```go
// For Teacher → Staff → Person hierarchy
type teacherResult struct {
    Teacher *users.Teacher `bun:"teacher"`
    Staff   *users.Staff   `bun:"staff"`
    Person  *users.Person  `bun:"person"`
}

err := r.db.NewSelect().
    Model(&result).
    ModelTableExpr(`users.teachers AS "teacher"`).
    // Explicit column mapping required
    ColumnExpr(`"teacher".id AS "teacher__id"`).
    ColumnExpr(`"staff".id AS "staff__id"`).
    ColumnExpr(`"person".* AS "person__*"`).
    Join(`INNER JOIN users.staff AS "staff" ON "staff".id = "teacher".staff_id`).
    Join(`INNER JOIN users.persons AS "person" ON "person".id = "staff".person_id`).
    Where(`"teacher".id = ?`, id).
    Scan(ctx)
```

**Query Options Pattern** (models/base/):
```go
options := base.NewQueryOptions()
filter := base.NewFilter()
filter.Equal("status", "active")
filter.ILike("name", "%pattern%")
filter.In("id", []int64{1, 2, 3})
options.Filter = filter
options.WithPagination(1, 50)  // page, per_page

// In repository
query := r.db.NewSelect().Model(&items)
options.ApplyToQuery(query)
```

**Transaction Handling via Context**:
```go
// Service layer starts transaction
tx, err := r.db.BeginTx(ctx, nil)
if err != nil {
    return err
}
defer tx.Rollback()

// Pass transaction via context
ctx = base.ContextWithTx(ctx, tx)

// Repository checks for transaction
func (r *Repository) Create(ctx context.Context, item *Item) error {
    db := r.db
    if tx, ok := base.TxFromContext(ctx); ok {
        db = tx  // Use transaction if present
    }
    _, err := db.NewInsert().Model(item).Exec(ctx)
    return err
}

// Commit in service
return tx.Commit()
```

#### Factory Pattern for Dependency Injection
```go
// Repository Factory (database/repositories/factory.go)
type Factory struct {
    db *bun.DB
}

func NewFactory(db *bun.DB) *Factory {
    return &Factory{db: db}
}

func (f *Factory) NewUserRepository() users.Repository {
    return user.NewRepository(f.db)
}

// Service Factory (services/factory.go)
type Factory struct {
    repos *repositories.Factory
    db    *bun.DB
}

func NewFactory(repos *repositories.Factory, db *bun.DB) (*Factory, error) {
    return &Factory{repos: repos, db: db}, nil
}

func (f *Factory) NewAuthService() auth.Service {
    return authService.New(f.repos.NewAccountRepository(), f.repos.NewTokenRepository())
}

// Usage in main
repoFactory := repositories.NewFactory(db)
serviceFactory, _ := services.NewFactory(repoFactory, db)
authService := serviceFactory.NewAuthService()
```

#### Error Handling Patterns
```go
// Always check errors
if err != nil {
    return fmt.Errorf("failed to fetch user: %w", err)
}

// HTTP error responses
type ErrorResponse struct {
    Status  string `json:"status"`   // "error"
    Message string `json:"message"`  // Human-readable
    Code    string `json:"code,omitempty"`  // Machine-readable
}

// In handlers
if err != nil {
    render.Status(r, http.StatusBadRequest)
    render.JSON(w, r, ErrorResponse{
        Status:  "error",
        Message: "Invalid request",
        Code:    "INVALID_INPUT",
    })
    return
}
```

#### Validation Pattern (ozzo-validation)
```go
// In model file
func (u *User) Validate() error {
    return validation.ValidateStruct(u,
        validation.Field(&u.Email, validation.Required, is.Email),
        validation.Field(&u.Password, validation.Required, validation.Length(8, 100)),
        validation.Field(&u.FirstName, validation.Required, validation.Length(1, 50)),
    )
}

// In handler
user := &User{}
if err := json.NewDecoder(r.Body).Decode(user); err != nil {
    return err
}
if err := user.Validate(); err != nil {
    render.Status(r, http.StatusBadRequest)
    render.JSON(w, r, ErrorResponse{Status: "error", Message: err.Error()})
    return
}
```

#### Migration Pattern
```go
// database/migrations/{version}_{name}.go
package migrations

const (
    AuthAccountsVersion     = "1.0.1"
    AuthAccountsDescription = "Create auth.accounts table"
)

var AuthAccountsDependencies = []string{
    "0.0.0",  // Schemas must exist
}

func init() {
    MigrationRegistry[AuthAccountsVersion] = &Migration{
        Version:     AuthAccountsVersion,
        Description: AuthAccountsDescription,
        DependsOn:   AuthAccountsDependencies,
    }

    Migrations.MustRegister(
        func(ctx context.Context, db *bun.DB) error {
            // Up migration
            _, err := db.ExecContext(ctx, `
                CREATE TABLE IF NOT EXISTS auth.accounts (
                    id BIGSERIAL PRIMARY KEY,
                    email VARCHAR(255) UNIQUE NOT NULL,
                    created_at TIMESTAMP DEFAULT NOW()
                )
            `)
            return err
        },
        func(ctx context.Context, db *bun.DB) error {
            // Down migration
            _, err := db.ExecContext(ctx, `DROP TABLE IF EXISTS auth.accounts CASCADE`)
            return err
        },
    )
}
```

### Frontend TypeScript Patterns

#### TypeScript Configuration (tsconfig.json)
```json
{
  "compilerOptions": {
    "target": "es2022",
    "lib": ["dom", "dom.iterable", "ES2022"],
    "module": "ESNext",
    "moduleResolution": "Bundler",
    "jsx": "preserve",
    "strict": true,
    "noUncheckedIndexedAccess": true,  // Safer array/object access
    "checkJs": true,                   // Type-check JS files
    "esModuleInterop": true,
    "skipLibCheck": true,
    "isolatedModules": true,
    "verbatimModuleSyntax": true,      // Explicit import type
    "noEmit": true,
    "incremental": true,
    "resolveJsonModule": true,
    "paths": {
      "~/*": ["./src/*"],
      "@/*": ["./src/*"]
    }
  }
}
```

**Key Strictness Settings**:
- `strict: true` - All type-checking options enabled
- `noUncheckedIndexedAccess: true` - Array access returns `T | undefined`
- `checkJs: true` - Type-check JavaScript files
- `verbatimModuleSyntax: true` - Requires explicit `import type`

#### Type Definition Patterns
```typescript
// Backend response types (snake_case)
interface BackendGroup {
  id: number;               // int64 in Go
  name: string;
  room_id: number | null;
  created_at: string;       // ISO timestamp
  representative?: {
    id: number;
    staff_id: number;
  };
}

// Frontend types (camelCase)
interface Group {
  id: string;               // Convert int64 to string
  name: string;
  roomId: string | null;
  createdAt: Date;          // Parse to Date object
  representative?: Teacher;
}

// Type mapping helper
export function mapGroupResponse(data: BackendGroup): Group {
  return {
    id: data.id.toString(),
    name: data.name,
    roomId: data.room_id?.toString() ?? null,
    createdAt: new Date(data.created_at),
    representative: data.representative
      ? mapTeacherResponse(data.representative)
      : undefined,
  };
}
```

#### Module System
- **Type**: ESM (ES Modules)
- **Import Style**: Named exports preferred
```typescript
// Named exports
export function fetchGroups(): Promise<Group[]> { }
export type Group = { id: string; name: string };

// Import
import { fetchGroups, type Group } from "~/lib/groups-api";
```

#### File Naming Conventions
- **Components**: `kebab-case.tsx` (e.g., `group-list.tsx`, `student-form.tsx`)
- **Utilities**: `kebab-case.ts` (e.g., `auth-helpers.ts`, `api-client.ts`)
- **API Routes**: `route.ts` (Next.js convention)
- **Pages**: `page.tsx` (Next.js convention)
- **Test Files**: Not currently implemented (would be `*.test.ts`)

#### Naming Conventions
- **Variables**: camelCase (`const groupList = []`)
- **Functions**: camelCase (`function fetchGroups()`)
- **Types/Interfaces**: PascalCase (`interface Group`, `type ApiResponse`)
- **Components**: PascalCase (`function GroupList()`)
- **Constants**: SCREAMING_SNAKE_CASE (`const API_BASE_URL`)
- **Private vars**: Prefix with underscore (`const _unusedVar`)

### Code Style & Linting

#### ESLint Configuration (eslint.config.js)
```javascript
export default tseslint.config(
  { ignores: [".next"] },
  ...compat.extends("next/core-web-vitals"),
  {
    files: ["**/*.ts", "**/*.tsx"],
    extends: [
      ...tseslint.configs.recommended,
      ...tseslint.configs.recommendedTypeChecked,
      ...tseslint.configs.stylisticTypeChecked,
    ],
    rules: {
      "@typescript-eslint/array-type": "off",
      "@typescript-eslint/consistent-type-definitions": "off",
      "@typescript-eslint/consistent-type-imports": [
        "warn",
        { prefer: "type-imports", fixStyle: "inline-type-imports" },
      ],
      "@typescript-eslint/no-unused-vars": [
        "warn",
        { argsIgnorePattern: "^_" },
      ],
      "@typescript-eslint/require-await": "off",
      "@typescript-eslint/no-misused-promises": [
        "error",
        { checksVoidReturn: { attributes: false } },
      ],
    },
    linterOptions: {
      reportUnusedDisableDirectives: true,
    },
  },
);
```

**Key Rules**:
- `consistent-type-imports`: Must use `import type { X }` for types
- `no-unused-vars`: Unused vars must start with `_`
- `prefer-nullish-coalescing`: Use `??` instead of `||` for defaults
- **Zero warnings policy**: `npm run lint` fails if any warnings exist

#### Prettier Configuration (prettier.config.js)
```javascript
export default {
  plugins: ["prettier-plugin-tailwindcss"],  // Auto-sort Tailwind classes
};
```
- Uses Prettier defaults (2 spaces, semicolons, double quotes)
- Tailwind plugin sorts class names consistently

#### Import Ordering
ESLint and Prettier handle import sorting automatically:
```typescript
// 1. React/Next.js
import { useState } from "react";
import { NextRequest, NextResponse } from "next/server";

// 2. External libraries
import axios from "axios";
import { z } from "zod";

// 3. Internal imports (absolute paths via ~/ or @/)
import { auth } from "~/server/auth";
import type { ApiResponse } from "~/lib/api-helpers";
import { mapGroupResponse } from "~/lib/groups-helpers";

// 4. Relative imports
import { Button } from "../ui/button";
```

### Component Organization

#### Backend API Structure
```
api/{domain}/
├── api.go          # Router setup with middleware
├── handlers.go     # HTTP handler functions
└── types.go        # Request/response DTOs (optional)
```

**Example (api/groups/api.go)**:
```go
package groups

import (
    "github.com/go-chi/chi/v5"
    "github.com/moto-nrw/project-phoenix/auth/authorize"
    "github.com/moto-nrw/project-phoenix/auth/jwt"
    "github.com/moto-nrw/project-phoenix/services"
)

type Resource struct {
    service services.GroupService
}

func NewResource(service services.GroupService) *Resource {
    return &Resource{service: service}
}

func (rs *Resource) Router() chi.Router {
    r := chi.NewRouter()

    // Apply JWT authentication to all routes
    r.Use(jwt.Authenticator)

    // Permission-based routes
    r.With(authorize.RequiresPermission("groups.read")).Get("/", rs.list)
    r.With(authorize.RequiresPermission("groups.write")).Post("/", rs.create)
    r.With(authorize.RequiresPermission("groups.read")).Get("/{id}", rs.get)
    r.With(authorize.RequiresPermission("groups.write")).Put("/{id}", rs.update)
    r.With(authorize.RequiresPermission("groups.delete")).Delete("/{id}", rs.delete)

    return r
}
```

#### Frontend Component Structure
```
components/{domain}/
├── {feature}-list.tsx       # List/table components
├── {feature}-form.tsx       # Create/edit forms
├── {feature}-card.tsx       # Display cards
├── {feature}-detail.tsx     # Detail views
└── {feature}-select.tsx     # Dropdown selectors
```

**Naming Pattern**: `{domain}-{component-type}.tsx`
- `group-list.tsx` - Group table
- `student-form.tsx` - Student create/edit form
- `room-card.tsx` - Room display card
- `activity-select.tsx` - Activity dropdown

#### Frontend Service Layer Structure
```
lib/
├── {domain}-api.ts          # Backend API calls
├── {domain}-helpers.ts      # Type transformations
└── {domain}-service.ts      # Complex business logic
```

**Example (lib/groups-api.ts)**:
```typescript
import { apiGet, apiPost, apiPut, apiDelete } from "./api-client";
import type { Group } from "./groups-helpers";
import { mapGroupResponse } from "./groups-helpers";

export async function fetchGroups(token: string): Promise<Group[]> {
  const response = await apiGet("/groups", token);
  return response.data.data.map(mapGroupResponse);
}

export async function createGroup(
  data: CreateGroupRequest,
  token: string
): Promise<Group> {
  const response = await apiPost("/groups", data, token);
  return mapGroupResponse(response.data);
}
```

### Test File Organization

#### Backend Test Location
- **Co-located**: Test files live next to source files
- **Naming**: `{file}_test.go`
- **Example**:
  - Source: `backend/database/repositories/education/group_repository.go`
  - Test: `backend/database/repositories/education/group_repository_test.go`

#### Frontend Test Location (Not Implemented)
- Would follow Next.js convention: `{file}.test.ts` or `{file}.test.tsx`
- Would likely use `__tests__/` directories or co-located tests

---

## 4. Testing Strategy

### Backend Testing (Go)

#### Test Framework
- **Framework**: `testing` (standard library)
- **Assertions**: `github.com/stretchr/testify v1.10.0`
  - `require`: Fatal assertions (stop test on failure)
  - `assert`: Non-fatal assertions (continue test)
  - `mock`: Mocking utilities

#### Test Structure
```go
package education_test

import (
    "context"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"

    "github.com/moto-nrw/project-phoenix/models/education"
    "github.com/moto-nrw/project-phoenix/test"
)

func TestGroupRepository_Create(t *testing.T) {
    // Setup
    db := test.SetupTestDB(t)
    defer test.CleanupTestDB(db)

    repo := education.NewRepository(db)
    ctx := context.Background()

    // Test data
    group := &education.Group{
        Name: "Test Group",
        Type: "class",
    }

    // Execute
    err := repo.Create(ctx, group)

    // Assert
    require.NoError(t, err, "Create should not return error")
    assert.NotZero(t, group.ID, "Created group should have ID")
    assert.Equal(t, "Test Group", group.Name)
}

func TestGroupRepository_Create_DuplicateName(t *testing.T) {
    db := test.SetupTestDB(t)
    defer test.CleanupTestDB(db)

    repo := education.NewRepository(db)
    ctx := context.Background()

    // Create first group
    group1 := &education.Group{Name: "Duplicate"}
    require.NoError(t, repo.Create(ctx, group1))

    // Try to create duplicate
    group2 := &education.Group{Name: "Duplicate"}
    err := repo.Create(ctx, group2)

    assert.Error(t, err, "Should fail on duplicate name")
}
```

#### Test Organization
- **Unit Tests**: Test individual functions/methods in isolation
- **Integration Tests**: Test components together (e.g., service + repository + database)
- **Test Helpers**: `backend/test/helpers.go` - Database setup, test data creation

#### Coverage Requirements
- **Current**: No formal coverage requirements
- **Goal**: Critical paths covered (auth, permissions, data access)

#### Mocking Strategies
- **Interface Mocking**: Use `testify/mock` for interface implementations
- **Database**: Use real test database (not mocks) for integration tests
  - Separate test database or in-memory SQLite (if applicable)
  - Transactions rolled back after each test

#### Running Tests
```bash
# All tests
go test ./...

# Verbose output
go test -v ./api/auth

# With race detection
go test -race ./...

# Specific test
go test ./api/auth -run TestLogin

# With coverage
go test -cover ./...
```

### API Testing (Bruno)

#### Test Framework
- **Tool**: Bruno CLI (command-line API testing)
- **Version**: Latest (installed via npm/homebrew)
- **Test Files**: `.bru` format (custom Bruno DSL)
- **Test Count**: 52 files in `bruno/dev/` directory

#### Test Structure
**Example (bruno/dev/groups.bru)**:
```
meta {
  name: Groups API
  type: http
  seq: 1
}

get {
  url: {{baseUrl}}/api/groups
  body: none
  auth: bearer
}

auth:bearer {
  token: {{accessToken}}
}

assert {
  res.status: eq 200
  res.body.data: isArray
  res.body.data.length: gte 25
}

tests {
  test("Groups returned successfully", function() {
    expect(res.status).to.equal(200);
    expect(res.body.data).to.be.an('array');
    expect(res.body.data.length).to.be.at.least(25);
  });
}
```

#### Test Organization
- **dev/**: Development tests (52 files) - Fast feedback loop
  - `auth.bru` - Authentication flows
  - `groups.bru` - Group CRUD (expects 25 groups)
  - `students.bru` - Student management (expects 50 students)
  - `rooms.bru` - Room management (expects 24 rooms)
  - `device-*.bru` - RFID device integration tests
  - `attendance-*.bru` - Attendance tracking tests
- **examples/**: API usage examples for documentation
- **manual/**: Pre-release manual verification tests

#### Test Execution
```bash
cd bruno

# Quick domain-specific tests (with fresh auth tokens)
./dev-test.sh groups          # ~44ms - Test groups API
./dev-test.sh students        # ~50ms - Test students API
./dev-test.sh rooms           # ~19ms - Test rooms API
./dev-test.sh devices         # ~117ms - RFID device auth
./dev-test.sh attendance      # Web + RFID attendance tests

# Full test suite
./dev-test.sh all             # ~252ms - All tests

# Manual traditional Bruno
bru run dev/ --env Local      # Requires manual token management
bru run dev/groups.bru --env Local --env-var accessToken="$TOKEN"
```

#### Test Timing Benchmarks
- **Groups API**: ~44ms (25 groups)
- **Students API**: ~50ms (50 students)
- **Rooms API**: ~19ms (24 rooms)
- **Device Auth**: ~117ms (two-layer auth)
- **Full Suite**: ~252ms (all 52 tests)

#### Bruno vs Traditional Testing
**Advantages**:
- Simple test runner (`./dev-test.sh`) handles authentication automatically
- Fast execution (~252ms for complete API surface)
- Human-readable `.bru` format
- Minimal setup (just environment file)

**Disadvantages**:
- Not integrated into CI/CD (manual execution)
- No coverage metrics
- Limited assertion capabilities vs Jest/Vitest

### Frontend Testing (Not Implemented)

#### Recommended Framework (Future)
- **Unit/Component**: React Testing Library + Vitest
- **E2E**: Playwright or Cypress
- **API Mocking**: MSW (Mock Service Worker)

#### Potential Test Structure (If Implemented)
```typescript
// components/groups/group-list.test.tsx
import { render, screen, waitFor } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import { GroupList } from "./group-list";
import * as groupsApi from "~/lib/groups-api";

vi.mock("~/lib/groups-api");

describe("GroupList", () => {
  it("renders groups after loading", async () => {
    const mockGroups = [
      { id: "1", name: "Group A", createdAt: new Date() },
      { id: "2", name: "Group B", createdAt: new Date() },
    ];

    vi.spyOn(groupsApi, "fetchGroups").mockResolvedValue(mockGroups);

    render(<GroupList />);

    expect(screen.getByText("Loading...")).toBeInTheDocument();

    await waitFor(() => {
      expect(screen.getByText("Group A")).toBeInTheDocument();
      expect(screen.getByText("Group B")).toBeInTheDocument();
    });
  });
});
```

---

## 5. Build & Development

### Package Manager
- **Frontend**: npm v11.3.0 (enforced in package.json)
- **Backend**: Go modules (go.mod/go.sum)
- **Lock Files**:
  - `frontend/package-lock.json` - npm dependency tree
  - `backend/go.sum` - Go module checksums

### Build Tools

#### Backend (Go)
- **Compiler**: Go 1.23.0 (toolchain 1.24.1)
- **Build Command**: `go build -ldflags="-s -w" -o main .`
  - `-s`: Strip symbol table
  - `-w`: Strip DWARF debug info
  - Produces optimized binary
- **Docker Build**: Multi-stage Dockerfile
  ```dockerfile
  FROM golang:1.23-alpine AS builder
  WORKDIR /src
  COPY go.mod go.sum ./
  RUN go mod download
  COPY . .
  RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o main .

  FROM alpine:latest
  RUN apk add --no-cache ca-certificates tzdata
  COPY --from=builder /src/main /main
  CMD ["/main", "serve"]
  ```

#### Frontend (Next.js)
- **Build Tool**: Next.js built-in compiler (Turbopack for dev, Webpack for prod)
- **Dev Server**: Turbo mode (`next dev --turbo`)
- **Production Build**: `next build` → `.next/` directory
- **Docker Build**: Multi-stage Dockerfile
  ```dockerfile
  FROM node:20-alpine AS deps
  WORKDIR /app
  COPY package*.json ./
  RUN npm ci --only=production

  FROM node:20-alpine AS builder
  WORKDIR /app
  COPY --from=deps /app/node_modules ./node_modules
  COPY . .
  RUN npm run build

  FROM node:20-alpine AS runner
  WORKDIR /app
  COPY --from=builder /app/.next ./.next
  COPY --from=builder /app/public ./public
  CMD ["npm", "start"]
  ```

### Development Workflow

#### Starting Development Environment
```bash
# 1. Automated setup (first time only)
./scripts/setup-dev.sh          # Creates .env, SSL certs, configs

# 2. Start services with Docker Compose
docker compose up -d postgres   # Start database first
docker compose up -d            # Start all services

# 3. Run migrations and seed data (first time)
docker compose exec server ./main migrate
docker compose exec server ./main seed

# 4. Access services
# - Backend: http://localhost:8080
# - Frontend: http://localhost:3000
# - PostgreSQL: localhost:5432 (with SSL)
```

#### Backend Development
```bash
cd backend

# Start server (reads dev.env)
go run main.go serve            # Port 8080

# Database operations
go run main.go migrate          # Run pending migrations
go run main.go migrate status   # Check migration status
go run main.go seed             # Populate test data (150 persons, 25 groups, 24 rooms)
go run main.go seed --reset     # Clear and re-seed

# Generate API documentation
go run main.go gendoc           # Creates docs/routes.md + docs/openapi.yaml

# Code quality
golangci-lint run --timeout 10m # Run linter
golangci-lint run --fix         # Auto-fix issues
go fmt ./...                    # Format code
/Users/yonnock/go/bin/goimports -w .  # Organize imports
go mod tidy                     # Clean dependencies

# Testing
go test ./...                   # All tests
go test -v ./api/auth           # Specific package
go test -race ./...             # Race detection
```

#### Frontend Development
```bash
cd frontend

# Start dev server (with hot reload)
npm run dev                     # http://localhost:3000 (Turbo mode)

# Code quality (ALWAYS run before committing!)
npm run check                   # Lint + typecheck (0 warnings policy)
npm run lint                    # ESLint only
npm run lint:fix                # Auto-fix linting issues
npm run typecheck               # TypeScript type checking only

# Formatting
npm run format:check            # Check Prettier formatting
npm run format:write            # Auto-format files

# Production build
npm run build                   # Build for production
npm run start                   # Start production server
npm run preview                 # Build + start production
```

### Available npm Scripts (Frontend)
```json
{
  "scripts": {
    "build": "next build",                                 // Production build
    "check": "next lint && tsc --noEmit",                 // Lint + typecheck
    "check:fresh": "rm -f *.tsbuildinfo && next lint && tsc --noEmit",  // Clean check
    "dev": "next dev --turbo",                            // Dev server with Turbo
    "format:check": "prettier --check \"**/*.{ts,tsx,js,jsx,mdx}\" --cache",
    "format:write": "prettier --write \"**/*.{ts,tsx,js,jsx,mdx}\" --cache",
    "lint": "next lint",                                  // ESLint only
    "lint:fix": "next lint --fix",                        // Auto-fix lint issues
    "preview": "next build && next start",                // Build + production server
    "start": "next start",                                // Production server
    "typecheck": "tsc --noEmit"                          // TypeScript check only
  }
}
```

### Environment Setup Requirements

#### System Requirements
- **Go**: 1.23+ (1.24.1 toolchain recommended)
- **Node.js**: 20+ (Alpine-based Docker images use Node 20)
- **npm**: 11.3.0 (enforced)
- **Docker**: Latest with Compose v2+
- **PostgreSQL**: 17+ (Alpine image in Docker)
- **Operating System**: macOS (development), Linux (production)

#### Environment Files
1. **Root `.env`** (Docker Compose):
   ```bash
   TZ=Europe/Berlin
   POSTGRES_PASSWORD=your_secure_db_password
   AUTH_JWT_SECRET=your_jwt_secret_at_least_32_chars_long
   NEXTAUTH_SECRET=your_nextauth_secret_at_least_32_chars
   ```

2. **`backend/dev.env`** (Backend config):
   ```bash
   DB_DSN=postgres://postgres:password@localhost:5432/postgres?sslmode=require
   AUTH_JWT_SECRET=your_jwt_secret_at_least_32_chars_long
   ADMIN_EMAIL=admin@example.com
   ADMIN_PASSWORD=Test1234%
   LOG_LEVEL=debug
   DB_DEBUG=true
   ```

3. **`frontend/.env.local`** (Frontend config):
   ```bash
   NEXT_PUBLIC_API_URL=http://localhost:8080
   NEXTAUTH_URL=http://localhost:3000
   NEXTAUTH_SECRET=your_nextauth_secret
   ```

#### SSL Certificate Generation (REQUIRED)
```bash
cd config/ssl/postgres
chmod +x create-certs.sh
./create-certs.sh               # Generates CA + server certs

# Files created in certs/ (git-ignored):
# - ca.crt, ca.key          # Certificate Authority
# - server.crt, server.key  # PostgreSQL server certificate
```

### Hot Reload/Watch Mode Setup

#### Backend (Go)
- **Docker Compose Watch Mode**:
  ```yaml
  develop:
    watch:
      - action: rebuild
        path: ./backend
        ignore:
          - backend/bin/
          - backend/*.log
  ```
  - **Trigger**: Any Go file change
  - **Action**: Rebuild container (CGO disabled, fast compilation)
  - **CRITICAL**: Must rebuild container after Go code changes (`docker compose build server`)

#### Frontend (Next.js)
- **Turbo Mode**: `next dev --turbo` (faster than Webpack)
- **Docker Volume Mounts**: Hot reload via mounted directories
  ```yaml
  volumes:
    - ./frontend/src:/app/src              # Source code
    - ./frontend/public:/app/public        # Static assets
    - ./frontend/tsconfig.json:/app/tsconfig.json
    - frontend_node_modules:/app/node_modules  # Named volume
  ```
  - **Trigger**: File save in mounted directories
  - **Action**: Automatic page reload (Fast Refresh)

---

## 6. Dependencies & Integrations

### Critical Backend Dependencies

#### HTTP & Routing
- **chi/v5 v5.2.0**: Lightweight HTTP router
  - Used for: Route definition, middleware chaining, RESTful patterns
  - Why: Fast, idiomatic Go, excellent middleware support
- **cors v1.2.1**: CORS middleware
  - Configured via `ENABLE_CORS` + `CORS_ALLOWED_ORIGINS`

#### Database & ORM
- **uptrace/bun v1.2.11**: SQL-first ORM
  - Used for: Database queries, migrations, relation loading
  - Why: Better PostgreSQL support than GORM, schema-aware queries
  - Dialects: pgdialect v1.2.11
  - Driver: pgdriver v1.2.11 (native, no CGO)
  - Debug: bundebug v1.2.11 (SQL logging)

#### Authentication
- **jwtauth/v5 v5.3.2**: JWT middleware for Chi
  - Used for: Token verification, user context
- **lestrrat-go/jwx/v2 v2.1.3**: JWT/JWS/JWE implementation
  - Used for: Token generation, signing, parsing
- **golang.org/x/crypto v0.36.0**: Cryptographic functions
  - Used for: Argon2id password hashing, bcrypt
  - **Security**: Argon2id with memory=64MB, iterations=3, parallelism=2

#### Validation & Data
- **ozzo-validation v3.6.0**: Struct validation
  - Used for: Model validation, custom rules
- **gofrs/uuid v4.4.0**: UUID generation
  - Used for: Token IDs, unique identifiers

#### Email
- **wneessen/go-mail v0.6.2**: SMTP email client
  - Used for: Sending emails (password reset, notifications)
- **vanng822/go-premailer v1.22.0**: HTML email inliner
  - Used for: Convert CSS to inline styles for email compatibility

#### CLI & Configuration
- **spf13/cobra v1.8.1**: CLI framework
  - Used for: Command structure (serve, migrate, seed, gendoc)
- **spf13/viper v1.19.0**: Configuration management
  - Used for: Reading dev.env, environment variables

#### Logging
- **sirupsen/logrus v1.9.3**: Structured logging
  - Used for: Application logging, security events

#### Testing
- **stretchr/testify v1.10.0**: Testing utilities
  - Components: require, assert, mock

### Critical Frontend Dependencies

#### Core Framework
- **next v15.2.3**: React framework
  - Used for: App Router, server-side rendering, API routes
  - Why: Industry standard, excellent DX, built-in optimizations
- **react v19.0.0**: UI library
  - Features: Server components, automatic batching, transitions
- **react-dom v19.0.0**: DOM renderer

#### TypeScript & Validation
- **typescript v5.8.2**: Type system
  - Config: Strict mode, noUncheckedIndexedAccess
- **zod v3.24.2**: Schema validation
  - Used for: Environment variable validation, form validation
- **@t3-oss/env-nextjs v0.12.0**: Type-safe env with Zod
  - Used for: src/env.js - validates client/server env vars

#### Styling
- **tailwindcss v4.0.15**: Utility-first CSS framework
  - Used for: All component styling
  - Why: Fast development, consistent design, small bundle size
- **@tailwindcss/postcss v4.0.15**: Tailwind PostCSS plugin
- **lucide-react v0.509.0**: Icon library
  - Used for: UI icons (chevrons, checkmarks, etc.)

#### Authentication
- **next-auth v5.0.0-beta.25**: Authentication for Next.js
  - Strategy: JWT (custom provider with backend API)
  - Used for: Session management, token refresh

#### HTTP Client
- **axios v1.8.4**: Promise-based HTTP client
  - Used for: Backend API calls, request/response interceptors
  - Why: Better error handling than fetch, interceptor support

#### UI Effects
- **canvas-confetti v1.9.3**: Confetti animations
  - Used for: Success celebrations (attendance, check-in)

#### Fonts
- **@fontsource/geist-sans v5.2.5**: Geist font family
  - Used for: Primary UI font

#### Development Tools
- **eslint v9.23.0**: JavaScript linter
- **typescript-eslint v8.27.0**: TypeScript ESLint plugin
- **prettier v3.5.3**: Code formatter
- **prettier-plugin-tailwindcss v0.6.11**: Tailwind class sorter
- **sharp v0.34.1**: Image optimization (Next.js requirement)

### Internal Package Dependencies (Monorepo)

#### Backend Service Dependencies
```
API Layer (api/)
  ↓ depends on
Services Layer (services/)
  ↓ depends on
Repository Layer (database/repositories/)
  ↓ depends on
Models Layer (models/)

Cross-cutting:
- auth/ (used by API, Services)
- middleware/ (used by API)
- logging/ (used by all layers)
```

**Factory Pattern Dependencies**:
```
services.Factory
  ↓ requires
repositories.Factory
  ↓ requires
*bun.DB (database connection)
```

### External APIs and Services

#### Email Service (SMTP)
- **Provider**: Configurable via environment
  ```bash
  EMAIL_SMTP_HOST=smtp.gmail.com
  EMAIL_SMTP_PORT=465
  EMAIL_SMTP_USER=your_email@gmail.com
  EMAIL_SMTP_PASSWORD=app_password
  ```
- **Used for**: Password reset, notifications (future)

#### No External SaaS Dependencies
- All services run locally or in Docker
- No cloud APIs (AWS, Azure, GCP)
- No third-party analytics or monitoring

### Authentication/Authorization Approach

#### Backend Authentication Flow
1. **Login** (`POST /auth/login`):
   ```
   User submits email + password
     ↓
   UserPass authenticator validates credentials
     ↓
   Argon2id password verification
     ↓
   Generate JWT access token (15min) + refresh token (1hr)
     ↓
   Clean up old tokens for user (prevent accumulation)
     ↓
   Return tokens to client
   ```

2. **Token Storage**:
   - Access tokens stored in `auth.tokens` table
   - Refresh tokens stored with `is_refresh=true` flag
   - Cleanup on login prevents token buildup

3. **Authorization**:
   - JWT middleware extracts token from `Authorization: Bearer <token>`
   - User ID stored in request context
   - Permission middleware checks `auth.role_permissions` table
   - Policies evaluate access based on user context (GDPR compliance)

#### Frontend Authentication Flow
1. **NextAuth Configuration** (src/server/auth.ts):
   ```typescript
   providers: [
     CredentialsProvider({
       async authorize(credentials) {
         const response = await fetch("http://backend:8080/auth/login", {
           method: "POST",
           body: JSON.stringify({ email, password }),
         });
         const data = await response.json();
         return { id: data.user_id, token: data.access_token };
       },
     }),
   ],
   callbacks: {
     async jwt({ token, user }) {
       if (user) token.accessToken = user.token;
       return token;
     },
     async session({ session, token }) {
       session.user.token = token.accessToken;
       return session;
     },
   },
   ```

2. **API Route Wrapper** (lib/route-wrapper.ts):
   - Extracts JWT from NextAuth session
   - Passes token to backend via Authorization header
   - Handles 401 errors (token expired)
   - Retries with refreshed token if available

3. **Token Refresh**:
   - Not currently implemented in frontend
   - Backend supports `POST /auth/refresh` endpoint
   - Future: Automatic refresh before expiry

### Database(s) Used

#### PostgreSQL 17
- **Image**: `postgres:17-alpine` (Docker)
- **Connection**: SSL required (`sslmode=require`)
- **Pooling**: Managed by Bun ORM
- **Timezone**: Europe/Berlin (set in all containers)

#### Schema Organization (11 Schemas)
```sql
-- Authentication & Authorization
auth                -- accounts, tokens, roles, permissions

-- User Management
users               -- persons, staff, students, teachers, guardians

-- Educational Structure
education           -- groups, substitutions

-- Time Management
schedule            -- timeframes, dateframes, recurrence patterns

-- Activities
activities          -- activities, enrollments, activity types

-- Facilities
facilities          -- rooms, buildings

-- IoT & Devices
iot                 -- devices (RFID readers), rfid_cards

-- User Feedback
feedback            -- feedback entries

-- Real-time Tracking
active              -- visits, attendance, group_supervisors, groups

-- System Configuration
config              -- system_configs

-- Metadata
meta                -- database metadata
```

#### ORM/Query Approach
- **ORM**: Bun (SQL-first, not ActiveRecord pattern)
- **Query Builder**: Bun's fluent API
  ```go
  // Example: Load groups with rooms
  var groups []*education.Group
  err := db.NewSelect().
      Model(&groups).
      ModelTableExpr(`education.groups AS "group"`).
      Relation("Room").
      Where(`"group".active = ?`, true).
      Order(`"group".name ASC`).
      Scan(ctx)
  ```
- **Raw SQL**: Supported for complex queries
  ```go
  var result struct {
      Count int
  }
  err := db.NewRaw(`
      SELECT COUNT(*) as count
      FROM active.visits
      WHERE end_time IS NULL
  `).Scan(ctx, &result)
  ```

---

## 7. Common Workflows

### How to Add a New Feature (Step by Step)

#### Backend Feature (Example: Add "Class Schedule" Feature)

1. **Define Model** (`backend/models/schedule/class_schedule.go`):
   ```go
   package schedule

   type ClassSchedule struct {
       ID        int64     `bun:"id,pk,autoincrement"`
       GroupID   int64     `bun:"group_id,notnull"`
       DayOfWeek int       `bun:"day_of_week,notnull"`
       StartTime string    `bun:"start_time,notnull"`
       EndTime   string    `bun:"end_time,notnull"`
       CreatedAt time.Time `bun:"created_at,default:now()"`

       // Relations
       Group *education.Group `bun:"rel:belongs-to,join:group_id=id"`
   }

   func (cs *ClassSchedule) Validate() error {
       return validation.ValidateStruct(cs,
           validation.Field(&cs.GroupID, validation.Required),
           validation.Field(&cs.DayOfWeek, validation.Min(0), validation.Max(6)),
       )
   }
   ```

2. **Create Migration** (`backend/database/migrations/004005001_schedule_class_schedules.go`):
   ```go
   const (
       ClassSchedulesVersion = "4.5.1"
       ClassSchedulesDescription = "Create schedule.class_schedules table"
   )

   var ClassSchedulesDependencies = []string{
       "3.0.1",  // education.groups must exist
   }

   func init() {
       Migrations.MustRegister(
           func(ctx context.Context, db *bun.DB) error {
               _, err := db.ExecContext(ctx, `
                   CREATE TABLE schedule.class_schedules (
                       id BIGSERIAL PRIMARY KEY,
                       group_id BIGINT NOT NULL REFERENCES education.groups(id),
                       day_of_week INT NOT NULL CHECK (day_of_week BETWEEN 0 AND 6),
                       start_time TIME NOT NULL,
                       end_time TIME NOT NULL,
                       created_at TIMESTAMP DEFAULT NOW(),
                       UNIQUE(group_id, day_of_week, start_time)
                   )
               `)
               return err
           },
           func(ctx context.Context, db *bun.DB) error {
               _, err := db.ExecContext(ctx, `DROP TABLE IF EXISTS schedule.class_schedules CASCADE`)
               return err
           },
       )
   }
   ```

3. **Create Repository** (`backend/database/repositories/schedule/class_schedule_repository.go`):
   ```go
   type ClassScheduleRepository interface {
       Create(ctx context.Context, schedule *schedule.ClassSchedule) error
       GetByGroupID(ctx context.Context, groupID int64) ([]*schedule.ClassSchedule, error)
   }

   type classScheduleRepository struct {
       db *bun.DB
   }

   func NewClassScheduleRepository(db *bun.DB) ClassScheduleRepository {
       return &classScheduleRepository{db: db}
   }

   func (r *classScheduleRepository) Create(ctx context.Context, cs *schedule.ClassSchedule) error {
       _, err := r.db.NewInsert().Model(cs).Exec(ctx)
       return err
   }
   ```

4. **Add to Repository Factory** (`backend/database/repositories/factory.go`):
   ```go
   func (f *Factory) NewClassScheduleRepository() schedule.ClassScheduleRepository {
       return scheduleRepo.NewClassScheduleRepository(f.db)
   }
   ```

5. **Create Service** (`backend/services/schedule/class_schedule_service.go`):
   ```go
   type ClassScheduleService interface {
       CreateSchedule(ctx context.Context, schedule *schedule.ClassSchedule) error
       GetGroupSchedules(ctx context.Context, groupID int64) ([]*schedule.ClassSchedule, error)
   }

   type classScheduleService struct {
       repo schedule.ClassScheduleRepository
   }

   func NewClassScheduleService(repo schedule.ClassScheduleRepository) ClassScheduleService {
       return &classScheduleService{repo: repo}
   }
   ```

6. **Add to Service Factory** (`backend/services/factory.go`):
   ```go
   func (f *Factory) NewClassScheduleService() schedule.ClassScheduleService {
       return scheduleService.NewClassScheduleService(
           f.repos.NewClassScheduleRepository(),
       )
   }
   ```

7. **Create API Handlers** (`backend/api/schedules/class_schedule_handlers.go`):
   ```go
   func (rs *Resource) createClassSchedule(w http.ResponseWriter, r *http.Request) {
       var req CreateClassScheduleRequest
       if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
           render.Status(r, http.StatusBadRequest)
           render.JSON(w, r, ErrorResponse{Status: "error", Message: "Invalid request"})
           return
       }

       schedule := &schedule.ClassSchedule{
           GroupID:   req.GroupID,
           DayOfWeek: req.DayOfWeek,
           StartTime: req.StartTime,
           EndTime:   req.EndTime,
       }

       if err := rs.classScheduleService.CreateSchedule(r.Context(), schedule); err != nil {
           render.Status(r, http.StatusInternalServerError)
           render.JSON(w, r, ErrorResponse{Status: "error", Message: err.Error()})
           return
       }

       render.JSON(w, r, ApiResponse{Status: "success", Data: schedule})
   }
   ```

8. **Add Routes** (`backend/api/schedules/api.go`):
   ```go
   func (rs *Resource) Router() chi.Router {
       r := chi.NewRouter()
       r.Use(jwt.Authenticator)

       r.With(authorize.RequiresPermission("schedules.write")).
           Post("/class-schedules", rs.createClassSchedule)
       r.With(authorize.RequiresPermission("schedules.read")).
           Get("/class-schedules/group/{groupID}", rs.getGroupSchedules)

       return r
   }
   ```

9. **Run Migration**:
   ```bash
   go run main.go migrate
   ```

10. **Test with Bruno** (`bruno/dev/class-schedules.bru`):
    ```
    post {
      url: {{baseUrl}}/api/schedules/class-schedules
      body: json
    }

    body:json {
      {
        "group_id": 1,
        "day_of_week": 1,
        "start_time": "08:00",
        "end_time": "09:30"
      }
    }
    ```

11. **Update API Documentation**:
    ```bash
    go run main.go gendoc
    ```

#### Frontend Feature (Example: Add "Class Schedule Display")

1. **Check Backend API** (from `docs/routes.md`):
   ```
   GET /api/schedules/class-schedules/group/{groupID}
   POST /api/schedules/class-schedules
   ```

2. **Define Types** (`frontend/src/lib/schedule-helpers.ts`):
   ```typescript
   // Backend response type
   interface BackendClassSchedule {
     id: number;
     group_id: number;
     day_of_week: number;
     start_time: string;
     end_time: string;
     created_at: string;
   }

   // Frontend type
   export interface ClassSchedule {
     id: string;
     groupId: string;
     dayOfWeek: number;
     startTime: string;
     endTime: string;
     createdAt: Date;
   }

   // Type mapping
   export function mapClassScheduleResponse(data: BackendClassSchedule): ClassSchedule {
     return {
       id: data.id.toString(),
       groupId: data.group_id.toString(),
       dayOfWeek: data.day_of_week,
       startTime: data.start_time,
       endTime: data.end_time,
       createdAt: new Date(data.created_at),
     };
   }
   ```

3. **Create API Client** (`frontend/src/lib/schedule-api.ts`):
   ```typescript
   import { apiGet, apiPost } from "./api-client";
   import type { ClassSchedule } from "./schedule-helpers";
   import { mapClassScheduleResponse } from "./schedule-helpers";

   export async function fetchGroupSchedules(
     groupId: string,
     token: string
   ): Promise<ClassSchedule[]> {
     const response = await apiGet(
       `/schedules/class-schedules/group/${groupId}`,
       token
     );
     return response.data.data.map(mapClassScheduleResponse);
   }

   export async function createClassSchedule(
     data: CreateScheduleRequest,
     token: string
   ): Promise<ClassSchedule> {
     const response = await apiPost("/schedules/class-schedules", data, token);
     return mapClassScheduleResponse(response.data);
   }
   ```

4. **Create Next.js API Route** (`frontend/src/app/api/schedules/class-schedules/group/[groupId]/route.ts`):
   ```typescript
   import { createGetHandler } from "~/lib/route-wrapper";
   import { fetchGroupSchedules } from "~/lib/schedule-api";

   export const GET = createGetHandler(async (request, token, params) => {
     const groupId = params.groupId as string;
     const schedules = await fetchGroupSchedules(groupId, token);
     return { data: schedules };
   });
   ```

5. **Create UI Component** (`frontend/src/components/schedules/schedule-list.tsx`):
   ```typescript
   "use client";

   import { useEffect, useState } from "react";
   import type { ClassSchedule } from "~/lib/schedule-helpers";

   const DAYS = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"];

   export function ScheduleList({ groupId }: { groupId: string }) {
     const [schedules, setSchedules] = useState<ClassSchedule[]>([]);
     const [loading, setLoading] = useState(true);

     useEffect(() => {
       fetch(`/api/schedules/class-schedules/group/${groupId}`)
         .then((res) => res.json())
         .then((data) => setSchedules(data.data))
         .finally(() => setLoading(false));
     }, [groupId]);

     if (loading) return <div>Loading schedules...</div>;

     return (
       <div className="space-y-2">
         {schedules.map((schedule) => (
           <div key={schedule.id} className="rounded border p-4">
             <div className="font-medium">{DAYS[schedule.dayOfWeek]}</div>
             <div className="text-sm text-gray-600">
               {schedule.startTime} - {schedule.endTime}
             </div>
           </div>
         ))}
       </div>
     );
   }
   ```

6. **Add to Page** (`frontend/src/app/(auth)/groups/[id]/page.tsx`):
   ```typescript
   import { ScheduleList } from "~/components/schedules/schedule-list";

   export default async function GroupDetailPage({
     params,
   }: {
     params: Promise<{ id: string }>;
   }) {
     const { id } = await params;

     return (
       <div>
         <h1>Group Details</h1>
         <ScheduleList groupId={id} />
       </div>
     );
   }
   ```

7. **Test Integration**:
   ```bash
   # Backend
   cd backend && go run main.go serve

   # Frontend
   cd frontend && npm run dev

   # Visit http://localhost:3000/groups/1
   ```

8. **Run Quality Checks**:
   ```bash
   cd frontend
   npm run check  # Lint + typecheck (must pass with 0 warnings)
   ```

### How to Fix a Bug (Process)

1. **Reproduce the Bug**:
   - Identify exact steps to trigger issue
   - Document expected vs actual behavior
   - Check logs for errors

2. **Locate the Source**:
   - **Frontend Bug**: Check browser console, Network tab
   - **Backend Bug**: Check server logs (`docker compose logs server`)
   - **Database Bug**: Check PostgreSQL logs, run query manually

3. **Write a Test** (Test-Driven Development):
   ```go
   // backend/api/groups/handlers_test.go
   func TestUpdateGroup_DuplicateName(t *testing.T) {
       // Setup
       db := test.SetupTestDB(t)
       defer test.CleanupTestDB(db)

       // Create existing group
       existing := &education.Group{Name: "Math Class"}
       require.NoError(t, repo.Create(ctx, existing))

       // Try to rename another group to duplicate name
       other := &education.Group{Name: "Science Class"}
       require.NoError(t, repo.Create(ctx, other))

       other.Name = "Math Class"
       err := repo.Update(ctx, other)

       // Assert error
       assert.Error(t, err, "Should fail on duplicate name")
       assert.Contains(t, err.Error(), "duplicate")
   }
   ```

4. **Fix the Issue**:
   - Make minimal, focused changes
   - Follow existing patterns
   - Add validation/checks to prevent regression

5. **Verify the Fix**:
   ```bash
   # Run specific test
   go test ./api/groups -run TestUpdateGroup_DuplicateName -v

   # Run all related tests
   go test ./api/groups -v

   # Test with Bruno
   cd bruno && ./dev-test.sh groups
   ```

6. **Code Quality**:
   ```bash
   # Backend
   golangci-lint run --fix
   go fmt ./...
   /Users/yonnock/go/bin/goimports -w .

   # Frontend
   npm run check
   npm run format:write
   ```

7. **Commit**:
   ```bash
   git add .
   git commit -m "fix: prevent duplicate group names on update

   - Add unique constraint validation in repository layer
   - Add test for duplicate name detection
   - Update error message to be user-friendly"
   ```

### How to Run Tests

#### Backend Tests
```bash
cd backend

# All tests
go test ./...

# Specific package
go test ./api/auth -v

# Specific test
go test ./api/auth -run TestLogin -v

# With race detection
go test -race ./...

# With coverage
go test -cover ./...
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out  # View in browser
```

#### API Tests (Bruno)
```bash
cd bruno

# Quick domain tests (recommended)
./dev-test.sh groups          # ~44ms
./dev-test.sh students        # ~50ms
./dev-test.sh rooms           # ~19ms
./dev-test.sh devices         # ~117ms
./dev-test.sh attendance      # Web + RFID tests

# Full test suite
./dev-test.sh all             # ~252ms

# Manual Bruno CLI
bru run dev/ --env Local
bru run dev/groups.bru --env Local --env-var accessToken="$(get_token)"
```

#### Frontend Tests
- **Not implemented** (no test infrastructure currently)
- Future: `npm test` would run Vitest/Jest

### How to Create a Build

#### Backend Build
```bash
cd backend

# Local build
go build -ldflags="-s -w" -o main .

# Docker build
docker build -t project-phoenix-backend .

# Docker Compose build
docker compose build server
```

**Output**:
- Binary: `backend/main` (~15MB stripped)
- Docker image: Multi-stage Alpine-based (~25MB)

#### Frontend Build
```bash
cd frontend

# Local build
npm run build

# Docker build
docker build -t project-phoenix-frontend .

# Docker Compose build
docker compose build frontend
```

**Output**:
- `.next/` directory (optimized pages, static assets)
- Docker image: Multi-stage Node Alpine-based (~200MB)

#### Full Stack Build
```bash
# From project root
docker compose build

# Outputs:
# - project-phoenix-server:latest
# - project-phoenix-frontend:latest
# - postgres:17-alpine (pulled)
```

### Git Workflow

#### Branching Strategy
- **main**: Production-ready code (protected)
- **development**: Integration branch for features
- **feature/***: New features (`feature/class-schedules`)
- **fix/***: Bug fixes (`fix/duplicate-group-names`)
- **refactor/***: Code improvements (`refactor/backend`)

#### Workflow
```bash
# 1. Create feature branch from development
git checkout development
git pull origin development
git checkout -b feature/class-schedules

# 2. Make changes
# ... edit files ...

# 3. Check status
git status
git diff

# 4. Stage specific files (NEVER use git add .)
git add backend/models/schedule/class_schedule.go
git add backend/database/migrations/004005001_schedule_class_schedules.go
git add backend/api/schedules/class_schedule_handlers.go

# 5. Commit with conventional format
git commit -m "feat: add class schedule feature

- Create class_schedule model with validation
- Add migration for schedule.class_schedules table
- Implement repository and service layers
- Add API endpoints for CRUD operations
- Test with Bruno API tests"

# 6. Push to remote
git push origin feature/class-schedules

# 7. Create Pull Request (targeting development, NOT main)
gh pr create --base development --title "feat: add class schedule feature" --body "..."
```

#### Commit Conventions
```
<type>: <subject>

<body>

<footer>
```

**Types**:
- `feat:` - New feature
- `fix:` - Bug fix
- `refactor:` - Code restructuring
- `chore:` - Maintenance tasks
- `docs:` - Documentation only
- `test:` - Add/update tests
- `style:` - Formatting changes

**Example**:
```
fix: prevent duplicate group names on update

- Add unique constraint check in repository
- Return user-friendly error message
- Add test coverage for edge case

Closes #123
```

**CRITICAL**: Never include "Co-Authored-By: Claude" or similar in commits

### Deployment Process

#### Docker Compose Deployment (Development)
```bash
# 1. Generate SSL certificates (first time only)
cd config/ssl/postgres && ./create-certs.sh && cd ../../..

# 2. Create environment files
cp .env.example .env
cp backend/dev.env.example backend/dev.env
cp frontend/.env.local.example frontend/.env.local
# Edit with production values

# 3. Build and start services
docker compose build
docker compose up -d

# 4. Run migrations
docker compose exec server ./main migrate

# 5. Seed data (development only)
docker compose exec server ./main seed

# 6. Check health
docker compose ps
docker compose logs -f server
docker compose logs -f frontend
```

#### Production Deployment (Future)
- **Not currently configured** (no production Docker Compose)
- Would require:
  - `docker-compose.prod.yml` with production settings
  - Real SSL certificates (Let's Encrypt)
  - Environment-specific secrets (not committed)
  - Reverse proxy (nginx/Traefik)
  - Database backups
  - Monitoring/logging (Prometheus, Grafana)

---

## 8. Error Handling

### Backend Error Handling Patterns

#### Standard Error Response
```go
type ErrorResponse struct {
    Status  string `json:"status"`   // Always "error"
    Message string `json:"message"`  // Human-readable error
    Code    string `json:"code,omitempty"`  // Machine-readable code (optional)
}

// Example usage
render.Status(r, http.StatusBadRequest)
render.JSON(w, r, ErrorResponse{
    Status:  "error",
    Message: "Group name already exists",
    Code:    "DUPLICATE_NAME",
})
```

#### Error Handling in Layers

**Handler Layer** (HTTP errors):
```go
func (rs *Resource) create(w http.ResponseWriter, r *http.Request) {
    var req CreateRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        render.Status(r, http.StatusBadRequest)
        render.JSON(w, r, ErrorResponse{
            Status:  "error",
            Message: "Invalid JSON",
            Code:    "INVALID_JSON",
        })
        return
    }

    result, err := rs.service.Create(r.Context(), &req)
    if err != nil {
        // Check for specific error types
        if errors.Is(err, ErrDuplicateName) {
            render.Status(r, http.StatusConflict)
            render.JSON(w, r, ErrorResponse{
                Status:  "error",
                Message: err.Error(),
                Code:    "DUPLICATE_NAME",
            })
            return
        }

        // Generic server error
        render.Status(r, http.StatusInternalServerError)
        render.JSON(w, r, ErrorResponse{
            Status:  "error",
            Message: "Internal server error",
        })
        return
    }

    render.JSON(w, r, ApiResponse{Status: "success", Data: result})
}
```

**Service Layer** (business logic errors):
```go
var (
    ErrDuplicateName = errors.New("name already exists")
    ErrNotFound      = errors.New("not found")
    ErrUnauthorized  = errors.New("unauthorized")
)

func (s *service) Create(ctx context.Context, req *Request) (*Result, error) {
    // Validate
    if err := req.Validate(); err != nil {
        return nil, fmt.Errorf("validation failed: %w", err)
    }

    // Check duplicates
    existing, err := s.repo.GetByName(ctx, req.Name)
    if err == nil && existing != nil {
        return nil, ErrDuplicateName
    }

    // Create
    result, err := s.repo.Create(ctx, req)
    if err != nil {
        return nil, fmt.Errorf("failed to create: %w", err)
    }

    return result, nil
}
```

**Repository Layer** (data access errors):
```go
func (r *repository) Create(ctx context.Context, item *Item) error {
    _, err := r.db.NewInsert().Model(item).Exec(ctx)
    if err != nil {
        // Wrap with context
        return fmt.Errorf("failed to insert item: %w", err)
    }
    return nil
}
```

### Frontend Error Handling

#### API Error Handling (route-wrapper.ts)
```typescript
export function createGetHandler<T>(
  handler: (request: NextRequest, token: string, params: Record<string, unknown>) => Promise<T>
) {
  return async (
    request: NextRequest,
    context: { params: Promise<Record<string, string | string[] | undefined>> }
  ): Promise<NextResponse<ApiResponse<T> | ApiErrorResponse | T>> => {
    try {
      const session = await auth();

      if (!session?.user?.token) {
        return NextResponse.json(
          { error: "Unauthorized" },
          { status: 401 }
        );
      }

      try {
        const data = await handler(request, session.user.token, safeParams);
        return NextResponse.json({ success: true, data });
      } catch (handlerError) {
        // Handle 401 (token expired)
        if (handlerError instanceof Error && handlerError.message.includes("API error (401)")) {
          const updatedSession = await auth();

          // Retry with refreshed token
          if (updatedSession?.user?.token && updatedSession.user.token !== session.user.token) {
            const retryData = await handler(request, updatedSession.user.token, safeParams);
            return NextResponse.json({ success: true, data: retryData });
          }

          return NextResponse.json(
            { error: "Token expired", code: "TOKEN_EXPIRED" },
            { status: 401 }
          );
        }
        throw handlerError;
      }
    } catch (error) {
      return handleApiError(error);
    }
  };
}

// Generic error handler
function handleApiError(error: unknown): NextResponse<ApiErrorResponse> {
  console.error("API Error:", error);

  if (error instanceof Error) {
    // Extract status from error message if present
    const match = error.message.match(/API error \((\d+)\)/);
    if (match) {
      const status = parseInt(match[1] ?? "500", 10);
      return NextResponse.json(
        { error: error.message },
        { status }
      );
    }
  }

  return NextResponse.json(
    { error: "Internal server error" },
    { status: 500 }
  );
}
```

#### Component Error Handling
```typescript
"use client";

import { useState } from "react";

export function GroupForm() {
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    setLoading(true);

    try {
      const response = await fetch("/api/groups", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ name: "New Group" }),
      });

      if (!response.ok) {
        const errorData = await response.json();
        throw new Error(errorData.error ?? "Failed to create group");
      }

      // Success
      window.location.href = "/groups";
    } catch (err) {
      setError(err instanceof Error ? err.message : "An error occurred");
    } finally {
      setLoading(false);
    }
  };

  return (
    <form onSubmit={handleSubmit}>
      {error && (
        <div className="rounded bg-red-50 p-4 text-red-800">
          {error}
        </div>
      )}
      {/* Form fields */}
      <button type="submit" disabled={loading}>
        {loading ? "Creating..." : "Create Group"}
      </button>
    </form>
  );
}
```

### Logging Approach

#### Backend Logging (logrus)
```go
import "github.com/sirupsen/logrus"

// Structured logging
logrus.WithFields(logrus.Fields{
    "user_id": userID,
    "action":  "create_group",
    "group_id": group.ID,
}).Info("Group created successfully")

// Log levels
logrus.Debug("Detailed debug information")
logrus.Info("General information")
logrus.Warn("Warning message")
logrus.Error("Error occurred but recoverable")
logrus.Fatal("Critical error, application will exit")

// Error logging with context
if err != nil {
    logrus.WithFields(logrus.Fields{
        "error": err.Error(),
        "user_id": userID,
    }).Error("Failed to create group")
    return err
}
```

**Log Configuration** (dev.env):
```bash
LOG_LEVEL=debug             # debug, info, warn, error
LOG_TEXTLOGGING=true        # Use text format (not JSON)
```

#### Frontend Logging
```typescript
// Console logging (development only)
console.log("Debug info:", data);
console.warn("Warning:", message);
console.error("Error:", error);

// Production: Would integrate with error tracking service (Sentry, etc.)
```

### Monitoring/Observability Setup

#### Current State
- **Backend**: Console logging with logrus (no persistent logs)
- **Frontend**: Browser console (no error tracking)
- **Database**: PostgreSQL logs (Docker Compose logs)
- **No monitoring tools** (Prometheus, Grafana, etc.)

#### Future Monitoring (Planned)
- **Application Logs**: Ship to centralized logging (ELK Stack, Loki)
- **Metrics**: Prometheus + Grafana dashboards
- **Tracing**: OpenTelemetry (already imported in Go dependencies)
- **Error Tracking**: Sentry for frontend and backend
- **Uptime Monitoring**: Healthcheck endpoints + external monitoring

#### Healthcheck Endpoints
```go
// Backend (api/database/api.go)
GET /api/database/health
Response: {"status": "healthy", "database": "connected"}

// Frontend (Next.js built-in)
GET /api/health (if implemented)
```

**Docker Healthchecks**:
```yaml
# PostgreSQL
healthcheck:
  test: ["CMD-SHELL", "pg_isready -U postgres"]
  interval: 1s
  timeout: 1s
  retries: 2

# Frontend
healthcheck:
  test: ["CMD", "wget", "-qO-", "http://localhost:3000"]
  interval: 30s
  timeout: 10s
  retries: 3
```

---

## 9. Project-Specific Quirks

### Non-Obvious Architectural Decisions

#### 1. Multi-Schema PostgreSQL Database
**Decision**: Use 11 separate schemas instead of prefixing table names
**Rationale**:
- Better organization and namespace isolation
- Clearer domain boundaries
- Easier to grant schema-level permissions
- Prevents table name conflicts across domains

**Impact**:
- **CRITICAL**: All queries must use schema-qualified table names (`education.groups`, not `groups`)
- **CRITICAL**: BUN ORM requires quotes around table aliases: `ModelTableExpr('education.groups AS "group"')`
- Migrations must create schemas first (migration 0.0.0)

#### 2. Factory Pattern for Services and Repositories
**Decision**: Use factory pattern instead of dependency injection framework
**Rationale**:
- Go doesn't have built-in DI (no Spring, no NestJS)
- Factories make dependencies explicit and testable
- Allows swapping implementations easily (e.g., mock repositories in tests)

**Impact**:
- Every service/repository has a factory
- API layer gets services from service factory
- Service layer gets repositories from repository factory

#### 3. Frontend Proxies All API Calls Through Next.js
**Decision**: Don't call backend directly from browser; proxy through Next.js API routes
**Rationale**:
- Security: JWT tokens never exposed to client-side JavaScript
- Flexibility: Can add caching, rate limiting, transformation
- CORS: No CORS issues between frontend and backend

**Impact**:
- Every backend endpoint has a corresponding Next.js route handler
- Adds latency (extra hop through Next.js)
- More code to maintain (route wrappers)

#### 4. Token Cleanup on Login
**Decision**: Delete old tokens for user on login (not just expired ones)
**Rationale**:
- Prevents token table bloating with active sessions
- Enforces single-session-per-user by default
- Simplifies token management

**Impact**:
- Users logged out from other devices on login
- Must be documented for users expecting multi-device sessions
- Could be made configurable in future

#### 5. Student Location Tracking - Two Systems
**Decision**: Maintain both deprecated boolean flags AND real tracking tables
**Rationale**:
- **Transition period**: Moving from manual flags to automated tracking
- Backward compatibility during migration
- Frontend still uses deprecated flags (needs refactoring)

**CRITICAL**:
- **Real system**: `active.visits` + `active.attendance` tables (functional, accurate)
- **Deprecated system**: `users.students` flags (`in_house`, `wc`, `school_yard`) (broken, being phased out)
- **Action needed**: Frontend must switch to `active.visits` for location data

#### 6. No Backdating for Substitutions
**Decision**: Substitutions must start today or in the future (no past substitutions)
**Rationale**:
- Prevents data manipulation and retroactive changes
- Maintains audit trail integrity
- Simplifies logic (no overlapping past substitutions)

**Impact**:
- Validation error if `start_date < today`
- Users must create substitutions in advance
- Cannot fix historical data through substitutions

### Known Issues or Workarounds Currently in Place

#### 1. Docker Backend Rebuild Required After Go Code Changes
**Issue**: Backend code changes don't reflect without rebuilding container
**Root Cause**: Go compilation happens in Docker build, not at runtime
**Workaround**:
```bash
docker compose build server  # MUST rebuild after Go changes
docker compose up -d server
```
**Future Fix**: Use volume mount + hot reload tool (air, reflex)

#### 2. Frontend API Route Params Type Complexity
**Issue**: Next.js 15 requires `params: Promise<Record<string, string | string[] | undefined>>`
**Root Cause**: Next.js 15 made params async
**Workaround**: Route wrapper extracts and awaits params automatically
```typescript
const contextParams = await context.params;
```
**Impact**: All route handlers must use `createGetHandler`, `createPostHandler`, etc.

#### 3. BUN ORM Column Mapping Errors
**Issue**: "Column not found" errors with schema-qualified tables
**Root Cause**: BUN requires exact column mapping for nested relationships
**Workaround**: Explicit `ColumnExpr` for each joined table
```go
ColumnExpr(`"teacher".id AS "teacher__id"`)
ColumnExpr(`"staff".* AS "staff__*"`)
```
**Prevention**: Always test queries with nested relations

#### 4. Bruno Tests Require Fresh Tokens
**Issue**: Cached tokens expire during test execution
**Root Cause**: Tokens expire every 15 minutes
**Workaround**: `dev-test.sh` gets fresh token for each test run
```bash
TOKEN=$(curl -s ... | jq -r '.access_token')
bru run dev/groups.bru --env-var accessToken="$TOKEN"
```
**Future Fix**: Implement token refresh in Bruno pre-request scripts

#### 5. SSL Certificate Expiration
**Issue**: Self-signed certificates expire after 1 year
**Root Cause**: OpenSSL default validity period
**Workaround**: Script to check expiration
```bash
./config/ssl/postgres/check-cert-expiration.sh
```
**Reminder**: Regenerate certificates annually with `create-certs.sh`

### Performance Considerations

#### 1. Database Query Optimization
- **Pagination**: All list endpoints default to 50 items per page
  ```go
  options := base.NewQueryOptions()
  options.WithPagination(1, 50)  // page, per_page
  ```
- **Eager Loading**: Use `Relation()` to avoid N+1 queries
  ```go
  query.Relation("Room").Relation("Representative")
  ```
- **Indexes**: Critical columns indexed (IDs, foreign keys, frequently queried fields)

#### 2. Frontend Optimizations
- **React 19 Optimizations**: Automatic batching, transitions
- **Next.js Optimizations**: Image optimization (sharp), code splitting, static generation
- **Turbo Mode**: Faster dev server (~3x faster than Webpack)

#### 3. JWT Token Size
- **Issue**: Large JWTs increase request overhead
- **Mitigation**: Minimal claims (user ID, role, permissions not embedded)
- **Future**: Consider opaque tokens with session lookup

#### 4. Docker Build Caching
- **Multi-stage builds**: Separate dependency and build layers
- **Cache mounts**: Speed up `go mod download` and `npm ci`
  ```dockerfile
  RUN --mount=type=cache,target=/go/pkg/mod go mod download
  RUN --mount=type=cache,target=/root/.npm npm ci
  ```

#### 5. API Response Size
- **Backend**: Paginated responses reduce payload size
- **Frontend**: Only request necessary fields (no "select all" queries)
- **Gzip**: Not currently enabled (future: add gzip middleware)

### Security Requirements

#### 1. GDPR Compliance
- **SSL/TLS**: Database connections encrypted (minimum TLS 1.2)
- **Data Retention**: Automated cleanup based on privacy consent (1-31 days)
- **Audit Logging**: All deletions logged in `audit.data_deletions`
- **Access Control**: Teachers only see students in their assigned groups
- **Right to Erasure**: Hard delete functionality for student data

#### 2. Authentication Security
- **Password Hashing**: Argon2id (memory=64MB, iterations=3, parallelism=2)
- **Token Expiry**: Access tokens 15min, refresh tokens 1hr
- **Token Storage**: Database-backed (can revoke tokens)
- **PIN Security**: 4-digit PINs hashed with Argon2id, attempt limiting

#### 3. HTTP Security Headers (middleware/security_headers.go)
```go
w.Header().Set("X-Frame-Options", "DENY")
w.Header().Set("X-Content-Type-Options", "nosniff")
w.Header().Set("X-XSS-Protection", "1; mode=block")
w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
w.Header().Set("Content-Security-Policy", "default-src 'self'")
```

#### 4. Rate Limiting (Optional)
```bash
RATE_LIMIT_ENABLED=false                       # Enable in production
RATE_LIMIT_REQUESTS_PER_MINUTE=60             # General API
RATE_LIMIT_AUTH_REQUESTS_PER_MINUTE=5         # Auth endpoints
```

#### 5. Input Validation
- **Backend**: ozzo-validation on all models
- **Frontend**: Zod validation on forms (future)
- **SQL Injection**: Prevented by BUN ORM parameterized queries

### Areas That Need Special Attention When Modifying

#### 1. Database Migrations
**⚠️ CRITICAL AREA**
- **Why**: Irreversible in production, can corrupt data
- **Process**:
  1. Always add migration, never edit existing ones
  2. Test rollback function
  3. Check dependencies (DependsOn array)
  4. Run `migrate status` and `migrate validate` before deploy
- **Example Issue**: Missing CASCADE on DROP TABLE breaks rollback

#### 2. Permission System (auth/authorize/)
**⚠️ SECURITY-CRITICAL**
- **Why**: Mistakes can leak private data or lock out users
- **Process**:
  1. Define new permissions in `auth/authorize/permissions/`
  2. Update policies in `auth/authorize/policies/`
  3. Test with different user roles
  4. Verify GDPR compliance (student data access)
- **Example Issue**: Granting "GroupsRead" instead of "StudentVisitsRead" exposes all students

#### 3. BUN ORM Queries with Schema-Qualified Tables
**⚠️ HIGH ERROR RATE**
- **Why**: Easy to forget quotes around aliases, causes runtime errors
- **Process**:
  1. Always use `ModelTableExpr` with quoted alias
  2. Test queries with nested relations
  3. Check for "column not found" errors
- **Checklist**:
  ```go
  ✓ ModelTableExpr(`education.groups AS "group"`)
  ✗ ModelTableExpr(`education.groups AS group`)
  ✓ ColumnExpr(`"group".id AS "group__id"`)
  ```

#### 4. NextAuth Token Handling (frontend/src/server/auth.ts)
**⚠️ AUTHENTICATION-CRITICAL**
- **Why**: Token refresh, session management are complex
- **Process**:
  1. Test token expiry scenarios
  2. Verify refresh token flow
  3. Check logout clears session
- **Example Issue**: Not clearing session on logout leaves stale tokens

#### 5. Student Location Tracking Migration
**⚠️ DATA INTEGRITY RISK**
- **Why**: Two systems coexist, easy to use wrong one
- **Process**:
  1. Always use `active.visits` for current location
  2. Never update deprecated flags (`in_house`, `wc`, `school_yard`)
  3. Frontend refactoring needed to switch to real system
- **Checklist**:
  ```
  ✓ Use active.visits + active.attendance
  ✗ Use users.students boolean flags
  ✓ Query for latest visit with end_time IS NULL
  ```

#### 6. CORS Configuration (api/base.go)
**⚠️ SECURITY-CRITICAL**
- **Why**: Misconfiguration allows unauthorized origins
- **Process**:
  1. Development: `CORS_ALLOWED_ORIGINS=*` (OK)
  2. Production: Explicit origins only (`http://yourdomain.com`)
  3. Never use `*` in production
- **Example Issue**: Leaving `*` in production allows phishing sites

#### 7. Environment Variable Validation (frontend/src/env.js)
**⚠️ DEPLOYMENT-CRITICAL**
- **Why**: Missing env vars cause runtime errors
- **Process**:
  1. Add new vars to Zod schema (server or client)
  2. Update `.env.example`
  3. Update Docker Compose env mapping
- **Example Issue**: Forgetting to add `NEXT_PUBLIC_` prefix makes var unavailable in browser

---

## 10. File Inventory

### Configuration Files and Their Purpose

#### Root Configuration
- **`.env`** (git-ignored): Docker Compose environment variables
  - Purpose: Central config for all services (DB password, JWT secrets, ports)
  - Used by: docker-compose.yml
- **`.env.example`**: Template for .env file
  - Purpose: Onboarding reference, shows all required variables
- **`.gitignore`**: Git ignore patterns
  - Purpose: Excludes build artifacts, secrets, IDE files, node_modules, etc.
- **`docker-compose.yml`**: Main development stack
  - Services: server (backend), frontend, postgres
  - Purpose: Local development environment
- **`docker-compose.example.yml`**: Example Docker Compose configuration
- **`docker-compose.cleanup.yml`**: GDPR data cleanup scheduler
  - Service: cleanup (runs daily at 2:00 AM)
- **`docker-compose.scheduler.yml`**: Automated task scheduler
  - Service: scheduler (runs session end at 18:00)
- **`docker-compose.sonar.yml`**: SonarQube code analysis stack
- **`sonar-project.properties`**: SonarQube project configuration

#### Backend Configuration
- **`backend/go.mod`**: Go module definition
  - Module: `github.com/moto-nrw/project-phoenix`
  - Go version: 1.23.0
- **`backend/go.sum`**: Go module checksums (auto-generated)
- **`backend/dev.env.example`**: Backend environment template
  - Purpose: Local dev config (DB connection, JWT secrets, admin account)
- **`backend/dev.env`** (git-ignored): Actual backend config
- **`backend/Dockerfile`**: Multi-stage Go build
  - Stage 1: builder (Go 1.23-alpine)
  - Stage 2: runtime (Alpine with CA certs)
- **`backend/main.go`**: Entry point (delegates to cmd/)

#### Frontend Configuration
- **`frontend/package.json`**: npm dependencies and scripts
  - Package manager: npm@11.3.0 (enforced)
  - Dependencies: Next.js, React, TypeScript, Tailwind CSS
- **`frontend/package-lock.json`**: npm dependency tree (auto-generated)
- **`frontend/tsconfig.json`**: TypeScript compiler configuration
  - Strict mode: enabled
  - Path aliases: `~/` and `@/` → `./src/`
- **`frontend/eslint.config.js`**: ESLint v9 flat config
  - Extends: next/core-web-vitals, typescript-eslint
  - Custom rules: type imports, unused vars
- **`frontend/prettier.config.js`**: Prettier configuration
  - Plugin: prettier-plugin-tailwindcss (auto-sorts classes)
- **`frontend/postcss.config.js`**: PostCSS configuration
  - Plugin: @tailwindcss/postcss
- **`frontend/tailwind.config.js`**: Tailwind CSS v4 configuration
  - Content: Scans src/ for class usage
- **`frontend/next.config.js`**: Next.js configuration
  - Imports: src/env.js (validates environment variables)
- **`frontend/.env.example`**: Frontend environment template
- **`frontend/.env.local`** (git-ignored): Actual frontend config
- **`frontend/Dockerfile`**: Multi-stage Node build
  - Stage 1: deps (production dependencies)
  - Stage 2: dev-deps (all dependencies)
  - Stage 3: builder (builds app)
  - Stage 4: runner (production server)

#### SSL Configuration
- **`config/ssl/postgres/create-certs.sh`**: Certificate generation script
  - Generates: CA cert + server cert/key for PostgreSQL
- **`config/ssl/postgres/check-cert-expiration.sh`**: Certificate expiry checker
- **`config/ssl/postgres/postgresql.conf`**: PostgreSQL SSL configuration
  - Enforces TLS 1.2+ with strong ciphers
- **`config/ssl/postgres/pg_hba.conf`**: PostgreSQL host-based authentication
  - Requires SSL for all connections
- **`config/ssl/postgres/certs/`** (git-ignored): Generated certificates
  - `ca.crt`, `ca.key` - Certificate Authority
  - `server.crt`, `server.key` - PostgreSQL server certificate

#### Bruno API Testing
- **`bruno/bruno.json`**: Bruno CLI configuration
- **`bruno/environments/`**: Test environment configs (Local, Staging, etc.)
- **`bruno/dev-test.sh`**: Test runner wrapper
  - Gets fresh auth tokens automatically
  - Provides shortcuts: `./dev-test.sh groups`, `./dev-test.sh all`

#### Documentation Files
- **`README.md`**: Project README (overview, quick start)
- **`CLAUDE.md`**: Project-wide Claude Code instructions (34KB)
- **`CLAUDE.local.md`**: User-specific local Claude instructions
- **`backend/CLAUDE.md`**: Backend-specific Claude instructions
- **`frontend/CLAUDE.md`**: Frontend-specific Claude instructions
- **`RFID_IMPLEMENTATION_GUIDE.md`**: Comprehensive RFID integration guide
- **`FRONTEND_BACKEND_IMPLEMENTATION_STATUS.md`**: Feature status tracking
- **`WEBSITE_DESIGN_SYNTHESIS.md`**: Design system documentation
- **`LICENSE`**: MIT License

### Key Source Directories and Contents

#### Backend Source Directories
- **`backend/api/`**: HTTP handlers (thin layer, route definitions)
  - 15 domain subdirectories (active, activities, auth, config, database, feedback, groups, iot, rooms, schedules, staff, students, substitutions, usercontext, users)
  - Each domain: `api.go` (router), `handlers.go` (request handlers), optional `types.go`
- **`backend/auth/`**: Authentication and authorization
  - `authorize/` - Permission middleware, policies
  - `device/` - RFID device authentication
  - `jwt/` - JWT middleware
  - `userpass/` - Username/password authenticator
- **`backend/cmd/`**: CLI commands
  - `root.go` - Command tree root
  - `serve.go` - HTTP server
  - `migrate.go` - Database migrations
  - `seed.go` - Test data generation
  - `cleanup.go` - GDPR data cleanup
  - `gendoc.go` - API documentation generation
- **`backend/database/`**: Data access layer
  - `migrations/` - 100+ migration files
  - `repositories/` - Repository implementations (12 domains)
  - `db.go` - Database connection setup
- **`backend/models/`**: Domain models (12 domains)
  - Each domain: Model structs, validation, repository interfaces
- **`backend/services/`**: Business logic layer (12 domains)
  - Each domain: Service interface + implementation
  - `factory.go` - Service factory
- **`backend/middleware/`**: HTTP middleware
  - `security_headers.go` - CSP, HSTS, X-Frame-Options
- **`backend/logging/`**: Logging utilities
- **`backend/email/`**: Email services
  - `templates/email/` - HTML email templates
- **`backend/seed/`**: Test data generation
  - `fixed/` - Fixed seed data (rooms, buildings)
  - `runtime/` - Runtime-generated data (students, groups)
- **`backend/test/`**: Test utilities and integration tests
- **`backend/public/uploads/`**: User-uploaded files

#### Frontend Source Directories
- **`frontend/src/app/`**: Next.js App Router (file-system routing)
  - `(auth)/` - Auth-protected routes (dashboard, groups, students, rooms, etc.)
  - `api/` - Next.js API route handlers (15 domains)
  - `login/` - Login page
  - `layout.tsx` - Root layout
  - `page.tsx` - Home page
- **`frontend/src/components/`**: React components (domain-organized)
  - 10 domain subdirectories (activities, auth, dashboard, facilities, groups, rooms, etc.)
  - Shared components: `session-provider.tsx`, `animated-background.tsx`
- **`frontend/src/lib/`**: Domain services and utilities
  - 15+ domain files (`{domain}-api.ts`, `{domain}-helpers.ts`, `{domain}-service.ts`)
  - `api-client.ts` - Base axios instance
  - `api-helpers.ts` - Error handling
  - `route-wrapper.ts` - API route wrappers
- **`frontend/src/server/`**: Server-only code
  - `auth.ts` - NextAuth configuration
- **`frontend/src/styles/`**: Global styles
  - `globals.css` - Tailwind directives
- **`frontend/public/`**: Static assets
  - `images/` - Image files

### Generated/Build Directories to Ignore

#### Backend
- **`backend/bin/`**: Compiled binaries (git-ignored)
- **`backend/*.log`**: Log files (git-ignored)
- **`backend/tmp/`**: Temporary files (git-ignored)

#### Frontend
- **`frontend/.next/`**: Next.js build output (git-ignored)
  - Contains optimized pages, static assets, server functions
- **`frontend/node_modules/`**: npm dependencies (git-ignored)
- **`frontend/tmp/`**: Temporary files (git-ignored)
- **`frontend/*.tsbuildinfo`**: TypeScript incremental build cache (git-ignored)

#### Docker
- **Volumes**: `postgres` (database data), `frontend_node_modules` (named volume)
- **Images**: Built images cached locally

### Files Containing or Potentially Containing Sensitive Data

#### ⚠️ NEVER COMMIT THESE FILES:
- **`.env`**: Contains database passwords, JWT secrets, API keys
- **`backend/dev.env`**: Backend config with DB connection string, admin password
- **`frontend/.env.local`**: Frontend config with NextAuth secret
- **`config/ssl/postgres/certs/*`**: SSL certificates and private keys
- **`backend/public/uploads/*`**: User-uploaded files (may contain PII)
- **`*.log`**: Log files may contain sensitive data (user IDs, requests)

#### Safe to Commit (Templates):
- **`.env.example`**: Template with placeholder values
- **`backend/dev.env.example`**: Template with instructions
- **`frontend/.env.example`**: Template with defaults

#### Database Sensitive Data:
- **`auth.accounts`**: Email addresses, password hashes
- **`auth.tokens`**: JWT tokens (active sessions)
- **`users.persons`**: Names, birthdays (PII)
- **`users.students`**: Student data (GDPR-protected)
- **`iot.devices`**: API keys for RFID readers

#### Security Best Practices:
1. Never commit `.env*` files (except `*.example`)
2. Regenerate secrets for production (never use example values)
3. Use `git add <specific-file>` (never `git add .`)
4. Check `.gitignore` before committing
5. Use `git diff --cached` to review staged changes
6. Rotate secrets if accidentally committed (change immediately)

---

## Summary

This comprehensive documentation covers every aspect of Project Phoenix:

- **Business Context**: GDPR-compliant RFID attendance system for educational institutions
- **Technical Stack**: Go 1.23 + Chi + Bun ORM backend, Next.js 15 + React 19 + TypeScript frontend, PostgreSQL 17 with SSL
- **Architecture**: Domain-Driven Design with factory pattern, multi-schema database, JWT authentication
- **Code Patterns**: Strict TypeScript, BUN ORM schema-qualified queries, repository/service layers
- **Testing**: Bruno API tests (~252ms), Go test framework (limited coverage)
- **Development**: Docker Compose with hot reload, automated setup script
- **Security**: Argon2id passwords, SSL database, GDPR compliance, audit logging
- **Special Considerations**: Multi-schema queries require quoted aliases, student location tracking in transition, backend rebuild required after Go changes

**Total Codebase Statistics**:
- **Backend**: 326 Go files across 12 domains
- **Frontend**: 332 TypeScript/TSX files
- **Migrations**: 100+ database migrations
- **API Tests**: 52 Bruno test files
- **Lines of Documentation**: 34KB in CLAUDE.md alone

This documentation should provide complete context for Claude Code optimization and comprehensive developer onboarding.

---

**Document Version**: 1.0
**Last Updated**: 2025-10-02
**Maintained By**: Project Phoenix Team
