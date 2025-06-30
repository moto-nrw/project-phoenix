# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Context

**Project Name:** Project-Phoenix Frontend

**Description:** Next.js frontend application for a student attendance and room management system. Provides a modern web interface for tracking student presence via RFID and managing educational facilities.

**Key Technologies:**
- Next.js v15+ with App Router
- React v19+ 
- TypeScript (strict mode)
- Tailwind CSS v4+
- NextAuth for JWT authentication
- Zod for environment validation
- Axios for API calls
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

```bash
# NextAuth
NEXTAUTH_URL=http://localhost:3000          # Frontend URL for auth
NEXTAUTH_SECRET=your_secret_here            # Generate with: openssl rand -base64 32
AUTH_SECRET=your_auth_secret_key            # Legacy - use NEXTAUTH_SECRET

# API Configuration  
NEXT_PUBLIC_API_URL=http://localhost:8080   # Backend API URL

# Docker
SKIP_ENV_VALIDATION=true                    # Set for Docker builds
```

## Code Architecture

### High-Level Architecture

The frontend follows a domain-driven structure with clear separation of concerns:

1. **Route Handlers** (`/src/app/api/`): Next.js API routes that proxy requests to the backend
   - All handlers use `route-wrapper.ts` for consistent auth and error handling
   - Context parameter must include `params: Promise<Record<string, string | string[] | undefined>>` for Next.js 15+
   - Returns `ApiResponse<T>` or `ApiErrorResponse`

2. **Domain Services** (`/src/lib/`): Business logic and API integration
   - API clients: `{domain}-api.ts` - Backend API calls
   - Helpers: `{domain}-helpers.ts` - Data transformation between frontend/backend
   - Services: `{domain}-service.ts` - Complex business logic orchestration

3. **Component Structure** (`/src/components/`):
   - Domain folders contain related components
   - Naming: `{domain}-form.tsx`, `{domain}-list.tsx`
   - Shared UI components in `/ui/`

### Key Architectural Patterns

**Route Handler Pattern** (Next.js 15+):
```typescript
// In app/api/{resource}/route.ts
export const GET = createGetHandler(async (request, token, params) => {
  const response = await apiGet(`/api/resources`, token);
  return response.data; // Extract data from paginated response
});

export const POST = createPostHandler(async (request, token, params) => {
  const body = await request.json();
  return await apiPost('/api/resources', body, token);
});
```

**API Client Pattern**:
```typescript
// In lib/{domain}-api.ts
export async function fetchResources(filters?: ResourceFilters): Promise<Resource[]> {
  const session = await getSession();
  const token = session?.user?.token;
  
  const response = await api.get('/resources', {
    headers: { Authorization: `Bearer ${token}` },
    params: filters
  });
  
  return response.data.data.map(mapResourceResponse);
}
```

**Data Mapping Pattern**:
```typescript
// In lib/{domain}-helpers.ts
export function mapResourceResponse(data: BackendResource): Resource {
  return {
    id: data.id.toString(),              // Backend uses int64, frontend uses string
    name: data.name,
    createdAt: new Date(data.created_at), // Snake case to camel case
    // Handle nested objects
    teacher: data.teacher ? mapTeacherResponse(data.teacher) : undefined
  };
}
```

**Environment Validation** (using Zod):
```typescript
// src/env.js
export const env = createEnv({
  server: {
    NEXTAUTH_SECRET: z.string().optional(),
    NODE_ENV: z.enum(["development", "test", "production"]).default("development"),
  },
  client: {
    NEXT_PUBLIC_API_URL: z.string().url().optional().default("http://localhost:8080"),
  },
  runtimeEnv: {
    // Map actual env vars
  },
  skipValidation: !!process.env.SKIP_ENV_VALIDATION,
});
```

### Authentication Flow

1. User logs in via `/app/api/auth/login` route
2. Backend returns JWT access token (15min) and refresh token (1hr)
3. NextAuth stores tokens in session
4. Route handlers extract token from session for API calls
5. API clients include token in Authorization header
6. Refresh token used automatically when access token expires

### Error Handling

```typescript
// Standardized error structure
type ApiErrorResponse = {
  error: string;
  status?: number;
  code?: string;
};

// Error handling in API routes
try {
  const response = await apiCall();
  return NextResponse.json(response);
} catch (error) {
  return handleApiError(error);
}
```

## TypeScript Configuration

**Key tsconfig.json settings:**
- `strict: true` - Full TypeScript strict mode
- `noUncheckedIndexedAccess: true` - Safer array/object access
- Path aliases: `~/*` and `@/*` map to `./src/*`
- Target: ES2022 with ESNext modules

## ESLint Configuration

**Important rules:**
- `max-warnings: 0` - Zero warnings allowed
- `@typescript-eslint/consistent-type-imports` - Use `import type` 
- `@typescript-eslint/prefer-nullish-coalescing` - Use `??` not `||` for nullish checks
- `@typescript-eslint/no-unused-vars` - Prefix unused vars with `_`

