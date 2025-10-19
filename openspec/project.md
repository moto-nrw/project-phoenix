# Project Context

## Purpose
Project Phoenix delivers a modern RFID-driven student attendance and room management platform for educational institutions. The system tracks where students are in real time, orchestrates room and activity assignments, and surfaces actionable analytics so administrators can keep campuses safe, compliant, and efficiently utilized.

## Tech Stack
- **Backend**: Go 1.21+, Chi router, Bun ORM, PostgreSQL 17+, JWT-based auth with refresh tokens, structured logging.
- **Frontend**: Next.js 15+ (App Router), React 19+, TypeScript 5+, Tailwind CSS 4+, NextAuth.js for session management.
- **Infrastructure & Tooling**: Docker & Docker Compose, Caddy for production serving, GitHub Actions CI, OpenAPI 3 documentation, OpenSpec for change management.

## Project Conventions

### Code Style
- Go code must pass `gofmt`, `goimports`, and `golangci-lint run`; exported names use PascalCase, internals camelCase, constants SCREAMING_SNAKE_CASE, filenames snake_case.
- TypeScript/React follows ESLint 9 + Prettier (Tailwind plugin) with auto-format via `npm run format:write`; components/hooks in PascalCase, utilities in camelCase, feature folders kebab-case.
- Keep comments minimal and purposeful, prefer clear naming, and co-locate tests next to implementations.

### Architecture Patterns
- Backend is layered: HTTP handlers live in `backend/api`, auth flows in `backend/auth`, business logic/services in `backend/services`, and data models in `backend/models`; Bun ORM queries use quoted aliases (e.g., `ModelTableExpr("education.groups AS \"group\"")`).
- Database migrations and seeds reside in `backend/database` and `backend/seed`; integration harnesses live under `backend/test`.
- Frontend uses Next.js App Router with domain-focused folders (`frontend/src/app`, `components`, `lib`, `styles`) and shared UI primitives; state is managed with React Context and server actions where appropriate.
- Specs and proposals are tracked with OpenSpec (`openspec/specs`, `openspec/changes`) to keep requirements synchronized with implementation.

### Testing Strategy
- Run `cd backend && go test ./...` for unit coverage; add `-race` when touching goroutines or concurrent code paths.
- Execute `golangci-lint run` before pushing to catch style and static-analysis issues.
- Frontend validation uses `cd frontend && npm run check`; add `.test.tsx` co-located tests for critical UI/logic.
- Use Bruno collections via `cd bruno && ./dev-test.sh {suite}` to exercise API flows based on generated documentation; document any manual QA steps in PR notes.

### Git Workflow
- Follow Conventional Commit-style prefixes (e.g., `fix:`, `chore:`, `feat:`) with ≤72 character subjects and optional issue references.
- Branch per feature or fix; keep PRs scoped to a single capability or spec change.
- Confirm pre-push checks (`go test ./...`, `golangci-lint run`, `npm run check`) before opening a PR, and attach relevant screenshots or curl samples for UX/API changes.

## Domain Context
- Core problem space is RFID-based student attendance, room occupancy, and group supervision for schools and training centers.
- The platform supports schedules, multi-supervisor oversight, and analytics dashboards that highlight utilization, compliance, and engagement trends.
- Accuracy, real-time updates, and security (role-based access, encrypted transport) are critical to institutional trust.

## Important Constraints
- Do not commit real secrets; derive env files from `backend/dev.env.example` and `frontend/.env.local.example`.
- Local development must generate SSL certificates via `config/ssl/postgres/create-certs.sh` to satisfy data-protection policies (GDPR-ready posture).
- Prefer incremental, low-complexity changes (<100 lines) unless scale/performance needs are proven; document architectural shifts via OpenSpec proposals before coding.
- Maintain compatibility with Docker-based workflows and Postgres 17+; avoid introducing dependencies that break containerized environments.

## External Dependencies
- PostgreSQL 17+ database (can run via Docker Compose).
- NextAuth.js for authentication integrations (OAuth providers configured per environment).
- Bruno CLI for API regression testing, OpenAPI 3.0 spec for client generators, and GitHub Actions for CI.
- Caddy (production) and Docker/Docker Compose for deployment orchestration.
