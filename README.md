# Project Phoenix

A modern room and student management system with RFID authentication support.

It's a Go backend API that helps schools track students, manage rooms, and organize activity groups. Key features include:

- RFID student location tracking system
- Room occupancy monitoring and visit history
- Student grouping and activity management
- JWT authentication with passwordless login
- PostgreSQL database with Bun ORM

The architecture uses Chi router for API endpoints with separate resource packages like rfid, room, student, and activity. It's designed
to help educational institutions monitor student whereabouts and manage facilities through a REST API that can be used by web browsers,
Tauri desktop apps, and RFID readers.


## Overview

Project Phoenix is a full-stack application designed to manage:
- Room occupancy and scheduling
- Student registration and tracking
- Group management and activities
- RFID-based authentication and access control

## Architecture

- **Backend**: Go-based REST API with JWT authentication
- **Database**: PostgreSQL with Bun ORM
- **Frontend**: incoming

## Getting Started

### Using Docker Compose

```bash
# Start the database
docker compose up -d postgres

# Run migrations
docker compose run server ./main migrate

# Start the API server
docker compose up
```

### Local Development

```bash
# Navigate to backend directory
cd backend

# Set up environment variables (modify dev.env as needed)
cp dev.env .env

# Run migrations
go run main.go migrate

# Start server
go run main.go serve
```

### Testing

```bash
# Run all tests
go test ./...
```

## Documentation

- API documentation: See `routes.md` and `/docs` directory
- RFID integration: See `backend/docs/rfid-integration.md`

## License

See LICENSE file for details.