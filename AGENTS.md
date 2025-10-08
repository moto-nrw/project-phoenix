# Repository Guidelines

## Project Structure & Module Organization
Project root splits into `backend` (Go service) and `frontend` (Next.js app). Router and HTTP handlers live in `backend/api`, auth flows in `backend/auth`, CLI utilities in `backend/cmd`, and business logic in `backend/services` alongside schema models in `backend/models`. Migrations and seeds are under `backend/database` and `backend/seed`; race harnesses and integration fixtures sit in `backend/test`. Frontend code is organized under `frontend/src` with `app/`, `components/`, `lib/`, and `styles/`, and static assets in `frontend/public`. Repository-level docs stay in `docs/`; env and SSL templates are in `config/`.

## Build, Test, and Development Commands
- `./scripts/setup-dev.sh` prepares env files and SSL certificates for new machines.
- `docker compose up -d postgres` starts the DB; `docker compose up` spins the full stack.
- `cd backend && go run main.go migrate && go run main.go serve` migrates and serves the API on :8080.
- `cd backend && go run main.go gendoc --routes` regenerates `backend/routes.md`; add `--openapi` to update `docs/openapi.yaml`.
- `cd frontend && npm install && npm run dev` runs the Next.js server on :3000.
- Pre-push checks: `cd backend && go test ./...` (add `-race` for concurrency changes) plus `golangci-lint run`; `cd frontend && npm run check`.

## Coding Style & Naming Conventions
Run `gofmt`, `goimports`, and `golangci-lint run` before committing; external names stay PascalCase, internals camelCase, constants SCREAMING_SNAKE_CASE, and filenames snake_case. Keep services and repositories domain-scoped and use quoted schema aliases in Bun queries (e.g., `ModelTableExpr(`education.groups AS "group"`)`). TypeScript follows ESLint 9 + Prettier (Tailwind plugin); components/hooks use PascalCase, utilities camelCase, and feature folders kebab-case. Use `npm run lint` or `npm run format:write` to enforce consistency.

## Testing & QA Guidelines
Co-locate Go tests with their packages and favor table-driven cases; run `go test -cover ./...` and the concurrency scripts in `backend/test` when touching goroutines. Use Bruno collections via `cd bruno && ./dev-test.sh {suite}` to exercise API flows built from gendoc output. Frontend relies on `npm run check`; add `.test.tsx` specs under `frontend/src/__tests__` and document manual QA in PR notes.

## Security, Commits & PRs
Do not commit real secrets; keep `.env` files derived from templates in `backend/dev.env.example` and `frontend/.env.local.example`, and regenerate SSL certs with `config/ssl/postgres/create-certs.sh` when needed. Commits follow the Conventional-style prefixes seen in history (`fix:`, `chore:`, `Feat/...`) with ≤72 char subjects and optional issue references. PRs should summarize scope, link tracking issues, attach screenshots or curl samples for UX/API work, note schema or config changes, and confirm `go test ./...`, `golangci-lint run`, and `npm run check` have passed.
