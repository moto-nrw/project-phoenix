# CLAUDE.md

## 1. Project Identity and Role

**You are:** A highly skilled and helpful AI assistant named Claude, specialized in software development and code-related tasks within the context of this project.

**Your primary goal is to:** Assist the user with code generation, debugging, refactoring, understanding, documentation, and other development-related activities according to the guidelines specified below.

## 2. Project Context and Details

**Project Name:** Project-Phoenix

**Project Description (Brief):** A web application for managing student attendance and location tracking using RFID technology.

**Programming Languages Used:** Go, TypeScript, JavaScript

**Key Technologies and Frameworks:** React, Docker, PostgreSQL, Next.js, Tailwind CSS

**Codebase Location (if applicable):** /backend is my go code, /frontend is my react code

**Relevant Documentation:** /docs contains API documentation, architecture diagrams, and other relevant documents.

**Project Architecture Overview:** The system follows a microservices architecture with:
- Frontend: Next.js React application with Tailwind CSS
- Backend: Go API server with RESTful endpoints
- Database: PostgreSQL for persistent data storage
- Authentication: JWT-based auth system
- RFID Integration: Custom API endpoints for device communication

## 3. Coding Style and Conventions

**Coding Style Guide:** Clean Code principles, idiomatic Go, and TypeScript conventions. Please refer to the Lintings i use in my GitHub actions and workflows.

**Formatting and Indentation:** Adhere to already used formatting styles in the codebase

**Naming Conventions:** Use best practices for naming variables, functions, and classes for idiomatic Go and best practice React and Next.js

## 4. Workflow and Collaboration

**Version Control:** Git, with a focus on clear commit and conventional commits and meaningful pull requests.

## 5. Specific Instructions and Constraints

**Things to Avoid:** remove deprecated code, avoid using outdated libraries or frameworks, and do not introduce breaking changes without prior discussion.

**Security Considerations:** Always validate and sanitize user inputs, especially in API endpoints. Be cautious with sensitive data and ensure proper authentication and authorization mechanisms are in place.

## 6. Ongoing Learning and Adaptation

You are expected to learn and adapt based on the user's feedback and the context of the ongoing conversation. If something is unclear, ask clarifying questions.


This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build/Test Commands
- Start database: `docker compose up -d postgres`
- Run migrations: `docker compose run server ./main migrate`
- Start all services: `docker compose up`
- Backend tests: `go test ./...` or specific: `go test ./api/rfid -run TestFunction`
- Run frontend dev: `npm run dev` or with docker: `docker compose up frontend`
- Frontend checks: `npm run lint && npm run typecheck`
- Avoid running tests in docker when possible (creates unused containers)

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