# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 1. Project Identity and Role

**You are:** A highly skilled and helpful AI assistant named Claude, specialized in software development and code-related tasks within the context of this project.

**Your primary goal is to:** Assist the user with code generation, debugging, refactoring, understanding, documentation, and other development-related activities according to the guidelines specified below.

## 2. Project Context and Details

**Project Name:** Project-Phoenix

**Project Description:** A web application for managing student attendance and location tracking using RFID technology in educational institutions. Tracks student presence, room occupancy, and provides comprehensive management for rooms, activities, and student groups.

**Programming Languages Used:** Go, TypeScript, JavaScript

**Key Technologies and Frameworks:**
- Backend: Go with Chi router, Bun ORM for PostgreSQL
- Frontend: Next.js (v15+) with React (v19+), Tailwind CSS (v4+)
- Database: PostgreSQL
- Authentication: JWT-based auth system
- RFID Integration: Custom API endpoints for device communication
- Deployment: Docker/Docker Compose

**Project Architecture Overview:**
- **Frontend:** Next.js App Router architecture with TypeScript
- **Backend:** Go API server with RESTful endpoints organized in domain-specific packages
- **Database:** PostgreSQL with multi-schema design (auth, users, education, facilities, activities, etc.)
- **Auth System:** JWT-based authentication with comprehensive role-based access control
- **API Structure:** Follows resource-oriented design with clear domain boundaries

## 3. Code Structure and Organization

### Backend (Go)
- **api/**: API handlers and route definitions organized by domain
- **auth/**: Authentication and authorization mechanisms
- **cmd/**: CLI commands for server, migrations, and documentation
- **database/**: Database connections and migrations
- **models/**: Data models with validation and business logic
- **services/**: Core business logic organized by domain
- **email/**: Email templating and delivery services
- **logging/**: Structured logging utilities

### Frontend (Next.js)
- **src/app/**: Next.js App Router pages and API routes
- **src/components/**: Reusable UI components organized by domain/function
- **src/lib/**: Utility functions, API clients, and helpers
- **src/styles/**: Global CSS and Tailwind configuration

## Build/Test Commands

### Backend (Go) Commands
```bash
# Server and database
go run main.go serve            # Start the backend server
go run main.go migrate          # Run database migrations

# Testing
go test ./...                   # Run all backend tests
go test ./api/users -run TestFunction  # Run specific test

# Documentation
go run main.go gendoc           # Generate API documentation (routes.md and OpenAPI)
go run main.go gendoc --routes  # Generate only routes documentation
go run main.go gendoc --openapi # Generate only OpenAPI specification

# Dependencies
go mod tidy                     # Clean up and organize Go dependencies
go get -u ./...                 # Update all dependencies
```

### Frontend (Next.js/npm) Commands
```bash
# Development
npm run dev                     # Start development server with turbo
npm run build                   # Build for production
npm run start                   # Start production server
npm run preview                 # Build and preview production version

# Linting and Type Checking
npm run lint                    # Run ESLint to check for code issues
npm run lint:fix                # Automatically fix linting issues
npm run typecheck               # Run TypeScript type checking
npm run check                   # Run both lint and type checking

# Formatting
npm run format:check            # Check code formatting with Prettier
npm run format:write            # Fix code formatting issues
```

### Docker Commands
```bash
docker compose up               # Start all services
docker compose up -d            # Start all services in detached mode
docker compose up -d postgres   # Start only the database
docker compose run server ./main migrate  # Run migrations in docker
docker compose run frontend npm run lint  # Run frontend lint checks in docker
docker compose logs postgres    # Check database logs
docker compose down             # Stop all services
```

## Environment Setup

### Quick Start
- Start database: `docker compose up -d postgres`
- Run migrations: `docker compose run server ./main migrate`
- Start all services: `docker compose up`
- Frontend checks: `npm run lint && npm run typecheck`
- Avoid running tests in docker when possible (creates unused containers)

### Backend Environment Variables (.env)
- `LOG_LEVEL`: Set to `debug` for development
- `DB_DSN`: Database connection string
- `DB_DEBUG`: Set to `true` to see SQL queries
- `AUTH_JWT_SECRET`: Secret key for JWT generation
- `ENABLE_CORS`: Set to `true` for local development

### Frontend Environment Variables
- `NEXT_PUBLIC_API_URL`: Backend API URL
- `NEXTAUTH_URL`: Frontend URL for authentication
- `NEXTAUTH_SECRET`: Secret for NextAuth

## Database Structure

The database uses multiple schemas to organize tables by domain:

- **auth**: Authentication-related tables (accounts, tokens, permissions)
- **users**: User profile and identity tables (persons, students, teachers)
- **education**: Group and class management
- **schedule**: Time and schedule management
- **activities**: Student activities and enrollments
- **facilities**: Rooms and physical locations
- **iot**: IoT device management
- **feedback**: User feedback
- **config**: System configuration

## Key API Endpoints

- Authentication: `/api/auth/token`, `/api/auth/login`
- Students: `/api/students`
- Rooms: `/api/rooms`
- Activities: `/api/activities`
- Groups: `/api/groups`
- Active visits: `/api/active/visits`

## Debugging Tips

- Backend logging: Check `logging/logger.go` for available log levels
- RFID device issues: See `/docs/rfid-integration-guide.md` for troubleshooting
- JWT authentication: Use debug endpoint `api/auth/debug-token/` to inspect tokens
- Database connection issues: Run `docker compose logs postgres` to check for errors
- Frontend errors: Check browser console and Next.js error overlay
- API response problems: Use the network tab to inspect request/response payloads

## Code Style Guidelines

- Go: Follow idiomatic patterns with consistent error handling
- TypeScript: Use type imports (`import type { X }`) with inline style
- Frontend components: PascalCase for components, camelCase for functions
- Group imports logically (standard lib, external, internal)
- Maintain consistent API response structures across endpoints
- Use package-specific error types with descriptive messages
- No backward compatibility requirements or API versioning
- All code should follow existing patterns in the codebase
- Always explain implementation plans before making changes