# Project Phoenix -- Backend

GDPR-compliant RFID student attendance and room management system for educational institutions.

## Tech Stack

| Component | Version / Library |
|-----------|-------------------|
| Language | Go 1.27.0 |
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
scripts/backend-architecture.sh audit-issues --api-url https://api.github.com
scripts/backend-architecture.sh explain --scope production --source <package> --target <package>
scripts/backend-architecture.sh diagram
scripts/backend-architecture.sh dependencies --focus module:timetable-activities
scripts/backend-architecture.sh dependencies --focus package:services/schedule
```

`check` evaluates the strict target policy in `architecture/policy.json`,
including production, internal-test, and external-test import scopes plus
table ownership, tenant-safe projections, public contract purity, direct
database access, and legacy-composition references. The normal command requires
exact equality with `architecture/legacy.jsonl`; each exact tuple names its one
open migration issue. `audit-issues` checks those issue states separately from
the deterministic graph result. `explain` names the rule for one edge.
`diagram` writes a temporary bundle containing the strict `target.svg`, the
condensed current `migration.svg`, machine-readable `architecture.json`, and a
policy-derived `go-arch-lint.yml`. `dependencies` writes `dependencies.svg`,
`dependencies.json`, and the matching policy-build-context Goda query for one
exact module owner or package. Both projection commands load the committed
baseline, print their temporary output directory, and reject committed output
locations.

## CLI Commands

Run application commands through Docker Compose from the repository root:

```bash
docker compose up -d
docker compose run server go run . migrate
docker compose run server go run . migrate reset
docker compose run server go run . migrate status
docker compose run server go run . migrate validate
docker compose run server go run . seed --email <op-email> --password '<pw>' --pin 1234
docker compose run server go run . cleanup visits
docker compose run server go run . cleanup preview
docker compose run server go run . cleanup stats
docker compose run server go run . gendoc
docker compose run server go run . simulate live
```

## Setup

1. Copy and edit the environment file:
   ```bash
   cp dev.env.example dev.env
   ```
2. Start the stack from the repository root; migrations run automatically:
   ```bash
   docker compose up -d
   ```

See `dev.env.example` for all available environment variables (database, auth, SMTP, rate limiting, scheduled tasks).

## Testing

```bash
../scripts/run-go-toolchain.sh go test ./...          # Run all tests
../scripts/run-go-toolchain.sh go test -v ./api/auth  # Specific package, verbose
../scripts/run-go-toolchain.sh go test -race ./...    # With race detection
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
