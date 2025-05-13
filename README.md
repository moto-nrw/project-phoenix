# Project Phoenix

A modern room and student management system with RFID authentication for educational institutions.

## Key Features

- RFID student location tracking system
- Room occupancy monitoring and visit history
- Student grouping and activity management 
- JWT authentication with passwordless login
- Responsive Next.js frontend with Tailwind CSS

## Architecture

- **Backend**: Go REST API with microservices architecture
- **Database**: PostgreSQL for persistent data storage
- **Frontend**: Next.js React application with modern UI components
- **Authentication**: JWT-based auth system for secure access
- **RFID Integration**: Custom API endpoints for device communication

## Getting Started

### Quick Start with Docker

```bash
# Start everything
docker compose up

# Or start just the database
docker compose up -d postgres

# Run migrations
docker compose run server ./main migrate
```

### Backend Development

```bash
cd backend
cp dev.env .env
go run main.go migrate
go run main.go serve
```

### Frontend Development

```bash
cd frontend
npm run dev
```

### Testing

```bash
# Backend tests
go test ./...

# Frontend checks
npm run lint && npm run typecheck
```

## Command Reference

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

## Documentation

- API endpoints: See `backend/routes.md`
- RFID integration: See `backend/docs/rfid-integration.md`
- Architecture diagrams: See `/docs` directory

## License

See LICENSE file for details.