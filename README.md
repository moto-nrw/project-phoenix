# Project Phoenix

![moto Logo](frontend/public/images/moto_transparent.png)

## Security Notice

This repository now follows enhanced security practices:

- All sensitive configuration now uses example templates
- Real configuration files (.env, etc.) are no longer tracked
- SSL certificates must be generated locally
- See [Security Guidelines](docs/security.md) for details

### Quick Setup (New Development Environment)

```bash
# Clone the repository
git clone https://github.com/moto-nrw/project-phoenix.git
cd project-phoenix

# Run the setup script to create configuration files and certificates
./scripts/setup-dev.sh

# Start the development environment
docker-compose up -d
```

[![Go](https://img.shields.io/badge/go-1.21+-blue)](https://go.dev)
[![React](https://img.shields.io/badge/react-19-blue)](https://reactjs.org)
[![Next.js](https://img.shields.io/badge/next.js-15-blue)](https://nextjs.org)
[![TypeScript](https://img.shields.io/badge/typescript-5-blue)](https://www.typescriptlang.org)
[![PostgreSQL](https://img.shields.io/badge/postgresql-17-blue)](https://www.postgresql.org)
[![Docker](https://img.shields.io/badge/docker-compose-blue)](https://www.docker.com)

<p align="center">
  <strong>A modern RFID-based student attendance and room management system for educational institutions</strong>
</p>

<p align="center">
  <a href="#key-features">Features</a> •
  <a href="#tech-stack">Tech Stack</a> •
  <a href="#getting-started">Getting Started</a> •
  <a href="#deployment">Deployment</a> •
  <a href="#documentation">Documentation</a> •
  <a href="#contributing">Contributing</a> •
  <a href="#license">License</a>
</p>

## Overview

Project Phoenix is a comprehensive room and student management system designed for educational institutions. It leverages RFID technology to track student attendance and location in real-time, providing administrators with powerful tools for monitoring room occupancy, managing activities, and generating detailed analytics.

## Key Features

### Core Functionality
- 🏷️ **RFID Student Tracking** - Real-time location tracking using RFID technology
- 🏫 **Room Management** - Monitor room occupancy and usage patterns
- 👥 **Group Management** - Organize students into groups and manage activities
- 📊 **Analytics Dashboard** - Comprehensive reporting and utilization statistics
- 🗓️ **Schedule Management** - Handle class schedules and time-based activities
- 🎯 **Activity Tracking** - Track student participation in various activities

### Technical Features
- 🔐 **JWT Authentication** - Secure authentication with role-based access control
- 🚀 **RESTful API** - Well-documented API with OpenAPI specification
- 📱 **Responsive UI** - Modern, mobile-friendly interface
- 🐳 **Docker Support** - Easy deployment with containerization
- 🔄 **Real-time Updates** - Live tracking of student movements and room occupancy
- 🌐 **Multi-language Support** - Internationalization ready

## Tech Stack

### Backend
- **Language**: Go 1.21+
- **Framework**: Chi Router
- **ORM**: Bun ORM
- **Database**: PostgreSQL 17+
- **Authentication**: JWT with refresh tokens
- **Documentation**: OpenAPI 3.0

### Frontend
- **Framework**: Next.js 15+
- **Language**: TypeScript
- **UI Library**: React 19+
- **Styling**: Tailwind CSS 4+
- **State Management**: React Context
- **Authentication**: NextAuth.js

### Infrastructure
- **Containerization**: Docker & Docker Compose
- **Deployment**: Production-ready with Caddy
- **Monitoring**: Structured logging
- **CI/CD**: GitHub Actions ready

## Getting Started

### Prerequisites
- Docker and Docker Compose (recommended)
- Go 1.21+ (for backend development)
- Node.js 20+ and npm 11+ (for frontend development)
- PostgreSQL 17+ (if running without Docker)

### Quick Start with Docker

1. Clone the repository:
```bash
git clone https://github.com/moto-nrw/project-phoenix.git
cd project-phoenix
```

2. Generate SSL certificates for PostgreSQL (required for GDPR compliance):
```bash
# Generate self-signed certificates for local development
cd config/ssl/postgres
./create-certs.sh
cd ../../../  # Return to project root
```

3. Start all services:
```bash
docker compose up
```

4. Run database migrations:
```bash
docker compose run server ./main migrate
```

The application will be available at:
- Backend API: http://localhost:8080
- Frontend: http://localhost:3000

### Development Setup

#### Backend Development
```bash
cd backend
cp dev.env.example dev.env      # Create environment configuration

# Edit dev.env with your database credentials

# If using Docker for the database:
docker compose up -d postgres

# Run migrations and start the server:
go run main.go migrate
go run main.go serve
```

#### Frontend Development
```bash
cd frontend
npm install                     # Install dependencies
npm run dev                     # Start development server
```

### Environment Configuration

#### Backend Environment Variables (dev.env)
```env
# Database
DB_DSN=postgres://username:password@localhost:5432/database?sslmode=require
DB_DEBUG=true                   # Enable SQL query logging
# Note: sslmode=require enables SSL for GDPR compliance and security

# Authentication
AUTH_JWT_SECRET=your_jwt_secret_here
AUTH_JWT_EXPIRY=15m            # Access token expiry
AUTH_JWT_REFRESH_EXPIRY=1h     # Refresh token expiry

# Admin Account (for initial setup)
ADMIN_EMAIL=admin@example.com
ADMIN_PASSWORD=SecurePassword123!

# Server Configuration
LOG_LEVEL=debug                # Options: debug, info, warn, error
ENABLE_CORS=true              # Required for local development
PORT=8080
```

#### Frontend Environment Variables (.env.local)
```env
# API Configuration
NEXT_PUBLIC_API_URL=http://localhost:8080

# NextAuth Configuration
NEXTAUTH_URL=http://localhost:3000
NEXTAUTH_SECRET=your_nextauth_secret_here
```

## Command Reference

### Backend Commands
```bash
# Server Operations
go run main.go serve            # Start the backend server
go run main.go migrate          # Run database migrations
go run main.go migrate status   # Check migration status
go run main.go migrate reset    # Reset database (WARNING: deletes all data)

# Testing
go test ./...                   # Run all tests
go test ./api/auth -v          # Run specific package tests with verbose output
go test -race ./...            # Run tests with race condition detection

# Documentation
go run main.go gendoc           # Generate routes.md and OpenAPI spec
go run main.go gendoc --routes  # Generate only routes documentation
go run main.go gendoc --openapi # Generate only OpenAPI specification

# Code Quality
go fmt ./...                    # Format code
golangci-lint run              # Run linter
go mod tidy                    # Clean up dependencies
```

### Frontend Commands
```bash
# Development
npm run dev                     # Start development server
npm run build                   # Build for production
npm run start                   # Start production server
npm run preview                 # Preview production build

# Code Quality (Run before committing!)
npm run lint                    # ESLint check
npm run lint:fix               # Auto-fix linting issues
npm run typecheck              # TypeScript type checking
npm run check                  # Run both lint and typecheck

# Formatting
npm run format:check           # Check Prettier formatting
npm run format:write          # Fix formatting issues
```

### Docker Commands
```bash
# Service Management
docker compose up              # Start all services
docker compose up -d          # Start in detached mode
docker compose down           # Stop all services
docker compose logs -f        # View logs

# Database Operations
docker compose run server ./main migrate
docker compose exec postgres psql -U phoenix

# Frontend Operations
docker compose run frontend npm run lint
docker compose run frontend npm run build
```

## Architecture

### Project Structure
```
project-phoenix/
├── backend/                   # Go backend API
│   ├── api/                   # HTTP handlers and routes
│   ├── auth/                  # Authentication logic
│   ├── cmd/                   # CLI commands
│   ├── database/              # DB migrations and repositories
│   ├── models/                # Domain models
│   ├── services/              # Business logic
│   └── docs/                  # API documentation
├── frontend/                  # Next.js frontend
│   ├── src/
│   │   ├── app/              # Next.js App Router
│   │   ├── components/       # Reusable UI components
│   │   └── lib/              # Utilities and API clients
│   └── public/               # Static assets
├── deployment/               # Deployment configurations
├── docs/                     # Project documentation
└── docker-compose.yml        # Docker configuration
```

### Database Schema
The database uses PostgreSQL schemas to organize tables by domain:
- **auth**: Authentication and authorization
- **users**: User profiles, students, teachers, staff
- **education**: Groups and educational structures
- **facilities**: Rooms and physical locations
- **activities**: Student activities and enrollments
- **active**: Real-time session tracking
- **schedule**: Time and schedule management
- **iot**: RFID device management
- **feedback**: User feedback
- **config**: System configuration

## API Documentation

### Documentation Generation
```bash
cd backend
go run main.go gendoc          # Generate all documentation
```

This creates:
- `backend/routes.md` - Complete route documentation
- `backend/docs/openapi.yaml` - OpenAPI 3.0 specification

### Key API Endpoints
- **Authentication**: `/api/auth/login`, `/api/auth/token`
- **Students**: `/api/students`, `/api/students/{id}`
- **Rooms**: `/api/rooms`, `/api/rooms/{id}/history`
- **Activities**: `/api/activities`, `/api/activities/{id}/students`
- **Groups**: `/api/groups`, `/api/groups/{id}/students`
- **Active Sessions**: `/api/active/groups`, `/api/active/visits`

### RFID Integration
Detailed RFID integration documentation is available at:
- `backend/docs/rfid-integration-guide.md`
- `backend/docs/rfid-examples.md`

## Deployment

### Production Deployment with Docker

1. Clone the repository on your production server
2. Create production environment files:
```bash
cp backend/dev.env.example backend/prod.env
cp frontend/.env.local.example frontend/.env.production
```

3. Configure environment variables for production
4. Deploy using Docker Compose:
```bash
cd deployment/production
docker compose -f docker-compose.prod.yml up -d
```

### Configuration
Production deployment includes:
- Caddy for automatic HTTPS
- PostgreSQL with persistent volumes
- Health checks and restart policies
- Resource limits

## Development Workflow

### Initial Development Setup
1. Generate PostgreSQL SSL certificates (run once)
   ```bash
   cd config/ssl/postgres
   ./create-certs.sh
   ```
2. These certificates are excluded from git and must be generated by each developer
3. See `config/ssl/postgres/README.md` for more SSL details

### Backend Development Flow
1. Define models in `models/{domain}/`
2. Create repository interface and implementation
3. Implement service layer business logic
4. Create API handlers in `api/{domain}/`
5. Write comprehensive tests
6. Generate documentation

### Frontend Development Flow
1. Define TypeScript interfaces
2. Create API client functions
3. Implement service layer
4. Build UI components
5. Create pages and routes
6. Run lint and type checks

### Code Quality Standards
- Backend: Use `golangci-lint` and `go fmt`
- Frontend: ESLint with zero warnings policy
- Always run tests before committing
- Follow existing code patterns and conventions

## Testing

### Backend Testing
```bash
go test ./...                  # Run all tests
go test ./api/auth -v         # Run specific package tests
go test -race ./...           # Check for race conditions
```

### Frontend Testing
```bash
npm run lint                   # Run ESLint
npm run typecheck             # Run TypeScript checks
npm run check                 # Run all checks
```

## Troubleshooting

### Common Issues

**Database Connection**
- Verify PostgreSQL is running
- Check `DB_DSN` in environment configuration
- Ensure database exists and migrations are run
- Verify SSL certificates are generated (`config/ssl/postgres/create-certs.sh`)
- For SSL issues, check PostgreSQL logs for certificate-related errors

**Authentication Issues**
- Verify `AUTH_JWT_SECRET` is set
- Check token expiry configuration
- Ensure CORS is enabled for development

**Frontend API Connection**
- Verify `NEXT_PUBLIC_API_URL` is correct
- Check CORS configuration
- Ensure backend is running

**Docker Issues**
- Check port availability (3000, 8080, 5432)
- Verify volume permissions
- Review container logs with `docker compose logs`

## Contributing

We welcome contributions! Please follow these guidelines:

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

### Development Standards
- Write tests for new features
- Update documentation
- Follow existing code style
- Add appropriate logging
- Consider security implications

## Security

- Report security vulnerabilities to kontakt@moto.nrw
- Use environment variables for sensitive data
- Never commit secrets or SSL certificates to the repository
- Follow OWASP security guidelines
- Regular dependency updates
- Database connections use SSL encryption (GDPR compliance)
- Self-signed certificates for development, CA-signed for production

## License

This project is licensed under the GNU Affero General Public License v3.0 - see the [LICENSE](LICENSE) file for details.

## Contact

- **Project Website**: [moto.nrw](https://moto.nrw)
- **Email**: kontakt@moto.nrw
- **GitHub**: [github.com/moto-nrw/project-phoenix](https://github.com/moto-nrw/project-phoenix)

---

<p align="center">Made with ❤️ by moto</p>
