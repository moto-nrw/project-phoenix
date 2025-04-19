# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build Commands
- Run server: `go run main.go serve`
- Run tests: `go test ./...`
- Run specific test: `go test ./auth/pwdless -run TestAuthResource_login`
- Lint code: `go vet ./...`
- Generate API docs: `go run main.go gendoc`
- Run migrations: `go run main.go migrate`

## Code Style Guidelines
- File organization: Package-based structure with resource separation
- Imports: Standard library first, third-party second, project imports last
- Naming: CamelCase for exported items, package names match directories
- Error handling: Custom error types in separate error.go files per package
- Testing: Table-driven tests with descriptive names
- Interfaces: Define before implementation, use mocks for testing
- Documentation: Godoc style comments for exported functions and types
- Dependencies: Use Chi router, Cobra CLI, JWT auth, Bun ORM with PostgreSQL

## Development Roadmap
- Implement Room Management system
- Build User Management components
- Develop Time Management models
- Create Group Management functionality
- Implement Activity Group (AG) system
- Build Student Management features
- Develop Visit Tracking system
- Add Room Merging/Combined Groups functionality

## Project Documentation
- You can find my documentation of my project in /docs. Please make sure you are implementing as told in the documentation!

## Testing & Debugging Tips
### Authentication Flow Analysis
- Before testing, fully trace the auth flow (e.g., passwordless email -> token -> JWT exchange)
- Check token lifespans in config files - short-lived tokens (minutes) can cause test failures

### Server Process Management
- ALWAYS properly terminate previous server processes before starting new ones
- Use lsof -i:PORT to find processes using specific ports
- Use kill -9 PID to forcefully terminate processes that won't respond to SIGINT
- Consider running server with output capture: go run main.go serve > server_output.log 2>&1 &

### Token Extraction Techniques
- Use response parsing when chaining commands: ACCESS_TOKEN=$(curl ... | jq -r '.access_token')
- For debugging, always verify token content: echo "Using token: ${ACCESS_TOKEN:0:20}..."
- With passwordless auth, check server logs/output for email-delivered tokens

### Efficient Request Chaining
- Chain authentication and API requests in single commands with token extraction
- Add appropriate delays when tokens need time to become active
- Keep token requests and usage close together to avoid expiration

### Testing Symptoms Checklist
- 401 Unauthorized → Check token validity/expiration
- Address already in use → Find and kill previous server processes
- Empty response → Check if server output contains error messages

### Key Technical Concepts:
- Go backend development with REST API endpoints
- Chi router for API routing
- Bun ORM for PostgreSQL database operations
- JWT authentication for securing API endpoints
- Passwordless email authentication flow
- Database transactions for ensuring data consistency
- Database migrations for schema evolution
- Entity relationships (Room, RoomOccupancy, Timespan)
- Unit testing for model validation