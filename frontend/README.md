# Project Phoenix -- Frontend

Next.js web application for GDPR-compliant RFID student attendance and room management.

## Tech Stack

- **Next.js 16** (App Router) with **React 19** and **TypeScript 5**
- **Tailwind CSS 4** for styling
- **NextAuth v5** (beta) for JWT authentication
- **Axios** + **SWR** for API communication and caching
- **Zod** for runtime validation (env vars via `@t3-oss/env-nextjs`)
- **Recharts** for data visualization
- **Framer Motion** for animations
- **Vitest** + **Testing Library** for unit tests, **Playwright** for E2E
- **pnpm 10** as package manager

## Getting Started

```bash
cp .env.example .env.local   # Configure environment variables
pnpm install                  # Install dependencies
pnpm run dev                  # Start dev server at http://localhost:3000
```

## Environment Variables

See `.env.example` for all required variables. Key ones:

| Variable | Purpose |
|----------|---------|
| `NEXT_PUBLIC_API_URL` | Backend API URL (default `http://localhost:8080`) |
| `NEXTAUTH_URL` | Frontend URL for auth callbacks |
| `NEXTAUTH_SECRET` | JWT signing secret (`openssl rand -base64 32`) |
| `SKIP_ENV_VALIDATION` | Set `true` for Docker builds |

## Scripts

| Command | Description |
|---------|-------------|
| `pnpm run dev` | Start development server |
| `pnpm run build` | Production build |
| `pnpm run start` | Start production server |
| `pnpm run check` | Lint + typecheck (run before committing) |
| `pnpm run lint` | Oxlint |
| `pnpm run typecheck` | TypeScript type checking |
| `pnpm run format:write` | Auto-format with Prettier |
| `pnpm run test` | Run unit tests (Vitest) |
| `pnpm run test:run` | Run tests once (CI mode) |
| `pnpm run knip` | Detect unused dependencies and exports |

## Project Structure

```
src/
  app/
    (operator)/     # Operator-facing pages (separate auth context)
    (protected)/    # Authenticated teacher/admin pages
    (public)/       # Public pages (invitations, login)
    api/            # Next.js route handlers (proxy to Go backend)
    reset-password/ # Password reset flow
  components/       # React components organized by domain
  contexts/         # React context providers
  hooks/            # Custom React hooks (SSE, SWR, etc.)
  lib/              # API clients, helpers, utilities
  server/           # Server-side auth configuration
  styles/           # Global CSS
  test/             # Test utilities, mocks, fixtures
```

## Docker

Development and production Dockerfiles are provided (`Dockerfile`, `Dockerfile.prod`). Both use Node 20 Alpine with pnpm.

```bash
docker compose up frontend    # Run via docker-compose from project root
```

## More Information

See the [root README](../README.md) for full project documentation, backend setup, and database configuration.