## Common Patterns

### Form Handling
```typescript
// Forms use controlled components
export function ResourceForm({ onSubmit }: Props) {
  const [name, setName] = useState("");
  
  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    await onSubmit({ name });
  };
  
  return (
    <form onSubmit={handleSubmit}>
      <Input value={name} onChange={(e) => setName(e.target.value)} />
    </form>
  );
}
```

### List Components with Loading States
```typescript
export function ResourceList() {
  const [resources, setResources] = useState<Resource[]>([]);
  const [loading, setLoading] = useState(true);
  
  useEffect(() => {
    fetchResources()
      .then(setResources)
      .finally(() => setLoading(false));
  }, []);
  
  if (loading) return <div>Loading...</div>;
  
  return (
    <ul>
      {resources.map(resource => (
        <li key={resource.id}>{resource.name}</li>
      ))}
    </ul>
  );
}
```

### Suspense Boundaries (Required for useSearchParams)
```typescript
// In page.tsx files
export default function Page() {
  return (
    <Suspense fallback={<div>Loading...</div>}>
      <PageContent />
    </Suspense>
  );
}

function PageContent() {
  const searchParams = useSearchParams(); // Now safe to use
  // ...
}
```

### API Response Types
```typescript
// Paginated response from backend
interface PaginatedResponse<T> {
  data: T[];
  total: number;
  page: number;
  per_page: number;
}

// Frontend wrapper
interface ApiResponse<T> {
  data: T;
  status: "success";
}
```

## Domain-Specific Patterns

### Active Sessions (Real-time tracking)
- Groups can have active sessions with multiple supervisors
- **Multiple supervisor management**: New `SupervisorMultiSelect` component for assigning multiple supervisors to groups
- Students check in/out of rooms via RFID
- Visit tracking includes start/end times
- Combined groups can contain multiple regular groups

### Activities Domain
- Activities have schedules with timeframes
- Students enrolled in activities
- Supervisors assigned to activities
- Categories for activity organization

### User Management
- Teachers linked to Staff → Person hierarchy
- Students have guardians and privacy consent
- RFID cards associated with persons
- Role-based permissions

## Common Issues and Solutions

### Linting Issues
- **Nullish coalescing**: Use `??` instead of `||` for default values
- **Type imports**: Always use `import type { X }` for types
- **Unused vars**: Prefix with underscore: `_unusedVar`

### Type Errors
- **API responses**: Ensure proper typing with generics
- **Route params**: Use proper Next.js 15+ context typing
- **Async components**: Only server components can be async

### Build Issues
- Run `npm run check` before committing
- Fix all ESLint errors (0 warnings policy)
- Ensure all TypeScript errors resolved

### Runtime Issues
- **useSearchParams**: Wrap in Suspense boundary
- **Hydration errors**: Check for client/server mismatches
- **Auth errors**: Verify session and token handling

## Docker Development

```bash
# Frontend runs on port 3000
# Backend API calls use internal Docker network
# Set SKIP_ENV_VALIDATION=true in Docker builds
docker compose up frontend
```

## Backend API Integration

The frontend proxies all API calls through Next.js route handlers to the Go backend:

**Key API patterns:**
- All endpoints prefixed with `/api/`
- JWT token in Authorization header
- Paginated responses for lists
- Snake_case from backend converted to camelCase
- Int64 IDs from backend stored as strings in frontend

**Major API domains:**
- `/api/auth/*` - Login, logout, refresh tokens
- `/api/students/*` - Student CRUD and enrollment
- `/api/rooms/*` - Room management and occupancy
- `/api/activities/*` - Activity scheduling and enrollment
- `/api/groups/*` - Group and combined group management
- `/api/active/*` - Real-time session tracking
- `/api/rfid-cards/*` - RFID card management

## Development Workflow

1. Check existing patterns in similar files
2. Create/update types in helpers file
3. Implement API client functions
4. Create/update route handlers
5. Build UI components
6. Always run `npm run check` before committing
7. Handle errors gracefully with user feedback

## Testing

Currently, the project does not have testing infrastructure set up. When adding tests:
- Consider React Testing Library for component tests
- Use MSW (Mock Service Worker) for API mocking
- Add test scripts to package.json
- Configure Jest or Vitest as test runner

## Performance Considerations

- Use React 19's built-in optimizations (automatic batching, transitions)
- Implement proper loading states with Suspense
- Lazy load heavy components with dynamic imports
- Use proper cache headers for API responses
- Implement pagination for large lists

## Security Best Practices

- Never expose JWT tokens in client-side code
- Use HTTP-only cookies for auth tokens when possible
- Validate all user inputs on both frontend and backend
- Sanitize data before rendering to prevent XSS
- Use environment variables for sensitive configuration
- Never commit `.env.local` file