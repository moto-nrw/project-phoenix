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

## Documentation

- API endpoints: See `backend/routes.md`
- RFID integration: See `backend/docs/rfid-integration.md`
- Architecture diagrams: See `/docs` directory

## License

See LICENSE file for details.