# Project Phoenix -- Backend

GDPR-compliant RFID student attendance and room management system for educational institutions.

## Tech Stack

| Component | Version / Library |
|-----------|-------------------|
| Language | Go 1.25+ |
| Router | chi/v5 |
| ORM | bun (pgdialect, pgdriver) |
| Database | PostgreSQL 17+ (multi-schema, SSL) |
| Auth | JWT via lestrrat-go/jwx/v3, chi/jwtauth |
| Config | Viper + Cobra CLI |
| Logging | log/slog (stdlib) |
| Email | go-mail |
| Real-time | Server-Sent Events (SSE) |
| Excel | excelize/v2 |
| Validation | ozzo-validation |

## Architecture

```
Handler -> Service -> Repository -> Database
```

| Layer | Path | Responsibility |
|-------|------|----------------|
| Handlers | `api/{domain}/` | HTTP adapters (thin, no business logic) |
| Services | `services/{domain}/` | Business logic, orchestration, transactions |
| Repositories | `database/repositories/{domain}/` | Data access (Bun ORM) |
| Models | `models/{domain}/` | Domain entities, shared across layers |

Factory pattern for dependency injection:
`repositories.NewFactory(db)` -> `services.NewFactory(repoFactory, db, logger)`

### Architecture checks

Run these commands from the repository root:

```bash
scripts/backend-architecture.sh check
scripts/backend-architecture.sh legacy-check
scripts/backend-architecture.sh diagram
scripts/backend-architecture.sh dependencies --focus module:timetable-activities
scripts/backend-architecture.sh dependencies --focus package:services/schedule
```

`check` evaluates the strict target policy in `architecture/policy.json`,
including production, internal-test, and external-test import scopes plus
table ownership, tenant-safe projections, public contract purity, direct
database access, and legacy-composition references. It currently reports the
existing target-policy violations with project-relative file, line, and
declaration evidence. The stable ratchet key does not include that evidence.
`legacy-check`
validates every production Go package against `.go-arch-lint.yml`; it excludes
`_test.go` files and fails when a package has no component. `diagram` writes a
temporary bundle containing the strict `target.svg`, the condensed current
`migration.svg`, machine-readable `architecture.json`, and a policy-derived
`go-arch-lint.yml`. `dependencies` writes `dependencies.svg`,
`dependencies.json`, and the matching policy-build-context Goda query for one
exact module owner or package. Both commands print their temporary output
directory; an explicit `--output` must also point inside the system temp tree.

## CLI Commands

```bash
go run main.go serve                # Start HTTP server
go run main.go migrate              # Run pending migrations
go run main.go migrate reset        # Drop and recreate all tables
go run main.go migrate status       # Show migration status
go run main.go migrate validate     # Validate migration dependencies
go run main.go seed --email <op-email> --password '<pw>' --pin 1234   # Seed test data via API (flags + running server required)
go run main.go cleanup visits       # Delete expired visit records (GDPR)
go run main.go cleanup preview      # Dry-run cleanup
go run main.go cleanup stats        # Data retention statistics
go run main.go gendoc               # Generate route docs and OpenAPI spec
go run main.go simulate live        # Continuous random-event simulation (see also full-day, status)
```

## Setup

1. Copy and edit the environment file:
   ```bash
   cp dev.env.example dev.env
   ```
2. Start PostgreSQL (or use Docker Compose from the project root).
3. Run migrations and start the server:
   ```bash
   go run main.go migrate
   go run main.go serve
   ```

See `dev.env.example` for all available environment variables (database, auth, SMTP, rate limiting, scheduled tasks).

## Testing

```bash
go test ./...                              # Run all tests
go test -v ./api/auth                      # Specific package, verbose
go test -race ./...                        # With race detection
APP_ENV=test go run main.go migrate reset  # Reset test DB (port 5433)
```

Tests use a real PostgreSQL test database (port 5433) with hermetic fixtures. See `test/helpers.go` and `test/fixtures.go`.

## Docker

```bash
# Production build (multi-stage, non-root)
docker build -f Dockerfile -t phoenix-backend .

# Development with hot reload (air)
docker build -f Dockerfile.dev -t phoenix-backend-dev .
```

See the [root README](../README.md) for full Docker Compose setup.
