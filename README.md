# Project Phoenix

A modern room and student management system with RFID authentication for educational institutions. This application allows tracking student attendance and location using RFID technology while providing comprehensive management tools for rooms, activities, and groups.

## Key Features

- RFID student location tracking system
- Room occupancy monitoring and visit history
- Student grouping and activity management 
- JWT authentication with passwordless login options
- Responsive Next.js frontend with Tailwind CSS
- RESTful API with OpenAPI documentation
- Role-based access control
- Comprehensive reporting and analytics

## Architecture

- **Backend**: Go REST API with microservices architecture
- **Database**: PostgreSQL for persistent data storage
- **Frontend**: Next.js React application with modern UI components
- **Authentication**: JWT-based auth system for secure access
- **RFID Integration**: Custom API endpoints for device communication

## Prerequisites

- **Docker** and **Docker Compose** (recommended for easy setup)
- **Go** 1.21+ (for backend development)
- **Node.js** 20+ and **npm** 11+ (for frontend development)
- **PostgreSQL** 17+ (if running database locally)

## Getting Started

### Quick Start with Docker

The fastest way to get the entire system running:

```bash
# Start everything
docker compose up

# Or start just the database
docker compose up -d postgres

# Run migrations
docker compose run server ./main migrate
```

Once running:
- Backend API: http://localhost:8080
- Frontend: http://localhost:3000

### Backend Development Setup

```bash
cd backend
cp dev.env.example .env  # Create environment file from template

# Configure your .env file with appropriate values
# Start PostgreSQL (if using Docker)
docker compose up -d postgres

# Run migrations and start server
go run main.go migrate
go run main.go serve
```

### Frontend Development Setup

```bash
cd frontend
npm install              # Install dependencies
npm run dev             # Start development server
```

The frontend will be available at http://localhost:3000

### Environment Configuration

#### Backend (.env file)
- `LOG_LEVEL`: Set to `debug` for development
- `DB_DSN`: Database connection string
- `DB_DEBUG`: Set to `true` to see SQL queries
- `AUTH_JWT_SECRET`: Secret key for JWT generation (set a strong value in production)
- `ENABLE_CORS`: Set to `true` for local development

#### Frontend (.env file)
- `NEXT_PUBLIC_API_URL`: Backend API URL
- `NEXTAUTH_URL`: Frontend URL for authentication
- `NEXTAUTH_SECRET`: Secret for NextAuth (set a strong value in production)

### Testing

```bash
# Backend tests
go test ./...                           # Run all tests
go test ./api/users -run TestFunction   # Run specific test

# Frontend checks
npm run lint && npm run typecheck      # Syntax and type checking
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

## Project Structure

```
project-phoenix/
├── backend/                # Go backend API
│   ├── api/                # API handlers and route definitions
│   ├── auth/               # Authentication and authorization
│   ├── cmd/                # CLI commands
│   ├── database/           # Database migrations and definitions
│   ├── docs/               # API documentation
│   ├── models/             # Data models
│   ├── services/           # Business logic
│   └── main.go             # Application entry point
├── docs/                   # Project documentation
├── frontend/               # Next.js frontend
│   ├── public/             # Static assets
│   └── src/
│       ├── app/            # Next.js app router
│       ├── components/     # Reusable UI components
│       └── lib/            # Utility functions and API clients
└── docker-compose.yml      # Docker configuration
```

## API Documentation

- **API Routes**: Generated at `backend/routes.md` (run `go run main.go gendoc --routes`)
- **OpenAPI Specification**: Generated at `backend/docs/openapi.yaml` (run `go run main.go gendoc --openapi`)
- **RFID Integration Guide**: See `backend/docs/rfid-integration-guide.md`

### Key API Endpoints

- Authentication: `/api/auth/token`
- Students: `/api/students`
- Rooms: `/api/rooms`
- Activities: `/api/activities`
- Groups: `/api/groups`

## Troubleshooting

### Common Issues

- **Database Connection Errors**: Verify PostgreSQL is running and the DB_DSN is correct in .env
- **JWT Authentication Issues**: Check the `AUTH_JWT_SECRET` value in your .env file
- **CORS Errors**: Ensure `ENABLE_CORS` is set to `true` for development
- **Frontend API Connection**: Check that `NEXT_PUBLIC_API_URL` points to the correct backend URL

### Getting Help

- Check the logs with `docker compose logs service_name`
- Backend debugging: Set `LOG_LEVEL=debug` in the .env file
- Frontend debugging: Check the browser console and Next.js error overlay

## License

This project is licensed under the terms of the license file included in the repository. See LICENSE file for details.