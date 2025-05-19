# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Context

**Project Name:** Project-Phoenix Frontend

**Description:** Next.js frontend application for a student attendance and room management system. Provides a modern web interface for tracking student presence via RFID and managing educational facilities.

**Key Technologies:**
- Next.js v15+ with App Router
- React v19+ 
- TypeScript
- Tailwind CSS v4+
- NextAuth for authentication
- Zod for environment validation
- Turbo for dev server

## Common Development Commands

```bash
# Development
npm run dev                     # Start dev server with turbo (http://localhost:3000)
npm run build                   # Build for production
npm run start                   # Start production server
npm run preview                 # Build and preview production version

# Code Quality (Run these before committing!)
npm run lint                    # ESLint check with max-warnings=0
npm run lint:fix                # Auto-fix linting issues  
npm run typecheck               # TypeScript type checking
npm run check                   # Run both lint and typecheck

# Formatting
npm run format:check            # Check Prettier formatting
npm run format:write            # Fix formatting issues
```

## Environment Configuration

Copy `.env.example` to `.env.local` and configure:

- `NEXT_PUBLIC_API_URL`: Backend API URL (default: http://localhost:8080)
- `NEXTAUTH_URL`: Frontend URL for auth (default: http://localhost:3000)
- `NEXTAUTH_SECRET`: Generate with `openssl rand -base64 32`
- `SKIP_ENV_VALIDATION`: Set to true in Docker builds

## Code Architecture

### High-Level Architecture

The frontend follows a domain-driven structure with clear separation of concerns:

1. **Route Handlers** (`/src/app/api/`): Next.js API routes that proxy requests to the backend
   - All handlers use `route-wrapper.ts` for consistent auth and error handling
   - Context parameter must include `params: Promise<Record<string, string | string[] | undefined>>`

2. **Domain Services** (`/src/lib/`): Business logic and API integration
   - API clients use the pattern `{domain}-api.ts`
   - Helpers transform data between frontend/backend formats (`{domain}-helpers.ts`)
   - Services orchestrate complex operations (`{domain}-service.ts`)

3. **Component Structure** (`/src/components/`):
   - Domain folders contain related components
   - Form and list components follow naming conventions
   - Shared UI components in `/ui/`

### Key Architectural Patterns

**Route Handler Pattern** (Next.js 15+):
```typescript
export const GET = createGetHandler(async (request, token, params) => {
  // Handler implementation
});
```

**API Client Pattern**:
```typescript
// In lib/{domain}-api.ts
export async function fetchResources(): Promise<ApiResponse<Resource[]>> {
  const response = await apiGet('/resources', token);
  return mapResourcesResponse(response);
}
```

**Environment Validation** (using Zod):
```typescript
// src/env.js
export const env = createEnv({
  server: { /* server-side env vars */ },
  client: { /* client-side env vars */ },
  runtimeEnv: { /* actual env values */ }
});
```

### Authentication Flow

1. NextAuth handles JWT-based authentication
2. Session includes user token for backend API calls
3. Route handlers extract token from session
4. API clients include token in Authorization header

### Error Handling

- Route handlers return structured `ApiResponse<T>` or `ApiErrorResponse`
- `handleApiError` utility provides consistent error formatting
- Frontend components handle errors gracefully with user feedback

## Code Style Requirements

- Use TypeScript type imports: `import type { X }` with inline style
- Follow ESLint configuration (strict with 0 warnings allowed)
- Components use PascalCase, functions use camelCase
- Keep imports organized (React first, external deps, internal deps)
- All async operations should have proper error handling

## Common Patterns

### Form Handling
Forms use controlled components with validation. See existing form components for patterns.

### List Components  
Lists follow iterator pattern with loading states. Check `student-list.tsx` for example.

### Authentication
NextAuth handles auth flow. Use `useSession` hook for client-side auth state.

### Suspense Boundaries
Pages using `useSearchParams()` must be wrapped in Suspense boundaries:
```typescript
export default function Page() {
  return (
    <Suspense fallback={<Loading />}>
      <PageContent />
    </Suspense>
  );
}
```

## Running in Docker

The frontend runs in Docker with these considerations:
- Set `SKIP_ENV_VALIDATION=true` to bypass env checks
- Use `http://server:8080` for internal API calls to backend container
- Frontend available at http://localhost:3000

## Common Issues

- **Linting fails**: Run `npm run lint:fix` first, then manually fix remaining issues
- **Type errors**: Check imports and ensure proper typing of API responses
- **Build fails**: Usually due to linting or type errors - run `npm run check` locally first
- **Route handler errors**: Ensure context parameter is properly typed for Next.js 15+
- **useSearchParams errors**: Wrap components in Suspense boundaries

## Backend Integration

The frontend communicates with a Go backend API. Key endpoints:
- `/api/auth/*` - Authentication
- `/api/students` - Student management
- `/api/rooms` - Room tracking
- `/api/activities` - Activity management
- `/api/groups` - Group management

Check backend `routes.md` for full API documentation.